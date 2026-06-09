---
name: find-experts
description: >
  Discover and collaborate with specialized agents. Use when the user's
  request falls outside your expertise, your tools/skills are insufficient,
  or the task needs multi-domain parallel work.
allowed-tools: sub-agent collect-results task-create team-create team-list bash
---

## When to Use

Trigger when: request outside your expertise, tools/skills insufficient, or multi-domain task needs parallel delegation. Do NOT use for tasks you can handle directly.

## Workflow

You are the orchestrator — own the outcome from start to finish.

### 1: Discover

```bash
python3 scripts/list_agents.py
python3 scripts/list_models.py
python3 scripts/list_skills.py
```

Select experts by matching role + description against task requirements. For multi-domain tasks, pick multiple. If no suitable expert exists, use the **agent-creator** skill to create one (`--body` is the system prompt).

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
python3 scripts/rank_task.py --agent-name "<name>" --task "<desc>" --score N --notes "<eval>"
```

Score 1-10: 9-10 exceptional, 7-8 good, 5-6 adequate with gaps, 3-4 significant gaps, 1-2 unusable. Be honest — inflated scores corrupt the statistical profile.

### 6: Report

Fully resolved → deliver result with summary of who contributed. Partial/issues → explain gaps and propose next steps (retry, different expert, or supplement yourself).

## Anti-Patterns

- Do not use `sub-agent` for tasks within your own expertise or trivial tasks you can finish faster
- Do not create an agent before running `list_agents.py` first
- Do not accept unverified output — inspect before reporting to user

## References

- `references/agent-best-practices.md` — shared with agent-creator
