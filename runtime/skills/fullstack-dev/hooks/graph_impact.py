#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PostToolUse hook: 写代码文件后自动附「上游影响面摘要」（知识图谱加速器）。

定位：补 `SKILL.md Step 3（执行前查上游）` 的自动兜底——该时机原为 agent 显式调用
`--query`，实际执行中易遗漏；本钩子在**写盘后**自动查询一次上游 2 跳依赖，把结果
摘要成 ≤3 行送达 Agent（JSON `hookSpecificOutput.additionalContext`）。

边界（有意收敛，防噪声）：
- 仅代码文件（.php/.phtml/.inc/.php3/.php5/.js/.jsx/.ts/.tsx/.py/.java）；
- 记忆/工具/规则区（`.ai-memory/**`、`.codebuddy/**`）跳过——属元工具区，图谱无意义；
- **仅当图谱三件套齐全**（graph/meta/symbols）才查询：搭配 `--no-rebuild` 时
  `build_graph` 直接复用缓存、**绝不重建**；三件套缺失 → 静默，不冒险触发全量重建；
- **无上游依赖 → 静默**；任何异常/超时 → 静默 `exit 0`，**绝不阻断写入**。

纪律：图谱仅加速器，**完整影响面须 grep 复核**，摘要附带此提示。
"""
import sys
import os
import json
import subprocess

HOOK_DIR = os.path.dirname(os.path.abspath(__file__))

CODE_EXT = {".php", ".phtml", ".inc", ".php3", ".php5",
            ".js", ".jsx", ".ts", ".tsx", ".py", ".java"}
MAX_SHOW = 5         # 摘要中展示的上游文件数
DEPTH = 2
TIMEOUT = 15

# 记忆根目录候选（按序探测）：`.ai-memory` 为本技能口径，`.codebuddy` 为兼容口径
MEMORY_RELS = (".ai-memory", os.path.join(".codebuddy", "memory"))
# 图谱三件套
GRAPH_FILES = ("graph.json", "meta.json", "symbols.json")
# 项目根标记
ROOT_MARKERS = (".ai-memory", ".codebuddy", ".git")
SKIP_PREFIXES = (".ai-memory/", ".codebuddy/")


def _fix_stdout_encoding():
    """Windows 控制台默认 GBK，直接 print 非 ASCII（如 ⚠）会抛 UnicodeEncodeError。"""
    try:
        import io
        enc = (sys.stdout.encoding or "").lower().replace("-", "")
        if enc != "utf8":
            sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")
    except Exception:
        pass


def emit(lines, event="PostToolUse"):
    """JSON 契约输出：`hookSpecificOutput.additionalContext` 在 PostToolUse 可达 Agent
    （平台以 <system-reminder> 注入）。`systemMessage` 供 IDE UI。
    **只输出这一个 JSON**，不与裸 print 混排。"""
    msg = "\n".join(lines)
    print(json.dumps({
        "systemMessage": msg,
        "hookSpecificOutput": {"hookEventName": event, "additionalContext": msg},
    }, ensure_ascii=False))


def read_event():
    try:
        raw = sys.stdin.buffer.read().decode("utf-8", errors="ignore")
    except Exception:
        return {}
    raw = raw.lstrip("\ufeff")
    if not raw.strip():
        return {}
    try:
        return json.loads(raw)
    except Exception:
        return {}


def _climb(start, markers):
    """从 start 起逐级上溯，返回首个含 markers 任一子项的目录。"""
    d = os.path.abspath(start)
    while True:
        for mk in markers:
            if os.path.exists(os.path.join(d, mk)):
                return d
        nd = os.path.dirname(d)
        if nd == d:
            return None
        d = nd


def resolve_root(target_path):
    """解析项目根：环境变量 PROJECT_ROOT → 目标文件上溯 → 脚本位置上溯。"""
    env = os.environ.get("PROJECT_ROOT", "")
    if env and os.path.isdir(env):
        return os.path.abspath(env)
    r = _climb(os.path.dirname(os.path.abspath(target_path)), ROOT_MARKERS)
    if r:
        return r
    return _climb(HOOK_DIR, ROOT_MARKERS)


def find_graph_dir(root):
    """按序探测图谱目录；三件套齐全才返回，否则 None。"""
    if not root:
        return None
    for rel in MEMORY_RELS:
        kg = os.path.join(root, rel, "knowledge-graph")
        try:
            if all(os.path.isfile(os.path.join(kg, f)) for f in GRAPH_FILES):
                return kg
        except OSError:
            continue
    return None


def find_graph_script(root):
    """按序探测 build_graph.py：脚本同目录 → 技能 scripts/ → 项目 .codebuddy/{hooks,scripts}/。"""
    cands = [
        os.path.join(HOOK_DIR, "build_graph.py"),
        os.path.abspath(os.path.join(HOOK_DIR, "..", "scripts", "build_graph.py")),
    ]
    if root:
        cands += [
            os.path.join(root, ".codebuddy", "hooks", "build_graph.py"),
            os.path.join(root, ".codebuddy", "scripts", "build_graph.py"),
        ]
    for c in cands:
        if os.path.isfile(c):
            return c
    return None


def main():
    _fix_stdout_encoding()
    data = read_event()
    tool_input = data.get("tool_input") or {}
    path = (tool_input.get("filePath") or tool_input.get("file_path")
            or tool_input.get("path") or "")
    if not path:
        return 0
    if os.path.splitext(path)[1].lower() not in CODE_EXT:
        return 0

    root = resolve_root(path)
    if not root:
        return 0
    try:
        rel = os.path.relpath(os.path.abspath(path), root).replace("\\", "/")
    except Exception:
        return 0
    if rel.startswith("..") or rel.startswith(SKIP_PREFIXES):
        return 0                       # 项目外 / 元工具区

    kg = find_graph_dir(root)
    if not kg:
        return 0                       # 图谱缺失/不新鲜 → 静默（不触发重建）
    graph_script = find_graph_script(root)
    if not graph_script:
        return 0

    try:
        r = subprocess.run(
            [sys.executable, graph_script, "--root", root, "--query", rel,
             "--direction", "up", "--depth", str(DEPTH), "--no-rebuild"],
            capture_output=True, text=True, timeout=TIMEOUT,
        )
    except Exception:
        return 0
    out = r.stdout or ""
    i = out.find("{")
    if i < 0:
        return 0
    try:
        graph = json.loads(out[i:])
    except Exception:
        return 0

    nodes = graph.get("nodes") or {}
    ups = sorted(k for k in nodes if k != rel)
    if not ups:
        return 0                       # 无上游依赖 → 静默
    show = ", ".join(ups[:MAX_SHOW])
    more = len(ups) - MAX_SHOW
    lines = ["[GRAPH-IMPACT] %s 被 %d 处上游依赖：%s%s"
             % (rel, len(ups), show, ("…（+%d）" % more) if more > 0 else "")]
    cyc = graph.get("cycle") or 0
    if cyc:
        lines.append("⚠ 该依赖闭包含 %d 个环（循环依赖，图谱已折叠）" % cyc)
    lines.append("（图谱仅加速器：完整影响面须 grep 复核，见 Rules §14 G1/G2'/G3）")
    emit(lines)
    return 0


if __name__ == "__main__":
    sys.exit(main())
