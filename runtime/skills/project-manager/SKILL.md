---
name: project-manager
description: >
  This skill should be used when the user asks to "create a project", "manage project",
  "project management", "make a plan", "plan something", "break down goals", "decompose goals",
  "schedule tasks", "arrange tasks", "assign agents", "check progress", "view project status",
  "generate report", "project report", "gantt chart", or any request involving multi-step project
  planning, long-running goal-oriented work, periodic task execution, or cross-agent coordination.
  Also use when the user mentions wanting to track progress over time, needs recurring tasks
  (daily/weekly/monthly), or asks for a work breakdown structure (WBS). If the user describes
  a goal and needs help turning it into an actionable, scheduled, and tracked plan, use this skill.
  Provides intelligent project management including goal decomposition, task scheduling with
  MindX Scheduler, multi-Agent coordination, progress tracking via gograph, and automated
  report generation.
allowed-tools: bash read_file write_file
---

## When to Use This Skill

- The user proposes a new project or initiative with a vague idea ("I want to build a community", "help me plan something")
- Someone asks to break down a goal into tasks or "make a plan" for a complex effort
- A goal feels too vague or large to act on directly and needs decomposition
- The user wants to set up recurring or scheduled work (daily, weekly, monthly tasks)
- Someone asks to check project progress, view status, or generate a report
- The user needs to coordinate work across multiple agents with dependencies
- A project needs a work breakdown structure (WBS) with clear deliverables
- The user mentions tracking progress over time or needs a Gantt chart
- Someone wants to assign tasks to specific agents and monitor execution
- The user describes a deadline-driven effort with multiple moving parts

## What This Skill Does

1. **Transforms vague ideas** into structured, quantifiable project definitions with success metrics
2. **Decomposes goals** using MECE principles into actionable tasks with clear agent assignments
3. **Schedules recurring work** via MindX Scheduler using Cron-based timing rules
4. **Tracks execution** through gograph graph database with full audit trail and execution history
5. **Generates progress reports** with Markdown summaries, actionable recommendations, and Mermaid Gantt charts

## How to Use

```
I want to build a community around my open-source project
```

```
Can you help me plan a 3-month product launch? We need to hit these milestones...
```

```
Break this goal down into manageable tasks and schedule them for the right agents
```

```
How is my community growth project going? Show me a status report
```

```
Set up a weekly blog post and a Friday analytics report that run automatically
```

## Workflow Overview

```
User describes a vague idea
        │
        ▼
  Phase 0: Check Daemon → ensure MindX service is running (system service)
        │
        ▼
  Phase 1: Clarify intent → structured project definition
        │
        ▼
  Phase 2: Decompose goals → WBS task tree → record in GraphDB
        │
        ▼
  Phase 3: Schedule tasks → assign Agents → register via WebSocket Scheduler
        │
        ▼
  Phase 4: Track execution → record results in GraphDB → monitor progress
        │
        ▼
  Phase 5: Generate reports → Markdown brief + Mermaid Gantt chart
```

---

## Instructions

### Phase 0: Prerequisites — Daemon + GraphDB (cypherdb)

**Trigger:** User triggers any operation of this skill (this is a prerequisite for all subsequent Phases).

#### Why Phase 0 Exists

This skill depends on two infrastructure components:

```
project-manager skill
    │
    ├── Phase 1-2 (Planning) ──► graph_client.py ──► cypherdb ──► GraphDB (.db)
    │                              (pip install cypherdb)           (local file)
    │
    ├── Phase 3 (Scheduling)  ──► scheduler_client.py ──► WebSocket ──► MindX Daemon
    │                                                                 │
    ├── Phase 4 (Tracking)   ◄──────────────────────────────────────────┘
    │                          (Daemon executes scheduled tasks and writes back results)
    │
    └── Phase 5 (Reporting) ◄──── graph_client.py ◄──── cypherdb ◄──┘
```

**Without these two dependencies, Phases 1-5 cannot work:**

| Dependency       | Purpose                                                                           | Installation                   | Verification                    |
| ---------------- | --------------------------------------------------------------------------------- | ------------------------------ | ------------------------------- |
| **MindX Daemon** | WebSocket Gateway + Cron Scheduler + Agent session routing                        | `mindx start` / system service | `scheduler_client.py test-conn` |
| **cypherdb**     | Embedded graph database, stores project/goal/task/session nodes and relationships | `pip install cypherdb`         | `python3 -c "import cypherdb"`  |

#### Step 0.1 - Check Dependencies

**Always run both checks first before any operation:**

```bash
# 1. Check cypherdb (GraphDB engine)
python3 -c "import cypherdb; print('✅ cypherdb ready')"

# 2. Check Daemon (WebSocket service)
python3 scripts/scheduler_client.py test-conn
```

| Check    | Result               | Meaning                                  | Action                             |
| -------- | -------------------- | ---------------------------------------- | ---------------------------------- |
| cypherdb | ✅ ready              | GraphDB engine available                 | Continue to Daemon check           |
| cypherdb | ❌ ImportError        | cypherdb not installed                   | `pip install cypherdb` (Step 0.1b) |
| Daemon   | ✅ Connection OK      | Daemon running and accepting connections | Proceed to Phase 1                 |
| Daemon   | ❌ Connection refused | Daemon is not running                    | Go to Step 0.2                     |
| Daemon   | ❌ Connection timeout | Daemon is hung or misconfigured          | Go to Step 0.3                     |

#### Step 0.1b - Install cypherdb (if missing)

```bash
pip install cypherdb
```

cypherdb is a pure-Python package with no system dependencies. Works on **macOS, Linux, and Windows**.

#### Step 0.2 - Quick Start: Foreground Mode (Development Only)

For quick testing during development, start Daemon in foreground:

```bash
mindx start --port :1314
```

**⚠️ Warning:** This blocks the current terminal. Closing the terminal kills the Daemon and all scheduled tasks stop. For persistent operation, use Step 0.3.

#### Step 0.3 - Install as System Service (Production Recommended)

Install MindX Daemon as a system service so it:
- Starts automatically on boot/reboot
- Restarts automatically on crash
- Runs in the background without blocking a terminal

**Choose the installation method based on your OS:**

##### macOS (launchd)

```bash
cat > ~/Library/LaunchAgents/com.mindx.daemon.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.mindx.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/mindx</string>
        <string>start</string>
        <string>--port</string>
        <string>:1314</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$HOME/.mindx/logs/daemon.stdout.log</string>
    <key>StandardErrorPath</key>
    <string>$HOME/.mindx/logs/daemon.stderr.log</string>
</dict>
</plist>
EOF

mkdir -p $HOME/.mindx/logs && launchctl load ~/Library/LaunchAgents/com.mindx.daemon.plist
```

