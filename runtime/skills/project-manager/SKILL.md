---
name: project-manager
description: >
  Turns vague ideas into structured projects with recurring tasks —
  decompose goals, assign recurring work to agents considering
  dependencies and priorities, track progress, adjust the plan
  continuously, and proactively report.
---

# When to Use

- User has a long-running or recurring need (social media ops, periodic reports, project management)
- Task is too large or ongoing for one-shot execution
- Need to decompose a vague goal into concrete recurring tasks

**Do NOT use** for tasks you can complete in one response.

---

## Workflow

### Phase 1: Plan — Understand & Decompose

Talk to the user. Extract measurable goals before writing anything.

| Ask                            | Why                 |
| ------------------------------ | ------------------- |
| "What does success look like?" | Define finish line  |
| "How will you measure it?"     | Quantifiable        |
| "Any deadlines?"               | Time boundaries     |
| "Does any part repeat?"        | Find recurring work |

Confirm plan. Decompose goal into tasks. Each task = **who does it, when, what**.

```bash
graph_client.py create-project --name "..." --description "..."
graph_client.py create-goal --project-id "..." --title "..." --weight N

# Save task_id — it becomes the session_id for all future communication
task_id=$(graph_client.py create-task --goal-id "..." --title "..." --agent "@x" --prompt "..." | python3 -c "import sys,json;print(json.load(sys.stdin)[0].get('t.id',''))")
```

---

### Phase 2: Assign — Set Recurring Work

For each task, link it to the scheduler. **task_id = session_id**.

```bash
assign-task.py assign --agent "@x" --task "..." --cron "0 0 9 * * 1" --session-id "$task_id"
```

**Critical:** Every task prompt must include a reporting instruction, or the agent works silently and you never hear back:

> When you finish, use AgentTalk to report the result to project-manager in session "{task_id}" with a summary.

This closes the loop — agent reports back autonomously after each execution, and you know which session to use for follow-up.

---

### Phase 3: Track — Review & Adjust

Agents report back via AgentTalk(session=task_id). Follow up with the same session:

```
AgentTalk(agent_name="@writer", session_id="{task_id}", message="Focus on Kubernetes next week.")
```

Proactively check progress:

```bash
query-progress.py --project-id "..."
```

Adjust as needed:
- Report received? → Acknowledge, give feedback, assign next steps via AgentTalk.
- Failing? → Fix prompt, change agent, adjust plan.
- Dependencies blocked? → Reschedule or reorder.
- Priorities changed? → Update task priorities.

---

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

| What             | Command                                                                                |
| ---------------- | -------------------------------------------------------------------------------------- |
| Create project   | `graph_client.py create-project --name ... --description ...`                          |
| Create goal      | `graph_client.py create-goal --project-id ... --title ... --weight N`                  |
| Create task      | `graph_client.py create-task --goal-id ... --title ... --agent @x --prompt "..."`      |
| Update task      | `graph_client.py update-task --task-id ... --status ... [--result "..."]`              |
| Assign recurring | `assign-task.py assign --agent @x --task "..." --cron "..." [--session-id "task-xxx"]` |
| List assignments | `assign-task.py list`                                                                  |
| Query progress   | `query-progress.py --project-id ...`                                                   |
| Talk to agent    | **AgentTalk** tool: `agent_name`, `session_id`, `message`                              |
