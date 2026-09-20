#!/usr/bin/env python3

import argparse
import json
from pathlib import Path


def add_section(lines: list[str], title: str, items: list[str]) -> None:
    lines.extend(("", title))
    if items:
        lines.extend(f"- {item}" for item in items)
    else:
        lines.append("- 无")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--release-tag")
    parser.add_argument("decision", type=Path)
    args = parser.parse_args()

    decision = json.loads(args.decision.read_text(encoding="utf-8"))
    lines: list[str] = []
    if args.release_tag:
        lines.extend(
            (
                f"Release {args.release_tag}",
                "",
                "本版本包含经过双上游兼容审查和完整验证的更新。",
                decision["summary"],
            )
        )
    else:
        lines.extend(
            (
                f"审查结论：{'允许进入验证' if decision['decision'] == 'resolved' else '停止合并'}",
                f"摘要：{decision['summary']}",
            )
        )

    add_section(lines, "上游变化", decision["upstream_changes"])
    add_section(lines, "影响范围", decision["impacts"])
    add_section(lines, "合并后行为", decision["merged_behavior"])
    add_section(lines, "冲突处理", decision["conflicts"])
    add_section(lines, "共享账号池排除路径", decision["excluded_shared_account_pool_paths"])
    add_section(lines, "剩余风险", decision["risks"])
    print("\n".join(lines))


if __name__ == "__main__":
    main()
