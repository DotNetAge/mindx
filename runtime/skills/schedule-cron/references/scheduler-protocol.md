# Scheduler JSON-RPC Protocol

All methods use JSON-RPC 2.0 over WebSocket to `ws://localhost:1314/ws`.

## schedule.add — Register a recurring task

**Request:**
```json
{"jsonrpc":"2.0","id":"<uuid>","method":"schedule.add","params":{"agent":"writer","content":"...","cron_expr":"0 0 9 * * 1","session_id":"...","project_dir":"..."}}
```

**Response:** Full ScheduleEntry object (id, agent, content, cron_expr, enabled, success_count, failure_count, created_at, etc.)

## schedule.list — List all tasks

**Request:**
```json
{"jsonrpc":"2.0","id":"<uuid>","method":"schedule.list"}
```

**Response:** Array of ScheduleEntry objects.

## schedule.del — Delete a task

**Request:**
```json
{"jsonrpc":"2.0","id":"<uuid>","method":"schedule.del","params":{"id":"a1b2c3d4"}}
```

**Response:** `{"status":"deleted","id":"a1b2c3d4"}`

## Always use the script

Call `python3 scripts/scheduler_client.py` — never construct raw JSON-RPC manually.