> **Directory note:** `$HOME/.mindx/` is the **Home Directory (Layer 1)** of the 4-layer directory architecture.
> Logs are stored under `$HOME/.mindx/logs/`. Scheduled task data is stored under `$HOME/.mindx/data/schedules/`.

Service management: `launchctl list | grep mindx` (status) / `launchctl unload/load` (restart) / `rm plist && launchctl remove` (uninstall)

##### Linux (systemd)

```bash
sudo tee /etc/systemd/system/mindx.service > /dev/null << 'EOF'
[Unit]
Description=MindX AI Agent Daemon
After=network.target
[Service]
Type=simple
ExecStart=/usr/local/bin/mindx start --port :1314
Restart=always
RestartSec=5
StandardOutput=append:/var/log/mindx/stdout.log
StandardError=append:/var/log/mindx/stderr.log
[Install]
WantedBy=multi-user.target
EOF

sudo mkdir -p /var/log/minx && sudo systemctl daemon-reload && sudo systemctl enable --now mindx.service
```

Service management: `systemctl status mindx` (status) / `systemctl restart mindx` (restart) / `systemctl disable && rm` (uninstall)

#### Step 0.4 - Verify Installation

After installation, always verify both dependencies before proceeding:

```bash
# 1. Verify cypherdb (GraphDB)
python3 -c "import cypherdb; db = cypherdb.Database('runtime/data/test.db'); print('✅ GraphDB ready')"

# 2. Verify Daemon (WebSocket + Scheduler)
python3 scripts/scheduler_client.py test-conn && python3 scripts/scheduler_client.py list-jobs
```

Both checks must pass before proceeding to Phase 1.

### Phase 1: Project Initialization & Goal Clarification

**Trigger:** User proposes a new project ("I want to do xxx", "plan xxx", "create a new project").

#### Step 1.1 - Understand & Guide Intent

The user often provides only a fuzzy idea. You MUST communicate enough to extract quantifiable, verifiable goals. Ask across these dimensions:

| Dimension                | Example Question                        | Why It Matters              |
| ------------------------ | --------------------------------------- | --------------------------- |
| **Core goal**            | "What does success look like?"          | Defines the finish line     |
| **Quantifiable metrics** | "How will you measure it?"              | Makes progress trackable    |
| **Time range**           | "Any deadlines or milestones?"          | Enables scheduling          |
| **Resource constraints** | "What agents are available?"            | Informs task assignments    |
| **Periodic needs**       | "Does any part of this need to repeat?" | Determines Scheduler config |

#### Common Goal-Setting Mistakes

**Too vague:**
> "I want more community engagement"

This is impossible to track or measure. There's no definition of "more" and no timeline.

**Better (quantifiable):**
> "I want to increase daily active users from 50 to 200 within 3 months, measured by login count"

Now there's a baseline (50), target (200), timeline (3 months), and measurement method (login count).

**Overlapping sub-goals:**
> Goal 1: "Write content" / Goal 2: "Create blog posts"

Blog posts ARE content — these overlap and will cause double-counting.

**Good decomposition (MECE):**
> Goal 1: "Research trending topics" / Goal 2: "Write technical articles" / Goal 3: "Promote on social media"

These are distinct phases: research → create → distribute. No overlap, full coverage.

#### Step 1.2 - Output Structured Project Definition

After gathering info, confirm with the user using this format:

```markdown
## 📋 Project Definition Confirmation

**Project Name**: {name}
**Core Goal**: {one-sentence description}
**Success Metrics**: {quantifiable KPIs}
**Time Range**: {start} ~ {end}
**Periodic Needs**: {daily/weekly/monthly/custom}

### Expected Sub-Goals (pending confirmation):
1. **{sub-goal 1}** — Weight: X%
2. **{sub-goal 2}** — Weight: Y%
3. **{sub-goal 3}** — Weight: Z%

Please confirm or adjust the above.
```

#### Step 1.3 - Initialize Project Node in GraphDB

Create the project root node:

```bash
python3 scripts/graph_client.py create-project \
  --name "{project_name}" \
  --description "{description}" \
  --status "active" \
  --metrics '{"kpi": "..."}' \
  --timeline '{"start": "...", "end": "..."}'
```

Save the returned project ID — all subsequent phases reference it.

#### Step 1.4 - Record Sub-Goals in GraphDB

For each confirmed sub-goal, create a Goal node linked to the project:

```bash
python3 scripts/graph_client.py create-goal \
  --project-id "{proj_id}" \
  --title "{goal_title}" \
  --description "{description}" \
  --weight 0.4 \
  --metrics '{"target": "..."}'
```

---

### Phase 2: WBS Goal Decomposition

**Trigger:** User confirms the project definition, or explicitly asks to "break down goals" / "make a plan".

#### Step 2.1 - Decomposition Principles

**MECE** (Mutually Exclusive, Collectively Exhaustive):
- Sub-goals should not overlap with each other
- Together they must cover the full project scope

**Granularity guide:**

| Level                   | Content             | Effort    | Actionable?              |
| ----------------------- | ------------------- | --------- | ------------------------ |
| L1: Deliverable         | Major output module | 1–2 weeks | ❌ Needs decomposition    |
| L2: Work package        | Concrete work unit  | 2–8 hours | ✅ Can assign to an Agent |
| L3: Activity (optional) | Fine-grained step   | < 2 hours | ✅ Finest granularity     |

#### Common Decomposition Mistakes

**Too coarse (not actionable):**
> Task: "Improve community engagement"

This is a goal, not a task. There's no specific action to take.

**Better (actionable):**
> Task: "Write and publish a weekly technical blog post every Monday at 9 AM"

Clear action, clear schedule, clear quality bar.

**Too fine (micromanagement):**
> Task: "Open the text editor" → Task: "Type the title" → Task: "Write the introduction"

These are sub-steps, not tasks. The Agent knows how to write — just tell it what to write.

**Better (right granularity):**
> Task: "Write a 2000-word article about Kubernetes best practices"

Enough detail for the Agent to execute, not so much that you're dictating every keystroke.

#### Step 2.2 - Execute Decomposition

Recursively decompose each sub-goal into actionable tasks. Example:

```
Goal: Increase community activity by 50% (3 months)

L1 Deliverables:
├── [G1] Content Creation (40%)
│   ├── [L2] Topic Research (@researcher)
│   ├── [L2] Article Writing (@writer)
│   └── [L2] Publish & Promote (@social-media)
│
├── [G2] User Engagement (35%)
│   ├── [L2] Reply to Comments (@assistant)
│   ├── [L2] New User Onboarding (@analyst)
│   └── [L2] Topic Interaction (@social-media)
│
└── [G3] Data Analysis (25%)
    ├── [L2] Data Collection (@analyst)
    ├── [L2] Trend Analysis (@analyst)
    └── [L2] Report Generation (@writer)
```

