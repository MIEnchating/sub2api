#!/usr/bin/env bash

set -Eeuo pipefail
umask 077

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
AUTOMATION_DIR="${SUB2API_SYNC_DIR:-/root/workspace/automation/sub2api-upstream-sync}"
CONFIG_FILE="${SUB2API_SYNC_CONFIG:-$AUTOMATION_DIR/config.env}"
if [[ -f "$CONFIG_FILE" ]]; then
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
GO_IMAGE="${SUB2API_GO_IMAGE:-golang:1.27.0}"
COREPACK_BIN="${SUB2API_COREPACK_BIN:-/usr/bin/corepack}"
PNPM_VERSION="${SUB2API_PNPM_VERSION:-9.15.9}"
VALIDATE="${SUB2API_VALIDATE:-true}"
DRY_RUN="${SUB2API_DRY_RUN:-false}"
VALIDATION_REPAIR_ATTEMPTS="${SUB2API_VALIDATION_REPAIR_ATTEMPTS:-3}"
REVIEW_REPAIR_ATTEMPTS="${SUB2API_REVIEW_REPAIR_ATTEMPTS:-3}"
RELEASE_ENABLED="${SUB2API_RELEASE_ENABLED:-true}"
RELEASE_MIN_UPSTREAM_COMMITS="${SUB2API_RELEASE_MIN_UPSTREAM_COMMITS:-3}"
RELEASE_MIN_CHANGED_FILES="${SUB2API_RELEASE_MIN_CHANGED_FILES:-8}"
RELEASE_MIN_DIFF_LINES="${SUB2API_RELEASE_MIN_DIFF_LINES:-150}"
RELEASE_REQUIRE_NO_RISKS="${SUB2API_RELEASE_REQUIRE_NO_RISKS:-true}"
RELEASE_IGNORE_ENVIRONMENT_RISKS="${SUB2API_RELEASE_IGNORE_ENVIRONMENT_RISKS:-true}"
REMOTE_WORKFLOW_TIMEOUT_MINUTES="${SUB2API_REMOTE_WORKFLOW_TIMEOUT_MINUTES:-90}"
REMOTE_WORKFLOW_POLL_SECONDS="${SUB2API_REMOTE_WORKFLOW_POLL_SECONDS:-15}"
REMOTE_WORKFLOW_RETRY_ATTEMPTS="${SUB2API_REMOTE_WORKFLOW_RETRY_ATTEMPTS:-1}"
EMAIL_ENABLED="${EMAIL_ENABLED:-false}"
EMAIL_TO="${EMAIL_TO:-}"
SMTP_CONFIG_SOURCE="${SMTP_CONFIG_SOURCE:-environment}"

STATE_DIR="$AUTOMATION_DIR/state"
WORKTREE_ROOT="$AUTOMATION_DIR/worktrees"
LOG_DIR="$AUTOMATION_DIR/logs"
TOOLS_DIR="$AUTOMATION_DIR/tools"
LOCK_FILE="$STATE_DIR/sync.lock"

mkdir -p "$STATE_DIR" "$WORKTREE_ROOT" "$LOG_DIR" "$TOOLS_DIR"
exec 9>"$LOCK_FILE"
if ! flock -n 9; then
  printf '[%s] another sub2api upstream sync is already running\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
  exit 0
fi

RUN_ID="$(date -u +'%Y%m%dT%H%M%SZ')-$$"
WORKTREE="$WORKTREE_ROOT/$RUN_ID"
LOG_FILE="$LOG_DIR/$RUN_ID.log"
REPORT_FILE="$STATE_DIR/last-report.txt"
HTML_REPORT_FILE="$STATE_DIR/last-report.html"
FAILURE_SUMMARY_FILE="$STATE_DIR/$RUN_ID-failure-summary.txt"
VALIDATION_FAILURES_FILE="$STATE_DIR/$RUN_ID-validation-failures.txt"
REVIEW_SUMMARY_FILE="$STATE_DIR/$RUN_ID-review-summary.txt"
REVIEW_DECISION_FILE="$STATE_DIR/$RUN_ID-review-decision.json"
RELEASE_NOTES_FILE="$STATE_DIR/$RUN_ID-release-notes.txt"
PRIMARY_REF="$PRIMARY_REMOTE/$PRIMARY_BRANCH"
SECOND_REF="$SECOND_REMOTE/$SECOND_BRANCH"
ORIGIN_REF="$ORIGIN_REMOTE/$TARGET_BRANCH"
SYNC_BRANCH="automation/sub2api-upstream-sync-$RUN_ID"
WORKTREE_CREATED=false
NOTIFIED=false
CURRENT_STAGE='初始化'
ORIGIN_HEAD=''
PRIMARY_HEAD=''
SECOND_HEAD=''
CANDIDATE_COMMIT=''
PUSHED_COMMIT=''
VALIDATION_RESULT='未执行'
VALIDATION_SUCCEEDED=false
REPAIR_COUNT=0
FAILED_COMMAND=''
RELEASE_STATE='未评估'
RELEASE_TAG=''
RELEASE_BASE_TAG=''
RELEASE_BASE_COMMIT=''
RELEASE_REASON=''
REMOTE_WORKFLOW_STATE='未执行'
REMOTE_REPAIR_COUNT=0
PENDING_RELEASE_FILE="$STATE_DIR/pending-release.env"
PENDING_RELEASE_NOTES_FILE="$STATE_DIR/pending-release-notes.txt"

exec > >(tee -a "$LOG_FILE") 2>&1

log() {
  printf '[%s] %s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$*"
}

load_smtp_config() {
  local smtp_rows key value
  [[ "$SMTP_CONFIG_SOURCE" == 'new_api_postgres' ]] || return 0
  : "${SMTP_POSTGRES_CONTAINER:=postgres}"
  : "${SMTP_DB_USER:=root}"
  : "${SMTP_DB_NAME:=new-api}"
  smtp_rows="$(docker exec "$SMTP_POSTGRES_CONTAINER" psql \
    -U "$SMTP_DB_USER" -d "$SMTP_DB_NAME" -AtF $'\t' \
    -c "SELECT key, value FROM options WHERE key IN ('SMTPServer','SMTPPort','SMTPSSLEnabled','SMTPStartTLSEnabled','SMTPAccount','SMTPFrom','SMTPToken') ORDER BY key;")" || return 1
  while IFS=$'\t' read -r key value; do
    case "$key" in
      SMTPAccount) SMTP_USERNAME="$value" ;;
      SMTPFrom) EMAIL_FROM="$value" ;;
      SMTPPort) SMTP_PORT="$value" ;;
      SMTPServer) SMTP_HOST="$value" ;;
      SMTPSSLEnabled) SMTP_SSL_ENABLED="$value" ;;
      SMTPStartTLSEnabled) SMTP_STARTTLS_ENABLED="$value" ;;
      SMTPToken) SMTP_AUTH_CODE="$value" ;;
    esac
  done <<< "$smtp_rows"
}

