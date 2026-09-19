#!/usr/bin/env bash

set -Eeuo pipefail
umask 022

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
AUTOMATION_DIR="${SUB2API_SYNC_DIR:-/root/workspace/automation/sub2api-upstream-sync}"
CONFIG_FILE="${SUB2API_SYNC_CONFIG:-$AUTOMATION_DIR/config.env}"
if [[ -f "$CONFIG_FILE" ]]; then
  # The scheduled configuration is local and should be owned by root.
  # shellcheck disable=SC1090
  source "$CONFIG_FILE"
fi
REPO_DIR="${SUB2API_REPO_DIR:-$(cd -- "$SCRIPT_DIR/.." && pwd)}"
TARGET_BRANCH="${SUB2API_TARGET_BRANCH:-main}"
ORIGIN_REMOTE="${SUB2API_ORIGIN_REMOTE:-origin}"
PRIMARY_REMOTE="${SUB2API_PRIMARY_REMOTE:-upstream}"
PRIMARY_BRANCH="${SUB2API_PRIMARY_BRANCH:-main}"
SECOND_REMOTE="${SUB2API_SECOND_REMOTE:-overdraft}"
SECOND_BRANCH="${SUB2API_SECOND_BRANCH:-sub2api-custom}"
CODEX_BIN="${SUB2API_CODEX_BIN:-/usr/bin/codex}"
VALIDATE="${SUB2API_VALIDATE:-true}"
DRY_RUN="${SUB2API_DRY_RUN:-false}"

STATE_DIR="$AUTOMATION_DIR/state"
WORKTREE_ROOT="$AUTOMATION_DIR/worktrees"
LOG_DIR="$AUTOMATION_DIR/logs"
LOCK_FILE="$STATE_DIR/sync.lock"

mkdir -p "$STATE_DIR" "$WORKTREE_ROOT" "$LOG_DIR"
exec 9>"$LOCK_FILE"
if ! flock -n 9; then
  printf '[%s] another sub2api upstream sync is already running\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
  exit 0
fi

RUN_ID="$(date -u +'%Y%m%dT%H%M%SZ')-$$"
WORKTREE="$WORKTREE_ROOT/$RUN_ID"
LOG_FILE="$LOG_DIR/$RUN_ID.log"
PRIMARY_REF="$PRIMARY_REMOTE/$PRIMARY_BRANCH"
SECOND_REF="$SECOND_REMOTE/$SECOND_BRANCH"
ORIGIN_REF="$ORIGIN_REMOTE/$TARGET_BRANCH"
SYNC_BRANCH="automation/sub2api-upstream-sync-$RUN_ID"
WORKTREE_CREATED=false

exec > >(tee -a "$LOG_FILE") 2>&1

log() {
  printf '[%s] %s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$*"
}

fail() {
  log "BLOCKED: $*"
  exit 1
}

cleanup() {
  local exit_code=$?
  trap - EXIT
  if [[ "$WORKTREE_CREATED" == true ]]; then
    git -C "$WORKTREE" merge --abort >/dev/null 2>&1 || true
    git -C "$REPO_DIR" worktree remove --force "$WORKTREE" >/dev/null 2>&1 || true
    git -C "$REPO_DIR" branch -D "$SYNC_BRANCH" >/dev/null 2>&1 || true
  fi
  exit "$exit_code"
}
trap cleanup EXIT

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

validate_primary_worktree() {
  local branch_name ahead behind
  [[ -d "$REPO_DIR/.git" ]] || fail "repository not found: $REPO_DIR"
  branch_name="$(git -C "$REPO_DIR" branch --show-current)"
  [[ "$branch_name" == "$TARGET_BRANCH" ]] || fail "current branch is ${branch_name:-detached}, expected $TARGET_BRANCH"
  [[ -z "$(git -C "$REPO_DIR" status --porcelain=v1)" ]] || fail 'primary worktree is dirty; resolve and push local changes before scheduled sync'
  read -r behind ahead < <(git -C "$REPO_DIR" rev-list --left-right --count "$ORIGIN_REF...HEAD")
  if (( behind > 0 && ahead > 0 )); then
    fail "local branch diverged from $ORIGIN_REF (ahead=$ahead behind=$behind)"
  fi
  if (( ahead > 0 )); then
    log "pushing $ahead existing local commit(s) before upstream merge"
    git -C "$REPO_DIR" push "$ORIGIN_REMOTE" "HEAD:$TARGET_BRANCH"
    git -C "$REPO_DIR" fetch "$ORIGIN_REMOTE" "$TARGET_BRANCH"
  elif (( behind > 0 )); then
    git -C "$REPO_DIR" merge --ff-only "$ORIGIN_REF"
  fi
}

