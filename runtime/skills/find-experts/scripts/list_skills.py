#!/usr/bin/env python3
"""
List all installed skills from the skills directory.

Usage:
    python list_skills.py --skills-dir <directory>

Output: JSON array of skill summaries.
"""

import os
import sys
import argparse

# Add scripts dir to path for yaml_helper import
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import yaml_helper


def parse_skill_definition(skill_dir):
    skill_md = os.path.join(skill_dir, "SKILL.md")
    if not os.path.isfile(skill_md):
        return None

    with open(skill_md, "r", encoding="utf-8") as f:
        content = f.read()

    frontmatter = yaml_helper.parse_yaml_frontmatter(content)
    if frontmatter is None:
        return None

    name = frontmatter.get("name", os.path.basename(skill_dir))
    description = frontmatter.get("description", "")

    return {"name": name, "description": description}


def main():
    parser = argparse.ArgumentParser(description="List installed skills")
    parser.add_argument("--skills-dir", required=True, help="Path to skills directory")
    args = parser.parse_args()

    if not os.path.isdir(args.skills_dir):
        yaml_helper.fail(f"Skills directory not found: {args.skills_dir}")

    skills = []
    for entry in sorted(os.listdir(args.skills_dir)):
        entry_path = os.path.join(args.skills_dir, entry)
        if os.path.isdir(entry_path):
            skill = parse_skill_definition(entry_path)
            if skill:
                skills.append(skill)

    yaml_helper.output_json(skills)


if __name__ == "__main__":
    main()
