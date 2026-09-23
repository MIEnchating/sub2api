import os
import re
import subprocess
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).resolve().parents[1] / "scripts/upstream-sync.sh"


class ValidationLogTest(unittest.TestCase):
    def test_each_check_and_retry_preserves_its_own_log(self):
        shell = SCRIPT.read_text()
        function = "record_check() {" + shell.split("record_check() {", 1)[1].split("\n}\n", 1)[0] + "\n}"
        with tempfile.TemporaryDirectory(prefix="sync-validation-test-") as tmp:
            failures = Path(tmp) / "failures.txt"
            result = subprocess.run(
                ["bash", "-c", "set -Eeuo pipefail\n"
                 "log() { :; }\n" + function + "\n"
                 "record_check '前端完整测试' bash -c 'echo frontend-failure >&2; exit 1'\n"
                 "record_check '部署脚本验证' printf 'deployment-success\\n'\n"
                 "record_check '前端完整测试' bash -c 'echo retry-failure >&2; exit 2'\n"
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


if __name__ == "__main__":
    unittest.main()
