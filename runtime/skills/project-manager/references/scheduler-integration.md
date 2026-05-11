# MindX Scheduler Integration Guide

This document explains how to register tasks with MindX Scheduler using the WebSocket CMD protocol.

## Core Concepts

**Key Understanding:**

- `/job-add`, `/job-list`, `/job-del` are **NOT CLI commands**
- They are **MindX WebSocket CMD protocol instructions**
- Sent to the server via the Gateway's WebSocket channel
- External code cannot call the Scheduler API directly, but CAN operate on JSON files

## Directory Architecture (4-Layer)

The Scheduler stores data in the **Home Directory (Layer 1)**:

```
$HOME/.mindx/  (or $MINDX_HOME)
└── data/
    └── schedules/     ← Scheduler JSON files stored here
```

**Path resolution:**
- `SCHEDULES_DIR` = `$HOME/.mindx/data/schedules/`
- Resolved internally via `Settings.SchedulesDir()`
- Auto-created on first task registration

---

## Protocol Format

### Message Structure

```
CMD|<command_name>|<arguments>|||
```

**Components:**

| Part | Description | Example |
|------|-------------|---------|
| `CMD` | Fixed protocol prefix | `CMD` |
| `<command_name>` | Command name | `job-add`, `job-list`, `job-del` |
| `<arguments>` | Argument string | `@agent content expr="cron"` |
| `\|\|\|` | Fixed terminator | `\|\|\|` |

---

## /job-add — Create Scheduled Message

### Syntax

```
CMD|job-add|@<agent-name> <content> expr="<cron-expression>"|||
```

### Parameters

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `@<agent-name>` | ✅ | Target agent (must start with @) | `@assistant` |
| `<content>` | ✅ | Message content to send | `Daily standup reminder` |
| `expr="<cron>"` | ✅ | Cron expression (6-field, includes seconds) | `expr="0 0 9 * * *"` |

### Examples

#### Example 1: Send a message every day at 9:00 AM

```
CMD|job-add|@assistant Daily standup reminder expr="0 0 9 * * *"|||
```

#### Example 2: Run data analysis every Monday at 10:00 AM

```
CMD|job-add|@analyst Please analyze this week's user data trends expr="0 0 10 * * 1"|||
```

#### Example 3: Health check every 30 minutes

```
CMD|job-add|@monitor Run system health check expr="*/30 * * * * *"|||
```

#### Example 4: Long-form content

```
CMD|job-add|@writer Time for the weekly summary. Please write a report covering:
1. Completed feature modules
2. Technical challenges and solutions
3. Next week's plan expr="0 0 17 * * 5"|||
```

### Success Response

```json
{
    "cmd": "CMD",
    "name": "job-add",
    "data": "✅ Scheduled message created:\n  ID: a1b2c3d4\n  Target: @assistant\n  Content: Daily standup reminder\n  Schedule: 0 0 9 * * *"
}
```

### Error Response

```json
{
    "cmd": "CMD",
    "name": "job-add",
    "error": "Missing target agent: use @<agent-name> format to specify one"
}
```

---

## /job-list — Query Task List

### Syntax

```
CMD|job-list|||
```

**Note:** No parameters.

### Success Response (Table Type)

```json
{
    "cmd": "CMD",
    "name": "job-list",
    "response_type": "table",
    "data": {
        "title": "Scheduled Message Tasks",
        "headers": ["ID", "Target Agent", "Content", "Schedule", "Status", "Success/Fail"],
        "rows": [
            ["a1b2c3", "@assistant", "Daily standup reminder", "0 0 9 * * *", "✅ Enabled", "10/0"],
            ["d4e5f6", "@analyst", "Data analysis", "0 0 10 * * 1", "✅ Enabled", "5/1"]
        ]
    }
}
```

---

## /job-del — Delete Scheduled Message

### Syntax

```
CMD|job-del|id=<task-id>|||
```

### Parameters

| Parameter | Required | Description | How to Obtain |
|-----------|----------|-------------|---------------|
| `id=<task-id>` | ✅ | Task ID (8-character string) | From `/job-list` output |

### Example

```
CMD|job-del|id=a1b2c3d4|||
```

### Success Response

```json
{
    "cmd": "CMD",
    "name": "job-del",
    "data": "🗑️ Scheduled message deleted:\n  ID: a1b2c3d4\n  Target: @assistant\n  Content: Daily standup reminder"
}
```

---

## Usage Strategies in a Skill

Since a Skill runs inside an Agent, it cannot send WebSocket messages directly. Use one of the following strategies:

### Strategy A: Output Instructions for MasterAgent to Execute

**Best for:** Project initialization phase, when creating multiple scheduled tasks at once.

```markdown
## 📋 Scheduler Commands to Execute

Run the following commands to register the project's scheduled tasks:

### Content Creation Goal (@writer)
1. `/job-add @writer "Every Monday: Write a technical blog post" expr="0 0 9 * * 1"`
2. `/job-add @writer "Every Wednesday: Write community engagement copy" expr="0 0 9 * * 3"`

### Data Analysis Goal (@analyst)
3. `/job-add @analyst "Every Friday: Analyze this week's data and generate a report" expr="0 0 16 * * 5"`

Total: N tasks pending registration.
```

**Pros:**
- User/MasterAgent can review before executing
- Parameters can be adjusted or tasks skipped
- Aligns with the "communicate first" design philosophy

**Cons:**
- Requires manual execution
- Risk of omission or wrong order

---

### Strategy B: Write to a Pending Queue File

**Best for:** High-automation scenarios.

**Implementation:** Create a helper script in `scripts/`:

