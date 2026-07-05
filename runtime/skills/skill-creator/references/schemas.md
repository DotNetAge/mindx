# MindX Skill Schemas

This document defines the data schemas used by MindX skills.

## Skill Locations

A skill is a directory containing at least a `SKILL.md` file. The daemon manages the skills registry internally. Skills are installed into the managed registry with:

```bash
mindx skill add <path-to-skill-directory>
```

After installation, run `mindx reload skills` so the daemon picks up the new version.

## SKILL.md Frontmatter

A MindX skill is a Markdown file with a YAML frontmatter block at the top.

### Required Fields

```yaml
---
name: <kebab-case-name>
description: >
  <One sentence for LLM routing>
---
```

### Optional Fields

```yaml
---
name: <kebab-case-name>
description: >
  <One sentence for LLM routing>
allowed-tools: Read Edit Task Bash Grep Glob
metadata:
  requires:
    bins:
      - python3
    env:
      - MY_API_KEY
  name_zh: <中文名>
  description_zh: <中文描述>
---
```

### Field Definitions

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | Yes | Unique skill identifier. Lowercase letters, digits, and hyphens only. Must start with a letter. |
| `description` | string | Yes | Routing description for the LLM. Should state what the skill does and when to use it. Keep it under 1024 characters. |
| `allowed-tools` | string | No | Space-separated list of tools the skill may use when loaded. Must be valid MindX tool names. |
| `metadata` | object | No | Arbitrary key-value metadata. Spec extensions (including `requires`) are stored here. |
| `metadata.requires` | object | No | Runtime dependency declarations. If unmet at load time, the skill is quietly skipped. |
| `metadata.requires.bins` | list of strings | No | Required executables on PATH. If any is missing, the skill is skipped. Example: `["python3", "git"]`. |
| `metadata.requires.env` | list of strings | No | Required environment variables. If any is empty/not set, the skill is skipped. Example: `["MY_API_KEY"]`. |
| `metadata.name_zh` | string | No | Simplified Chinese display name. |
| `metadata.name_zh-tw` | string | No | Traditional Chinese display name. |
| `metadata.description_zh` | string | No | Simplified Chinese description. |
| `metadata.description_zh-tw` | string | No | Traditional Chinese description. |

### Validation Rules

- `name` must be unique within the skill registry
- `description` must be non-empty and should not exceed 1024 characters
- `allowed-tools` entries must be from the MindX tool set
- `metadata.requires.bins` checks are performed by the daemon at load time via `exec.LookPath`; unmet bins cause the skill to be skipped
- `metadata.requires.env` checks are performed by the daemon at load time via `os.Getenv`; missing/empty env vars cause the skill to be skipped
- Directory name should match the `name` field

## Example SKILL.md

```markdown
---
name: sql-reviewer
description: >
  Reviews SQL queries for correctness, performance, and security issues.
  Use when the user provides a SQL query and asks for feedback or optimization.
allowed-tools: Read Edit Task
metadata:
  name_zh: SQL 审查者
  description_zh: 审查 SQL 查询的正确性、性能和安全性问题
---

## When to Use

- User asks "review this SQL query"
- User asks "is this query safe?"
- User asks "how can I optimize this query?"

## Workflow

### Step 1: Read the Query

Read the SQL query from the user's message or from a file they referenced.

### Step 2: Check Correctness

Verify that the query is syntactically valid and that table/column names exist.

### Step 3: Check Security

Look for SQL injection risks, missing parameterization, and overly broad permissions.

### Step 4: Check Performance

Identify missing indexes, unnecessary joins, full table scans, and N+1 patterns.

### Step 5: Provide Feedback

Return a concise report with:

- Critical issues (must fix)
- Warnings (should fix)
- Suggestions (optional improvements)
- A corrected query if applicable

## Important Notes

- Do not execute queries against a database unless explicitly asked.
- Do not modify user files unless the user asks for a rewrite.
- Keep feedback focused on SQL-specific concerns.
```
