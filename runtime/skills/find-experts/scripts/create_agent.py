#!/usr/bin/env python3
"""
Create a new agent definition file in the agents directory.

Usage:
    python create_agent.py --agents-dir <directory> \
        --name <agent_name> \
        --role <agent_role> \
        --description "<description>" \
        --model <model_name> \
        [--skills skill1,skill2,...]

Output: JSON object with the created agent info and file path.
"""

import os
import sys
import argparse

# Add scripts dir to path for yaml_helper import
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import yaml_helper


def build_agent_markdown(name, role, description, model, skills):
    lines = ["---"]
    lines.append(f"name: {name}")
    lines.append(f"role: {role}")
    lines.append("description: >")
    for line in description.strip().split("\n"):
        lines.append(f"  {line}")
    lines.append(f'model: "{model}"')
    if skills:
        lines.append("skills:")
        for s in skills:
            lines.append(f"  - {s}")
    lines.append("---")
    lines.append("")
    return "\n".join(lines) + "\n"


def main():
    parser = argparse.ArgumentParser(description="Create a new agent definition")
    parser.add_argument("--agents-dir", required=True, help="Path to agents directory")
    parser.add_argument("--name", required=True, help="Agent identifier (filename without .md)")
    parser.add_argument("--role", required=True, help="Short role title")
    parser.add_argument("--description", required=True, help="Detailed role description")
    parser.add_argument("--model", required=True, help="Model name to use")
    parser.add_argument("--skills", type=str, default="", help="Comma-separated skill names")
    args = parser.parse_args()

    if not os.path.isdir(args.agents_dir):
        yaml_helper.fail(f"Agents directory not found: {args.agents_dir}")

    filepath = os.path.join(args.agents_dir, f"{args.name}.md")
    if os.path.exists(filepath):
        yaml_helper.fail(f"Agent file already exists: {filepath}")

    skills_list = []
    if args.skills:
        skills_list = [s.strip() for s in args.skills.split(",") if s.strip()]

    content = build_agent_markdown(args.name, args.role, args.description, args.model, skills_list)

    with open(filepath, "w", encoding="utf-8") as f:
        f.write(content)

    result = {
        "status": "created",
        "name": args.name,
        "role": args.role,
        "model": args.model,
        "skills": skills_list,
        "file": filepath,
    }
    yaml_helper.output_json(result)


if __name__ == "__main__":
    main()
