#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""初始化部署器（**工具**，面向技能使用者在其自有机器上运行）：探测工具 → 生成计划 → 幂等部署 hooks → 校验。

位置说明：本脚本是**主动调用的工具**，位于 `scripts/`（与 `build_graph.py` 同级），
**不被** `hooks.json` 注册、不由 IDE 自动触发；`hooks/` 目录只放运行时钩子脚本。

与 `hooks/install_hooks.py` 的分工（对齐 Rules §21 自动化守卫）：
- `hooks/install_hooks.py`：只把 `hooks.json` 的占位符替换成真实路径（单一职责）。
- 本脚本：**编排**——探测使用者机器上装了/在用哪些工具 → 决定各工具 hooks 配置落点
  → 幂等部署 → 校验回读。

数据源：平台定义（探测路径 / 配置落点 / skill 加载形态）**全部**来自 `scripts/platforms.json`
——单一事实源，本脚本不内联任何平台数据；新增或修正平台只改该 JSON，不改本脚本。
文件缺失或内容非法时**拒绝启动**（退出码 2），不静默降级为空表。

探测维度（**工具级**，五路证据任一命中即视为"在用"）：
1. `exe`：常见安装位置的可执行文件（%VAR%/~/绝对路径候选，跨平台）
2. `cmd`：PATH 中的 CLI 命令（`shutil.which`）
3. `data`：应用数据目录（工具"用过"的强证据）
4. `user_cfg`：**用户级**配置落点（如 `~/.codebuddy/settings.json`）
5. `proj_cfg`：**项目级**配置落点（如 `<项目根>/.codebuddy/settings.json`）

部署作用域（`--scope`）：
- `project`（默认）：只写**当前项目**的配置（`<项目根>/<tool>/...`），影响范围最小。
- `user`：只写**用户级**配置（`~/...`）——⚠️ 影响该用户**所有项目**。
- `both`：两处都写。

安全边界（对使用者机器）：
- **默认 dry-run**：不带 `--apply` 绝不写任何文件。
- 每个目标写前 `.bak`；合并**只追加不删除**；同名冲突默认跳过（`--allow-overwrite` 才替换）。
- 单目标写入失败**不影响其他目标**（逐目标隔离，失败记入报告）。
- 目标配置与技能模板同文件 → 拒绝写入；技能安装目录作为**项目根**时拒绝写项目级配置。
- 所有路径经 `%VAR%` / `~` 展开，**不写死任何具体机器的绝对路径**。

用法：
  python scripts/init_deploy.py                       # 探测 + 出计划（dry-run）
  python scripts/init_deploy.py --apply               # 按探测结果部署（默认 project 作用域）
  python scripts/init_deploy.py --scope both --apply  # 用户级 + 项目级都写（⚠️ 全局影响）
  python scripts/init_deploy.py --tools codebuddy,trae --apply
  python scripts/init_deploy.py --json
