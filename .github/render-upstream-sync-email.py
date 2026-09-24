#!/usr/bin/env python3
"""Render sync reports and compose offline multipart emails (no SMTP access)."""

import argparse
import re
from datetime import datetime, timezone
from email.message import EmailMessage
from email.policy import SMTP
from email.utils import format_datetime, make_msgid
from html import escape
from pathlib import Path


SECTION_TITLES = {
    "具体失败原因", "最终失败检查", "Codex 合并审查及共享账号池排除记录", "Codex 合并审查及功能排除记录",
}
REVIEW_HEADINGS = {"上游变化", "影响范围", "合并后行为", "冲突处理", "共享账号池排除路径", "批量生图排除路径", "剩余风险"}
TEMPLATE = Path(__file__).with_name("upstream-sync-email.html")


def parse_report(text):
    metadata, sections = {}, {}
    current = None
    for line in text.splitlines():
        stripped = line.strip()
        if stripped in SECTION_TITLES:
            current = stripped
            sections[current] = []
        elif not stripped or re.fullmatch(r"[-=]{3,}", stripped):
            if current:
                sections[current].append("")
        elif stripped.startswith("完整日志："):
            metadata["完整日志"] = stripped.partition("：")[2].strip()
        elif current:
            sections[current].append(line)
        elif "：" in stripped:
            key, _, value = stripped.partition("：")
            metadata[key] = value.strip()
    return metadata, sections


def inline(text):
    # Treat diagnostic output as data. Never accept raw HTML or active links.
    tokens = re.split(r"(`[^`\n]+`|\*\*[^*\n]+\*\*)", text)
    rendered = []
    for token in tokens:
        if token.startswith("`") and token.endswith("`"):
            rendered.append('<code style="font-family:Consolas,monospace;font-size:12px;background:#f1f5f9;padding:2px 4px;word-break:break-all;">' + escape(token[1:-1]) + '</code>')
        elif token.startswith("**") and token.endswith("**"):
            rendered.append('<strong style="font-weight:600;color:#172b4d;">' + escape(token[2:-2]) + '</strong>')
        else:
            rendered.append(escape(token))
    return "".join(rendered)


def rich_text(lines):
    blocks, paragraph, code = [], [], None

    def flush():
        if paragraph:
            blocks.append('<p style="margin:0 0 12px;line-height:1.85;overflow-wrap:anywhere;">' + inline(" ".join(paragraph)) + '</p>')
            paragraph.clear()

    for line in lines:
        stripped = line.strip()
        if stripped.startswith("```"):
            flush()
            if code is None:
                code = []
            else:
                blocks.append('<pre style="white-space:pre-wrap;word-break:break-all;background:#f3f6fa;padding:14px;font-size:12px;line-height:1.8;">' + escape("\n".join(code)) + '</pre>')
                code = None
        elif code is not None:
            code.append(line)
        elif not stripped:
            flush()
        elif stripped in REVIEW_HEADINGS or re.match(r"^#{1,6}\s+", stripped):
            flush()
            blocks.append('<h3 style="margin:18px 0 10px;font-size:14px;line-height:24px;color:#172b4d;">' + inline(re.sub(r"^#{1,6}\s+", "", stripped)) + '</h3>')
        elif re.match(r"^(?:\d+[.)、]|[-*])\s+", stripped):
            flush()
            match = re.match(r"^(\d+[.)、]|[-*])\s+(.*)", stripped)
            marker, content = match.groups()
            marker = "•" if marker in ("-", "*") else marker.rstrip(".)、")
            inset = 'padding-left:18px;' if line.startswith(" ") else ''
            blocks.append(f'<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="table-layout:fixed;margin:0 0 10px;{inset}"><tr><td width="26" valign="top" style="color:#64748b;font-size:12px;line-height:25px;">{escape(marker)}</td><td style="font-size:14px;line-height:25px;overflow-wrap:anywhere;word-break:break-word;">{inline(content)}</td></tr></table>')
        else:
            paragraph.append(re.sub(r"^#{1,6}\s+", "", stripped))
    flush()
    if code is not None:
        blocks.append('<pre style="white-space:pre-wrap;word-break:break-all;">' + escape("\n".join(code)) + '</pre>')
    return "".join(blocks)


