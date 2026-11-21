#!/usr/bin/env python3
"""run-mcp 技能的唯一入口：把「解析服务 / 解析工具 / 启用 / 调用 / 截断」压进一次脚本执行。

设计目标是让 LLM 一次工具调用就能跑完：
- 服务名精确（slug）→ 直接 call，不列清单；
- 服务名给了中文标题或关键词 → 首次 call 报「不在 mcp.json」，脚本自动用 list 做模糊匹配后重试；
- 条目 disabled → 直接报错退出，绝不替用户启用；用户明确要求后由 admin enable 手动处理；
- 工具名模糊/不存在 → 自动用 tools 匹配后重试；
- 结果过长 → 只回前 N 字符，全文落盘给路径。

LLM 只在 exit 2/5/6 时才需要看候选再补一次调用，其余情况一次到底。
"""
import argparse
import json
import os
import shutil
import subprocess
import sys
import time
from pathlib import Path

CACHE_DIR = Path(os.environ.get("XDG_CACHE_HOME", Path.home() / ".cache")) / "mindx-run-mcp"
CATALOG_TTL = 300
MAX_RESULT_CHARS = 3000

EXIT_OK = 0
EXIT_AMBIGUOUS = 2
EXIT_NO_CLI = 3
EXIT_NO_DAEMON = 4
EXIT_NO_SERVER = 5
EXIT_NO_TOOL = 6
EXIT_CALL_FAILED = 7
EXIT_DISABLED_REFUSED = 8


def resolve_cli() -> str:
    env = os.environ.get("MINDX_CLI")
    if env and Path(env).is_file() and os.access(env, os.X_OK):
        return env
    found = shutil.which("mindx")
    if found:
        return found
    for candidate in (Path.home() / ".mindx/bin/mindx", Path("/usr/local/bin/mindx")):
        if candidate.is_file() and os.access(candidate, os.X_OK):
            return str(candidate)
    fail(EXIT_NO_CLI, "NO_CLI", "找不到 mindx 命令：PATH、~/.mindx/bin、/usr/local/bin 都没有。设置 MINDX_CLI 指向可执行文件。")


def fail(code: int, kind: str, message: str, **extra) -> None:
    payload = {"error": message, "code": kind, **extra}
    print(json.dumps(payload, ensure_ascii=False, indent=1))
    sys.exit(code)


def run_cli(cli: str, args: list[str], timeout: int = 120) -> tuple[int, str, str]:
    try:
        proc = subprocess.run([cli, "mcp", *args], capture_output=True, text=True, timeout=timeout)
    except subprocess.TimeoutExpired:
        return 1, "", f"timeout after {timeout}s"
    return proc.returncode, proc.stdout, proc.stderr


def error_line(stderr: str) -> str:
    lines = [ln.strip() for ln in stderr.splitlines() if ln.strip()]
    for ln in reversed(lines):
        if "error" in ln.lower() or "fail" in ln.lower() or "未启用" in ln or "mcp.json" in ln:
            return ln
    return lines[-1] if lines else ""


def load_catalog(cli: str, refresh: bool = False) -> list[dict]:
    CACHE_DIR.mkdir(parents=True, exist_ok=True)
    cache = CACHE_DIR / "catalog.json"
    if not refresh and cache.is_file() and time.time() - cache.stat().st_mtime < CATALOG_TTL:
        try:
            return json.loads(cache.read_text())
        except json.JSONDecodeError:
            pass
    code, out, err = run_cli(cli, ["list", "--json"])
    if code != 0:
        fail(EXIT_NO_DAEMON, "NO_DAEMON", error_line(err) or "无法读取 MCP 清单，确认 MindX 正在运行。")
    try:
        data = json.loads(out)
    except json.JSONDecodeError:
        fail(EXIT_NO_DAEMON, "NO_DAEMON", "list --json 返回内容无法解析。")
    cache.write_text(json.dumps(data, ensure_ascii=False))
    return data


def match_servers(catalog: list[dict], token: str) -> list[dict]:
    q = token.strip().lower()
    scored = []
    for item in catalog:
        name = str(item.get("name", "")).lower()
        title = str(item.get("title", "")).lower()
        desc = str(item.get("description", "")).lower()
        score = 0
        if name == q:
            score = 100
        elif q in name:
            score = 80
        elif q in title:
            score = 70
        elif q in desc:
            score = 40
        if score:
            scored.append((score, item))
    scored.sort(key=lambda pair: (-pair[0], pair[1].get("name", "")))
    return [item for _, item in scored]


def slim(servers: list[dict]) -> list[dict]:
    return [
        {"name": s.get("name"), "title": s.get("title") or "", "enabled": bool(s.get("enabled"))}
        for s in servers[:5]
    ]