#### Step 2.3 - Record to GraphDB

Create Task nodes for each leaf task and establish relationships:

```bash
# Create task nodes under each goal
python3 scripts/graph_client.py create-task \
  --goal-id "{goal_id}" \
  --title "{task_title}" \
  --agent "@writer" \
  --cron-expr "0 0 9 * * 1" \
  --prompt "Detailed execution instructions..."

# Establish dependency relationships (if any)
python3 scripts/graph_client.py add-dependency \
  --task-id "{task_id}" \
  --depends-on "{predecessor_id}"
```

#### Step 2.4 - Output Task Tree

Present the decomposition to the user:

```markdown
## ✅ Goal Decomposition Complete

📊 Project: {name} | Total Tasks: N | Max Depth: L

### Critical Path:
{List tasks on the critical path}

### Ready to Execute (no dependencies):
1. [{task_name}](@agent) — Est. Xh — P{0-2}

### Resource Allocation:
| Agent       | Tasks | %   |
| ----------- | ----- | --- |
| @writer     | N     | X%  |
| @researcher | M     | Y%  |

⚠️ Risk Notes:
{Any identified risks}
```

---

## Task State Machine

Every task in the system follows a defined lifecycle. The **Primary Agent** is responsible for managing state transitions and ensuring SubAgent sessions are properly tracked throughout execution.

### State Definition

```
                         ┌─────────────────┐
                         │    pending      │
                         │ (initial/reset)  │
                         └────────┬────────┘
                                  │ schedule
                                  ▼
                         ┌─────────────────┐
                    ┌───►│   scheduled     │◄──┐
                    │    │ (registered)     │   │
                    │    └────────┬────────┘   │
                    │             trigger       │ retry
                    │             ▼              │
                    │    ┌─────────────────┐    │
                    │    │  in_progress    │────┘
                    │    │ (executing)      │
                    │    └────┬────┬───────┘
                    │         │    │
                    │    complete│  │interrupt
                    │         ▼    ▼
                    │    ┌───────────┐  ┌─────────────────────┐
                    │    │ completed │  │ awaiting_* (interrupted)│
                    │    │ (success)  │  └──────┬──────────────┘
                    │    └───────────┘         │
                    │                   ┌─────┴─────┬──────────┐
                    │                   │           │          │
                    │                   ▼           ▼          ▼
                    │           ┌───────────┐ ┌──────────┐ ┌──────────┐
                    │           │awaiting_  │ │awaiting_  │ │awaiting_ │
                    │           │auth       │ │clarify    │ │resource  │
                    │           └─────┬─────┘ └─────┬─────┘ └─────┬─────┘
                    │                 │            │             │
                    │        resolved │      answered│      available│
                    │                 ▼            ▼             ▼
                    │           ┌──────────────────────────────────┐
                    └──────────►│        in_progress (resume)      │
                                └──────────────────────────────────┘
                                                             │
                                                        timeout / fail
                                                             ▼
                                                    ┌─────────────┐
                                                    │   failed    │
                                                    │ (execution failed)│
                                                    └─────────────┘
```

### State Descriptions

| State                    | Description                                              | Who Manages            | Session Action                     |
| ------------------------ | -------------------------------------------------------- | ---------------------- | ---------------------------------- |
| `pending`                | Initial state, task defined but not yet scheduled        | Primary Agent          | —                                  |
| `scheduled`              | Registered with Scheduler, waiting for trigger           | Scheduler auto         | —                                  |
| `in_progress`            | SubAgent is actively executing                           | SubAgent               | session active                     |
| `completed`              | Task finished successfully                               | Primary Agent (verify) | session closed                     |
| `awaiting_authorization` | SubAgent blocked, needs user permission                  | **Primary Agent**      | session **preserved**              |
| `awaiting_clarification` | SubAgent needs more info to continue                     | **Primary Agent**      | session **preserved**              |
| `awaiting_resource`      | SubAgent missing required resource (API key, file, etc.) | **Primary Agent**      | session **preserved**              |
| `failed`                 | Execution failed after retries or timeout                | Primary Agent          | session may be preserved for debug |

### Critical Rule: Session Preservation

**When a SubAgent enters any `awaiting_*` state, its session MUST be preserved in GraphDB.**

The Primary Agent uses this `session_id` to resume the SubAgent in the correct conversation context using the Demon command format:

```
@agent_name <session_id> <content>
```

Without `session_id`, resuming a SubAgent creates a new context-less session, losing all prior work and conversation history.

---

### Phase 3: Task Scheduling & Agent Assignment

**Trigger:** Automatically after Phase 2 completes, or when the user asks to "schedule tasks" / "set up recurring work".

#### Step 3.1 - Analyze Each Leaf Task

For every leaf-level task, determine:

**Agent Selection Guide:**

| Task Type        | Recommended Agent | Examples                        |
| ---------------- | ----------------- | ------------------------------- |
| Content creation | @writer           | Articles, copy, reports         |
| Data research    | @researcher       | Surveys, information gathering  |
| Data analysis    | @analyst          | Analysis, visualization         |
| General task     | @assistant        | Simple processing, coordination |

**Cron Expression Design:**

| Scenario          | Cron Expression  | Meaning                       |
| ----------------- | ---------------- | ----------------------------- |
| Daily execution   | `0 0 9 * * *`    | Every day at 9:00 AM          |
| Weekly execution  | `0 0 9 * * 1`    | Every Monday at 9:00 AM       |
| Monthly execution | `0 0 9 1 * *`    | 1st of every month at 9:00 AM |
| Custom interval   | `*/30 * * * * *` | Every 30 minutes              |

Note: MindX Scheduler uses a 6-field cron format (second minute hour day month weekday).

**Prompt Design:**

Write clear execution instructions for each task, including:
- Task background and context
- Specific input/output requirements
- Quality standards and acceptance criteria
- Links to relevant resources

#### Common Scheduling Mistakes

**Wrong cron format (5-field instead of 6-field):**
> `0 9 * * 1`

MindX Scheduler requires 6 fields (includes seconds). This will either fail or be misinterpreted.

**Correct (6-field):**
> `0 0 9 * * 1`

Format: second minute hour day month weekday.

**Vague task prompt:**
> "Write something about AI"

The Agent doesn't know the audience, tone, length, or angle. The output will be generic.

