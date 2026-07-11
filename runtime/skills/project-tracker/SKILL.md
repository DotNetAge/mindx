---
name: project-tracker
description: >
  创建、编排和跟踪长期或周期性项目，可能涉及多个不同领域智能体的协作。
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

## 使用场景

- 需要创建长期运行或周期性项目时；
- 需要根据项目计划编排执行任务时；
- 需要检查项目进度和状态，并将智能体执行结果与项目目标对齐时；

## 任务

### 创建项目

当用户要求创建新项目时执行此步骤。

1. 明确项目目标和范围。与用户充分沟通，通过假设性问题澄清需求，包括：
   - 项目结束日期
   - 项目完成标准/完成定义，以及是否可验证
   - 是否需要多个不同领域的智能体
   - 是否有特殊时间要求，如特定智能体在每天固定时间完成任务
2. 使用 `scripts/project-tool.py project create` 将收集到的项目信息写入项目数据库，完成项目创建：
   ```bash
   python3 scripts/project-tool.py project create \
     --name "项目名称" \
     --description "项目描述" \
     --end-date "2026-08-01" \
     --acceptance-criteria "项目验收标准"
   ```
   - 周期性项目添加 `--recurring`；
   - 命令返回 JSON。记录 `id` 字段，后续所有操作都将其用作 `--project` 参数；
3. 按照 [项目整体进度报告](references/project-brief-format.md) 中的格式向用户展示项目创建结果，供用户确认。信息包括但不限于：
   - 项目名称
   - 项目目标和预期成果
   - 项目描述、特殊要求或具体说明
   - 验收标准
   - 是否为周期性任务（周期性任务的验收标准按单位周期定义）
4. 评估项目复杂度并分解目标。使用 `scripts/project-tool.py goal add` 写入每个目标的验收标准：
   ```bash
   python3 scripts/project-tool.py goal add \
     --project <project_id> \
     --title "目标 1" \
     --acceptance-criteria "此目标的验收标准"
   ```

### 编排项目任务

项目创建完成后执行此步骤。

1. 根据时间要求将目标和里程碑分解为具体、可验证的任务。每个任务包括：
   - 具体任务描述
   - 最适合执行该任务的智能体名称。如果不确定有哪些可用智能体，运行 `mindx agent list`；
   - 任务触发时间，需考虑依赖关系和预估持续时间（基于 LLM 大致执行时间估算）；
   - 验证任务完成的完成标准
2. 使用 Mermaid 绘制甘特图，将编排好的任务展示给用户确认。此过程可能需要多轮调整，直到用户满意。
3. 使用 `scripts/project-tool.py task add` 将每个任务写入项目数据库：
   ```bash
   python3 scripts/project-tool.py task add \
     --project <project_id> \
     --description "具体任务描述" \
     --agent <agent_name> \
     --due-date "2026-07-05" \
     --check-criteria "任务完成指标" \
     --depends-on "task-xxx,task-yyy" \
     --max-retries 5
   ```
   - 仅在存在依赖关系时提供 `--depends-on`。多个任务 ID 用逗号分隔；
   - 一次性任务使用 `--due-date`；周期性任务使用 `--cron`；
   - 命令返回 JSON。记录 `id` 字段作为 `<task_id>`；
4. 对于需要周期性触发的任务，使用 `mindx schedule add` 将其注册到调度器（`<current_agent_name>` 是当前执行此技能的智能体名称，`<project_dir>` 是当前项目目录）：
   ```bash
   mindx schedule add \
     --agent <current_agent_name> \
     --content "请完成项目 <project_id> 的任务 <task_id>：<任务描述>" \
     --cron "0 0 9 * * *" \
     --session-id "<task_id>" \
     --project-dir <project_dir>
   ```
   - 常用 Cron 模式：
     - 每天 09:00：`0 0 9 * * *`
     - 工作日 10:00：`0 0 10 * * 1-5`
     - 每周一 09:00：`0 0 9 * * 1`
   - 命令返回调度 ID。使用 `scripts/project-tool.py task update` 将其回写到任务记录：
     ```bash
     python3 scripts/project-tool.py task update \
       --project <project_id> \
       --id <task_id> \
       --schedule-id <schedule_id>
     ```

### 执行/跟踪项目任务

当用户要求完成当天的任务时执行此步骤。

1. 使用 `scripts/project-tool.py task today` 读取今天到期的任务列表：
   ```bash
   python3 scripts/project-tool.py task today --project <project_id>
   ```
   - 也可使用 `scripts/project-tool.py task next` 查看依赖已完成的可执行任务；
2. 使用 Task 工具记录并执行这些任务；
3. 使用 SubAgent 工具将任务委派给指定智能体，委派内容为具体任务描述。等待智能体完成执行，收集结果，并与任务的 `check_criteria` 比较，确定任务是否完成：
   - 如果结果符合预期，将任务标记为已完成，并记录结果摘要：
     ```bash
     python3 scripts/project-tool.py task update \
       --project <project_id> \
       --id <task_id> \
       --status completed \
       --result-summary "结果摘要"
     ```
   - 如果结果不符合预期且 `retry_count < max_retries`，增加重试计数并重新委派：
     ```bash
     python3 scripts/project-tool.py task update \
       --project <project_id> \
       --id <task_id> \
       --status in_progress \
       --retry-count <new_retry_count>
     ```
     然后告知智能体需要修复的内容，并重新委派；
   - 同一任务最多只能重新委派给 SubAgent 五次进行调整。如果五次尝试后仍无法满足标准，将其标记为失败，并记录原因：
     ```bash
     python3 scripts/project-tool.py task update \
       --project <project_id> \
       --id <task_id> \
       --status failed \
       --failure-reason "失败原因"
     ```
4. 如果任务需要多个不同领域的智能体协作，使用团队工具组建专家团队共同完成任务。收集每个智能体的执行结果，评估是否符合预期。任务状态更新仍使用 `scripts/project-tool.py task update`。

### 每日工作报告

当用户要求查看当天工作状态时执行此步骤。

1. 使用 `scripts/project-tool.py report daily` 生成日报数据：
   ```bash
   python3 scripts/project-tool.py report daily --project <project_id> --date <YYYY-MM-DD>
   ```
   - 省略 `--date` 时默认使用今天日期；
2. 汇总任务要求和执行结果，然后按照 `references/daily-report-format.md` 中的要求生成并展示日报。

### 报告项目整体进度

当用户要求查看项目整体进度时执行此步骤。

1. 使用 `scripts/project-tool.py report progress` 读取当前项目的目标、里程碑和所有任务状态：
   ```bash
   python3 scripts/project-tool.py report progress --project <project_id>
   ```
2. 按照 `references/progress-report-format.md` 中的格式填写并展示项目整体进度报告。

---

## 注意事项

- **脚本输出的可靠性取决于输入数据。** 如果 project-tool.py 返回空数据或不正确的数据，先检查项目 ID 和文件路径，再向用户报告故障。
- **任务依赖循环可能导致死锁。** 如果任务 A 依赖 B，而 B 又依赖 A，两者都无法启动。创建关联任务前，验证依赖图没有循环。
- **智能体可用性不保证。** 如果目标智能体繁忙，子智能体可能会失败。项目协调任务应包含重试策略和每个步骤的回退方案。
- **调度变更会级联影响。** 移动一个任务的截止日期会影响所有下游依赖。重新调度时，始终检查哪些其他任务受到影响，并更新它们。
