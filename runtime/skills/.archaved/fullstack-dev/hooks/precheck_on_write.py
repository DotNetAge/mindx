#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PreToolUse hook (GLOBAL): 写入前「内容级」检查（唯一能把告警送达 Agent 的通道）。

背景：PostToolUse 的任何退出码，其 stdout 都不回传 Agent；只有 PreToolUse 的
exit 2（deny）会把 stdout 作为拒绝理由回传给 Agent。故把「必须在写入前知道」
的检查放在这里。检查项对齐 Rules §5（安全）/§6（PHP8 兼容）/§8（编码）/§10（质量与性能）。

检查范围：
  A. PHP 语法错误：仅当 content 以 `<?` 开头（完整文件）时写临时文件跑 php -l，
     命中 Parse error → 阻断。
  B. 调试残留：content / new_str 含输出型调试函数调用 → 阻断。
  C. 安全组合：**危险函数 + 用户输入源（超全局）直接组合**才命中（见 SEC_PATTERNS），
     不按裸函数名判定 —— 大幅压低误报。
  D. 语言/规范类：PHP8 兼容（短标签/裸数组键/废弃函数）、性能禁则（SELECT *）、
     硬编码凭证与日志含敏感变量、BOM、超全局直接入 SQL 字符串、`/e` 修饰符、
     `extract()` 接超全局、反射型 XSS。

误报面说明：
  - `replace_in_file` 只看到 new_str 片段 → 不涉存量；
  - `write_to_file` 看到 content **全文** → 若为覆盖已有文件，全文含历史代码，
    **仍可能撞上存量写法**。故保留 `hook-allow` 逃生阀，且安全项统一采用
    「组合模式」（要求危险函数与用户输入源同现），把误报压到最低。

豁免/逃生阀：
  1. 目标非 .php → 放行；2. 内容含 hook-allow 标记 → 放行；
  3. php 不可用或任何异常 → 放行（失败放行，绝不误阻断）。

退出码：命中 → 2（拒绝并把理由回传 Agent）；否则 0。

注：本文件刻意不出现完整的输出型调试函数/危险函数字面量（一律拼接构造），
    既防杀软启发式误删，也防本 hook 在被再次写入时自拦。