send_email() {
  local subject="$1" body_file="$2"
  local message_file curl_config smtp_scheme escaped_user
  if [[ "$EMAIL_ENABLED" != 'true' ]]; then
    log "email disabled; report retained at $REPORT_FILE"
    return 0
  fi
  load_smtp_config || { log 'unable to load SMTP configuration'; return 1; }
  if [[ -z "${SMTP_HOST:-}" || -z "${SMTP_PORT:-}" || -z "${SMTP_USERNAME:-}" || \
    -z "${SMTP_AUTH_CODE:-}" || -z "${EMAIL_FROM:-}" || -z "$EMAIL_TO" ]]; then
    log 'email enabled, but SMTP configuration is incomplete'
    return 1
  fi
  if [[ "$SMTP_AUTH_CODE" == *$'\n'* || "$SMTP_AUTH_CODE" == *$'\r'* || \
    ! "$SMTP_HOST" =~ ^[A-Za-z0-9.-]+$ || ! "$SMTP_PORT" =~ ^[0-9]+$ || \
    ! "$SMTP_USERNAME" =~ ^[A-Za-z0-9@._+-]+$ || \
    ! "$EMAIL_FROM" =~ ^[A-Za-z0-9._+-]+@[A-Za-z0-9.-]+$ || \
    ! "$EMAIL_TO" =~ ^[A-Za-z0-9._+-]+@[A-Za-z0-9.-]+$ ]]; then
    log 'SMTP configuration contains an invalid value'
    return 1
  fi

  message_file="$STATE_DIR/$RUN_ID-message.eml"
  curl_config="$STATE_DIR/$RUN_ID-curl.conf"
  smtp_scheme='smtp'
  [[ "${SMTP_SSL_ENABLED:-false}" == 'true' ]] && smtp_scheme='smtps'
  if ! python3 "$SCRIPT_DIR/../.github/render-upstream-sync-email.py" message \
    --text "$body_file" --html "$HTML_REPORT_FILE" --output "$message_file" \
    --subject "$subject" --sender "$EMAIL_FROM" --recipient "$EMAIL_TO"; then
    log 'unable to compose email; local report retained'
    rm -f "$message_file"
    return 1
  fi
  escaped_user="${SMTP_USERNAME//\\/\\\\}:${SMTP_AUTH_CODE//\\/\\\\}"
  escaped_user="${escaped_user//\"/\\\"}"
  {
    printf 'url = "%s://%s:%s"\n' "$smtp_scheme" "$SMTP_HOST" "$SMTP_PORT"
    printf 'user = "%s"\n' "$escaped_user"
    printf 'mail-from = "%s"\n' "$EMAIL_FROM"
    printf 'mail-rcpt = "%s"\n' "$EMAIL_TO"
    printf 'upload-file = "%s"\n' "$message_file"
    printf 'silent\nshow-error\nfail\n'
    if [[ "$smtp_scheme" == 'smtps' || "${SMTP_STARTTLS_ENABLED:-false}" == 'true' ]]; then
      printf 'ssl-reqd\n'
    fi
  } > "$curl_config"
  chmod 600 "$message_file" "$curl_config"
  if ! /usr/bin/curl --config "$curl_config"; then
    log "email delivery failed; report retained at $REPORT_FILE"
    rm -f "$message_file" "$curl_config"
    return 1
  fi
  rm -f "$message_file" "$curl_config"
  log "notification sent to $EMAIL_TO"
}

workflow_trigger_state() {
  local workflow="$1" commit="$2" path yaml name
  while IFS= read -r path; do
    [[ "$path" =~ \.ya?ml$ ]] || continue
    yaml="$(git -C "$REPO_DIR" show "$commit:$path" 2>/dev/null || true)"
    name="$(printf '%s\n' "$yaml" | sed -n 's/^name:[[:space:]]*//p' | head -1 | sed -E 's/^['\"']|['\"']$//g')"
    [[ "$name" == "$workflow" ]] || continue
    if grep -Eq '^[[:space:]]+push:' <<< "$yaml"; then
      return 0
    fi
    return 1
  done < <(git -C "$REPO_DIR" ls-tree -r --name-only "$commit" .github/workflows 2>/dev/null)
  return 2
}

assert_no_conflict_markers() {
  local markers
  markers="$(git -C "$WORKTREE" grep -n -I -E '^(<<<<<<<|>>>>>>>)( |$)' -- . 2>/dev/null || true)"
  [[ -z "$markers" ]] || {
    log "unresolved conflict markers remain:\n$markers"
    return 1
  }
}

wait_for_remote_workflow() {
  local workflow="$1" commit="$2" label="$3" ref="${4:-}" repo run_json run_info run_id status conclusion trigger_state
  REMOTE_FAILED_RUN_ID=''
  if workflow_trigger_state "$workflow" "$commit"; then
    trigger_state=0
  else
    trigger_state=$?
  fi
  if (( trigger_state == 2 )); then
    log "$label workflow is not defined for commit $commit; skipping this optional gate"
    return 2
  fi
  if (( trigger_state == 1 )); then
    log "$label workflow has no push trigger for commit $commit; local validation and the release preflight remain the gates"
    return 2
  fi
  local retries=0 deadline=$((SECONDS + REMOTE_WORKFLOW_TIMEOUT_MINUTES * 60))
  repo="$(git -C "$REPO_DIR" remote get-url --push "$ORIGIN_REMOTE" | sed -E 's#^git@github.com:##; s#^https://github.com/##; s#\.git$##')"
  while (( SECONDS < deadline )); do
    local -a run_args=(gh run list --repo "$repo" --workflow "$workflow" --commit "$commit" --limit 20 \
      --json databaseId,status,conclusion,headSha)
    [[ -n "$ref" ]] && run_args+=(--branch "$ref")
    run_json="$("${run_args[@]}" 2>/dev/null || true)"
    run_info="$(printf '%s' "$run_json" | python3 -c '
import json, sys
try:
    rows = json.load(sys.stdin)
except Exception:
    rows = []
sha = sys.argv[1]
matching = [row for row in rows if row.get("headSha") == sha]
if matching:
    row = max(matching, key=lambda item: int(item.get("databaseId", 0)))
    print(row.get("databaseId"), row.get("status"), row.get("conclusion") or "")
' "$commit")"
    if [[ -n "$run_info" ]]; then
      read -r run_id status conclusion <<< "$run_info"
      log "$label workflow $run_id: $status ${conclusion:-}"
      if [[ "$status" == completed ]]; then
        if [[ "$conclusion" == success ]]; then return 0; fi
        if (( retries < REMOTE_WORKFLOW_RETRY_ATTEMPTS )); then
          retries=$((retries + 1))
          log "$label workflow failed; rerunning failed jobs ($retries/$REMOTE_WORKFLOW_RETRY_ATTEMPTS)"
          gh run rerun "$run_id" --repo "$repo" --failed >/dev/null
        else
          REMOTE_FAILED_RUN_ID="$run_id"
          return 1
        fi
      fi
    else
      log "waiting for $label workflow for commit $commit"
    fi
    sleep "$REMOTE_WORKFLOW_POLL_SECONDS"
  done
  return 1
}

