#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PreToolUse hook (GLOBAL): 写入 Plan/*_plan.md 时断言「推理链外化」核心节齐备**且内容达标**。

背景：`plan_guard.py` 只保证「改动核心目录前存在进行中 plan」，不检查 plan 本身是否
承载了推理链——目标 / 步骤 / 验证一旦缺失，plan 就退化为空壳，规划门禁形同虚设。
本 hook 补这一环：写入 `Plan/*_plan.md` 时校验核心节及其**内容密度**，不达标即阻断并把
缺项回传 Agent（PostToolUse 的 stdout 不回传 Agent，只有 PreToolUse exit 2 能把理由送达）。

检查分级（有意分级，避免误伤分步写作与精简 plan）：
- **阻断级·形态**（缺任一即 exit 2）：`## 目标` / `## 步骤` / `## 验证方式`
  —— 三节分别对应「做什么 / 怎么做 / 怎么验」，是推理链外化的**最小集**。
- **阻断级·内容**（存在但过薄同样 exit 2）：`## 目标` 内容行 < 2；`## 步骤` 无勾选条目
  （`- [ ]` / `- [x]`）；`## 验证方式` 无命令级证据（代码围栏或含命令词的行）。
  目的：堵住「只敲三个标题即可过闸」的空壳写法（标题级门禁的空子成本≈0）。
  **只判「是否具体化」，不判「推理对错」**——后者无法机械判定，须靠 SELF-AUDIT 人工对证。
- **阻断级·完成态兑现**（仅当 `## 状态` 含「已完成」时生效）：`## 步骤` 不得留有未勾选条目；
  `## 验证方式` 须含实测痕迹（✅ / 实测 / exit / PASS）。目的：把「未外化即视为未做」绑到
  **收尾那一刻**——即 Layer 3「推理外化」的收尾对证（此前只有写入时的形态要求）。
  作用域**有意限定**在 `## 步骤`：门禁勾核表里的「其余〔未命中〕」是合法未勾选项，按全文判会误伤。
- **提示级**（仅在已阻断时随理由一并列出）：`## 状态` / `## 文件清单` /
  `## 风险点` / `## 决策记录`。

适用范围：仅 `Plan/*_plan.md`。`_audit_logic.md`、`*_spec.md` 与其它任何文件一律放行。

逃生阀与失败放行（对齐本包「绝不误阻断」原则）：
1. 目标非 `Plan/*_plan.md` → 放行；2. 内容中**独立一行**的 hook-allow → 放行；
3. 任何异常（stdin 非 JSON / 取不到内容等）→ 放行。

> 逃生阀取「独立成行」而非全文子串：plan 正文常会**讨论这个标记本身**（如用例表里
> 写「逃生阀 hook-allow」），子串判定会让这类 plan 直接免疫门禁——本钩子自己的
> Plan/31/32 就踩过这个盲区。其它阻断型 hook 面向代码/命令，保持子串语义
> （命令串里的 `# hook-allow` 是行尾注释，行首语义会失效），故此处有意不统一。

