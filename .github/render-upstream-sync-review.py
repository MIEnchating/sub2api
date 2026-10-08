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


def compact_review(decision: dict) -> str:
    """Keep notifications short; the full decision and release notes stay intact."""
    lines = ["自动审查摘要", f"- {decision['summary']}"]
    for title, key, limit in (
        ("关键上游变化", "upstream_changes", 5),
        ("兼容性核对", "impacts", 3),
        ("合并后行为", "merged_behavior", 4),
        ("冲突处理", "conflicts", 3),
    ):
        items = decision.get(key, [])
        if not items or items == ["无"]:
            continue
        add_section(lines, title, items[:limit])
        if len(items) > limit:
            lines.append(f"- 其余 {len(items) - limit} 项见完整审查记录。")
    lines.extend(("", "功能排除"))
    for label, key in (("共享账号池", "excluded_shared_account_pool_paths"),
                       ("批量生图", "excluded_batch_image_paths")):
        items = decision.get(key, [])
        lines.append(f"- {label}：本轮记录 {len(items)} 项排除路径，明细见完整审查记录。")
    # Never truncate risks; every unresolved finding must remain visible.
    add_section(lines, "仍需关注的风险", decision.get("risks", []))
    return "\n".join(lines)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--release-tag")
    parser.add_argument("--compact", action="store_true")
    parser.add_argument("decision", type=Path)
    args = parser.parse_args()

    decision = json.loads(args.decision.read_text(encoding="utf-8"))
    if args.compact:
        print(compact_review(decision))
        return
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
    add_section(lines, "批量生图排除路径", decision.get("excluded_batch_image_paths", []))
    add_section(lines, "剩余风险", decision["risks"])
    print("\n".join(lines))


if __name__ == "__main__":
    main()
