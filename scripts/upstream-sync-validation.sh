#!/usr/bin/env bash
# Sourced afresh for each pass so candidate repairs apply in the running job.

backend_go_image() {
  local version
  version="$(awk '$1 == "go" { print $2 }' "$WORKTREE/backend/go.mod")" || return 1
  if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    printf 'Invalid backend Go version: %s\n' "$version" >&2
    return 1
  fi
  printf '%s\n' "${GO_IMAGE:-golang:$version}"
}

docker_go() {
  local image
  local -a integration_args=()
  image="$(backend_go_image)" || return 1
  if [[ " $* " == *' -tags=integration '* ]]; then
    # Testcontainers must reach Docker and the ephemeral ports it publishes.
    integration_args=(--network host -v /var/run/docker.sock:/var/run/docker.sock
      -v "$(command -v docker):/usr/local/bin/docker:ro" -e CI=true
      -e DOCKER_HOST=unix:///var/run/docker.sock -e TESTCONTAINERS_HOST_OVERRIDE=127.0.0.1)
  fi
  docker run --rm \
    "${integration_args[@]}" \
    -v "$WORKTREE:/src" \
    -v "$TOOLS_DIR:/tools" \
    -v sub2api-sync-go-mod:/go/pkg/mod \
    -v sub2api-sync-go-build:/root/.cache/go-build \
    -e GOTOOLCHAIN=local -e GOMAXPROCS=4 -e GOFLAGS=-p=4 \
    -w /src/backend "$image" "$@"
}

check_backend_security() {
  docker_go /bin/bash -ec '
    tool_dir="/tools/$(go env GOVERSION)/$(go env GOOS)-$(go env GOARCH)/govulncheck"
    mkdir -p "$tool_dir"
    test -x "$tool_dir/govulncheck" || GOBIN="$tool_dir" go install golang.org/x/vuln/cmd/govulncheck@latest
    "$tool_dir/govulncheck" ./...
  '
}

check_backend_lint() {
  docker_go /bin/bash -ec '
    tool_dir="/tools/$(go env GOVERSION)/$(go env GOOS)-$(go env GOARCH)/golangci-lint-v2.14.0"
    mkdir -p "$tool_dir"
    test -x "$tool_dir/golangci-lint" || GOBIN="$tool_dir" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
    "$tool_dir/golangci-lint" run --timeout=30m ./...
  '
}

check_frontend_audit() {
  (cd "$WORKTREE/frontend" && frontend_pnpm audit --prod --audit-level=high --json > audit.json || true)
  python3 "$WORKTREE/tools/check_pnpm_audit_exceptions.py" \
    --audit "$WORKTREE/frontend/audit.json" \
    --exceptions "$WORKTREE/.github/audit-exceptions.yml"
}

check_backend_integration() (
  local database_id port attempt
  database_id="$(docker run -d --rm -p 127.0.0.1::5432 \
    -e POSTGRES_PASSWORD=integration-test -e POSTGRES_DB=sub2api_test postgres:18-alpine)" || return 1
  trap 'docker rm -f "$database_id" >/dev/null 2>&1 || true' EXIT
  for attempt in {1..30}; do
    if docker exec "$database_id" pg_isready -U postgres -d sub2api_test >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
  docker exec "$database_id" pg_isready -U postgres -d sub2api_test >/dev/null || return 1
  port="$(docker port "$database_id" 5432/tcp)" || return 1
  docker_go env "MIGRATION_TEST_DATABASE_URL=postgres://postgres:integration-test@${port}/sub2api_test?sslmode=disable" \
    go test -count=1 -tags=integration ./...
)

check_deployment_scripts() {
  cd "$WORKTREE"
  /bin/bash -n deploy/apple-container.sh || return 1
  if command -v plutil >/dev/null 2>&1; then
    /bin/bash deploy/tests/apple-container-test.sh || return 1
  else
    log 'skipping macOS-only apple-container runtime test because plutil is unavailable'
  fi
  /bin/sh deploy/tests/docker-compose-security-test.sh || return 1
  /bin/sh deploy/tests/docker-compose-gateway-env-test.sh || return 1
  /bin/sh deploy/tests/docker-runtime-resources-test.sh || return 1
  /bin/sh deploy/test-caddyfile-cache.sh || return 1
}

frontend_pnpm() {
  "$COREPACK_BIN" "pnpm@$PNPM_VERSION" "$@"
}

run_validation_checks() {
  local pass="$1"
  : > "$VALIDATION_FAILURES_FILE"
  VALIDATION_RESULT="第 $pass 轮全量验证进行中"
  log "starting complete validation pass $pass"
  record_check '候选差异空白检查' git -C "$WORKTREE" diff HEAD --check
  record_check '上游功能排除检查' python3 "$WORKTREE/.github/check-upstream-exclusions.py" "$WORKTREE"
  record_check '上游排除规则及报告测试' python3 -m unittest discover -s "$WORKTREE/.github" -p 'test_upstream*.py'
  record_check '后端普通测试' docker_go go test ./...
  record_check '后端单元测试' docker_go go test -tags=unit ./...
  record_check '后端集成测试' check_backend_integration
  record_check '后端静态检查' docker_go go vet ./...
  record_check 'golangci-lint' check_backend_lint
  record_check '后端漏洞检查' check_backend_security
  record_check '后端生产构建' docker_go /bin/bash -ec 'CGO_ENABLED=0 go build -trimpath -o /tmp/sub2api-server ./cmd/server'
  record_check '前端依赖锁定安装' frontend_pnpm --dir "$WORKTREE/frontend" install --frozen-lockfile
  record_check '前端 ESLint' frontend_pnpm --dir "$WORKTREE/frontend" run lint:check
  record_check '前端类型检查' frontend_pnpm --dir "$WORKTREE/frontend" run typecheck
  record_check '前端完整测试' frontend_pnpm --dir "$WORKTREE/frontend" run test:run --maxWorkers=2
  record_check '前端生产构建及 i18n' frontend_pnpm --dir "$WORKTREE/frontend" run build
  record_check '前端依赖安全审计' check_frontend_audit
  record_check '部署脚本验证' check_deployment_scripts
  CURRENT_STAGE="全量验证结果汇总"
  if [[ -s "$VALIDATION_FAILURES_FILE" ]]; then
    VALIDATION_RESULT="第 $pass 轮失败（$(wc -l < "$VALIDATION_FAILURES_FILE") 项）"
    return 1
  fi
  VALIDATION_RESULT="第 $pass 轮全部通过"
  return 0
}
