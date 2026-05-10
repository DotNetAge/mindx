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

# Project Manager Skill

An AI-powered project management system that transforms vague ideas into executable, scheduled, and tracked plans. Use this when the user wants to plan something substantial — not a single task, but a multi-step effort requiring goals, timelines, agent assignments, and progress tracking.

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
./scripts/gograph.sh create-project \
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
./scripts/gograph.sh create-goal \
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
./scripts/gograph.sh create-task \
  --goal-id "{goal_id}" \
  --title "{task_title}" \
  --agent "@writer" \
  --cron-expr "0 0 9 * * 1" \
  --prompt "Detailed execution instructions..."

# Establish dependency relationships (if any)
./scripts/gograph.sh add-dependency \
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
./scripts/gograph.sh update-task \
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

### Phase 4: Progress Tracking & Reporting

**Trigger:** User asks "how is xxx project going", "show project status", "generate a report", or a scheduled daily/weekly report job fires.

#### Step 4.1 - Query GraphDB for Project Data

```bash
# Project overview
./scripts/gograph.sh query-project --project-id "{proj_id}"

# All goals and their progress
./scripts/gograph.sh query-goals --project-id "{proj_id}"

# All tasks under a specific goal
./scripts/gograph.sh query-tasks --goal-id "{goal_id}"
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

1. **Start with metrics first** — If the user can't define success criteria, the project will drift. Push for quantifiable goals before decomposing. A goal without a number is just an opinion.

2. **Keep leaf tasks at 2-8 hours** — Anything larger needs further breakdown. Anything smaller is micromanagement. If a task says "Write a 5000-word ebook", break it into chapters.

3. **Use batch-add for project initialization** — Registering tasks one by one is slow and error-prone. Always use the JSON file approach with `scheduler_client.py batch-add`.

4. **Always confirm before scheduling** — Show the user the full task tree before registering with Scheduler. It's much easier to adjust a plan than to delete and recreate scheduled tasks.

5. **Monitor failure_count, not just status** — A task with status "scheduled" but `failure_count > 0` needs attention even if it hasn't been marked "failed". Repeated failures indicate a systemic issue (bad prompt, wrong agent, missing resource).

6. **Dependencies should flow logically** — Avoid diamond dependencies (A→B, A→C, B→D, C→D) when possible. They create scheduling conflicts and make the critical path hard to reason about.

7. **Record scheduler_id in GraphDB** — This creates a traceable link between the graph database task node and the actual Scheduler job. Without it, you can't correlate execution history with task definitions.

8. **Generate reports proactively** — Don't wait for the user to ask. If a project has been running for a week, generate an unsolicited status report. It builds trust and catches problems early.

9. **Weight goals, not tasks** — Goal weights should reflect business importance, not effort. A goal with 1 task might be more important than a goal with 10 tasks.

10. **Use the 6-field cron format** — MindX Scheduler requires seconds as the first field. Traditional 5-field cron expressions (`0 9 * * 1`) will not work. Always use `0 0 9 * * 1` format.

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

### scripts/gograph.sh

Graph database CLI wrapper for project CRUD operations. See `references/data-model.md` for the complete data model.

Quick reference:
```bash
./scripts/gograph.sh create-project --name "..." --description "..."
./scripts/gograph.sh create-goal --project-id "..." --title "..." --weight 0.4
./scripts/gograph.sh create-task --goal-id "..." --agent "@writer" --cron-expr "..." --prompt "..."
./scripts/gograph.sh update-task --task-id "..." --status completed --scheduler-id "..."
./scripts/gograph.sh progress-report --project-id "..."
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