def plain_report(text):
    # The text alternative remains readable when HTML is disabled.
    text = re.sub(r"\*\*([^*\n]+)\*\*", r"\1", text)
    return re.sub(r"`([^`\n]+)`", r"\1", text)


def state_tone(value):
    if any(word in value for word in ("失败", "受阻", "超时")):
        return "#b42332", "#fff1f2"
    if any(word in value for word in ("进行中", "等待", "修复中", "发布中", "已触发", "待发布")):
        return "#87530d", "#fff8e6"
    if any(word in value for word in ("未", "不发布", "跳过", "无须", "没有")):
        return "#526277", "#f3f6fa"
    if any(word in value for word in ("通过", "成功", "已推送", "已完成")):
        return "#08775b", "#edf9f4"
    return "#526277", "#f3f6fa"


def table_rows(rows):
    return ''.join(
        f'<tr><td class="meta-label" width="112" valign="top" style="width:112px;padding:10px 14px 10px 0;border-bottom:1px solid #edf1f5;color:#58697e;font-size:13px;line-height:22px;">{escape(label)}</td><td valign="top" style="padding:10px 0;border-bottom:1px solid #edf1f5;color:#25364a;font-size:13px;line-height:22px;word-break:break-word;overflow-wrap:anywhere;">{escape(value)}</td></tr>'
        for label, value in rows if value
    )


def section(title, content):
    return f'<tr><td class="section" style="padding:0 30px 26px;"><h2 style="margin:0 0 12px;font-size:16px;line-height:24px;font-weight:700;color:#172b4d;">{escape(title)}</h2>{content}</td></tr>'


def metadata_table(rows):
    return '<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="width:100%;table-layout:fixed;border-collapse:collapse;">' + table_rows(rows) + '</table>'


