"""Exercise release recovery against temporary repositories and fake GitHub runs."""
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(os.environ.get('SYNC_TEST_SCRIPT', Path(__file__).resolve().parents[1] / 'scripts/upstream-sync.sh'))


def functions(*names):
    shell = SCRIPT.read_text()
    return '\n'.join(name + '() {' + shell.split(name + '() {', 1)[1].split('\n}\n', 1)[0] + '\n}' for name in names)


class ReleaseRecoveryTest(unittest.TestCase):
    def test_rerun_waits_for_a_new_attempt_before_using_completed_result(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            gh = root / 'gh'
            gh.write_text('''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
counter = Path(os.environ['STATE_DIR']) / 'calls'
n = int(counter.read_text()) if counter.exists() else 0
if sys.argv[1:3] == ['run', 'list']:
    n += 1
    counter.write_text(str(n))
    print(json.dumps([{'databaseId': 42, 'headSha': 'abc', 'status': 'completed',
                      'conclusion': 'failure'}]))
elif sys.argv[1:3] == ['run', 'rerun']:
    (counter.parent / 'reruns').open('a').write('rerun\\n')
elif sys.argv[1] == 'api':
    if sys.argv[-1] == '.run_attempt':
        print(1)
    else:
        print('1 completed failure' if n < 3 else '2 completed success')
''')
            gh.chmod(0o755)
            command = '''set -eu
log() { :; }
workflow_trigger_state() { return 0; }
git() { echo git@github.com:owner/project.git; }
sleep() { :; }
''' + functions('wait_for_remote_workflow') + '\nwait_for_remote_workflow Release abc Release v1'
            result = subprocess.run(['bash', '-c', command], text=True, capture_output=True,
                env=dict(os.environ, PATH=tmp + os.pathsep + os.environ['PATH'], STATE_DIR=tmp,
                         REPO_DIR=tmp, ORIGIN_REMOTE='origin', NODE_BIN='node',
                         REMOTE_WORKFLOW_TIMEOUT_MINUTES='1', REMOTE_WORKFLOW_POLL_SECONDS='1',
                         REMOTE_WORKFLOW_RETRY_ATTEMPTS='1'))
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual((root / 'calls').read_text(), '3')
            self.assertEqual((root / 'reruns').read_text(), 'rerun\n')

    def test_published_requires_stable_release_with_assets_and_api_errors_stay_distinct(self):
        with tempfile.TemporaryDirectory() as tmp:
            command = '''set -u
 git() { echo git@github.com:owner/project.git; }
 gh() { printf '%s' "$RESPONSE"; printf '%s' "$API_ERROR" >&2; return "$API_STATUS"; }
''' + functions('release_is_published') + '\nrelease_is_published v1'
            base = dict(draft=False, prerelease=False, published_at='2026-09-25', assets=[{'size': 100}])
            cases = [(base, 0, '', 0), (dict(base, draft=True), 0, '', 1),
                     (dict(base, prerelease=True), 0, '', 1), (dict(base, assets=[]), 0, '', 1),
                     ({}, 1, 'gh: Not Found (HTTP 404)', 1), ({}, 1, 'HTTP 503', 2)]
            for response, status, error, expected in cases:
                with self.subTest(response=response, error=error):
                    result = subprocess.run(['bash', '-c', command], text=True, capture_output=True,
                        env=dict(os.environ, REPO_DIR=tmp, STATE_DIR=tmp, RUN_ID='test', ORIGIN_REMOTE='origin',
                                 RESPONSE=json.dumps(response), API_ERROR=error, API_STATUS=str(status)))
                    self.assertEqual(result.returncode, expected, result.stderr)

    def test_published_assets_do_not_hide_incomplete_release_workflows(self):
        with tempfile.TemporaryDirectory() as tmp:
            command = """set -u
 git() { echo git@github.com:owner/project.git; }
 release_is_published() { return 0; }
 workflow_trigger_state() { return 0; }
 gh() { printf '%s' "$WORKFLOW_RESPONSE"; return "$API_STATUS"; }
""" + functions('release_is_complete') + '\nrelease_is_complete v1'
            for status, conclusion, api_status, expected in [
                ('completed', 'success', 0, 0), ('completed', 'failure', 0, 1),
                ('in_progress', '', 0, 1), ('completed', 'success', 1, 2),
            ]:
                result = subprocess.run(['bash', '-c', command], text=True, capture_output=True,
                    env=dict(os.environ, REPO_DIR=tmp, ORIGIN_REMOTE='origin', TARGET_BRANCH='main',
                        WORKFLOW_RESPONSE=json.dumps([dict(status=status, conclusion=conclusion)]),
                        API_STATUS=str(api_status)))
                self.assertEqual(result.returncode, expected, result.stderr)

    def test_existing_tag_recovery_retains_pending_state_until_success(self):
        if 'sub2api' not in str(SCRIPT):
            self.skipTest('new-api selects pending notes instead of a state file')
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'pending').write_text('tag=v2026.9.25\ncommit=' + 'a' * 40 + '\n')
            (root / 'pending-notes').write_text('Notes\n')
            command = '''set -eu
log() { :; }
fail() { echo "$*" >&2; exit 9; }
release_is_complete() { return 1; }
workflow_trigger_state() { return 0; }
git() {
  case "$*" in
    *ls-remote*) echo 'a refs/tags/v2026.9.25' ;;
    *rev-parse*) printf '%040d\\n' 1 ;;
    *fetch*|*merge-base*) return 0 ;;
    *) echo 'unexpected git operation' >&2; exit 8 ;;
  esac
}
wait_for_release_workflow() {
  [[ -s "$PENDING_RELEASE_FILE" ]] || fail 'pending state cleared too early'
  [[ "$MODE" == success ]] || fail 'release failed'
}
write_report() { :; }
send_email() { :; }
''' + functions('recover_pending_release', 'clear_pending_release') + '\nrecover_pending_release'
            env = dict(os.environ, STATE_DIR=tmp, RUN_ID='test', REPO_DIR=tmp, ORIGIN_REMOTE='origin',
                ORIGIN_REF='origin/main', TARGET_BRANCH='main', RELEASE_ENABLED='true',
                PENDING_RELEASE_FILE=tmp + '/pending', PENDING_RELEASE_NOTES_FILE=tmp + '/pending-notes',
                RELEASE_NOTES_FILE=tmp + '/notes', REPORT_FILE=tmp + '/report')
            result = subprocess.run(['bash', '-c', command], env=dict(env, MODE='failure'), text=True, capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertTrue((root / 'pending').exists())
            result = subprocess.run(['bash', '-c', command], env=dict(env, MODE='success'), text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertFalse((root / 'pending').exists())

    def test_release_failure_collects_logs_repairs_revalidates_and_uses_new_tag(self):
        self.exercise_release_repair('workflow')

    def test_successful_workflow_with_missing_assets_also_enters_repair(self):
        self.exercise_release_repair('assets')

    def exercise_release_repair(self, mode):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = root / 'repo'
            remote = root / 'remote.git'
            state = root / 'state'
            state.mkdir()
            env = dict(os.environ, GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_NOSYSTEM='1',
                GIT_AUTHOR_NAME='Test', GIT_AUTHOR_EMAIL='test@example.test',
                GIT_COMMITTER_NAME='Test', GIT_COMMITTER_EMAIL='test@example.test')
            def git(*args, cwd=repo):
                return subprocess.check_output(['git', '-C', str(cwd), *args], env=env, stderr=subprocess.DEVNULL).decode().strip()
            git('init', '--initial-branch=main', str(repo), cwd=root)
            notes = repo / '.github/release-notes'
            notes.mkdir(parents=True)
            (notes / 'v2026.09.25.md').write_text('Release v2026.09.25\n## 自上次正式发布以来的累计提交\n- update\n')
            git('add', '.')
            git('commit', '-m', 'original release')
            initial = git('rev-parse', 'HEAD')
            git('tag', 'v2026.09.25')
            git('clone', '--bare', str(repo), str(remote), cwd=root)
            git('remote', 'add', 'origin', str(remote))
            git('fetch', 'origin')
            gh = root / 'gh'
            gh.write_text('''#!/usr/bin/env python3
import json,sys
if sys.argv[1] == 'run':
    print(json.dumps({'jobs': [{'databaseId': 1, 'name': 'release build', 'conclusion': 'failure'}]}))
else:
    print('release metadata invalid')
''')
            gh.chmod(0o755)
            codex = root / 'codex'
            codex.write_text('''#!/usr/bin/env python3
import os
from pathlib import Path
state = Path(os.environ['STATE_DIR'])
assert 'Release' in (state / 'failures').read_text()
assert any('release metadata invalid' in p.read_text() for p in state.glob('*release-*.log.*'))
(Path(os.environ['WORKTREE']) / 'repair').write_text('fixed release build\\n')
''')
            codex.chmod(0o755)
            (state / 'notes').write_text('Release v2026.09.25\n')
            command = '''set -eu
log() { :; }
fail() { echo "$*" >&2; exit 9; }
abort_sync() { fail "$@"; }
next_release_tag() { echo v2026.09.25-2; }
wait_for_remote_workflow() {
  REMOTE_FAILED_RUN_ID=42
  [[ "$RECOVERY_MODE" == assets || "$2" != "$INITIAL_COMMIT" ]]
}
release_is_published() { [[ "$1" == v2026.09.25-2 ]]; }
wait_for_remote_workflows() { echo checked >> "$STATE_DIR/ci-checked"; }
run_validation_with_repairs() { VALIDATION_SUCCEEDED=true; echo validated >> "$STATE_DIR/validated"; }
persist_pending_release() { printf '%s %s\\n' "$1" "$2" > "$STATE_DIR/pending"; }
publish_release() {
  [[ -s "$STATE_DIR/validated" && -s "$STATE_DIR/ci-checked" ]] || fail 'published without validation'
  [[ -z "$(git -C "$WORKTREE" status --porcelain)" ]] || fail 'dirty publication'
  git -C "$WORKTREE" tag "$1"
  git -C "$WORKTREE" push origin "refs/tags/$1"
  RELEASE_TAG="$1"
}
''' + functions('repair_remote_candidate', 'run_codex_validation_repair', 'wait_for_release_workflow') + '''
wait_for_release_workflow v2026.09.25 "$INITIAL_COMMIT"
[[ "$RELEASE_STATE" == 已完成发布 ]]
[[ "$REMOTE_REPAIR_COUNT" == 1 ]]
'''
            result = subprocess.run(['bash', '-c', command], text=True, capture_output=True,
                env=dict(env, PATH=tmp + os.pathsep + env['PATH'], STATE_DIR=str(state), RUN_ID='test',
                    REPO_DIR=str(repo), WORKTREE=str(repo), SCRIPT_DIR=str(SCRIPT.parent), CODEX_BIN=str(codex),
                    WORKTREE_CREATED='true', ORIGIN_REMOTE='origin', TARGET_BRANCH='main', ORIGIN_REF='origin/main',
                    INITIAL_COMMIT=initial, MERGED_COMMIT=initial, PUSHED_COMMIT=initial, RECOVERY_MODE=mode,
                    CANDIDATE_COMMIT=initial, REMOTE_REPAIR_COUNT='0', VALIDATION_REPAIR_ATTEMPTS='3',
                    CURRENT_ORIGIN_HEAD=initial, CURRENT_UPSTREAM_HEAD=initial, UPSTREAM_REMOTE='upstream',
                    PRIMARY_REF='upstream/main', SECOND_REF='overdraft/main',
                    VALIDATION_FAILURES_FILE=str(state / 'failures'), LOG_FILE=str(state / 'log'),
                    RELEASE_NOTES_FILE=str(state / 'notes')))
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(git('rev-parse', 'v2026.09.25', cwd=remote), initial)
            repaired = git('rev-parse', 'main', cwd=remote)
            self.assertNotEqual(repaired, initial)
            self.assertEqual(git('rev-parse', 'HEAD^'), initial)
            self.assertEqual(git('rev-parse', 'v2026.09.25-2', cwd=remote), repaired)
            if 'new-api' in str(SCRIPT):
                self.assertIn('### 自上次正式发布以来的累计提交', (notes / 'v2026.09.25-2.md').read_text())


if __name__ == '__main__':
    unittest.main()
