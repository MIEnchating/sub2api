import importlib.util
import json
import os
import re
import subprocess
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(os.environ.get("SYNC_TEST_SCRIPT", Path(__file__).resolve().parents[1] / "scripts/upstream-sync.sh"))
HELPER = Path(os.environ.get("SYNC_TEST_RELEASE_HELPER", Path(__file__).with_name("upstream-release-window.py")))
spec = importlib.util.spec_from_file_location("release_window", HELPER)
release_window = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release_window)


class ValidationLogTest(unittest.TestCase):
    def test_each_check_and_retry_preserves_its_own_log(self):
        shell = SCRIPT.read_text()
        name = 'record_check' if 'record_check() {' in shell else 'run_validation_command'
        function = name + "() {" + shell.split(name + "() {", 1)[1].split("\n}\n", 1)[0] + "\n}"
        with tempfile.TemporaryDirectory(prefix="sync-validation-test-") as tmp:
            failures = Path(tmp) / "failures.txt"
            result = subprocess.run(
                ["bash", "-c", "set -Eeuo pipefail\n"
                 "log() { :; }\n" + function + "\n"
                 f"{name} '前端完整测试' bash -c 'echo frontend-failure >&2; exit 1'\n"
                 f"{name} '部署脚本验证' printf 'deployment-success\\n'\n"
                 f"{name} '前端完整测试' bash -c 'echo retry-failure >&2; exit 2'\n"
                 "wait\n"],
                env=os.environ | {"STATE_DIR": tmp, "RUN_ID": "test-run", "LC_ALL": "C",
                                  "VALIDATION_FAILURES_FILE": str(failures)},
                capture_output=True, text=True,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            entries = failures.read_text().splitlines()
            self.assertEqual(len(entries), 2)
            paths = [Path(re.search(r"日志：(.*)）$", entry).group(1)) for entry in entries]
            self.assertNotEqual(paths[0], paths[1])
            self.assertEqual(paths[0].read_text(), "frontend-failure\n")
            self.assertEqual(paths[1].read_text(), "retry-failure\n")
            logs = list(Path(tmp).glob("test-run-check-*"))
            self.assertEqual(len(logs), 3)
            self.assertIn("deployment-success\n", [path.read_text() for path in logs])


class ReleaseWindowTest(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory(prefix='release-window-test-')
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        self.repo = self.root / 'repo'
        self.env = dict(os.environ, GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_NOSYSTEM='1',
            GIT_AUTHOR_NAME='Test', GIT_AUTHOR_EMAIL='test@example.test',
            GIT_COMMITTER_NAME='Test', GIT_COMMITTER_EMAIL='test@example.test')
        self.git('init', '--initial-branch=main', str(self.repo), cwd=self.root)
        (self.repo / 'fork-only').write_text('already released custom feature\n' * 200)
        self.git('add', '.')
        self.git('commit', '-m', 'released baseline')
        self.base = self.git('rev-parse', 'HEAD')
        self.git('tag', 'v1')
        self.git('branch', 'upstream')

    def git(self, *args, cwd=None):
        return subprocess.check_output(['git', '-C', str(cwd or self.repo), *args],
            env=self.env, stderr=subprocess.DEVNULL).decode().strip()

    def add_updates(self, count):
        self.git('checkout', 'upstream')
        start = len(list(self.repo.glob('update-*')))
        for index in range(start, start + count):
            (self.repo / f'update-{index}').write_text(f'change {index}\n')
            self.git('add', '.')
            self.git('commit', '-m', f'upstream update {index}')
        self.git('checkout', 'main')
        self.git('merge', '--no-ff', '-m', 'daily sync', 'upstream')

    def test_two_plus_one_plus_two_accumulates_and_successful_release_resets(self):
        cumulative = 0
        for count in [2, 1, 2]:
            self.add_updates(count)
            cumulative += count
            # Two upstream refs pointing at shared history must not double-count.
            self.assertEqual(release_window.measure(str(self.repo), self.base, ['upstream', 'HEAD'], 'HEAD'),
                             (cumulative, cumulative, cumulative))
        latest = self.git('rev-parse', 'HEAD')
        self.git('tag', 'v2')
        self.add_updates(1)
        self.assertEqual(release_window.measure(str(self.repo), latest, ['upstream'], 'HEAD'), (1, 1, 1))

    def test_published_release_only_and_release_branch_baseline(self):
        self.add_updates(2)
        self.git('tag', 'unpublished')
        self.git('checkout', '-b', 'release')
        (self.repo / 'version').write_text('v2\n')
        self.git('add', '.')
        self.git('commit', '-m', 'release version')
        released = self.git('rev-parse', 'HEAD')
        self.git('tag', 'v2')
        self.git('checkout', 'main')
        remote = self.root / 'fork.git'
        self.git('clone', '--bare', str(self.repo), str(remote), cwd=self.root)
        self.git('remote', 'add', 'origin', str(remote))
        self.add_updates(1)
        old = {'tag_name': 'v1', 'published_at': '2026-09-01', 'assets': [{}]}
        second = {'tag_name': 'v2', 'published_at': '2026-09-02', 'assets': [{}]}
        draft = dict(second, tag_name='unpublished', published_at='2026-09-03', draft=True)
        preview = dict(draft, draft=False, prerelease=True)
        no_assets = dict(draft, draft=False, assets=[])
        self.assertEqual(release_window.release_baseline(str(self.repo), 'origin', 'HEAD',
            [draft, preview, no_assets, old]), ('v1', self.base))
        self.assertEqual(release_window.release_baseline(str(self.repo), 'origin', 'HEAD',
            [draft, old, second]), ('v2', released))

    def test_daily_checks_do_not_reset_and_net_diff_excludes_reverted_changes(self):
        self.add_updates(2)
        (self.repo / 'update-0').unlink()
        self.git('add', '.')
        self.git('commit', '-m', 'revert one change')
        first = release_window.measure(str(self.repo), self.base, ['upstream'], 'HEAD')
        self.assertEqual(first, (2, 1, 1))
        self.assertEqual(release_window.measure(str(self.repo), self.base, ['upstream'], 'HEAD'), first)
        self.assertEqual(release_window.release_baseline(str(self.repo), 'origin', 'HEAD', []), ('initial', self.base))

    def test_eligibility_uses_accumulated_window_instead_of_last_sync(self):
        self.add_updates(2)
        self.add_updates(1)
        yesterday = self.git('rev-parse', 'HEAD')
        self.add_updates(2)
        head = self.git('rev-parse', 'HEAD')
        review = self.root / 'review.json'
        review.write_text(json.dumps({'decision': 'resolved', 'risks': [], 'merged_behavior': ['updates']}))
        shell = SCRIPT.read_text()
        function = 'evaluate_release_eligibility() {' + shell.split('evaluate_release_eligibility() {', 1)[1].split('\n}\n', 1)[0] + '\n}'
        env = dict(self.env, SCRIPT_DIR=str(SCRIPT.parent), NODE_BIN='node', WORKTREE=str(self.repo), REPO_DIR=str(self.repo),
            CURRENT_ORIGIN_HEAD=yesterday, ORIGIN_HEAD=yesterday, CURRENT_UPSTREAM_HEAD=head, PRIMARY_HEAD=head, SECOND_HEAD=head,
            CANDIDATE_COMMIT=head, RELEASE_BASE_TAG='v1', RELEASE_BASE_COMMIT=self.base, RELEASE_ENABLED='true',
            RELEASE_MIN_UPSTREAM_COMMITS='5', RELEASE_MIN_CHANGED_FILES='5', RELEASE_MIN_DIFF_LINES='5',
            RELEASE_REQUIRE_NO_RISKS='true', RELEASE_IGNORE_ENVIRONMENT_RISKS='false',
            DECISION_FILE=str(review), REVIEW_DECISION_FILE=str(review))
        result = subprocess.run(['bash', '-c', 'set -eu\nlog() { :; }\nfail() { exit 9; }\nabort_sync() { exit 9; }\n' + function +
            '\nevaluate_release_eligibility\necho "$RELEASE_STATE $RELEASE_REASON"'], env=env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('待发布', result.stdout)
        self.assertIn('提交=5/5 文件=5/5 行=5/5', result.stdout)

    def test_release_recovery_tags_checked_repair_not_original_failed_commit(self):
        tag = 'v2026.9.24'
        notes = self.repo / '.github/release-notes' / (tag + '.md')
        notes.parent.mkdir(parents=True)
        notes.write_text('Release notes\n')
        self.git('add', '.')
        self.git('commit', '-m', 'prepare release')
        original = self.git('rev-parse', 'HEAD')
        (self.repo / 'fix').write_text('CI repair\n')
        self.git('add', '.')
        self.git('commit', '-m', 'repair CI')
        repaired = self.git('rev-parse', 'HEAD')
        remote = self.root / 'remote.git'
        self.git('clone', '--bare', str(self.repo), str(remote), cwd=self.root)
        self.git('remote', 'add', 'origin', str(remote))
        self.git('fetch', 'origin')
        state = self.root / 'state'
        state.mkdir()
        pending = state / 'pending'
        pending.write_text(f'tag={tag}\ncommit={original}\n')
        pending_notes = state / 'pending-notes'
        pending_notes.write_text('Release notes\n')
        shell = SCRIPT.read_text()
        function = 'recover_pending_release() {' + shell.split('recover_pending_release() {', 1)[1].split('\n}\n', 1)[0] + '\n}'
        env = dict(self.env, REPO_DIR=str(self.repo), STATE_DIR=str(state), RUN_ID='test',
            ORIGIN_REMOTE='origin', ORIGIN_REF='origin/main', TARGET_BRANCH='main', RELEASE_ENABLED='true',
            PENDING_RELEASE_FILE=str(pending), PENDING_RELEASE_NOTES_FILE=str(pending_notes),
            RELEASE_NOTES_FILE=str(state / 'notes'), REPORT_FILE=str(state / 'report'),
            HTML_REPORT_FILE=str(state / 'report.html'), EXPECTED_COMMIT=repaired, EXPECTED_TAG=tag,
            WORKTREE_CREATED='true', VALIDATION_FAILURES_FILE=str(state / 'failures'), WORKTREE=str(self.repo))
        command = '''set -eu
log() { :; }
fail() { echo "$*" >&2; exit 9; }
abort_sync() { fail "$@"; }
pending_release_tag() { echo "$EXPECTED_TAG"; }
release_is_complete() { return 1; }
run_validation_command() { :; }
workflow_trigger_state() { return 0; }
persist_pending_release() { :; }
clear_pending_release() { :; }
write_report() { :; }
send_email() { :; }
wait_for_remote_workflows() {
  [[ "$1" == "$EXPECTED_COMMIT" ]] || fail 'checked stale commit'
  [[ "$CHECK_RESULT" == success ]] || fail 'CI failed'
}
wait_for_release_workflow() {
  [[ "$2" == "$EXPECTED_COMMIT" ]] || fail 'released stale commit'
}
''' + function + '\nif recover_pending_release; then :; else exit 7; fi'
        failed = subprocess.run(['bash', '-c', command], env=dict(env, CHECK_RESULT='failure'), text=True, capture_output=True)
        self.assertNotEqual(failed.returncode, 0)
        self.assertEqual(self.git('tag', '--list', tag, cwd=remote), '')
        result = subprocess.run(['bash', '-c', command], env=dict(env, CHECK_RESULT='success'), text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.git('rev-parse', tag + '^{commit}', cwd=remote), repaired)

    def test_remote_failure_repairs_new_commit_or_stops_without_unsafe_push(self):
        remote = self.root / 'remote.git'
        self.git('clone', '--bare', str(self.repo), str(remote), cwd=self.root)
        self.git('remote', 'add', 'origin', str(remote))
        self.git('fetch', 'origin')
        tools = self.root / 'tools'
        tools.mkdir()
        gh = tools / 'gh'
        gh.write_text('''#!/usr/bin/env python3
import json, sys
if sys.argv[1] == 'run':
    print(json.dumps({'headSha': 'fixture', 'status': 'completed', 'conclusion': 'failure',
        'jobs': [{'databaseId': 10, 'name': 'shell', 'conclusion': 'failure'}]}))
else:
    print('mktemp: File exists')
''')
        gh.chmod(0o755)
        codex = tools / 'codex'
        codex.write_text('''#!/usr/bin/env python3
import os
from pathlib import Path
root = Path(os.environ['WORKTREE'])
failures = Path(os.environ['VALIDATION_FAILURES_FILE']).read_text()
assert 'CI' in failures and 'Security Scan' in failures
logs = list(Path(os.environ['STATE_DIR']).glob('*remote.log.*'))
assert logs and all('mktemp: File exists' in path.read_text() for path in logs)
if os.environ['REPAIR_MODE'] != 'nochange':
    path = root / 'repair'
    path.write_text(path.read_text() + 'fixed\\n' if path.exists() else 'fixed\\n')
''')
        codex.chmod(0o755)
        shell = SCRIPT.read_text()
        functions = '\n'.join(name + '() {' + shell.split(name + '() {', 1)[1].split('\n}\n', 1)[0] + '\n}'
            for name in ['wait_for_remote_workflows', 'repair_remote_candidate', 'run_codex_validation_repair'])
        for mode in ['success', 'nochange', 'validation_failure', 'concurrent', 'exhausted']:
            with self.subTest(mode=mode):
                case_repo = self.root / mode
                self.git('clone', str(remote), str(case_repo), cwd=self.root)
                initial = self.git('rev-parse', 'HEAD', cwd=case_repo)
                state = self.root / ('state-' + mode)
                state.mkdir()
                env = dict(self.env, PATH=str(tools) + os.pathsep + os.environ['PATH'],
                    REPO_DIR=str(case_repo), WORKTREE=str(case_repo), STATE_DIR=str(state),
                    SCRIPT_DIR=str(SCRIPT.parent), RUN_ID='test', CODEX_BIN=str(codex),
                    VALIDATION_FAILURES_FILE=str(state / 'failures'), LOG_FILE=str(state / 'log'),
                    CURRENT_ORIGIN_HEAD=initial, CURRENT_UPSTREAM_HEAD=initial, INITIAL_COMMIT=initial,
                    PRIMARY_REF='upstream/main', SECOND_REF='overdraft/main', ORIGIN_REF='origin/main',
                    ORIGIN_REMOTE='origin', UPSTREAM_REMOTE='upstream', TARGET_BRANCH='main',
                    WORKTREE_CREATED='true', REMOTE_REPAIR_COUNT='0', VALIDATION_REPAIR_ATTEMPTS='2',
                    REPAIR_COUNT='0', PUSHED_COMMIT=initial, MERGED_COMMIT=initial, CANDIDATE_COMMIT=initial,
                    RELEASE_TAG='', RELEASE_NOTES_FILE=str(state / 'notes'), REPAIR_MODE=mode)
                if mode == 'concurrent':
                    self.git('fetch', 'origin')
                    self.git('merge', '--ff-only', 'origin/main')
                    (self.repo / 'concurrent').write_text('other work')
                    self.git('add', '.')
                    self.git('commit', '-m', 'other user update')
                    self.git('push', 'origin', 'HEAD:main')
                command = '''set -eu
log() { :; }
require_command() { :; }
fail() { echo "$*" >&2; exit 9; }
abort_sync() { fail "$@"; }
wait_for_remote_workflow() {
  REMOTE_FAILED_RUN_ID=123
  [[ "$2" != "$INITIAL_COMMIT" && "$REPAIR_MODE" != exhausted ]]
}
run_validation_with_repairs() {
  printf 'validated\\n' >> "$STATE_DIR/validation-ran"
  VALIDATION_SUCCEEDED=true
  [[ "$REPAIR_MODE" != validation_failure ]] || VALIDATION_SUCCEEDED=false
}
''' + functions + '\nwait_for_remote_workflows "$INITIAL_COMMIT"'
                before_remote = self.git('rev-parse', 'main', cwd=remote)
                result = subprocess.run(['bash', '-c', command], env=env, text=True, capture_output=True)
                final_remote = self.git('rev-parse', 'main', cwd=remote)
                if mode == 'success':
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertNotEqual(initial, final_remote)
                    self.assertEqual(self.git('rev-parse', 'HEAD^', cwd=case_repo), initial)
                    self.assertTrue((state / 'validation-ran').exists())
                else:
                    self.assertNotEqual(result.returncode, 0)
                    if mode == 'exhausted':
                        self.assertEqual(len((state / 'validation-ran').read_text().splitlines()), 2)
                        self.assertEqual(self.git('rev-list', '--count', initial + '..HEAD', cwd=case_repo), '2')
                    else:
                        self.assertEqual(final_remote, before_remote)


if __name__ == "__main__":
    unittest.main()
