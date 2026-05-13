#!/usr/bin/env python3
"""
List all agent definitions from the agents directory.

Usage:
    python list_agents.py --agents-dir <directory>

Output: JSON array of agent summaries.
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
    parser = argparse.ArgumentParser(description="List available agents")
    parser.add_argument("--agents-dir", required=True, help="Path to agents directory")
    args = parser.parse_args()

    agents_dir = args.agents_dir
    if not os.path.isdir(agents_dir):
        yaml_helper.fail(f"Agents directory not found: {agents_dir}")

    agents = []
    for filepath in sorted(glob.glob(os.path.join(agents_dir, "*.md"))):
        agent = parse_agent_file(filepath)
        if agent:
            agents.append(agent)

    yaml_helper.output_json(agents)


if __name__ == "__main__":
    main()
