#!/usr/bin/env python3
"""资源社区发布校验（决策路由的确定性判定来源）。

用法：
  python3 release_check.py --phase pre  --kind skill --name copywriting --version 2.2.0
  python3 release_check.py --phase post --kind skill --name copywriting --version 2.2.0 [--org slug]

输出：stdout 一份 JSON（verdict = pass|warn|fail，checks 逐项带 status/detail/fix），
退出码 0 = pass/warn，1 = fail。进度与告警走 stderr，不污染 JSON。

设计约束：全部输入走命令行参数，禁止交互式提示；不依赖第三方库；不猜远端状态——
一切判定以 mindx CLI 实际输出为准。
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from pathlib import Path

MINDX_BIN = os.environ.get("MINDX_BIN") or str(Path.home() / ".mindx" / "bin" / "mindx")
# 表格输出带 ANSI 颜色（列表类命令），解析前必须剥离，否则名字对不上
ANSI_RE = re.compile(r"\x1b\[[0-9;]*m")
HOME = Path.home()
KINDS = ("agent", "skill", "team")

ENTITY_FILE = {
    "skill": HOME / ".mindx" / "skills" / "{name}" / "SKILL.md",
    "agent": HOME / ".mindx" / "agents" / "{name}" / "IDENTITY.md",
    "team": HOME / ".mindx" / "teams" / "{name}" / "TEAM.md",
}


def log(msg: str) -> None:
    print(f"[release-check] {msg}", file=sys.stderr)


def run_mindx(args: list[str], timeout: int = 90) -> tuple[int, str]:
    """跑 mindx 子命令，返回 (退出码, stdout)。daemon 不可达时退出码非零、stdout 可能为空。"""
    try:
        proc = subprocess.run(
            [MINDX_BIN, *args],
            capture_output=True,
            text=True,
            timeout=timeout,
            env={**os.environ, "NO_COLOR": "1", "TERM": "dumb"},
        )
    except FileNotFoundError:
        log(f"找不到 mindx 可执行文件：{MINDX_BIN}（可用 MINDX_BIN 覆盖）")
        return 127, ""
    except subprocess.TimeoutExpired:
        log(f"mindx {' '.join(args)} 超时（{timeout}s）")
        return 124, ""
    return proc.returncode, ANSI_RE.sub("", proc.stdout)


def parse_json_stdout(stdout: str) -> dict | None:
    """stdout 前部混有 INFO 日志，从第一个 '{' 开始解析。"""
    idx = stdout.find("{")
    if idx < 0:
        return None
    try:
        return json.loads(stdout[idx:])
    except json.JSONDecodeError:
        return None


def read_frontmatter(path: Path) -> dict:
    """解析 frontmatter：只取 version 与块列表（skills / members），足够判定。

    技能版本写在 `metadata.version`（嵌套一级），agent/team 写顶层 `version`：
    两处都归一成 `version` 键，避免读音分叉。
    """
    if not path.is_file():
        return {}
    text = path.read_text(encoding="utf-8")
    if not text.startswith("---"):
        return {}
    end = text.find("\n---", 3)
    if end < 0:
        return {}
    body = text[3:end]
    data: dict = {}
    current_list: str | None = None
    parent: str | None = None
    for raw in body.splitlines():
        if re.match(r"^\s{2,}-\s+", raw) and current_list:
            data[current_list].append(raw.split("-", 1)[1].strip())
            continue
        indent = len(raw) - len(raw.lstrip())
        m = re.match(r"^([A-Za-z_]+):\s*(.*)$", raw.strip())
        if not m:
            continue
        key, value = m.group(1), m.group(2).strip()
        if indent == 0:
            parent = None
        if value == "":
            current_list = key
            parent = key
            data.setdefault(key, [])
            continue
        current_list = None
        if indent > 0 and parent:
            data[f"{parent}.{key}"] = value
            if key == "version":
                data.setdefault("version", value)
        else:
            data[key] = value
    return data


def registered_names(entity: str) -> set[str] | None:
    """本机注册表里的实体名集合；命令失败返回 None（不判定）。

    两种表格形态：agent/team list 用 `│` 分列，skill list 用固定宽度空格对齐——
    按 `│` 优先、退化为「首列 + 两空格以上」解析，否则技能引用会被误判为缺失。
    """
    code, out = run_mindx([entity, "list"])
    if code != 0:
        return None
    names: set[str] = set()
    for line in out.splitlines():
        if "│" in line:
            head = line.split("│", 1)[0].strip()
        else:
            m = re.match(r"^([A-Za-z0-9_.\-]+)\s{2,}", line)
            head = m.group(1) if m else ""
        if head and head != "Name" and not head.startswith("─"):
            names.add(head)
    return names


def find_package(kind: str, name: str) -> tuple[dict | None, dict | None]:
    """查社区同名包（精确名匹配）。--q 是模糊检索且会命中包内技能名，必须逐条精确比对。"""
    code, out = run_mindx(["market", "list", "--kind", kind, "--q", name, "--json"])
    if code != 0:
        return None, None
    payload = parse_json_stdout(out)
    if not payload:
        return None, None
    exact = None
    similar = []
    for pkg in payload.get("packages") or []:
        if pkg.get("name") == name:
            exact = pkg
        elif name in (pkg.get("name") or ""):
            similar.append(pkg.get("name"))
    return exact, {"similar": similar}


def check_phase_pre(args) -> tuple[list[dict], dict]:
    checks: list[dict] = []
    ctx: dict = {}

    # 1) 登录态与发布者 UUID
    code, out = run_mindx(["market", "whoami", "--json"])
    payload = parse_json_stdout(out) if code == 0 else None
    uid = ""
    if not payload:
        checks.append({
            "id": "login",
            "status": "fail",
            "detail": "无法读取社区账号状态（daemon 未运行或未登录）",
            "fix": "先启动 MindX（mindx start），在账户页登录社区账号后重跑",
        })
    elif not payload.get("has_token"):
        checks.append({
            "id": "login",
            "status": "fail",
            "detail": "当前设备未绑定社区账号（匿名身份不能发布）",
            "fix": "在 MindX 账户页登录社区账号后重跑",
        })
    elif payload.get("token_error"):
        checks.append({
            "id": "login",
            "status": "fail",
            "detail": f"社区令牌校验失败：{payload['token_error']}",
            "fix": "重新登录刷新令牌后重跑",
        })
    else:
        user = payload.get("user") or {}
        uid = (user.get("id") or "").strip()
        checks.append({
            "id": "login",
            "status": "pass" if uid else "fail",
            "detail": f"已登录：{user.get('nick_name') or user.get('login') or '未知'}（{uid or '缺少 UUID'}）",
            "fix": "" if uid else "账号档案缺少 id，无法冻结发布者身份；联系社区支持",
        })

    # 2) 源文件版本 = 目标版本（daemon 会比对，不一致直接拒绝）
    path = Path(str(ENTITY_FILE[args.kind]).format(name=args.name))
    fm = read_frontmatter(path)
    local_version = str(fm.get("version") or "").strip()
    ctx["definition_path"] = str(path)
    ctx["local_version"] = local_version
    if not path.is_file():
        checks.append({
            "id": "definition",
            "status": "fail",
            "detail": f"本机不存在该实体定义：{path}",
            "fix": "确认名字与类型；未创建的先用对应的 creator 技能创建",
        })
    elif not local_version:
        checks.append({
            "id": "definition",
            "status": "warn",
            "detail": f"定义文件未声明 version：{path}",
            "fix": "在 frontmatter 写入 version，发布链路以它为准",
        })
    elif local_version != args.version:
        checks.append({
            "id": "definition",
            "status": "fail",
            "detail": f"目标版本 {args.version} 与源文件 frontmatter 版本 {local_version} 不一致",
            "fix": f"统一版本：把 {path} 的 version 改为 {args.version}（或按源文件版本发布）",
        })
    else:
        checks.append({
            "id": "definition",
            "status": "pass",
            "detail": f"源文件与目标版本一致：{local_version}",
            "fix": "",
        })

    # 3) 版本占用（版本字节终身绑定，同版本重发必然 409）
    exact, meta = find_package(args.kind, args.name)
    ctx["remote"] = None if exact is None else {
        "version": exact.get("version"),
        "ownerId": exact.get("ownerId"),
        "publisherId": exact.get("publisherId"),
        "versionCount": exact.get("versionCount"),
    }
    if exact is None:
        checks.append({
            "id": "version",
            "status": "pass",
            "detail": f"社区无同名包，本次为新建包（{args.kind}/{args.name}）；新建包受单账号 200 包配额限制",
            "fix": "",
        })
        if meta and meta.get("similar"):
            checks.append({
                "id": "naming",
                "status": "warn",
                "detail": f"存在名字相近的包：{', '.join(meta['similar'][:5])}",
                "fix": "确认不是同一实体的旧名；是则改用 --publish-name 或直接更新旧包",
            })
    else:
        remote_version = str(exact.get("version") or "").strip()
        owner = (exact.get("ownerId") or "").strip()
        if remote_version == args.version:
            checks.append({
                "id": "version",
                "status": "fail",
                "detail": f"社区已有同版本 {remote_version}（版本字节终身绑定，重发必然 409）",
                "fix": "把源文件 version 调高后重发；禁止同版本硬重试",
            })
        else:
            checks.append({
                "id": "version",
                "status": "pass",
                "detail": f"社区最新版本 {remote_version}，目标 {args.version} 未占用",
                "fix": "",
            })
        if uid and owner and owner != uid:
            checks.append({
                "id": "ownership",
                "status": "fail",
                "detail": f"同名包归属他人（owner {owner[:8]}…），直接发布会 403",
                "fix": "衍生演进：加 --publish-name <新名> 与 --derived-* 声明谱系；无谱系关系则换名",
            })
        else:
            checks.append({
                "id": "ownership",
                "status": "pass",
                "detail": "同名包归属本账号" if owner else "同名包未标注归属，按新建/更新继续",
                "fix": "",
            })

    # 4) 引用完整性（agent.skills / team.members）
    if args.kind == "agent":
        refs = fm.get("skills") or []
        known = registered_names("skill")
        label = "技能"
    elif args.kind == "team":
        refs = list(fm.get("members") or [])
        leader = str(fm.get("leader") or "").strip()
        if leader and leader not in refs:
            refs.append(leader)
        known = registered_names("agent")
        label = "成员"
    else:
        refs, known, label = [], None, ""

    if known is None and refs:
        checks.append({
            "id": "references",
            "status": "warn",
            "detail": f"无法读取本机{label}注册表，跳过引用校验",
            "fix": "确认 daemon 运行后重跑；发布回执的 warnings 会暴露缺失项",
        })
    elif refs:
        missing = [r for r in refs if r not in known]
        checks.append({
            "id": "references",
            "status": "fail" if missing else "pass",
            "detail": (f"引用的{label}本机缺失：{', '.join(missing)}" if missing
                       else f"{len(refs)} 个{label}引用均在本机注册表"),
            "fix": "先安装或创建缺失项再发布；否则社区端只能显示 slug、安装后缺件" if missing else "",
        })

    return checks, ctx


def check_phase_post(args) -> tuple[list[dict], dict]:
    checks: list[dict] = []
    ctx: dict = {}

    code, out = run_mindx(["market", "whoami", "--json"])
    payload = parse_json_stdout(out) if code == 0 else None
    uid = ((payload or {}).get("user") or {}).get("id", "").strip() if payload else ""

    exact, _ = find_package(args.kind, args.name)
    if exact is None:
        checks.append({
            "id": "exists",
            "status": "fail",
            "detail": f"社区查不到包 {args.kind}/{args.name}",
            "fix": "确认发布回执无报错；包名与发布名（--publish-name）不一致时按实际发布名查",
        })
        return checks, ctx

    ctx["remote"] = {
        "version": exact.get("version"),
        "publisherId": exact.get("publisherId"),
        "ownerId": exact.get("ownerId"),
        "nameZh": exact.get("nameZh"),
        "versionCount": exact.get("versionCount"),
    }

    remote_version = str(exact.get("version") or "").strip()
    checks.append({
        "id": "version",
        "status": "pass" if remote_version == args.version else "fail",
        "detail": f"社区最新版本 {remote_version}（目标 {args.version}）",
        "fix": "" if remote_version == args.version else "目标版本未成为最新版本：检查回执 created/sha256 与服务端版本列表",
    })

    publisher = str(exact.get("publisherId") or "").strip()
    if publisher and uid and publisher == uid:
        status, detail, fix = "pass", f"publisherId 与本机账号一致：{publisher}", ""
    elif publisher:
        status, detail, fix = "warn", f"publisherId={publisher} 与本机账号 {uid or '未知'} 不一致", \
            "包可能由其他账号打包；确认是否预期行为，禁止冒充他人身份"
    else:
        status, detail, fix = "warn", "该版本没有 publisher_id（发布者身份冻结上线前的旧版本）", \
            "发一个新版本才携带该字段；旧版本无法回填"

    checks.append({"id": "publisher", "status": status, "detail": detail, "fix": fix})

    if args.kind == "skill":
        name_zh = str(exact.get("nameZh") or "").strip()
        checks.append({
            "id": "display",
            "status": "pass" if name_zh else "warn",
            "detail": f"技能中文名：{name_zh or '缺失（社区只显示 slug）'}",
            "fix": "" if name_zh else "在 SKILL.md metadata 写 name_zh 后升版本重发",
        })
    else:
        skill_names = exact.get("skillNames") or {}
        checks.append({
            "id": "display",
            "status": "pass" if skill_names else "warn",
            "detail": f"技能中文名映射 {len(skill_names)} 条",
            "fix": "" if skill_names else "包内技能缺 name_zh，社区端只显示 slug；补 frontmatter 后升版本重发",
        })

    return checks, ctx


def main() -> int:
    ap = argparse.ArgumentParser(
        description="资源社区发布校验：发布前（登录/版本/引用）与发布后（版本/publisher/中文名）两阶段",
    )
    ap.add_argument("--phase", required=True, choices=["pre", "post"])
    ap.add_argument("--kind", required=True, choices=list(KINDS) + ["plugin"])
    ap.add_argument("--name", required=True)
    ap.add_argument("--version", required=True)
    ap.add_argument("--org", default="", help="组织 slug（仅记录，不参与本地判定）")
    args = ap.parse_args()

    if args.kind == "plugin":
        result = {
            "verdict": "fail",
            "phase": args.phase,
            "context": {"kind": "plugin", "name": args.name, "version": args.version},
            "checks": [{
                "id": "channel",
                "status": "fail",
                "detail": "插件不走 mindx market publish 通道",
                "fix": "mw plugin pack 产出 .mindpkg → 落 mindx-market/contents/plugins/<id>/ → 仓库 build-market.mjs 上架",
            }],
        }
        print(json.dumps(result, ensure_ascii=False, indent=2))
        return 1

    checks, ctx = check_phase_pre(args) if args.phase == "pre" else check_phase_post(args)
    statuses = {c["status"] for c in checks}
    verdict = "fail" if "fail" in statuses else ("warn" if "warn" in statuses else "pass")
    result = {
        "verdict": verdict,
        "phase": args.phase,
        "context": {"kind": args.kind, "name": args.name, "version": args.version, "org": args.org, **ctx},
        "checks": checks,
    }
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 1 if verdict == "fail" else 0


if __name__ == "__main__":
    sys.exit(main())
