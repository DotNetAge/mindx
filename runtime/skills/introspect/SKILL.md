---
name: introspect
description: >
  Discover available skills and match them against your agent profile to
  recommend which skills to equip. Use when the user asks about your
  capabilities or wants to optimize your skill set.
metadata:
  name_zh: 自我审视
  name_zh-tw: 自我審視
  description_zh: 发现可用技能并与你的智能体画像匹配，推荐应配备哪些技能
  description_zh-tw: 發現可用技能並與你的智慧體畫像匹配，推薦應配備哪些技能
---

## When to Use

- User asks "what can you do?", "recommend skills", "audit me", "introspect"
- New skills were recently installed or generated (e.g. by `evolve`)
- You need to reassess your skill fit after a role/description change

**Do NOT use** for tasks you can already handle with equipped skills.


## Workflow

### 1. Get your profile

All agents are returned as JSON — find your entry by matching your `name`:

```bash
mindx agent list --json
```

Note your current `role`, `description`, and already-equipped `skills`.

### 2. Discover available skills

```bash
mindx skill list --json
```

### 3. Match and recommend

Compare your profile against available skills. Recommend where:
- Skill purpose aligns with your `role`/`description`
- Skill is **not already equipped**
- Domain-specific skills over universal tools (`bash`, `grep`, `read`)

Present as:
```
Introspect complete — <name> (<role>)

Equipped (N): skill-a, skill-b

Recommended:
  skill-x  — Description
  skill-y  — Description

Would you like me to add these to your agent definition?
```