def render_report(text, preview=False):
    data, sections = parse_report(text)
    status = data.get("结果", "状态未记录")
    failed = any(word in status for word in ("失败", "受阻"))
    success = status == "成功"
    pushed = data.get("已推送版本", "未记录")
    has_push = bool(re.fullmatch(r"[a-fA-F0-9]{7,64}", pushed))
    color, soft = ("#b42332", "#fff1f2") if failed else (("#08775b", "#edf9f4") if success else ("#315bc8", "#eff4ff"))
    remote_state = data.get("远程工作流", "未执行")
    release_state = data.get("版本发布", "未评估")
    release_failure = failed and has_push and "发布" in status and "失败" not in remote_state
    title = "版本发布失败" if release_failure else ("推送后检查失败" if failed and has_push else ("同步失败" if failed else ("同步完成" if success else status)))
    if failed and "主工作区不干净" in data.get("结论", ""):
        title = "同步受阻"
    subtitle = "代码已推送，请查看发布结果" if release_failure else ("代码已推送，远程检查与发布结果见下方" if has_push else ("本次候选代码未推送" if pushed == "未推送" else "推送状态未记录"))
    if "测试" in status:
        subtitle = "仅验证通知格式，不代表代码已合并"
    cards = [
        ("代码推送", "已推送" if has_push else pushed),
        ("本地验证", data.get("全量验证", "未记录")),
        ("远程 CI / 安全扫描", remote_state),
        ("版本发布", release_state),
    ]
    cards_html = ''
    for index, (label, value) in enumerate(cards):
        tone, background = state_tone(value)
        if index % 2 == 0:
            cards_html += '<tr>'
        cards_html += f'<td class="card" width="50%" valign="top" style="width:50%;padding:0 4px 8px;"><div style="padding:14px 16px;border:1px solid #e4eaf2;border-radius:6px;background:{background};"><div style="font-size:12px;line-height:20px;color:#58697e;">{escape(label)}</div><div style="margin-top:5px;font-size:14px;font-weight:600;line-height:23px;color:{tone};word-break:break-word;overflow-wrap:anywhere;">{escape(value)}</div></div></td>'
        if index % 2 == 1:
            cards_html += '</tr>'
    execution = [("执行时间", data.get("执行时间", "未记录")), ("当前阶段", data.get("失败/当前阶段", "未记录")), ("本地自动修复", data.get("Codex 集中修复次数", "未记录") + " 轮"), ("远程 CI 修复", data.get("远程 CI 自动修复次数", "未记录") + " 轮")]
    content = section("本次执行", metadata_table(execution))
    for name in ("具体失败原因", "最终失败检查"):
        lines = sections.get(name, [])
        if any(line.strip() for line in lines):
            content += section(name, '<div style="padding:14px 16px;background:#fff1f2;border-left:3px solid #b42332;font-size:14px;line-height:25px;color:#25364a;">' + rich_text(lines) + '</div>')
    release = [(key, data.get(key, "")) for key in ("版本标签", "发布判断")]
    if any(value for _, value in release):
        content += section("发布判断", metadata_table(release))
    for name in ("Codex 合并审查及共享账号池排除记录", "Codex 合并审查及功能排除记录"):
        lines = sections.get(name, [])
        if any(line.strip() for line in lines):
            content += section("合并审查与排除记录", '<div style="font-size:14px;line-height:25px;color:#42536b;">' + rich_text(lines) + '</div>')
    refs = [(key, data.get(key, "")) for key in ("目标分支", "主上游", "第二上游", "合并前版本", "主上游版本", "第二上游版本", "候选版本")]
    if has_push:
        refs.append(("已推送版本", pushed))
    if any(value for _, value in refs):
        content += section("同步范围与版本", metadata_table(refs))
    values = {
        "PROJECT": "sub2api", "REPORT_NAME": "双上游同步报告", "TITLE": escape(title),
        "STATUS": escape(status), "COLOR": color, "SOFT": soft,
        "SUBTITLE": escape(subtitle), "SUMMARY": inline(data.get('结论', '请查看下方执行记录')),
        "PREHEADER": escape(title + ' · ' + data.get('结论', '')[:160]),
        "PREVIEW": '<tr><td style="padding:12px;background:#fff8e6;color:#87530d;font-size:12px;text-align:center;">历史报告排版预览 · 未运行同步或发送邮件</td></tr>' if preview else '',
        "CARDS": cards_html, "CONTENT": content, "LOG": escape(data.get('完整日志', '未记录')),
        "FOOTER": "保留主上游与第二上游的审查记录，排除项以本次报告为准。",
    }
    # Single pass: diagnostic text resembling a placeholder remains literal.
    return re.sub(r"\{\{([A-Z_]+)\}\}", lambda match: values[match[1]], TEMPLATE.read_text(encoding="utf-8"))


def compose_message(text, html, subject, sender, recipient):
    message = EmailMessage(policy=SMTP)
    message["From"] = sender
    message["To"] = recipient
    message["Subject"] = subject
    message["Date"] = format_datetime(datetime.now(timezone.utc))
    message["Message-ID"] = make_msgid()
    message.set_content(plain_report(text), charset="utf-8", cte="quoted-printable")
    if not html or not html.strip():
        try:
            html = render_report(text)
        except OSError:
            # A missing template must not suppress the failure notification.
            html = '<!doctype html><html lang="zh-CN"><meta charset="utf-8"><body style="font-family:Arial,sans-serif;color:#25364a;"><h1>sub2api 双上游同步报告</h1><pre style="white-space:pre-wrap;word-break:break-all;">' + escape(plain_report(text)) + '</pre></body></html>'
    message.add_alternative(html, subtype="html", charset="utf-8", cte="quoted-printable")
    return message.as_bytes()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("render", "message"))
    parser.add_argument("--text", required=True, type=Path)
    parser.add_argument("--html", type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--preview", action="store_true")
    parser.add_argument("--subject")
    parser.add_argument("--sender")
    parser.add_argument("--recipient")
    args = parser.parse_args()
    text = args.text.read_text(encoding="utf-8")
    if args.mode == "render":
        args.output.write_text(render_report(text, args.preview), encoding="utf-8")
    else:
        if not all((args.subject, args.sender, args.recipient)):
            parser.error("message requires --subject, --sender and --recipient")
        html = args.html.read_text(encoding="utf-8") if args.html and args.html.is_file() else None
        args.output.write_bytes(compose_message(text, html, args.subject, args.sender, args.recipient))


if __name__ == "__main__":
    main()