def resolve_server(cli: str, token: str) -> dict:
    catalog = load_catalog(cli, refresh=True)
    matches = match_servers(catalog, token)
    exact = [m for m in matches if str(m.get("name", "")).lower() == token.strip().lower()]
    if exact:
        return exact[0]
    if len(matches) == 1:
        return matches[0]
    if not matches:
        fail(EXIT_NO_SERVER, "SERVER_NOT_FOUND", f"没有匹配「{token}」的 MCP 服务。")
    fail(EXIT_AMBIGUOUS, "AMBIGUOUS_SERVER", f"「{token}」匹配到多个服务，选一个准确的 name 重跑。", candidates=slim(matches))


def call_tool(cli: str, server: str, tool: str, args: dict, timeout: int) -> tuple[dict | None, str]:
    cmd = ["call", server, tool, "--json", "--timeout", str(timeout)]
    if args:
        cmd += ["--args-json", json.dumps(args, ensure_ascii=False)]
    code, out, err = run_cli(cli, cmd, timeout=timeout + 15)
    if code == 0:
        try:
            return json.loads(out), ""
        except json.JSONDecodeError:
            return {"result": out}, ""
    return None, error_line(err) or f"call 退出码 {code}"


def format_result(raw: dict, server: str, tool: str) -> dict:
    result = raw.get("result", "")
    if isinstance(result, str):
        try:
            parsed = json.loads(result)
            result = parsed
        except json.JSONDecodeError:
            pass
    text = result if isinstance(result, str) else json.dumps(result, ensure_ascii=False)
    payload = {"server": server, "tool": tool, "result": result}
    if len(text) > MAX_RESULT_CHARS:
        CACHE_DIR.mkdir(parents=True, exist_ok=True)
        path = CACHE_DIR / f"result-{int(time.time())}.json"
        path.write_text(text if isinstance(result, str) else json.dumps(result, ensure_ascii=False, indent=2))
        payload["result"] = text[:MAX_RESULT_CHARS] + "…[截断]"
        payload["truncated"] = True
        payload["full_result_path"] = str(path)
    return payload


def cmd_run(cli: str, server_token: str, tool: str | None, args: dict, timeout: int, refresh: bool) -> None:
    server = server_token

    for attempt in range(3):
        if tool is None:
            resolved = resolve_server(cli, server)
            server = resolved.get("name")
            code, out, err = run_cli(cli, ["tools", server, "--json"], timeout=timeout + 15)
            if code != 0:
                fail(EXIT_CALL_FAILED, "TOOLS_FAILED", error_line(err) or "列工具失败。")
            tools = json.loads(out)
            print(json.dumps({"server": server, "tools": [{k: t.get(k) for k in ("name", "description")} for t in tools[:15]]}, ensure_ascii=False, indent=1))
            return

        raw, message = call_tool(cli, server, tool, args, timeout)

        if raw is not None:
            print(json.dumps(format_result(raw, server, tool), ensure_ascii=False, indent=1))
            return

        lowered = message.lower()
        if "不在 mcp.json" in message or "mcp.json" in message:
            resolved = resolve_server(cli, server)
            server = resolved.get("name")
            continue

        if "未启用" in message or "disabled" in lowered:
            fail(EXIT_DISABLED_REFUSED, "DISABLED", f"{server} 未启用；本技能不替用户启用。用户确认后执行: python3 scripts/mcp.py admin enable {server}")

        tool_missing = (
            "unknown tool" in lowered
            or ("not found" in lowered and "tool" in lowered)
            or "未知工具" in message
            or "工具不存在" in message
        )
        if raw is None and tool_missing:
            code, out, err = run_cli(cli, ["tools", server, "--json"], timeout=timeout + 15)
            if code != 0:
                break
            tools = [t.get("name", "") for t in json.loads(out)]
            matches = [n for n in tools if n.lower() == tool.lower()] or [n for n in tools if tool.lower() in n.lower()]
            if len(matches) == 1:
                tool = matches[0]
                continue
            if matches:
                fail(EXIT_AMBIGUOUS, "AMBIGUOUS_TOOL", f"「{tool}」匹配到多个工具。", candidates=matches[:8])
            fail(EXIT_NO_TOOL, "TOOL_NOT_FOUND", f"{server} 没有名为「{tool}」的工具。", candidates=tools[:15])

        fail(EXIT_CALL_FAILED, "CALL_FAILED", message or "调用失败。")

    fail(EXIT_CALL_FAILED, "CALL_FAILED", message or "调用失败。")


def passthrough(cli: str, argv: list[str]) -> None:
    proc = subprocess.run([cli, "mcp", *argv])
    sys.exit(proc.returncode)


