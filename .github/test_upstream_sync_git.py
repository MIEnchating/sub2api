"""Real Git regressions for conflict retries and post-push finalization."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


SCRIPT = Path(os.environ.get('SYNC_TEST_SCRIPT', Path(__file__).resolve().parents[1] / 'scripts/upstream-sync.sh'))
HELPER = SCRIPT.with_name('upstream-sync-git.py')
IS_NEW = SCRIPT.name == 'sync-upstream.sh'
PREFIX = 'automation/upstream-sync-' if IS_NEW else 'automation/sub2api-upstream-sync-'


def functions(*names):
    shell = SCRIPT.read_text()
    return '\n'.join(name + '() {' + shell.split(name + '() {', 1)[1].split('\n}\n', 1)[0] + '\n}' for name in names)


def fixture(schema):
    if 'enum' in schema:
        return schema['enum'][0]
    if schema['type'] == 'string':
        return '测试'
    if schema['type'] == 'array':
        return []
    return {key: fixture(schema['properties'][key]) for key in schema['required']}


class SyncGitTest(unittest.TestCase):
    def setUp(self):
        folder = tempfile.TemporaryDirectory(prefix='sync-git-')
        self.addCleanup(folder.cleanup)
        self.root = Path(folder.name)
        self.repo = self.root / 'repo'
        self.remote = self.root / 'remote.git'
        self.trees = self.root / 'trees'
        self.env = {key: value for key, value in os.environ.items() if not key.startswith('GIT_')}
        self.env.update(GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_NOSYSTEM='1',
                        GIT_AUTHOR_NAME='Test', GIT_AUTHOR_EMAIL='test@example.test',
                        GIT_COMMITTER_NAME='Test', GIT_COMMITTER_EMAIL='test@example.test',
                        GIT_TERMINAL_PROMPT='0')
        self.git(self.root, 'init', '--bare', '--initial-branch=main', str(self.remote))
        self.git(self.root, 'init', '--initial-branch=main', str(self.repo))
        (self.repo / 'file').write_text('base\n')
        (self.repo / '.gitignore').write_text('build/\n')
        self.git(self.repo, 'add', '.')
        self.git(self.repo, 'commit', '-m', 'base')
        self.git(self.repo, 'remote', 'add', 'origin', str(self.remote))
        self.git(self.repo, 'push', '-u', 'origin', 'main')
        self.base = self.git(self.repo, 'rev-parse', 'HEAD')

    def git(self, repo, *args):
        return subprocess.check_output(['git', '-C', str(repo), *args], env=self.env,
                                       stderr=subprocess.DEVNULL, text=True).strip()

    def candidate(self, name='candidate', push=True, outside=False):
        path = (self.root if outside else self.trees) / name
        self.git(self.repo, 'worktree', 'add', '-b', PREFIX + name, str(path), 'main')
        (path / 'file').write_text(name + '\n')
        self.git(path, 'commit', '-am', name)
        if push:
            self.git(path, 'push', 'origin', 'HEAD:main')
        return path

    def finalize(self, *args):
        result = subprocess.run(['python3', str(HELPER), 'finalize', '--repo', str(self.repo),
            '--remote', 'origin', '--target', 'main', '--worktree-root', str(self.trees),
            '--branch-prefix', PREFIX, *args], env=self.env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        return result

    def branch_exists(self, name='candidate'):
        return bool(self.git(self.repo, 'branch', '--list', PREFIX + name))

    def test_fast_forward_and_remove_pushed_candidate_and_old_branch(self):
        tree = self.candidate()
        self.git(self.repo, 'branch', PREFIX + 'old', 'origin/main')
        (tree / 'build').mkdir()
        (tree / 'build/output').write_text('generated')
        self.finalize()
        self.assertEqual(self.git(self.repo, 'rev-parse', 'HEAD'), self.git(self.remote, 'rev-parse', 'main'))
        self.assertFalse(tree.exists())
        self.assertFalse(self.branch_exists())
        self.assertFalse(self.branch_exists('old'))

    def test_sync_only_keeps_active_candidate_for_ci_repairs(self):
        tree = self.candidate()
        self.finalize('--sync-only')
        self.assertTrue(tree.exists())
        self.assertTrue(self.branch_exists())
        self.assertEqual(self.git(self.repo, 'rev-parse', 'HEAD'), self.git(tree, 'rev-parse', 'HEAD'))

    def test_unpushed_candidate_and_orphan_branch_are_retained(self):
        tree = self.candidate(push=False)
        self.git(self.repo, 'branch', PREFIX + 'orphan', PREFIX + 'candidate')
        self.finalize()
        self.assertTrue(tree.exists())
        self.assertTrue(self.branch_exists('orphan'))
        self.assertEqual(self.git(self.repo, 'rev-parse', 'HEAD'), self.base)

    def test_untracked_dirty_candidate_is_retained(self):
        tree = self.candidate()
        (tree / 'untracked').write_text('repair in progress')
        self.finalize()
        self.assertTrue(tree.exists())
        self.assertTrue(self.branch_exists())

    def test_staged_repair_is_retained_after_push(self):
        tree = self.candidate()
        (tree / 'file').write_text('repair')
        self.git(tree, 'add', '.')
        self.finalize()
        self.assertEqual((tree / 'file').read_text(), 'repair')

    def test_unpushed_repair_commit_is_retained_after_push(self):
        tree = self.candidate()
        self.git(tree, 'commit', '--allow-empty', '-m', 'repair')
        self.finalize()
        self.assertTrue(tree.exists())

    def test_locked_candidate_is_retained(self):
        tree = self.candidate()
        self.git(self.repo, 'worktree', 'lock', str(tree))
        self.finalize()
        self.assertTrue(tree.exists())
        self.assertTrue(self.branch_exists())

    def test_unfinished_operation_is_retained_even_with_clean_status(self):
        tree = self.candidate()
        operation = Path(self.git(tree, 'rev-parse', '--git-path', 'CHERRY_PICK_HEAD'))
        operation.write_text(self.base + '\n')
        self.finalize()
        self.assertTrue(tree.exists())

    def test_candidate_outside_owned_root_is_retained(self):
        tree = self.candidate(outside=True)
        self.finalize()
        self.assertTrue(tree.exists())
        self.assertTrue(self.branch_exists())

    def test_dirty_primary_is_preserved_but_merged_candidate_is_removed(self):
        tree = self.candidate()
        (self.repo / 'file').write_text('user work')
        self.finalize()
        self.assertEqual((self.repo / 'file').read_text(), 'user work')
        self.assertEqual(self.git(self.repo, 'rev-parse', 'HEAD'), self.base)
        self.assertFalse(tree.exists())

    def test_wrong_primary_branch_is_not_fast_forwarded(self):
        self.candidate()
        self.git(self.repo, 'switch', '-c', 'user-feature')
        self.finalize()
        self.assertEqual(self.git(self.repo, 'rev-parse', 'HEAD'), self.base)

    def test_divergent_primary_and_unrelated_branch_are_preserved(self):
        self.candidate()
        self.git(self.repo, 'commit', '--allow-empty', '-m', 'local work')
        local = self.git(self.repo, 'rev-parse', 'HEAD')
        self.git(self.repo, 'branch', 'user-feature', 'origin/main')
        self.finalize()
        self.assertEqual(self.git(self.repo, 'rev-parse', 'HEAD'), local)
        self.assertTrue(self.git(self.repo, 'branch', '--list', 'user-feature'))

    def test_failed_fetch_cannot_delete_using_stale_tracking_ref(self):
        tree = self.candidate()
        self.git(self.repo, 'remote', 'set-url', 'origin', str(self.root / 'missing.git'))
        result = subprocess.run(['python3', str(HELPER), 'finalize', '--repo', str(self.repo),
            '--remote', 'origin', '--target', 'main', '--worktree-root', str(self.trees),
            '--branch-prefix', PREFIX], env=self.env, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertTrue(tree.exists())
        self.assertTrue(self.branch_exists())
        self.assertEqual(self.git(self.repo, 'rev-parse', 'HEAD'), self.base)

    def test_failed_repair_commit_cannot_pass_validation_or_skip_retry(self):
        validation = 'run_complete_validation_pass' if IS_NEW else 'run_validation_pass'
        command = 'set -Eeuo pipefail\nlog() { :; }\n' + functions('run_validation_with_repairs')
        command += f'\n{validation}() {{ echo "pass $1"; [[ "$1" != 1 ]]; }}\n'
        command += '''
run_codex_validation_repair() { echo "repair $1"; [[ "$1" == 2 ]]; }
run_validation_with_repairs
echo "$VALIDATION_SUCCEEDED"
'''
        result = subprocess.run(['bash', '-c', command], capture_output=True, text=True,
            env=self.env | dict(VALIDATE='true', VALIDATION_REPAIR_ATTEMPTS='3',
                LOG_FILE=str(self.root / 'log'), VALIDATION_FAILURES_FILE=str(self.root / 'failures')))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.splitlines(),
                         ['pass 1', 'repair 1', 'pass 2', 'repair 2', 'pass 3', 'true'])

    def test_cleanup_runs_after_release_failure_and_preserves_exit_status(self):
        tree = self.candidate()
        env = self.env | dict(SCRIPT_DIR=str(SCRIPT.parent), REPO_DIR=str(self.repo),
            ORIGIN_REMOTE='origin', TARGET_BRANCH='main', WORKTREE_ROOT=str(self.trees),
            FINALIZATION_ENABLED='true', DRY_RUN='false', SYNC_DRY_RUN='false',
            DB_SERVICES_CREATED='false', PROMPT_FILE=str(self.root / 'prompt'))
        command = 'set -Eeuo pipefail\nlog() { echo "$*"; }\n' + functions('finalize_sync', 'cleanup')
        command += '\ntrap cleanup EXIT\nexit 42'
        result = subprocess.run(['bash', '-c', command], env=env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 42, result.stderr)
        self.assertFalse(tree.exists())
        self.assertFalse(self.branch_exists())
        self.assertEqual(self.git(self.repo, 'rev-parse', 'HEAD'), self.git(self.remote, 'rev-parse', 'main'))

    def test_dry_run_finalization_does_not_modify_any_refs(self):
        tree = self.candidate()
        command = 'set -eu\nlog() { :; }\n' + functions('finalize_sync') + '\nfinalize_sync'
        subprocess.run(['bash', '-c', command], env=self.env | dict(FINALIZATION_ENABLED='true',
                       DRY_RUN='true', SYNC_DRY_RUN='true'), check=True)
        self.assertTrue(tree.exists())
        self.assertEqual(self.git(self.repo, 'rev-parse', 'HEAD'), self.base)

    def make_conflict(self):
        self.git(self.repo, 'checkout', '-b', 'upstream')
        (self.repo / 'file').write_text('upstream\n')
        self.git(self.repo, 'commit', '-am', 'upstream')
        self.git(self.repo, 'checkout', 'main')
        (self.repo / 'file').write_text('fork\n')
        self.git(self.repo, 'commit', '-am', 'fork')
        result = subprocess.run(['git', '-C', str(self.repo), 'merge', 'upstream'],
                                env=self.env, capture_output=True)
        self.assertNotEqual(result.returncode, 0)

    def test_review_preserves_conflict_stages_until_validated(self):
        self.make_conflict()
        stages = self.git(self.repo, 'ls-files', '-u')
        for contents, succeeds in [('<<<<<<< HEAD\nfork\n=======\nupstream\n>>>>>>> upstream\n', False),
                                   ('merged   \n', False), ('fork and upstream\n', True)]:
            (self.repo / 'file').write_text(contents)
            result = subprocess.run(['python3', str(HELPER), 'review', '--repo', str(self.repo)],
                                    env=self.env, capture_output=True, text=True)
            self.assertEqual(result.returncode == 0, succeeds, result.stderr)
            self.assertEqual(self.git(self.repo, 'ls-files', '-u'), stages)

    def test_review_checks_newly_tracked_files_under_ignored_paths(self):
        (self.repo / 'build').mkdir()
        (self.repo / 'build/new-file').write_text('<<<<<<< HEAD\n')
        self.git(self.repo, 'add', '-f', 'build/new-file')
        result = subprocess.run(['python3', str(HELPER), 'review', '--repo', str(self.repo)],
                                env=self.env, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('build/new-file', result.stderr)

    def test_resolved_decision_with_actual_conflicts_retries_then_succeeds(self):
        self.exercise_review_retry('markers')

    def test_resolved_decision_with_excluded_feature_retries_then_succeeds(self):
        if IS_NEW:
            self.skipTest('Feature exclusions are sub2api product rules')
        self.exercise_review_retry('exclusions')

    def exercise_review_retry(self, mode):
        self.make_conflict()
        stages = self.git(self.repo, 'ls-files', '-u')
        state = self.root / 'state'
        state.mkdir()
        schema = SCRIPT.parent / 'decision-schema.json' if IS_NEW else SCRIPT.parent.parent / '.github/upstream-sync-decision-schema.json'
        (state / 'decision').write_text(json.dumps(fixture(json.loads(schema.read_text()))))
        if not IS_NEW:
            (self.repo / '.github').mkdir()
            for name in ('check-upstream-exclusions.py', 'upstream-exclusions.json'):
                shutil.copyfile(schema.parent / name, self.repo / '.github' / name)
        fake = self.root / 'codex'
        fake.write_text('''#!/usr/bin/env python3
import os, sys, subprocess
from pathlib import Path
state = Path(os.environ['STATE_DIR'])
repo = Path(os.environ['WORKTREE'])
counter = state / 'count'
n = int(counter.read_text()) + 1 if counter.exists() else 1
counter.write_text(str(n))
if n > 1:
    assert subprocess.check_output(['git', '-C', str(repo), 'ls-files', '-u']).strip()
    (repo / 'file').write_text('fork and upstream\\n')
    (repo / 'shared-pool.txt').unlink(missing_ok=True)
elif os.environ['RETRY_MODE'] == 'exclusions':
    (repo / 'file').write_text('fork and upstream\\n')
    (repo / 'shared-pool.txt').write_text('excluded')
Path(sys.argv[sys.argv.index('--output-last-message') + 1]).write_text((state / 'decision').read_text())
''')
        fake.chmod(0o755)
        name = 'run_codex_review' if IS_NEW else 'run_codex_merge_review'
        command = 'set -Eeuo pipefail\nlog() { :; }\nfail() { echo "$*" >&2; exit 1; }\nabort_sync() { fail "$@"; }\n'
        command += functions(name) + '\n' + name + ' true'
        env = self.env | dict(SCRIPT_DIR=str(SCRIPT.parent), REPO_DIR=str(SCRIPT.parent.parent),
            WORKTREE=str(self.repo), STATE_DIR=str(state), RUN_ID='test', CODEX_BIN=str(fake),
            REVIEW_REPAIR_ATTEMPTS='3', RETRY_MODE=mode, LOG_FILE=str(state / 'log'),
            PROMPT_FILE=str(state / 'prompt'), DECISION_FILE=str(state / 'accepted'),
            REVIEW_DECISION_FILE=str(state / 'accepted'), REVIEW_SUMMARY_FILE=str(state / 'summary'),
            ORIGIN_REMOTE='origin', UPSTREAM_REMOTE='upstream', TARGET_BRANCH='main',
            CURRENT_ORIGIN_HEAD='base', CURRENT_UPSTREAM_HEAD='upstream', ORIGIN_REF='origin/main',
            PRIMARY_REF='upstream/main', SECOND_REF='overdraft/main')
        result = subprocess.run(['bash', '-c', command], env=env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((state / 'count').read_text(), '2')
        self.assertEqual((self.repo / 'file').read_text(), 'fork and upstream\n')
        if not IS_NEW:
            self.assertEqual(self.git(self.repo, 'ls-files', '-u'), stages)


if __name__ == '__main__':
    unittest.main()
