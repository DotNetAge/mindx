# GraphDB Data Model

This document defines the complete data model used by the Project Manager skill in the gograph graph database.

## Node Types

### 1. Project

The root node of a project, representing a complete project entity.

**Properties:**

| Property | Type | Required | Description | Example |
|----------|------|----------|-------------|---------|
| `id` | string | ✅ Auto-generated | Unique identifier, format `proj-{8-char uuid}` | `proj-a1b2c3d4` |
| `name` | string | ✅ | Project name | `"Community Operations"` |
| `description` | string | ✅ | Project description / goal | `"Increase community activity by 50%"` |
| `status` | string | ✅ | Status: `active`/`completed`/`paused`/`cancelled` | `"active"` |
| `progress` | float | ✅ | Overall progress (0.0–1.0) | `0.45` |
| `created_at` | string | ✅ | Creation time (ISO 8601) | `"2026-05-06T09:00:00Z"` |
| `updated_at` | string | ✅ | Last update time | `"2026-05-07T14:30:00Z"` |
| `metrics` | object | ❌ | Success KPI definition | `{"kpi": "activity+50%"}` |
| `timeline` | object | ❌ | Time range | `{"start": "...", "end": "..."}` |

---

### 2. Goal

A sub-goal under a project, representing the first level (L1) of a WBS decomposition.

**Properties:**

| Property | Type | Required | Description | Example |
|----------|------|----------|-------------|---------|
| `id` | string | ✅ | Unique identifier, format `goal-{8-char uuid}` | `goal-e5f6g7h8` |
| `title` | string | ✅ | Goal title | `"Content Creation"` |
| `description` | string | ✅ | Detailed goal description | `"Publish 3 high-quality articles per week"` |
| `weight` | float | ✅ | Weight percentage (0.0–1.0) | `0.4` (40%) |
| `status` | string | ✅ | Status: `pending`/`in_progress`/`completed`/`blocked` | `"in_progress"` |
| `progress` | float | ✅ | Progress (0.0–1.0) | `0.65` |
| `created_at` | string | ✅ | Creation time | `"2026-05-06T09:00:00Z"` |
| `updated_at` | string | ✅ | Last update time | `"2026-05-07T14:30:00Z"` |
| `metrics` | object | ❌ | Goal-specific KPIs | `{"target": "3 articles/week"}` |

---

### 3. Task

The smallest actionable work unit, assigned to a specific Agent and optionally scheduled for execution.

**Properties:**

| Property | Type | Required | Description | Example |
|----------|------|----------|-------------|---------|
| `id` | string | ✅ | Unique identifier, format `task-{8-char uuid}` | `task-i9j0k1l2` |
| `title` | string | ✅ | Task title | `"Write technical blog post"` |
| `agent` | string | ✅ | Assigned Agent | `"@writer"` |
| `cron_expr` | string | ❌ | Scheduler Cron expression | `"0 0 9 * * 1"` |
| `prompt` | string | ✅ | Execution instructions / prompt | `"Write an article about..."` |
| `status` | string | ✅ | Status enum | `"scheduled"` |
| `priority` | string | ✅ | Priority: `urgent`/`high`/`normal`/`low` | `"high"` |
| `progress` | float | ✅ | Progress (0.0–1.0) | `0.0` |
| `scheduler_id` | string | ❌ | Scheduler task ID | `"a1b2c3d4"` |
| `summary` | string | ❌ | Latest execution summary | `"Completed article draft..."` |
| `success_count` | int | ✅ | Number of successful executions | `5` |
| `failure_count` | int | ✅ | Number of failed executions | `1` |
| `created_at` | string | ✅ | Creation time | `"2026-05-06T09:00:00Z"` |
| `updated_at` | string | ✅ | Last update time | `"2026-05-07T09:00:00Z"` |

**Task.status enum values:**

| Value | Meaning | When Set |
|-------|---------|----------|
| `pending` | Pending | Newly created, waiting for scheduling or manual execution |
| `scheduled` | Scheduled | Registered with Scheduler, waiting to fire |
| `in_progress` | In Progress | Agent is currently working on it |
| `completed` | Completed | Successfully finished |
| `failed` | Failed | Execution error, can be retried |
| `blocked` | Blocked | A predecessor task has not completed |
| `skipped` | Skipped | Skipped by user or system |
| `cancelled` | Cancelled | No longer needed |

---

### 4. Execution

Records the result of each individual task execution.

**Properties:**

| Property | Type | Required | Description | Example |
|----------|------|----------|-------------|---------|
| `id` | string | ✅ | Unique identifier, format `exec-{8-char uuid}` | `exec-m3n4o5p6` |
| `status` | string | ✅ | Execution result: `success`/`failed`/`timeout` | `"success"` |
| `result` | string | ❌ | Execution result / output description | `"Completed first draft, ~2000 words..."` |
| `error` | string | ❌ | Error message (on failure) | `"Agent timed out"` |
| `duration_seconds` | int | ❌ | Execution duration in seconds | `120` |
| `executed_at` | string | ✅ | Execution timestamp | `"2026-05-07T09:05:00Z"` |

---

### 5. Resource (Optional)

Represents resources required by a task (tools, agents, etc.).

**Properties:**

| Property | Type | Required | Description | Example |
|----------|------|----------|-------------|---------|
| `id` | string | ✅ | Unique identifier | `"res-q7r8s9t0"` |
| `name` | string | ✅ | Display name | `"Professional Writing Agent"` |
| `type` | string | ✅ | Type: `agent`/`tool`/`file`/`api` | `"agent"` |
| `ref` | string | ✅ | Reference identifier | `"@writer"` or `"bash"` |
| `description` | string | ❌ | Why this resource is needed | `"For long-form article creation"` |

