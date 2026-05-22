---
name: introspect
description: >
  Scans all available skills and matches them against your own agent definition
  (role, description, skills) to discover skills worth equipping.
  Use when the user says "审视", "自我检查", "introspect", "查看能力", "我有什么用",
  "应该装什么技能", "推荐技能", or after evolve generates new skills.
  Also triggers automatically when your skills list changes or new skills are installed,
  and when the user asks about your capabilities or what you can do.
allowed-tools: bash glob grep read
---

# Introspect — Self-Assessment

Scan every skill available in the system, compare against your agent definition, and recommend which skills are worth equipping.

The LLM knows the correct paths from the SystemPrompt — always pass them explicitly.

Your agent definition file (`<workspace>/agents/<your-name>.yml` or `.md`) already has a `skills` field — introspect tells you what to add to it.

**Core logic:**

```
All system skills ──→ Match against your role + description ──→ Recommend
                                                                      ↓
                                                      You update the `skills` list
```

---

## Phase 1: Discover the Capability Map

### List all available skills

Compact output (default) — one line per skill:

```bash
python scripts/introspect list-skills --skills-dir <workspace>/skills
```

Output:

```
verify                         Rigorous verification of changes through testing...
bug-hunter                     Expert SOP for locating, isolating and fixing bugs...
architect                      High-level orchestration for system design...
docker-expert                  Containerization and Docker expertise...
```

For machine-readable JSON (e.g. piping to another tool):

```bash
python scripts/introspect list-skills --skills-dir <workspace>/skills --json
```

### List all agents (optional)

For reference on what other agents have equipped:

```bash
python scripts/introspect list-agents --agents-dir <workspace>/agents
```

Compact output:

```
architect      System Architect     [architect, simplify, batch, find-experts]
developer      Software Engineer    [file-organizer, pdf]
```

Use `--json` for full detail.

---

## Phase 2: Match Analysis

### Run the matching engine

```bash
python scripts/introspect match <your-agent-name> --skills-dir <workspace>/skills --agents-dir <workspace>/agents
```

Output (JSON — already structured and compact enough for LLM consumption):

```json
{
  "agent_name": "developer",
  "role": "Software Engineer",
  "equipped_skills": ["file-organizer", "pdf"],
  "recommended": [
    {"name": "verify", "score": 3.0, "equipped": false},
    {"name": "bug-hunter", "score": 2.5, "equipped": false}
  ],
  "orphaned": [],
  "already_equipped": [
    {"name": "file-organizer", "score": 1.0},
    {"name": "pdf", "score": 0.5}
  ]
}
```

### Interpreting results

| Field | Meaning |
|-------|---------|
| `recommended` | High match but not yet equipped — **should be added** |
| `already_equipped` | Already equipped and matching — **keep as is** |
| `orphaned` | Equipped but doesn't align with your role — **consider removing** |
| `score` | Match strength; ≥ 2.0 is the recommendation threshold |

**A low score doesn't mean a skill is useless.** It only means the skill isn't directly related to your core domain. The user may have deliberately equipped cross-domain skills for specific needs — honor those decisions.

---

## Phase 3: Report and Recommend

Present the analysis clearly to the user.

### When there are recommendations

```
Introspect complete!

Agent: developer (Software Engineer)

Recommended (3):
  verify          — Test verification (match 3.0)
  bug-hunter      — Bug localization (match 2.5)
  code-reviewer   — Code review (match 2.0)

Orphaned (0):

Would you like me to add these to your agent definition?
Edit: <workspace>/agents/developer.yml → add to the `skills:` list.
```

### When everything is optimal

```
Introspect complete — your skill configuration is well-aligned.

5 skills equipped, 4 with strong role match.
No new recommendations.
```

### After evolve generates new skills

Coordinate with evolve:

1. Run `evolve` → generates `evolved-pr-review-flow`
2. Run `introspect match <your-name>` to check if the new skill fits
3. If it appears in `recommended` → add it
4. If not → the skill doesn't match your role; keep it available for other agents

---

## Edge Cases

| Situation | Handling |
|-----------|----------|
| Agent definition file missing | Report "agent definition not found", suggest saving config first |
| No skills directory | Report "no skills installed in the system" |
| All skills already equipped | Report "your skill list is already comprehensive" |
| All match scores very low | Explain that the skill library doesn't overlap with this agent's domain; suggest installing relevant skills |
| New evolved skills detected | Prompt the user to run match to decide whether to equip |

---

## Anti-Patterns

- **Never modify the agent definition automatically.** The `skills` list is the user's decision space. Introspect only recommends.
- **Prioritize role match over description match.** The `role` field is the agent's core identity; `description` provides supplementary context.
- **Don't confuse tool skills with domain skills.** `bash`, `grep`, `read` are universal tools everyone uses. `verify`, `bug-hunter`, `architect` are domain-specific professional skills. Introspect matches the latter.
- **Evolved skills carry an `evolved-` prefix.** They score slightly differently since auto-generated quality may vary. The final judgment is always the user's.
- **Don't recommend the same skill twice.** Once a skill is equipped, it drops out of recommendations in future runs.