wait_for_remote_workflows() {
  local commit="$1" result skipped=0 workflow repo check_log remote_head
  CURRENT_STAGE='等待远程 CI 与安全扫描'
  REMOTE_WORKFLOW_STATE='等待中'
  require_command gh
  repo="$(git -C "$REPO_DIR" remote get-url --push "$ORIGIN_REMOTE" | sed -E 's#^git@github.com:##; s#^https://github.com/##; s#\.git$##')"
  while true; do
    skipped=0
    : > "$VALIDATION_FAILURES_FILE"
    for workflow in 'CI' 'Security Scan'; do
      if wait_for_remote_workflow "$workflow" "$commit" "$workflow"; then continue; else result=$?; fi
      if (( result == 2 )); then skipped=$((skipped + 1)); continue; fi
      check_log="$(mktemp "$STATE_DIR/$RUN_ID-remote.log.XXXXXX")"
      if [[ -n "$REMOTE_FAILED_RUN_ID" ]]; then
        python3 "$SCRIPT_DIR/../.github/upstream-remote-failure.py" "$repo" "$REMOTE_FAILED_RUN_ID" "$check_log" || true
      else
        printf '%s 未确认结束或等待超时；不得猜测测试错误。\n' "$workflow" > "$check_log"
      fi
      printf '%s（提交 %s，日志：%s）\n' "$workflow" "$commit" "$check_log" >> "$VALIDATION_FAILURES_FILE"
    done
    [[ -s "$VALIDATION_FAILURES_FILE" ]] || break
    REMOTE_WORKFLOW_STATE='失败，自动修复中'
    (( REMOTE_REPAIR_COUNT < VALIDATION_REPAIR_ATTEMPTS )) || fail '远程检查经过集中修复后仍失败，详见失败作业日志'
    git -C "$REPO_DIR" fetch "$ORIGIN_REMOTE" "$TARGET_BRANCH" || fail '无法获取远程修复基准'
    remote_head="$(git -C "$REPO_DIR" rev-parse "$ORIGIN_REF")"
    [[ "$remote_head" == "$commit" ]] || fail '远程主分支已有并发更新，保留候选，不覆盖其他提交'
    if [[ "$WORKTREE_CREATED" != true ]]; then
      git -C "$REPO_DIR" worktree add -b "$SYNC_BRANCH" "$WORKTREE" "$commit" || fail '无法创建远程修复工作树'
      WORKTREE_CREATED=true
    fi
    [[ "$(git -C "$WORKTREE" rev-parse HEAD)" == "$commit" ]] || fail '远程修复工作树不匹配已推送提交'
    REMOTE_REPAIR_COUNT=$((REMOTE_REPAIR_COUNT + 1))
    CANDIDATE_COMMIT="$commit"
    run_codex_validation_repair "$REMOTE_REPAIR_COUNT" || fail '远程检查修复失败'
    [[ "$CANDIDATE_COMMIT" != "$commit" ]] || fail '远程错误无法安全自动修复，已保留具体失败日志'
    run_validation_with_repairs
    [[ "$VALIDATION_SUCCEEDED" == true ]] || fail '远程修复后的完整本地验证仍未通过'
    [[ -z "$(git -C "$WORKTREE" status --porcelain=v1)" ]] || fail '远程修复验证后工作树不干净'
    git -C "$WORKTREE" push "$ORIGIN_REMOTE" "HEAD:$TARGET_BRANCH" || fail '远程修复推送失败，保留候选'
    commit="$(git -C "$WORKTREE" rev-parse HEAD)"
    PUSHED_COMMIT="$commit"
    if [[ -n "$RELEASE_TAG" && -s "$RELEASE_NOTES_FILE" ]]; then
      persist_pending_release "$RELEASE_TAG" "$commit"
    fi
  done
  if (( skipped == 0 )); then
    REMOTE_WORKFLOW_STATE='CI 与安全扫描通过'
  else
    REMOTE_WORKFLOW_STATE="适用远程工作流通过，${skipped} 个未配置 push 触发的可选工作流已跳过"
  fi
}

wait_for_release_workflow() {
  local tag="$1" commit="$2" repo release_json
  repo="$(git -C "$REPO_DIR" remote get-url --push "$ORIGIN_REMOTE" | sed -E 's#^git@github.com:##; s#^https://github.com/##; s#\.git$##')"
  CURRENT_STAGE='等待远程 Release 工作流'
  wait_for_remote_workflow 'Release' "$commit" 'Release' "$tag" || fail "远程 Release 工作流未通过或超时（标签 $tag）"
  release_json="$(gh release view "$tag" --repo "$repo" --json isDraft,isPrerelease,assets 2>/dev/null || true)"
  python3 -c 'import json,sys; d=json.load(sys.stdin); raise SystemExit(0 if not d.get("isDraft") and not d.get("isPrerelease") and d.get("assets") else 1)' <<< "$release_json" || \
    fail "远程 Release 工作流虽结束，但正式 Release 或资产未确认（标签 $tag）"
  RELEASE_STATE='已完成发布'
  REMOTE_WORKFLOW_STATE='CI、安全扫描与 Release 全部通过'
}

write_report() {
  local status="$1" reason="$2"
  {
    printf 'sub2api 双上游同步报告\n'
    printf '========================\n\n'
    printf '结果：%s\n' "$status"
    printf '结论：%s\n' "$reason"
    printf '执行时间：%s（北京时间）\n' "$(TZ=Asia/Shanghai date +'%Y-%m-%d %H:%M:%S')"
    printf '失败/当前阶段：%s\n' "$CURRENT_STAGE"
    printf '目标分支：%s\n' "$ORIGIN_REF"
    printf '主上游：%s\n' "$PRIMARY_REF"
    printf '第二上游：%s（排除共享账号池和批量生图）\n' "$SECOND_REF"
    [[ -n "$ORIGIN_HEAD" ]] && printf '合并前版本：%s\n' "$ORIGIN_HEAD"
    [[ -n "$PRIMARY_HEAD" ]] && printf '主上游版本：%s\n' "$PRIMARY_HEAD"
    [[ -n "$SECOND_HEAD" ]] && printf '第二上游版本：%s\n' "$SECOND_HEAD"
    [[ -n "$CANDIDATE_COMMIT" ]] && printf '候选版本：%s\n' "$CANDIDATE_COMMIT"
    printf '已推送版本：%s\n' "${PUSHED_COMMIT:-未推送}"
    printf '全量验证：%s\n' "$VALIDATION_RESULT"
    printf 'Codex 集中修复次数：%s/%s\n' "$REPAIR_COUNT" "$VALIDATION_REPAIR_ATTEMPTS"
    printf '远程 CI 自动修复次数：%s/%s\n' "${REMOTE_REPAIR_COUNT:-0}" "$VALIDATION_REPAIR_ATTEMPTS"
    printf '版本发布：%s\n' "$RELEASE_STATE"
    printf '远程工作流：%s\n' "${REMOTE_WORKFLOW_STATE:-未执行}"
    [[ -n "$RELEASE_TAG" ]] && printf '版本标签：%s\n' "$RELEASE_TAG"
    [[ -n "$RELEASE_REASON" ]] && printf '发布判断：%s\n' "$RELEASE_REASON"
    if [[ -s "$FAILURE_SUMMARY_FILE" ]]; then
      printf '\n具体失败原因\n------------\n'
      cat "$FAILURE_SUMMARY_FILE"
      printf '\n'
    fi
    if [[ -s "$VALIDATION_FAILURES_FILE" ]]; then
      printf '\n最终失败检查\n------------\n'
      cat "$VALIDATION_FAILURES_FILE"
    fi
    if [[ -s "$REVIEW_SUMMARY_FILE" ]]; then
      printf '\nCodex 合并审查及功能排除记录\n----------------------------------\n'
      cat "$REVIEW_SUMMARY_FILE"
      printf '\n'
    fi
    printf '\n完整日志：%s\n' "$LOG_FILE"
  } > "$REPORT_FILE"
  # A rendering error must not prevent the failure notification itself.
  if ! python3 "$SCRIPT_DIR/../.github/render-upstream-sync-email.py" render \
    --text "$REPORT_FILE" --output "$HTML_REPORT_FILE"; then
    rm -f "$HTML_REPORT_FILE"
    log 'HTML rendering failed; email composer will regenerate the HTML report'
  fi
}

generate_failure_summary() {
  local prompt_file="$STATE_DIR/$RUN_ID-failure-summary-prompt.txt"
  local analysis_dir="$REPO_DIR"
  [[ -d "$WORKTREE" ]] && analysis_dir="$WORKTREE"
  : > "$FAILURE_SUMMARY_FILE"
  if [[ ! -x "$CODEX_BIN" ]]; then
    printf 'Codex 不可用，无法生成进一步诊断。失败命令：%s\n' "${FAILED_COMMAND:-未知}" > "$FAILURE_SUMMARY_FILE"
    return 0
  fi
  cat > "$prompt_file" <<EOF
只读分析本次 sub2api 双上游自动同步为什么失败，并生成简体中文故障摘要。

完整日志：$LOG_FILE
最终失败检查清单：$VALIDATION_FAILURES_FILE
失败阶段：$CURRENT_STAGE
失败命令：${FAILED_COMMAND:-未记录}
Codex 已执行集中修复：$REPAIR_COUNT/$VALIDATION_REPAIR_ATTEMPTS 次
候选工作树：$WORKTREE

输出必须具体、简洁并包含：
1. 每个最终失败的检查或测试名称；
2. 关键错误信息及最可能的直接原因；
3. 已进行的自动修复轮次和仍未解决的原因；
4. 明确说明代码是否推送（当前值：${PUSHED_COMMIT:-未推送}）。
不要修改文件，不要执行 fetch、commit、push、发布或重启服务。不要输出泛化建议。
EOF
  if ! "$CODEX_BIN" exec --ephemeral --sandbox read-only --color never \
    -C "$analysis_dir" --output-last-message "$FAILURE_SUMMARY_FILE" - < "$prompt_file"; then
    printf 'Codex 故障分析自身执行失败；请查看最终失败检查和完整日志。\n' > "$FAILURE_SUMMARY_FILE"
  fi
  rm -f "$prompt_file"
}

