#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PreToolUse hook (GLOBAL): 命令卫生守卫——拦下「注定失败或污染产物」的命令写法。

背景：本包错误册已记录两类由**命令写法**（而非逻辑）导致的返工，且二者都在命令执行前
即可判定，故放这里拦截（PostToolUse 的裸 stdout 不回传 Agent，事后提醒无法挽回已污染的结果）：
- `ERR-003`：多语句 / 含正则转义的 Python 经命令行内联传参，被 PowerShell 词法层改写而失败；
- `ERR-002`：`py_compile` 即便加 `-B` 仍写 `.pyc`，污染交付目录（语法自检应用 `ast.parse`）。

检查项（命中其一即 exit 2 拒绝，并把正确做法回传 Agent）：
- A. 内联 Python 载荷含**换行**或**反斜杠**（正则 / 转义）→ 要求落盘探针再执行。
  刻意**不含分号**：本包实测单行分号式（如 `import ast,io;ast.parse(...)`）可正常执行，
  且它正是规则 B 推荐的替代写法——拦下它会自相矛盾。
- B. 命令含 `-m py_compile` 或 `py_compile.` → 要求改用 `ast.parse` 做零产物语法自检。

字段来源：命令类工具的命令串（`command` / `cmd` / `script`，与 `rule_ref_guard.py` 同口径）；
写类工具（无命令字段）自然放行。

逃生阀与失败放行（对齐本包「绝不误阻断」原则）：
1. 命令不含上述特征 → 放行；2. 命令串含 hook-allow 标记 → 放行；
3. 任何异常（含 stdout 编码失败）→ 放行。

退出码：命中 → 2（拒绝并把理由回传 Agent）；否则 0。
"""
import sys
import json
import re

ALLOW_MARK = "hook-allow"
CMD_KEYS = ("command", "cmd", "script")

# A. 内联 Python：py / python / python3（含 `.exe` 与引号形式），容许 -B / -u / -3 等中间标志。
# 实测教训：早期只覆盖 `python -c` 一种写法，漏掉本包惯用的 `python -B -c`（运行时负对照实测漏放）。
_INLINE_PY = re.compile(r'\b(?:py|python3?)(?:\.exe)?["\']?\s+(?:-\S+\s+)*-c\b', re.IGNORECASE)
# A. 危险载荷字符：换行（真多行）与反斜杠（正则 / 转义）
_RISKY_CHARS = ("\n", "\\")
# B. 语法自检误用 py_compile（必写 .pyc）
_PY_COMPILE = re.compile(r"-m\s+py_compile\b|py_compile\s*\.", re.IGNORECASE)


def _payload_after_c(cmd):
    """返回 `-c` 之后的内联载荷子串；未命中内联 Python 时返回 None。

    只对**载荷**判危险字符：整条命令行常含 Windows 路径（大量 `\\`），按整串判定会把
    「内联 Python ＋ 任意路径」的合法命令一律误阻——本包运行时实测踩过该误报。
    """
    m = _INLINE_PY.search(cmd)
    if not m:
        return None
    rest = cmd[m.end():]
    i = 0
    while i < len(rest) and rest[i] in " \t":
        i += 1
    if i < len(rest) and rest[i] in "\"'":
        j = rest.find(rest[i], i + 1)
        return rest[i:j + 1] if j > i else rest[i:]
    for k in range(i, len(rest)):
        if rest[k] in ";|&\n":
            return rest[i:k]
    return rest[i:]


def _fix_stdout_encoding():
    """Windows 控制台默认 GBK，直接 print 非 ASCII 会抛 UnicodeEncodeError。"""
    try:
        import io
        enc = (sys.stdout.encoding or "").lower().replace("-", "")
        if enc != "utf8":
            sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")
    except Exception:
        pass


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

    reason = ""
    if _PY_COMPILE.search(cmd):
        reason = ("命令含 `py_compile`：即使加 `-B` 仍会写出 `.pyc`，污染交付目录（本包 ERR-002）。\n"
                  "改用零产物通道做语法自检（单行分号式，不含换行与反斜杠，不会被本 hook 拦）：\n"
                  "  python -c \"import ast,io;ast.parse(io.open('X.py',encoding='utf-8').read());print('SYNTAX_OK')\"")
    elif any(c in (_payload_after_c(cmd) or "") for c in _RISKY_CHARS):
        reason = ("内联 Python 载荷含换行或反斜杠（正则 / 转义）：会被 shell 词法层改写而失败（本包 ERR-003）。\n"
                  "改为落盘探针三步：写入属性为「临时目录/<name>.py」的文件 → 用 python <file> 执行 → 执行后删除并核验其不存在。")
    if not reason:
        return 0

    print("[CMD-HYGIENE] 拒绝执行该命令：" + cmd.strip().splitlines()[0][:120])
    print(reason)
    print("如确属特例需要放行，请在命令串中含 %s 标记。" % ALLOW_MARK)
    return 2


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:
        sys.exit(0)
