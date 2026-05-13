#!/usr/bin/env python3
"""
Find a specific agent by name, role, or description keyword.

Usage:
    python find_agent.py --agents-dir <directory> [--name <name>] [--role <keyword>] [--desc <keyword>]

At least one search filter must be provided. Matches are case-insensitive.

Output: JSON array of matching agents (may be empty).
"""

import os
import sys
import argparse
import glob

# Add scripts dir to path for yaml_helper import
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import yaml_helper


def parse_agent_file(filepath):
    with open(filepath, "r", encoding="utf-8") as f:
        content = f.read()

    frontmatter = yaml_helper.parse_yaml_frontmatter(content)
    if frontmatter is None:
        return None

    return {
        "name": frontmatter.get("name", ""),
        "role": frontmatter.get("role", ""),
        "description": frontmatter.get("description", ""),
        "model": frontmatter.get("model", ""),
        "skills": frontmatter.get("skills", []) if isinstance(frontmatter.get("skills"), list) else [],
    }


def main():
    parser = argparse.ArgumentParser(description="Find agents by name, role, or description")
    parser.add_argument("--agents-dir", required=True, help="Path to agents directory")
    parser.add_argument("--name", type=str, help="Exact agent name to find")
    parser.add_argument("--role", type=str, help="Keyword to search in role")
    parser.add_argument("--desc", type=str, help="Keyword to search in description")
    args = parser.parse_args()

    if not args.name and not args.role and not args.desc:
        yaml_helper.fail("At least one filter (--name, --role, --desc) is required")

    if not os.path.isdir(args.agents_dir):
        yaml_helper.fail(f"Agents directory not found: {args.agents_dir}")

    matches = []
    for filepath in sorted(glob.glob(os.path.join(args.agents_dir, "*.md"))):
        agent = parse_agent_file(filepath)
        if agent is None:
            continue

        if args.name:
            if agent["name"].lower() == args.name.lower():
                matches.append(agent)
                continue
            else:
                continue

        if args.role:
            if args.role.lower() not in agent["role"].lower():
                continue

        if args.desc:
            if args.desc.lower() not in agent["description"].lower():
                continue

        matches.append(agent)

    yaml_helper.output_json(matches)


if __name__ == "__main__":
    main()
