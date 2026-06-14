#!/usr/bin/env python3
"""
Unified CLI for agent operations via JSON-RPC.

Aligns with Daemon RPC interface:
  - agent.list (no params) -> list all agents
  - agent.get  (name)      -> get single agent (errors if not found)
  - agent.create (name, role, description, model [, skills, introduction, meta])
  - model.list (no params) -> list all models
  - skill.list (no params) -> list all skills

Usage:
    python agent_cli.py list models
    python agent_cli.py list skills
    python agent_cli.py list agents
    python agent_cli.py check <name>
    python agent_cli.py create --name <n> --role <r> --description <d> --model <m> [options...]
"""

import json
import os
import sys
import argparse

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import rpc_client


RPC_METHODS = {
    "models": "model.list",
    "skills": "skill.list",
    "agents": "agent.list",
}


def cmd_list(args):
    """List models, skills, or agents via RPC."""
    method = RPC_METHODS.get(args.what)
    if not method:
        print(f"Unknown list target: {args.what}", file=sys.stderr)
        sys.exit(1)
    result = rpc_client.rpc_call(method)
    print(json.dumps(result, indent=2, ensure_ascii=False))


def cmd_check(args):
    """Check if an agent name already exists.
    
    Uses agent.list (no-param) and filters client-side, because
    agent.get returns an RPC error on not-found which triggers
    rpc_client's sys.exit(1).
    """
    result = rpc_client.rpc_call("agent.list")
    agents = result if isinstance(result, list) else []
    name = args.name.lower()

    matches = [a for a in agents if a.get("name", "").lower() == name]
    if matches:
        print(json.dumps({"exists": True, "agent": matches[0]}, indent=2, ensure_ascii=False))
    else:
        print(json.dumps({"exists": False, "agent": None}, indent=2, ensure_ascii=False))


def cmd_create(args):
    """Create a new agent via agent.create RPC."""
    skills_list = [s.strip() for s in args.skills.split(",") if s.strip()] if args.skills else []

    params = {
        "name": args.name,
        "role": args.role,
        "description": args.description,
        "model": args.model,
    }
    if skills_list:
        params["skills"] = skills_list
    if args.introduction:
        params["introduction"] = args.introduction
    if args.name_zh or args.name_zh_tw:
        meta = {}
        if args.name_zh:
            meta["name_zh"] = args.name_zh
        if args.name_zh_tw:
            meta["name_zh_tw"] = args.name_zh_tw
        params["meta"] = meta

    result = rpc_client.rpc_call("agent.create", params)
    print(json.dumps(result, indent=2, ensure_ascii=False))


def main():
    parser = argparse.ArgumentParser(description="MindX Agent CLI")
    sub = parser.add_subparsers(dest="command", required=True)

    # --- list ---
    list_p = sub.add_parser("list", help="List models, skills, or agents")
    list_p.add_argument("what", choices=["models", "skills", "agents"])
    list_p.set_defaults(func=cmd_list)

    # --- check ---
    check_p = sub.add_parser("check", help="Check if an agent name exists")
    check_p.add_argument("name", help="Agent name to check")
    check_p.set_defaults(func=cmd_check)

    # --- create ---
    create_p = sub.add_parser("create", help="Create a new agent")
    create_p.add_argument("--name", required=True, help="Agent identifier (lowercase-hyphen)")
    create_p.add_argument("--role", required=True, help="Short role title")
    create_p.add_argument("--description", required=True, help="Description for LLM routing")
    create_p.add_argument("--model", required=True, help="Model name to use")
    create_p.add_argument("--skills", type=str, default="", help="Comma-separated skill names")
    create_p.add_argument("--introduction", type=str, default="", help="Full system prompt / working instructions for the agent")
    create_p.add_argument("--name-zh", type=str, default="", help="Chinese name (stored in meta.name_zh)")
    create_p.add_argument("--name-zh-tw", type=str, default="", help="Traditional Chinese name (stored in meta.name_zh_tw)")
    create_p.set_defaults(func=cmd_create)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
