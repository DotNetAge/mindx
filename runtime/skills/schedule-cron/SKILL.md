---
name: schedule-cron
description: >
  This skill should be used when the user asks to "add a scheduled task", "create a cron job",
  "schedule a message", "set up recurring work", "register a timer", "add a timer task",
  "schedule for agent", "send message to agent on schedule", "create timed message", or any
  request involving scheduling content delivery to agents via MindX Scheduler. Also use when
  the user wants to "list scheduled tasks", "delete a scheduled task", "view cron jobs",
  "check timers", or needs to manage existing scheduled jobs. Provides task registration,
  listing, and deletion via WebSocket communication with MindX Gateway.
allowed-tools: bash read_file write_file
---

## When to Use This Skill

- The user wants to schedule a message to be sent to a specific agent at a set time
- Someone asks to set up recurring work ("every day at 9 AM", "weekly on Monday")
- A task needs to be registered with the MindX Scheduler for automated execution
- The user wants to see all currently scheduled tasks
- Someone needs to delete or cancel an existing scheduled task
- The user mentions "cron", "timer", "recurring", or "scheduled" in the context of agent messaging
- An automated workflow needs to register tasks programmatically

## What This Skill Does

1. **Registers scheduled messages** — Adds new tasks with agent target, content, and cron schedule via WebSocket
2. **Lists active schedules** — Shows all currently registered tasks with their status and execution stats
3. **Deletes schedules** — Removes unwanted or obsolete scheduled tasks by ID
4. **Batch registers tasks** — Adds multiple tasks at once from a JSON file for project initialization

## How to Use

```
Schedule a message to @writer every Monday at 9 AM about writing a technical blog post
```

```
List all my scheduled tasks
```

```
Delete the scheduled task with ID a1b2c3d4
```

```
Set up these three tasks to run every week: writer posts on Monday, researcher investigates on Tuesday, analyst reports on Friday
```

```
Create a health check that runs every 30 minutes
```

## Instructions

### Step 1: Validate Input Parameters

Before registering any task, verify the user has provided all required information:

| Parameter       | Format        | Required | Example                                |
| --------------- | ------------- | -------- | -------------------------------------- |
| Agent           | `@name`       | Yes      | `@writer`, `@analyst`                  |
| Session ID      | `id` or `new` | No*      | `sess_abc123` (resume) / `new` (fresh) |
| Content         | Text          | Yes      | `"Write a weekly summary"`             |
| Cron expression | 6-field       | Yes      | `"0 0 9 * * 1"`                        |

\* When omitted or set to `"new"`, the **client (this skill)** generates a **UUID v4** as the session identifier (e.g., `a1b2c3d4-e5f6-7890-abcd-ef1234567890`). Session IDs are **client-managed** — the Daemon routes commands to the correct session but never creates session IDs. Use an existing session ID only when you need to resume a previously interrupted agent in its original conversation context.

#### Common Input Mistakes

**Missing agent prefix:**
> `writer`

Agents must always start with `@`. The correct format is `@writer`.

**Correct:**
> `@writer`

**Wrong cron format (5-field instead of 6-field):**
> `0 9 * * 1`

MindX Scheduler uses 6 fields (second minute hour day month weekday).

**Correct (6-field):**
> `0 0 9 * * 1`

### Step 2: Choose the Right Operation

Determine what the user wants to do and use the corresponding command:

| User Intent        | Command     | Script                        |
| ------------------ | ----------- | ----------------------------- |
| Add one task       | `add-job`   | `scripts/scheduler_client.py` |
| Add multiple tasks | `batch-add` | `scripts/scheduler_client.py` |
| List all tasks     | `list-jobs` | `scripts/scheduler_client.py` |
| Delete a task      | `del-job`   | `scripts/scheduler_client.py` |
| Test connection    | `test-conn` | `scripts/scheduler_client.py` |

### Step 3: Execute the Operation

**Prerequisite:** Ensure the `websocket-client` library is installed:

```bash
pip install websocket-client
```

#### Adding a Single Task

Run the script with the add-job command:

```bash
python3 scripts/scheduler_client.py add-job \
    --agent "@writer" \
    --session-id "new" \
    --content "Every Monday: Write a technical blog post about AI engineering practices" \
    --cron "0 0 9 * * 1"
```

The `--session-id` parameter controls agent conversation context:
- `"new"` or omitted — **Client generates a UUID v4** (e.g., `a1b2c3d4-e5f6-...`) as a fresh session identifier for each scheduled trigger. The Daemon receives this ID and routes the command to a new session context.
- `<existing_id>` — Resumes an interrupted agent in its original conversation context using a previously recorded session ID.

The response includes both the task **ID** and the **session_id** (when using `"new"`). Save both if you plan to manage or resume sessions later.

#### Adding Multiple Tasks (Batch)