**Better (specific prompt):**
> "Write a 1500-word technical blog post about Kubernetes deployment strategies for a senior developer audience. Cover blue-green, canary, and rolling deployments with pros/cons for each."

#### Step 3.2 - Register Tasks via WebSocket Scheduler

Use `scripts/scheduler_client.py` to communicate with MindX Gateway and register scheduled jobs.

**Prerequisite check:**

```bash
# Test Gateway connectivity
python3 scripts/scheduler_client.py test-conn

# Install dependency (first-time only)
pip install websocket-client
```

**Option A: Register a single task** — useful for ad-hoc additions:

```bash
python3 scripts/scheduler_client.py add-job \
    --agent "@writer" \
    --content "Every Monday: Write a technical blog post about AI engineering practices" \
    --cron "0 0 9 * * 1"
```

**Option B: Batch register tasks (recommended for project initialization):**

1. Create a JSON task file:
```json
[
    {
        "agent": "@writer",
        "content": "Every Monday: Write a technical blog post about AI engineering practices",
        "cron_expr": "0 0 9 * * 1"
    },
    {
        "agent": "@researcher",
        "content": "Every Monday: Research trending AI topics for the week",
        "cron_expr": "0 0 10 * * 1"
    },
    {
        "agent": "@analyst",
        "content": "Every Friday: Analyze community data and generate a weekly report",
        "cron_expr": "0 0 16 * * 5"
    }
]
```

2. Submit to Scheduler:
```bash
python3 scripts/scheduler_client.py batch-add --file /tmp/project-tasks.json
```

3. Verify registration:
```bash
python3 scripts/scheduler_client.py list-jobs
python3 scripts/scheduler_client.py list-jobs --json  # machine-readable output
```

#### Step 3.3 - Record Scheduler Info to GraphDB

Link the Scheduler-returned task ID back to the GraphDB Task node so you can trace execution history:

```bash
# Update the task with the scheduler_id from the registration response
python3 scripts/graph_client.py update-task \
    --task-id "{graphdb_task_id}" \
    --status "scheduled" \
    --scheduler-id "{scheduler_returned_id}"
```

#### Step 3.4 - Output Scheduling Confirmation

Show the user a summary of what was registered:

```markdown
## ✅ Task Scheduling Complete

📋 Project: {name} | Scheduled Tasks: N | Gateway: localhost:8081

### Registration Details:
| #   | Task   | Agent       | Cron         | Scheduler ID | Status       |
| --- | ------ | ----------- | ------------ | ------------ | ------------ |
| 1   | {task} | @writer     | 0 0 9 * * 1  | a1b2c3d4     | ✅ Registered |
| 2   | {task} | @researcher | 0 0 10 * * 1 | e5f6g7h8     | ✅ Registered |

### Next Execution Times:
- {Task 1}: {timestamp}
- {Task 2}: {timestamp}

⚠️ Notes:
{Any failures or warnings}
```

---

### Phase 3.5: SubAgent Session Registration

**Trigger:** Automatically after Phase 3.3 completes — every scheduled task must have its session tracking initialized.

#### Why This Phase Exists

SubAgents run as **black-box processes** in the MindX Daemon (Demon). When a SubAgent is triggered by the Scheduler:

1. The Demon creates a new session for that Agent
2. The Agent begins executing the task
3. **The Agent may be interrupted** — needing authorization, clarification, or resources
4. When interrupted, the Agent's session **pauses but must not be lost**
5. The Primary Agent needs to know **which session** to resume

Without recording `session_id`, the Primary Agent cannot:
- Resume an interrupted SubAgent in the correct context
- Verify which specific execution instance produced which output
- Diagnose failures by inspecting the actual conversation log

#### Step 3.5.1 - Initialize Session Record

After each task is registered with the Scheduler, create a session tracking record in GraphDB:

```bash
python3 scripts/graph_client.py register-session \
    --task-id "{graphdb_task_id}" \
    --agent "@writer" \
    --session-status "initialized" \
    --created-by "primary_agent"
```

This returns a `session_id` that uniquely identifies this task's execution context.

#### Step 3.5.2 - Link Session to Scheduler Job

Update the task node with both scheduler and session information:

```bash
python3 scripts/graph_client.py update-task \
    --task-id "{task_id}" \
    --scheduler-id "{scheduler_returned_id}" \
    --session-id "{session_id}" \
    --status "scheduled"
```

**The task now has two critical identifiers:**

| ID             | Source          | Purpose                                                     |
| -------------- | --------------- | ----------------------------------------------------------- |
| `scheduler_id` | MindX Scheduler | Correlates trigger events with task definitions             |
| `session_id`   | GraphDB / Demon | Enables Primary Agent to resume SubAgent in correct context |

#### Step 3.5.3 - Demon Communication Protocol

When communicating with SubAgents through the MindX Daemon, use this command format:

```
@agent_name <session_id> <content>
```

**Format breakdown:**

| Component      | Required | Description                                                                                                                  |
| -------------- | -------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `@agent_name`  | ✅        | Target agent identifier (e.g., `@writer`, `@researcher`)                                                                     |
| `<session_id>` | ✅        | Session to target — **client generates a UUID v4** (e.g., `a1b2c3d4-...`) for new sessions, or provide existing ID to resume |
| `<content>`    | ✅        | The message/instruction to send                                                                                              |

**Session ID behavior (client-managed):**
- `"new"` — **The client (this skill) generates a UUID v4** (e.g., `550e8400-e29b-41d4-a716-446655440000`) as the session identifier before sending the command to the Daemon. Record this UUID in GraphDB via `register-session` if you need to resume the session later.
- `<existing_id>` — Resumes the exact conversation context of that session.

**Examples:**

```bash
# Start a new session for a task
@writer new Write a 1500-word article about Kubernetes best practices

# Resume an interrupted session (after user authorized access)
@writer sess_abc123 User has granted API access permission. Please continue from where you left off.

# Ask for clarification in an existing session
@researcher sess_def456 The user clarified: they want market data for APAC region only, not global. Please adjust your research scope.

# Check status of a running session
@analyst sess_ghi789 Please report your current progress and any blockers.
```

**⚠️ Critical: Never omit `session_id`**

Omitting `session_id` creates a new orphan session with no connection to the original task context. This causes:
- Duplicate work (Agent re-starts from scratch)
- Context loss (prior conversation history inaccessible)
- GraphDB desynchronization (execution results cannot be linked to task)

#### Step 3.5.4 - Output Session Registration Summary

After registering all sessions:

```markdown
## ✅ Session Registration Complete

📋 Project: {name} | Tasks: N | Sessions Registered: N

### Session Tracking Table:
| #   | Task        | Agent       | Scheduler ID | Session ID | Status      |
| --- | ----------- | ----------- | ------------ | ---------- | ----------- |
| 1   | {task_name} | @writer     | sched_123    | sess_abc   | initialized |
| 2   | {task_name} | @researcher | sched_456    | sess_def   | initialized |
| 3   | {task_name} | @analyst    | sched_789    | sess_ghi   | initialized |

### Demon Command Reference (for Primary Agent use):
- Resume writer: `@writer sess_abc <message>`
- Resume researcher: `@researcher sess_def <message>`
- Resume analyst: `@analyst sess_ghi <message>`
```

---

### Phase 4: Progress Tracking & Reporting

**Trigger:** User asks "how is xxx project going", "show project status", "generate a report", or a scheduled daily/weekly report job fires.

#### Step 4.1 - Query GraphDB for Project Data

```bash
# Project overview
python3 scripts/graph_client.py query-project --project-id "{proj_id}"

# All goals and their progress
python3 scripts/graph_client.py query-goals --project-id "{proj_id}"

# All tasks under a specific goal
python3 scripts/graph_client.py query-tasks --goal-id "{goal_id}"
```

#### Step 4.2 - Generate Markdown Progress Report

Use this template — fill in data from GraphDB queries (do NOT fabricate numbers):

```markdown
# 📊 {Project Name} — Progress Report

**Updated**: {timestamp}

---

## Overall Progress

| Metric             | Value   |
| ------------------ | ------- |
| Overall completion | {X}%    |
| Completed tasks    | {N}/{M} |
| In-progress tasks  | {K}     |
| Overdue tasks      | {L} ⚠️   |

---

## Goal Details

### 🎯 {Goal 1 Name} ({progress}%)

**Status**: {pending/in_progress/completed}

| Task     | Agent  | Status | Last Run | Success/Fail |
| -------- | ------ | ------ | -------- | ------------ |
| {Task 1} | @agent | ✅      | {date}   | 5/0          |
| {Task 2} | @agent | 🔄      | {date}   | 3/1          |
| {Task 3} | @agent | ⏳      | —        | 0/0          |

*(Repeat for each goal)*

---

## ⚠️ Issues Requiring Attention

1. **{Issue 1}**
   - Impact: {scope}
   - Recommendation: {solution}

---

## 💡 Next Steps

1. **[P0]** {Recommendation 1}
2. **[P1]** {Recommendation 2}
```

#### Step 4.3 - Generate Mermaid Gantt Chart (Optional)

Only generate when the user explicitly asks for a chart:

````mermaid
gantt
    title {Project Name} Gantt Chart
    dateFormat  YYYY-MM-DD

    section {Goal 1 Name}
    {Task 1}          :a1, {start}, {duration}
    {Task 2}          :a2, after a1, {duration}

    section {Goal 2 Name}
    {Task 3}          :b1, {start}, {duration}
````

---

### Phase 4.5: Active Verification & Interruption Recovery

**Trigger:** Primary Agent proactively checks task execution status, OR detects `awaiting_*` states in GraphDB queries.

#### Why This Phase Exists — The "Closed-Loop" Problem

The traditional project management approach is **open-loop**:

```
Primary Agent → Scheduler → SubAgent → (black box) → GraphDB result
                                                    ↑
                                          Primary Agent only reads result
```

This fails when:
1. **SubAgent is interrupted** — needs authorization, clarification, or resources
2. **SubAgent produces wrong output** — no one verifies quality before marking "completed"
3. **SubAgent gets stuck** — no timeout or health check mechanism

The **closed-loop** design adds active verification and recovery:

```
Primary Agent → Scheduler → SubAgent ──┬──► Normal completion
                                       ├──► Interrupted (awaiting_*) ──► Primary Agent handles
                                       └──► Stuck/timeout ──► Primary Agent intervenes
        ↑                              │
        └──── Verify output ◄──────────┘
             Resume session if needed
```

#### Step 4.5.1 - Periodic Health Check

Query GraphDB for all tasks in non-terminal states:

```bash
# Find all tasks that need attention
python3 scripts/graph_client.py query-tasks --status "in_progress,awaiting_authorization,awaiting_clarification,awaiting_resource"
```

**Check intervals:**
| Task Type           | Check Frequency | Reason                           |
| ------------------- | --------------- | -------------------------------- |
| Short tasks (< 2h)  | Every 30 min    | Quick tasks should complete fast |
| Medium tasks (2-8h) | Every 2 hours   | Allow time for execution         |
| Long tasks (> 8h)   | Every 4 hours   | Research/analysis takes time     |

#### Step 4.5.2 - Handle Interrupted Sessions (Core Recovery Logic)

When a task is in any `awaiting_*` state, the Primary Agent MUST take action:

##### Scenario A: Awaiting Authorization

**Cause:** SubAgent needs user permission to proceed (e.g., access sensitive data, deploy to production, send external communication).

**Recovery Flow:**

```mermaid
flowchart TD
    A[Detect awaiting_authorization] --> B[Read interruption_context from GraphDB]
    B --> C{What permission is needed?}
    C --> D[Ask User for authorization]
    D --> E{User response}
    E -->|Granted| F[Update session: authorized]
    F --> G[Resume SubAgent with session_id]
    G --> H[@agent_name session_id Permission granted. Continue execution.]
    E -->|Denied| I[Mark task as failed / cancelled]
    E -->|Needs context| J[Clarify with user, then re-ask]
```

**Primary Agent actions:**

```bash
# 1. Get the interruption details
python3 scripts/graph_client.py get-session --session-id "{session_id}"

# 2. Present to user (using the interruption_context)
# Output:
## 🔐 Authorization Required

**Task**: {task_name} (**@agent**)
**Session**: `{session_id}`
**Requested**: {what the SubAgent needs permission for}
**Reason**: {why it's needed}

Options:
- [ ] Grant permission
- [ ] Deny (cancel task)
- [ ] Need more context before deciding

# 3. After user grants permission, resume the SubAgent
# Use Demon command format:
@{agent_name} {session_id} User has granted permission for {action}. Please continue from where you left off.

# 4. Update GraphDB state
python3 scripts/graph_client.py update-session \
    --session-id "{session_id}" \
    --status "resumed" \
    --resolution "user_authorized" \
    --resolved-at "{timestamp}"
```

##### Scenario B: Awaiting Clarification

**Cause:** SubAgent encountered ambiguity in task instructions and cannot proceed without human input.

**Recovery Flow:**

```mermaid
flowchart TD
    A[Detect awaiting_clarification] --> B[Read SubAgent's question from session log]
    B --> C[Analyze: Can Primary Agent answer directly?]
    C -->|Yes, have context| D[Answer directly via Demon command]
    C -->|No, need user input| E[Forward question to user]
    D --> F[@agent_name session_id Answer to your question...]
    E --> G[Collect user's answer]
    G --> F
    F --> H[SubAgent resumes execution]
```