run_codex_merge_review() {
  local phase="${1:-final双上游合并}"
  local prompt_file="$STATE_DIR/$RUN_ID-review.txt"
  cat > "$prompt_file" <<EOF
审查并完成当前临时工作树中的${phase}。

仓库：$REPO_DIR
临时工作树：$WORKTREE
本项目基准：$ORIGIN_REF
主上游：$PRIMARY_REF
第二上游：$SECOND_REF

产品决策（必须严格执行）：
1. $PRIMARY_REF 的所有代码和功能都无条件保留并合并。
2. $SECOND_REF 的所有代码和功能都无条件保留并合并，唯一例外是“共享账号池”功能及其支持实现。不要因为功能较大、文件较多或与本项目已有实现相邻，就额外排除第二上游的其他功能。
3. 共享账号池包括其用户/管理员页面、API、数据库迁移、账号调度/权限/计费支持、测试、文档和专属资源；与普通账号池、Gemini 上游共享额度等同名但不属于该功能的代码必须保留。

要求：
- 阅读 AGENTS.md、两上游提交记录、双方差异和当前合并结果。
- 处理所有文本冲突和语义冲突；禁止简单选择 ours/theirs。
- 删除或恢复仅属于共享账号池的实现，保留第二上游的其他全部更新。
- 保留本项目已有功能、权限、计费、数据库兼容性和测试不变量。
- 不要 fetch、commit、push、打 tag、发布或重启服务。
- 直接修改工作树文件；外层脚本负责暂存、验证、提交和推送。
- 完成后确认没有未解决冲突、冲突标记或明显的共享账号池专属文件残留。
EOF
  log "running Codex merge review: $phase"
  "$CODEX_BIN" exec --ephemeral --sandbox workspace-write --color never -C "$WORKTREE" - < "$prompt_file"
}

finish_merge_stage() {
  local ref="$1"
  if ! git -C "$WORKTREE" rev-parse -q --verify MERGE_HEAD >/dev/null; then
    return 0
  fi
  if [[ -n "$(git -C "$WORKTREE" diff --name-only --diff-filter=U)" ]]; then
    run_codex_merge_review "解决 $ref 的文本与语义冲突"
  fi
  git -C "$WORKTREE" add --all
  [[ -z "$(git -C "$WORKTREE" diff --name-only --diff-filter=U)" ]] || fail "$ref remains unresolved after Codex review"
  git -C "$WORKTREE" diff --cached --check
  git -C "$WORKTREE" commit --no-edit
}

run_validation() {
  [[ "$VALIDATE" == true ]] || { log 'validation skipped by configuration'; return; }
  log 'running backend tests'
  (cd "$WORKTREE/backend" && go test ./...)
  log 'running backend vet'
  (cd "$WORKTREE/backend" && go vet ./...)
  log 'running frontend checks'
  pnpm --dir "$WORKTREE/frontend" install --frozen-lockfile
  pnpm --dir "$WORKTREE/frontend" run lint:check
  pnpm --dir "$WORKTREE/frontend" run typecheck
  pnpm --dir "$WORKTREE/frontend" run check:i18n
  pnpm --dir "$WORKTREE/frontend" run build
}

main() {
  require_command git
  require_command "$CODEX_BIN"
  require_command go
  require_command pnpm
  validate_primary_worktree

  log "fetching $ORIGIN_REMOTE, $PRIMARY_REMOTE and $SECOND_REMOTE"
  git -C "$REPO_DIR" fetch --prune "$ORIGIN_REMOTE"
  git -C "$REPO_DIR" fetch --prune "$PRIMARY_REMOTE"
  git -C "$REPO_DIR" fetch --prune "$SECOND_REMOTE"
  validate_primary_worktree

  if git -C "$REPO_DIR" merge-base --is-ancestor "$PRIMARY_REF" "$ORIGIN_REF" && \
    git -C "$REPO_DIR" merge-base --is-ancestor "$SECOND_REF" "$ORIGIN_REF"; then
    log 'no pending updates from either upstream'
    exit 0
  fi

  git -C "$REPO_DIR" worktree add --detach "$WORKTREE" "$ORIGIN_REF"
  WORKTREE_CREATED=true
  git -C "$WORKTREE" switch -c "$SYNC_BRANCH"

  log "merging all primary upstream changes from $PRIMARY_REF"
  if ! git -C "$WORKTREE" merge --no-ff --no-commit "$PRIMARY_REF"; then
    git -C "$WORKTREE" rev-parse -q --verify MERGE_HEAD >/dev/null || fail "unable to start merge for $PRIMARY_REF"
  fi
  finish_merge_stage "$PRIMARY_REF"
  log "merging all second upstream changes from $SECOND_REF"
  if ! git -C "$WORKTREE" merge --no-ff --no-commit "$SECOND_REF"; then
    git -C "$WORKTREE" rev-parse -q --verify MERGE_HEAD >/dev/null || fail "unable to start merge for $SECOND_REF"
  fi
  finish_merge_stage "$SECOND_REF"
  run_codex_merge_review '最终双上游兼容审查'

  git -C "$WORKTREE" add --all
  [[ -z "$(git -C "$WORKTREE" diff --name-only --diff-filter=U)" ]] || fail 'unresolved merge entries remain after Codex review'
  git -C "$WORKTREE" diff --cached --check
  if git -C "$WORKTREE" diff --cached --name-only | grep -Eiq '(^|/)(shared.?pool|shared_account|shared-account|SharedPool|add-shared-account-pool)'; then
    fail 'shared account pool files remain in the candidate merge'
  fi
  if [[ "$DRY_RUN" == true ]]; then
    log 'dry run completed; candidate was not committed or pushed'
    exit 0
  fi

  git -C "$WORKTREE" reset --soft "$ORIGIN_REF"
  git -C "$WORKTREE" commit -m "merge: synchronize primary and custom upstreams"
  run_validation
  [[ -z "$(git -C "$WORKTREE" status --porcelain=v1)" ]] || fail 'validation modified tracked files; refusing to push'
  git -C "$WORKTREE" push "$ORIGIN_REMOTE" "HEAD:$TARGET_BRANCH"
  git -C "$REPO_DIR" fetch "$ORIGIN_REMOTE" "$TARGET_BRANCH"
  git -C "$REPO_DIR" merge --ff-only "$ORIGIN_REF"
  log "upstream sync completed: $(git -C "$REPO_DIR" rev-parse HEAD)"
}

main "$@"
