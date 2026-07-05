---
name: project-tracker
description: >
  Create, orchestrate, and track long-running or periodic projects,
  potentially involving collaboration among multiple agents from different domains.
allowed-tools:
  - Bash(python3 scripts/project-tool.py *)
  - Bash(mindx schedule *)
  - Bash(mindx agent list)
  - SubAgent
metadata:
  requires:
    bins:
      - python3
  name_zh: 项目跟踪
  name_zh-tw: 專案追蹤
  description_zh: 创建、编排和跟踪长期或周期性项目，可能涉及多个不同领域 agent 协作
  description_zh-tw: 建立、編排和追蹤長期或週期性專案，可能涉及多個不同領域 agent 協作
---

## When to Use

- When you need to create a long-running or periodic project;
- When you need to orchestrate execution tasks according to a project plan;
- When you need to check project progress and status, and align agent execution results with project goals;

## Tasks

### Create Project

Run this step when the user asks you to create a new project.

1. Identify project goals and scope. Communicate fully with the user and use hypothetical questions to clarify requirements, including:
   - Project end date
   - Project completion criteria / definition of done, and whether you can verify them
   - Whether multiple agents from different domains are needed
   - Any special timing requirements, such as a specific agent completing a task at a fixed time each day
2. Use `scripts/project-tool.py project create` to write the collected project information into the project database and complete project creation:
   ```bash
   python3 scripts/project-tool.py project create \
     --name "Project Name" \
     --description "Project description" \
     --end-date "2026-08-01" \
     --acceptance-criteria "Project acceptance criteria"
   ```
   - For periodic projects, add `--recurring`;
   - The command returns JSON. Note the `id` field and use it as the `--project` argument for all subsequent operations;
3. Present the project creation result to the user for confirmation, following the format in `references/project-brief.md`. Information includes but is not limited to:
   - Project name
   - Project objective and expected outcome
   - Project description, special requirements, or specific notes
   - Acceptance criteria
   - Whether it is a recurring task (for recurring tasks, acceptance criteria are defined per unit period)
4. Evaluate project complexity and decompose goals. Use `scripts/project-tool.py goal add` to write acceptance criteria for each goal:
   ```bash
   python3 scripts/project-tool.py goal add \
     --project <project_id> \
     --title "Goal 1" \
     --acceptance-criteria "Acceptance criteria for this goal"
   ```

### Orchestrate Project Tasks

Run this step after project creation is complete.

1. Decompose goals and milestones into concrete, verifiable tasks based on timing requirements. For each task, include:
   - Concrete task description
   - The name of the most suitable agent to execute the task. If unsure which agents are available, run `mindx agent list`;
   - Task trigger time, considering dependencies and estimated duration (estimate based on approximate LLM execution time);
   - Completion criteria for verifying the task is done
2. Draw a Gantt chart using Mermaid to present the orchestrated tasks to the user for confirmation. This process may involve several rounds of adjustment until the user is satisfied.
3. Use `scripts/project-tool.py task add` to write each task into the project database:
   ```bash
   python3 scripts/project-tool.py task add \
     --project <project_id> \
     --description "Concrete task description" \
     --agent <agent_name> \
     --due-date "2026-07-05" \
     --check-criteria "Task completion indicator" \
     --depends-on "task-xxx,task-yyy" \
     --max-retries 5
   ```
   - Provide `--depends-on` only when dependencies exist. Separate multiple task IDs with commas;
   - Use `--due-date` for one-time tasks; use `--cron` for periodic tasks;
   - The command returns JSON. Note the `id` field as `<task_id>`;
4. For tasks that need to be triggered periodically, register them with the scheduler using `mindx schedule add` (where `<current_agent_name>` is the name of the agent currently executing this skill, and `<project_dir>` is the current project directory):
   ```bash
   mindx schedule add \
     --agent <current_agent_name> \
     --content "Please complete task <task_id> of project <project_id>: <task description>" \
     --cron "0 0 9 * * *" \
     --session-id "<task_id>" \
     --project-dir <project_dir>
   ```
   - Common Cron patterns:
     - Daily at 09:00: `0 0 9 * * *`
     - Weekdays at 10:00: `0 0 10 * * 1-5`
     - Every Monday at 09:00: `0 0 9 * * 1`
   - The command returns a schedule ID. Write it back to the task record using `scripts/project-tool.py task update`:
     ```bash
     python3 scripts/project-tool.py task update \
       --project <project_id> \
       --id <task_id> \
       --schedule-id <schedule_id>
     ```

### Execute / Track Project Tasks

Run this step when the user asks you to complete the day's tasks.

1. Use `scripts/project-tool.py task today` to read the list of tasks due today:
   ```bash
   python3 scripts/project-tool.py task today --project <project_id>
   ```
   - You can also use `scripts/project-tool.py task next` to view executable tasks whose dependencies are completed;
2. Record and execute these tasks using the Task tool;
3. Delegate tasks to the specified agent using the SubAgent tool. The delegation content is the concrete task description. Wait for the agent to complete execution, collect the results, and compare them with the task's `check_criteria` to determine whether the task is complete:
   - If the result meets expectations, mark the task as completed and record a result summary:
     ```bash
     python3 scripts/project-tool.py task update \
       --project <project_id> \
       --id <task_id> \
       --status completed \
       --result-summary "Result summary"
     ```
   - If the result does not meet expectations and `retry_count < max_retries`, increase the retry count and re-delegate:
     ```bash
     python3 scripts/project-tool.py task update \
       --project <project_id> \
       --id <task_id> \
       --status in_progress \
       --retry-count <new_retry_count>
     ```
     Then tell the agent what needs to be fixed and delegate again;
   - The same task may only be re-delegated to a SubAgent up to five times for adjustments. If it still cannot meet the criteria after five attempts, mark it as failed and record the reason:
     ```bash
     python3 scripts/project-tool.py task update \
       --project <project_id> \
       --id <task_id> \
       --status failed \
       --failure-reason "Failure reason"
     ```
4. If a task requires collaboration among multiple agents from different domains, use the team tools to form an expert team of those agents to complete the task together. Collect each agent's execution result and evaluate whether it meets expectations. Task status updates still use `scripts/project-tool.py task update`.

### Daily Work Report

Run this step when the user asks to view the day's work status.

1. Use `scripts/project-tool.py report daily` to generate daily report data:
   ```bash
   python3 scripts/project-tool.py report daily --project <project_id> --date <YYYY-MM-DD>
   ```
   - When `--date` is omitted, today's date is used by default;
2. Summarize task requirements and execution results, then generate and display a daily report following the requirements in `references/daily-report-format.md`.

### Report Overall Project Progress

Run this step when the user asks to view overall project progress.

1. Use `scripts/project-tool.py report progress` to read the current project's goals, milestones, and all task statuses:
   ```bash
   python3 scripts/project-tool.py report progress --project <project_id>
   ```
2. Fill out and display the overall project progress report according to the format in `references/progress-report-format.md`.

---

## Gotchas

- **Script output is only as reliable as the input data.** If the project-tool.py returns empty or incorrect data, check the project ID and file paths before reporting failures to the user.
- **Task dependency cycles can deadlock.** If task A depends on B, and B depends on A, neither will start. Before creating linked tasks, verify the dependency graph has no cycles.
- **Agent availability is not guaranteed.** A sub-agent may fail if the target agent is busy. Project coordination tasks should include a retry strategy and a fallback plan for each step.
- **Schedule changes cascade.** Moving one task's deadline affects all downstream dependencies. When rescheduling, always check what other tasks are affected and update them.
