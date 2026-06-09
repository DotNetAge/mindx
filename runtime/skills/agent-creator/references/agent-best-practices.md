# Agent Definition Reference

Every agent = single `.md` file with YAML frontmatter:

```markdown
---
name: <identifier>
role: <short-title>
description: >
  Multi-line capabilities...
model: "<model-name>"
skills:
  - skill-a
  - skill-b
---

## Identity (optional body)
...system prompt...
```

## Field Requirements

| Field         | Format                       | Rule                                                                            |
| ------------- | ---------------------------- | ------------------------------------------------------------------------------- |
| `name`        | lowercase-hyphen             | Unique, noun-based, reflects role (e.g. `python-engineer`)                      |
| `role`        | ~5 words                     | Human-readable, include seniority if helpful (e.g. `Senior Python Engineer`)    |
| `description` | YAML folded `>`, <1024 chars | Covers: what agent does, technical domains, quality standards, scope boundaries |
| `model`       | exact name from `model.list` | Match complexity to task — don't waste expensive models on trivial work         |
| `skills`      | list from `skill.list`       | Only domain-relevant skills — each adds context overhead                        |

## Body (Optional, after closing `---`)

Structure if included:
- **Introduction** — who the agent is
- **Core Responsibilities** — tasks handled directly
- **Scope Boundaries** — WITHIN scope / OUT OF scope (prevents misdelegation)

## Anti-Patterns

- Duplicate names — always check `agent.list` first
- Over-scoped descriptions ("does everything") — defeats specialist delegation
- Skill hoarding — every skill adds context overhead
- Model mismatch — heavy model for trivial tasks, light model for complex reasoning
- Missing scope boundaries — leads to misrouting
- Name-role mismatch — `python-engineer` with role "Full-Stack Developer" confuses routing

## Creation Checklist

Before running `create_agent.py`:
- [ ] Checked `agent.list` — confirmed no duplicate name exists
- [ ] Checked `skill.list` — identified relevant skills for this domain
- [ ] Checked `model.list` — selected appropriate model
- [ ] `name` lowercase-hyphenated, unique, noun-based
- [ ] `role` concise (~5 words), seniority if relevant
- [ ] `description` covers responsibilities, domains, quality, AND boundaries (<1024 chars)
- [ ] `model` matches task complexity
- [ ] `skills` minimal — only domain-relevant
