#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PostToolUse hook: 改技能文档后自动跑「跨文件步骤号一致性」检查。

背景：`hooks/step_ref_check.py` 是常驻校验器，但过去**没有自动触发路径**——
只能靠 Agent 记得手跑，弱模型漏跑无兜底。本 hook 补上该自动路径。

触发条件（命中其一即跑）：
- 本次工具调用的**路径类字段**（filePath/file_path/path/target_file）命中技能根下的
  `SKILL.md` / `README.md` / `FAQ.md` 或 `references/**.md`（这些文件含跨文件步骤号引用）
- 或**命令类字段**（command/cmd/script）字符串中出现 `step_ref_check`
> 只取路径/命令字段，**不取 content/new_str**（否则任何提及这些文件名的文档写入都会误触发）。

行为：
- 调同目录 `step_ref_check.py --root <技能根>` 做步骤号核对；**只有发现不匹配才回显**
  （PostToolUse 送达通道＝JSON `hookSpecificOutput.additionalContext`）；无悬空 / 超时 /
  异常一律静默 `exit 0`。

测试参数（供造反例，生产不传）：`--root`
退出码：0 = 未触发或无悬空（静默）；1 = 发现悬空（已回显）
"""
import argparse
import json
import os
import subprocess
import sys

HOOK_DIR = os.path.dirname(os.path.abspath(__file__))
# 技能根：脚本位于 <技能根>/hooks/ → 上溯一级
DEFAULT_ROOT = os.path.abspath(os.path.join(HOOK_DIR, ".."))

# 含跨文件步骤号引用的技能文档（相对技能根）
DOC_RELS = ("SKILL.md", "README.md", "FAQ.md")
REFS_REL = "references"

# 只读这些字段判触发（避免 content/new_str 里的文本误触发）
PATH_KEYS = ("filePath", "file_path", "path", "target_file", "notebook_path")
CMD_KEYS = ("command", "cmd", "script")

TIMEOUT = 15          # 子进程硬超时（秒），超时即静默放行
MAX_DETAIL_LINES = 8  # 回显明细上限（防刷屏）


def _fix_stdout_encoding():
    """Windows 控制台默认 GBK，直接 print 非 ASCII 会抛 UnicodeEncodeError。"""
    try:
        enc = (sys.stdout.encoding or "").lower().replace("-", "")
        if enc != "utf8":
            import io
            sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")
    except Exception:
        pass


def emit(lines, event="PostToolUse"):
    """JSON 契约输出：`hookSpecificOutput.additionalContext` 在 PostToolUse 可达 Agent
    （平台以 <system-reminder> 注入）。**只输出这一个 JSON**，不与裸 print 混排。"""
    msg = "\n".join(lines)
    print(json.dumps({
        "systemMessage": msg,
        "hookSpecificOutput": {"hookEventName": event, "additionalContext": msg},
    }, ensure_ascii=False))


def detect_target(tool_input, root):
    """判定本次改动是否命中技能文档；命中返回 True。只读路径/命令字段。"""
    chunks = []
    for k in PATH_KEYS + CMD_KEYS:
        v = tool_input.get(k)
        if isinstance(v, str) and v.strip():
            chunks.append(v.replace("\\", "/"))
    blob = " ".join(chunks).lower()
    if not blob:
        return False
    if "step_ref_check" in blob:
        return True
    try:
        for rel in DOC_RELS:
            if os.path.join(root, rel).replace("\\", "/").lower() in blob:
                return True
        if (os.path.join(root, REFS_REL).replace("\\", "/").lower() in blob
                and ".md" in blob):
            return True
        # 兜底：相对路径写法（SKILL.md / references/xxx.md）
        if any(rel.lower() in blob for rel in DOC_RELS):
            return True
        if (REFS_REL + "/") in blob and ".md" in blob:
            return True
    except Exception:
        return False
    return False


def main():
    _fix_stdout_encoding()
    parser = argparse.ArgumentParser(description="改技能文档后自动跑步骤号一致性检查")
    parser.add_argument("--root", default=DEFAULT_ROOT, help="技能根（默认脚本上一级）")
    args = parser.parse_args()
    root = os.path.abspath(args.root)

    try:
        raw = sys.stdin.buffer.read().decode("utf-8", "replace").lstrip("\ufeff")
        data = json.loads(raw) if raw.strip() else {}
    except Exception:
        return 0  # 解析异常一律放行（非阻断）

    tool_input = data.get("tool_input", {}) or {}
    if not isinstance(tool_input, dict):
        return 0
    if not detect_target(tool_input, root):
        return 0  # 与技能文档无关的写操作：静默（零噪声）

    checker = os.path.join(HOOK_DIR, "step_ref_check.py")
    if not os.path.isfile(checker):
        return 0  # 校验器缺失：放行（绝不影响原工具执行）
    cmd = [sys.executable, checker, "--root", root]
    try:
        proc = subprocess.run(cmd, cwd=root, capture_output=True, timeout=TIMEOUT)
    except Exception:
        return 0  # 超时/启动失败：静默放行

    if proc.returncode == 0:
        return 0  # 无悬空：静默（不产生噪声）

    out = (proc.stdout or b"").decode("utf-8", "replace")
    detail = [ln.strip() for ln in out.splitlines()
              if ("不匹配" in ln or "期望" in ln or "上下文" in ln)][:MAX_DETAIL_LINES]
    if not detail:
        detail = ["（校验器异常退出，未输出明细；请手动跑 step_ref_check.py 复核）"]
    emit(["[STEP-REF] 跨文件步骤号一致性：发现不匹配（hook 自动执行，对齐 Rules §21 引用一致性）",
          "技能根: %s" % root] + detail +
         ["处置：改正引用文案或目标 reference 章节号；自查 `python hooks/step_ref_check.py`"])
    return 1


if __name__ == "__main__":
    sys.exit(main())