**Primary Agent actions:**

```bash
# The SubAgent's question is stored in interruption_context
# Example question from @writer:
# "The task says 'write about Kubernetes' but doesn't specify:
#  - Target audience (beginner/senior)?
#  - Article length?
#  - Focus area (networking, security, storage)?"

# Primary Agent checks original task prompt for missing details
# If found in project context → answer directly:
@writer sess_abc123 Regarding your questions:
1. Target audience: senior developers (from project definition)
2. Length: 1500 words (from task prompt)
3. Focus: deployment strategies (from goal description)

Please proceed with these parameters.

# If NOT found in context → ask user:
## ❓ SubAgent Needs Clarification

**Task**: {task_name} (**@agent**)
**Session**: `{session_id}`
**Agent's Question**: {the actual question from SubAgent}

Please provide an answer so the agent can continue.
```

##### Scenario C: Awaiting Resource

**Cause:** SubAgent cannot find a required resource (API key, config file, database credentials, external service endpoint).

**Recovery Flow:**

```mermaid
flowchart TD
    A[Detect awaiting_resource] --> B[Identify missing resource]
    B --> C{Resource type?}
    C -->|Config/credential| D[Check if available in project environment]
    C -->|External service| E[Verify service availability]
    D -->|Available| F[Provide resource path/details to SubAgent]
    D -->|Not configured| G[Ask user to provide or configure]
    E -->|Available| F
    E -->|Down| H[Notify user, mark task as blocked]
    F --> I[@agent_name session_id Resource located at ... Please retry.]
```

#### Step 4.5.3 - Output Quality Verification

When a task transitions to `completed`, the Primary Agent SHOULD verify the output before accepting it:

```bash
# 1. Retrieve the task output reference from GraphDB
python3 scripts/graph_client.py get-task-output --task-id "{task_id}"

# 2. Basic verification checklist:
# - Does output exist and is non-empty?
# - Does output match expected format (document, code, data)?
# - Are success metrics within acceptable range?

# 3. If verification fails:
python3 scripts/graph_client.py update-task \
    --task-id "{task_id}" \
    --status "verification_failed" \
    --verification-note "{what was wrong}"

# 4. Request re-execution with corrected prompt:
@{agent_name} new {corrected_prompt_with_more_context}
```

#### Step 4.5.4 - Session Recovery Command Reference

Quick reference for all Demon commands used in recovery scenarios:

| Scenario                  | Command Pattern                     | Example                                          |
| ------------------------- | ----------------------------------- | ------------------------------------------------ |
| New task execution        | `@agent new <prompt>`               | `@writer new Write article about X`              |
| Resume after auth granted | `@agent <sess_id> <message>`        | `@writer sess_abc Permission granted. Continue.` |
| Provide clarification     | `@agent <sess_id> <answer>`         | `@researcher sess_def Audience is senior devs.`  |
| Provide resource info     | `@agent <sess_id> <resource_info>`  | `@analyst sess_ghi API key is in /secrets/`      |
| Request status update     | `@agent <sess_id> Report progress.` | `@writer sess_abc What's your current status?`   |
| Cancel and restart        | (update task + new session)         | Update GraphDB, then `@agent new <prompt>`       |

---

### Phase 5: Exception Handling & Adjustments

#### Scenario 1: Task Execution Failure

**Detection:** GraphDB query returns `failure_count > 0` for a task.

**Actions:**
1. Check the `last_error` field to understand the cause
2. Decide if the Agent assignment or Prompt needs adjustment
3. If retrying, set task status back to `pending`
4. Flag the risk in your report

#### Scenario 2: Goal Drift

**Detection:** Current velocity vs. planned velocity — predict if deadline is achievable.

**Actions:**
1. Proactively notify the user of potential risk
2. Offer adjustment options (reduce scope, add resources, extend deadline)
3. Update risk assessment in GraphDB

#### Scenario 3: Scope Change

**When the user modifies goals or adds new tasks:**
1. Assess the impact scope
2. Update affected GraphDB nodes
3. Recalculate dependencies
4. If any scheduled tasks need updating, output new Scheduler commands

#### Scenario 4: SubAgent Interruption Recovery

**Detection:** GraphDB query returns task status in `awaiting_authorization`, `awaiting_clarification`, or `awaiting_resource`.

This is the **most critical exception scenario** — an interrupted SubAgent is a paused process consuming resources while waiting for input. Without timely recovery, the entire project pipeline can stall.

**Recovery Decision Tree:**

```
Task in awaiting_* state
    │
    ├─► awaiting_authorization
    │     └─► Can Primary Agent auto-authorize based on project policy?
    │           ├─ Yes → Auto-resolve, resume session
    │           └─ No  → Escalate to user (see Phase 4.5.2-A)
    │
    ├─► awaiting_clarification
    │     └─► Does Primary Agent have enough context to answer?
    │           ├─ Yes → Answer directly, resume session (see Phase 4.5.2-B)
    │           └─ No  → Forward question to user
    │
    └─► awaiting_resource
          └─► Is resource available in environment?
                ├─ Yes → Provide path/info, resume session (see Phase 4.5.2-C)
                └─ No  → Ask user to configure/provide resource
```

**Timeout Handling:**

Every `awaiting_*` state should have a configurable timeout:

```bash
# Check for stale interrupted sessions (e.g., awaiting for > 24 hours)
python3 scripts/graph_client.py query-sessions \
    --status "awaiting_*" \
    --stale-threshold "24h"

# For stale sessions:
# Option 1: Force-timeout and mark failed
python3 scripts/graph_client.py update-session \
    --session-id "{session_id}" \
    --status "timeout" \
    --timeout-reason "No response within 24h threshold"

# Option 2: Escalate with urgency flag
# Output:
## ⚠️ STALE INTERRUPTION DETECTED

**Task**: {task_name}
**Session**: `{session_id}`
**Waiting since**: {timestamp} ({X} hours ago)
**Type**: {authorization/clarification/resource}

This session has exceeded the response timeout. Immediate action required.
```

**Retry Strategy After Recovery:**

When resuming a session after interruption, the SubAgent receives the full context of what happened:

```bash
# The resume command MUST include context about the resolution
@writer sess_abc123 RESUME CONTEXT:
- You were interrupted at {timestamp} due to: authorization_required
- Resolution: User granted permission at {resolution_timestamp}
- Your last action before interrupt: {last_action_from_session_log}

Please continue your task from where you left off. All required permissions are now in place.
```

