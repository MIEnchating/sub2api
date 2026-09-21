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
    "具体失败原因", "最终失败检查", "Codex 合并审查及共享账号池排除记录",
}
FONT = "-apple-system,BlinkMacSystemFont,'Segoe UI','PingFang SC','Microsoft YaHei',Arial,sans-serif"


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
        elif re.match(r"^(?:\d+[.)、]|[-*])\s+", stripped):
            flush()
            match = re.match(r"^(\d+[.)、]|[-*])\s+(.*)", stripped)
            marker, content = match.groups()
            marker = "•" if marker in ("-", "*") else marker.rstrip(".)、")
            inset = 'padding-left:18px;' if line.startswith(" ") else ''
            blocks.append(f'<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="margin:0 0 12px;{inset}"><tr><td width="26" valign="top" style="color:#64748b;font-size:12px;line-height:26px;">{escape(marker)}</td><td style="font-size:14px;line-height:26px;overflow-wrap:anywhere;word-break:break-word;">{inline(content)}</td></tr></table>')
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


def table_rows(rows):
    return ''.join(
        f'<tr><td width="98" valign="top" style="padding:10px 14px 10px 0;border-bottom:1px solid #edf1f5;color:#64748b;font-size:13px;line-height:22px;">{escape(label)}</td><td valign="top" style="padding:10px 0;border-bottom:1px solid #edf1f5;color:#24364b;font-size:13px;line-height:22px;word-break:break-all;">{escape(value)}</td></tr>'
        for label, value in rows if value
    )


def section(title, content):
    return f'<tr><td class="section" style="padding:0 30px 28px;"><h2 style="margin:0 0 14px;font-size:16px;line-height:24px;font-weight:600;color:#172b4d;">{escape(title)}</h2>{content}</td></tr>'


