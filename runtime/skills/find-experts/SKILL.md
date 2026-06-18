---
name: find-experts
description: >
  Discover and collaborate with specialized agents. Use when the user's
  request falls outside your expertise, your tools/skills are insufficient,
  or the task needs multi-domain parallel work.
allowed-tools: sub-agent collect-results task-create task-update task-get task-list team-create team-list team-get-tasks team-delete bash
metadata:
  name_zh: 寻求协作
  name_zh-tw: 尋求協作
  description_zh: 发现并与专业智能体协作——当任务超出你的专业范围、工具或技能不足，或需要多领域并行工作时使用
  description_zh-tw: 發現並與專業智慧體協作——當任務超出你的專業範圍、工具或技能不足，或需要多領域並行工作時使用
---

## Trigger Decision

Use this skill when **any** of these is true:

- The task requires domain knowledge outside your core expertise
- The task decomposes into independent sub-domains suitable for parallel delegation
- The user explicitly requests a specialist role ("find a security expert", "get a code reviewer")

Do **NOT** use this skill when:

- You can complete the task directly with your own tools and skills
- The task is a simple file edit, search, or single command execution
- Delegation overhead would exceed just doing it yourself

## Mode Selector

Choose the execution mode based on task complexity:

```
Task complexity
    │
    ├─ Single domain, one-off question
    │   → Mode 0: Single Expert
    │
    ├─ Multiple independent domains, no dependencies
    │   → Mode 1: Parallel Experts
    │
    └─ Complex multi-phase work with interdependencies
        → Mode 2: Team Orchestration
```

## Mode 0: Single Expert

**When**: One specialist opinion or action is needed.

**Flow**:

1. `mindx agent list --json` — pick one expert matching the domain
2. `sub-agent(agent_name, task)` — delegate (task must be self-contained)
3. `collect-results(task_ids)` — block until done
4. Inspect result → deliver or supplement yourself

**Coordination rules**:

- Write the `task` parameter as a self-contained brief. The sub-agent cannot see our conversation context.
- Include in the brief: user's original request, specific deliverable format, constraints.
- If the result is insufficient, either retry with a refined task or supplement the gap yourself.

---

## Mode 1: Parallel Experts

**When**: Multiple independent domains need simultaneous work (e.g., security audit + performance review + test coverage).

**Flow**:

1. `mindx agent list --json` — identify all required experts (one per domain)
2. Launch ALL `sub-agent` calls in a **single response** — they execute in parallel automatically
3. `collect-results(task_ids)` — gather all results at once
4. `task-create` + `task-update(completed)` — track what was done (optional but recommended)
5. Synthesize results into a unified answer

**Coordination rules**:

- All `sub-agent` calls in the same response run in parallel. Do NOT split them across multiple responses — that serializes them.
- Each `task` brief must be independent and self-contained. No cross-references between briefs ("see what the other expert finds").
- If any sub-agent fails, decide: retry with clearer instructions, substitute yourself, or report the partial result with explanation.
- When collecting results, do not forward raw output blindly. Verify completeness before reporting to the user.

---

## Mode 2: Team Orchestration

**When**: Complex task with phased workstreams, role assignments, dependency tracking, and progress monitoring needed.

**Flow**:

### Phase 1: Assemble

```
team-create(
  team_name="<kebab-case-name>",
  description="<what the team is working on>",
  leader="<your name or designated coordinator>",
  members=["<expert-1>", "<expert-2>", ...],
  tasks=["<task-for-member-1>", "<task-for-member-2>", ...]  // optional, auto-assigned round-robin
)
```

**Key detail**: The `tasks` parameter auto-creates Task records and assigns owners round-robin across members. Prefer this over manual `task-create` per member.

If you need custom assignment (not round-robin), omit `tasks` from `team-create`, then manually:

```
task-create(subject, description)          // create without owner
task-update(task_id, owner="<expert-name>") // assign specifically
```

### Phase 2: Wire Dependencies (if needed)

If some work must precede others:

```
// "Architecture design" must finish before "Backend implementation"
task-update(task_id=backend_task, addBlockedBy=[architecture_task])

// Both "Frontend" and "Tests" depend on "API design"
task-update(task_id=frontend_task, addBlockedBy=[api_task])
task-update(task_id=test_task, addBlockedBy=[api_task])
```

