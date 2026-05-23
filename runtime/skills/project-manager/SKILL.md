---
name: project-manager
description: >
  Turns vague ideas into structured projects with recurring tasks —
  decompose goals, assign recurring work to agents considering
  dependencies and priorities, track progress, adjust the plan
  continuously, and proactively report. Use for any long-running,
  multi-agent, or periodic work.
---

# When to Use This Skill

- The user has a long-term or recurring need (运营小红书、每日简报、定期报告)
- The user says "项目", "计划", "manage", "run", "operate"
- The task is too large or ongoing for one-shot execution
- You need to decompose a vague goal into concrete recurring tasks
- You receive a message asking you to check a project's status

**Do NOT use** for tasks you can complete directly in one response.

---

## Workflow

### Phase 1: Plan — Understand and Decompose

Talk to the user. Extract a **measurable** goal before writing anything down.

| Ask | Purpose |
|-----|---------|
| "What does success look like?" | Define the finish line |
| "How will you measure it?" | Make it quantifiable |
| "Any deadlines?" | Set time boundaries |
| "Does any part repeat?" | Identify recurring work |

Confirm the plan with the user. Then decompose the goal into deliverables,
each deliverable into concrete recurring tasks. Every task answers three
questions: **who does it, when do they do it, what do they do**.

Tasks may depend on each other and have different priorities. This is why
they're recorded in a graph database — dependencies, priorities, and
assignments are all connected and queryable.

Record the project structure. **Each task gets a unique task ID — this ID
will become the session_id for all communication about this task with the
assigned agent.**

```bash
python3 scripts/graph_client.py create-project --name "..." ...
python3 scripts/graph_client.py create-goal --project-id "..." ...
# Save the returned task ID — it's your session_id for talking to this agent
# about this specific work.
python3 scripts/graph_client.py create-task --goal-id "..." --agent "@x" --prompt "..." ...
```

### Phase 2: Assign — Set Recurring Work

For each recurring task, assign it to the agent with its timing and prompt.
This tells the system: every Monday at 9 AM, @writer writes a blog post.

```bash
python3 scripts/assign-task.py --agent "@writer" --task "..." --cron "0 0 9 * * 1"
```

**Important:** The task prompt must include a reporting instruction. Without it,
the agent finishes its work silently and you never hear back. Add this to
every task prompt:

> When you finish, use AgentTalk to report the result to project-manager
> in session "{task_id}" with a summary of what was done.

Use the **task ID** as the session_id — not the project ID. Each task has its
own conversation thread so multiple discussions with the same agent stay
separate. The task_id is returned by both `create-task` and `assign-task`.

This closes the loop — the agent reports back autonomously after each execution,
and you know exactly which session to use when following up.

### Phase 3: Track — Review Progress

Agents report back automatically via AgentTalk after each execution,
using the **task ID as session_id**. When you receive a report, you
know exactly which task it belongs to.

To follow up or give feedback on a specific task, use AgentTalk with
the same task_id as the session:

```
AgentTalk(agent_name="@writer", session_id="{task_id}", message="Good work. Next week focus on Kubernetes.")
```

This keeps each task's conversation in its own thread — @writer sees
the full history of that task, not mixed with other tasks.

You can also proactively check on any project:

```bash
python3 scripts/query-progress.py --project-id "..."
```

Review and **adjust**:

- Report received? → Acknowledge, give feedback, or assign next steps via AgentTalk.
- Tasks succeeding? → Keep going. Consider giving positive feedback via AgentTalk.
- Tasks failing? → Fix the prompt, change the agent, or adjust the plan.
- Dependencies blocked? → Reschedule or reorder.
- Priorities changed? → Update task priorities in the project.
- On track? → Report. Off track? → Tell the user why and propose changes.

The plan is never static. Every report or check-in is an opportunity to rebalance.
This is the hardest but most valuable part of being a PM.

### Phase 4: Report — Proactively Communicate

Tell the user what happened before they ask:

```
{name} — {X}% complete

{goal}: {completed}/{total} tasks

Recent: {task} — {summary}
Issues: {what needs attention}
Next: {plan for next period}
```

---

## Command Reference

| What | Command |
|------|---------|
| Create project | `graph_client.py create-project --name ... --description ...` |
| Create goal | `graph_client.py create-goal --project-id ... --title ... --weight N` |
| Create task | `graph_client.py create-task --goal-id ... --title ... --agent @x --prompt "..."` |
| Update task status | `graph_client.py update-task --task-id ... --status ...` |
| Assign recurring | `assign-task.py --agent @x --task "..." --cron "..."` |
| List assignments | `assign-task.py list` |
| Query progress | `query-progress.py --project-id ...` |
| Talk to agent | Use **AgentTalk** tool: `agent_name`, `session_id`, `message` |
