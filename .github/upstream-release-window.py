#!/usr/bin/env python3
"""Resolve a published release baseline and measure the accumulated candidate delta."""
import argparse
import json
import re
import subprocess
import sys


def git(repo, *args):
    return subprocess.check_output(["git", "-C", repo, *args], text=True).strip()


def release_baseline(repo, remote, head, releases):
    published = sorted(
        (item for item in releases if not item.get("draft") and not item.get("prerelease")
         and item.get("published_at") and item.get("assets")),
        key=lambda item: item["published_at"], reverse=True,
    )
    for item in published:
        tag = item["tag_name"]
        subprocess.run(["git", "check-ref-format", "refs/tags/" + tag], check=True, capture_output=True)
        ref = "refs/upstream-sync/releases/" + tag
        # Read tags from the fork remote, never a same-named upstream/local tag.
        subprocess.run(["git", "-C", repo, "fetch", "--no-tags", remote,
                        "+refs/tags/" + tag + ":" + ref], check=True, stdout=sys.stderr)
        commit = git(repo, "rev-parse", ref + "^{commit}")
        # Release commits may live on a release branch or predate a rewritten
        # integration history. Compare the actual shipped tree, not an older tag.
        git(repo, "merge-base", commit, head)
        return tag, commit
    # Initial release: retain the original repository baseline across all syncs.
    return "initial", git(repo, "rev-list", "--first-parent", "--max-parents=0", head)


def measure(repo, baseline, upstreams, candidate=None):
    count = git(repo, "rev-list", "--count", "--no-merges", *upstreams, "--not", baseline)
    diff_args = [baseline] + ([candidate] if candidate else [])
    names = subprocess.check_output(["git", "-C", repo, "diff", "--name-only", "-z", *diff_args])
    stats = subprocess.check_output(["git", "-C", repo, "diff", "--numstat", "-z", *diff_args])
    lines = 0
    for record in stats.split(b"\0"):
        fields = record.split(b"\t", 2)
        if len(fields) == 3:
            lines += sum(int(value) for value in fields[:2] if value.isdigit())
    return int(count), len([name for name in names.split(b"\0") if name]), lines


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=["baseline", "measure", "notes"])
    parser.add_argument("--repo", required=True)
    parser.add_argument("--remote", default="origin")
    parser.add_argument("--head", default="HEAD")
    parser.add_argument("--baseline")
    parser.add_argument("--candidate")
    parser.add_argument("--upstream", action="append", default=[])
    args = parser.parse_args()
    if args.mode == "baseline":
        url = git(args.repo, "remote", "get-url", "--push", args.remote)
        match = re.fullmatch(r"(?:git@github\.com:|https://github\.com/)([^/]+/[^/]+?)(?:\.git)?", url)
        if not match:
            raise ValueError("Unsupported GitHub remote: cannot determine release repository")
        rows = subprocess.check_output(
            ["gh", "api", "--paginate", f"repos/{match[1]}/releases?per_page=100",
             "--jq", '.[] | {tag_name,draft,prerelease,published_at,assets} | @json'], text=True)
        tag, commit = release_baseline(args.repo, args.remote, args.head,
                                       [json.loads(row) for row in rows.splitlines() if row])
        print(tag, commit)
    elif args.mode == "measure":
        if not args.baseline or not args.upstream:
            parser.error("measure requires --baseline and --upstream")
        print(*measure(args.repo, args.baseline, args.upstream, args.candidate))
    else:
        if not args.baseline:
            parser.error("notes requires --baseline")
        print("\n## 自上次正式发布以来的累计提交\n")
        # First-parent history lists integrations once instead of duplicating every upstream commit.
        print(git(args.repo, "log", "--first-parent", "--format=- %h %s",
                  args.baseline + ".." + (args.candidate or args.head)))
        print("\n本次候选包含上述已合并但尚未正式发布的更新，以及本报告审查的新增变更。")


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, RuntimeError, subprocess.CalledProcessError) as error:
        print(f"Release baseline error: {error}", file=sys.stderr)
        sys.exit(1)