Create a JSON file with the task list:

```json
[
    {
        "agent": "@writer",
        "session_id": "new",
        "content": "Every Monday: Write a technical blog post",
        "cron_expr": "0 0 9 * * 1"
    },
    {
        "agent": "@analyst",
        "session_id": "sess_weekly_report",
        "content": "Every Friday: Generate weekly analytics report",
        "cron_expr": "0 0 16 * * 5"
    }
]
```

Submit to the Scheduler:

```bash
python3 scripts/scheduler_client.py batch-add --file tasks.json
```

This processes all tasks in sequence and reports success/failure for each.

#### Listing All Tasks

```bash
python3 scripts/scheduler_client.py list-jobs
```

For machine-readable output (useful for programmatic processing):

```bash
python3 scripts/scheduler_client.py list-jobs --json
```

#### Deleting a Task

```bash
python3 scripts/scheduler_client.py del-job --id a1b2c3d4
```

The task ID comes from the `list-jobs` output or from the `add-job` response.

#### Testing Connection

Before registering tasks, verify the Gateway is reachable:

```bash
python3 scripts/scheduler_client.py test-conn
```

### Step 4: Confirm the Result

After any operation, report the outcome to the user. The MindX Gateway returns responses in Chinese — translate and summarize them for the user when appropriate.

**On success (add-job) — raw server response:**
```
✅ 定时消息已创建:
  ID: a1b2c3d4
  目标: @writer
  Session: (auto)
  内容: Every Monday: Write a technical blog post about AI engineering practices
  调度: 0 0 9 * * 1
```

Report to the user (summarized):
```
✅ Scheduled message created:
   ID: a1b2c3d4
   Agent: @writer
   Session: new (fresh)
   Cron: 0 0 9 * * 1
   Next run: Every Monday at 9:00 AM
```

**On success (job-list) — raw server response:**
```
📋 定时消息任务列表
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
ID         目标Agent  Session  发送内容                             调度规则             状态   成功/失败
a1b2c3     @writer    (auto)   每周一: 撰写技术博客...                0 0 9 * * 1        ✅ 启用  10/0
d4e5f6     @analyst   sess_abc 每周五: 生成分析报...                  0 0 16 * * 5       ✅ 启用  5/1
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
共 2 个任务
```

**On success (job-del) — raw server response:**
```
🗑️ 定时消息已删除:
  ID: a1b2c3d4
  目标: @writer
  内容: Every Monday: Write a technical blog post
```

**On error:**
```json
{
    "cmd": "CMD",
    "name": "job-add",
    "error": "缺少目标智能体: 请使用 @<agent-name> 格式指定"
}
```

Always translate error messages from Chinese to English when reporting to the user.

---

## Examples

### Example 1: Simple Task Registration

**User:** "Schedule @writer to write a daily standup reminder every morning at 9"

**Process:**
1. Validated: agent = `@writer`, session_id = `"new"` (fresh daily session), cron = `0 0 9 * * *` (daily at 9 AM)
2. Ran: `scheduler_client.py add-job --agent "@writer" --session-id "new" --content "Daily standup reminder" --cron "0 0 9 * * *"`
3. Received task ID: `a1b2c3d4`

**Output:**
```
✅ Scheduled message added:
   ID: a1b2c3d4
   Agent: @writer
   Session: new (fresh)
   Content: Daily standup reminder
   Cron: 0 0 9 * * *
   Status: enabled
```

---

### Example 2: Edge Case — Vague Content

**User:** "Schedule @writer to write something"

**Process:**
1. Detected vague content: "write something" — no topic, no audience, no quality bar
2. Asked clarifying question: "What should the writer produce? Please specify the topic, audience, and expected length."
3. User provided: "A 1500-word article about Kubernetes for senior developers"
4. Registered with specific content, session_id = `"new"`

**Key insight:** Vague content leads to poor output quality. Always push for specificity when the content is just a placeholder phrase like "something", "write an article", or "do the task".

---

### Example 3: Batch Registration for Project Initialization

**User:** "Set up the full weekly schedule for our content team: writer on Monday, researcher on Tuesday, analyst on Friday"

**Process:**
1. Created JSON with 3 tasks:
   - `@writer` on Monday 9 AM (`session_id: "new"`)
   - `@researcher` on Tuesday 10 AM (`session_id: "sess_research"`)
   - `@analyst` on Friday 4 PM (`session_id: "sess_weekly_report"`)
2. Ran: `scheduler_client.py batch-add --file weekly-schedule.json`
3. All 3 tasks registered successfully

**Output:**
```
📋 Preparing to batch register 3 tasks...
============================================================

[1/3] Registering task: @writer
  ✅ Success: a1b2c3d4

[2/3] Registering task: @researcher
  ✅ Success: e5f6g7h8

[3/3] Registering task: @analyst
  ✅ Success: i9j0k1l2

============================================================

📊 Batch operation complete:
   ✅ Success: 3/3
   ❌ Failed: 0/3
```

