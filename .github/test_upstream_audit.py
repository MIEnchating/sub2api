"""Keep incomplete dependency audits from passing the release gate."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


CHECKER = Path(__file__).resolve().parents[1] / "tools/check_pnpm_audit_exceptions.py"


class AuditReportTest(unittest.TestCase):
    def check_report(self, report):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            audit = root / "audit.json"
            audit.write_text(report)
            exceptions = root / "exceptions.yml"
            exceptions.write_text("version: 1\nexceptions: []\n")
            return subprocess.run(
                [sys.executable, str(CHECKER), "--audit", str(audit),
                 "--exceptions", str(exceptions)],
                capture_output=True, text=True,
            )

    def test_rejects_failed_empty_and_malformed_reports(self):
        reports = [
            "", "{", "null", "[]", "{}",
            '{"error":{"code":"EAI_AGAIN"}}',
            '{"advisories":{},"error":{}}',
            '{"advisories":[]}', '{"vulnerabilities":null}',
        ]
        for report in reports:
            with self.subTest(report=report):
                result = self.check_report(report)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("Invalid audit report", result.stderr)

    def test_accepts_successful_clean_reports(self):
        for key in ("advisories", "vulnerabilities"):
            with self.subTest(key=key):
                result = self.check_report(json.dumps({key: {}}))
                self.assertEqual(result.returncode, 0, result.stderr)

    def test_still_rejects_high_severity_without_exception(self):
        report = {"advisories": {"1": {
            "module_name": "test-package", "severity": "high",
            "github_advisory_id": "GHSA-test", "title": "test vulnerability",
        }}}
        result = self.check_report(json.dumps(report))
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("missing exceptions", result.stdout + result.stderr)

    def test_upstream_exception_remains_accepted_after_review_date(self):
        report = {"advisories": {"1": {
            "module_name": "upstream-package", "severity": "high",
            "github_advisory_id": "GHSA-upstream", "title": "upstream vulnerability",
        }}}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            audit = root / "audit.json"
            audit.write_text(json.dumps(report))
            exceptions = root / "exceptions.yml"
            exceptions.write_text("""version: 1
exceptions:
  - package: upstream-package
    advisory: GHSA-upstream
    origin: upstream
    severity: high
    mitigation: tracked by upstream
    expires_on: "2020-01-01"
""")
            result = subprocess.run(
                [sys.executable, str(CHECKER), "--audit", str(audit),
                 "--exceptions", str(exceptions)],
                capture_output=True, text=True,
            )
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_expired_local_exception_still_blocks(self):
        report = {"advisories": {"1": {
            "module_name": "local-package", "severity": "high",
            "github_advisory_id": "GHSA-local", "title": "local vulnerability",
        }}}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            audit = root / "audit.json"
            audit.write_text(json.dumps(report))
            exceptions = root / "exceptions.yml"
            exceptions.write_text("""version: 1
exceptions:
  - package: local-package
    advisory: GHSA-local
    severity: high
    mitigation: tracked locally
    expires_on: "2020-01-01"
""")
            result = subprocess.run(
                [sys.executable, str(CHECKER), "--audit", str(audit),
                 "--exceptions", str(exceptions)],
                capture_output=True, text=True,
            )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Exceptions expired", result.stderr)

    def test_rejects_incomplete_vulnerability_entries(self):
        reports = [
            {"advisories": {"1": None}},
            {"advisories": {"1": {"module_name": "test-package"}}},
            {"advisories": {"1": {"severity": "critical"}}},
            {"advisories": {"1": {"module_name": "test-package", "severity": 1}}},
            {"advisories": {"1": {"module_name": "test-package", "severity": "unknown"}}},
            {"vulnerabilities": {"test-package": None}},
        ]
        for via in (None, [], {}, "", [None], [{}]):
            reports.append({"vulnerabilities": {"test-package": {
                "severity": "critical", "via": via,
            }}})
        for report in reports:
            with self.subTest(report=report):
                result = self.check_report(json.dumps(report))
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("Invalid audit report", result.stderr)
                self.assertNotIn("Traceback", result.stderr)

    def test_numeric_source_is_checked_without_a_title(self):
        result = self.check_report(json.dumps({"vulnerabilities": {"test-package": {
            "severity": "high", "via": [{"source": 123}],
        }}}))
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("missing exceptions", result.stderr)
        self.assertIn("[123]", result.stderr)
        self.assertNotIn("Traceback", result.stderr)

    def test_indirect_high_severity_is_not_skipped(self):
        result = self.check_report(json.dumps({"vulnerabilities": {"test-package": {
            "severity": "high", "via": ["affected-dependency"],
        }}}))
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("missing exceptions", result.stderr)

    def test_complete_low_severity_results_remain_allowed(self):
        reports = [
            {"advisories": {"1": {
                "module_name": "test-package", "severity": "low", "id": 123,
            }}},
            {"vulnerabilities": {"test-package": {
                "severity": "moderate", "via": [{"source": 123}],
            }}},
        ]
        for report in reports:
            with self.subTest(report=report):
                result = self.check_report(json.dumps(report))
                self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main()
