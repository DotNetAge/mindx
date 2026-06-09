# GraphDB Data Model

## Node Types

| Node      | ID Prefix | Created By         |
| --------- | --------- | ------------------ |
| Project   | `proj-`   | `create-project`   |
| Goal      | `goal-`   | `create-goal`      |
| Task      | `task-`   | `create-task`      |
| Execution | `exec-`   | `record-execution` |

### Key Fields Per Node

**Project:** `id`, `name`, `description`, `status`, `progress`
**Goal:** `id`, `title`, `weight`, `status`, `progress`
**Task:** `id`, `title`, `agent`, `cron_expr`, `prompt`, `status`, `priority`, `scheduler_id`, `summary`, `session_id`

**Task statuses:**
`pending` → `scheduled` → `in_progress` → `completed`
`failed`, `blocked`, `skipped`, `cancelled`

## Relationships

| Name            | From → To        | Meaning                        |
| --------------- | ---------------- | ------------------------------ |
| `HAS_GOAL`      | Project → Goal   | Project contains goals         |
| `CONTAINS`      | Goal → Task      | Goal contains tasks            |
| `DEPENDS_ON`    | Task → Task      | Task depends on predecessor    |
| `HAS_EXECUTION` | Task → Execution | Execution history              |
| `HAS_SESSION`   | Task → Session   | Session for agent conversation |

## Essential Cypher Patterns

### Create everything (done by scripts, but good to understand the structure)

```
Project -[:HAS_GOAL]-> Goal -[:CONTAINS]-> Task -[:HAS_EXECUTION]-> Execution
                                                                  |
                                                            [:DEPENDS_ON]
                                                            (Task → Task)
```

### Get full project structure

```cypher
MATCH (p:Project {id: 'proj-xxx'})
OPTIONAL MATCH (p)-[:HAS_GOAL]->(g:Goal)-[:CONTAINS]->(t:Task)
OPTIONAL MATCH (t)-[:HAS_EXECUTION]->(e:Execution)
RETURN p, collect(DISTINCT g) as goals, collect(DISTINCT {task: t, executions: collect(e)}) as tasks
```

### Progress report for a project

```cypher
MATCH (p:Project {id: 'proj-xxx'})-[:HAS_GOAL]->(g:Goal)-[:CONTAINS]->(t:Task)
RETURN p.name, p.progress,
       g.title, g.weight, g.status,
       count(t) as total, sum(CASE WHEN t.status='completed' THEN 1 ELSE 0 END) as done
```

### Find blocked or failed tasks

```cypher
MATCH (t:Task) WHERE t.status IN ['blocked','failed'] RETURN t.id, t.title, t.agent, t.status
```

### Get task execution history

```cypher
MATCH (t:Task {id: 'task-xxx'})-[:HAS_EXECUTION]->(e:Execution)
RETURN e.status, e.result, e.executed_at ORDER BY e.executed_at DESC
```

## Rules

- All IDs use 8-char hex UUID suffix: `task-a1b2c3d4`
- `task_id` doubles as `session_id` for agent communication
- Always use `progress-report` script for structured queries — it handles the aggregation logic
