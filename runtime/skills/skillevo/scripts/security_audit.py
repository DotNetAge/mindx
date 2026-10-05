#!/usr/bin/env python3
"""
Sentinel —— luban 内嵌安全审计模块（Phase 0.3）。
对目标 skill 的 scripts/ + references/ 做正则安全扫描，输出 JSON 评分报告。

用法：
    python security_audit.py <skill_dir> [--diagnostics <path>]

输出格式（JSON）：
    包含 5 类 Sentinel 子分（0-2 评分制）、汇总和命中详情。
"""

import os
import re
import sys
import json
import argparse
from collections import defaultdict

RULES = [
    {
        "category": "恶意指令",
        "dim": "dim10",
        "patterns": [
            (r'\bexec\s*\(',           "exec() 系统调用"),
            (r'\bsystem\s*\(',         "system() 系统调用"),
            (r'\bsubprocess\b',        "subprocess 模块调用"),
            (r'\bos\.system\b',        "os.system() 调用"),
            (r'\bpopen\b',             "popen 管道执行"),
            (r'\brm\s+-rf\b',          "rm -rf 递归强制删除"),
            (r'\bdel\s+/[fFqQ]',       "del /f 强制删除"),
            (r'\bformat\s+[a-zA-Z]:',  "format 磁盘格式化"),
            (r'\breg\s+delete\b',      "reg delete 注册表删除"),
            (r'\bkill\s+-9\b',         "kill -9 强制杀进程"),
        ],
    },
    {
        "category": "硬编码凭据",
        "dim": "dim10",
        "patterns": [
            (r'(?i)(api[_-]?key|apikey)\s*[=:]\s*["\'][a-zA-Z0-9_\-]{8,}', "API Key 明文赋值"),
            (r'(?i)password\s*[=:]\s*["\'][^"\']{3,}', "password 明文赋值"),
            (r'(?i)token\s*[=:]\s*["\'][a-zA-Z0-9_\-\.]{8,}', "token 明文赋值"),
            (r'(?i)secret\s*[=:]\s*["\'][a-zA-Z0-9_\-]{6,}', "secret 明文赋值"),
            (r'-----BEGIN\s+(RSA|EC|DSA|OPENSSH)?\s*PRIVATE KEY-----', "私钥明文嵌入"),
        ],
    },
    {
        "category": "Prompt 注入",
        "dim": "dim10",
        "patterns": [
            (r'(?i)\bDAN\b', "DAN 模式关键词"),
            (r'(?i)jailbreak', "jailbreak 关键词"),
            (r'(?i)simulate\s+(that\s+)?(you\s+are|being)', "模拟角色扮演注入"),
            (r'(?i)system\s+override', "系统指令覆盖"),
            (r'(?i)ignore\s+(all\s+)?(previous|above|prior).*instruction', "忽略之前指令"),
            (r'(?i)developer\s*mode', "developer mode 诱导"),
            (r'(?i)roleplay\s+as', "角色扮演注入"),
            (r'(?i)you\s+are\s+now\s+.*(free|unrestricted|unlimited)', "解除限制注入"),
        ],
    },
    {
        "category": "数据外泄",
        "dim": "dim10",
        "patterns": [
            (r'(?i)smtp\s*(lib|send|connect)',     "SMTP 邮件外发"),
            (r'(?i)upload.*(external|remote)',      "上传到外部服务器"),
            (r'(?i)requests\.post\b',                "HTTP POST 外发数据"),
            (r'(?i)ftp\s*(upload|put|store)',        "FTP 上传"),
            (r'(?i)curl\s+.*\b(upload|post|put)\b', "curl 上传数据"),
            (r'(?i)scp\b',                           "SCP 文件传输"),
        ],
    },
    {
        "category": "权限越权",
        "dim": "dim10",
        "patterns": [
            (r'\bchmod\s+[0-7]{3,4}\b',  "chmod 权限修改"),
            (r'\bchown\b',                "chown 所有者变更"),
            (r'\bsudo\b',                 "sudo 提权"),
            (r'\bsu\s+-',                 "su 切换用户"),
            (r'\bicacls\b',               "icacls Windows 权限修改"),
        ],
    },
]


