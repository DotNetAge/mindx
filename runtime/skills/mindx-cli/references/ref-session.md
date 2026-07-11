# Session 管理

Session 是用户与 Agent 之间对话的基本单元。每个 Session 都有独立的消息历史、上下文，以及可选关联的文件变更记录。**所有命令需要守护进程处于运行状态。**

## 生命周期

```
创建 → (交互) → 获取/列表 → 确认/回滚 → 删除
```

## CRUD 操作

| 任务                      | 命令                                           | 说明                           |
| ------------------------- | ---------------------------------------------- | ------------------------------ |
| 创建新 Session            | `mindx session create --agent <name>`          | 开始一段全新对话               |
| 设置项目目录              | `mindx session create ... --project-dir /path` | 绑定工作目录                   |
| 列出所有 Session          | `mindx session list`                           | 列出所有 Agent 的全部 Session  |
| 按 Agent 过滤             | `mindx session list --agent csm-lead`          | 仅显示该 Agent 的 Session      |
| 以 JSON 格式列出          | `mindx session list --json`                    | 机器可读输出                   |
| 获取 Session 详情         | `mindx session get --session-id <id>`          | 完整消息历史 + 元数据          |
| 以 JSON 获取 Session 详情 | `mindx session get --session-id <id> --json`   | 机器可读输出                   |
| 仅获取元数据              | `mindx session meta --session-id <id>`         | 轻量查询 —— 不含消息           |
| 删除 Session              | `mindx session delete --session-id <id>`       | **破坏性操作** —— 移除历史记录 |

## 文件变更管理

当 Agent 在 Session 期间修改了文件，这些变更会被追踪，可以确认或回滚。

| 任务         | 命令                                                          | 说明                    |
| ------------ | ------------------------------------------------------------- | ----------------------- |
| 确认文件变更 | `mindx session confirm --session-id <id> --files "a.go,b.go"` | 接受修改                |
| 回滚文件变更 | `mindx session rollback --session-id <id> --files "a.go"`     | 恢复到 Session 前的状态 |

### 工作流程
```bash
# 1. 为任务创建一个 Session
SESSION_ID=$(mindx session create --agent developer --project-dir ./myapp)

# 2. Agent 开始工作...（文件在此 Session 下被修改）

# 3. 查看变更概况
mindx session meta --session-id $SESSION_ID

# 4. 确认好的变更，回滚有问题的
mindx session confirm --session-id $SESSION_ID --files "main.go,utils.go"
mindx session rollback --session-id $SESSION_ID --files "experimental.go"

# 5. 完成后，归档或删除
mindx session delete --session-id $SESSION_ID
```

## Session 与定时任务的联动

Session 和定时任务可以协同工作：

```bash
# 创建一个用于周期性工作的 Session 并获取其 ID
TASK_SESSION=$(mindx session create --agent weekly-reporter | awk '{print $3}')

# 让定时任务使用该 Session ID
mindx schedule add \
  --agent weekly-reporter \
  --content "Generate weekly report. Use AgentTalk to report back in session '$TASK_SESSION'" \
  --cron "0 17 * * 5" \          # 每周五下午 5 点
  --session-id $TASK_SESSION

# 之后：查看该 Session 中发生了什么
mindx session get --session-id $TASK_SESSION
```

这正是 customer-success 等 Skill 将长期定时任务关联到图谱节点进行追踪的方式。
