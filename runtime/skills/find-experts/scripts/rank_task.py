#!/usr/bin/env python3
"""
Record a performance score for an agent's task completion.

Scores are appended to the agent's YAML frontmatter under a `performance` key,
along with a `completes` counter that tracks the total number of scored tasks.
The average score is derivable from `scores` and `completes`.

Output: JSON object with scoring result, all scores, and completes count.
"""

import os
import re
import sys
import argparse
from datetime import datetime, timezone

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import yaml_helper


def find_frontmatter_bounds(content):
    """Find start and end positions of YAML frontmatter in markdown content.
    Returns (start, end) where start is index of opening --- line,
    and end is index of closing --- line (exclusive for the line).
    Returns (None, None) if no valid frontmatter found."""
    if not content.startswith("---"):
        return None, None

    end_marker = content.find("\n---", 3)
    if end_marker == -1:
        return None, None

    return 0, end_marker + 4


def add_performance_to_frontmatter(frontmatter_text, score_entry):
    """Insert or append a performance score to the frontmatter YAML text.
    Also increments the completes counter.
    Returns the modified frontmatter text."""
    lines = frontmatter_text.split("\n")
    perf_key_idx = None
    perf_indent = ""
    scores_key_idx = None
    scores_indent = ""
    completes_key_idx = None
    completes_indent = ""

    for i, line in enumerate(lines):
        stripped = line.strip()
        if stripped == "performance:" or stripped.startswith("performance:"):
            perf_key_idx = i
            perf_indent = line[:len(line) - len(line.lstrip())]
        if perf_key_idx is not None and stripped == "scores:":
            scores_key_idx = i
            scores_indent = line[:len(line) - len(line.lstrip())]
        if perf_key_idx is not None and re.match(r'^completes:\s*\d+', stripped):
            completes_key_idx = i
            completes_indent = line[:len(line) - len(line.lstrip())]

    if perf_key_idx is None:
        new_lines = [
            "performance:",
            "  scores:",
            "    - task: >",
            f"      {score_entry['task']}",
            f"      score: {score_entry['score']}",
            f"      timestamp: \"{score_entry['timestamp']}\"",
        ]
        if score_entry.get("notes"):
            new_lines.append(f'      notes: "{score_entry["notes"]}"')
        new_lines.append("  completes: 1")
        lines.extend(new_lines)
        return "\n".join(lines)

    if scores_key_idx is None:
        scores_indent = perf_indent + "  "
        insert_lines = [
            f"{scores_indent}scores:",
            f"{scores_indent}  - task: >",
            f"{scores_indent}    {score_entry['task']}",
            f"{scores_indent}    score: {score_entry['score']}",
            f"{scores_indent}    timestamp: \"{score_entry['timestamp']}\"",
        ]
        if score_entry.get("notes"):
            insert_lines.append(f'{scores_indent}    notes: "{score_entry["notes"]}"')
        lines = lines[:perf_key_idx + 1] + insert_lines + lines[perf_key_idx + 1:]
        # Add completes: 1 after scores block
        lines.insert(perf_key_idx + 1 + len(insert_lines), f"{scores_indent}completes: 1")
    else:
        last_score_idx = scores_key_idx
        item_indent = scores_indent + "  "
        for i in range(scores_key_idx + 1, len(lines)):
            if lines[i].startswith(item_indent):
                last_score_idx = i
            elif lines[i].strip() == "":
                last_score_idx = i
            elif not lines[i].startswith(item_indent) and lines[i].strip():
                deeper_indent = item_indent + "  "
                if lines[i].startswith(deeper_indent):
                    last_score_idx = i
                else:
                    break

        new_entry_lines = [
            f"{item_indent}- task: >",
            f"{item_indent}  {score_entry['task']}",
            f"{item_indent}  score: {score_entry['score']}",
            f"{item_indent}  timestamp: \"{score_entry['timestamp']}\"",
        ]
        if score_entry.get("notes"):
            new_entry_lines.append(f'{item_indent}  notes: "{score_entry["notes"]}"')

        lines = lines[:last_score_idx + 1] + new_entry_lines + lines[last_score_idx + 1:]

        # Adjust completes_key_idx after insertion
        if completes_key_idx is not None and completes_key_idx > last_score_idx:
            completes_key_idx += len(new_entry_lines)

    # Increment completes counter
    if completes_key_idx is not None:
        m = re.match(r'^(completes:\s*)(\d+)', lines[completes_key_idx].strip())
        if m:
            new_val = int(m.group(2)) + 1
            lines[completes_key_idx] = f"{completes_indent}{m.group(1)}{new_val}"
    else:
        # completes not found in existing frontmatter, add it
        completes_line = f"{perf_indent}  completes: 1"
        if len(lines) > 0 and lines[-1].strip() == "":
            lines.insert(len(lines) - 1, completes_line)
        else:
            lines.append(completes_line)

    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description="Score an agent's task performance")
    parser.add_argument("--agents-dir", required=True, help="Path to agents directory")
    parser.add_argument("--agent-name", required=True, help="Name of agent to score")
    parser.add_argument("--task", required=True, help="Description of the delegated task")
    parser.add_argument("--score", type=int, required=True, help="Score from 1 to 10")
    parser.add_argument("--notes", type=str, default="", help="Optional evaluation notes")
    args = parser.parse_args()

    if args.score < 1 or args.score > 10:
        yaml_helper.fail("Score must be between 1 and 10")

    filepath = os.path.join(args.agents_dir, f"{args.agent_name}.md")
    if not os.path.exists(filepath):
        yaml_helper.fail(f"Agent file not found: {filepath}")

    with open(filepath, "r", encoding="utf-8") as f:
        content = f.read()

    start, end = find_frontmatter_bounds(content)
    if start is None or end is None:
        yaml_helper.fail(f"No valid YAML frontmatter found in: {filepath}")

    # Parse existing frontmatter to extract existing scores for stats
    frontmatter = yaml_helper.parse_yaml_frontmatter(content)
    if frontmatter is None:
        yaml_helper.fail(f"Failed to parse frontmatter in: {filepath}")

    timestamp = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

    score_entry = {
        "task": args.task,
        "score": args.score,
        "timestamp": timestamp,
    }
    if args.notes:
        score_entry["notes"] = args.notes

    # Extract existing frontmatter text (between the two --- markers)
    frontmatter_text = content[start + 4:end - 4]

    # Modify the frontmatter text to add the new score
    new_frontmatter_text = add_performance_to_frontmatter(frontmatter_text, score_entry)

    # Rebuild the file content
    new_content = "---\n" + new_frontmatter_text + "\n---" + content[end:]

    with open(filepath, "w", encoding="utf-8") as f:
        f.write(new_content)

    # Read back to compute stats from the modified frontmatter text directly.
    # yaml_helper's simple parser does not handle nested dicts like performance.scores,
    # so we extract scores and completes via text parsing from the rewritten frontmatter block.
    scores = []
    completes = 0
    in_scores = False
    current_score = 0
    for line in new_frontmatter_text.split("\n"):
        stripped = line.strip()
        if stripped == "scores:":
            in_scores = True
            continue
        if stripped.startswith("completes:"):
            m = re.match(r'^completes:\s*(\d+)', stripped)
            if m:
                completes = int(m.group(1))
            continue
        if in_scores:
            m = re.match(r'^\s*score:\s*(\d+)', line)
            if m:
                current_score = int(m.group(1))
                continue
            if stripped.startswith("timestamp:"):
                scores.append(current_score)
                continue
            # Exit scores block when we hit a non-indented key at a higher level
            if line and not line[0].isspace() and re.match(r'^\w', line):
                break

    result = {
        "status": "scored",
        "agent": args.agent_name,
        "task": args.task,
        "score": args.score,
        "notes": args.notes if args.notes else None,
        "timestamp": timestamp,
        "scores": scores,
        "completes": completes,
    }
    yaml_helper.output_json(result)


if __name__ == "__main__":
    main()