def render_report(text, preview=False):
    data, sections = parse_report(text)
    status = data.get("结果", "状态未记录")
    failed = any(word in status for word in ("失败", "受阻"))
    success = status == "成功"
    pushed = data.get("已推送版本", "未记录")
    has_push = bool(re.fullmatch(r"[a-fA-F0-9]{7,64}", pushed))
    color, soft = ("#b42332", "#fff1f2") if failed else (("#08775b", "#edf9f4") if success else ("#315bc8", "#eff4ff"))
    title = "版本发布失败" if failed and has_push else ("同步失败" if failed else ("同步完成" if success else status))
    if failed and "主工作区不干净" in data.get("结论", ""):
        title = "同步受阻"
    subtitle = "代码已推送，请查看发布结果" if has_push else ("本次候选代码未推送" if pushed == "未推送" else "推送状态未记录")
    if "测试" in status:
        subtitle = "仅验证通知格式，不代表代码已合并"
    cards = [
        ("代码推送", "已推送" if has_push else pushed, "#08775b" if has_push else "#475569"),
        ("全量验证", data.get("全量验证", "未记录"), "#475569"),
        ("版本发布", data.get("版本发布", "未记录"), "#475569"),
    ]
    cards_html = ''.join(f'<td width="33%" valign="top" style="padding:14px 12px;border:1px solid #e6ecf3;background:#f8fafc;"><div style="font-size:12px;color:#64748b;margin-bottom:7px;">{escape(label)}</div><div style="font-size:14px;font-weight:600;line-height:22px;color:{tone};word-break:break-word;">{escape(value)}</div></td>' for label, value, tone in cards)
    execution = [("执行时间", data.get("执行时间", "未记录")), ("当前阶段", data.get("失败/当前阶段", "未记录")), ("自动修复", data.get("Codex 集中修复次数", "未记录") + " 轮")]
    content = section("本次执行", '<table role="presentation" width="100%" cellspacing="0" cellpadding="0">' + table_rows(execution) + '</table>')
    for name in ("具体失败原因", "最终失败检查", "Codex 合并审查及共享账号池排除记录"):
        lines = sections.get(name, [])
        if any(line.strip() for line in lines):
            label = "合并审查与排除记录" if name.startswith("Codex") else name
            content += section(label, '<div style="font-size:14px;line-height:26px;color:#42536b;">' + rich_text(lines) + '</div>')
    refs = [(key, data.get(key, "")) for key in ("目标分支", "主上游", "第二上游", "合并前版本", "主上游版本", "第二上游版本", "候选版本")]
    if has_push:
        refs.append(("已推送版本", pushed))
    if any(value for _, value in refs):
        content += section("同步范围与版本", '<table role="presentation" width="100%" cellspacing="0" cellpadding="0">' + table_rows(refs) + '</table>')
    release = [(key, data.get(key, "")) for key in ("版本标签", "发布判断")]
    if any(value for _, value in release):
        content += section("发布说明", '<table role="presentation" width="100%" cellspacing="0" cellpadding="0">' + table_rows(release) + '</table>')
    preview_label = '<div style="padding:12px;background:#fff8df;color:#825b12;font-size:12px;text-align:center;">历史报告排版预览 · 未运行同步或发送邮件</div>' if preview else ''
    return f'''<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>sub2api · 双上游同步报告</title>
<style>@media only screen and (max-width:600px){{.outer{{padding:12px 8px!important}}.section{{padding-left:18px!important;padding-right:18px!important}}}}</style></head>
<body style="margin:0;background:#edf2f7;font-family:{FONT};color:#24364b;">
<div style="display:none;font-size:1px;line-height:1px;max-height:0;max-width:0;opacity:0;overflow:hidden;mso-hide:all;">{escape(title + ' · ' + data.get('结论', '')[:160])}</div>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#edf2f7;"><tr><td class="outer" align="center" style="padding:32px 12px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:720px;background:#fff;border:1px solid #dde5ef;border-radius:12px;overflow:hidden;">
<tr><td>{preview_label}</td></tr>
<tr><td class="section" style="padding:24px 30px;background:#172b4d;color:#fff;"><div style="font-size:20px;font-weight:700;letter-spacing:.3px;">sub2api <span style="font-size:12px;font-weight:400;color:#bdcbe1;">/ 自动同步</span></div><div style="margin-top:6px;font-size:12px;color:#bdcbe1;">双上游同步报告</div></td></tr>
<tr><td class="section" style="padding:26px 30px 22px;"><span style="display:inline-block;border-radius:5px;padding:5px 9px;font-size:12px;font-weight:600;color:{color};background:{soft};">{escape(status)}</span><h1 style="font-size:27px;line-height:36px;color:#172b4d;margin:13px 0 4px;">{escape(title)}</h1><p style="font-size:13px;line-height:22px;margin:0 0 18px;color:#64748b;">{escape(subtitle)}</p><div style="border-left:3px solid {color};padding:12px 15px;background:{soft};font-size:15px;line-height:26px;color:#24364b;overflow-wrap:anywhere;">{inline(data.get('结论', '请查看下方执行记录'))}</div></td></tr>
<tr><td class="section" style="padding:0 30px 26px;"><table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="table-layout:fixed;border-collapse:collapse;"><tr>{cards_html}</tr></table></td></tr>
{content}
<tr><td class="section" style="padding:20px 30px;background:#f8fafc;border-top:1px solid #e6ecf3;"><div style="font-size:12px;font-weight:600;color:#475569;">完整日志 · 服务器本地路径</div><div style="margin-top:6px;font-family:Consolas,monospace;font-size:11px;line-height:19px;color:#64748b;word-break:break-all;">{escape(data.get('完整日志', '未记录'))}</div><div style="margin-top:14px;font-size:11px;line-height:18px;color:#7b8aa0;">由 Linux 定时同步任务自动生成 · 执行时间以北京时间为准</div></td></tr>
</table></td></tr></table></body></html>'''


def compose_message(text, html, subject, sender, recipient):
    message = EmailMessage(policy=SMTP)
    message["From"] = sender
    message["To"] = recipient
    message["Subject"] = subject
    message["Date"] = format_datetime(datetime.now(timezone.utc))
    message["Message-ID"] = make_msgid()
    message.set_content(plain_report(text), charset="utf-8")
    if html:
        message.add_alternative(html, subtype="html", charset="utf-8")
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
