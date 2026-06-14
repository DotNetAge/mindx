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
# Generate IDs
PROJ_ID=$(mindx utils uuid)
GOAL_ID=$(mindx utils uuid)

# Create project node
mindx graph upsert-nodes --nodes '[{
  "id":"'"$PROJ_ID"'",
  "labels":["Project"],
  "properties":{"name":"Project Name","description":"...","status":"active","progress":0.0}
}]'

# Create goal node linked to project
mindx graph upsert-nodes --nodes '[{
  "id":"'"$GOAL_ID"'",
  "labels":["Goal"],
  "properties":{"title":"Goal Title","description":"...","weight":1.0,"status":"pending","progress":0.0}
}]'
mindx graph upsert-edges --edges '[{
  "from_node_id":"'"$PROJ_ID"'",
  "to_node_id":"'"$GOAL_ID"'",
  "type":"HAS_GOAL",
  "properties":{}
}]'

# Create task node under goal
TASK_ID=$(mindx utils uuid)
mindx graph upsert-nodes --nodes '[{
  "id":"'"$TASK_ID"'",
  "labels":["Task"],
  "properties":{"title":"Task Title","agent":"agent-name","cron_expr":"","prompt":"...","status":"pending","priority":"normal","progress":0.0}
}]'
mindx graph upsert-edges --edges '[{
  "from_node_id":"'"$GOAL_ID"'",
  "to_node_id":"'"$TASK_ID"'",
  "type":"CONTAINS",
  "properties":{}
}]'

echo "Project: $PROJ_ID  Goal: $GOAL_ID  Task: $TASK_ID"
```

> **task_id = session_id**: Save the task ID — use it as the session_id when setting up recurring work in Phase 2. This links the scheduled execution back to the graph task.

---

### Phase 2: Assign — Set Recurring Work

For each task, link it to the scheduler:

```bash
mindx schedule add --agent "<agent-name>" \
    --content "<prompt>" \
    --cron "0 0 9 * * 1" \
    --session-id "$TASK_ID" \
    --enabled true
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
# Query full project progress
mindx graph query --cypher "
  MATCH (p:Project {id: '$PROJ_ID'})-[:HAS_GOAL]->(g:Goal)
  OPTIONAL MATCH (g)-[:CONTAINS]->(t:Task)
  RETURN p.id, p.name, p.status, p.progress,
         g.id as goal_id, g.title as goal_title, g.status as goal_status,
         count(t) as total_tasks,
         count(CASE WHEN t.status = 'completed' THEN 1 END) as completed,
         count(CASE WHEN t.status = 'in_progress' THEN 1 END) as in_progress,
         count(CASE WHEN t.status = 'failed' THEN 1 END) as failed
"

# List tasks by status
mindx graph query --cypher "
  MATCH (g:Goal)-[:CONTAINS]->(t:Task)
  WHERE g.id = '$GOAL_ID'
  RETURN t.id, t.title, t.agent, t.status, t.priority, t.progress
  ORDER BY t.updated_at DESC
"
```

Update task status:

```bash
# Mark task completed
mindx graph exec --cypher "
  MATCH (t:Task {id: 'task-id-here'})
  SET t.status = 'completed', t.progress = 1.0, t.updated_at = timestamp()
  RETURN t.id, t.title, t.status
"

# Add dependency
mindx graph upsert-edges --edges '[{
  "from_node_id":"task-id-here",
  "to_node_id":"depends-on-task-id",
  "type":"DEPENDS_ON",
  "properties":{}
}]'
```

To query sessions assigned to a specific task or agent:

```bash
# Find sessions by task
mindx session get --session-id "$TASK_ID"
```

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

| What                       | Command                                                                                          |
| -------------------------- | ------------------------------------------------------------------------------------------------ |
| Generate ID                | `mindx utils uuid` or `mindx utils ulid`                                                         |
| Create node                | `mindx graph upsert-nodes --nodes '[...]'`                                                       |
| Create edge                | `mindx graph upsert-edges --edges '[...]'`                                                       |
| Execute graph write        | `mindx graph exec --cypher "MATCH (n {id:'x'}) SET n.status='completed'"`                        |
| Query graph (read)         | `mindx graph query --cypher "MATCH (p:Project) RETURN p.id, p.name, p.status"`                    |
| Get single node            | `mindx graph get-node --id "node-id"`                                                            |
| Find neighbors             | `mindx graph neighbors --id "node-id" --depth 2`                                                 |
| Schedule add               | `mindx schedule add --agent x --content "..." --cron "..." --session-id "..."`                    |
| Schedule list              | `mindx schedule list`                                                                            |
| Schedule delete            | `mindx schedule delete --id "..."`                                                               |
| Session create             | `mindx session create --agent x --project-dir /path`                                             |
| Session get                | `mindx session get --session-id "..."`                                                           |
| Session list               | `mindx session list`                                                                             |
| Semantic search            | `mindx query "search terms"`                                                                     |
| Talk to agent              | **AgentTalk** tool: `agent_name`, `session_id`, `message`                                        |
