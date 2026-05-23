---
name: introspect
description: >
  Discovers available skills and matches them against your agent definition
  (role, description, skills) to recommend which skills to equip.
  Use when the user asks about your capabilities, requests a self-assessment,
  or wants to know which skills to install. Also triggers after evolve
  generates new skills.
---

# When to Use This Skill

Trigger this skill when any of the following is true:

- The user asks "what can you do?", "recommend skills", "audit my skills",
  "review my capabilities", "self-assessment", "introspect"
- New skills were recently installed and you should evaluate them
- The `evolve` skill generated new skills and you need to assess fit
- The user asks what skills they should install for your role

**Do NOT use** for tasks already within your equipped skills — handle those directly.

---

## Workflow

## Step 1: Identify Yourself

List all agent definitions to confirm your own agent configuration:

```bash
python scripts/introspect whoami
```

Output (JSON array — all configured agents):

```json
[
  {
    "name": "developer",
    "role": "Software Engineer",
    "description": "Responsible for writing and maintaining code...",
    "skills": ["file-organizer", "pdf"],
    "model": "qwen3.6-plus"
  }
]
```

Locate your own entry by matching your agent name. Note your current `role`,
`description`, and already-equipped `skills`.

## Step 2: Discover Available Skills

List every skill available in the system:

```bash
python scripts/introspect list-skills
```

For machine-readable JSON:

```bash
python scripts/introspect list-skills --json
```

## Step 3: Match and Recommend

Compare your agent profile against all available skills. Recommend skills where:

- The skill's purpose aligns with your `role` and `description`
- The skill is **not already equipped** in your `skills` list
- Domain-specific skills (e.g. `verify`, `bug-hunter`, `architect`) over universal
  tools (`bash`, `grep`, `read`)

Present your findings:

```
Introspect complete — developer (Software Engineer)

Equipped (2): file-organizer, pdf

Recommended:
  verify          — Rigorous verification of changes
  bug-hunter      — Expert SOP for locating bugs

Would you like me to add these to your agent definition?
```

---

## Anti-Patterns

- **Never modify the agent definition automatically.** Recommendations only.
- **Don't recommend already-equipped skills.** Check the agent's `skills` list first.
- **Don't confuse tool skills with domain skills.** `bash`, `grep`, `read` are universal tools, not domain skills.
