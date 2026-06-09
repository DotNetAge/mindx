# Scheduler Integration

Register recurring tasks via JSON-RPC 2.0 over WebSocket (port 1314).

**Methods:** `schedule.add`, `schedule.list`, `schedule.del`

---

## `schedule.add`

### Parameters

| Field         | Type   | Required | Description                                           |
| ------------- | ------ | -------- | ----------------------------------------------------- |
| `agent`       | string | yes      | Agent name, `@` prefix optional (stripped internally) |
| `content`     | string | yes      | Prompt to send to the agent                           |
| `cron_expr`   | string | yes      | 6-field cron expression                               |
| `session_id`  | string | no       | Link to a graph task ID (recommended)                 |
| `project_dir` | string | no       | Working directory                                     |

### Request

```json
{"jsonrpc":"2.0","id":"<uuid>","method":"schedule.add","params":{"agent":"writer","content":"Daily standup","cron_expr":"0 0 9 * * *","session_id":"task-xxx","project_dir":"/path"}}
```

### Response

```json
{"id":"a1b2c3","agent":"writer","content":"Daily standup","cron_expr":"0 0 9 * * *","session_id":"task-xxx","enabled":true}
```

### Error

```json
{"code":-32600,"message":"agent is required"}
```

---

## `schedule.list`

```json
{"jsonrpc":"2.0","id":"<uuid>","method":"schedule.list"}
```

Returns array of schedule entries (same fields as `schedule.add` response).

---

## `schedule.del`

| Field | Type   | Required | Description                                          |
| ----- | ------ | -------- | ---------------------------------------------------- |
| `id`  | string | yes      | 8-char hex ID from `schedule.add` or `schedule.list` |

```json
{"jsonrpc":"2.0","id":"<uuid>","method":"schedule.del","params":{"id":"a1b2c3"}}
```

Response: `{"status":"deleted","id":"a1b2c3"}`

---

## Helper Scripts

### assign-task.py (recommended for LLM)

```bash
# Create a recurring task
python3 scripts/assign-task.py assign --agent @writer --task "Write blog monthly" --cron "0 0 9 * * 1" --session-id "task-xxx" --project-dir /path

# List all tasks
python3 scripts/assign-task.py list
python3 scripts/assign-task.py list --json
```

### scheduler_client.py (when you need a Python client)

```python
from scheduler_client import MindXSchedulerClient, JobAddParams

with MindXSchedulerClient() as client:
    result = client.add_job(JobAddParams(agent="@writer", content="...", cron_expr="0 0 9 * * 1", session_id="task-xxx"))
    # result.task_id → "a1b2c3"
    # result.session_id → "task-xxx"
```

---

## Important Notes

- Agent `@` prefix is stripped before storage — `@writer` and `writer` are equivalent
- Cron uses **6-field** format (seconds, minute, hour, day, month, weekday)
- Pass the graph task ID as `--session-id` so execution reports are linked to the task
- Session is auto-generated as UUID v4 if omitted
