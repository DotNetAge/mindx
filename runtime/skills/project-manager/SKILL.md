---
name: project-manager
description: >
  Turns vague ideas into structured projects with recurring tasks —
  decompose goals, assign recurring work to agents considering
  dependencies and priorities, track progress, adjust the plan
  continuously, and proactively report.
metadata:
  name_zh: 项目管理
  name_zh-tw: 專案管理
  description_zh: 将模糊想法转化为结构化项目并设置重复任务，分解目标、分配工作、跟踪进度和主动报告
  description_zh-tw: 將模糊想法轉化為結構化專案並設定重複任務，分解目標、分配工作、追蹤進度和主動報告
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
python3 scripts/graph_client.py create-project --name "..." --description "..."

# Save the returned project-id for next steps
python3 scripts/graph_client.py create-goal --project-id "proj-xxx" --title "..." --weight N

# create-task outputs the task_id — use it as session_id for agent communication
task_id=$(python3 scripts/graph_client.py create-task --goal-id "goal-xxx" --title "..." --agent "x" --prompt "..." | python3 -c "import sys,json;print(json.load(sys.stdin)[0].get('t.id',''))")
```

---

### Phase 2: Assign — Set Recurring Work

For each task, link it to the scheduler. **task_id = session_id**.

```bash
python3 scripts/scheduler_client.py add-job --agent "x" --content "..." --cron "0 0 9 * * 1" --session-id "$task_id"
```

**Critical:** Every task prompt must include a reporting instruction, or the agent works silently and you never hear back:

> When you finish, use AgentTalk to report the result to project-manager in session "{task_id}" with a summary.

This closes the loop — agent reports back autonomously after each execution, and you know which session to use for follow-up.

---

### Phase 3: Track — Review & Adjust

Agents report back via AgentTalk(session=task_id). Follow up with the same session:

```
AgentTalk(agent_name="writer", session_id="{task_id}", message="Focus on Kubernetes next week.")
```

Proactively check progress:

```bash
python3 scripts/query-progress.py --project-id "..."
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

| What             | Command                                                                                    |
| ---------------- | ------------------------------------------------------------------------------------------ |
| Create project   | `python3 scripts/graph_client.py create-project --name ... --description ...`              |
| Query project    | `python3 scripts/graph_client.py query-project --project-id ...`                           |
| List projects    | `python3 scripts/graph_client.py list-projects`                                            |
| Update project   | `python3 scripts/graph_client.py update-project --project-id ... --status ...`             |
| Create goal      | `python3 scripts/graph_client.py create-goal --project-id ... --title ... --weight N`      |
| Query goals      | `python3 scripts/graph_client.py query-goals --project-id ...`                             |
| Update goal      | `python3 scripts/graph_client.py update-goal --goal-id ... --status ...`                   |
| Create task      | `python3 scripts/graph_client.py create-task --goal-id ... --title ... --agent x --prompt "..."` |
| Update task      | `python3 scripts/graph_client.py update-task --task-id ... --status ... [--result "..."]`  |
| Record execution | `python3 scripts/graph_client.py record-execution --task-id ... --status ... --result "..." --duration N` |
| Query tasks      | `python3 scripts/graph_client.py query-tasks --goal-id ... [--status ...]`                 |
| Get task         | `python3 scripts/graph_client.py get-task --task-id ...`                                   |
| Add dependency   | `python3 scripts/graph_client.py add-dependency --task-id ... --depends-on ...`            |
| Remove dependency| `python3 scripts/graph_client.py remove-dependency --task-id ... --depends-on ...`         |
| Register session | `python3 scripts/graph_client.py register-session --task-id ... --agent x`                 |
| Get session      | `python3 scripts/graph_client.py get-session --session-id ...`                             |
| Query sessions   | `python3 scripts/graph_client.py query-sessions [--status ...] [--stale-threshold ...]`    |
| Progress report  | `python3 scripts/graph_client.py progress-report --project-id ...`                         |
| Schedule add     | `python3 scripts/scheduler_client.py add-job --agent x --content "..." --cron "..." [--session-id ...]` |
| Schedule list    | `python3 scripts/scheduler_client.py list-jobs`                                            |
| Schedule delete  | `python3 scripts/scheduler_client.py del-job --id ...`                                     |
| Assign task      | `python3 scripts/assign-task.py assign ...`                                                |
| Talk to agent    | **AgentTalk** tool: `agent_name`, `session_id`, `message`                                  |
