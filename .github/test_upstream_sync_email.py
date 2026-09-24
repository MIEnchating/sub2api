import importlib.util
import os
import subprocess
import tempfile
import unittest
from email import policy
from email.parser import BytesParser
from pathlib import Path
from unittest.mock import patch


SCRIPT_DIR = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("sync_email", SCRIPT_DIR / "render-upstream-sync-email.py")
email_report = importlib.util.module_from_spec(spec)
spec.loader.exec_module(email_report)

REPORT = """sub2api 双上游同步报告
========================

结果：失败，未推送
结论：主工作区不干净；请先处理本地修改
执行时间：2026-09-21 01:30:33（北京时间）
失败/当前阶段：检查主工作区
目标分支：origin/main
主上游：upstream/main
第二上游：overdraft/sub2api-custom（仅排除共享账号池）
已推送版本：未推送
全量验证：未执行
Codex 集中修复次数：0/3
版本发布：未评估

具体失败原因
------------
1. **失败检查：工作区检查。** 存在未提交修改
   - 涉及文件：`backend/server.go`

2. **直接原因：** 测试 <script>alert("unsafe")</script> & 配置

最终失败检查
------------
Go unit tests | exit 1 | TestExample

完整日志：/tmp/sync/example.log
"""


class SyncEmailTest(unittest.TestCase):
    def test_failure_report_formats_markdown_and_escapes_untrusted_content(self):
        html = email_report.render_report(REPORT)
        self.assertIn("同步受阻", html)
        self.assertIn("本次候选代码未推送", html)
        self.assertIn("<strong", html)
        self.assertIn("<code", html)
        self.assertNotIn("**失败检查", html)
        self.assertNotIn("`backend/server.go`", html)
        self.assertNotIn("<script>", html)
        self.assertIn("&lt;script&gt;", html)
        self.assertIn("TestExample", html)
        self.assertIn("0/3 轮", html)
        self.assertIn("overdraft/sub2api-custom", html)
        self.assertNotIn("历史报告排版预览", html)
        self.assertNotIn("{{REPORT_NAME}}", html)
        self.assertIn("sub2api · 双上游同步报告</title>", html)

    def test_post_push_failure_does_not_claim_code_was_not_pushed(self):
        text = REPORT.replace("失败，未推送", "发布失败，代码已推送")
        text = text.replace("主工作区不干净；请先处理本地修改", "发布工作流失败")
        text = text.replace("已推送版本：未推送", "已推送版本：abcdef1234567")
        html = email_report.render_report(text)
        self.assertIn("版本发布失败", html)
        self.assertIn("代码已推送，请查看发布结果", html)
        self.assertNotIn("本次候选代码未推送", html)
        self.assertNotIn("线上版本未发生变化", html)
        self.assertIn("abcdef1234567", html)

    def test_success_review_release_and_preview(self):
        text = """结果：成功
结论：双上游合并完成
已推送版本：1234567abcdef
全量验证：第 1 轮全部通过
版本发布：已触发发布
版本标签：v2026.09.21
发布判断：更新达到发布阈值

Codex 合并审查及共享账号池排除记录
----------------------------------
上游变化
- 修复查询异常
- 保留自定义渠道
完整日志：/tmp/sync/success.log
"""
        html = email_report.render_report(text, preview=True)
        for value in ("同步完成", "合并审查与排除记录", "修复查询异常", "v2026.09.21", "更新达到发布阈值", "历史报告排版预览"):
            self.assertIn(value, html)
        self.assertNotIn("具体失败原因</h2>", html)

    def test_mime_roundtrip_supports_chinese_and_plain_fallback(self):
        html = email_report.render_report(REPORT)
        raw = email_report.compose_message(REPORT, html, "【sub2api】同步结果", "bot@example.com", "owner@example.com")
        message = BytesParser(policy=policy.default).parsebytes(raw)
        self.assertEqual(message["Subject"], "【sub2api】同步结果")
        self.assertEqual(message.get_content_type(), "multipart/alternative")
        plain, rich = list(message.iter_parts())
        self.assertEqual(plain.get_content_type(), "text/plain")
        self.assertEqual(rich.get_content_type(), "text/html")
        self.assertIn("工作区检查", plain.get_content())
        self.assertNotIn("**", plain.get_content())
        self.assertEqual(rich.get_content().replace("\r\n", "\n").strip(), html.strip())
        self.assertTrue(all(len(line) <= 998 for line in raw.split(b"\r\n")))
        fallback = BytesParser(policy=policy.default).parsebytes(email_report.compose_message(REPORT, None, "失败", "bot@example.com", "owner@example.com"))
        self.assertEqual(fallback.get_content_type(), "multipart/alternative")
        self.assertIn("TestExample", fallback.get_body(preferencelist=("html",)).get_content())

    def test_remote_ci_failure_and_pending_release_remain_distinct(self):
        text = REPORT.replace("失败，未推送", "发布失败，代码已推送").replace(
            "主工作区不干净；请先处理本地修改", "远程 CI 自动修复后仍失败").replace(
            "已推送版本：未推送", "已推送版本：abcdef1234567")
        text = text.replace("版本发布：未评估", "远程工作流：失败，自动修复中\n远程 CI 自动修复次数：2/3\n版本发布：待发布\n发布判断：自 v1 累计，提交=5/3 文件=9/8 行=180/150")
        html = email_report.render_report(text)
        for value in ("推送后检查失败", "远程 CI / 安全扫描", "2/3 轮", "自 v1 累计", "提交=5/3"):
            self.assertIn(value, html)
        self.assertNotIn("版本发布失败</h1>", html)
        self.assertNotIn("本次候选代码未推送", html)
        self.assertEqual(email_report.state_tone("已触发发布"), ("#87530d", "#fff8e6"))
        self.assertEqual(email_report.state_tone("已完成发布"), ("#08775b", "#edf9f4"))
        self.assertEqual(email_report.state_tone("失败，自动修复中"), ("#b42332", "#fff1f2"))

    def test_placeholder_like_diagnostics_are_literal_and_missing_template_keeps_email(self):
        text = REPORT.replace('TestExample', '{{CONTENT}} <img src=x onerror=alert(1)>')
        html = email_report.render_report(text)
        self.assertIn('{{CONTENT}}', html)
        self.assertNotIn('<img', html)
        with patch.object(email_report, 'TEMPLATE', Path('/nonexistent/report-template.html')):
            message = BytesParser(policy=policy.default).parsebytes(email_report.compose_message(
                text, None, '失败报告', 'bot@example.com', 'owner@example.com'))
        self.assertIsNotNone(message.get_body(preferencelist=('plain',)))
        fallback = message.get_body(preferencelist=('html',)).get_content()
        self.assertIn('{{CONTENT}}', fallback)
        self.assertNotIn('<img', fallback)

    def test_scheduled_writer_renders_html_and_removes_stale_html_on_error(self):
        # Exercise the production report function without sourcing its main sync,
        # configuration, cleanup traps, SMTP or Git operations.
        shell = (SCRIPT_DIR.parent / "scripts/upstream-sync.sh").read_text()
        function = shell.split("write_report() {", 1)[1].split("\n}\n", 1)[0]
        with tempfile.TemporaryDirectory(prefix="sync-email-test-") as folder:
            root = Path(folder)
            env = os.environ.copy()
            env.update({
                "SCRIPT_DIR": str(SCRIPT_DIR.parent / "scripts"), "REPORT_FILE": str(root / "report.txt"),
                "HTML_REPORT_FILE": str(root / "report.html"), "CURRENT_STAGE": "检查主工作区",
                "ORIGIN_REF": "origin/main", "PRIMARY_REF": "upstream/main", "SECOND_REF": "overdraft/sub2api-custom",
                "ORIGIN_HEAD": "", "PRIMARY_HEAD": "", "SECOND_HEAD": "", "CANDIDATE_COMMIT": "", "PUSHED_COMMIT": "",
                "VALIDATION_RESULT": "未执行", "REPAIR_COUNT": "0", "VALIDATION_REPAIR_ATTEMPTS": "3",
                "RELEASE_STATE": "未评估", "RELEASE_TAG": "", "RELEASE_REASON": "",
                "FAILURE_SUMMARY_FILE": str(root / "failure.txt"), "VALIDATION_FAILURES_FILE": str(root / "checks.txt"),
                "REVIEW_SUMMARY_FILE": str(root / "review.txt"), "LOG_FILE": "/tmp/example.log",
            })
            command = 'set -eu\nlog() { printf "%s\\n" "$*"; }\nwrite_report() {' + function + '\n}\nwrite_report "失败，未推送" "主工作区不干净；请先处理本地修改"'
            subprocess.run(["bash", "-c", command], env=env, check=True, capture_output=True)
            self.assertIn("同步受阻", (root / "report.html").read_text())
            env["SCRIPT_DIR"] = str(root / "missing-renderer")
            result = subprocess.run(["bash", "-c", command], env=env, check=True, capture_output=True, text=True)
            self.assertIn("regenerate the HTML", result.stdout)
            self.assertTrue((root / "report.txt").is_file())
            self.assertFalse((root / "report.html").exists())


if __name__ == "__main__":
    unittest.main()
