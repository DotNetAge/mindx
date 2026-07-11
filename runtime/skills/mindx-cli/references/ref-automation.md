# 自动化与统计

定时任务、Token 用量追踪和翻译功能。

## 定时任务（Cron）

按固定周期运行 Agent。**需要守护进程。**

| 任务 | 命令 | 说明 |
|------|------|------|
| 列出所有定时任务 | `mindx schedule list` | 显示 Agent、Cron 表达式、启用状态、绑定的 Session |
| 新建定时任务 | `mindx schedule add --agent <name> --content "<prompt>" --cron "0 0 9 * * 1"` | **三个参数均为必填** |
| 绑定到 Session | `mindx schedule add ... --session-id <id>` | 将执行结果关联到一个被追踪的 Session |
| 设置项目目录 | `mindx schedule add ... --project-dir /path` | 定时任务的执行工作目录 |
| 创建时不启用 | `mindx schedule add ... --enabled=false` | 仅创建，暂不激活 |
| 删除定时任务 | `mindx schedule delete --id <schedule-id>` | 永久移除 |
| 以 JSON 格式列出 | `mindx schedule list --json` | 输出机器可读的 JSON |

### Cron 表达式格式

使用 6 位 Cron 表达式，首位为秒：

```
┌───────────── 秒 (0-59)
│ ┌───────────── 分 (0-59)
│ │ ┌───────────── 时 (0-23)
│ │ │ ┌───────────── 日 (1-31)
│ │ │ │ ┌───────────── 月 (1-12)
│ │ │ │ │ ┌───────────── 星期 (0-7, 0 和 7 均表示周日)
│ │ │ │ │ │
* * * * * *
```

### 常用定时配置

| 调度场景 | Cron 表达式 | 用途 |
|----------|------------|------|
| 每个工作日早 9 点 | `0 0 9 * * 1-5` | 每日简报 |
| 每周五下午 5 点 | `0 0 17 * * 5` | 周报 |
| 每月 1 号上午 10 点 | `0 0 10 1 * *` | 月度总结 |
| 每 6 小时 | `0 0 */6 * * *` | 健康检查 |
| 每周日中午 12 点 | `0 0 12 * * 0` | 每周清理 |

### 示例
```bash
# 每日健康检查
mindx schedule add \
  --agent health-monitor \
  --content "Check system health. Report any issues." \
  --cron "0 0 8 * * *" \
  --enabled=true

# 带 Session 追踪的周报
mindx schedule add \
  --agent weekly-reporter \
  --content "Generate weekly progress report. Report back via AgentTalk." \
  --cron "0 0 17 * * 5" \
  --session-id $WEEKLY_SESSION_ID

# 添加/编辑定时任务后：
mindx restart   # 守护进程重新加载定时任务配置
```

## Token 用量统计

追踪 LLM API 的 Token 消耗量。**需要守护进程。**

| 任务 | 命令 | 说明 |
|------|------|------|
| 概览 | `mindx token overview` | 本月与上月对比 |
| 以 JSON 格式输出概览 | `mindx token overview --json` | 机器可读输出 |
| 按月明细 | `mindx token monthly` | 当月每日用量 |
| 指定月份 | `mindx token monthly --year 2026 --month 6` | 历史数据 |
| 按模型统计 | `mindx token by-model --model qwen-max` | 筛选单个模型 |
| 按模型 + 月份统计 | `mindx token by-model --model qwen-max --year 2026 --month 6` | |
| 累计总量 | `mindx token total` | 历史总用量 |
| 以 JSON 输出总量 | `mindx token total --json` | 机器可读输出 |
| 按 Session 统计 | `mindx token session --session-id <id>` | 查看某次对话的消耗 |

### 费用监控流程
```bash
# 月度回顾
mindx token overview
mindx token by-model --model gpt-4o
mindx token by-model --model qwen-max

# 如果某个 Session 消耗过高
mindx token session --session-id abc123

# 定位高消耗的 Session
mindx token monthly --year 2026 --month 6
```

## 翻译

通过守护进程进行文本翻译。**需要守护进程。**

| 任务 | 命令 | 说明 |
|------|------|------|
| 翻译文本 | `mindx translate --text "Hello world" --lang zh` | 目标语言代码 |
| 翻译长文本 | `mindx translate --text "$(cat file.txt)" --lang en` | 通过管道传入内容 |

### 支持的语言代码
常用代码：`en`、`zh`、`ja`、`ko`、`fr`、`de`、`es`、`pt`、`ru`、`ar`
（实际支持情况取决于所配置模型的能力。）