退出码：0 = 成功（含 dry-run）；2 = 前置检查失败（模板缺失 / 脚本缺失 / 无 Python）；3 = 部分或全部目标写入失败。
"""
import argparse
import json
import os
import re
import shutil
import sys

# 脚本位于 <技能根>/scripts/ → 技能根取其父目录；若被放到别处则退化为脚本所在目录
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
SKILL_ROOT = (os.path.dirname(SCRIPT_DIR)
              if os.path.basename(SCRIPT_DIR).lower() == "scripts" else SCRIPT_DIR)
DEFAULT_HOOKS_DIR = os.path.join(SKILL_ROOT, "hooks")
# 配置源与消费者同目录：模板与脚本同放 scripts/
DEFAULT_TEMPLATE = os.path.join(SCRIPT_DIR, "hooks.json")
# 平台声明（单一事实源）：探测路径 / 配置落点 / skill 加载形态
DEFAULT_PLATFORMS = os.path.join(SCRIPT_DIR, "platforms.json")

# 生成 hooks.installed.json 时追加到 _comment 末尾：保留模板原文（含各平台适配说明），
# 仅补一句自动生成提示，避免注释与"占位符已替换为真实路径"的事实冲突
GENERATED_COMMENT_SUFFIX = (" ｜ 【自动生成】占位符已替换为真实机器路径，勿手改；"
                            "换机 / 换项目 / 技能升级后重跑部署器覆盖本文件即可。")

# ---------------------------------------------------------------- 平台声明（外部数据源）
# 定义见 scripts/platforms.json 的 _comment；该文件缺失或非法即 fail-closed 终止，不降级为空表
TOOL_PROFILES = []

PLACEHOLDER_PY = "{{PYTHON_BIN}}"
PLACEHOLDER_DIR = "{{HOOKS_DIR}}"
PY_NAME_RE = re.compile(r"([\w\-]+\.py)")
ROOT_MARKERS = (".codebuddy", ".claude", ".trae", ".cursor", ".windsurf", ".ai-memory", ".git")
COPY_REL = os.path.join(".codebuddy", "hooks")
MAX_DETAIL = 20


def _fix_stdout_encoding():
    """Windows 控制台默认 GBK，输出中文/符号会抛 UnicodeEncodeError（stdout 与 stderr 同理）。"""
    try:
        import io
        for name in ("stdout", "stderr"):
            stream = getattr(sys, name, None)
            if stream is None or not hasattr(stream, "buffer"):
                continue
            enc = (stream.encoding or "").lower().replace("-", "")
            if enc != "utf8":
                setattr(sys, name, io.TextIOWrapper(stream.buffer, encoding="utf-8", errors="replace"))
    except Exception:
        pass


# ---------------------------------------------------------------- 路径与探测
def expand_path(p, root=""):
    """展开 `~` 与 `%VAR%`（Windows）/ `$VAR`（Unix）；相对路径挂到 root 下。"""
    if not p:
        return ""
    if p.startswith("~"):
        p = os.path.expanduser(p)
    p = os.path.expandvars(p)
    if not os.path.isabs(p) and root:
        p = os.path.join(root, p)
    return os.path.normpath(p)


def _climb(start, markers):
    d = os.path.abspath(start)
    while True:
        for mk in markers:
            if os.path.exists(os.path.join(d, mk)):
                return d
        nd = os.path.dirname(d)
        if nd == d:
            return None
        d = nd


def detect_python(cli_bin):
    for cand in (cli_bin, os.environ.get("PYTHON_BIN", "")):
        if cand and os.path.isfile(cand):
            return os.path.abspath(cand)
    if sys.executable and os.path.isfile(sys.executable):
        return sys.executable
    for name in ("python3", "python"):
        p = shutil.which(name)
        if p:
            return p
    return ""


def detect_root(cli_root):
    if cli_root:
        return os.path.abspath(cli_root)
    env = os.environ.get("PROJECT_ROOT", "")
    if env and os.path.isdir(env):
        return os.path.abspath(env)
    return _climb(os.getcwd(), ROOT_MARKERS) or _climb(SCRIPT_DIR, ROOT_MARKERS)


def detect_hooks_dir(cli_dir):
    """hooks 脚本目录：`--hooks-dir` → `<技能根>/hooks` → 脚本所在目录（兜底）。"""
    if cli_dir:
        return os.path.abspath(cli_dir)
    if os.path.isdir(DEFAULT_HOOKS_DIR):
        return DEFAULT_HOOKS_DIR
    return SCRIPT_DIR


def _fail(msg):
    """前置检查失败：写 stderr 并以退出码 2 终止（2 = 前置检查失败，见模块 docstring）。"""
    print("[FAIL] " + msg, file=sys.stderr)
    raise SystemExit(2)


def load_platforms(path=""):
    """加载平台声明文件（单一事实源 `scripts/platforms.json`）。

    失败即终止（fail-closed）——**不**静默回退空表：空表会被下游读成"本机没装任何工具"，
    把配置错误伪装成环境缺失。返回平台条目列表，字段语义见该文件 `_comment`。
    """
    p = os.path.abspath(path) if path else DEFAULT_PLATFORMS
    if not os.path.isfile(p):
        _fail("平台声明文件缺失: %s\n"
              "       该文件是平台落点的单一事实源，随技能包分发（scripts/platforms.json）。\n"
              "       请确认技能包完整；或用 --platforms <路径> 显式指定。" % p)
    try:
        # utf-8-sig：Windows 记事本 / PowerShell 存盘常带 BOM，兼容有无 BOM 两种写法
        with open(p, encoding="utf-8-sig") as f:
            data = json.load(f)
    except (ValueError, OSError) as e:
        _fail("平台声明文件无法解析: %s\n       %s" % (p, e))
    profs = data.get("platforms") if isinstance(data, dict) else None
    if not isinstance(profs, list) or not profs:
        _fail("平台声明文件缺少非空的 platforms 数组: %s" % p)
    ids = []
    for i, prof in enumerate(profs):
        if not isinstance(prof, dict) or not prof.get("id") or not prof.get("name"):
            _fail("platforms[%d] 缺少 id 或 name: %s" % (i, p))
        ids.append(prof["id"])
    dup = sorted({x for x in ids if ids.count(x) > 1})
    if dup:
        _fail("platforms 中 id 重复: %s (%s)" % (", ".join(dup), p))
    return profs


def detect_tools(root):
    """五路证据并集探测工具。返回 [tool_dict...]，每项含 installed / evidence / 落点。"""
    out = []
    for prof in TOOL_PROFILES:
        ev = []
        for raw in prof.get("cmd", []):
            p = shutil.which(raw)
            if p:
                ev.append({"kind": "cmd", "ref": raw, "path": p})
        for kind in ("exe", "data", "user_cfg"):
            for raw in prof.get(kind, []):
                p = expand_path(raw)
                if p and os.path.exists(p):
                    ev.append({"kind": kind, "ref": raw, "path": p})
        for rel in prof.get("proj_cfg", []):
            if root:
                p = os.path.normpath(os.path.join(root, rel))
                if os.path.isfile(p):
                    ev.append({"kind": "proj_cfg", "ref": rel, "path": p})
        item = {
            "id": prof["id"],
            "name": prof["name"],
            "installed": bool(ev),
            "evidence": ev,
            "user_cfg": [expand_path(x) for x in prof.get("user_cfg", [])],
            "proj_cfg": [os.path.normpath(os.path.join(root, x)) if root else ""
                         for x in prof.get("proj_cfg", [])],
        }
        out.append(item)
    return out


def resolve_targets(tool, scope, root):
    """按作用域给出该工具的配置落点列表（去重、保序）。"""
    out = []
    if scope in ("user", "both"):
        for p in tool["user_cfg"]:
            if p and p not in out:
                out.append(("user", p))
    if scope in ("project", "both") and root:
        for p in tool["proj_cfg"]:
            if p and p not in out:
                out.append(("project", p))
    return out


# ---------------------------------------------------------------- 模板渲染
def load_template(template_path):
    with open(template_path, encoding="utf-8") as f:
        raw = f.read()
    data = json.loads(raw)
    if not isinstance(data, dict) or not isinstance(data.get("hooks"), dict):
        raise ValueError("模板缺少 hooks 段: %s" % template_path)
    return raw, data


def template_scripts(template):
    names = set()
    for _event, items in (template.get("hooks") or {}).items():
        for it in items or []:
            for h in (it or {}).get("hooks", []) or []:
                for m in PY_NAME_RE.finditer(h.get("command", "")):
                    names.add(m.group(1))
    return sorted(names)


def check_scripts(hooks_dir, names):
    ok, missing = [], []
    for n in names:
        (ok if os.path.isfile(os.path.join(hooks_dir, n)) else missing).append(n)
    return ok, missing


def render_command(template_cmd, python_bin, hooks_dir):
    """把模板 command 的占位符换成真实路径（**原始**路径，不在此转义）。

    本函数产物会经 json.dump 落盘，json.dump 自身会转义反斜杠；若在此预转义会被二次
    转义成 `\\\\`。转义只保留在「对模板文本做字符串级替换」的通道（写 hooks.installed.json）。
    """
    return template_cmd.replace(PLACEHOLDER_PY, python_bin).replace(PLACEHOLDER_DIR, hooks_dir)


def _norm_cmd(cmd):
    """归一化 command 供幂等比较：连续反斜杠折叠为单个。

    历史版本把路径写成双反斜杠；折叠后与本次生成串判等，旧部署重跑 --apply 时
    不会被当成「不同路径」而重复注册同名脚本。
    """
    while "\\\\" in cmd:
        cmd = cmd.replace("\\\\", "\\")
    return cmd


def count_entries(template):
    n = 0
    for _event, items in (template.get("hooks") or {}).items():
        for it in items or []:
            n += len((it or {}).get("hooks", []) or [])
    return n


# ---------------------------------------------------------------- 配置合并与校验
def load_config(cfg_path):
    if not os.path.isfile(cfg_path):
        return {}
    with open(cfg_path, encoding="utf-8") as f:
        raw = f.read().lstrip("\ufeff")
    if not raw.strip():
        return {}
    data = json.loads(raw)
    if not isinstance(data, dict):
        raise ValueError("配置根不是对象: %s" % cfg_path)
    return data


def _existing_keys(cfg, event):
    keys = set()
    for it in ((cfg.get("hooks") or {}).get(event) or []):
        matcher = (it or {}).get("matcher", "")
        for h in (it or {}).get("hooks", []) or []:
            keys.add((matcher, _norm_cmd(h.get("command", ""))))
    return keys


def merge_config(cfg, template, python_bin, hooks_dir, allow_overwrite):
    """幂等合并：只追加缺失项，既有其他条目原样保留。返回 (new_cfg, added, skipped)。"""
    new_cfg = json.loads(json.dumps(cfg))
    hooks = new_cfg.setdefault("hooks", {})
    added, skipped = [], []
    for event, items in (template.get("hooks") or {}).items():
        bucket = hooks.setdefault(event, [])
        have = _existing_keys(new_cfg, event)
        for it in items or []:
            matcher = (it or {}).get("matcher", "")
            for h in (it or {}).get("hooks", []) or []:
                cmd = render_command(h.get("command", ""), python_bin, hooks_dir)
                cmd_key = _norm_cmd(cmd)
                if (matcher, cmd_key) in have:
                    skipped.append((event, matcher, "已存在"))
                    continue
                m = PY_NAME_RE.search(cmd)
                base = m.group(1) if m else ""
                conflict = False
                for it2 in bucket:
                    if (it2 or {}).get("matcher", "") != matcher:
                        continue
                    for h2 in (it2 or {}).get("hooks", []) or []:
                        m2 = PY_NAME_RE.search(h2.get("command", ""))
                        if m2 and base and m2.group(1) == base and h2.get("command", "") != cmd:
                            conflict = True
                            break
                    if conflict:
                        break
                if conflict and not allow_overwrite:
                    skipped.append((event, matcher, "同名脚本已由其他路径注册，跳过（--allow-overwrite 可替换）"))
                    continue
                if conflict and allow_overwrite:
                    for it2 in bucket:
                        if (it2 or {}).get("matcher", "") != matcher:
                            continue
                        hs = (it2 or {}).get("hooks", []) or []
                        for h2 in list(hs):
                            m2 = PY_NAME_RE.search(h2.get("command", ""))
                            if m2 and base and m2.group(1) == base:
                                hs.remove(h2)
                entry = dict(h)
                entry["command"] = cmd
                bucket.append({"matcher": matcher, "hooks": [entry]})
                have.add((matcher, cmd_key))
                added.append((event, matcher, base))
    return new_cfg, added, skipped


def verify_config(cfg, hooks_dir):
    ok, problems = 0, []
    for event, items in (cfg.get("hooks") or {}).items():
        for it in items or []:
            for h in (it or {}).get("hooks", []) or []:
                m = PY_NAME_RE.search(h.get("command", ""))
                if not m:
                    continue
                script = m.group(1)
                if os.path.isfile(os.path.join(hooks_dir, script)):
                    ok += 1
                else:
                    problems.append("%s / %s：脚本不存在 %s" % (event, script, os.path.join(hooks_dir, script)))
    return ok, problems


# ---------------------------------------------------------------- 编排
def _write_config(cfg_path, new_cfg, report):
    """写单个目标配置（已存在则先 .bak）。成功返回 (True, bak_path 或 "")；失败返回 (False, "")。

    注意：**新建文件天然没有 .bak**，不可用「是否生成备份」反推成功与否（否则新建会被误判失败）。
    """
    bak = ""
    try:
        if os.path.isfile(cfg_path):
            shutil.copy2(cfg_path, cfg_path + ".bak")
            bak = cfg_path + ".bak"
        os.makedirs(os.path.dirname(cfg_path), exist_ok=True)
        with open(cfg_path, "w", encoding="utf-8", newline="\n") as f:
            json.dump(new_cfg, f, ensure_ascii=False, indent=2)
            f.write("\n")
        return True, bak
    except Exception as e:
        report["errors"].append("写入失败 %s：%s" % (cfg_path, e))
        return False, ""


def build_report(args):
    hooks_dir = detect_hooks_dir(args.hooks_dir)
    template_path = args.template or DEFAULT_TEMPLATE
    root = detect_root(args.root)
    python_bin = detect_python(args.python_bin)

    rp = {
        "mode": "apply" if args.apply else "dry-run",
        "scope": args.scope,
        "project_root": root or "",
        "python_bin": python_bin,
        "hooks_dir": hooks_dir,
        "template": template_path,
        "skill_root": SKILL_ROOT,
        "errors": [],
        "warnings": [],
        "tools": [],
        "targets": [],
    }

    if not os.path.isfile(template_path):
        rp["errors"].append("模板不存在：%s（用 --template 指定）" % template_path)
        return rp, 2
    if not python_bin:
        rp["errors"].append("未探测到 Python 解释器（用 --python-bin 或设置 PYTHON_BIN）")
        return rp, 2
    try:
        _raw, template = load_template(template_path)
    except (OSError, ValueError) as e:
        rp["errors"].append("模板读取/解析失败：%s" % e)
        return rp, 2

    names = template_scripts(template)
    ok, missing = check_scripts(hooks_dir, names)
    rp["scripts_total"], rp["scripts_ok"], rp["scripts_missing"] = len(names), len(ok), missing
    rp["hooks_entries"] = count_entries(template)
    if missing:
        rp["errors"].append("hooks 目录缺少脚本：%s（补齐后重跑，本次不写任何文件）" % "、".join(missing))
        return rp, 2

    # --- 工具探测 ---
    tools = detect_tools(root)
    if args.tools:
        allow = {t.strip().lower() for t in args.tools.split(",") if t.strip()}
        tools = [t for t in tools if t["id"] in allow]
    rp["tools"] = tools
    installed = [t for t in tools if t["installed"]]
    if args.scope in ("user", "both"):
        rp["warnings"].append("作用域含 user：将写入用户级配置（%s/…），影响该用户**所有项目**" % os.path.expanduser("~"))
    if not installed:
        rp["warnings"].append("未探测到已安装/在用的工具：仅生成 hooks.installed.json；接入方式见 README「方式 B」")
    root_is_skill = bool(root) and os.path.abspath(root) == SKILL_ROOT
    if root_is_skill:
        rp["warnings"].append("项目根疑似技能安装目录（%s）：项目级配置不写入，请用 --root 指定真实项目根" % SKILL_ROOT)

    # --- 计划 ---
    plan = []
    for tool in installed:
        targets = resolve_targets(tool, args.scope, root)
        if root_is_skill:
            targets = [t for t in targets if t[0] != "project"]
        for kind, cfg in targets:
            if os.path.abspath(cfg) == os.path.abspath(template_path):
                rp["warnings"].append("目标与技能模板同文件，拒绝写入：%s" % cfg)
                continue
            plan.append({"tool": tool["id"], "tool_name": tool["name"], "kind": kind,
                         "cfg": cfg, "exists": os.path.isfile(cfg)})
    if args.first_only and plan:
        plan = plan[:1]
    rp["plan"] = plan

    installed_path = os.path.join(os.path.dirname(template_path), "hooks.installed.json")
    rp["installed_json"] = installed_path

    if not args.apply:
        # dry-run：只统计，不写盘
        for item in plan:
            try:
                cfg = load_config(item["cfg"])
                _n, added, skipped = merge_config(cfg, template, python_bin, hooks_dir, args.allow_overwrite)
                item["add_entries"], item["skip_entries"] = len(added), len(skipped)
            except ValueError as e:
                item["error"] = "配置解析失败（apply 时将拒绝写入）：%s" % e
        return rp, 0

    # --- apply ---
    failed = 0
    try:
        raw, _tpl = load_template(template_path)
        rendered = raw.replace(PLACEHOLDER_PY, python_bin.replace("\\", "\\\\")) \
                      .replace(PLACEHOLDER_DIR, hooks_dir.replace("\\", "\\\\"))
        json.loads(rendered)
        m = re.search(r'"_comment"\s*:\s*"((?:[^"\\]|\\.)*)"', rendered)
        if m:
            tail = json.loads('"%s"' % m.group(1)) + GENERATED_COMMENT_SUFFIX
            rendered = (rendered[:m.start()] + '"_comment": ' + json.dumps(tail, ensure_ascii=False)
                        + rendered[m.end():])
        json.loads(rendered)
        if args.events:
            keep = {e.strip() for e in args.events.split(",") if e.strip()}
            cfg = json.loads(rendered)
            hk = cfg.get("hooks")
            if isinstance(hk, dict):
                cfg["hooks"] = {k: v for k, v in hk.items() if k in keep}
                rendered = json.dumps(cfg, ensure_ascii=False, indent=2) + "\n"
                json.loads(rendered)
                rp["events_kept"] = sorted(cfg["hooks"].keys())
        if os.path.isfile(installed_path):
            shutil.copy2(installed_path, installed_path + ".bak")
        with open(installed_path, "w", encoding="utf-8", newline="\n") as f:
            f.write(rendered)
        rp["installed_json_written"] = installed_path
    except Exception as e:
        rp["errors"].append("生成 hooks.installed.json 失败：%s" % e)
        return rp, 3

    eff_hooks_dir = hooks_dir
    if args.copy_to_project and root and not root_is_skill:
        dest_dir = os.path.join(root, COPY_REL)
        copied, skip_files = [], []
        try:
            os.makedirs(dest_dir, exist_ok=True)
            for n in names:
                dst = os.path.join(dest_dir, n)
                if os.path.exists(dst):
                    skip_files.append(n)
                    continue
                shutil.copy2(os.path.join(hooks_dir, n), dst)
                copied.append(n)
            eff_hooks_dir = dest_dir
            rp["copied"], rp["copy_skipped"] = copied, skip_files
        except Exception as e:
            rp["errors"].append("复制脚本失败：%s" % e)
            failed += 1

    for item in plan:
        try:
            cfg = load_config(item["cfg"])
        except ValueError as e:
            item["error"] = "配置解析失败，拒绝写入以免破坏：%s" % e
            rp["errors"].append(item["error"])
            failed += 1
            continue
        new_cfg, added, skipped = merge_config(cfg, template, python_bin, eff_hooks_dir, args.allow_overwrite)
        ok, bak = _write_config(item["cfg"], new_cfg, rp)
        if ok:
            item["add_entries"], item["skip_entries"] = len(added), len(skipped)
            if bak:
                item["backup"] = bak
            try:
                chk = load_config(item["cfg"])
                okc, problems = verify_config(chk, eff_hooks_dir)
                item["verify_ok"], item["verify_problems"] = okc, problems
            except Exception as e:
                item["verify_problems"] = ["校验回读失败：%s" % e]
                failed += 1
        else:
            failed += 1
        rp["targets"].append(item)

    return rp, (3 if failed else 0)


def print_report(rp):
    L = []
    L.append("[INIT-DEPLOY] 模式：%s ｜ 作用域：%s" % (
        "apply（已落地）" if rp["mode"] == "apply" else "dry-run（未写入）", rp.get("scope", "project")))
    L.append("  技能根    : %s" % rp.get("skill_root", ""))
    L.append("  项目根    : %s" % (rp["project_root"] or "（未探测到）"))
    L.append("  Python    : %s" % (rp["python_bin"] or "（未探测到）"))
    L.append("  hooks 目录: %s" % rp["hooks_dir"])
    L.append("  模板      : %s（%s 个钩子条目）" % (rp["template"], rp.get("hooks_entries", "?")))
    if "scripts_total" in rp:
        L.append("  脚本完整性: %d/%d%s" % (
            rp.get("scripts_ok", 0), rp.get("scripts_total", 0),
            "（缺：%s）" % "、".join(rp["scripts_missing"]) if rp.get("scripts_missing") else " OK"))
    L.append("  工具探测  :")
    for t in rp.get("tools", []):
        mark = "✓ 在用" if t["installed"] else "· 未探测到"
        ev = "；".join("%s=%s" % (e["kind"], e["path"]) for e in t["evidence"][:3]) or "-"
        L.append("    %s %s —— %s" % (mark, t["name"], ev))
    plan = rp.get("plan") or []
    if plan:
        L.append("  部署计划  :")
        for it in plan:
            extra = ""
            if "add_entries" in it:
                extra = "（新增 %s 条 / 跳过 %s 条）" % (it.get("add_entries"), it.get("skip_entries"))
            if it.get("error"):
                extra = "（%s）" % it["error"]
            L.append("    [%s] %s → %s%s" % (
                "用户级" if it["kind"] == "user" else "项目级", it["tool_name"], it["cfg"], extra))
    elif rp.get("tools"):
        if any(t.get("installed") for t in rp["tools"]):
            L.append("  部署计划  : 无（已探测到在用工具，但当前作用域 / 项目根下无可用落点，见下方提示）")
        else:
            L.append("  部署计划  : 无（未探测到在用工具）")
    if rp.get("installed_json_written"):
        L.append("  hooks.installed.json → %s" % rp["installed_json_written"])
    if rp.get("copied") is not None:
        L.append("  复制脚本  : %d 个（跳过 %d 个已存在）" % (len(rp.get("copied") or []), len(rp.get("copy_skipped") or [])))
    for it in rp.get("targets", []):
        if it.get("add_entries") is not None:
            L.append("  已写入    : %s → 新增 %s 条 / 跳过 %s 条%s%s" % (
                it["cfg"], it.get("add_entries"), it.get("skip_entries"),
                "｜备份 %s" % it["backup"] if it.get("backup") else "",
                "｜校验 ✅ %d 条" % it["verify_ok"] if it.get("verify_ok") else ""))
    if rp.get("warnings"):
        L.append("  提示      :")
        for w in rp["warnings"]:
            L.append("    - %s" % w)
    if rp.get("errors"):
        L.append("  错误      :")
        for e in rp["errors"]:
            L.append("    - %s" % e)
    if rp["mode"] == "dry-run" and not rp.get("errors"):
        L.append("  执行      : 加 --apply 落地（写前 .bak；只追加不删除；同名冲突默认跳过）")
    print("\n".join(L))


def main():
    _fix_stdout_encoding()
    parser = argparse.ArgumentParser(
        description="dev-expert 初始化部署器（工具）：探测工具 → 出计划 → 幂等部署 hooks → 校验",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument("--root", default="", help="项目根（默认：PROJECT_ROOT 或上溯标记目录）")
    parser.add_argument("--hooks-dir", default="", help="hook 脚本目录（默认：<技能根>/hooks）")
    parser.add_argument("--python-bin", default="", help="Python 解释器路径（默认：PYTHON_BIN 或当前解释器）")
    parser.add_argument("--template", default="", help="hooks.json 模板路径（默认：<技能根>/scripts/hooks.json）")
    parser.add_argument("--platforms", default="",
                        help="平台声明文件（默认：<技能根>/scripts/platforms.json）")
    parser.add_argument("--scope", choices=["project", "user", "both"], default="project",
                        help="部署作用域：project=仅当前项目（默认）；user=仅用户级（⚠️ 影响所有项目）；both=两处")
    parser.add_argument("--tools", default="", help="限定工具 id（逗号分隔，如 codebuddy,trae；默认全部探测到的）")
    parser.add_argument("--first-only", action="store_true", help="只处理探测到的第一个工具的首个落点")
    parser.add_argument("--copy-to-project", action="store_true",
                        help="把脚本复制到 <项目根>/.codebuddy/hooks/（已存在同名则跳过）")
    parser.add_argument("--apply", action="store_true", help="实际落地（默认 dry-run，不写任何文件）")
    parser.add_argument("--allow-overwrite", action="store_true",
                        help="同名脚本已由其他路径注册时替换（默认跳过）")
    parser.add_argument("--json", action="store_true", help="输出机器可读 JSON")
    parser.add_argument("--events", default="",
                        help="顶层事件键白名单（逗号分隔，如 PreToolUse,PostToolUse）；默认全量不裁剪。"
                             "平台不认某事件键时可裁剪，避免整份配置加载失败")
    args = parser.parse_args()

    global TOOL_PROFILES
    TOOL_PROFILES = load_platforms(args.platforms)

    report, code = build_report(args)
    if args.json:
        print(json.dumps(report, ensure_ascii=False, indent=2))
    else:
        print_report(report)
    return code


if __name__ == "__main__":
    sys.exit(main())