def main() -> None:
    parser = argparse.ArgumentParser(
        prog="mcp.py",
        description="调用 MCP 服务：自动解析服务名/工具名、不替用户启用、截断长结果。常规场景一次调用完成。",
        epilog="退出码: 0 成功 / 2 候选不唯一 / 3 找不到 CLI / 4 主机不可达 / 5 服务不存在 / 6 工具不存在 / 7 调用失败 / 8 条目未启用（需用户确认后执行 admin enable）",
    )
    parser.add_argument("--refresh", action="store_true", help="忽略服务清单缓存，强制重读")
    sub = parser.add_subparsers(dest="command", required=True)

    run = sub.add_parser("run", help="解析并调用：run <服务> [工具] [--args-json ...]")
    run.add_argument("server", help="服务 slug 或中文标题/关键词")
    run.add_argument("tool", nargs="?", help="工具名；省略则列出该服务的工具")
    run.add_argument("--args-json", default="{}", help="工具参数 JSON 对象")
    run.add_argument("--arg", action="append", default=[], metavar="K=V", help="字符串参数，可重复")
    run.add_argument("--timeout", type=int, default=60, help="调用超时秒数，默认 60")


    find = sub.add_parser("find", help="按关键词找服务，只输出 name/title/enabled")
    find.add_argument("keyword")

    tools = sub.add_parser("tools", help="列出服务的工具")
    tools.add_argument("server")
    tools.add_argument("--timeout", type=int, default=60)

    call = sub.add_parser("call", help="精确调用，不做任何解析/自动启用")
    call.add_argument("server")
    call.add_argument("tool")
    call.add_argument("--args-json", default="{}")
    call.add_argument("--arg", action="append", default=[])
    call.add_argument("--timeout", type=int, default=60)

    admin = sub.add_parser("admin", help="原样转发其余 mindx mcp 子命令（add/enable/disable/test/list）")
    admin.add_argument("rest", nargs=argparse.REMAINDER)

    doctor = sub.add_parser("doctor", help="打印 CLI 路径并探测主机")
    doctor.add_argument("--timeout", type=int, default=30)

    args = parser.parse_args()
    cli = resolve_cli()

    if args.command == "doctor":
        code, _, err = run_cli(cli, ["list", "--json"], timeout=args.timeout)
        status = "可达" if code == 0 else "不可达"
        print(json.dumps({"cli": cli, "daemon": status, "detail": "" if code == 0 else error_line(err)}, ensure_ascii=False, indent=1))
        sys.exit(0 if code == 0 else EXIT_NO_DAEMON)

    if args.command == "find":
        matches = match_servers(load_catalog(cli, refresh=args.refresh), args.keyword)
        if not matches:
            fail(EXIT_NO_SERVER, "SERVER_NOT_FOUND", f"没有匹配「{args.keyword}」的 MCP 服务。")
        print(json.dumps({"candidates": slim(matches), "more": len(matches) > 5}, ensure_ascii=False, indent=1))
        return

    if args.command == "tools":
        resolved = resolve_server(cli, args.server)
        server = resolved.get("name")
        code, out, err = run_cli(cli, ["tools", server, "--json"], timeout=args.timeout + 15)
        if code != 0:
            fail(EXIT_CALL_FAILED, "TOOLS_FAILED", error_line(err) or "列工具失败。")
        tools = json.loads(out)
        print(json.dumps({"server": server, "tools": [{k: t.get(k) for k in ("name", "description")} for t in tools[:20]]}, ensure_ascii=False, indent=1))
        return

    if args.command == "call":
        payload = dict_from_args(args.args_json, args.arg)
        raw, message = call_tool(cli, args.server, args.tool, payload, args.timeout)
        if raw is None:
            fail(EXIT_CALL_FAILED, "CALL_FAILED", message)
        print(json.dumps(format_result(raw, args.server, args.tool), ensure_ascii=False, indent=1))
        return

    if args.command == "admin":
        if not args.rest:
            parser.error("admin 需要至少一个子命令")
        passthrough(cli, args.rest)

    if args.command == "run":
        payload = dict_from_args(args.args_json, args.arg)
        cmd_run(cli, args.server, args.tool, payload, args.timeout, args.refresh)


def dict_from_args(args_json: str, pairs: list[str]) -> dict:
    try:
        payload = json.loads(args_json or "{}")
    except json.JSONDecodeError:
        fail(EXIT_CALL_FAILED, "BAD_ARGS", "--args-json 必须是合法 JSON 对象。")
    if not isinstance(payload, dict):
        fail(EXIT_CALL_FAILED, "BAD_ARGS", "--args-json 必须是 JSON 对象。")
    for pair in pairs:
        if "=" not in pair:
            fail(EXIT_CALL_FAILED, "BAD_ARGS", f"--arg 需要 K=V，收到 {pair}")
        key, value = pair.split("=", 1)
        payload[key] = value
    return payload


if __name__ == "__main__":
    main()
