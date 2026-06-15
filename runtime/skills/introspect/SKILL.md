---
name: introspect
description: >
  Discover available skills and match them against your agent profile to
  recommend which skills to equip or remove. Use when the user asks about your
  capabilities or wants to optimize your skill set.
allowed-tools: bash
metadata:
  name_zh: 自我审视
  name_zh-tw: 自我審視
  description_zh: 发现可用技能并与你的智能体画像匹配，推荐应配备或移除哪些技能
  description_zh-tw: 發現可用技能並與你的智慧體畫像匹配，推薦應配備或移除哪些技能
---

## Trigger Decision

Use this skill when:

- User asks "what can you do?", "recommend skills", "audit me", "introspect", "optimize"
- New skills were recently installed or generated (e.g. by `evolve`)
- You need to reassess skill fit after a role/description change

**Do NOT use** for tasks you can already handle with equipped skills.

## Workflow

### 1: Gather Data

```bash
mindx agent list --json        # find your own entry
mindx agent get <your-name>     # get your full config (role, description, current skills)
mindx skill list --json         # all available skills in the system
```

Extract from the results:
- Your `name`, `role`, `description`, `model`, current `skills[]`
- The pool of available skill names + their descriptions

### 2: Analyze — Score Each Available Skill

For every skill in the pool that is **not already equipped**, evaluate on these dimensions:

| Dimension       | Weight   | Question                                                  |
| --------------- | -------- | --------------------------------------------------------- |
| Role match      | High     | Does the skill's purpose align with my role keywords?     |
| Description fit | High     | Would this skill help with tasks matching my description? |
| Tool complement | Medium   | Does it provide tools/abilities I don't currently have?   |
| Overlap risk    | Negative | Does it duplicate functionality I already have?           |

Score each dimension 1-3, sum weighted scores. Only recommend skills above threshold.

### 3: Output Recommendations

Present as:

```
Introspect complete — <name> (<role>)

Current setup:
  Model: <model>
  Equipped (N): skill-a, skill-b, skill-c

Recommended to add (M):
  ⭐ skill-x  — <why it fits, which dimension scored high>
  ⭐ skill-y  — <why it fits>

Not recommended:
  skill-p  — <reason: overlap / out-of-scope / low relevance>

Potentially redundant (already equipped):
  skill-b  — <reason: may overlap with skill-a / no longer needed>

Would you like me to apply these changes?
```

### 4: Apply Changes (if user confirms)

When the user approves, execute updates:

```bash
# Add new skills (append to existing, do not replace)
mindx agent update --agent-name "<your-name>" --skills "existing-skill-1,existing-skill-2,<new-skill-x>,<new-skill-y>"

# Optionally update role/description if they evolved
# mindx agent update --agent-name "<your-name>" --role "Updated role"
```

**Important**: The `--skills` flag replaces the entire skills list. Always include all current skills plus any new ones.

If the user wants to remove redundant skills, omit them from the `--skills` list.

### 5: Reverse Introspection (Audit)

Also check for issues in the current configuration:

| Check                                        | Action if issue found                         |
| -------------------------------------------- | --------------------------------------------- |
| Skills with no matching available skill file | Warn user — skill may have been deleted/moved |
| Role vs skills mismatch                      | Suggest role or skills update                 |
| More than 8 equipped skills                  | Warn about context bloat — suggest pruning    |
| No skills equipped at all                    | Strongly recommend adding foundational skills |

Report findings as part of step 3 output under an "Audit notes" section.

## Anti-Patterns

- Do not recommend skills the user already has equipped
- Do not replace the entire skill list without preserving existing ones
- Do not recommend based solely on keyword matching — consider actual utility
- Do not skip the reverse audit — finding what to remove is as valuable as finding what to add
