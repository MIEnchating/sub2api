import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
VALIDATOR = ROOT / 'scripts/upstream-sync-validation.sh'


class BackendIntegrationValidationTest(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory(prefix='sync-integration-validator-')
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        (self.root / 'backend').mkdir()
        (self.root / 'backend/go.mod').write_text('module example.test/backend\n\ngo 1.27.3\n')
        binary = self.root / 'bin'
        binary.mkdir()
        self.calls = self.root / 'docker-calls.jsonl'
        docker = binary / 'docker'
        docker.write_text('''#!/usr/bin/env python3
import json
import os
from pathlib import Path
import sys

args = sys.argv[1:]
log = Path(os.environ['DOCKER_CALLS'])
previous = [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []
with log.open('a') as output:
    output.write(json.dumps(args) + '\\n')
mode = os.environ.get('DOCKER_SCENARIO', 'ready')
if args[0] == 'run':
    if 'postgres:18-alpine' in args:
        if mode == 'start_failure':
            print('fixture: cannot start database', file=sys.stderr)
            sys.exit(41)
        print('fixture-postgres')
    else:
        sys.exit(int(os.environ.get('GO_TEST_STATUS', '0')))
elif args[0] == 'exec':
    probes = sum(call[0] == 'exec' for call in previous)
    if '-h' not in args:
        # The temporary init server accepts Unix sockets, then stops. The old
        # validator mistakes it for readiness and fails its unguarded recheck.
        sys.exit(0 if probes == 0 else 1)
    assert args[args.index('-h') + 1] == '127.0.0.1', args
    sys.exit(1 if mode == 'timeout' or probes + 1 < int(os.environ.get('READY_AFTER', '3')) else 0)
elif args[0] == 'port':
    if mode == 'port_failure':
        sys.exit(42)
    print(os.environ.get('DATABASE_PORT', '127.0.0.1:54329'))
elif args[0] == 'logs':
    print('fixture: database startup diagnostic', file=sys.stderr)
elif args[0] == 'rm':
    sys.exit(int(os.environ.get('CLEANUP_STATUS', '0')))
else:
    raise AssertionError(args)
''')
        docker.chmod(0o755)
        sleep = binary / 'sleep'
        sleep.write_text('#!/bin/sh\nexit 0\n')
        sleep.chmod(0o755)
        self.env = os.environ | {
            'PATH': str(binary) + os.pathsep + os.environ['PATH'],
            'WORKTREE': str(self.root), 'TOOLS_DIR': str(self.root / 'tools'),
            'GO_IMAGE': '', 'DOCKER_CALLS': str(self.calls), 'VALIDATOR': str(VALIDATOR),
        }

    def run_check(self, command='check_backend_integration', **env):
        return subprocess.run(['bash', '-Eeuo', 'pipefail', '-c',
                               'source "$VALIDATOR"\n' + command],
                              env=self.env | env, text=True, capture_output=True, timeout=15)

    def docker_calls(self, operation):
        return [args for args in map(json.loads, self.calls.read_text().splitlines())
                if args[0] == operation]

    def assert_cleaned(self):
        self.assertEqual(self.docker_calls('rm'), [['rm', '-f', 'fixture-postgres']])

    def assert_tests_not_started(self):
        self.assertEqual(len(self.docker_calls('run')), 1)

    def test_waits_for_tcp_after_temporary_init_server_stops(self):
        result = self.run_check()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(self.docker_calls('exec')), 3)
        args = self.docker_calls('run')[-1]
        self.assertIn('--network', args)
        self.assertEqual(args[args.index('--network') + 1], 'host')
        self.assertIn('CI=true', args)
        self.assertIn('/var/run/docker.sock:/var/run/docker.sock', args)
        for key in ('MIGRATION_TEST_DATABASE_URL', 'QQBOT_TEST_DSN'):
            self.assertIn(key + '=postgres://postgres:integration-test@127.0.0.1:54329/sub2api_test?sslmode=disable', args)
        self.assertEqual(args[-5:], ['go', 'test', '-count=1', '-tags=integration', './...'])
        self.assert_cleaned()

    def test_last_readiness_attempt_is_accepted_without_extra_probe(self):
        result = self.run_check(READY_AFTER='30')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(self.docker_calls('exec')), 30)
        self.assert_cleaned()

    def test_timeout_reports_database_logs_and_does_not_start_tests(self):
        result = self.run_check(DOCKER_SCENARIO='timeout')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('PostgreSQL did not become TCP-ready', result.stderr)
        self.assertIn('fixture: database startup diagnostic', result.stderr)
        self.assertEqual(len(self.docker_calls('exec')), 30)
        self.assertEqual(self.docker_calls('logs'), [['logs', '--tail', '50', 'fixture-postgres']])
        # Keep failed containers until their diagnostics have been collected.
        self.assertNotIn('--rm', self.docker_calls('run')[0])
        self.assert_tests_not_started()
        self.assert_cleaned()

    def test_start_failure_has_stage_diagnostic_without_cleanup_of_unknown_container(self):
        result = self.run_check(DOCKER_SCENARIO='start_failure')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Failed to start integration PostgreSQL', result.stderr)
        self.assertIn('fixture: cannot start database', result.stderr)
        self.assertEqual(self.docker_calls('rm'), [])
        self.assert_tests_not_started()

    def test_port_lookup_failure_is_diagnosed_and_cleaned(self):
        result = self.run_check(DOCKER_SCENARIO='port_failure')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Failed to resolve integration PostgreSQL port', result.stderr)
        self.assert_tests_not_started()
        self.assert_cleaned()

    def test_invalid_port_mapping_never_reaches_tests(self):
        for port in ('', '0.0.0.0:54329', '127.0.0.1:0', '127.0.0.1:65536',
                     '127.0.0.1:abc', '127.0.0.1:54329\n127.0.0.1:54330'):
            with self.subTest(port=port):
                self.calls.unlink(missing_ok=True)
                result = self.run_check(DATABASE_PORT=port)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('Invalid integration PostgreSQL port mapping', result.stderr)
                self.assert_tests_not_started()
                self.assert_cleaned()

    def test_silent_test_failure_keeps_exit_code_even_if_cleanup_fails(self):
        result = self.run_check(GO_TEST_STATUS='23', CLEANUP_STATUS='43')
        self.assertEqual(result.returncode, 23, result.stderr)
        self.assertIn('go test -count=1 -tags=integration ./... failed (exit 23)', result.stderr)
        self.assert_cleaned()

    def test_record_check_keeps_diagnostics_and_continues_validation(self):
        source = (ROOT / 'scripts/upstream-sync.sh').read_text()
        record = 'record_check() {' + source.split('record_check() {', 1)[1].split('\n}\n', 1)[0] + '\n}'
        failures = self.root / 'failures.txt'
        result = self.run_check(record + '''
log() { :; }
record_check 'integration' check_backend_integration
record_check 'next' printf 'next check ran\\n'
wait
''', STATE_DIR=str(self.root), RUN_ID='test', VALIDATION_FAILURES_FILE=str(failures),
                                DOCKER_SCENARIO='timeout')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('next check ran', result.stdout)
        self.assertEqual(len(failures.read_text().splitlines()), 1)
        logs = list(self.root.glob('test-check-integration.log.*'))
        self.assertEqual(len(logs), 1)
        self.assertIn('PostgreSQL did not become TCP-ready', logs[0].read_text())
        self.assertIn('fixture: database startup diagnostic', logs[0].read_text())
        self.assert_cleaned()


if __name__ == '__main__':
    unittest.main()
