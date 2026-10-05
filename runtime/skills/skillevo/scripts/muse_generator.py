#!/usr/bin/env python3
"""
MUSE 测试用例生成器 —— 从 SKILL.md 自动提取可测试项，生成 tests.yaml。
用法: python muse_generator.py <skill_dir> [--merge]
      --merge: 与现有 tests.yaml 合并（去重追加），否则覆盖输出。
"""
import sys, json, re, hashlib, argparse
from pathlib import Path
from datetime import datetime, timezone, timedelta

try:
    from luban_common import extract_triggers
except ImportError:
    def extract_triggers(text: str, max_len: int = 20) -> set:
        triggers = set()
        desc_match = re.search(r"^description\s*:\s*(.+)", text, re.MULTILINE)
        if desc_match:
            desc = desc_match.group(1)
            for m in re.finditer(r"['\"「『]([^'\"」』]+)['\"」』]", desc):
                word = m.group(1).strip()
                if word and len(word) <= max_len:
                    triggers.add(word)
            for part in desc.split(","):
                part = part.strip().strip("'\"\\").strip()
                if part and len(part) <= max_len and not part.startswith(("触发", "如", "例", "当")):
                    triggers.add(part)
        for m in re.finditer(r"(?:触发|关键词)[：:]\s*(.+)", text):
            for word in re.split(r"[,，、/]", m.group(1)):
                word = word.strip().strip("'\"").strip()
                if word and len(word) <= 30:
                    triggers.add(word)
        return triggers

CST = timezone(timedelta(hours=8))


def hash_snapshot(filepath: Path) -> str:
    """计算文件的 SHA256 快照"""
    return hashlib.sha256(filepath.read_bytes()).hexdigest()


def extract_rules(text: str) -> dict[str, list[str]]:
    must = []
    forbid = []
    for line in text.split("\n"):
        line = line.strip()
        if not line:
            continue
        if "必须" in line:
            must.append(line[:120])
        if re.search(r"(?:禁止|严禁|不得)", line):
            forbid.append(line[:120])
    return {"must": must[:10], "forbid": forbid[:10]}


def extract_format_constraints(text: str) -> list[str]:
    constraints = []
    for m in re.finditer(r"(?:输出|格式|返回).*?(?:必须|要求|应|需).*?(?:JSON|YAML|Markdown|表格|列表|代码块)", text):
        constraints.append(m.group(0)[:120])
    return constraints[:5]


def extract_steps(text: str) -> list[str]:
    steps = re.findall(r"(?:Step\s*\d+|步骤\s*\d+)[：:\s]*(.+)", text)
    return steps[:10]