#### Scenario 5: Session Corruption or Loss

**Detection:** `session_id` exists in GraphDB but Demon reports session not found or corrupted.

**Causes:** Demon restart, storage failure, or session expiration.

**Recovery Actions:**
1. Mark old session as `lost` in GraphDB
2. Create a new session via `@agent_name new <prompt>`
3. Include as much context from the original task prompt as possible
4. Link new `session_id` to the existing task node
5. Log the session loss event for audit trail

```bash
# Record session loss before creating replacement
python3 scripts/graph_client.py update-session \
    --session-id "{old_session_id}" \
    --status "lost" \
    --loss-reason "{demon_restart | corruption | expiration}" \
    --replacement-session-id "{new_session_id}"

# Create fresh session
@{agent_name} new This is a restarted session for task: {task_description}
Original session ({old_session_id}) was lost. Please begin execution with full context.
```

---

## Examples

### Example 1: New Project from Vague Idea

**User:** "I want to increase engagement in our developer community"

**Process:**
1. Phase 1: Guided clarification — identified 3 sub-goals (content creation, user engagement, data analysis), timeline: 3 months, metric: daily active users from 50 to 200
2. Phase 2: WBS decomposition — 12 leaf tasks across 3 goals with agent assignments
3. Phase 3: Scheduled 8 recurring tasks via Scheduler using `batch-add`
4. Phase 4: First weekly report generated showing 2/8 tasks completed

**Output:**
```markdown
## ✅ Goal Decomposition Complete

📊 Project: Community Growth | Total Tasks: 12 | Max Depth: 3

### Resource Allocation:
| Agent       | Tasks | %   |
| ----------- | ----- | --- |
| @writer     | 4     | 33% |
| @researcher | 3     | 25% |
| @analyst    | 3     | 25% |
| @assistant  | 2     | 17% |
```

---

### Example 2: Edge Case — Task Failure Recovery

**User:** "The writer agent keeps failing on the weekly blog post task"

**Process:**
1. Query GraphDB → found 3 consecutive failures, `failure_count: 3`
2. Checked task prompt → "Write a blog post" (too vague, no topic guidance)
3. Updated task prompt with specific topic: "Write about Kubernetes deployment strategies for senior developers, 1500 words"
4. Reset status to `pending` → next execution succeeded, `success_count` incremented to 1

**Key insight:** Most task failures stem from vague prompts, not Agent capability issues.

---

### Example 3: Complex Multi-Agent Project with Dependencies

**User:** "Plan a product launch with content, PR, and developer outreach — some tasks need to happen in order"

**Process:**
1. Phase 1: Identified 4 goals (content, PR, devrel, analytics) with 16 total tasks
2. Phase 2: Established dependencies — "Press release" depends on "Product demo ready", "Blog post" depends on "Screenshots available"
3. Phase 3: Scheduled 10 recurring tasks, 6 one-off tasks (not scheduled)
4. Phase 4: Weekly reports tracked blocked tasks vs. ready-to-execute tasks

**Output:** Dependency chain visualization showing critical path through PR → Content → Launch Day.

---

## Pro Tips

1. **Daemon first, always** — Phase 0 is not optional. Every invocation of this skill MUST verify the Daemon is running via `scheduler_client.py test-conn` before attempting any scheduling operation. A silent connection failure wastes the entire planning effort.

2. **Start with metrics first** — If the user can't define success criteria, the project will drift. Push for quantifiable goals before decomposing. A goal without a number is just an opinion.

3. **Keep leaf tasks at 2-8 hours** — Anything larger needs further breakdown. Anything smaller is micromanagement. If a task says "Write a 5000-word ebook", break it into chapters.

4. **Use batch-add for project initialization** — Registering tasks one by one is slow and error-prone. Always use the JSON file approach with `scheduler_client.py batch-add`.

5. **Always confirm before scheduling** — Show the user the full task tree before registering with Scheduler. It's much easier to adjust a plan than to delete and recreate scheduled tasks.

6. **Monitor failure_count, not just status** — A task with status "scheduled" but `failure_count > 0` needs attention even if it hasn't been marked "failed". Repeated failures indicate a systemic issue (bad prompt, wrong agent, missing resource).

7. **Dependencies should flow logically** — Avoid diamond dependencies (A→B, A→C, B→D, C→D) when possible. They create scheduling conflicts and make the critical path hard to reason about.

8. **Record scheduler_id in GraphDB** — This creates a traceable link between the graph database task node and the actual Scheduler job. Without it, you can't correlate execution history with task definitions.

9. **Generate reports proactively** — Don't wait for the user to ask. If a project has been running for a week, generate an unsolicited status report. It builds trust and catches problems early.

10. **Weight goals, not tasks** — Goal weights should reflect business importance, not effort. A goal with 1 task might be more important than a goal with 10 tasks.

11. **Use the 6-field cron format** — MindX Scheduler requires seconds as the first field. Traditional 5-field cron expressions (`0 9 * * 1`) will not work. Always use `0 0 9 * * 1` format.

12. **Always record session_id when scheduling tasks** — Without `session_id`, you cannot resume an interrupted SubAgent in the correct context. Every task registered with the Scheduler MUST have a corresponding session record in GraphDB from Phase 3.5.

13. **Use the Demon protocol correctly: `@agent <session_id> <content>`** — Never send commands to SubAgents without a session ID. Omitting it creates orphan sessions that desynchronize from your task tracking. For new sessions, **the client generates a UUID v4** (e.g., `550e8400-e29b-41d4-a716-446655440000`) as the session identifier — record this client-generated UUID in GraphDB immediately via `register-session`. Session IDs are **client-managed resources**; the Daemon only routes them, never creates them. Use an existing recorded `session_id` only when resuming interrupted sessions.

14. **Treat awaiting_* states as urgent** — An interrupted SubAgent is a blocked pipeline. Check for interruptions at every health check cycle (Phase 4.5.1). A task stuck in `awaiting_authorization` for 24 hours is a project blocker, not a minor issue.

15. **Include RESUME CONTEXT when recovering sessions** — When resuming an interrupted SubAgent, always tell it: (a) why it was interrupted, (b) what happened during the interruption, (c) what resolution was applied. This prevents duplicate work and confusion.

16. **Primary Agent owns the closed-loop** — The Primary Agent is not just a task scheduler; it is the supervisor responsible for verifying output quality and recovering interrupted sessions. Passive status queries are insufficient — active verification (Phase 4.5) is mandatory for reliable multi-agent coordination.

---

## Common Project Management Requests

```
Plan a project for [goal] with [N] month timeline
```

```
Break down this goal: "[goal description]"
```

```
Schedule [task] to run every [day/week/month] at [time]
```