notify_failure() {
  local reason="$1" status="${2:-失败，未推送}" subject='【sub2api】双上游同步失败，代码未推送'
  [[ "$NOTIFIED" == 'true' ]] && return 0
  NOTIFIED=true
  if [[ -n "$PUSHED_COMMIT" ]]; then
    status='发布失败，代码已推送'
    subject='【sub2api】代码已推送，但版本发布失败'
  fi
  generate_failure_summary || true
  write_report "$status" "$reason"
  send_email "$subject" "$REPORT_FILE" || true
}

fail() {
  local reason="$1"
  trap - ERR
  log "BLOCKED: $reason"
  notify_failure "$reason"
  exit 1
}

on_error() {
  local exit_code="$1" line="$2" command="$3"
  trap - ERR
  FAILED_COMMAND="$command"
  log "FAILED at line $line during $CURRENT_STAGE: $command"
  if [[ -n "$PUSHED_COMMIT" ]]; then
    notify_failure "$CURRENT_STAGE 失败（状态码 $exit_code），主分支代码已推送但版本发布未完成"
  else
    notify_failure "$CURRENT_STAGE 失败（状态码 $exit_code），候选代码未推送"
  fi
  exit "$exit_code"
}

cleanup() {
  local exit_code=$?
  trap - ERR EXIT
  if [[ "$WORKTREE_CREATED" == true && "$exit_code" == 0 ]]; then
    git -C "$WORKTREE" merge --abort >/dev/null 2>&1 || true
    git -C "$REPO_DIR" worktree remove --force "$WORKTREE" >/dev/null 2>&1 || true
    git -C "$REPO_DIR" branch -D "$SYNC_BRANCH" >/dev/null 2>&1 || true
  elif [[ "$WORKTREE_CREATED" == true ]]; then
    log "Preserving failed candidate and conflict stages for recovery: $WORKTREE ($SYNC_BRANCH)"
  fi
  exit "$exit_code"
}

trap 'on_error $? $LINENO "$BASH_COMMAND"' ERR
trap cleanup EXIT

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "缺少必需命令：$1"
}