退出码：命中缺节或内容过薄 → 2（拒绝并把理由回传 Agent）；否则 0。
"""
import sys
import os
import json
import re
import typing

ALLOW_MARK = "hook-allow"
# 逃生阀须独立成行（见 docstring）；代码/命令类 hook 仍用子串语义
ALLOW_LINE = re.compile(r"^\s*hook-allow\b")

# 阻断级·形态：推理链外化的最小集（做什么 / 怎么做 / 怎么验）
REQUIRED = ("## 目标", "## 步骤", "## 验证方式")
# 提示级：仅在已阻断时一并列出，不单独触发阻断
ADVISORY = ("## 状态", "## 文件清单", "## 风险点", "## 决策记录")
# 阻断级·内容：具体化判据（只判是否具体，不判对错）
GOAL_MIN_LINES = 2
CHECKBOX = re.compile(r"^\s*[-*]\s*\[[ xX]\]")
# 未勾选条目：仅用于「完成态兑现」判据，且只判 `## 步骤` 区块
CHECKBOX_OPEN = re.compile(r"^\s*[-*]\s*\[ \]")
# 完成态兑现判据的核验目标
DONE_MARK = "已完成"
EVIDENCE_MARKS = ("✅", "实测", "exit", "PASS")
CMD_WORDS = ("python", "pytest", "npm ", "pnpm", "yarn", "php ", "composer",
             "java ", "mvn", "gradle", "curl", "grep", "node ", "dotnet")


def _fix_stdout_encoding():
    """Windows 控制台默认 GBK，直接 print 非 ASCII（如 ✅）会抛 UnicodeEncodeError。"""
    try:
        import io
        enc = (sys.stdout.encoding or "").lower().replace("-", "")
        if enc != "utf8":
            sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")
    except Exception:
        pass


def _sections(text: str) -> "list[tuple[str, list[str]]]":
    """按 `## ` 切分，返回 [(标题, 正文行列表)]；`###` 及其内容归入其父节。"""
    secs: "list[tuple[str, list[str]]]" = []
    title = None
    body: "list[str]" = []
    for line in text.splitlines():
        if line.startswith("## ") and not line.startswith("### "):
            if title is not None:
                secs.append((title, body))
            title, body = line.strip(), []
        elif title is not None:
            body.append(line)
    if title is not None:
        secs.append((title, body))
    return secs


def _density_problems(sections: "list[tuple[str, list[str]]]") -> "list[str]":
    """返回「内容过薄」的缺项说明（空列表＝达标）。按前缀识别节，只判已存在的节。"""
    problems: "list[str]" = []
    for key in REQUIRED:
        lines = [l for title, body in sections if title.startswith(key) for l in body]
        if not lines:
            continue
        if key == "## 目标":
            n = len([l for l in lines if l.strip()])
            if n < GOAL_MIN_LINES:
                problems.append("`## 目标` 内容过薄（实 %d 行，需 ≥ %d 行）" % (n, GOAL_MIN_LINES))
        elif key == "## 步骤":
            if not any(CHECKBOX.match(l) for l in lines):
                problems.append("`## 步骤` 无可勾选条目（需 `- [ ]` / `- [x]`，便于执行与跟踪）")
        elif key == "## 验证方式":
            has_fence = any(l.strip().startswith("```") for l in lines)
            has_cmd = any(w in l.lower() for l in lines for w in CMD_WORDS)
            if not (has_fence or has_cmd):
                problems.append("`## 验证方式` 无命令级证据（需代码围栏或含命令词的行）")
    return problems


def _status_lines(sections: "list[tuple[str, list[str]]]") -> "list[str]":
    """收集 `## 状态` 节的标题行与正文行。

    标题行也取：状态值有「## 状态: 已完成」行内写法，只取正文行会让该写法
    因正文字为空而绕过完成态门禁。
    """
    out: "list[str]" = []
    for title, body in sections:
        if title.startswith("## 状态"):
            out.append(title)
            out.extend(body)
    return out


def _done_problems(sections: "list[tuple[str, list[str]]]") -> "list[str]":
    """完成态兑现判据：仅当 `## 状态` 声明完成时生效。

    作用域**有意限定**：未勾选条目只判 `## 步骤` 区块——门禁勾核表里的
    「其余〔未命中〕」是合法的未勾选项，按全文判会误伤真实计划。
    """
    status = _status_lines(sections)
    if not any(DONE_MARK in l for l in status):
        return []
    problems: "list[str]" = []
    steps = [l for t, b in sections if t.startswith("## 步骤") for l in b]
    opened = [l for l in steps if CHECKBOX_OPEN.match(l)]
    if opened:
        problems.append("`## 状态` 已声明完成，但 `## 步骤` 仍有 %d 项未勾选"
                        "（勾选完成项，或把延后项移入「已知限制」）" % len(opened))
    verify = [l for t, b in sections if t.startswith("## 验证方式") for l in b]
    if not any(m in l for l in verify for m in EVIDENCE_MARKS):
        problems.append("`## 状态` 已声明完成，但 `## 验证方式` 无实测痕迹"
                        "（需 ✅ / 实测 / exit / PASS 等结果标记——未外化即视为未做）")
    return problems


def _has_allow_mark(text: str) -> bool:
    """逃生阀判定：仅认「独立一行」的 hook-allow，避免正文提及即免疫门禁。"""
    return any(ALLOW_LINE.match(l) for l in text.splitlines())


def main():
    _fix_stdout_encoding()
    try:
        raw = sys.stdin.read()
        if not raw.strip():
            return 0
        data = typing.cast("dict[str, object]", json.loads(raw))
    except Exception:
        return 0

    parsed = data.get("tool_input")
    tool_input = typing.cast("dict[str, object]", parsed) if isinstance(parsed, dict) else {}

    path = ""
    for key in ("filePath", "file_path", "path"):
        if tool_input.get(key):
            path = str(tool_input[key])
            break
    if not path:
        return 0

    parts = os.path.abspath(path).replace("\\", "/").split("/")
    if "Plan" not in parts or not parts[-1].endswith("_plan.md"):
        return 0

    content = ""
    for key in ("content", "new_str", "new_string", "text"):
        v = tool_input.get(key)
        if isinstance(v, str) and v:
            content = v
            break
    if not content or _has_allow_mark(content):
        return 0

    sections = _sections(content)
    missing = [s for s in REQUIRED if s not in content]
    thin = _density_problems(sections) + _done_problems(sections)
    if not missing and not thin:
        return 0

    advisory = [s for s in ADVISORY if s not in content]
    print("[PLAN-QUALITY] 拒绝写入未达标的计划：" + path)
    if missing:
        print("推理链外化核心节缺失：" + "、".join(missing))
    if thin:
        print("未达标项（内容密度 / 完成态兑现）：")
        for item in thin:
            print("  - " + item)
    print("这三节对应「做什么 / 怎么做 / 怎么验」；只是存在而内容过薄时计划仍是空壳，规划门禁失去意义（对齐 Rules §13 十节骨架与规划自审）。")
    print("本钩子只判「是否具体化」，不判推理对错；请补齐可执行内容后重试。")
    if advisory:
        print("另建议补齐（本次不阻断）：" + "、".join(advisory))
    print("如确需先落骨架再补，请**独立一行**写 " + ALLOW_MARK + " 放行（正文提及不算）。")
    return 2


if __name__ == "__main__":
    sys.exit(main())
