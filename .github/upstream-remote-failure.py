#!/usr/bin/env python3
"""Save failed GitHub job details and logs before attempting a local repair."""
import argparse
import json
import subprocess
from pathlib import Path


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("repo")
    parser.add_argument("run")
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    result = subprocess.run(["gh", "run", "view", args.run, "--repo", args.repo,
                             "--json", "headSha,status,conclusion,jobs"], text=True, capture_output=True)
    with args.output.open("w", encoding="utf-8") as output:
        output.write(f"GitHub run: https://github.com/{args.repo}/actions/runs/{args.run}\n")
        if result.returncode:
            output.write(result.stderr)
            raise SystemExit(result.returncode)
        data = json.loads(result.stdout)
        output.write(json.dumps(data, ensure_ascii=False, indent=2) + "\n")
        for job in data["jobs"]:
            if job.get("conclusion") in ("success", "skipped", None, ""):
                continue
            output.write("\nFailed job: " + job["name"] + "\n")
            log = subprocess.run(["gh", "api", f"repos/{args.repo}/actions/jobs/{job['databaseId']}/logs"],
                                 text=True, capture_output=True)
            output.write(log.stdout if log.returncode == 0 else "Log download failed: " + log.stderr)


if __name__ == "__main__":
    main()
