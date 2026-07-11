---
name: scheduler
description: >
  创建、查看和删除定时任务。
  当用户要求为智能体安排定时任务、自动化、设置周期性任务或 cron 作业时使用。
allowed-tools:
  - Bash(mindx schedule *)
metadata:
  name_zh: 定时任务
  name_zh-tw: 定時任務
  description_zh: 创建、查看和删除定时任务
  description_zh-tw: 建立、檢視和刪除定時任務
---

# 调度器：周期性任务管理

你正在通过 `mindx schedule` CLI 管理定时任务。

## 命令参考

### `mindx schedule list`

列出所有定时任务。

```
mindx schedule list
```

输出：包含 **ID**、**Agent**、**Cron**、**Enabled**、**Created** 列的表格。

### `mindx schedule add`

添加新的周期性定时任务。

必需参数：
- `--agent` — 目标智能体名称（如 `writer`、`architect`）
- `--content` — 定时任务触发时发送给智能体的提示内容
- `--cron` — 6 字段 cron 表达式（如 `"0 0 9 * * *"` 表示每天 09:00）

可选参数：
- `--session-id` — 关联现有的会话 UUID 或图任务 ID
- `--project-dir` — 设置任务的项目工作目录
- `--enabled` — 立即启用（默认：`true`；传入 `--enabled=false` 创建禁用状态）

示例：

```
mindx schedule add \
  --agent writer \
  --content "每日站会报告" \
  --cron "0 0 9 * * *"

mindx schedule add \
  --agent writer \
  --content "博客文章" \
  --cron "0 0 9 * * 1" \
  --session-id "task-abc123" \
  --project-dir /path/to/project

mindx schedule add \
  --agent architect \
  --content "审查待处理的 PR 并总结" \
  --cron "0 0 10 * * 1-5" \
  --enabled false
```

### `mindx schedule delete`

通过 ID 删除定时任务。

必需参数：
- `--id` — 调度条目 ID（在 `list` 输出中显示）

示例：

```
mindx schedule delete --id a1b2c3d4
```

## 常用 Cron 模式

| 用途               | Cron 表达式        | 描述                           |
| ------------------ | ------------------ | ------------------------------ |
| 每天上午 9 点      | `0 0 9 * * *`      | 每天 09:00                     |
| 工作日上午 10 点   | `0 0 10 * * 1-5`   | 周一至周五 10:00               |
| 每周一             | `0 0 9 * * 1`      | 每周一 09:00                   |
| 每小时             | `0 * * * *`        | 每小时的第 0 分钟              |
| 每 30 分钟         | `*/30 * * * *`     | 每小时的 :00 和 :30            |
| 每月 1 号          | `0 0 9 1 * *`      | 每月 1 号 09:00                |