The system rejects circular dependencies automatically. You don't need to check manually.

### Phase 3: Execute by Wave

Execute tasks in dependency order. Independent tasks in the same wave are dispatched together:

```
// Wave 1: No-dependency tasks
task-update(task_id=wave1_tasks, status="in_progress")
sub-agent(agent_name=..., task=...)  // all wave-1 calls in ONE response
collect-results(task_ids=[...])
task-update(task_id=wave1_tasks, status="completed")

// Wave 2: Tasks that depended on wave 1
task-update(task_id=wave2_tasks, status="in_progress")
sub-agent(...)  // all wave-2 calls in ONE response
collect-results(...)
task-update(task_id=wave2_tasks, status="completed")

// Repeat until all waves done
```

### Phase 4: Verify and Close

```
team-get-tasks(team_name="...")  // confirm all completed
team-delete(team_name="...")     // cleanup
```

**Coordination rules**:

- Always mark `in_progress` before dispatching, `completed` immediately after collecting results.
- Every 3rd completion triggers a verification nudge — act on it (run tests, review files).
- If a sub-agent result is inadequate, use `task-update(status="cancelled")` with notes, then re-delegate or handle yourself.
- Do not leave tasks stuck in `in_progress`. If blocked, investigate or cancel.

---

## Cross-Mode Rules (Always Apply)

| Rule                                      | Rationale                                                          |
| ----------------------------------------- | ------------------------------------------------------------------ |
| SubAgent `task` must be self-contained    | Sub-agents have zero visibility into our conversation context      |
| Multiple SubAgent calls = parallelism     | Same response → concurrent execution; split responses → serialized |
| CollectResults after every SubAgent batch | Results sit in ResultStore until collected; uncollected = wasted   |
| TaskUpdate immediately on state change    | Don't batch status updates; real-time tracking enables recovery    |
| Never trust sub-agent output blindly      | Always inspect before forwarding to user                           |
| Score experts honestly                    | Inflated scores corrupt the selection statistics for future calls  |

## Quality Gates

Before delivering the final result to the user:

1. **Completeness**: Does the result address all parts of the original request?
2. **Correctness**: Are there factual errors, hallucinations, or logical gaps?
3. **Actionability**: Can the user actually use this result, or does it need synthesis/formatting?
4. **Attribution**: Clearly note which expert(s) contributed which parts.

## Scoring

After each engagement, score the expert(s) used:

```bash
mindx agent score --agent-name "<name>" --task "<desc>" --score N --notes "<eval>"
```

| Score | Meaning                                         |
| ----- | ----------------------------------------------- |
| 9-10  | Exceptional — exceeded expectations             |
| 7-8   | Good — solid result, usable as-is               |
| 5-6   | Adequate — usable with gaps you had to fill     |
| 3-4   | Significant gaps — major supplementation needed |
| 1-2   | Unusable — you essentially redid the work       |

Be honest. Inflated scores harm future expert selection.

## Reporting Format

### Full resolution

```
Done. Here's the result:

[Result content]

Contributors:
- <expert-name> (<role>): <what they delivered>
- <expert-name> (<role>): <what they delivered>
- Self: <what you supplemented or synthesized>
```

### Partial / issues

```
Partial result. Here's what I was able to obtain:

[What was completed]

Gaps / Issues:
- <description of what's missing or wrong>

Suggested next steps:
- Option A: Retry with <adjusted approach>
- Option B: Try a different expert: <name> (<reason>)
- Option C: I can handle the remaining part directly
```

## Anti-Patterns

- Do not use `sub-agent` for tasks within your own expertise or trivial tasks you can finish faster yourself
- Do not create an agent before running `mindx agent list --json` first — an expert may already exist
- Do not accept unverified sub-agent output — always inspect before reporting to the user
- Do not split parallel `sub-agent` calls across multiple responses — that defeats parallelism
- Do not leave tasks in `in_progress` indefinitely — complete or cancel them
- Do not skip `collect-results` — uncollected results consume resources with no benefit
- Do not inflate scores — honest feedback improves the expert pool over time
