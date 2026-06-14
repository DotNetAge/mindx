#!/usr/bin/env python3
"""
Create a new agent.

Usage:
    python create_agent.py --name <agent_name> --role <agent_role> \\
        --description "<description>" --model <model_name> \\
        [--body "<system_prompt>"] [--skills skill1,skill2,...]

If --body is omitted, a default prompt is generated from the role and description.
Output: JSON object with status, agent_name, and message.
"""

import json
import os
import sys
import argparse

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import rpc_client


def build_default_body(name, role, description):
    return (
        f"## Identity\n\n"
        f"I am a **{role}** — {description}\n\n"
        f"## Core Responsibilities\n\n"
        f"As a {role}, I handle tasks related to my domain of expertise. "
        f"I work autonomously and report results back to the delegating agent."
    )


def main():
    parser = argparse.ArgumentParser(description="Create a new agent")
    parser.add_argument("--name", required=True, help="Agent identifier")
    parser.add_argument("--role", required=True, help="Short role title")
    parser.add_argument("--description", required=True, help="Detailed role description")
    parser.add_argument("--model", required=True, help="Model name to use")
    parser.add_argument("--body", default="", help="Full system prompt / body")
    parser.add_argument("--skills", type=str, default="", help="Comma-separated skill names")
    args = parser.parse_args()

    skills_list = []
    if args.skills:
        skills_list = [s.strip() for s in args.skills.split(",") if s.strip()]

    body = args.body
    if not body:
        body = build_default_body(args.name, args.role, args.description)

    params = {
        "name": args.name,
        "role": args.role,
        "description": args.description,
        "model": args.model,
        "body": body,
    }
    if skills_list:
        params["skills"] = skills_list

    result = rpc_client.rpc_call("agent.create", params)
    print(json.dumps(result, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
