#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PostToolUse hook (GLOBAL): 测量口径提示——命令跑完后，对「不可靠的量化口径」发提醒。

背景：本包错误册记录了两类**测量口径**导致的错误结论：
- `ERR-001`：PowerShell `Get-Content` 未指定 `-Encoding`，默认按 ANSI/GBK 读 UTF-8 中文字档，
  字符数虚高，据此下结论必错；
- `ERR-004`：用宽通配符（如 `-Filter "*26*"`）统计文件数，把无关文件算进来，噪声被当成事实差异。

二者都不是「命令失败」，而是「**结果不可信**」——计数本身是正当操作，故**不阻断**：命令照跑，
但在 Agent 即将解读结果时把风险送达。

送达通道：PostToolUse 的裸 stdout 不回传 Agent；JSON `hookSpecificOutput.additionalContext`
可达（平台以 `<system-reminder>` 注入），本包 `rule_ref_guard.py` 为同类先例。

检查项（命中其一即提示；恒 exit 0）：
- A. 出现 `Get-Content` 且**未带** `-Encoding`，且同命令含计数类操作（`Measure-Object` / `.Count` /
  `.Length` / `-Line`）→ 提醒改用 Python 显式 UTF-8 计数。
- B. `-Filter` 的取值**两侧皆通配符且内含数字**（如 `"*26*"`，`ERR-004` 的特征），且同命令含计数类
  操作 → 提醒改用**精确名单**逐个判定。刻意只认「含数字」的模糊串：扩展名型（`*.py`）与
  名称型（`*test*`）多为正常用法，不提示。
- C. 命令同时出现 `open(` 与 `len(`，但**未出现** `encoding=` → 提醒显式指定编码。理由：规则 A
  自己就建议"改用 Python 显式编码计数"，若照做却漏 `encoding=`，同一类错照旧发生（`ERR-001` 同类）。

逃生阀与失败放行：取不到命令字段 / 含 hook-allow / 任何异常 → 静默 exit 0（不输出）。
退出码：恒 0（提示型，不阻断）。
"""
import sys
import json
import re

ALLOW_MARK = "hook-allow"
CMD_KEYS = ("command", "cmd", "script")

_GET_CONTENT = re.compile(r"\bGet-Content\b", re.IGNORECASE)
_ENCODING = re.compile(r"-Encoding\b", re.IGNORECASE)
_FILTER = re.compile(r"-Filter\s+['\"]([^'\"]+)['\"]", re.IGNORECASE)
# 计数类操作：PowerShell 的 Measure-Object（含 -Line）与属性计数
_COUNT_OP = re.compile(r"Measure-Object|\.Count\b|\.Length\b|-Line\b", re.IGNORECASE)
_HAS_DIGIT = re.compile(r"\d")
# C. Python 读文件缺 encoding（与 A 同类：编码口径）
_OPEN_CALL = re.compile(r"\bopen\s*\(")
_LEN_CALL = re.compile(r"\blen\s*\(")
_ENCODING_KW = re.compile(r"encoding\s*=")


def _fix_stdout_encoding():
    """Windows 控制台默认 GBK，直接 print 非 ASCII 会抛 UnicodeEncodeError。"""
    try:
        import io
        enc = (sys.stdout.encoding or "").lower().replace("-", "")
        if enc != "utf8":
            sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")
    except Exception:
        pass


def emit(lines):
    """JSON 契约输出：additionalContext 在 PostToolUse 可达 Agent。只输出这一个 JSON。"""
    msg = "\n".join(lines)
    print(json.dumps({
        "systemMessage": msg,
        "hookSpecificOutput": {"hookEventName": "PostToolUse", "additionalContext": msg},
    }, ensure_ascii=False))


def collect(cmd):
    """返回命中的提示行列表（空列表＝无需提示）。"""
    notes = []
    counting = bool(_COUNT_OP.search(cmd))
    if counting and _GET_CONTENT.search(cmd) and not _ENCODING.search(cmd):
        notes.append("- `Get-Content` 未指定 `-Encoding`：Windows 默认按 ANSI/GBK 读，"
                     "UTF-8 中文字档的字符数会虚高（本包 ERR-001）。下量化结论前改用 Python 显式编码计数，"
                     "或补 `-Encoding UTF8`。")
    if counting:
        for m in _FILTER.finditer(cmd):
            val = m.group(1)
            if len(val) > 2 and val.startswith("*") and val.endswith("*") and _HAS_DIGIT.search(val):
                notes.append("- `-Filter \"%s\"` 两侧皆通配符且含数字：模糊匹配会把无关文件算进来，"
                             "噪声易被当成事实差异（本包 ERR-004）。存在性核验请用**精确名单逐个**判定，"
                             "不要用通配符统计计数。" % val)
                break
    if _OPEN_CALL.search(cmd) and _LEN_CALL.search(cmd) and not _ENCODING_KW.search(cmd):
        notes.append("- Python 读文件未显式指定 `encoding=`：Windows 默认按 locale 解码，"
                     "UTF-8 中文字档的字符 / 行数会虚高（本包 ERR-001 同类）。"
                     "请写 `encoding='utf-8'`（或 `'utf-8-sig'`）后重新计数。")
    return notes


def main():
    _fix_stdout_encoding()
    try:
        raw = sys.stdin.read()
        if not raw.strip():
            return 0
        data = json.loads(raw)
    except Exception:
        return 0

    tool_input = data.get("tool_input", {}) or {}
    if not isinstance(tool_input, dict):
        tool_input = {}
    src = tool_input or data

    cmd = ""
    for key in CMD_KEYS:
        v = src.get(key)
        if isinstance(v, str) and v.strip():
            cmd = v
            break
    if not cmd or ALLOW_MARK in cmd:
        return 0

    notes = collect(cmd)
    if not notes:
        return 0
    emit(["⚠️ 测量口径提示（本条命令的结果可能不可信，下结论前先复核）："] + notes)
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:
        sys.exit(0)