validate_primary_worktree() {
  local branch_name git_dir operation
  [[ -d "$REPO_DIR/.git" ]] || fail "找不到项目仓库：$REPO_DIR"
  branch_name="$(git -C "$REPO_DIR" branch --show-current)"
  [[ "$branch_name" == "$TARGET_BRANCH" ]] || fail "当前分支为 ${branch_name:-detached}，预期为 $TARGET_BRANCH"
  [[ -z "$(git -C "$REPO_DIR" ls-files --unmerged)" ]] || fail '本地存在尚未解决的 Git 冲突，无法自动提交和推送'
  git_dir="$(git -C "$REPO_DIR" rev-parse --git-dir)"
  [[ "$git_dir" = /* ]] || git_dir="$REPO_DIR/$git_dir"
  for operation in MERGE_HEAD CHERRY_PICK_HEAD REVERT_HEAD rebase-merge rebase-apply sequencer; do
    [[ ! -e "$git_dir/$operation" ]] || \
      fail "本地存在未完成的 Git 操作（${operation}），无法自动提交和推送"
  done
}

prepare_primary_worktree() {
  local ahead behind
  CURRENT_STAGE='提交并同步本地修改'
  validate_primary_worktree
  if [[ -n "$(git -C "$REPO_DIR" status --porcelain=v1)" ]]; then
    [[ "$DRY_RUN" != true ]] || fail '试运行检测到本地未提交修改；试运行不会自动提交'
    log 'staging and committing local working tree changes before upstream merge'
    git -C "$REPO_DIR" add --all
    git -C "$REPO_DIR" diff --cached --check
    git -C "$REPO_DIR" commit -m 'chore: save local changes before upstream sync'
    [[ -z "$(git -C "$REPO_DIR" status --porcelain=v1)" ]] || \
      fail '本地提交完成后工作区仍有变化，拒绝继续推送'
  fi

  read -r behind ahead < <(git -C "$REPO_DIR" rev-list --left-right --count "$ORIGIN_REF...HEAD")
  (( behind == 0 || ahead == 0 )) || fail "本地分支与 $ORIGIN_REF 已分叉（领先 ${ahead}，落后 ${behind}）"
  if [[ "$DRY_RUN" == true ]]; then
    log "dry run: skipping local branch synchronization (ahead $ahead, behind $behind)"
    return 0
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
  local phase="${1:-最终双上游合并}" prompt_file="$STATE_DIR/$RUN_ID-review-prompt.txt"
  local attempt attempt_decision review_resolved=false
  [[ "$REVIEW_REPAIR_ATTEMPTS" =~ ^[1-9][0-9]*$ ]] || fail 'SUB2API_REVIEW_REPAIR_ATTEMPTS 必须为正整数'
  for ((attempt = 1; attempt <= REVIEW_REPAIR_ATTEMPTS; attempt++)); do
    attempt_decision="$STATE_DIR/$RUN_ID-review-decision-$attempt.json"
  cat > "$prompt_file" <<EOF
审查并完成当前临时工作树中的${phase}。

仓库：$REPO_DIR
临时工作树：$WORKTREE
本项目基准：$ORIGIN_REF
主上游：$PRIMARY_REF
第二上游：$SECOND_REF
发布累计基准：${RELEASE_BASE_TAG:-未启用发布}（${RELEASE_BASE_COMMIT:-无}）。除本轮合并外，审查该基准到候选的全部累计差异；报告和风险必须覆盖之前合并但尚未发布的更新。

产品决策：
1. $PRIMARY_REF 的所有代码和功能都保留并合并，但不得重新引入已退役的批量生图。
2. $SECOND_REF 的所有代码和功能都无条件保留并合并，例外是共享账号池、批量生图功能及其支持实现。
3. 共享账号池包括其用户/管理员页面、API、数据库迁移、账号调度、权限、计费支持、测试、文档和专属资源；普通账号池和同名但无关功能必须保留。

要求：
- 阅读 AGENTS.md、两上游提交记录、双方差异和当前合并结果。
- 处理所有文本及语义冲突，禁止简单选择 ours/theirs。
- 按 docs/UPSTREAM_EXCLUSIONS.md 删除或恢复仅属于共享账号池、批量生图的实现，保留第二上游的其他全部更新。
- 批量生图的历史 SQL 迁移及校验和兼容记录必须保留；不得删除历史数据或冻结余额。普通生图和异步单图任务必须保留。
- 保留本项目已有功能、权限、计费、数据库兼容性和测试不变量。
- 不要 fetch、commit、push、打 tag、发布或重启服务；直接修改工作树。
- 最终答复用中文列出：上游变化、冲突处理、共享账号池和批量生图排除的文件/代码位置、保留的第二上游功能和剩余风险。
- decision 仅可为 resolved 或 blocked；只有所有冲突已解决且产品规则均满足时才可 resolved。
- risks 只记录合并后仍未消除的具体风险，没有风险时必须返回空数组。
- 沙箱缺少 Go/Bun/Docker、无法监听本地端口、远程工作流尚未启动或工具不可用都不是合并风险；不要因此返回 blocked，也不要将这些环境限制写入 risks。仅记录产品、代码、安全、权限、计费或数据库风险。
- excluded_batch_image_paths 必须列出本轮删除、恢复或明确排除的批量生图路径；没有则返回空数组。
- excluded_shared_account_pool_paths 必须列出本轮删除、恢复或明确排除的共享账号池专属路径；没有则返回空数组。
- 这是第 $attempt/$REVIEW_REPAIR_ATTEMPTS 轮集中冲突修复。若上一轮已修改工作树，必须继续逐文件核对并修复，不要仅返回 blocked。
- 严格遵守 $REPO_DIR/.github/upstream-sync-decision-schema.json 的字段和类型；上一轮输出与校验错误见 $STATE_DIR/$RUN_ID-review-decision-* 和 $LOG_FILE。输出结构错误也要自行修复，只有结构完整且冲突已解决才能通过外层检查。
EOF
    log "running Codex merge review: $phase (attempt $attempt/$REVIEW_REPAIR_ATTEMPTS)"
    if ! "$CODEX_BIN" exec --ephemeral --sandbox workspace-write --color never \
    -C "$WORKTREE" \
    --output-schema "$REPO_DIR/.github/upstream-sync-decision-schema.json" \
    --output-last-message "$attempt_decision" - < "$prompt_file"; then
      log "Codex merge review invocation failed on attempt $attempt"
      continue
    fi
    if python3 "$SCRIPT_DIR/../.github/validate-upstream-sync-decision.py" \
      "$REPO_DIR/.github/upstream-sync-decision-schema.json" "$attempt_decision"
    then
      cp "$attempt_decision" "$REVIEW_DECISION_FILE"
      review_resolved=true
      break
    fi
    log "Codex reported unresolved conflicts on attempt $attempt; preserving worktree for another repair pass"
  done
  rm -f "$prompt_file"
  [[ "$review_resolved" == true ]] || fail "Codex 在 $phase 中经过 $REVIEW_REPAIR_ATTEMPTS 轮仍未能安全解决冲突"
  python3 "$REPO_DIR/.github/render-upstream-sync-review.py" \
    "$REVIEW_DECISION_FILE" > "$REVIEW_SUMMARY_FILE"
}

finish_merge_stage() {
  local ref="$1"
  git -C "$WORKTREE" rev-parse -q --verify MERGE_HEAD >/dev/null || return 0
  if [[ -n "$(git -C "$WORKTREE" diff --name-only --diff-filter=U)" ]]; then
    run_codex_merge_review "解决 $ref 的文本与语义冲突"
  fi
  git -C "$WORKTREE" add --all
  [[ -z "$(git -C "$WORKTREE" diff --name-only --diff-filter=U)" ]] || fail "$ref 经过 Codex 审查后仍有冲突"
  assert_no_conflict_markers || fail "$ref 经过 Codex 审查后仍有冲突标记"
  git -C "$WORKTREE" diff --cached --check
  git -C "$WORKTREE" commit --no-edit
}

record_check() {
  local label="$1"
  shift
  local safe_label check_log rc
  safe_label="$(printf '%s' "$label" | tr -cs '[:alnum:]_-' '_')"
  # Non-ASCII labels can collapse to the same safe_label. Keep every check and
  # validation pass in its own file so later checks cannot overwrite failures.
  check_log="$(mktemp "$STATE_DIR/$RUN_ID-check-${safe_label}.log.XXXXXX")"
  CURRENT_STAGE="全量验证：$label"
  log "validation check: $label"
  set +e
  "$@" > >(tee "$check_log") 2>&1
  rc=$?
  set -e
  if (( rc != 0 )); then
    printf '%s（状态码 %d，日志：%s）\n' "$label" "$rc" "$check_log" >> "$VALIDATION_FAILURES_FILE"
    log "validation failed: $label (exit $rc)"
  else
    log "validation passed: $label"
  fi
  return 0
}

docker_go() {
  docker run --rm \
    -v "$WORKTREE:/src" \
    -v sub2api-sync-go-mod:/go/pkg/mod \
    -v sub2api-sync-go-build:/root/.cache/go-build \
    -w /src/backend "$GO_IMAGE" "$@"
}

check_backend_security() {
  docker run --rm \
    -v "$WORKTREE:/src" \
    -v "$TOOLS_DIR:/tools" \
    -v sub2api-sync-go-mod:/go/pkg/mod \
    -v sub2api-sync-go-build:/root/.cache/go-build \
    -w /src/backend "$GO_IMAGE" /bin/bash -ec \
    'test -x /tools/govulncheck || GOBIN=/tools go install golang.org/x/vuln/cmd/govulncheck@latest; /tools/govulncheck ./...'
}

check_backend_lint() {
  docker run --rm \
    -v "$WORKTREE:/src" \
    -v "$TOOLS_DIR:/tools" \
    -v sub2api-sync-go-mod:/go/pkg/mod \
    -v sub2api-sync-go-build:/root/.cache/go-build \
    -w /src/backend "$GO_IMAGE" /bin/bash -ec \
    'test -x /tools/golangci-lint || GOBIN=/tools go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.0; /tools/golangci-lint run --timeout=30m ./...'
}

check_frontend_audit() {
  (cd "$WORKTREE/frontend" && frontend_pnpm audit --prod --audit-level=high --json > audit.json || true)
  python3 "$WORKTREE/tools/check_pnpm_audit_exceptions.py" \
    --audit "$WORKTREE/frontend/audit.json" \
    --exceptions "$WORKTREE/.github/audit-exceptions.yml"
}

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

run_validation_pass() {
  local pass="$1"
  : > "$VALIDATION_FAILURES_FILE"
  VALIDATION_RESULT="第 $pass 轮全量验证进行中"
  log "starting complete validation pass $pass"
  record_check '上游功能排除检查' python3 "$WORKTREE/.github/check-upstream-exclusions.py" "$WORKTREE"
  record_check '上游排除规则及报告测试' python3 -m unittest discover -s "$WORKTREE/.github" -p 'test_upstream*.py'
  record_check '后端普通测试' docker_go go test ./...
  record_check '后端单元测试' docker_go go test -tags=unit ./...
  record_check '后端集成测试' docker_go go test -tags=integration ./...
  record_check '后端静态检查' docker_go go vet ./...
  record_check 'golangci-lint' check_backend_lint
  record_check '后端漏洞检查' check_backend_security
  record_check '后端生产构建' docker_go /bin/bash -ec 'CGO_ENABLED=0 go build -trimpath -o /tmp/sub2api-server ./cmd/server'
  record_check '前端依赖锁定安装' frontend_pnpm --dir "$WORKTREE/frontend" install --frozen-lockfile
  record_check '前端 ESLint' frontend_pnpm --dir "$WORKTREE/frontend" run lint:check
  record_check '前端类型检查' frontend_pnpm --dir "$WORKTREE/frontend" run typecheck
  record_check '前端完整测试' frontend_pnpm --dir "$WORKTREE/frontend" run test:run
  record_check '前端生产构建及 i18n' frontend_pnpm --dir "$WORKTREE/frontend" run build
  record_check '前端依赖安全审计' check_frontend_audit
  record_check '部署脚本验证' check_deployment_scripts
  if [[ -s "$VALIDATION_FAILURES_FILE" ]]; then
    VALIDATION_RESULT="第 $pass 轮失败（$(wc -l < "$VALIDATION_FAILURES_FILE") 项）"
    return 1
  fi
  VALIDATION_RESULT="第 $pass 轮全部通过"
  return 0
}

run_codex_validation_repair() {
  local attempt="$1"
  local prompt_file="$STATE_DIR/$RUN_ID-validation-repair-$attempt.txt"
  cat > "$prompt_file" <<EOF
对 sub2api 候选双上游合并执行一次集中修复。本轮必须处理失败清单中的全部问题，不得只修第一个错误。

工作目录：$WORKTREE
完整日志：$LOG_FILE
本轮全量失败清单：$VALIDATION_FAILURES_FILE
这是第 $attempt/$VALIDATION_REPAIR_ATTEMPTS 次集中修复。
主上游：$PRIMARY_REF（保留全部非退役功能）
第二上游：$SECOND_REF（除共享账号池和批量生图外全部保留）

要求：
1. 先读取失败清单中每个检查的独立日志，归纳共同原因后一次性修复全部可修问题。
2. 按 docs/UPSTREAM_EXCLUSIONS.md 保留两个上游的非排除功能；不得重新引入共享账号池或批量生图；保留历史数据库迁移和校验和兼容记录。
3. 保留本项目权限、计费、数据库兼容、账号调度、日志隐私和测试不变量。
4. 可运行针对性检查辅助定位，但外层脚本会在修复结束后重新执行整套验证。
5. 不要 fetch、commit、push、打 tag、发布、重启服务或访问密钥。直接修改工作树并检查 diff。
6. 如果某项无法安全修复，在最终答复中写明具体测试名、错误和原因；不要仅给泛化建议。
EOF
  log "asking Codex to repair all validation failures (attempt $attempt/$VALIDATION_REPAIR_ATTEMPTS)"
  if ! "$CODEX_BIN" exec --ephemeral --sandbox workspace-write --color never -C "$WORKTREE" - < "$prompt_file"; then
    log "Codex repair invocation failed on attempt $attempt"
  fi
  rm -f "$prompt_file"
  git -C "$WORKTREE" add --all || return 1
  git -C "$WORKTREE" diff --cached --check || return 1
  if [[ -n "$(git -C "$WORKTREE" diff --cached --name-only)" ]]; then
    if [[ -n "$PUSHED_COMMIT" ]] && git -C "$WORKTREE" merge-base --is-ancestor HEAD "$PUSHED_COMMIT"; then
      git -C "$WORKTREE" commit -m 'fix: repair remote upstream sync validation' >/dev/null || return 1
    else
      git -C "$WORKTREE" commit --amend --no-edit >/dev/null || return 1
    fi
    CANDIDATE_COMMIT="$(git -C "$WORKTREE" rev-parse HEAD)"
  else
    log "Codex made no candidate changes on repair attempt $attempt"
  fi
}

run_validation_with_repairs() {
  local attempt
  VALIDATION_SUCCEEDED=false
  if [[ "$VALIDATE" != true ]]; then
    VALIDATION_RESULT='由配置跳过'
    VALIDATION_SUCCEEDED=true
    return 0
  fi
  if run_validation_pass 1; then
    VALIDATION_SUCCEEDED=true
    return 0
  fi
  for ((attempt = 1; attempt <= VALIDATION_REPAIR_ATTEMPTS; attempt++)); do
    REPAIR_COUNT="$attempt"
    run_codex_validation_repair "$attempt"
    if run_validation_pass "$((attempt + 1))"; then
      VALIDATION_SUCCEEDED=true
      return 0
    fi
  done
  return 0
}

create_final_merge_commit() {
  local tree commit
  local -a parents
  git -C "$WORKTREE" add --all
  tree="$(git -C "$WORKTREE" write-tree)"
  parents=(-p "$ORIGIN_HEAD")
  if [[ "$PRIMARY_HEAD" != "$ORIGIN_HEAD" ]]; then
    parents+=(-p "$PRIMARY_HEAD")
  fi
  if [[ "$SECOND_HEAD" != "$ORIGIN_HEAD" && "$SECOND_HEAD" != "$PRIMARY_HEAD" ]]; then
    parents+=(-p "$SECOND_HEAD")
  fi
  commit="$(printf '%s\n' 'merge: synchronize primary and custom upstreams' | \
    git -C "$WORKTREE" commit-tree "$tree" "${parents[@]}")"
  git -C "$WORKTREE" reset --hard "$commit" >/dev/null
  CANDIDATE_COMMIT="$commit"
}

evaluate_release_eligibility() {
  local upstream_commits changed_files diff_lines review_decision risk_count
  local merged_behavior_count ignored_risk_count release_check

  CURRENT_STAGE='评估版本发布条件'
  RELEASE_STATE='评估中'
  if [[ "$RELEASE_ENABLED" != true ]]; then
    RELEASE_STATE='未发布'
    RELEASE_REASON='发布功能已由配置关闭'
    return 0
  fi

  local metrics
  metrics="$(python3 "$SCRIPT_DIR/../.github/upstream-release-window.py" measure \
    --repo "$WORKTREE" --baseline "$RELEASE_BASE_COMMIT" --candidate "$CANDIDATE_COMMIT" \
    --upstream "$PRIMARY_HEAD" --upstream "$SECOND_HEAD")" || fail '累计发布统计失败'
  read -r upstream_commits changed_files diff_lines <<< "$metrics"
  read -r review_decision risk_count merged_behavior_count ignored_risk_count < <(
    python3 - "$REVIEW_DECISION_FILE" "$RELEASE_IGNORE_ENVIRONMENT_RISKS" <<'PY'
import json
import re
import sys

with open(sys.argv[1], encoding="utf-8") as source:
    decision = json.load(source)
risks = decision.get("risks", []) if isinstance(decision.get("risks", []), list) else []
ignore_environment = sys.argv[2].lower() == "true"
environment_only = re.compile(
    r"沙箱|sandbox|operation not permitted|监听本地端口|httptest|miniredis|plutil|"
    r"缺少 (?:go|bun|docker)|工具不可用|远程工作流尚未启动",
    re.IGNORECASE,
)
blocking_risks = [risk for risk in risks if not ignore_environment or not environment_only.search(str(risk))]
print(
    decision.get("decision", ""),
    len(blocking_risks),
    len(decision.get("merged_behavior", [])),
    len(risks) - len(blocking_risks),
)
PY
  )

  release_check="自 ${RELEASE_BASE_TAG} 累计：提交=${upstream_commits}/${RELEASE_MIN_UPSTREAM_COMMITS} 文件=${changed_files}/${RELEASE_MIN_CHANGED_FILES} 行=${diff_lines}/${RELEASE_MIN_DIFF_LINES} 审查=${review_decision} 阻断风险=${risk_count} 忽略环境风险=${ignored_risk_count}"
  log "release eligibility: $release_check"
  if [[ "$review_decision" != resolved ]]; then
    RELEASE_STATE='不发布'
    RELEASE_REASON="自动审查未确认更新可安全发布（$release_check）"
    return 0
  fi
  if [[ "$RELEASE_REQUIRE_NO_RISKS" == true && "$risk_count" != 0 ]]; then
    RELEASE_STATE='不发布'
    RELEASE_REASON="自动审查仍有未消除风险（$release_check）"
    return 0
  fi
  if (( merged_behavior_count == 0 )); then
    RELEASE_STATE='不发布'
    RELEASE_REASON="审查报告没有确认合并后的实际行为（$release_check）"
    return 0
  fi
  if (( upstream_commits < RELEASE_MIN_UPSTREAM_COMMITS ||
    changed_files < RELEASE_MIN_CHANGED_FILES ||
    diff_lines < RELEASE_MIN_DIFF_LINES )); then
    RELEASE_STATE='不发布'
    RELEASE_REASON="上游更新量未达到发布阈值（$release_check）"
    return 0
  fi

  RELEASE_STATE='待发布'
  RELEASE_REASON="更新量和兼容审查均达到发布条件（$release_check）"
}

next_release_tag() {
  local offset candidate remote_tag

  git -C "$REPO_DIR" fetch --tags "$ORIGIN_REMOTE" >/dev/null 2>&1 || true
  for offset in {0..365}; do
    candidate="v$(TZ=Asia/Shanghai date -d "+$offset day" +'%Y.%-m.%-d')"
    if git -C "$REPO_DIR" show-ref --verify --quiet "refs/tags/$candidate"; then
      continue
    fi
    remote_tag="$(git -C "$REPO_DIR" ls-remote "$ORIGIN_REMOTE" "refs/tags/$candidate" | awk 'NR == 1 {print $1}')"
    if [[ -z "$remote_tag" ]]; then
      printf '%s' "$candidate"
      return 0
    fi
  done
  return 1
}

prepare_release_notes() {
  local tag="$1"
  CURRENT_STAGE='生成版本说明'
  python3 "$REPO_DIR/.github/render-upstream-sync-review.py" \
    --release-tag "$tag" "$REVIEW_DECISION_FILE" > "$RELEASE_NOTES_FILE"
  python3 "$SCRIPT_DIR/../.github/upstream-release-window.py" notes --repo "$WORKTREE" \
    --baseline "$RELEASE_BASE_COMMIT" --candidate "$CANDIDATE_COMMIT" >> "$RELEASE_NOTES_FILE"
  [[ -s "$RELEASE_NOTES_FILE" ]] || fail '版本说明为空，停止发布'
}

publish_release() {
  local tag="$1" release_commit attempt remote_tag
  CURRENT_STAGE='推送版本标签'
  RELEASE_STATE='发布中'
  release_commit="$(git -C "$WORKTREE" rev-parse HEAD)"
  for attempt in 1 2 3; do
    remote_tag="$(git -C "$REPO_DIR" ls-remote "$ORIGIN_REMOTE" "refs/tags/$tag" "refs/tags/$tag^{}" | awk 'NR == 1 {print $1}')"
    if [[ -n "$remote_tag" ]]; then
      log "release tag $tag was claimed concurrently; selecting the next available date tag"
      tag="$(next_release_tag)" || return 1
      prepare_release_notes "$tag"
      continue
    fi
    git -C "$WORKTREE" tag -a "$tag" -F "$RELEASE_NOTES_FILE" "$release_commit"
    if git -C "$WORKTREE" push "$ORIGIN_REMOTE" "refs/tags/$tag"; then
      RELEASE_TAG="$tag"
      RELEASE_STATE='已触发发布'
      RELEASE_REASON="版本标签已推送，GitHub 发布工作流已触发（提交 $release_commit）"
      return 0
    fi
    git -C "$WORKTREE" tag -d "$tag" >/dev/null 2>&1 || true
    git -C "$REPO_DIR" fetch --tags "$ORIGIN_REMOTE" >/dev/null 2>&1 || true
    tag="$(next_release_tag)" || return 1
    prepare_release_notes "$tag"
    log "release tag push failed; retrying with $tag (attempt $((attempt + 1))/3)"
  done
  return 1
}

persist_pending_release() {
  local tag="$1" commit="$2"
  [[ -s "$RELEASE_NOTES_FILE" ]] || fail '版本说明为空，无法记录待发布状态'
  cp "$RELEASE_NOTES_FILE" "$PENDING_RELEASE_NOTES_FILE"
  chmod 600 "$PENDING_RELEASE_NOTES_FILE"
  {
    printf 'tag=%s\n' "$tag"
    printf 'commit=%s\n' "$commit"
  } > "$PENDING_RELEASE_FILE"
  chmod 600 "$PENDING_RELEASE_FILE"
}

clear_pending_release() {
  rm -f "$PENDING_RELEASE_FILE" "$PENDING_RELEASE_NOTES_FILE"
}

recover_pending_release() {
  local tag='' commit='' key value remote_tag
  local tag_pattern='^v[0-9]{4}\.[0-9]{1,2}\.[0-9]{1,2}(-[0-9]+)?$'
  [[ -s "$PENDING_RELEASE_FILE" ]] || return 1
  while IFS='=' read -r key value; do
    case "$key" in
      tag) tag="$value" ;;
      commit) commit="$value" ;;
    esac
  done < "$PENDING_RELEASE_FILE"
  [[ "$tag" =~ $tag_pattern ]] || fail '待发布状态中的版本标签无效'
  [[ "$commit" =~ ^[0-9a-f]{40}$ ]] || fail '待发布状态中的提交无效'
  if git -C "$REPO_DIR" ls-remote --exit-code "$ORIGIN_REMOTE" "refs/tags/$tag" >/dev/null 2>&1; then
    log "pending release $tag already has a remote tag; clearing stale recovery state"
    clear_pending_release
    return 1
  fi
  if ! git -C "$REPO_DIR" merge-base --is-ancestor "$commit" "$ORIGIN_REF"; then
    log "pending release $tag references a commit not present on $ORIGIN_REF; clearing stale recovery state"
    clear_pending_release
    return 1
  fi
  [[ -s "$PENDING_RELEASE_NOTES_FILE" ]] || fail '待发布版本说明丢失，拒绝盲目创建版本标签'
  # A repair can be pushed after the original candidate failed CI. Never retry
  # or tag that known-broken SHA when the target branch already contains a fix.
  commit="$(git -C "$REPO_DIR" rev-parse "$ORIGIN_REF")"
  cp "$PENDING_RELEASE_NOTES_FILE" "$RELEASE_NOTES_FILE"
  workflow_trigger_state 'Release' "$commit" >/dev/null || return 1
  CURRENT_STAGE='恢复已验证但未完成的 Release'
  RELEASE_TAG="$tag"
  RELEASE_STATE='发布中'
  RELEASE_REASON="检测到已推送提交 $commit 的版本标签 $tag 尚未完成，恢复 Release 流程"
  PUSHED_COMMIT="$commit"
  wait_for_remote_workflows "$commit"
  commit="$PUSHED_COMMIT"
  persist_pending_release "$tag" "$commit"
  remote_tag="$(git -C "$REPO_DIR" ls-remote "$ORIGIN_REMOTE" "refs/tags/$tag" | awk 'NR == 1 {print $1}')"
  [[ -z "$remote_tag" ]] || fail "待发布标签已被远程占用：$tag"
  git -C "$REPO_DIR" tag -a "$tag" -F "$PENDING_RELEASE_NOTES_FILE" "$commit" || fail "待发布标签创建失败：$tag"
  if ! git -C "$REPO_DIR" push "$ORIGIN_REMOTE" "refs/tags/$tag"; then
    git -C "$REPO_DIR" tag -d "$tag" >/dev/null 2>&1 || true
    fail "恢复版本标签推送失败：$tag"
  fi
  wait_for_release_workflow "$tag" "$commit"
  clear_pending_release
  write_report '成功' "已恢复已验证提交的 Release，版本标签 $tag 发布完成"
  send_email '【sub2api】中断 Release 恢复成功报告' "$REPORT_FILE" || \
    log "success email delivery failed; report retained at $REPORT_FILE"
  return 0
}

main() {
  local baseline_info no_upstream_updates=false
  require_command git
  require_command docker
  require_command "$CODEX_BIN"
  require_command "$COREPACK_BIN"
  require_command python3
  require_command gh
  require_command /usr/bin/curl
  require_command /usr/bin/base64
  [[ "$VALIDATION_REPAIR_ATTEMPTS" =~ ^[0-9]+$ ]] || fail 'SUB2API_VALIDATION_REPAIR_ATTEMPTS 必须为非负整数'
  [[ "$REVIEW_REPAIR_ATTEMPTS" =~ ^[1-9][0-9]*$ ]] || fail 'SUB2API_REVIEW_REPAIR_ATTEMPTS 必须为正整数'
  [[ "$REMOTE_WORKFLOW_TIMEOUT_MINUTES" =~ ^[1-9][0-9]*$ ]] || fail 'SUB2API_REMOTE_WORKFLOW_TIMEOUT_MINUTES 必须为正整数'
  [[ "$REMOTE_WORKFLOW_POLL_SECONDS" =~ ^[1-9][0-9]*$ ]] || fail 'SUB2API_REMOTE_WORKFLOW_POLL_SECONDS 必须为正整数'
  [[ "$REMOTE_WORKFLOW_RETRY_ATTEMPTS" =~ ^[0-9]+$ ]] || fail 'SUB2API_REMOTE_WORKFLOW_RETRY_ATTEMPTS 必须为非负整数'
  [[ "$RELEASE_MIN_UPSTREAM_COMMITS" =~ ^[0-9]+$ ]] || fail 'SUB2API_RELEASE_MIN_UPSTREAM_COMMITS 必须为非负整数'
  [[ "$RELEASE_MIN_CHANGED_FILES" =~ ^[0-9]+$ ]] || fail 'SUB2API_RELEASE_MIN_CHANGED_FILES 必须为非负整数'
  [[ "$RELEASE_MIN_DIFF_LINES" =~ ^[0-9]+$ ]] || fail 'SUB2API_RELEASE_MIN_DIFF_LINES 必须为非负整数'
  find "$LOG_DIR" -type f -mtime +30 -delete
  CURRENT_STAGE='检查主工作区'
  validate_primary_worktree

  CURRENT_STAGE='获取三个远程仓库'
  log "fetching $ORIGIN_REMOTE, $PRIMARY_REMOTE and $SECOND_REMOTE"
  git -C "$REPO_DIR" fetch --prune "$ORIGIN_REMOTE"
  git -C "$REPO_DIR" fetch --prune "$PRIMARY_REMOTE"
  git -C "$REPO_DIR" fetch --prune "$SECOND_REMOTE"
  prepare_primary_worktree
  ORIGIN_HEAD="$(git -C "$REPO_DIR" rev-parse "$ORIGIN_REF")"
  PRIMARY_HEAD="$(git -C "$REPO_DIR" rev-parse "$PRIMARY_REF")"
  SECOND_HEAD="$(git -C "$REPO_DIR" rev-parse "$SECOND_REF")"

  if recover_pending_release; then
    log "recovered pending release $RELEASE_TAG successfully"
    git -C "$REPO_DIR" fetch "$ORIGIN_REMOTE" "$TARGET_BRANCH"
    if [[ "$(git -C "$REPO_DIR" branch --show-current)" == "$TARGET_BRANCH" ]] && \
      [[ -z "$(git -C "$REPO_DIR" status --porcelain=v1)" ]]; then
      git -C "$REPO_DIR" merge --ff-only "$ORIGIN_REF"
    fi
    exit 0
  fi

  if [[ "$RELEASE_ENABLED" == true ]]; then
    baseline_info="$(python3 "$SCRIPT_DIR/../.github/upstream-release-window.py" baseline \
      --repo "$REPO_DIR" --remote "$ORIGIN_REMOTE" --head "$ORIGIN_HEAD")" || fail '无法确认上次成功发布版本，保留累计更新，下次重试'
    read -r RELEASE_BASE_TAG RELEASE_BASE_COMMIT <<< "$baseline_info"
    log "cumulative release baseline: $RELEASE_BASE_TAG ($RELEASE_BASE_COMMIT)"
  fi

  if git -C "$REPO_DIR" merge-base --is-ancestor "$PRIMARY_HEAD" "$ORIGIN_HEAD" && \
    git -C "$REPO_DIR" merge-base --is-ancestor "$SECOND_HEAD" "$ORIGIN_HEAD"; then
    no_upstream_updates=true
    if [[ "$RELEASE_ENABLED" != true || "$RELEASE_BASE_COMMIT" == "$ORIGIN_HEAD" ]]; then
      log 'no pending upstream or cumulative release updates'
      exit 0
    fi
  fi

  CURRENT_STAGE='生成双上游候选合并'
  git -C "$REPO_DIR" worktree add --detach "$WORKTREE" "$ORIGIN_HEAD"
  WORKTREE_CREATED=true
  git -C "$WORKTREE" switch -c "$SYNC_BRANCH"

  log "merging all primary upstream changes from $PRIMARY_REF"
  if ! git -C "$WORKTREE" merge --no-ff --no-commit "$PRIMARY_HEAD"; then
    git -C "$WORKTREE" rev-parse -q --verify MERGE_HEAD >/dev/null || fail "无法开始合并 $PRIMARY_REF"
  fi
  finish_merge_stage "$PRIMARY_REF"

  log "merging all second upstream changes from $SECOND_REF"
  if ! git -C "$WORKTREE" merge --no-ff --no-commit "$SECOND_HEAD"; then
    git -C "$WORKTREE" rev-parse -q --verify MERGE_HEAD >/dev/null || fail "无法开始合并 $SECOND_REF"
  fi
  finish_merge_stage "$SECOND_REF"

  CURRENT_STAGE='Codex 最终双上游兼容审查'
  run_codex_merge_review '最终双上游兼容审查'
  git -C "$WORKTREE" add --all
  [[ -z "$(git -C "$WORKTREE" diff --name-only --diff-filter=U)" ]] || fail 'Codex 最终审查后仍有未解决冲突'
  assert_no_conflict_markers || fail 'Codex 最终审查后仍有冲突标记'
  git -C "$WORKTREE" diff --cached --check
  if git -C "$WORKTREE" ls-files | grep -Eiq '(^|/)(shared.?pool|shared_account|shared-account|SharedPool|add-shared-account-pool)'; then
    fail '候选合并中仍有共享账号池专属路径'
  fi
  python3 "$WORKTREE/.github/check-upstream-exclusions.py" "$WORKTREE" || fail '候选合并重新引入了已排除功能'
  if [[ "$DRY_RUN" == true ]]; then
    log 'dry run completed; candidate was not committed, validated, or pushed'
    exit 0
  fi

  CURRENT_STAGE='生成保留双上游祖先关系的候选提交'
  create_final_merge_commit
  evaluate_release_eligibility
  if [[ "$RELEASE_STATE" == '待发布' ]]; then
    RELEASE_TAG="$(next_release_tag)" || fail '无法生成下一个可用的日期版本标签，停止发布'
    prepare_release_notes "$RELEASE_TAG"
    log "release candidate $RELEASE_TAG prepared"
  else
    log "release skipped: $RELEASE_REASON"
    if [[ "$no_upstream_updates" == true ]]; then
      write_report '没有上游更新' "$RELEASE_REASON；累计更新保留至下次评估"
      exit 0
    fi
  fi
  CURRENT_STAGE='整套验证与 Codex 集中修复'
  run_validation_with_repairs
  if [[ "$VALIDATION_SUCCEEDED" != true ]]; then
    fail "完成 $REPAIR_COUNT 次 Codex 集中修复后，整套验证仍有失败"
  fi
  [[ -z "$(git -C "$WORKTREE" status --porcelain=v1)" ]] || fail '验证修改了受版本控制文件，拒绝推送'
  if [[ "$RELEASE_STATE" == '待发布' ]]; then
    persist_pending_release "$RELEASE_TAG" "$CANDIDATE_COMMIT"
  fi

  CURRENT_STAGE='推送已验证候选提交'
  log "pushing validated candidate $CANDIDATE_COMMIT to $ORIGIN_REF"
  git -C "$WORKTREE" push "$ORIGIN_REMOTE" "HEAD:$TARGET_BRANCH"
  PUSHED_COMMIT="$(git -C "$WORKTREE" rev-parse HEAD)"
  wait_for_remote_workflows "$PUSHED_COMMIT"
  if [[ "$RELEASE_STATE" == '待发布' ]]; then
    publish_release "$RELEASE_TAG"
    wait_for_release_workflow "$RELEASE_TAG" "$PUSHED_COMMIT"
    clear_pending_release
  fi
  git -C "$REPO_DIR" fetch --prune "$ORIGIN_REMOTE" "$TARGET_BRANCH" --tags
  if [[ -z "$(git -C "$REPO_DIR" status --porcelain=v1)" ]] && \
    git -C "$REPO_DIR" merge-base --is-ancestor "HEAD" "$ORIGIN_REF"; then
    git -C "$REPO_DIR" merge --ff-only "$ORIGIN_REF"
  else
    log 'candidate pushed; primary worktree was left unchanged because it has local changes'
  fi
  CURRENT_STAGE='完成'
  write_report '成功' "两个上游已按规则合并，全部检查通过，代码已推送；$RELEASE_REASON"
  send_email '【sub2api】双上游代码合并成功报告' "$REPORT_FILE" || \
    log "success email delivery failed; report retained at $REPORT_FILE"
  log "upstream sync completed: $PUSHED_COMMIT"
}

if [[ "${1:-}" == '--test-email' ]]; then
  require_command docker
  require_command /usr/bin/curl
  require_command /usr/bin/base64
  CURRENT_STAGE='邮件投递测试'
  VALIDATION_RESULT='未执行（仅测试邮件）'
  write_report '邮件测试' 'SMTP 配置和中文报告投递测试，不包含代码合并'
  send_email '【sub2api】双上游同步邮件测试' "$REPORT_FILE"
  log 'email test completed successfully'
  exit 0
fi

main "$@"
