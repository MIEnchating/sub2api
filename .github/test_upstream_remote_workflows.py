"""Keep unavailable GitHub results distinct from completed, repairable failures."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

SCRIPT = Path(os.environ.get('SYNC_TEST_SCRIPT', Path(__file__).resolve().parents[1] / 'scripts/upstream-sync.sh'))


def functions(*names):
    shell = SCRIPT.read_text()
    return '\n'.join(name + '() {' + shell.split(name + '() {', 1)[1].split('\n}\n', 1)[0] + '\n}'
                     for name in names)


class RemoteWorkflowTest(unittest.TestCase):
    def exercise_wait(self, responses, *, timeout=False, retries=0, attempt_error=False, dispatch_status=0, startup_timeout=False):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            gh = root / 'gh'
            gh.write_text('''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
root = Path(os.environ['STATE_DIR'])
if sys.argv[1:3] == ['run', 'list']:
    counter = root / 'calls'
    n = int(counter.read_text()) if counter.exists() else 0
    counter.write_text(str(n + 1))
    responses = json.loads(os.environ['RESPONSES'])
    code, stdout, stderr = responses[min(n, len(responses) - 1)]
    print(stdout)
    print(stderr, file=sys.stderr)
    sys.exit(code)
elif sys.argv[1:3] == ['run', 'rerun']:
    (root / 'rerun').touch()
elif sys.argv[1] == 'api':
    if sys.argv[-1] == '.run_attempt':
        print(1)
    else:
        print('HTTP 503 attempt query unavailable', file=sys.stderr)
        sys.exit(1)
else:
    sys.exit(99)
''')
            gh.chmod(0o755)
            command = '''set -eu
log() { :; }
workflow_trigger_state() { return 0; }
git() { echo git@github.com:owner/project.git; }
sleep() {
  if [[ "$SIMULATE_TIMEOUT" == true ]]; then SECONDS=$((SECONDS + 61)); fi
  if [[ "$STARTUP_TIMEOUT" == true && -f "$STATE_DIR/dispatches" ]]; then SECONDS=$((SECONDS + 301)); fi
}
python3() {
  if [[ "$1" == */upstream-start-workflow.py || "$1" == */start-workflow.py ]]; then
    echo 'dispatch attempted' >> "$STATE_DIR/dispatches"
    echo 'dispatch diagnostic'
    return "$DISPATCH_STATUS"
  fi
  command python3 "$@"
}
''' + functions('wait_for_remote_workflow') + '''
result=0
wait_for_remote_workflow "$WORKFLOW" abc "$WORKFLOW" || result=$?
printf '%s|%s|%s\\n' "$result" "$REMOTE_WORKFLOW_UNCONFIRMED" "$REMOTE_FAILED_RUN_ID"
cat "$REMOTE_WORKFLOW_DIAGNOSTIC_LOG"
[[ ! -f "$STATE_DIR/dispatches" ]] || cat "$STATE_DIR/dispatches"
echo "platform_blocked=$REMOTE_PLATFORM_BLOCKED"
'''
            result = subprocess.run(['bash', '-c', command], text=True, capture_output=True, timeout=10,
                env=os.environ | dict(PATH=tmp + os.pathsep + os.environ['PATH'], STATE_DIR=tmp,
                    REPO_DIR=tmp, SCRIPT_DIR=str(SCRIPT.parent), TARGET_BRANCH='main',
                    ORIGIN_REMOTE='origin', WORKFLOW='Release' if attempt_error else 'CI',
                    DISPATCH_STATUS=str(dispatch_status), STARTUP_TIMEOUT=str(startup_timeout).lower(),
                    REMOTE_WORKFLOW_TIMEOUT_MINUTES='90' if startup_timeout else '1', REMOTE_WORKFLOW_POLL_SECONDS='1',
                    REMOTE_WORKFLOW_RETRY_ATTEMPTS=str(retries), RESPONSES=json.dumps(responses),
                    SIMULATE_TIMEOUT=str(timeout).lower()))
            self.assertEqual(result.returncode, 0, result.stderr)
            return result.stdout, int((root / 'calls').read_text())

    @staticmethod
    def run_response(status='completed', conclusion='success', sha='abc'):
        return (0, json.dumps([dict(databaseId=42, headSha=sha, status=status, conclusion=conclusion)]), '')

    def test_api_errors_are_bounded_and_retained(self):
        output, calls = self.exercise_wait([(1, '', 'HTTP 403 rate limit exceeded')])
        self.assertEqual(calls, 3)
        self.assertTrue(output.startswith('1|true|\n'), output)
        self.assertIn('HTTP 403 rate limit exceeded', output)
        self.assertIn('exit=1', output)

    def test_malformed_results_do_not_become_empty_run_lists(self):
        for payload in ('not JSON', '{}', '[null]', '[{}]',
                        json.dumps([dict(databaseId=42, headSha='abc', status='completed', conclusion=None)])):
            with self.subTest(payload=payload):
                output, calls = self.exercise_wait([(0, payload, '')])
                self.assertEqual(calls, 3)
                self.assertTrue(output.startswith('1|true|\n'), output)
                self.assertIn('invalid', output)

    def test_transient_query_failure_can_recover(self):
        output, calls = self.exercise_wait([(1, '', 'HTTP 502'), self.run_response()])
        self.assertEqual(calls, 2)
        self.assertTrue(output.startswith('0|false|42\n'), output)

    def test_empty_or_wrong_commit_results_remain_unconfirmed(self):
        for response in ((0, '[]', ''), self.run_response(sha='other')):
            output, calls = self.exercise_wait([response], timeout=True)
            self.assertEqual(calls, 1)
            self.assertTrue(output.startswith('1|true|\n'), output)
            self.assertIn('No matching run: CI commit=abc', output)
            self.assertIn('run=not-found', output)

    def test_in_progress_timeout_retains_run_id_without_authorizing_repair(self):
        output, calls = self.exercise_wait([self.run_response('in_progress', '')], timeout=True)
        self.assertEqual(calls, 1)
        self.assertTrue(output.startswith('1|true|42\n'), output)
        self.assertIn('run=42', output)

    def test_completed_failure_is_repairable(self):
        output, calls = self.exercise_wait([self.run_response(conclusion='failure')])
        self.assertEqual(calls, 1)
        self.assertTrue(output.startswith('1|false|42\n'), output)

    def test_missing_run_is_dispatched_once_and_then_checked(self):
        output, calls = self.exercise_wait([(0, '[]', '')] * 4 + [self.run_response()])
        self.assertEqual(calls, 5)
        self.assertTrue(output.startswith('0|false|42\n'), output)
        self.assertEqual(output.count('dispatch attempted'), 1)

    def test_platform_disabled_stops_startup_without_authorizing_code_repair(self):
        output, calls = self.exercise_wait([(0, '[]', '')], dispatch_status=3)
        self.assertEqual(calls, 3)
        self.assertTrue(output.startswith('1|true|\n'), output)
        self.assertIn('platform_blocked=true', output)

    def test_dispatch_error_stops_without_claiming_a_test_failure(self):
        output, calls = self.exercise_wait([(0, '[]', '')], dispatch_status=1)
        self.assertEqual(calls, 3)
        self.assertTrue(output.startswith('1|true|\n'), output)
        self.assertIn('dispatch diagnostic', output)

    def test_successful_dispatch_is_not_success_without_run(self):
        output, calls = self.exercise_wait([(0, '[]', '')], startup_timeout=True)
        self.assertEqual(calls, 4)
        self.assertTrue(output.startswith('1|true|\n'), output)
        self.assertIn('Workflow startup timed out', output)
        self.assertEqual(output.count('dispatch attempted'), 1)

    def test_rerun_query_errors_are_bounded_and_unconfirmed(self):
        output, calls = self.exercise_wait([self.run_response(conclusion='failure')], retries=1, attempt_error=True)
        self.assertEqual(calls, 4)
        self.assertTrue(output.startswith('1|true|42\n'), output)
        self.assertIn('HTTP 503 attempt query unavailable', output)
        self.assertIn('(3/3)', output)

    def test_unconfirmed_or_incomplete_logs_block_ci_and_release_repairs(self):
        for gate in ('wait_for_remote_workflows', 'wait_for_release_workflow'):
            for run_id, unconfirmed, collector_status in (('', 'true', '0'), ('42', 'true', '0'), ('42', 'false', '1')):
                with self.subTest(gate=gate, run_id=run_id, collector_status=collector_status), tempfile.TemporaryDirectory() as tmp:
                    command = '''set -eu
log() { :; }
require_command() { :; }
git() { echo git@github.com:owner/project.git; }
fail() { echo "$*" >&2; exit 9; }
abort_sync() { fail "$@"; }
repair_remote_candidate() { touch "$STATE_DIR/unsafe-repair"; exit 99; }
python3() { echo 'job diagnostics' > "$4"; return "$COLLECT_STATUS"; }
wait_for_remote_workflow() {
  REMOTE_FAILED_RUN_ID="$TEST_RUN_ID"
  REMOTE_WORKFLOW_UNCONFIRMED="$TEST_UNCONFIRMED"
  REMOTE_WORKFLOW_DIAGNOSTIC_LOG="$STATE_DIR/query-$1"
  echo "$1: query diagnostics" > "$REMOTE_WORKFLOW_DIAGNOSTIC_LOG"
  return 1
}
''' + functions(gate) + '\n' + gate + (' abc' if gate == 'wait_for_remote_workflows' else ' v1 abc')
                    result = subprocess.run(['bash', '-c', command], text=True, capture_output=True, timeout=10,
                        env=os.environ | dict(STATE_DIR=tmp, REPO_DIR=tmp, ORIGIN_REMOTE='origin', RUN_ID='test',
                            SCRIPT_DIR=str(SCRIPT.parent), VALIDATION_FAILURES_FILE=tmp + '/failures',
                            TEST_RUN_ID=run_id, TEST_UNCONFIRMED=unconfirmed, COLLECT_STATUS=collector_status))
                    self.assertEqual(result.returncode, 9, result.stderr)
                    self.assertFalse((Path(tmp) / 'unsafe-repair').exists())
                    failures = (Path(tmp) / 'failures').read_text().splitlines()
                    self.assertEqual(len(failures), 2 if gate == 'wait_for_remote_workflows' else (3 if SCRIPT.name == 'sync-upstream.sh' else 1))
                    for entry in failures:
                        log = Path(entry.split('日志：')[1].removesuffix('）')).read_text()
                        self.assertIn('query diagnostics', log)

    def test_platform_block_stops_other_gates_and_uses_actual_error_without_codex(self):
        for gate in ('wait_for_remote_workflows', 'wait_for_release_workflow'):
            with self.subTest(gate=gate), tempfile.TemporaryDirectory() as tmp:
                command = '''set -eu
log() { :; }
require_command() { :; }
git() { echo git@github.com:owner/project.git; }
fail() { generate_failure_summary; echo "$*" >&2; exit 9; }
abort_sync() { fail "$@"; }
repair_remote_candidate() { exit 99; }
wait_for_remote_workflow() {
  echo "$1" >> "$STATE_DIR/gates"
  REMOTE_PLATFORM_BLOCKED=true
  REMOTE_FAILED_RUN_ID=''
  REMOTE_WORKFLOW_UNCONFIRMED=true
  REMOTE_WORKFLOW_DIAGNOSTIC_LOG="$STATE_DIR/query"
  echo 'Actions has been disabled for this repository' > "$REMOTE_WORKFLOW_DIAGNOSTIC_LOG"
  return 1
}
''' + functions('generate_failure_summary', gate) + '\n' + gate + (' abc' if gate == 'wait_for_remote_workflows' else ' v1 abc')
                result = subprocess.run(['bash', '-c', command], text=True, capture_output=True,
                    env=os.environ | dict(STATE_DIR=tmp, RUN_ID='test', REPO_DIR=tmp,
                        ORIGIN_REMOTE='origin', VALIDATION_FAILURES_FILE=tmp + '/failures',
                        FAILURE_SUMMARY_FILE=tmp + '/summary', PUSHED_COMMIT='abc', MERGED_COMMIT='abc'))
                self.assertEqual(result.returncode, 9, result.stderr)
                self.assertEqual(len((Path(tmp) / 'gates').read_text().splitlines()), 1)
                self.assertIn('Actions has been disabled', (Path(tmp) / 'summary').read_text())
                self.assertIn('abc', (Path(tmp) / 'summary').read_text())


class WorkflowStartupTest(unittest.TestCase):
    def setUp(self):
        path = SCRIPT.parent / 'start-workflow.py' if SCRIPT.name == 'sync-upstream.sh' else Path(__file__).with_name('upstream-start-workflow.py')
        spec = importlib.util.spec_from_file_location('startup', path)
        self.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.module)
        self.args = SimpleNamespace(repo='owner/repo', directory='/repo', remote='origin',
                                    workflow='CI', commit='a' * 40, ref='main')
        self.workflow = dict(id=42, name='CI', state='active', path='.github/workflows/ci.yml')

    def responses(self, *, state='active', refs=None, dispatch=True):
        return [json.dumps({'workflows': [self.workflow | {'state': state}]}),
                refs if refs is not None else 'a' * 40 + '\trefs/heads/main\n',
                'on:\n  workflow_dispatch:\n' if dispatch else 'on:\n  push:\n', '']

    def test_dispatch_requires_exact_candidate_ref(self):
        with patch.object(self.module, 'run', side_effect=self.responses()) as run:
            self.module.start(self.args)
        self.assertEqual(json.loads(run.call_args.kwargs['stdin']), {'ref': 'main'})
        for refs in ('b' * 40 + '\trefs/heads/main\n', '',
                     'a' * 40 + '\trefs/heads/main\n' + 'a' * 40 + '\trefs/tags/main\n'):
            with self.subTest(refs=refs), patch.object(self.module, 'run', side_effect=self.responses(refs=refs)) as run:
                with self.assertRaisesRegex(RuntimeError, 'ambiguous, missing or no longer'):
                    self.module.start(self.args)
                self.assertEqual(run.call_count, 2)

    def test_annotated_release_tag_dispatch_uses_full_release_inputs(self):
        self.args.workflow = 'Release'
        self.args.ref = 'v2026.10.13'
        self.workflow['name'] = 'Release'
        refs = 'b' * 40 + '\trefs/tags/v2026.10.13\n' + 'a' * 40 + '\trefs/tags/v2026.10.13^{}\n'
        with patch.object(self.module, 'run', side_effect=self.responses(refs=refs)) as run:
            self.module.start(self.args)
        self.assertEqual(json.loads(run.call_args.kwargs['stdin']), dict(ref='v2026.10.13',
            inputs=dict(tag='v2026.10.13', dry_run=False, simple_release=False)))

    def test_inactivity_is_reenabled_but_manual_disable_is_preserved(self):
        with patch.object(self.module, 'run', side_effect=self.responses(state='disabled_inactivity') + ['']) as run:
            self.module.start(self.args)
        self.assertIn('PUT', run.call_args_list[3].args)
        self.assertIn('/enable', run.call_args_list[3].args[-1])
        with patch.object(self.module, 'run', side_effect=self.responses(state='disabled_manually')) as run:
            with self.assertRaisesRegex(RuntimeError, 'owner review'):
                self.module.start(self.args)
        self.assertEqual(run.call_count, 3)

    def test_no_dispatch_trigger_does_not_mutate_refs_or_start_release(self):
        with patch.object(self.module, 'run', side_effect=self.responses(dispatch=False)) as run:
            with self.assertRaisesRegex(RuntimeError, 'does not support'):
                self.module.start(self.args)
        self.assertEqual(run.call_count, 3)

    def test_disabled_execution_response_is_distinct_from_other_api_errors(self):
        for message, error in [('Actions has been disabled for this repository', self.module.PlatformDisabled),
                               ('HTTP 503 unavailable', RuntimeError)]:
            with patch.object(self.module.subprocess, 'run', return_value=subprocess.CompletedProcess([], 1, '', message)):
                with self.assertRaises(error):
                    self.module.run('gh', 'api', 'dispatch')


class RemoteFailureLogTest(unittest.TestCase):
    def test_all_job_downloads_are_attempted_and_errors_are_reported(self):
        helper = SCRIPT.parent / 'remote-failure.py' if SCRIPT.name == 'sync-upstream.sh' else Path(__file__).with_name('upstream-remote-failure.py')
        spec = importlib.util.spec_from_file_location('remote_failure', helper)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        jobs = [dict(databaseId=n, name=f'job-{n}', conclusion='failure') for n in (1, 2)]
        with tempfile.TemporaryDirectory() as tmp:
            output = Path(tmp) / 'log'
            responses = [subprocess.CompletedProcess([], 0, json.dumps({'jobs': jobs}), ''),
                         subprocess.CompletedProcess([], 1, '', 'HTTP 503'),
                         subprocess.CompletedProcess([], 0, 'second job failure details', '')]
            with patch('sys.argv', [str(helper), 'owner/repo', '42', str(output)]), \
                    patch.object(module.subprocess, 'run', side_effect=responses) as run:
                with self.assertRaises(SystemExit) as error:
                    module.main()
                self.assertEqual(error.exception.code, 1)
                self.assertEqual(run.call_count, 3)
            self.assertIn('HTTP 503', output.read_text())
            self.assertIn('second job failure details', output.read_text())


if __name__ == '__main__':
    unittest.main()
