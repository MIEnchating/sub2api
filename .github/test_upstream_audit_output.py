import json
import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
VALIDATOR = ROOT / 'scripts/upstream-sync-validation.sh'


class AuditOutputTest(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.root = Path(tmp.name)
        self.repo = self.root / 'candidate'
        self.state = self.root / 'state'
        self.state.mkdir()
        for directory in ('frontend', 'tools', '.github'):
            (self.repo / directory).mkdir(parents=True)
        shutil.copy(ROOT / 'tools/check_pnpm_audit_exceptions.py', self.repo / 'tools')
        (self.repo / '.github/audit-exceptions.yml').write_text('version: 1\nexceptions: []\n')
        (self.repo / 'frontend/audit.json').write_text('tracked historical audit\n')
        self.env = dict(os.environ, WORKTREE=str(self.repo), STATE_DIR=str(self.state),
                        RUN_ID='test', VALIDATOR=str(VALIDATOR),
                        GIT_AUTHOR_NAME='Test', GIT_AUTHOR_EMAIL='test@example.test',
                        GIT_COMMITTER_NAME='Test', GIT_COMMITTER_EMAIL='test@example.test')
        self.git('init', '-q')
        self.git('add', '.')
        self.git('commit', '-qm', 'fixture')

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.repo), *args], env=self.env, text=True)

    def run_audit(self, payload):
        return subprocess.run(['bash', '-ec', '''source "$VALIDATOR"
frontend_pnpm() { printf '%s\n' "$PAYLOAD"; return 1; }
check_frontend_audit
check_validation_worktree
'''], env=dict(self.env, PAYLOAD=payload), capture_output=True, text=True)

    def test_audit_keeps_tracked_file_clean_and_preserves_each_report(self):
        payload = json.dumps({'advisories': {}, 'metadata': {'vulnerabilities': {
            'info': 0, 'low': 0, 'moderate': 0, 'high': 0, 'critical': 0}}})
        for _ in range(2):
            result = self.run_audit(payload)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.git('status', '--porcelain'), '')
        reports = list(self.state.glob('test-frontend-audit.*.json'))
        self.assertEqual(len(reports), 2)
        self.assertTrue(all(json.loads(p.read_text()) == json.loads(payload) for p in reports))
        self.assertEqual((self.repo / 'frontend/audit.json').read_text(), 'tracked historical audit\n')

    def test_audit_errors_and_unknown_vulnerabilities_still_fail(self):
        for payload in ('', '{', '{}', '{"error":{"code":"ECONNRESET"}}', json.dumps({
                'advisories': {'1': {'module_name': 'unknown', 'severity': 'high',
                                     'github_advisory_id': 'GHSA-test-unknown'}},
                'metadata': {'vulnerabilities': {'high': 1, 'critical': 0}}})):
            with self.subTest(payload=payload):
                result = self.run_audit(payload)
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertEqual(self.git('status', '--porcelain'), '')

    def test_dirty_validation_is_collected_before_repair(self):
        (self.repo / 'frontend/audit.json').write_text('unexpected modification\n')
        failures = self.state / 'failures'
        result = subprocess.run(['bash', '-ec', '''source "$VALIDATOR"
log() { :; }
record_check() {
  local label="$1"; shift
  [[ "$label" == '验证后工作树清洁检查' ]] || return 0
  "$@" || printf '%s\n' "$label" >> "$VALIDATION_FAILURES_FILE"
}
if run_validation_checks 1; then exit 99; fi
cat "$VALIDATION_FAILURES_FILE"
echo "$VALIDATION_RESULT"
'''], env=dict(self.env, VALIDATION_FAILURES_FILE=str(failures)), capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('验证后工作树清洁检查', result.stdout)
        self.assertIn('第 1 轮失败（1 项）', result.stdout)


if __name__ == '__main__':
    unittest.main()