def find_text_files(skill_dir: str) -> list[str]:
    text_exts = {".md", ".py", ".sh", ".json", ".yaml", ".yml", ".toml", ".cfg", ".txt", ".tsv", ".csv"}
    files = []
    scan_dirs = [
        os.path.join(skill_dir, "scripts"),
        os.path.join(skill_dir, "references"),
    ]

    for d in scan_dirs:
        if not os.path.isdir(d):
            continue
        for root, _, filenames in os.walk(d):
            for fname in filenames:
                ext = os.path.splitext(fname)[1].lower()
                if ext in text_exts:
                    fp = os.path.join(root, fname)
                    if os.path.abspath(fp) == os.path.abspath(__file__):
                        continue
                    files.append(fp)
    return files


def scan_file(filepath: str) -> dict[str, list[dict]]:
    hits: dict[str, list[dict]] = defaultdict(list)
    try:
        with open(filepath, "r", encoding="utf-8", errors="ignore") as f:
            lines = f.readlines()
    except Exception:
        return hits

    for rule in RULES:
        cat = rule["category"]
        for pattern, desc in rule["patterns"]:
            for i, line in enumerate(lines, 1):
                if re.search(pattern, line):
                    hits[cat].append({
                        "category": cat,
                        "dim": rule["dim"],
                        "file": filepath,
                        "line": i,
                        "detail": f"[{cat}] {desc}: {line.strip()[:120]}"
                    })
    return hits


def compute_sentinel_score(cat_hits: dict[str, list[dict]]) -> dict:
    """按 SKILL.md 定义的 0-2 评分制计算各类别子分。
    每类：0 命中 = 2, 1 处 = 1, ≥2 处 = 0; 5 类取均值。
    """
    category_scores = {}
    for rule in RULES:
        cat = rule["category"]
        hit_count = len(cat_hits.get(cat, []))
        if hit_count == 0:
            score = 2.0
        elif hit_count == 1:
            score = 1.0
        else:
            score = 0.0
        category_scores[cat] = {"hit_count": hit_count, "score": score}

    avg_score = sum(v["score"] for v in category_scores.values()) / len(category_scores) if category_scores else 2.0
    total_hits = sum(v["hit_count"] for v in category_scores.values())

    return {
        "category_scores": category_scores,
        "sentinel_sub_score": round(avg_score, 2),
        "total_hits": total_hits,
    }


def write_diagnostics(hits: dict[str, list[dict]], score_result: dict, diag_path: str):
    os.makedirs(os.path.dirname(diag_path), exist_ok=True)
    with open(diag_path, "w", encoding="utf-8") as f:
        f.write("模块\t维度\t子分\t文件\t行号\t详情\n")
        for cat, items in hits.items():
            sub_score = score_result["category_scores"].get(cat, {}).get("score", 2)
            for item in items:
                f.write(f"Sentinel\tdim10\t{sub_score}\t{item['file']}\t{item['line']}\t{item['detail']}\n")


def main():
    parser = argparse.ArgumentParser(description="Sentinel 安全审计")
    parser.add_argument("skill_dir", help="目标 skill 目录路径")
    parser.add_argument("--diagnostics", default=None, help="diagnostics.tsv 路径")
    args = parser.parse_args()

    if not os.path.isdir(args.skill_dir):
        result = {"status": "error", "error": f"目录不存在: {args.skill_dir}"}
        print(json.dumps(result, ensure_ascii=False, indent=2))
        sys.exit(1)

    files = list(set(find_text_files(args.skill_dir)))

    all_cat_hits: dict[str, list[dict]] = defaultdict(list)
    for fp in files:
        file_hits = scan_file(fp)
        for cat, items in file_hits.items():
            all_cat_hits[cat].extend(items)

    score_result = compute_sentinel_score(all_cat_hits)

    hit_details = []
    for cat in sorted(all_cat_hits):
        for item in all_cat_hits[cat]:
            hit_details.append({
                "category": cat,
                "file": item["file"],
                "line": item["line"],
                "detail": item["detail"],
            })

    report = {
        "status": "ok",
        "skill_dir": os.path.abspath(args.skill_dir),
        "files_scanned": len(files),
        "total_hits": score_result["total_hits"],
        "sentinel_sub_score": score_result["sentinel_sub_score"],
        "category_scores": score_result["category_scores"],
        "hit_details": hit_details,
    }

    if args.diagnostics and all_cat_hits:
        write_diagnostics(all_cat_hits, score_result, args.diagnostics)

    print(json.dumps(report, ensure_ascii=False, indent=2))
    sys.exit(0)


if __name__ == "__main__":
    main()
