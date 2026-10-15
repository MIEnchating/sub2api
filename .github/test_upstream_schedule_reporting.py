"""Audit regressions shared with the host new-api synchronization script."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(os.environ.get('SYNC_TEST_SCRIPT', Path(__file__).resolve().parents[1] / 'scripts/upstream-sync.sh'))
IS_NEW = SCRIPT.name == 'sync-upstream.sh'


def functions(*names):
    source = SCRIPT.read_text()
    return '\n'.join(name + '() {' + source.split(name + '() {', 1)[1].split('\n}\n', 1)[0] + '\n}' for name in names)


class AuditTest(unittest.TestCase):
    def test_release_summary_does_not_confuse_intention_with_publication(self):
        cases = [
            ('SUCCESS', '已完成发布', 'true', '是，已发布 v2026.10.15'),
            ('SUCCESS', '待发布', 'true', '计划发布 v2026.10.15（尚未完成'),
            ('FAILED', '待发布', 'true', '否，本轮发布未完成'),
            ('BLOCKED', '发布中', 'true', '否，本轮发布未完成'),
            ('发布失败，代码已推送', '已触发发布', 'true', '否，本轮发布未完成'),
            ('SUCCESS', '不发布', 'false', '否（发布功能已关闭）'),
            ('SUCCESS', '不发布', 'true', '否（累计更新未达门槛）'),
            ('DRY_RUN', '已完成发布', 'true', '否（测试或试运行，不发布）'),
            ('PREVIEW', '已完成发布', 'true', '否（测试或试运行，不发布）'),
            ('邮件测试', '已完成发布', 'true', '否（测试或试运行，不发布）'),
        ]
        for status, state, enabled, expected in cases:
            with self.subTest(status=status, state=state):
                result = subprocess.run(['bash', '-c', 'set -eu\n' + functions('release_report_summary') + '\nrelease_report_summary "$STATUS"'],
                    env=os.environ | dict(STATUS=status, RELEASE_STATE=state, RELEASE_ENABLED=enabled,
                        RELEASE_TAG='v2026.10.15', RELEASE_REASON='累计更新未达门槛'),
                    text=True, capture_output=True, check=True)
                self.assertIn(expected, result.stdout)

    def test_actual_reports_show_publication_result_and_start_time(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            env = os.environ | dict(SCRIPT_DIR=str(SCRIPT.parent), NODE_BIN='node',
                REPORT_FILE=tmp + '/report.txt', HTML_REPORT_FILE=tmp + '/report.html',
                RUN_STARTED_AT='2026-10-15 00:30:00', RELEASE_TAG='v2026.10.15',
                RELEASE_ENABLED='true', RELEASE_REASON='累计更新未达门槛',
                MERGED_COMMIT='abc1234', PUSHED_COMMIT='abc1234',
                FAILURE_SUMMARY_FILE=tmp + '/failure', VALIDATION_FAILURES_FILE=tmp + '/checks',
                REVIEW_SUMMARY_FILE=tmp + '/review', REVIEW_DECISION_FILE=tmp + '/review.json',
                DECISION_FILE=tmp + '/decision.json', NOTIFIED='false', PUSH_STATE='通过')
            (root / 'failure').write_text('Actions has been disabled for this repository')
            (root / 'checks').write_text('CI 工作流未能启动')
            source = 'set -e\nlog() { :; }\n' + functions('release_report_summary', 'write_report')
            for status, state, expected in [
                ('SUCCESS' if IS_NEW else '成功', '已完成发布', '是，已发布 v2026.10.15'),
                ('SUCCESS' if IS_NEW else '成功', '不发布', '否（累计更新未达门槛）'),
                ('FAILED' if IS_NEW else '发布失败，代码已推送', '待发布', '否，本轮发布未完成'),
            ]:
                with self.subTest(state=state):
                    subprocess.run(['bash', '-c', source + '\nwrite_report "$STATUS" "报告回归测试"'],
                        env=env | dict(STATUS=status, RELEASE_STATE=state), capture_output=True, check=True)
                    for name in ('report.txt', 'report.html'):
                        report = (root / name).read_text()
                        self.assertIn('本次是否发布新版本', report)
                        self.assertIn(expected, report)
                        self.assertIn('2026-10-15 00:30:00', report)
            if IS_NEW:
                # Go through notification entrypoint, not just a hand-picked writer status.
                command = source + '\n' + functions('notify_failure') + '''
generate_failure_summary() { :; }
send_email() { :; }
notify_failure '远程检查受阻'
'''
                subprocess.run(['bash', '-c', command], env=env | dict(RELEASE_STATE='待发布'),
                    capture_output=True, check=True)
                for name in ('report.txt', 'report.html'):
                    report = (root / name).read_text()
                    self.assertIn('Actions has been disabled', report)
                    self.assertIn('CI 工作流未能启动', report)
                    self.assertNotIn('本次候选代码未推送', report)

    @unittest.skipIf(IS_NEW, 'sub2api configurable validation gate')
    def test_disabled_validation_cannot_authorize_push(self):
        result = subprocess.run(['bash', '-c', 'set -eu\n' + functions('run_validation_with_repairs') +
            '\nrun_validation_with_repairs\n[[ "$VALIDATION_SUCCEEDED" == false ]]'],
            env=os.environ | dict(VALIDATE='false', VALIDATION_SUCCEEDED='true'), capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)

    @unittest.skipUnless(IS_NEW, 'new-api multiple Go modules')
    def test_full_validation_collects_failure_and_continues_all_modules(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for module in ('.', 'relaykit', 'tokenkit'):
                folder = root / module
                folder.mkdir(exist_ok=True)
                (folder / 'go.mod').write_text('module example.org/test\n\ngo 1.26\n')
            (root / 'failures').touch()
            command = '''set -eu
log() { :; }
start_database_services() { return 1; }
docker() { printf '%s\\n' "$*" >> "$STATE_DIR/commands"; }
''' + functions('backend_go_image', 'run_validation_command', 'run_backend_checks', 'run_frontend_checks') + '''
run_frontend_checks
run_backend_checks
[[ "$BACKEND_STATE" == 失败 ]]
'''
            result = subprocess.run(['bash', '-c', command], capture_output=True, text=True,
                env=os.environ | dict(WORKTREE=tmp, STATE_DIR=tmp, GO_IMAGE='', BUN_IMAGE='oven/bun:latest',
                    RUN_ID='test', VALIDATION_FAILURES_FILE=tmp + '/failures', DB_NETWORK='test',
                    TEST_MYSQL_DSN='fixture', TEST_POSTGRES_DSN='fixture'))
            self.assertEqual(result.returncode, 0, result.stderr)
            commands = (root / 'commands').read_text()
            for expected in ('bun run test --maxWorkers=2', 'bun run build',
                             '/src/. golang:1.26 go build ./...',
                             '/src/relaykit golang:1.26 go build ./...',
                             '/src/tokenkit golang:1.26 go test ./...',
                             '/src/tokenkit golang:1.26 go vet ./...',
                             '/src/tokenkit golang:1.26 go build ./...'):
                self.assertIn(expected, commands)
            self.assertIn('启动数据库验证环境', (root / 'failures').read_text())

    @unittest.skipUnless(IS_NEW, 'new-api multiple Go modules')
    def test_go_image_tracks_highest_module_and_repair_changes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'go.mod').write_text('module example.org/root\n\ngo 1.26\n')
            (root / 'tokenkit').mkdir()
            module = root / 'tokenkit/go.mod'
            module.write_text('module example.org/tokenkit\n\ngo 1.27.1\ntoolchain go1.27.2\n')
            def image(pin=''):
                return subprocess.check_output(['bash', '-c', functions('backend_go_image') + '\nbackend_go_image'],
                    env=os.environ | dict(WORKTREE=tmp, GO_IMAGE=pin), text=True).strip()
            self.assertEqual(image(), 'golang:1.27.2')
            module.write_text('module example.org/tokenkit\n\ngo 1.28\n')
            self.assertEqual(image(), 'golang:1.28')
            self.assertEqual(image('custom-go:latest'), 'custom-go:latest')


if __name__ == '__main__':
    unittest.main()