```bash
#!/bin/bash
# scripts/schedule-tasks.sh - Write tasks to a pending queue

QUEUE_FILE="runtime/schedules/pending-jobs.txt"

add_job() {
    local agent="$1"
    local content="$2"
    local cron_expr="$3"

    echo "CMD|job-add|@${agent} ${content} expr=\"${cron_expr}\"|||" >> "$QUEUE_FILE"
    echo "✅ Added to queue: @${agent} ${content}"
}

# Usage:
# add_job "@writer" "Write article" "0 0 9 * * 1"
# add_job "@analyst" "Data report" "0 0 16 * * 5"
```

**Pros:**
- Highly automated
- Batch processing support
- Persistent record

**Cons:**
- Requires a consumer mechanism to read and execute the queue file
- No immediate execution feedback

---

### Strategy C: Direct JSON File Manipulation (Advanced)

**Best for:** Third-party system integrators.

**How it works:** Scheduler hot-reloads JSON files in the `<schedules-dir>/` directory.

**Create a JSON file directly:**

```bash
TASK_ID=$(uuidgen | cut -c1-8)

cat > "${SCHEDULES_DIR}/${TASK_ID}.json" << EOF
{
  "id": "${TASK_ID}",
  "agent": "writer",
  "content": "Every Monday: Write a technical blog post",
  "cron_expr": "0 0 9 * * 1",
  "enabled": true,
  "created_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "updated_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "success_count": 0,
  "failure_count": 0
}
EOF

echo "✅ Task created: ${TASK_ID} (auto-loaded within 5 seconds)"
```

**Pros:**
- Most direct approach
- No WebSocket layer needed
- Scheduler auto-detects and loads (≤5 second delay)

**Cons:**
- Bypasses the normal command layer
- Requires knowledge of the schedules directory path
- No parameter validation or error feedback

---

## Recommended Strategy Selection

| Scenario | Recommended Strategy | Reason |
|----------|---------------------|--------|
| Project initialization (interactive) | **A** | User confirms before scheduling |
| Batch automated tasks | **B** | Minimizes manual operations |
| Third-party system integration | **C** | Most direct integration method |
| Agent-internal triggering | **B or C** | Depends on permissions and needs |

---

## Coordination with GraphDB

### Data Flow

```
Phase 3: Task Scheduling
        │
        ├── 1. Decompose tasks → Record in GraphDB (Task node, status=pending)
        │
        ├── 2. Register with Scheduler → Get scheduler_id
        │       │
        │       └── Update GraphDB: task.scheduler_id = "{id}", status = "scheduled"
        │
Phase 4: Progress Tracking
        │
        └── 3. Periodically query GraphDB + Scheduler
                │
                ├── GraphDB: task.status, task.success_count, task.summary
                │
                └── Scheduler: next execution time, enabled status
```

### State Synchronization

After receiving execution results from the Scheduler, update GraphDB promptly:

```bash
# Task completed successfully
./scripts/gograph.sh update-task \
    --task-id "task-xxx" \
    --status completed \
    --result "Completed the weekly data analysis report..."

# Task failed
./scripts/gograph.sh update-task \
    --task-id "task-xxx" \
    --status failed \
    --result "Agent timed out"

# Record detailed execution log
./scripts/gograph.sh record-execution \
    --task-id "task-xxx" \
    --status success \
    --result "Output: report-weekly-2026-W19.md" \
    --duration 180
```

---

## Cron Expression Quick Reference

### 6-Field Format (Second-Level Precision)

```
┌───────────── Second (0-59)
│ ┌──────────── Minute (0-59)
│ │ ┌────────── Hour (0-23)
│ │ │ ┌──────── Day of month (1-31)
│ │ │ │ ┌────── Month (1-12)
│ │ │ │ │ ┌──── Day of week (0-6, 0=Sunday)
* * * * * *
```

### Common Expressions

| Expression | Description | Use Case |
|------------|-------------|----------|
| `* * * * * *` | Every minute | High-frequency monitoring |
| `*/5 * * * * *` | Every 5 minutes | Periodic checks |
| `0 * * * * *` | Every hour on the hour | Hourly tasks |
| `0 0 * * * *` | Daily at midnight | End-of-day summary |
| `0 0 9 * * *` | Daily at 9:00 AM | Daily tasks |
| `0 0 9 * * 1` | Every Monday at 9:00 AM | Weekly report tasks |
| `0 0 18 * * 5` | Every Friday at 6:00 PM | Weekend prep |
| `0 0 0 1 * *` | 1st of every month at midnight | Monthly tasks |
| `0 0 9 1,15 * *` | 1st and 15th at 9:00 AM | Biweekly tasks |
| `0 0 9 * * 1-5` | Weekdays at 9:00 AM | Workday tasks |

### Notes

1. **Use 6-field format** (includes seconds), not the traditional 5-field
2. **Online validator**: https://cronitor.io/cron-expression-debugger (select 6-field)
3. **Timezone**: Cron expressions use the server's local time
4. **Avoid excessive frequency**: Minimum interval recommended ≥ 1 minute

---

## Troubleshooting

### Issue 1: Task Not Firing on Schedule

**Steps:**

1. Verify the task exists in Scheduler:
   ```bash
   /job-list
   ```

2. Check that `enabled` is `true`

3. Validate the Cron expression format

4. Review Scheduler logs

### Issue 2: Agent Not Receiving Messages

**Possible causes:**

- Agent name typo (missing `@` prefix)
- Agent not registered in the Registry
- Scheduler service not running properly

**Solutions:**

1. Confirm Agent exists: check the `runtime/agents/` directory
2. Verify Scheduler status
3. Manually test Agent connectivity

### Issue 3: JSON File Changes Not Taking Effect

**Cause:** Hot-reload has up to 5 seconds of delay.

**Solution:**

- Wait 5–10 seconds
- Or restart the Scheduler service (not recommended)