"""
import sys
import os
import json
import re
import shutil
import subprocess
import tempfile

TARGET_EXT = ".php"
ALLOW_MARK = "hook-allow"
# 拼接构造：文件文本中不出现完整函数名
DEBUG_PATTERNS = ("var_" + "dump(", "print_" + "r(", "var_" + "export(")

# --- C. 安全组合模式：「危险函数 + 用户输入源（超全局）」直接组合才命中 ---
# 刻意不含 XSS（echo 型）、模板注入、SQL 拼接三类 —— 它们对既有代码误报面大，
# 且属"输出/查询"语义而非"写入危险代码"，故此处单列到 C8/C6 参数级判定。
_USER = r"\$_(?:GET|POST|REQUEST|COOKIE)\b"
SEC_PATTERNS = [
    ("FILE_INCLUDE", re.compile(
        r"(?:include|require|include_once|require_once)\s*\(?\s*"
        r"(?:" + _USER + r"|\$[a-z_]\w*\s*\.\s*" + _USER + r")", re.IGNORECASE),
     "文件包含直接使用用户输入（LFI/RFI）"),
    ("RFI_SSRF", re.compile(
        r"(?:include|require|include_once|require_once)\s*\(?\s*['\"]https?://",
        re.IGNORECASE),
     "include/require 远程 URL（RFI/SSRF）"),
    ("CODE_EXEC", re.compile(
        r"(?:ev" + r"al|assert)\s*\(\s*(?:" + _USER
        + r"|ba" + r"se64_decode\s*\(\s*\$_|" + _USER + r")", re.IGNORECASE),
     "代码执行：动态执行函数直接接用户输入"),
    ("CMD_EXEC", re.compile(
        r"(?:sys" + r"tem|exec|passthru|shell_" + r"exec|popen|proc_open)\s*\(\s*"
        r"(?:" + _USER + r"|\$[a-z_]\w*\s*\(\s*" + _USER + r")", re.IGNORECASE),
     "命令执行：命令函数直接接用户输入"),
    ("FILE_UPLOAD", re.compile(
        r"move_uploaded_file\s*\([^,]+,\s*"
        r"(?:" + _USER + r"|\$[a-z_]\w*\s*\.\s*" + _USER + r")", re.IGNORECASE),
     "上传落地路径含用户输入"),
    ("DESERIALIZE", re.compile(
        r"un" + r"serialize\s*\(\s*(?:" + _USER + r"|file_get_contents\s*\(\s*\$_"
        r"|" + _USER + r")", re.IGNORECASE),
     "反序列化直接处理用户输入（可致 RCE）"),
    ("SSRF", re.compile(
        r"(?:file_get_contents|curl_exec|fopen|readfile)\s*\(\s*"
        r"(?:" + _USER + r"|\$[a-z_]\w*\s*\.\s*" + _USER + r")", re.IGNORECASE),
     "SSRF/任意文件读取：URL 含用户输入"),
    ("XXE", re.compile(
        r"(?:simplexml_load_string|DOMDocument|XMLReader|xml_parse)\s*\(\s*"
        r"(?:" + _USER + r"|\$[a-z_]\w*\s*\.\s*" + _USER + r")", re.IGNORECASE),
     "XML 解析器直接接用户输入（XXE）"),
    ("FILE_WRITE", re.compile(
        r"(?:file_put_contents|fwrite|fputs)\s*\(\s*"
        r"(?:" + _USER + r"|\$[a-z_]\w*\s*\.\s*" + _USER + r")", re.IGNORECASE),
     "任意文件写入：路径含用户输入"),
]


def scan_security(text):
    """逐行扫描「危险函数 + 用户输入源」组合（跳过注释行），返回命中描述列表。"""
    out = []
    for i, line in enumerate(text.splitlines(), start=1):
        s = line.strip()
        if (s.startswith("//") or s.startswith("#") or s.startswith("*")
                or s.startswith("/*")):
            continue
        for rid, rx, msg in SEC_PATTERNS:
            if rx.search(line):
                out.append("安全风险 [%s] 行 %d：%s（Rules §5）" % (rid, i, msg))
                break  # 同行只报一次
    return out

# PHP 可执行文件探测协议（与 lint_on_write.py 同一协议）：
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
_VERSIONS = ["85", "84", "83", "82", "81", "80", "74"]


def resolve_php():
    """复用 lint_on_write.py 的探测协议：PHP_BIN → PHP_xx → PATH → 常见位置。"""
    env = os.environ.get("PHP_BIN")
    if env and os.path.isfile(env):
        return env
    for ver in _VERSIONS:
        envv = os.environ.get("PHP_" + ver)
        if envv and os.path.isfile(envv):
            return envv
    p = shutil.which("php")
    if p:
        return p
    for tpl in _WIN_CANDIDATES + _MAC_CANDIDATES + _LIN_CANDIDATES:
        for ver in _VERSIONS:
            cand = tpl.format(ver=ver)
            if os.path.isfile(cand):
                return cand
    return None


def php_syntax_error(content):
    """对完整 PHP 内容跑 php -l；返回错误摘要；php 不可用或无错返回 None。"""
    php = resolve_php()
    if not php:
        return None
    tmp = None
    try:
        fd, tmp = tempfile.mkstemp(suffix=".php")
        with os.fdopen(fd, "w", encoding="utf-8", newline="\n") as f:
            f.write(content)
        r = subprocess.run([php, "-l", tmp], capture_output=True, text=True, timeout=20)
        out = (r.stdout or "") + (r.stderr or "")
        if "Parse error" in out or "Errors parsing" in out:
            for ln in out.splitlines():
                if "Parse error" in ln or "Errors parsing" in ln:
                    # 回传时隐藏临时文件路径，避免与目标文件混淆
                    return ln.strip().replace(tmp, "<待写入内容>")
            return "Parse error"
        return None
    except Exception:
        return None
    finally:
        if tmp:
            try:
                os.remove(tmp)
            except OSError:
                pass


# --- C2. PHP8 兼容（规则取自 php8_compat.py，对齐 Rules §6） ---
# `=` 后不加 \b：`<?= $x ?>` 的 `=` 后是空格或 `$`，无词边界，加 \b 会把短回显标签误判为短标签
PHP8_SHORT_TAG = re.compile(r'<\?(?!php\b|xml\b|=)', re.IGNORECASE)
PHP8_BARE_KEY = re.compile(r'\$\w+\[([A-Za-z_][A-Za-z0-9_]*)\]')
PHP8_BAD_FUNCS = [
    (re.compile(r'\beach\s*\('), "each()"),
    (re.compile(r'create' + r'_function\s*\('), "create_function()"),
    (re.compile(r'\bget_magic_quotes_gpc\s*\('), "get_magic_quotes_gpc()"),
]
PHP8_NULL_COALESCE = re.compile(
    r'(!\s*\$\w+\s*\?\?)'              # !$x ??
    r'|(\?\?\s*[\'"]{0,2}\s*\.\s*\$?\w)'  # ?? '' . $b
)
PHP8_DQ_NULL = re.compile(r'"[^"]*\{\$[^}]*?\?\?[^}]*?\}[^"]*"')
PHP8_CLASS_EXISTS = re.compile(r'\bclass_exists\s*\(\s*[\'"][\\\\\w]+[\'"]\s*\)')
PHP8_FETCH1 = re.compile(r'\bfetch1\s*\((?:[^()]|\([^()]*\))*\)\s*\?\?')

# --- C3. SELECT *（对齐 Rules §10 性能禁则；规则取自 select_star.py） ---
SELECT_STAR_RE = re.compile(r"select\s+\*", re.IGNORECASE)

# --- C4. 硬编码凭证 / 日志含敏感变量（对齐 Rules §5/§19；规则取自 secret_scan.py，拼接构造防杀软） ---
CREDS_RE = re.compile(
    "(pass" + "word|pass" + "_word|pass" + "wd|pwd|api" + "_key|sec" + "ret|"
    "tok" + "en|access" + "_key|db" + "pass|db" + "_pass)"
    r"\s*=\s*[\"'][^\"']{4,}[\"']"
)
LOG_FN_RE = re.compile("(?:error" + "_log|sys" + "log|file" + "_put_contents)")
SENS_VAR_RE = re.compile("(?:pass" + "word|tok" + "en|sec" + "ret|phone|mobile|api" + "_key)")

BOM_CHAR = "\ufeff"


def scan_text_rules(text):
    """逐行扫描 PHP8 兼容 / 性能 / 凭证类规则（跳过注释行），返回描述列表。"""
    out = []
    for i, line in enumerate(text.splitlines(), start=1):
        s = line.strip()
        if (s.startswith("//") or s.startswith("#") or s.startswith("*")
                or s.startswith("/*")):
            continue
        if PHP8_SHORT_TAG.search(line):
            out.append("行 %d：短标签 <? 须改 <?php（Rules §6）" % i)
        for mk in PHP8_BARE_KEY.finditer(line):
            key = mk.group(1)
            if key.isupper() or key.lower() in ("true", "false", "null"):
                continue
            out.append("行 %d：裸数组键 $arr[%s] 须加引号（Rules §6）" % (i, key))
            break
        for rx, name in PHP8_BAD_FUNCS:
            if rx.search(line):
                out.append("行 %d：已废弃函数 %s（Rules §6）" % (i, name))
        if SELECT_STAR_RE.search(line):
            out.append("行 %d：SELECT * 裸查询，须显式列名（Rules §10）" % i)
        if CREDS_RE.search(line):
            out.append("行 %d：疑似硬编码明文凭证（Rules §5）" % i)
        elif LOG_FN_RE.search(line) and SENS_VAR_RE.search(line):
            out.append("行 %d：日志/写入可能含敏感变量（Rules §5/§19）" % i)
    if PHP8_NULL_COALESCE.search(text):
        out.append("?? 优先级陷阱（!$x??'' 或 $a??''.$b），须用 empty()/加括号（Rules §6）")
    if PHP8_DQ_NULL.search(text):
        out.append("双引号内 {$var??''} 存在解析歧义（Rules §6）")
    if PHP8_CLASS_EXISTS.search(text):
        out.append("class_exists() 缺第二参数（Rules §6）")
    if PHP8_FETCH1.search(text):
        out.append("fetch1() 返回 false 时 ?? 不拦截，须用 ?: []（Rules §6）")
    return out


# --- C6. SQL 注入（**只纳「超全局直接入 SQL 字符串」这一类**；框架惯用的
#     字符串拼接 / 双引号插值不纳入——那是既有风格，阻断会卡死正常改动。对齐 Rules §5） ---
SQL_KEYWORDS = (
    r"(?:SELECT\s|INSERT\s+INTO|UPDATE\s|DELETE\s+FROM|DROP\s+TABLE|"
    r"ALTER\s+TABLE|CREATE\s+TABLE|UNION\s+SELECT|TRUNCATE\s+TABLE|"
    r"REPLACE\s+INTO|LOAD\s+DATA\s+INFILE|LOAD\s+DATA\s+LOCAL\s+INFILE|"
    r"INTO\s+OUTFILE|INTO\s+DUMPFILE|GRANT\s|REVOKE\s|CALL\s)"
)
SQL_INJECT = re.compile(
    r'["\'][^"\']*' + SQL_KEYWORDS
    + r'[^"\']*\$_(?:GET|POST|REQUEST|COOKIE)\s*\[',
    re.IGNORECASE,
)

# --- C6b. CMS 查询门面 + 字符串 + 超全局（覆盖常见 CMS 数据访问层，可按需追加） ---
CMS_SQL_QUERY = re.compile(
    r"(?:empire->query|empire->fetch|db->query|db->fetchAll)\s*\(\s*"
    r"[\"'][^\"']*\$_(?:GET|POST|REQUEST)\b", re.IGNORECASE)
# ⚠ 不可用 `.*?`：会跨出引号匹配到 `"..." . intval($_GET['id'])` 这类
#   **已在字符串外被数值化包裹**的写法（实测误报）。`[^"']*` 把匹配限制在同一字符串内。
DEDESQL_QUERY = re.compile(
    r"dsql->(?:ExecuteNoneQuery|Execute|GetOne)\s*\(\s*"
    r"[\"'][^\"']*\$_(?:GET|POST|REQUEST)\b", re.IGNORECASE)

# --- C9. `/e` 修饰符代码执行（PHP<7；正则已收紧，防普通 e 字母误报。对齐 Rules §6） ---
PREG_E_EXEC = re.compile(
    r"preg_replace\s*\(\s*['\"]([/#~|%!@;,^]).*?\1[a-z]*e['\"]", re.IGNORECASE)

# --- C7. extract() 接超全局（新代码不应出现，对齐 Rules §5） ---
EXTRACT_UNTRUST = re.compile(
    r"\bext" + r"ract\s*\(\s*\$(?:_(?:POST|GET|REQUEST|COOKIE|FILES|SERVER|ENV)\b)",
    re.IGNORECASE,
)

# --- C8. 反射型 XSS（echo/print + 未转义超全局；**参数级豁免**，对齐 Rules §5） ---
XSS_REFLECT = re.compile(
    r"(?:echo|print|printf)\b[^;]{0,80}\$_(?:GET|POST|REQUEST)\b", re.IGNORECASE)
ESCAPE_NAMES = {"htmlspecialchars", "htmlentities", "strip_tags", "esc_html",
                "esc_attr", "esc_url", "esc_js", "wp_kses",
                "sanitize_text_field", "e"}
_USER_RE = re.compile(r"\$_(?:GET|POST|REQUEST|COOKIE|SERVER)\b", re.IGNORECASE)


def _wrapped_at(line, pos, names):
    """pos 处的用户输入是否被 names 中的函数调用包裹（向左找未闭合左括号）。"""
    depth = 0
    i = pos - 1
    while i >= 0:
        c = line[i]
        if c == ")":
            depth += 1
        elif c == "(":
            if depth == 0:
                j = i - 1
                while j >= 0 and line[j] in " \t":
                    j -= 1
                k = j
                while k >= 0 and (line[k].isalnum() or line[k] == "_"):
                    k -= 1
                return line[k + 1:j + 1].lower() in names
            depth -= 1
        i -= 1
    return False


def _xss_exempt(line, span):
    """span 内**所有**用户输入都被转义函数包裹才豁免（同行另处未包裹仍报）。"""
    start, end = span
    pos = [m.start() for m in _USER_RE.finditer(line) if start <= m.start() < end]
    if not pos:
        return False
    return all(_wrapped_at(line, p, ESCAPE_NAMES) for p in pos)


def scan_sec_ext(text):
    """逐行扫描 SQL 注入 / extract / 反射型 XSS（跳过注释行）。"""
    out = []
    for i, line in enumerate(text.splitlines(), start=1):
        s = line.strip()
        if (s.startswith("//") or s.startswith("#") or s.startswith("*")
                or s.startswith("/*")):
            continue
        if SQL_INJECT.search(line):
            out.append("行 %d：SQL 注入——用户输入直接进入 SQL 字符串，须参数绑定（Rules §5）" % i)
        elif CMS_SQL_QUERY.search(line) or DEDESQL_QUERY.search(line):
            out.append("行 %d：CMS SQL 注入——DB 查询方法拼接用户输入（Rules §5）" % i)
        elif PREG_E_EXEC.search(line):
            out.append("行 %d：preg_replace 使用 /e 修饰符，PHP<7 可致代码执行（Rules §6）" % i)
        if EXTRACT_UNTRUST.search(line):
            out.append("行 %d：extract() 接超全局数组，禁止处理不可信输入（Rules §5）" % i)
        m = XSS_REFLECT.search(line)
        if m and not _xss_exempt(line, m.span()):
            out.append("行 %d：反射型 XSS——未转义输出用户输入，须 htmlspecialchars()（Rules §5）" % i)
    return out


MAX_DIFF_BYTES = 1024 * 1024  # 超限退化为全文，避免大文件读取拖慢写入


def new_lines_of(ti, path):
    """提取「本次新增的文本」——只检新增，避免覆盖写入时把历史存量当新问题。

    - `replace_in_file` → `new_str`（片段，天然只含本次改动）
    - `write_to_file` 且目标**已存在** → 与磁盘旧文件 diff，取 `+` 行
    - `write_to_file` 且目标**不存在** → `content` 全文（新建文件）
    返回 `(文本, 说明)`；读旧文件失败或超限 → 退化为全文并标注（不静默漏检）。
    """
    content = ti.get("content") or ""
    new_str = ti.get("new_str") or ""
    if new_str:
        return new_str, "replace 片段"
    if not content:
        return "", "无内容"
    if not os.path.isfile(path):
        return content, "新建全文"
    try:
        if os.path.getsize(path) > MAX_DIFF_BYTES:
            return content, "全文(超 1MB 退化)"
        import difflib
        with open(path, encoding="utf-8", errors="ignore") as f:
            old = f.read()
        added = [ln[1:] for ln in difflib.unified_diff(
            old.splitlines(), content.splitlines(), lineterm="", n=0)
            if ln.startswith("+") and not ln.startswith("+++")]
        return "\n".join(added), "新增行 %d" % len(added)
    except Exception:
        return content, "全文(旧文件读取失败)"


def main():
    try:
        raw = sys.stdin.buffer.read().decode("utf-8", "replace").lstrip("\ufeff")
        if not raw.strip():
            return 0
        data = json.loads(raw)
    except Exception:
        return 0

    ti = data.get("tool_input", {}) or {}
    path = (ti.get("filePath") or ti.get("file_path") or ti.get("path") or "")
    if not path or not path.lower().endswith(TARGET_EXT):
        return 0

    content = ti.get("content") or ""
    new_str = ti.get("new_str") or ""
    whole = content if content else new_str
    if not whole:
        return 0
    if ALLOW_MARK in whole:
        return 0  # 逃生阀：显式标注放行

    # 只检「本次新增的文本」（覆盖写入不把历史存量当新问题）
    scanned, how = new_lines_of(ti, path)
    if not scanned.strip():
        return 0  # 本次无新增内容（如内容未变），无需检查

    problems = []

    hit = [p for p in DEBUG_PATTERNS if p in scanned]
    if hit:
        problems.append("调试残留：%s（Rules §10）" % "、".join(hit))

    sec = scan_security(scanned)
    problems.extend(sec[:5])
    if len(sec) > 5:
        problems.append("…另有 %d 处安全问题未展开" % (len(sec) - 5))

    se2 = scan_sec_ext(scanned)
    problems.extend(se2[:5])
    if len(se2) > 5:
        problems.append("…另有 %d 处安全问题未展开" % (len(se2) - 5))

    tr = scan_text_rules(scanned)
    problems.extend(tr[:8])
    if len(tr) > 8:
        problems.append("…另有 %d 处文本类问题未展开" % (len(tr) - 8))

    # BOM 是「整个文件」的属性 → 用全文判定（BOM 自身即缺陷，不存在存量误报问题）
    if BOM_CHAR in whole:
        problems.append("内容含 UTF-8 BOM（\\ufeff），会导致 PHP 输出隐藏字节（Rules §8）")

    # 语法检查必须用全文（php -l 需要完整文件）
    if content.lstrip().startswith("<?"):
        err = php_syntax_error(content)
        if err:
            problems.append("PHP 语法错误：%s" % err)

    if not problems:
        return 0

    print("[PRE-CHECK] 写入前检查未通过（本次写入已被拒绝，文件未落盘）：")
    for p in problems:
        print("  - %s" % p)
    print("文件: %s（检查范围：%s）" % (path, how))
    print("处置：修正后重写；若确需原样写入（如记录示例代码），请在内容中加 %s 标记。" % ALLOW_MARK)
    return 2


if __name__ == "__main__":
    sys.exit(main())
