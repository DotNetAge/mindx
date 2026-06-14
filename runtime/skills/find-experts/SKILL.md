---
name: find-experts
description: >
  Discover and collaborate with specialized agents. Use when the user's
  request falls outside your expertise, your tools/skills are insufficient,
  or the task needs multi-domain parallel work.
allowed-tools: sub-agent collect-results task-create team-create team-list bash
metadata:
  name_zh: 寻找专家
  name_zh-tw: 尋找專家
  description_zh: 发现并与专业智能体协作——当任务超出你的专业范围、工具或技能不足，或需要多领域并行工作时使用
  description_zh-tw: 發現並與專業智慧體協作——當任務超出你的專業範圍、工具或技能不足，或需要多領域並行工作時使用
---

## When to Use

Trigger when: request outside your expertise, tools/skills insufficient, or multi-domain task needs parallel delegation. Do NOT use for tasks you can handle directly.

## Prerequisite

The daemon must be running for all commands that query available resources:

```bash
mindx start
```

## Workflow

You are the orchestrator — own the outcome from start to finish.

### 1: Discover

```bash
mindx agent list --json
mindx model list --json
mindx skill list --json
```

Select experts by matching role + description against task requirements. For multi-domain tasks, pick multiple. If no suitable expert exists, use the **agent-creator** skill to create one (see agent-creator's workflow for details).

### 2: Delegate with `sub-agent`

Pass a self-contained brief: user context, deliverable, constraints. Each `sub-agent` call returns a tracking ID.

Repeat for each domain or expert. For multi-expert tasks, launch all `sub-agent` calls in parallel (async) then collect.

### 3: Team coordination (multi-expert tasks)

For complex tasks with parallel workstreams:
1. `team-create` — create a team from selected experts (`team_name`, `leader`, `members[]`, `description`)
2. `task-create` — create planning entries for each member (`subject`, `description`, `owner`)
3. `team-list` — verify team status
4. `sub-agent` — delegate to each member, pass tracking IDs
5. `collect-results` — gather all outputs; `task-update` to mark progress

### 4: Collect with `collect-results`

Pass the tracking ID(s) from step 2/3. Inspect results — verify completeness, correctness, edge cases. Do not accept polished output that misses the point.

### 5: Score

```bash
mindx agent score --agent-name "<name>" --task "<desc>" --score N --notes "<eval>"
```

Score 1-10: 9-10 exceptional, 7-8 good, 5-6 adequate with gaps, 3-4 significant gaps, 1-2 unusable. Be honest — inflated scores corrupt the statistical profile.

### 6: Report

Fully resolved → deliver result with summary of who contributed. Partial/issues → explain gaps and propose next steps (retry, different expert, or supplement yourself).

## Anti-Patterns

- Do not use `sub-agent` for tasks within your own expertise or trivial tasks you can finish faster
- Do not create an agent before running `mindx agent list --json` first
- Do not accept unverified output — inspect before reporting to user

## References

- `references/agent-best-practices.md` — shared with agent-creator