```
Show me the status of project [name]
```

```
Generate a weekly report for project [name]
```

```
The [agent] agent keeps failing on [task]. What's wrong?
```

```
I need a Gantt chart for project [name]
```

```
Adjust the schedule for [task] from [old_cron] to [new_cron]
```

---

## Related Use Cases

- **`task-decompose`** — A focused skill for decomposing individual goals without full project lifecycle management. Use this when you only need WBS breakdown, not scheduling or tracking.
- **`report-generator`** — Specialized report generation with custom templates. The project-manager skill generates standard progress reports; use report-generator for branded or stakeholder-specific formats.
- **`agent-coordinator`** — Multi-agent orchestration without persistence. Use this when you need real-time agent coordination but don't need to track progress over time.
- **`scheduler-admin`** — Direct Scheduler management without project context. Use this for ad-hoc task management outside of a project.

---

## Available Scripts

### scripts/graph_client.py

Python graph database client for project CRUD operations. Uses **cypherdb** (pure Python, cross-platform) as the embedded graph engine. See `references/data-model.md` for the complete data model.

**Prerequisite:** `pip install cypherdb`

**Task CRUD:**
```bash
python3 scripts/graph_client.py create-project --name "..." --description "..."
python3 scripts/graph_client.py create-goal --project-id "..." --title "..." --weight 0.4
python3 scripts/graph_client.py create-task --goal-id "..." --agent "@writer" --cron-expr "..." --prompt "..."
python3 scripts/graph_client.py update-task --task-id "..." --status completed --scheduler-id "..." --session-id "..."
python3 scripts/graph_client.py progress-report --project-id "..."
```

**Session Management (NEW — for SubAgent lifecycle tracking):**
```bash
# Register a new session for a task
# Note: session_id is a UUID v4 generated by the client (this skill) before sending @agent command
python3 scripts/graph_client.py register-session \
    --task-id "{task_id}" \
    --agent "@writer" \
    --session-id "{client_generated_uuid}" \
    --session-status "initialized" \
    --created-by "primary_agent"

# Query session details (includes interruption_context if interrupted)
python3 scripts/graph_client.py get-session --session-id "{session_id}"

# Update session state after recovery action
python3 scripts/graph_client.py update-session \
    --session-id "{session_id}" \
    --status "resumed|authorized|clarified|timeout|lost" \
    --resolution "user_authorized|auto_resolved|context_provided" \
    --resolved-at "{timestamp}"

# Find stale/interrupted sessions needing attention
python3 scripts/graph_client.py query-sessions \
    --status "awaiting_*" \
    --stale-threshold "24h"

# Query tasks by status (including interruption states)
python3 scripts/graph_client.py query-tasks \
    --status "in_progress,awaiting_authorization,awaiting_clarification,awaiting_resource"

# Get task output for quality verification
python3 scripts/graph_client.py get-task-output --task-id "{task_id}"
```

**Extended Task Update Fields:**
```bash
python3 scripts/graph_client.py update-task \
    --task-id "{task_id}" \
    --status "scheduled|in_progress|completed|failed|verification_failed|awaiting_authorization|awaiting_clarification|awaiting_resource" \
    --scheduler-id "{scheduler_returned_id}" \
    --session-id "{session_id}" \
    --interruption-type "authorization_required|clarification_needed|resource_missing" \
    --interruption-context '{"question": "...", "needed_resource": "..."}' \
    --verification-note "{quality_check_result}"
```

### scripts/scheduler_client.py

WebSocket client for MindX Scheduler registration. See `references/scheduler-integration.md` for protocol details.

Quick reference:
```bash
python3 scripts/scheduler_client.py test-conn
python3 scripts/scheduler_client.py add-job --agent "@writer" --content "..." --cron "..."
python3 scripts/scheduler_client.py batch-add --file tasks.json
python3 scripts/scheduler_client.py list-jobs
python3 scripts/scheduler_client.py del-job --id a1b2c3d4
```

### Demon Communication Protocol

All SubAgent communication goes through the MindX Daemon using this format:

```
@agent_name <session_id> <content>
```

**Complete command syntax reference:**

| Use Case                   | Command                             | session_id Value    |
| -------------------------- | ----------------------------------- | ------------------- |
| Start new execution        | `@agent new <prompt>`               | `new`               |
| Resume interrupted session | `@agent <sess_id> <message>`        | existing session ID |
| Send clarification answer  | `@agent <sess_id> <answer>`         | existing session ID |
| Notify of authorization    | `@agent <sess_id> <auth_result>`    | existing session ID |
| Provide resource location  | `@agent <sess_id> <resource_info>`  | existing session ID |
| Request status check       | `@agent <sess_id> Report progress.` | existing session ID |

**⚠️ Protocol Rules:**
1. `session_id` is ALWAYS required — never omit it
2. Use `"new"` only when intentionally starting a fresh context
3. When resuming, always include RESUME CONTEXT in the message body so the SubAgent understands what happened while it was paused

---

## Quality Checklist

Self-verify after completing each phase:

### Phase 1 ✅
- [ ] Do I truly understand what the user wants, or am I guessing?
- [ ] Are goals quantifiable and verifiable (not vague)?
- [ ] Is the timeline realistic?
- [ ] Did I confirm the project definition with the user before proceeding?

### Phase 2 ✅
- [ ] Does the decomposition follow MECE (no overlap, full coverage)?
- [ ] Are leaf tasks at the right granularity (2–8 hours of work)?
- [ ] Are dependencies correct? Any circular dependencies?
- [ ] Is the Agent assignment reasonable for each task?

### Phase 3 ✅
- [ ] Has Gateway connectivity been verified (test-conn)?
- [ ] Are cron expressions syntactically correct (6-field format)?
- [ ] Is each task prompt clear with context, requirements, and quality criteria?
- [ ] Were tasks registered via `scheduler_client.py`?
- [ ] Is the Scheduler-returned ID stored in GraphDB as `scheduler_id`?
- [ ] Was a scheduling confirmation report output?

### Phase 4 ✅
- [ ] Are all numbers sourced from GraphDB (not made up)?
- [ ] Does the report include actionable recommendations?
- [ ] Is the Gantt chart timeline accurate (if generated)?

---

## References

For detailed information, read these reference files when needed:

- **`references/data-model.md`** — Complete GraphDB data model: node types (Project, Goal, Task, Execution, Resource), properties, and relationship types (HAS_GOAL, CONTAINS, DEPENDS_ON, REQUIRES, HAS_EXECUTION)
- **`references/scheduler-integration.md`** — WebSocket CMD protocol details, scheduler_client.py architecture, troubleshooting guide