---

## Relationship Types

### 1. HAS_GOAL (Project → Goal)

A Project contains multiple Goals.

```cypher
(:Project)-[:HAS_GOAL {order: timestamp()}]->(:Goal)
```

**Properties:**
- `order`: Creation order (used for sorting)

**Cardinality:** 1:N

---

### 2. CONTAINS (Goal → Task)

A Goal contains multiple Tasks.

```cypher
(:Goal)-[:CONTAINS {order: integer}]->(:Task)
```

**Properties:**
- `order`: Task order within the goal

**Cardinality:** 1:N

---

### 3. DEPENDS_ON (Task → Task)

Dependency between tasks (a task depends on its predecessor).

```cypher
(:Task)-[:DEPENDS_ON]->(:Task)
```

**Properties:** None

**Cardinality:** M:N

**Constraint:** No circular dependencies allowed.

---

### 4. REQUIRES (Task → Resource) — Optional

Resources required by a task.

```cypher
(:Task)-[:REQUIRES {min_level: integer, is_required: boolean}]->(:Resource)
```

**Properties:**
- `min_level`: Minimum proficiency level (1–5)
- `is_required`: Whether the resource is mandatory

**Cardinality:** M:N

---

### 5. HAS_EXECUTION (Task → Execution)

Execution history for a task.

```cypher
(:Task)-[:HAS_EXECUTION]->(:Execution)
```

**Properties:** None

**Cardinality:** 1:N

---

## Complete Data Model Diagram

```
┌─────────────┐       ┌─────────────┐
│   Project   │       │    Goal     │
│─────────────│       │─────────────│
│ id          │──1:N──│ id          │
│ name        │ HAS_GOAL│ title       │
│ description │───────│ weight      │
│ status      │       │ progress    │
│ progress    │       │ status      │
│ metrics     │       │ metrics     │
│ timeline    │       └──────┬──────┘
└─────────────┘              │
                    1:N         │ 1:N
              ┌─────────────┴──────┐
              │       Task        │
              │───────────────────│
              │ id                │
              │ title             │
              │ agent             │
              │ cron_expr         │
              │ prompt            │
              │ status            │
              │ scheduler_id      │
              │ summary           │
              │ success_count     │
              │ failure_count     │
              └─────────┬─────────┘
                        │
              1:N         │ M:N
    ┌───────────────────┤ ┌──────────────┐
    │    Execution      │ │  Dependency  │
    │───────────────────│ │ (Task→Task)  │
    │ id               │ │              │
    │ status           │ │ Successor ───→ Predecessor
    │ result           │ │              │
    │ executed_at      │ └──────────────┘
    └───────────────────┘
```

---

## Cypher Query Examples

### Query Full Project Structure

```cypher
MATCH (p:Project {id: 'proj-xxx'})
OPTIONAL MATCH (p)-[:HAS_GOAL]->(g:Goal)
OPTIONAL MATCH (g)-[:CONTAINS]->(t:Task)
OPTIONAL MATCH (t)-[dep:DEPENDS_ON]->(pre:Task)
OPTIONAL MATCH (t)-[:HAS_EXECUTION]->(e:Execution)
RETURN p,
       collect(DISTINCT g) as goals,
       collect({
         task: t,
         depends_on: collect(pre.id),
         executions: collect(e)
       }) as tasks
```

### Query Goal Progress

```cypher
MATCH (g:Goal {id: 'goal-xxx'})-[:CONTAINS]->(t:Task)
WITH g,
     count(t) as total,
     count(CASE WHEN t.status = 'completed' THEN 1 END) as completed,
     count(CASE WHEN t.status = 'in_progress' THEN 1 END) as in_progress,
     sum(t.success_count) as successes,
     sum(t.failure_count) as failures
RETURN g.title, g.weight, g.status,
       round(coalesce(completed, 0) * 100.0 / total, 1) as completion_rate,
       total, completed, in_progress, successes, failures
```

### Find Blocked Tasks

```cypher
MATCH (t:Task {status: 'blocked'})-[dep:DEPENDS_ON]->(pre:Task)
WHERE pre.status <> 'completed'
RETURN t.id, t.title,
       pre.id as blocking_task_id,
       pre.title as blocking_title,
       pre.status as blocking_status
ORDER BY t.priority
```

### Find Overdue Tasks

```cypher
MATCH (t:Task)
WHERE t.deadline < datetime()
  AND t.status NOT IN ['completed', 'cancelled', 'skipped']
RETURN t.id, t.title, t.agent, t.deadline,
       t.status, t.failure_count
ORDER BY t.deadline ASC
LIMIT 20
```

---

## ID Naming Convention

All node IDs follow this convention for consistency:

| Node Type | Prefix | Format | Example |
|-----------|--------|--------|---------|
| Project | `proj-` | `proj-{8-char hex}` | `proj-a1b2c3d4` |
| Goal | `goal-` | `goal-{8-char hex}` | `goal-e5f6g7h8` |
| Task | `task-` | `task-{8-char hex}` | `task-i9j0k1l2` |
| Execution | `exec-` | `exec-{8-char hex}` | `exec-m3n4o5p6` |
| Resource | `res-` | `res-{8-char hex}` | `res-q7r8s9t0` |

**Note:** Uses the first 8 characters of a UUID (lowercase hexadecimal) to ensure uniqueness and brevity.
