import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).resolve().parents[1] / "scripts/upstream-sync.sh"


class SyncWorktreeTest(unittest.TestCase):
    def setUp(self):
        folder = tempfile.TemporaryDirectory(prefix="sync-worktree-test-")
        self.addCleanup(folder.cleanup)
        self.root = Path(folder.name)
        self.repo = self.root / "repo"
        self.remote = self.root / "remote.git"
        self.env = os.environ.copy()
        for key in list(self.env):
            if key.startswith("GIT_"):
                del self.env[key]
        self.env.update({
            "GIT_CONFIG_GLOBAL": os.devnull,
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_AUTHOR_NAME": "Sync Test",
            "GIT_AUTHOR_EMAIL": "sync@example.test",
            "GIT_COMMITTER_NAME": "Sync Test",
            "GIT_COMMITTER_EMAIL": "sync@example.test",
            "GIT_TERMINAL_PROMPT": "0",
            "REPO_DIR": str(self.repo),
            "TARGET_BRANCH": "main",
            "ORIGIN_REMOTE": "origin",
            "ORIGIN_REF": "origin/main",
            "DRY_RUN": "false",
        })
        self.git(self.root, "init", "--bare", "--initial-branch=main", str(self.remote))
        self.git(self.root, "init", "--initial-branch=main", str(self.repo))
        (self.repo / "tracked.txt").write_text("base\n")
        (self.repo / "deleted.txt").write_text("delete me\n")
        (self.repo / ".gitignore").write_text("ignored.txt\n")
        self.git(self.repo, "add", "--all")
        self.git(self.repo, "commit", "-m", "initial")
        self.git(self.repo, "remote", "add", "origin", str(self.remote))
        self.git(self.repo, "push", "-u", "origin", "main")
        self.initial = self.git(self.repo, "rev-parse", "HEAD")
        shell = SCRIPT.read_text()
        self.functions = "\n".join(
            name + "() {" + shell.split(name + "() {", 1)[1].split("\n}\n", 1)[0] + "\n}"
            for name in ("validate_primary_worktree", "prepare_primary_worktree")
        )

    def git(self, repo, *args):
        return subprocess.run(
            ["git", "-C", str(repo), *args], env=self.env, check=True,
            capture_output=True, text=True,
        ).stdout.strip()

    def prepare(self, dry_run=False):
        env = self.env | {"DRY_RUN": "true" if dry_run else "false"}
        return subprocess.run(
            ["bash", "-c", 'set -Eeuo pipefail\n'
             'log() { printf "%s\\n" "$*"; }\n'
             'fail() { printf "%s\\n" "$*" >&2; exit 1; }\n'
             + self.functions + "\nprepare_primary_worktree"],
            cwd=self.root, env=env, capture_output=True, text=True,
        )

    def advance_remote(self, filename="remote.txt"):
        peer = self.root / "peer"
        self.git(self.root, "clone", str(self.remote), str(peer))
        (peer / filename).write_text("remote change\n")
        self.git(peer, "add", "--all")
        self.git(peer, "commit", "-m", "remote update")
        self.git(peer, "push", "origin", "main")
        self.git(self.repo, "fetch", "origin")
        return self.git(peer, "rev-parse", "HEAD")

    def test_saves_all_local_changes_and_pushes_only_this_repository(self):
        other = self.root / "other"
        self.git(self.root, "clone", str(self.remote), str(other))
        (other / "tracked.txt").write_text("other project work\n")
        (self.repo / "tracked.txt").write_text("staged\n")
        self.git(self.repo, "add", "tracked.txt")
        (self.repo / "tracked.txt").write_text("latest local work\n")
        (self.repo / "new.txt").write_text("untracked work\n")
        (self.repo / "ignored.txt").write_text("local only\n")
        (self.repo / "deleted.txt").unlink()

        result = self.prepare()
        self.assertEqual(result.returncode, 0, result.stderr)
        head = self.git(self.repo, "rev-parse", "HEAD")
        self.assertEqual(self.git(self.remote, "rev-parse", "main"), head)
        self.assertEqual(self.git(self.repo, "status", "--porcelain"), "")
        self.assertEqual(self.git(self.remote, "show", "main:tracked.txt"), "latest local work")
        self.assertEqual(self.git(self.remote, "show", "main:new.txt"), "untracked work")
        files = self.git(self.remote, "ls-tree", "-r", "--name-only", "main").splitlines()
        self.assertNotIn("deleted.txt", files)
        self.assertNotIn("ignored.txt", files)
        self.assertEqual(self.git(other, "rev-parse", "HEAD"), self.initial)
        self.assertEqual((other / "tracked.txt").read_text(), "other project work\n")
        self.assertEqual(self.prepare().returncode, 0)
        self.assertEqual(self.git(self.repo, "rev-parse", "HEAD"), head)

    def test_pushes_existing_commit(self):
        self.git(self.repo, "commit", "--allow-empty", "-m", "local update")
        result = self.prepare()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.git(self.remote, "rev-parse", "main"), self.git(self.repo, "rev-parse", "HEAD"))

    def test_fast_forwards_clean_local_branch(self):
        remote_head = self.advance_remote()
        result = self.prepare()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.git(self.repo, "rev-parse", "HEAD"), remote_head)

    def test_preserves_local_snapshot_but_does_not_push_divergence(self):
        remote_head = self.advance_remote()
        (self.repo / "tracked.txt").write_text("local change\n")
        result = self.prepare()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("已分叉", result.stderr)
        self.assertEqual(self.git(self.remote, "rev-parse", "main"), remote_head)
        self.assertEqual(self.git(self.repo, "show", "HEAD:tracked.txt"), "local change")
        self.assertEqual(self.git(self.repo, "status", "--porcelain"), "")

    def test_refuses_unresolved_conflicts(self):
        self.advance_remote("tracked.txt")
        (self.repo / "tracked.txt").write_text("local change\n")
        self.git(self.repo, "commit", "-am", "local update")
        with self.assertRaises(subprocess.CalledProcessError):
            self.git(self.repo, "merge", "origin/main")
        before = self.git(self.repo, "ls-files", "--unmerged")
        result = self.prepare()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("尚未解决的 Git 冲突", result.stderr)
        self.assertEqual(self.git(self.repo, "ls-files", "--unmerged"), before)

    def test_refuses_unfinished_merge_even_without_conflicts(self):
        self.advance_remote()
        self.git(self.repo, "merge", "--no-ff", "--no-commit", "origin/main")
        result = self.prepare()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("未完成的 Git 操作", result.stderr)
        self.assertEqual(self.git(self.repo, "rev-parse", "HEAD"), self.initial)

    def test_refuses_wrong_branch(self):
        self.git(self.repo, "switch", "-c", "feature")
        (self.repo / "tracked.txt").write_text("unfinished work\n")
        result = self.prepare()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("预期为 main", result.stderr)
        self.assertEqual(self.git(self.repo, "rev-parse", "HEAD"), self.initial)

    def test_dry_run_does_not_commit_push_or_fast_forward(self):
        (self.repo / "tracked.txt").write_text("unfinished work\n")
        result = self.prepare(dry_run=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("试运行不会自动提交", result.stderr)
        self.assertEqual(self.git(self.repo, "rev-parse", "HEAD"), self.initial)
        self.git(self.repo, "commit", "-am", "local update")
        result = self.prepare(dry_run=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.git(self.remote, "rev-parse", "main"), self.initial)
        self.git(self.repo, "reset", "--hard", self.initial)
        self.advance_remote()
        result = self.prepare(dry_run=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.git(self.repo, "rev-parse", "HEAD"), self.initial)

    def test_failed_commit_stops_before_push(self):
        hook = self.repo / ".git/hooks/pre-commit"
        hook.write_text("#!/bin/sh\nexit 1\n")
        hook.chmod(0o755)
        (self.repo / "tracked.txt").write_text("local change\n")
        result = self.prepare()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.git(self.repo, "rev-parse", "HEAD"), self.initial)
        self.assertEqual(self.git(self.remote, "rev-parse", "main"), self.initial)
        self.assertEqual((self.repo / "tracked.txt").read_text(), "local change\n")


class ReviewDecisionTest(unittest.TestCase):
    def test_only_complete_resolved_review_is_accepted(self):
        root = SCRIPT.parents[1] / '.github'
        schema = json.loads((root / 'upstream-sync-decision-schema.json').read_text())
        valid = {key: ([] if spec['type'] == 'array' else '测试') for key, spec in schema['properties'].items()}
        valid['decision'] = 'resolved'
        cases = [
            (valid, 0),
            (dict(valid, decision='blocked'), 1),
            (dict(valid, conflicts=[{'files': []}]), 2),
            (dict(valid, risks=''), 2),
            ({key: value for key, value in valid.items() if key != 'excluded_batch_image_paths'}, 2),
            (dict(valid, verification=[]), 2),
        ]
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'decision.json'
            for decision, expected in cases:
                with self.subTest(decision=decision):
                    path.write_text(json.dumps(decision))
                    result = subprocess.run(['python3', str(root / 'validate-upstream-sync-decision.py'),
                        str(root / 'upstream-sync-decision-schema.json'), str(path)], capture_output=True)
                    self.assertEqual(result.returncode, expected, result.stderr)


if __name__ == "__main__":
    unittest.main()