---

## Cron Expression Reference

MindX Scheduler uses a 6-field cron format:

```
┌───────────── Second (0-59)
│ ┌──────────── Minute (0-59)
│ │ ┌────────── Hour (0-23)
│ │ │ ┌──────── Day of month (1-31)
│ │ │ │ ┌────── Month (1-12)
│ │ │ │ │ ┌──── Day of week (0-6, 0=Sunday)
* * * * * *
```

| Expression       | Description              | Use Case                  |
| ---------------- | ------------------------ | ------------------------- |
| `0 0 9 * * *`    | Daily at 9:00 AM         | Daily tasks               |
| `0 0 9 * * 1`    | Every Monday at 9:00 AM  | Weekly reports            |
| `0 0 16 * * 5`   | Every Friday at 4:00 PM  | Weekend summaries         |
| `0 0 0 1 * *`    | 1st of month at midnight | Monthly tasks             |
| `*/30 * * * * *` | Every 30 seconds         | High-frequency monitoring |

---

## Pro Tips

1. **Always test the connection first** — Use `test-conn` before registering tasks. It's faster to discover Gateway issues upfront than to fail during batch registration.

2. **Use the Demon protocol correctly: `@agent <session_id> <content>`** — The Scheduler sends commands to SubAgents using this three-part format. When `session_id` is `"new"` or omitted, **the client generates a UUID v4** (e.g., `550e8400-e29b-41d4-a716-446655440000`) as the new session identifier before sending the command to the Daemon. Session IDs are **client-managed** — never leave this field blank; always provide either a client-generated UUID (for new sessions) or an existing session ID (to resume).

3. **Use batch-add for multiple tasks** — Registering tasks one by one is slow and harder to track. The batch command processes everything in one run and gives a summary report.

4. **Content specificity matters** — The Scheduler sends the content directly to the agent as a prompt. Vague content ("write something") produces vague output. Include topic, audience, length, and format in the content string.

5. **Save task IDs for later** — The Scheduler returns a unique ID for each registered task. Store this if you plan to delete or reference the task later.

6. **6-field cron is required** — Traditional 5-field cron expressions won't work. Always include seconds as the first field.

7. **Session strategy depends on use case** — For independent recurring tasks (daily reminders), `"new"` is correct — the client generates a fresh **UUID v4** each time as a clean context. For continuing long-running work (weekly reports building on prior data), record the first generated UUID and reuse it in subsequent registrations so the agent retains memory across triggers.

---

## Common Schedule Requests

```
Schedule @writer to post every Monday at 9 AM
```

```
Create a daily reminder at 10:30 for the standup meeting
```

```
Show me all my scheduled tasks
```

```
Delete task abc12345
```

```
Set up a weekly analytics report every Friday evening
```

```
Register these 5 tasks from this schedule file
```

---

## Available Scripts

### scripts/scheduler_client.py

WebSocket client for MindX Scheduler. Communicates with the Gateway via JSON-RPC 2.0 over WebSocket to manage scheduled messages.

**Demon Communication Protocol:** All commands sent to SubAgents use `@agent <session_id> <content>` format.

```bash
# Test connection (verifies Daemon is running)
python3 scripts/scheduler_client.py test-conn

# Add a single task
python3 scripts/scheduler_client.py add-job --agent "@writer" --session-id "new" --content "..." --cron "..."

# Add a task resuming an existing session
python3 scripts/scheduler_client.py add-job --agent "@writer" --session-id "sess_abc123" --content "..." --cron "..."

# Batch add from JSON file
python3 scripts/scheduler_client.py batch-add --file tasks.json

# List all tasks
python3 scripts/scheduler_client.py list-jobs

# Delete a task
python3 scripts/scheduler_client.py del-job --id a1b2c3d4
```

Global options: `--host` (default: localhost), `--port` (default: 8081)

---

## Quality Checklist

Self-verify after completing any operation:

### Add Task ✅
- [ ] Is the agent name prefixed with @?
- [ ] Is the session_id set correctly? (`"new"` for fresh, stable ID for continuing work)
- [ ] Is the cron expression in 6-field format?
- [ ] Is the content specific enough to produce quality output?
- [ ] Did you save the returned task ID?

### List Tasks ✅
- [ ] Did you show the task ID alongside each entry?
- [ ] Is the Session column visible (shows whether task uses fresh or resumed context)?
- [ ] Are success/failure counts visible?
- [ ] Is the cron expression readable?

### Delete Task ✅
- [ ] Did you confirm the correct task ID with the user before deleting?
- [ ] Did you report the deletion outcome?

---

## References

- **`references/scheduler-protocol.md`** — WebSocket CMD protocol details, message format, error handling
