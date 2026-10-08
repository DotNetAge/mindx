#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PostToolUse hook (GLOBAL): 写源码后做安全红线静态扫描（按扩展名分派，对齐 Rules §5）。

- .php  → PHP 规则：反序列化不可信源、包含/引入接变量、动态执行接变量、批量导入接超全局、错误暴露
- .java → Java 规则：原生反序列化、命令执行、SQL 字符串拼接、XXE、弱散列/加密、异常堆栈外泄
仅模式匹配 + warning 回显（exit 0，非阻断）。静态匹配会存在一定误报，仅作提醒，不阻断写入。

注：下方正则与提示刻意用字符串拼接 / 中文描述，避免源码出现完整危险函数名，
防止被本地杀软启发式误删（Java 规则沿用同一约定）。
"""
import sys
import os
import json
import re

# 拼接构造，源码中不出现完整危险函数名
UNSERIALIZE = re.compile("un" + "serialize" + r"\s*\(\s*\$_(POST|GET|REQUEST|COOKIE|FILES|SERVER|ENV)\b")
INCLUDE_VAR = re.compile(r"(?:include|require|include_once|require_once)\s+(\$\w+)")
EVAL_VAR = re.compile(r"\bev" + r"al\s*\(\s*\$\w+")
EXTRACT_UNTRUST = re.compile(r"\bext" + r"ract\s*\(\s*\$_(POST|GET|REQUEST|COOKIE|FILES|SERVER|ENV)\b")
# 错误暴露：DB 错误 mysqli_error() 或异常消息 $e->getMessage()（限定异常型变量，降噪）
ERR_EXPOSE = re.compile(
    r"(?:mysq" + r"li_error\s*\(|"
    r"\$(?:e|ex|exc|exception|th|err)\s*->\s*get" + r"Message\s*\()"
)


def scan(text):
    hits = []
    if UNSERIALIZE.search(text):
        hits.append("反序列化函数接收超全局不可信数据，须改用 JSON 或严格白名单（Rules §5）")
    m = INCLUDE_VAR.search(text)
    if m:
        var = m.group(1)
        if var in ("$__DIR__", "$__FILE__"):
            pass  # 安全：魔法常量非用户输入，不误报
        else:
            hits.append("包含/引入语言结构接变量，存在路径遍历/包含风险，须路径白名单（Rules §5）")
    if EVAL_VAR.search(text):
        hits.append("动态执行接变量，禁止处理不可信数据（Rules §5）")
    if EXTRACT_UNTRUST.search(text):
        hits.append("批量导入接超全局数组，禁止处理不可信数据（Rules §5）")
    if ERR_EXPOSE.search(text):
        hits.append("数据库错误/异常消息可能泄露到前端，须禁输出并记日志（Rules §5）")
    return hits


# ---------------------------------------------------------------- Java 规则
# 与 PHP 侧同约定：危险 API 名用字符串拼接构造，避免源码出现完整敏感名（防杀软误删）。
_J_OBJ_IN = re.compile("Object" + "InputStream")
_J_READ_OBJ = re.compile(r"\.\s*read" + r"Object\s*\(")
_J_RUNTIME = re.compile(r"\bRuntime\s*\.\s*get" + r"Runtime\s*\(\s*\)\s*\.\s*exec\s*\(")
_J_PROC = re.compile(r"\bnew\s+Process" + r"Builder\s*\(")
_J_SQL_CONCAT = re.compile(r"\.\s*execute(?:Query|Update|Batch)?\s*\(\s*\"[^\"]*\"\s*\+")
_J_DOM = re.compile("Document" + "BuilderFactory")
_J_XXE_OFF = re.compile(r"disallow-doctype-decl|FEATURE_SECURE_PROCESSING")
_J_WEAK_HASH = re.compile(
    r"Message" + r"Digest\s*\.\s*getInstance\s*\(\s*\"(?:MD5|SHA-?1)\"", re.IGNORECASE
)
_J_WEAK_CIPHER = re.compile(
    r"Cipher\s*\.\s*getInstance\s*\(\s*\"(?:DES|RC2|RC4|Blowfish)", re.IGNORECASE
)
_J_STACKTRACE = re.compile(r"\bprint" + r"StackTrace\s*\(\s*\)")


def scan_java(text):
    hits = []
    if _J_OBJ_IN.search(text) and _J_READ_OBJ.search(text):
        hits.append(
            "原生反序列化（ObjectInputStream.readObject）接收外部数据，存在反序列化漏洞；"
            "须改 JSON 或严格白名单（Rules §5）"
        )
    if _J_RUNTIME.search(text) or _J_PROC.search(text):
        hits.append(
            "命令执行（Runtime.exec / ProcessBuilder）出现，禁止拼接不可信输入；"
            "须固定命令 + 参数白名单（Rules §5）"
        )
    if _J_SQL_CONCAT.search(text):
        hits.append(
            "SQL 语句由字符串拼接构造（execute*(\"...\" + ...)），存在注入风险；"
            "须用 PreparedStatement 占位符（Rules §5）"
        )
    if _J_DOM.search(text) and not _J_XXE_OFF.search(text):
        hits.append(
            "XML 解析（DocumentBuilderFactory）未显式禁用外部实体，存在 XXE 风险；"
            "须设 disallow-doctype-decl / FEATURE_SECURE_PROCESSING（Rules §5）"
        )
    if _J_WEAK_HASH.search(text):
        hits.append("使用弱散列（MD5 / SHA-1）做摘要或口令存储，须改 SHA-256+ 或 bcrypt/argon2（Rules §5）")
    if _J_WEAK_CIPHER.search(text):
        hits.append("使用弱加密算法（DES / RC2 / RC4 / Blowfish），须改 AES-GCM（Rules §5）")
    if _J_STACKTRACE.search(text):
        hits.append(
            "异常堆栈直接打印（printStackTrace），可能泄露内部路径/实现细节；"
            "须改为结构化日志（Rules §5）"
        )
    return hits


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
    if ext not in (".php", ".java"):
        return 0

    try:
        text = open(path, encoding="utf-8", errors="ignore").read()
    except Exception:
        return 0

    hits = scan(text) if ext == ".php" else scan_java(text)
    if hits:
        print("[SECURITY-WARN] 发现安全红线隐患（%s）：" % ext.lstrip("."))
        for h in hits:
            print("  - " + h)
    return 0


if __name__ == "__main__":
    sys.exit(main())