def generate_tests(skill_dir: Path) -> dict:
    sm = skill_dir / "SKILL.md"
    if not sm.exists():
        return {"error": f"SKILL.md 不存在: {sm}"}

    # P1: 修改前 hash 快照
    pre_hash = hash_snapshot(sm)

    text = sm.read_text(encoding="utf-8")
    triggers_list = sorted(extract_triggers(text))
    triggers = triggers_list if triggers_list else ["__placeholder__"]
    rules = extract_rules(text)
    formats = extract_format_constraints(text)
    steps = extract_steps(text)

    tests = []
    tid = 1

    # 1. 触发词识别
    for tw in triggers[:5]:
        tests.append({
            "id": f"trigger_{tid:03d}",
            "dimension": "触发词识别",
            "input": tw,
            "condition": f"用户输入包含触发词 '{tw}'",
            "expected": "技能被触发并进入对应流程",
            "status": "pending",
            "generated_at": datetime.now(tz=CST).strftime("%Y-%m-%d"),
        })
        tid += 1

    # 2. 输出格式
    for fc in formats[:3]:
        tests.append({
            "id": f"format_{tid:03d}",
            "dimension": "输出格式",
            "condition": fc,
            "expected": "输出符合约束",
            "status": "pending",
            "generated_at": datetime.now(tz=CST).strftime("%Y-%m-%d"),
        })
        tid += 1

    # 3. 关键规则遵守 - 必须
    for rule in rules["must"][:3]:
        tests.append({
            "id": f"must_{tid:03d}",
            "dimension": "关键规则遵守",
            "condition": f"验证规则: {rule}",
            "expected": "技能执行时遵守该规则",
            "status": "pending",
            "generated_at": datetime.now(tz=CST).strftime("%Y-%m-%d"),
        })
        tid += 1

    # 4. 关键规则遵守 - 禁止
    for rule in rules["forbid"][:2]:
        tests.append({
            "id": f"forbid_{tid:03d}",
            "dimension": "关键规则遵守",
            "condition": f"验证规则: {rule}",
            "expected": "技能执行时不违反该规则",
            "status": "pending",
            "generated_at": datetime.now(tz=CST).strftime("%Y-%m-%d"),
        })
        tid += 1

    # 5. 流程完整性
    if steps:
        for i, step in enumerate(steps[:3]):
            tests.append({
                "id": f"flow_{tid:03d}",
                "dimension": "流程完整性",
                "condition": f"检查步骤 {i+1}: {step}",
                "expected": "技能按步骤顺序执行，无遗漏",
                "status": "pending",
                "generated_at": datetime.now(tz=CST).strftime("%Y-%m-%d"),
            })
            tid += 1
    else:
        tests.append({
            "id": f"flow_{tid:03d}",
            "dimension": "流程完整性",
            "condition": "检查 SKILL.md 中是否包含步骤/流程定义",
            "expected": "技能包含明确的执行步骤",
            "status": "pending",
            "generated_at": datetime.now(tz=CST).strftime("%Y-%m-%d"),
        })
        tid += 1

    # 6. references 可达性
    ref_dir = skill_dir / "references"
    if ref_dir.exists():
        for ref_file in ref_dir.rglob("*"):
            if ref_file.is_file():
                rel = str(ref_file.relative_to(skill_dir)).replace("\\", "/")
                tests.append({
                    "id": f"refs_{tid:03d}",
                    "dimension": "references 可达性",
                    "condition": f"检查文件是否存在: {rel}",
                    "expected": "文件存在且可读取",
                    "status": "pending",
                    "generated_at": datetime.now(tz=CST).strftime("%Y-%m-%d"),
                })
                tid += 1

    # 7. 反例测试（构造明确不在范围内的输入）
    tests.append({
        "id": f"negative_{tid:03d}",
        "dimension": "反例测试",
        "condition": "输入与技能无关的内容（如'帮我推荐一部电影'或纯闲聊）",
        "expected": "技能不被触发，不产生误输出",
        "status": "pending",
        "generated_at": datetime.now(tz=CST).strftime("%Y-%m-%d"),
    })
    tid += 1

    return {
        "skill": sm.stem,
        "generated_at": datetime.now(tz=CST).strftime("%Y-%m-%d %H:%M:%S"),
        "pre_hash": pre_hash,
        "total_tests": len(tests),
        "tests": tests,
    }


def merge_with_existing(new_data: dict, existing_path: Path) -> dict:
    try:
        import yaml
        existing = yaml.safe_load(existing_path.read_text(encoding="utf-8")) or {}
    except Exception:
        existing = {}

    existing_tests = existing.get("tests", [])
    existing_ids = {t["id"] for t in existing_tests}

    for test in new_data["tests"]:
        if test["id"] not in existing_ids:
            existing_tests.append(test)
            existing_ids.add(test["id"])

    new_data["tests"] = existing_tests
    new_data["total_tests"] = len(existing_tests)
    return new_data


def output_yaml(data: dict, filepath: Path):
    """输出为 YAML 格式，字段对齐 SKILL.md 模板（condition/check 替代 rule/constraint/file）"""
    lines = [
        f"skill: {data['skill']}",
        f"generated_at: {data['generated_at']}",
        f"pre_hash: {data.get('pre_hash', '')}",
        f"total_tests: {data['total_tests']}",
        "tests:",
    ]
    for t in data["tests"]:
        lines.append(f"  - id: {t['id']}")
        lines.append(f"    dimension: {t['dimension']}")
        for key in ["input", "condition"]:
            if key in t and t[key]:
                val = t[key].replace('"', '\\"')
                lines.append(f'    {key}: "{val}"')
        if "check" in t and t["check"]:
            val = t["check"].replace('"', '\\"')
            lines.append(f'    check: "{val}"')
        if "expected" in t:
            val = t["expected"].replace('"', '\\"')
            lines.append(f'    expected: "{val}"')
        lines.append(f"    status: {t['status']}")
        lines.append(f"    generated_at: {t['generated_at']}")
    filepath.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main():
    parser = argparse.ArgumentParser(description="MUSE 测试用例生成器")
    parser.add_argument("skill_dir", help="目标 skill 目录路径")
    parser.add_argument("--merge", action="store_true", help="与现有 tests.yaml 合并（去重追加），否则覆盖输出")
    args = parser.parse_args()

    sd = Path(args.skill_dir).resolve()
    new_data = generate_tests(sd)
    out_path = sd / "tests.yaml"

    if args.merge and out_path.exists():
        new_data = merge_with_existing(new_data, out_path)

    output_yaml(new_data, out_path)

    report = {
        "skill": new_data["skill"],
        "total_tests": new_data["total_tests"],
        "output": str(out_path),
        "dimensions": list(set(t["dimension"] for t in new_data["tests"])),
    }
    print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
