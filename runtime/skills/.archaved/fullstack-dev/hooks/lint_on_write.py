#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PostToolUse hook (GLOBAL): 写源码后做语法自检（按扩展名分派，多语言）。

分派表：
- .php  → php -l（本环境退出码对语法错误仍为 0，故以 stdout 含 "Parse error"/"Errors parsing" 判定）
- .java → javac 单文件**语法级**编译：仅白名单语法错计为命中；cannot find symbol /
          package ... does not exist 等依赖类错误一律忽略，避免缺依赖误报
- .py   → python -m py_compile
- 其他扩展名不处理。
非阻塞：发现语法错误时 exit 1（warning，回显 AI），不阻断已完成的写入。
故障降级：解释器/编译器不可解析时静默 exit 0，避免误报卡住工作。
"""
import sys
import os
import json
import shutil
import subprocess

# PHP 可执行文件探测协议（对齐 references/cms-development.md「PHP 可执行文件探测协议」）：
# 严禁硬编码唯一路径，按优先级链解析，换机/换用户/跨平台可用。
_WIN_CANDIDATES = [
    r"F:\BtSoft\php\{ver}\php.exe",
    r"D:\phpstudy\php\{ver}\php.exe",
    r"C:\xampp\php\php.exe",
    r"C:\Program Files\php\php.exe",
]
_MAC_CANDIDATES = [
    "/opt/homebrew/bin/php@{ver}",
    "/usr/local/bin/php@{ver}",
    "/Applications/MAMP/bin/php/php{ver}/bin/php",
]
_LIN_CANDIDATES = [
    "/usr/bin/php{ver}",
    "/usr/local/bin/php{ver}",
    "/opt/php/{ver}/bin/php",
]
# 版本号高到低探测（宝塔面板默认路径形如 F:\BtSoft\php\{ver}）
_VERSIONS = ["85", "84", "83", "82", "81", "80", "74"]


def resolve_php():
    """按探测协议解析 PHP 可执行文件路径；全部未命中返回 None（调用方静默跳过）。"""
    env = os.environ.get("PHP_BIN")
    if env and os.path.isfile(env):
        return env
    # 多版本环境变量（PHP_85 / PHP_82 ...）
    for ver in _VERSIONS:
        envv = os.environ.get("PHP_" + ver)
        if envv and os.path.isfile(envv):
            return envv
    # 系统 PATH
    p = shutil.which("php")
    if p:
        return p
    # 常见安装位置自动扫描
    for tpl in _WIN_CANDIDATES + _MAC_CANDIDATES + _LIN_CANDIDATES:
        for ver in _VERSIONS:
            cand = tpl.format(ver=ver)
            if os.path.isfile(cand):
                return cand
    return None


# ---------------------------------------------------------------- Java / Python
# 解析协议同 PHP：环境变量 → PATH → 常见位置；禁写死单一路径，跨机/跨平台可用。
def resolve_javac():
    env = os.environ.get("JAVAC_BIN")
    if env and os.path.isfile(env):
        return env
    home = os.environ.get("JAVA_HOME")
    if home:
        for name in ("javac", "javac.exe"):
            cand = os.path.join(home, "bin", name)
            if os.path.isfile(cand):
                return cand
    return shutil.which("javac")


def resolve_python():
    env = os.environ.get("PYTHON_BIN")
    if env and os.path.isfile(env):
        return env
    return shutil.which("python") or shutil.which("python3") or sys.executable


# Java 语法错白名单：仅这些形态判为"语法错"。
# cannot find symbol / package ... does not exist / cannot access 等**依赖类**错误一律不计，
# 否则单文件编译必然因缺少 classpath 而误报，钩子会变成噪声源。
_JAVA_SYNTAX_MARKS = (
    "';' expected",
    "')' expected",
    "'}' expected",
    "']' expected",
    "illegal start of expression",
    "illegal start of type",
    "reached end of file while parsing",
    "class, interface, enum, or record expected",
    "not a statement",
    "unclosed string literal",
    "unclosed character literal",
    "unclosed comment",
    "invalid method declaration",
    "<identifier> expected",
)


def filter_java_syntax_errors(text):
    """从 javac 输出中筛出白名单语法错行（依赖类错误一律不计）。

    抽为纯函数以便单测：javac 在缺少 classpath 时必然报 cannot find symbol /
    package ... does not exist，若一并上报，钩子会退化成噪声源。
    """
    return [
        ln.strip()
        for ln in text.splitlines()
        if any(mark in ln for mark in _JAVA_SYNTAX_MARKS)
    ]


def check_java(javac_bin, path):
    """javac 单文件语法级检查：只返回白名单命中的语法错行（依赖类错误不计）。"""
    import tempfile
    out_dir = tempfile.mkdtemp(prefix="dev-expert-javac-")
    try:
        r = subprocess.run(
            [javac_bin, "-proc:none", "-nowarn", "-d", out_dir, path],
            capture_output=True, text=True, timeout=30,
        )
    except Exception:
        return []
    return filter_java_syntax_errors((r.stdout or "") + (r.stderr or ""))


def check_py(python_bin, path):
    """python -m py_compile 语法检查：非零退出即语法错。"""
    if not python_bin:
        return []
    try:
        r = subprocess.run(
            [python_bin, "-m", "py_compile", path],
            capture_output=True, text=True, timeout=30,
        )
    except Exception:
        return []
    if r.returncode == 0:
        return []
    text = ((r.stderr or "") + (r.stdout or "")).strip()
    lines = [ln for ln in text.splitlines() if ln.strip()]
    return lines[-3:] or ["py_compile 失败（退出码 %s）" % r.returncode]


def main():
    try:
        raw = sys.stdin.read()
        if not raw.strip():
            return 0
        data = json.loads(raw)
    except Exception:
        return 0

    tool_input = data.get("tool_input", {}) or {}
    path = (
        tool_input.get("filePath")
        or tool_input.get("file_path")
        or tool_input.get("path")
        or ""
    )
    if not path:
        return 0
    path = os.path.abspath(path)
    if not os.path.isfile(path):
        return 0
    ext = os.path.splitext(path)[1].lower()

    # ---- .php ----
    if ext == ".php":
        php_bin = resolve_php()
        if not php_bin:
            # PHP 不可解析（未配置 PHP_BIN 且不在 PATH/常见位置）：静默跳过，避免误报卡住工作
            return 0
        try:
            r = subprocess.run(
                [php_bin, "-l", path],
                capture_output=True, text=True, timeout=30,
            )
        except Exception:
            return 0
        out = (r.stdout or "") + (r.stderr or "")
        # 本环境 php -l 退出码对语法错误仍为 0（异常），但 stdout 含 Parse error 可靠。
        # 成功信息 "No syntax errors detected" 不含 "Parse error"，不会误判。
        if "Parse error" in out or "Errors parsing" in out:
            print("[LINT-FAIL] php -l 发现语法错误：")
            print(out.strip())
            return 1  # warning，回显给 AI
        return 0

    # ---- .java ----
    if ext == ".java":
        javac_bin = resolve_javac()
        if not javac_bin:
            return 0  # 无 javac 环境：静默跳过
        hits = check_java(javac_bin, path)
        if hits:
            print("[LINT-FAIL] javac 发现 Java 语法错误（已忽略依赖缺失类错误）：")
            for h in hits:
                print("  " + h)
            return 1
        return 0

    # ---- .py ----
    if ext == ".py":
        hits = check_py(resolve_python(), path)
        if hits:
            print("[LINT-FAIL] py_compile 发现 Python 语法错误：")
            for h in hits:
                print("  " + h)
            return 1
        return 0

    return 0


if __name__ == "__main__":
    sys.exit(main())
