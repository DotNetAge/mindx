# MindX Scheduler 使用指南

> **重要提示：** 本文档面向三类不同的读者群体，请根据你的角色选择相应章节：
>
> - 📱 **普通用户 / TUI 用户** → [第 4 章：通过 WebSocket 使用 Scheduler](#4-通过-websocket-使用-scheduler)
> - 🔌 **第三方系统集成者** → [第 5 章：第三方集成（JSON 文件操作）](#5-第三方集成json-文件操作)
> - 🔧 **MindX 核心开发者** → [第 6 章：编程级 API 参考](#6-编程级-api-参考仅限核心开发者)

---

## 📖 目录

**基础概念（所有读者必读）**
- [1. 核心概念与定位](#1-核心概念与定位)
  - [1.1 什么是 MindX Scheduler](#11-什么是-mindx-scheduler)
  - [1.2 它不是什么](#12-它不是什么)
  - [1.3 三种使用方式对比](#13-三种使用方式对比)

**数据模型（理解底层结构）**
- [2. ScheduleEntry 数据模型](#2-scheduleentry-数据模型)
  - [2.1 字段定义](#21-字段定义)
  - [2.2 JSON 存储格式](#22-json-存储格式)

**按读者类型分类**

- [4. 通过 WebSocket 使用 Scheduler](#4-通过-websocket-使用-scheduler) ← TUI/客户端用户
  - [4.1 WebSocket CMD 协议简介](#41-websocket-cmd-协议简介)
  - [4.2 /job-add 创建定时消息](#42-job-add-创建定时消息)
  - [4.3 /job-list 查询任务列表](#43-job-list-查询任务列表)
  - [4.4 /job-del 删除定时消息](#44-job-del-删除定时消息)
  - [4.5 Cron 表达式速查](#45-cron-表达式速查)

- [5. 第三方集成（JSON 文件操作）](#5-第三方集成json-文件操作) ← 系统集成者
  - [5.1 为什么只能操作文件](#51-为什么只能操作文件)
  - [5.2 数据目录位置](#52-数据目录位置)
  - [5.3 创建任务（写入 JSON）](#53-创建任务写入-json)
  - [5.4 查询任务（读取 JSON）](#54-查询任务读取-json)
  - [5.5 更新任务（修改 JSON）](#55-更新任务修改-json)
  - [5.6 删除任务（删除 JSON）](#56-删除任务删除-json)
  - [5.7 热更新机制](#57-热更新机制)
  - [5.8 完整示例：Shell 脚本管理](#58-完整示例shell-脚本管理)
  - [5.9 完整示例：Python 脚本管理](#59-完整示例python-脚本管理)

- [6. 编程级 API 参考（仅限核心开发者）](#6-编程级-api-参考仅限核心开发者) ← MindX 开发者
  - [6.1 Scheduler 核心组件](#61-scheduler-核心组件)
  - [6.2 FileSchedulerStore 接口](#62-fileschedulerstore-接口)
  - [6.3 CommandExecutor 接口](#63-commandexecutor-接口)
  - [6.4 数据流与生命周期](#64-数据流与生命周期)
  - [6.5 扩展开发指南](#65-扩展开发指南)

**通用参考**
- [7. 最佳实践与故障排查](#7-最佳实践与故障排查)
- [8. 附录](#8-附录)

---

## 1. 核心概念与定位

### 1.1 什么是 MindX Scheduler

MindX Scheduler 是一个**定时消息调度系统**，其唯一职责是：

> **按照预定的时间规则，自动向指定的 AI Agent 发送消息内容。**

**本质：** Agent 的"闹钟"系统 —— 到了设定时间，就给某个 Agent 发一条消息。

### 1.2 它不是什么

| ❌ 常见误解 | ✅ 正确理解 |
|------------|-----------|
| 通用的 Cron 任务执行器 | 只能向 Agent 发消息，不执行 shell 命令 |
| 工作流引擎 | 不支持任务依赖、条件分支等复杂逻辑 |
| 消息队列 | 不具备消息持久化、重试、确认机制 |
| CLI 命令工具 | job-xxx 是 **WebSocket CMD 协议指令**，不是命令行命令 |

### 1.3 三种使用方式对比

根据你的身份和需求，有三种方式可以管理 Scheduler 任务：

| 使用方式 | 适用场景 | 访问层级 | 复杂度 |
|---------|---------|---------|--------|
| **WebSocket CMD** | TUI 用户、Dashboard 用户 | 应用层协议 | ⭐ 简单 |
| **JSON 文件操作** | 第三方系统、脚本自动化、CI/CD | 文件系统层 | ⭐⭐ 中等 |
| **编程级 API** | MindX 核心开发者、扩展功能 | Go 代码内部 | ⭐⭐⭐ 高级 |

**重要限制：**

> ⚠️ **Scheduler 实例是 MindX 内部对象，第三方代码无法直接获取。**
>
> 因此：
> - ❌ 不能直接调用 `scheduler.List()` 或 `scheduler.AddJob()`
> - ✅ 但可以直接读写 `<schedules-dir>/` 下的 JSON 文件
> - ✅ Scheduler 会自动检测文件变化并热更新（最多 5 秒延迟）

---

## 2. ScheduleEntry 数据模型

### 2.1 字段定义

每个定时任务对应一个 `ScheduleEntry` 对象，存储为独立的 JSON 文件：

```go
type ScheduleEntry struct {
    // --- 核心字段 ---
    ID        string    `json:"id"`         // 任务唯一标识符（8位 UUID）
    Agent     string    `json:"agent"`      // 目标智能体名称
    Content   string    `json:"content"`     // 要发送的消息内容
    CronExpr  string    `json:"cron_expr"`   // 调度规则（6位 Cron 表达式）
    Enabled   bool      `json:"enabled"`     // 是否启用

    // --- 时间戳 ---
    CreatedAt time.Time `json:"created_at"`  // 创建时间
    UpdatedAt time.Time `json:"updated_at"`  // 最后更新时间
    LastRunAt time.Time `json:"last_run_at,omitempty"` // 最后执行时间

    // --- 执行统计 ---
    LastRunID  string `json:"last_run_id,omitempty"`  // 最后执行的运行 ID
    LastStatus string `json:"last_status,omitempty"`  // 最后状态："success"/"failed"
    LastError  string `json:"last_error,omitempty"`   // 最后错误信息
    SuccessCnt int    `json:"success_count"`          // 成功次数
    FailureCnt int    `json:"failure_count"`          // 失败次数
}
```

**字段语义说明：**

| 字段 | 必填 | 说明 | 示例 |
|------|------|------|------|
| `ID` | 自动生成 | 8 位 UUID 截断 | `"a1b2c3d4"` |
| `Agent` | ✅ | 目标智能体名称 | `"assistant"` |
| `Content` | ✅ | 要发送的消息内容 | `"每日晨会提醒"` |
| `CronExpr` | ✅ | 6 位 Cron 表达式 | `"0 0 9 * * *"` |
| `Enabled` | 默认 true | 是否启用 | `true` |

### 2.2 JSON 存储格式

每个任务存储为一个独立文件：

```
<schedules-dir>/
├── a1b2c3d4.json
├── e5f6g7h8.json
└── i9j0k1l2.json
```

**文件内容示例：**

```json
{
  "id": "a1b2c3d4",
  "agent": "assistant",
  "content": "每日晨会提醒",
  "cron_expr": "0 0 9 * * *",
  "enabled": true,
  "created_at": "2026-05-06T09:00:00Z",
  "updated_at": "2026-05-06T09:00:00Z",
  "last_run_at": "2026-05-07T09:00:00Z",
  "last_run_id": "xyz123ab",
  "last_status": "success",
  "last_error": "",
  "success_count": 10,
  "failure_count": 0
}
```

---

## 4. 通过 WebSocket 使用 Scheduler

> **适用人群：** TUI 终端用户、Dashboard Web 用户、任何使用 MindX WebSocket 客户端的用户

### ⚠️ 重要认知纠正

**`/job-add`、`/job-list`、`/job-del` 不是 CLI 命令！**

它们是 **MindX WebSocket CMD 协议的指令**，通过 WebSocket 通道发送到服务端执行。

### 4.1 WebSocket CMD 协议简介

#### 协议格式

```
CMD|<command_name>|<arguments>|||
```

**组成部分：**

| 部分 | 说明 | 示例 |
|------|------|------|
| `CMD` | 固定的协议前缀 | `CMD` |
| `<command_name>` | 指令名称 | `job-add`, `job-list`, `job-del` |
| `<arguments>` | 指令参数（空格分隔的字符串） | `@assistant 提醒 expr="0 0 9 * * *"` |
| `\|\|\|` | 固定的结束标记 | `\|\|\|` |

#### 完整的消息流程

```
客户端 (TUI/Dashboard)                    服务端 (MindX Gateway)
        │                                        │
        │  ── WebSocket 连接已建立 ──→            │
        │                                        │
        │  发送: CMD|job-add|@assistant 测试 expr="0 0 9 * * *"||│
        │  ───────────────────────────────→       │
        │                                        │
        │                          解析指令参数    │
        │                          执行业务逻辑    │
        │                          写入 JSON 文件  │
        │                                        │
        │  ←── 返回: {"cmd":"CMD","name":"job-add",  │
        │          "data":"✅ 定时消息已创建..."} ──│
        │                                        │
```

#### 响应格式

成功响应：
```json
{
    "cmd": "CMD",
    "name": "job-add",
    "data": "✅ 定时消息已创建:\n  ID: a1b2c3d4\n  ..."
}
```

错误响应：
```json
{
    "cmd": "CMD",
    "name": "job-add",
    "error": "缺少目标智能体: 请使用 @<agent-name> 格式指定"
}
```

表格类型响应（如 job-list）：
```json
{
    "cmd": "CMD",
    "name": "job-list",
    "response_type": "table",
    "data": {
        "title": "定时消息任务列表",
        "headers": ["ID", "目标Agent", ...],
        "rows": [["a1b2c3", "@assistant", ...], ...]
    }
}
```

### 4.2 /job-add 创建定时消息

#### 指令语法

```
CMD|job-add|@<agent-name> <content> expr="<cron-expression>"|||
```

#### 参数说明

| 参数 | 必填 | 说明 | 示例 |
|------|------|------|------|
| `@<agent-name>` | ✅ | 目标智能体（@ 前缀） | `@assistant` |
| `<content>` | ✅ | 要发送的消息内容 | `每日晨会提醒` |
| `expr="<cron>"` | ✅ | Cron 表达式（引号包裹） | `expr="0 0 9 * * *"` |

#### 使用示例

**在 TUI 中使用：**
```
输入: /job-add @assistant 每日晨会提醒 expr="0 0 9 * * *"
```

**原始 WebSocket 消息：**
```
CMD|job-add|@assistant 每日晨会提醒 expr="0 0 9 * * *"|||
```

**更多示例：**

```bash
# 每小时同步数据
/job-add @data-worker 开始数据同步 expr="0 0 * * * *"

# 每周五下午生成周报
/job-add @reporter 本周工作总结时间到了 expr="0 0 18 * * 5"

# 每10分钟健康检查
/job-add @monitor 执行系统健康检查 expr="*/10 * * * * *"
```

#### 成功响应

```
✅ 定时消息已创建:
  ID: a1b2c3d4
  目标: @assistant
  内容: 每日晨会提醒
  调度: 0 0 9 * * *
```

#### 错误场景

```bash
# 缺少 Agent
/job-add 测试消息 expr="0 0 9 * * *"
❌ 错误: 缺少目标智能体: 请使用 @<agent-name> 格式指定
💡 示例: /job-add @assistant 每日提醒 expr="0 0 9 * * *"

# 缺少 Cron 表达式
/job-add @assistant 测试消息
❌ 错误: 缺少 cron 表达式: 请使用 expr="<cron表达式>" 指定

# 无效的 Cron 表达式
/job-add @assistant 测试 expr="invalid"
❌ 错误: 无效的 cron 表达式: ...
```

### 4.3 /job-list 查询任务列表

#### 指令语法

```
CMD|job-list|||
```

**注意：** 此指令不需要参数。

#### 使用示例

**在 TUI 中使用：**
```
输入: /job-list
```

**原始 WebSocket 消息：**
```
CMD|job-list|||
```

#### 响应格式（表格）

| ID | 目标Agent | 发送内容 | 调度规则 | 状态 | 成功/失败 |
|----|-----------|----------|----------|------|-----------|
| a1b2c3 | @assistant | 每日晨会提醒 | 0 0 9 \* \* \* | ✅ 启用 | 10/0 |
| d4e5f6 | @data-wk | 请开始数据同步 | 0 0 \* \* \* \* | ✅ 启用 | 5/1 |
| g7h8i9 | @reporter | 生成本周报告... | 0 0 18 \* \* 5 | ❌ 禁用 | 0/0 |

**空列表响应：**
```
(暂无定时消息任务)
```

### 4.4 /job-del 删除定时消息

#### 指令语法

```
CMD|job-del|id=<task-id>|||
```

#### 参数说明

| 参数 | 必填 | 说明 | 获取方式 |
|------|------|------|---------|
| `id=<task-id>` | ✅ | 任务 ID（8 位字符串） | 从 `/job-list` 输出获取 |

#### 使用示例

**典型工作流：**

```bash
# 步骤 1: 先查看所有任务
/job-list
# 输出:
# ID     目标Agent  发送内容        调度规则          状态
# a1b2c3 @assistant 每日晨会提醒   0 0 9 * * *      ✅ 启用

# 步骤 2: 删除指定任务
/job-del id=a1b2c3

# 步骤 3: 确认删除成功
/job-list
# (任务应不再显示)
```

**原始 WebSocket 消息：**
```
CMD|job-del|id=a1b2c3|||
```

#### 成功响应

```
🗑️ 定时消息已删除:
  ID: a1b2c3d4
  目标: @assistant
  内容: 每日晨会提醒
```

### 4.5 Cron 表达式速查

MindX Scheduler 使用 **6 位 Cron 表达式**（支持秒级精度）：

```
┌───────────── 秒 (0-59)
│ ┌──────────── 分钟 (0-59)
│ │ ┌────────── 小时 (0-23)
│ │ │ ┌──────── 月中的天 (1-31)
│ │ │ │ ┌────── 月 (1-12)
│ │ │ │ │ ┌──── 周中的天 (0-6, 0=周日)
* * * * * *
```

**常用示例：**

| 表达式 | 说明 |
|--------|------|
| `* * * * * *` | 每分钟 |
| `*/5 * * * * *` | 每 5 分钟 |
| `0 * * * * *` | 每小时整点 |
| `0 0 * * * *` | 每天 0:00 |
| `0 0 9 * * *` | 每天 9:00 |
| `0 0 9 * * 1-5` | 工作日 9:00 |
| `0 0 18 * * 5` | 每周五 18:00 |
| `0 0 0 1 * *` | 每月 1 号 0:00 |

> 💡 **在线验证工具：** https://cronitor.io/cron-expression-debugger （选择 6 位格式）

---

## 5. 第三方集成（JSON 文件操作）

> **适用人群：** 第三方系统开发者、运维脚本编写者、CI/CD 自动化
>
> **前提条件：** 能够访问 MindX 服务器的文件系统

### 5.1 为什么只能操作文件

**架构限制：**

```
┌─────────────────────────────────────────┐
│           MindX 应用进程                 │
│  ┌─────────────────────────────────┐   │
│  │  App                            │   │
│  │  └── scheduler (私有字段)       │   │  ← 外部无法访问
│  │      └── store (私有字段)       │   │
│  │         └── dataDir             │   │
│  └─────────────────────────────────┘   │
│                  │                     │
│                  ▼                     │
│  ┌─────────────────────────────────┐   │
│  │  <schedules-dir>/               │   │  ← 唯一的外部接口
│  │  ├── a1b2c3d4.json              │   │     （文件系统）
│  │  ├── e5f6g7h8.json              │   │
│  │  └── i9j0k1l2.json              │   │
│  └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

**原因：**
- `Scheduler` 和 `FileSchedulerStore` 是 `App` 结构体的私有字段
- 没有暴露 HTTP REST API 或 gRPC 接口
- 唯一的对外接口就是**文件系统上的 JSON 文件**

**好消息：**
- ✅ 可以直接读写这些 JSON 文件
- ✅ Scheduler 有热更新机制（每 5 秒检测文件变化）
- ✅ 操作简单，无需依赖任何 SDK

### 5.2 数据目录位置

**4-Layer Directory Architecture:**

```
Layer 1: HOME_DIR (Home Directory)
└── ~/.mindx/ (or $MINDX_HOME)
    ├── data/
    │   └── schedules/    ← Scheduled task data stored here (★ this section)
    ├── sessions/
    ├── settings/
    └── logs/

Layer 2: PROJECT_DIR (Project Directory) — captured at session start
Layer 3: SESSION_DIR (Session Sandbox) — per-conversation temporary files
Layer 4: SCRIPT_CWD (Execution Directory) — runtime script execution context
```

**默认路径：**

```bash
$HOME/.mindx/data/schedules/
# Equivalent to: ~/.mindx/data/schedules/
```

**路径解析链：**

```go
// Internal resolution (in MindX application layer):
Settings.SchedulesDir()
  → Settings.DataDir()       // $HOME/.mindx/data/
    → Settings.UserPreferences() // $HOME/.mindx/
      → filepath.Join("data", "schedules")
```

**检查实际路径：**

```bash
# 方法 1: 查看环境变量
echo $MINDX_HOME   # 或默认值: $HOME/.mindx
ls -la $HOME/.mindx/data/schedules/

# 方法 2: 查看日志中的初始化信息
grep -i "scheduler\|schedule.*dir" $HOME/.mindx/logs/*.log 2>/dev/null || echo "No logs found"
```

**目录权限：**

```bash
# 确保有读写权限
ls -ld <schedules-dir>
# 应该输出类似: drwxr-x--- 2 user group 4096 ...

# 如果没有权限，需要调整
chmod 750 <schedules-dir>
```

### 5.3 创建任务（写入 JSON）

#### 方法：创建新的 JSON 文件

**完整示例：**

```bash
#!/bin/bash

# 配置变量
SCHEDULES_DIR="/path/to/mindx/schedules"

# 生成任务 ID（可以使用 uuidgen 或自定义）
TASK_ID=$(uuidgen | cut -c1-8)

# 创建 JSON 文件
cat > "${SCHEDULES_DIR}/${TASK_ID}.json" << EOF
{
  "id": "${TASK_ID}",
  "agent": "assistant",
  "content": "每日晨会提醒",
  "cron_expr": "0 0 9 * * *",
  "enabled": true,
  "created_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "updated_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "success_count": 0,
  "failure_count": 0
}
EOF

echo "✅ 任务已创建: ${TASK_ID}"
echo "⏳ Scheduler 将在 5 秒内自动加载此任务"
```

**Python 示例：**

```python
import json
import os
from datetime import datetime, timezone
import uuid

def create_scheduled_task(
    schedules_dir: str,
    agent: str,
    content: str,
    cron_expr: str,
    enabled: bool = True
) -> str:
    """
    创建一个定时任务
    
    Args:
        schedules_dir: schedules 目录路径
        agent: 目标智能体名称
        content: 要发送的内容
        cron_expr: Cron 表达式
        enabled: 是否启用
        
    Returns:
        任务 ID
    """
    task_id = uuid.uuid4().hex[:8]
    
    entry = {
        "id": task_id,
        "agent": agent,
        "content": content,
        "cron_expr": cron_expr,
        "enabled": enabled,
        "created_at": datetime.now(timezone.utc).isoformat(),
        "updated_at": datetime.now(timezone.utc).isoformat(),
        "success_count": 0,
        "failure_count": 0
    }
    
    file_path = os.path.join(schedules_dir, f"{task_id}.json")
    
    with open(file_path, 'w', encoding='utf-8') as f:
        json.dump(entry, f, indent=2, ensure_ascii=False)
    
    print(f"✅ 任务已创建: {task_id}")
    print(f"📁 文件位置: {file_path}")
    
    return task_id


# 使用示例
if __name__ == "__main__":
    SCHEDULES_DIR = "/path/to/mindx/schedules"
    
    create_scheduled_task(
        schedules_dir=SCHEDULES_DIR,
        agent="assistant",
        content="每日晨会提醒",
        cron_expr="0 0 9 * * *"
    )
```

### 5.4 查询任务（读取 JSON）

#### 方法：遍历目录读取所有 .json 文件

**Bash 示例：**

```bash
#!/bin/bash

SCHEDULES_DIR="/path/to/mindx/schedules"

echo "📋 当前所有定时任务:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
printf "%-10s %-15s %-30s %-20s %s\n" "ID" "AGENT" "CONTENT" "CRON" "STATUS"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

for json_file in "${SCHEDULES_DIR}"/*.json; do
    if [ -f "$json_file" ]; then
        # 使用 jq 解析（如果可用）
        if command -v jq &> /dev/null; then
            id=$(jq -r '.id' "$json_file")
            agent=$(jq -r '.agent' "$json_file")
            content=$(jq -r '.content' "$json_file" | cut -c1-28)
            cron=$(jq -r '.cron_expr' "$json_file")
            enabled=$(jq -r '.enabled' "$json_file")
            
            if [ "$enabled" = "true" ]; then
                status="✅ ON"
            else
                status="❌ OFF"
            fi
            
            printf "%-10s %-15s %-30s %-20s %s\n" "$id" "@$agent" "$content" "$cron" "$status"
        else
            # 没有 jq 时，只显示文件名
            filename=$(basename "$json_file" .json)
            echo "  📄 $filename.json"
        fi
    fi
done

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
```

**Python 示例：**

```python
import json
import os
from typing import List, Dict, Any

def list_all_tasks(schedules_dir: str) -> List[Dict[str, Any]]:
    """
    列出所有定时任务
    
    Args:
        schedules_dir: schedules 目录路径
        
    Returns:
        任务列表
    """
    tasks = []
    
    for filename in os.listdir(schedules_dir):
        if filename.endswith('.json'):
            filepath = os.path.join(schedules_dir, filename)
            
            with open(filepath, 'r', encoding='utf-8') as f:
                try:
                    task = json.load(f)
                    tasks.append(task)
                except json.JSONDecodeError as e:
                    print(f"⚠️ 无法解析 {filename}: {e}")
    
    return tasks


def format_task_table(tasks: List[Dict[str, Any]]) -> str:
    """
    格式化任务列表为表格
    """
    if not tasks:
        return "(暂无定时消息任务)"
    
    lines = []
    lines.append("╔═════════╦══════════╦═══════════════════════════╦═══════════════╦═══════╗")
    lines.append("║   ID    ║  AGENT   ║       CONTENT          ║   CRON EXPR ║ STATUS ║")
    lines.append("╠═════════╬══════════╬═══════════════════════════╬═══════════════╬═══════╣")
    
    for task in tasks:
        task_id = task.get('id', 'N/A')
        agent = f"@{task.get('agent', 'N/A')}"
        content = task.get('content', 'N/A')[:27]
        cron = task.get('cron_expr', 'N/A')
        enabled = task.get('enabled', False)
        
        status = "✅ ON" if enabled else "❌ OFF"
        
        lines.append(
            f"║ {task_id:^7} ║ {agent:^8} ║ {content:^27} ║ {cron:^11} ║ {status:^5} ║"
        )
    
    lines.append("╚═════════╩══════════╩═══════════════════════════╩═══════════════╩═══════╝")
    lines.append(f"\n共 {len(tasks)} 个定时消息任务")
    
    return '\n'.join(lines)


# 使用示例
if __name__ == "__main__":
    SCHEDULES_DIR = "/path/to/mindx/schedules"
    
    tasks = list_all_tasks(SCHEDULES_DIR)
    print(format_task_table(tasks))
```

### 5.5 更新任务（修改 JSON）

#### 方法：直接修改 JSON 文件的字段

**场景 1：禁用/启用任务**

```bash
#!/bin/bash

SCHEDULES_DIR="/path/to/mindx/schedules"
TASK_ID="a1b2c3d4"
FILE_PATH="${SCHEDULES_DIR}/${TASK_ID}.json"

if [ ! -f "$FILE_PATH" ]; then
    echo "❌ 任务不存在: ${TASK_ID}"
    exit 1
fi

# 使用 jq 修改 enabled 字段
if command -v jq &> /dev/null; then
    # 禁用任务
    jq '.enabled = false' "$FILE_PATH" > "${FILE_PATH}.tmp" && mv "${FILE_PATH}.tmp" "$FILE_PATH"
    
    echo "✅ 任务已禁用: ${TASK_ID}"
    echo "⏳ Scheduler 将在 5 秒内生效"
else
    echo "❌ 需要安装 jq 工具"
    exit 1
fi
```

**场景 2：修改消息内容或调度规则**

```python
import json

def update_task(
    schedules_dir: str,
    task_id: str,
    new_content: str = None,
    new_cron_expr: str = None,
    new_enabled: bool = None
) -> bool:
    """
    更新任务属性
    
    Args:
        schedules_dir: schedules 目录
        task_id: 任务 ID
        new_content: 新的内容（可选）
        new_cron_expr: 新的 Cron 表达式（可选）
        new_enabled: 新的启用状态（可选）
        
    Returns:
        是否成功
    """
    from datetime import datetime, timezone
    
    file_path = os.path.join(schedules_dir, f"{task_id}.json")
    
    if not os.path.exists(file_path):
        print(f"❌ 任务不存在: {task_id}")
        return False
    
    with open(file_path, 'r', encoding='utf-8') as f:
        task = json.load(f)
    
    # 更新字段
    if new_content is not None:
        task['content'] = new_content
    
    if new_cron_expr is not None:
        task['cron_expr'] = new_cron_expr
    
    if new_enabled is not None:
        task['enabled'] = new_enabled
    
    # 更新时间戳
    task['updated_at'] = datetime.now(timezone.utc).isoformat()
    
    # 原子性写入
    tmp_path = f"{file_path}.tmp"
    with open(tmp_path, 'w', encoding='utf-8') as f:
        json.dump(task, f, indent=2, ensure_ascii=False)
    
    os.replace(tmp_path, file_path)
    
    print(f"✅ 任务已更新: {task_id}")
    return True


# 使用示例
update_task(
    schedules_dir="/path/to/schedules",
    task_id="a1b2c3d4",
    new_content="更新后的消息内容",
    new_enabled=True
)
```

### 5.6 删除任务（删除 JSON）

#### 方法：删除对应的 JSON 文件

**Bash 示例：**

```bash
#!/bin/bash

SCHEDULES_DIR="/path/to/mindx/schedules"
TASK_ID="a1b2c3d4"
FILE_PATH="${SCHEDULES_DIR}/${TASK_ID}.json"

if [ ! -f "$FILE_PATH" ]; then
    echo "❌ 任务不存在: ${TASK_ID}"
    exit 1
fi

# 删除文件
rm "$FILE_PATH"

echo "🗑️ 任务已删除: ${TASK_ID}"
echo "⏳ Scheduler 将在 5 秒内从内存中移除此任务"
```

**Python 示例：**

```python
import os

def delete_task(schedules_dir: str, task_id: str) -> bool:
    """
    删除任务
    
    Args:
        schedules_dir: schedules 目录
        task_id: 任务 ID
        
    Returns:
        是否成功
    """
    file_path = os.path.join(schedules_dir, f"{task_id}.json")
    
    if not os.path.exists(file_path):
        print(f"❌ 任务不存在: {task_id}")
        return False
    
    os.remove(file_path)
    print(f"🗑️ 任务已删除: {task_id}")
    return True


# 使用示例
delete_task("/path/to/schedules", "a1b2c3d4")
```

### 5.7 热更新机制

**工作原理：**

MindX Scheduler 内置了一个**文件监听循环**，每 **5 秒**扫描一次 schedules 目录：

```
Scheduler.watchLoop()
    │
    └── Ticker (每 5 秒)
            │
            └── reloadAll()
                ├── List(): 读取所有 JSON 文件
                ├── 对比内存中的 entries
                ├── 新增文件 → addJob()     → 注册到 Cron 引擎
                ├── 缺失文件 → removeJob()  → 从 Cron 引擎移除
                └── 已禁用文件 → removeJob()
```

**对第三方集成的意义：**

| 操作 | 生效时间 | 说明 |
|------|---------|------|
| 创建新文件 | 最多 5 秒 | Scheduler 自动检测并加载 |
| 修改现有文件 | 最多 5 秒 | Scheduler 自动重新加载 |
| 删除文件 | 最多 5 秒 | Scheduler 自动从内存移除 |
| 禁用任务 | 最多 5 秒 | 设置 `enabled: false` 即可 |

**注意事项：**

- ⚠️ 延迟最多 5 秒（取决于 Ticker 间隔）
- ⚠️ 文件必须是有效的 JSON 格式
- ⚠️ 必须包含必填字段（id, agent, content, cron_expr）
- ✅ 建议使用原子性写入（先写临时文件，再 rename）

### 5.8 完整示例：Shell 脚本管理工具

```bash
#!/bin/bash
#
# MindX Scheduler 管理工具（Shell 版）
# 用于通过 JSON 文件管理定时任务
#

set -e

# ====== 配置 ======
SCHEDULES_DIR="${MINDX_SCHEDULER_DIR:-/var/lib/mindx/schedules}"

# ====== 工具函数 ======

usage() {
    cat << EOF
MindX Scheduler 管理工具

用法: $0 <command> [options]

命令:
  create   创建新的定时任务
  list     列出所有任务
  show     显示任务详情
  update   更新任务属性
  enable   启用任务
  disable  禁用任务
  delete   删除任务

示例:
  $0 create --agent assistant --content "每日提醒" --cron "0 0 9 * * *"
  $0 list
  $0 show <task-id>
  $0 update <task-id> --content "新内容"
  $0 disable <task-id>
  $0 delete <task-id>

环境变量:
  MINDX_SCHEDULER_DIR  Schedules 目录路径 (默认: /var/lib/mindx/schedules)
EOF
}

generate_id() {
    if command -v uuidgen &> /dev/null; then
        uuidgen | cut -c1-8
    else
        cat /proc/sys/kernel/random/uuid | cut -c1-8
    fi
}

check_jq() {
    if ! command -v jq &> /dev/null; then
        echo "❌ 此脚本需要 jq 工具"
        echo "   安装方法: brew install jq (macOS) / apt install jq (Linux)"
        exit 1
    fi
}

# ====== 命令实现 ======

cmd_create() {
    check_jq
    
    local agent="" content="" cron_expr=""
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --agent) agent="$2"; shift 2 ;;
            --content) content="$2"; shift 2 ;;
            --cron) cron_expr="$2"; shift 2 ;;
            *) echo "未知选项: $1"; exit 1 ;;
        esac
    done
    
    if [[ -z "$agent" || -z "$content" || -z "$cron_expr" ]]; then
        echo "❌ 缺少必要参数"
        echo "   用法: $0 create --agent <name> --content <text> --cron <expr>"
        exit 1
    fi
    
    local task_id=$(generate_id)
    local now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    
    local entry=$(cat << EOF
{
  "id": "${task_id}",
  "agent": "${agent}",
  "content": $(echo "$content" | jq -Rs .),
  "cron_expr": "${cron_expr}",
  "enabled": true,
  "created_at": "${now}",
  "updated_at": "${now}",
  "success_count": 0,
  "failure_count": 0
}
EOF
)
    
    local file_path="${SCHEDULES_DIR}/${task_id}.json"
    echo "$entry" > "$file_path"
    
    echo "✅ 任务已创建:"
    echo "   ID:      ${task_id}"
    echo "   Agent:   @${agent}"
    echo "   Content: ${content}"
    echo "   Cron:    ${cron_expr}"
    echo "   File:    ${file_path}"
    echo ""
    echo "⏳ Scheduler 将在 5 秒内自动加载"
}

cmd_list() {
    check_jq
    
    local count=0
    
    echo "📋 定时消息任务列表"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    printf "%-10s %-12s %-35s %-18s %-6s %s\n" \
        "ID" "AGENT" "CONTENT" "CRON" "STAT" "S/F"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    for json_file in "${SCHEDULES_DIR}"/*.json; do
        if [[ -f "$json_file" ]]; then
            local id=$(jq -r '.id' "$json_file")
            local agent=$(jq -r '.agent' "$json_file")
            local content=$(jq -r '.content' "$json_file" | cut -c1-33)
            local cron=$(jq -r '.cron_expr' "$json_file")
            local enabled=$(jq -r '.enabled' "$json_file")
            local success=$(jq -r '.success_count' "$json_file")
            local failure=$(jq -r '.failure_count' "$json_file")
            
            local status="✅"
            [[ "$enabled" != "true" ]] && status="❌"
            
            printf "%-10s @%-11s %-35s %-18s %-6s %s/%s\n" \
                "$id" "$agent" "$content" "$cron" "$status" "$success" "$failure"
            
            ((count++))
        fi
    done
    
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "共 ${count} 个任务"
}

cmd_show() {
    check_jq
    
    local task_id="$1"
    [[ -z "$task_id" ]] && { echo "❌ 缺少任务 ID"; exit 1; }
    
    local file_path="${SCHEDULES_DIR}/${task_id}.json"
    
    if [[ ! -f "$file_path" ]]; then
        echo "❌ 任务不存在: ${task_id}"
        exit 1
    fi
    
    echo "📄 任务详情: ${task_id}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    jq '.' "$file_path"
}

cmd_update() {
    check_jq
    
    local task_id="$1"; shift
    [[ -z "$task_id" ]] && { echo "❌ 缺少任务 ID"; exit 1; }
    
    local file_path="${SCHEDULES_DIR}/${task_id}.json"
    
    if [[ ! -f "$file_path" ]]; then
        echo "❌ 任务不存在: ${task_id}"
        exit 1
    fi
    
    local updates=""
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --content) 
                updates+=" | .content = $(echo "$2" | jq -Rs .)"
                shift 2 
                ;;
            --cron) 
                updates+=" | .cron_expr = \"$2\""
                shift 2 
                ;;
            --agent) 
                updates+=" | .agent = \"$2\""
                shift 2 
                ;;
            *) echo "未知选项: $1"; exit 1 ;;
        esac
    done
    
    if [[ -z "$updates" ]]; then
        echo "❌ 未指定要更新的字段"
        exit 1
    fi
    
    local now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    eval "jq '.updated_at = \"${now}\" ${updates}' \"${file_path}\" > \"${file_path}.tmp\""
    mv "${file_path}.tmp" "$file_path"
    
    echo "✅ 任务已更新: ${task_id}"
    echo "⏳ 变更将在 5 秒内生效"
}

cmd_enable() {
    check_jq
    
    local task_id="$1"
    [[ -z "$task_id" ]] && { echo "❌ 缺少任务 ID"; exit 1; }
    
    local file_path="${SCHEDULES_DIR}/${task_id}.json"}
    
    if [[ ! -f "$file_path" ]]; then
        echo "❌ 任务不存在: ${task_id}"
        exit 1
    fi
    
    local now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    jq ".enabled = true | .updated_at = \"${now}\"" "$file_path" > "${file_path}.tmp"
    mv "${file_path}.tmp" "$file_path"
    
    echo "✅ 任务已启用: ${task_id}"
}

cmd_disable() {
    check_jq
    
    local task_id="$1"
    [[ -z "$task_id" ]] && { echo "❌ 缺少任务 ID"; exit 1; }
    
    local file_path="${SCHEDULES_DIR}/${task_id}.json"
    
    if [[ ! -f "$file_path" ]]; then
        echo "❌ 任务不存在: ${task_id}"
        exit 1
    fi
    
    local now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    jq ".enabled = false | .updated_at = \"${now}\"" "$file_path" > "${file_path}.tmp"
    mv "${file_path}.tmp" "$file_path"
    
    echo "⏸️  任务已禁用: ${task_id}"
}

cmd_delete() {
    local task_id="$1"
    [[ -z "$task_id" ]] && { echo "❌ 缺少任务 ID"; exit 1; }
    
    local file_path="${SCHEDULES_DIR}/${task_id}.json"
    
    if [[ ! -f "$file_path" ]]; then
        echo "❌ 任务不存在: ${task_id}"
        exit 1
    fi
    
    rm "$file_path"
    
    echo "🗑️ 任务已删除: ${task_id}"
}

# ====== 主入口 ======

case "${1:-}" in
    create)  shift; cmd_create "$@" ;;
    list)    cmd_list ;;
    show)    shift; cmd_show "$@" ;;
    update)  shift; cmd_update "$@" ;;
    enable)  shift; cmd_enable "$@" ;;
    disable) shift; cmd_disable "$@" ;;
    delete)  shift; cmd_delete "$@" ;;
    -h|--help|*) usage ;;
esac
```

### 5.9 完整示例：Python 脚本管理工具

```python
#!/usr/bin/env python3
"""
MindX Scheduler 管理工具（Python 版）
用于通过 JSON 文件管理定时任务

用法:
    python scheduler_tool.py create --agent assistant --content "提醒" --cron "0 0 9 * * *"
    python scheduler_tool.py list
    python scheduler_tool.py show <task-id>
    python scheduler_tool.py update <task-id> --content "新内容"
    python scheduler_tool.py enable <task-id>
    python scheduler_tool.py disable <task-id>
    python scheduler_tool.py delete <task-id>
"""

import argparse
import json
import os
import sys
from datetime import datetime, timezone
from typing import List, Dict, Any, Optional
import uuid


class SchedulerManager:
    """MindX Scheduler 管理器（通过文件操作）"""
    
    def __init__(self, schedules_dir: str):
        self.schedules_dir = schedules_dir
        os.makedirs(schedules_dir, exist_ok=True)
    
    def _get_file_path(self, task_id: str) -> str:
        return os.path.join(self.schedules_dir, f"{task_id}.json")
    
    def _load_task(self, task_id: str) -> Optional[Dict[str, Any]]:
        file_path = self._get_file_path(task_id)
        if not os.path.exists(file_path):
            return None
        
        with open(file_path, 'r', encoding='utf-8') as f:
            return json.load(f)
    
    def _save_task(self, task: Dict[str, Any]) -> None:
        task_id = task['id']
        file_path = self._get_file_path(task_id)
        
        task['updated_at'] = datetime.now(timezone.utc).isoformat()
        
        tmp_path = f"{file_path}.tmp"
        with open(tmp_path, 'w', encoding='utf-8') as f:
            json.dump(task, f, indent=2, ensure_ascii=False)
        
        os.replace(tmp_path, file_path)
    
    def create(self, agent: str, content: str, cron_expr: str,
               enabled: bool = True) -> Dict[str, Any]:
        """创建新任务"""
        task_id = uuid.uuid4().hex[:8]
        now = datetime.now(timezone.utc).isoformat()
        
        task = {
            'id': task_id,
            'agent': agent,
            'content': content,
            'cron_expr': cron_expr,
            'enabled': enabled,
            'created_at': now,
            'updated_at': now,
            'success_count': 0,
            'failure_count': 0
        }
        
        self._save_task(task)
        
        print(f"✅ 任务已创建:")
        print(f"   ID:      {task_id}")
        print(f"   Agent:   @{agent}")
        print(f"   Content: {content}")
        print(f"   Cron:    {cron_expr}")
        print(f"   ⏳ Scheduler 将在 5 秒内自动加载")
        
        return task
    
    def list_all(self) -> List[Dict[str, Any]]:
        """列出所有任务"""
        tasks = []
        
        for filename in os.listdir(self.schedules_dir):
            if filename.endswith('.json'):
                file_path = os.path.join(self.schedules_dir, filename)
                try:
                    with open(file_path, 'r', encoding='utf-8') as f:
                        task = json.load(f)
                        tasks.append(task)
                except json.JSONDecodeError as e:
                    print(f"⚠️ 无法解析 {filename}: {e}", file=sys.stderr)
        
        return tasks
    
    def show(self, task_id: str) -> Optional[Dict[str, Any]]:
        """显示任务详情"""
        task = self._load_task(task_id)
        
        if task is None:
            print(f"❌ 任务不存在: {task_id}")
            return None
        
        print(f"📄 任务详情: {task_id}")
        print("=" * 60)
        for key, value in task.items():
            print(f"  {key:15}: {value}")
        
        return task
    
    def update(self, task_id: str, **kwargs) -> Optional[Dict[str, Any]]:
        """更新任务"""
        task = self._load_task(task_id)
        
        if task is None:
            print(f"❌ 任务不存在: {task_id}")
            return None
        
        valid_fields = {'content', 'cron_expr', 'agent', 'enabled'}
        
        for key, value in kwargs.items():
            if key in valid_fields:
                task[key] = value
            else:
                print(f"⚠️ 忽略无效字段: {key}")
        
        self._save_task(task)
        
        print(f"✅ 任务已更新: {task_id}")
        print(f"   ⏳ 变更将在 5 秒内生效")
        
        return task
    
    def enable(self, task_id: str) -> bool:
        """启用任务"""
        return self.update(task_id, enabled=True) is not None
    
    def disable(self, task_id: str) -> bool:
        """禁用任务"""
        return self.update(task_id, enabled=False) is not None
    
    def delete(self, task_id: str) -> bool:
        """删除任务"""
        file_path = self._get_file_path(task_id)
        
        if not os.path.exists(file_path):
            print(f"❌ 任务不存在: {task_id}")
            return False
        
        os.remove(file_path)
        print(f"🗑️ 任务已删除: {task_id}")
        return True


def format_table(tasks: List[Dict[str, Any]]) -> str:
    """格式化为表格"""
    if not tasks:
        return "\n(暂无定时消息任务)\n"
    
    header = f"{'ID':^10} {'AGENT':^12} {'CONTENT':^33} {'CRON':^18} {'STAT':^6} {'S/F':^5}"
    separator = "-" * len(header)
    
    lines = [header, separator]
    
    for task in tasks:
        task_id = task.get('id', 'N/A')
        agent = f"@{task.get('agent', 'N/A')}"
        content = str(task.get('content', 'N/A'))[:31]
        cron = task.get('cron_expr', 'N/A')
        enabled = task.get('enabled', False)
        success = task.get('success_count', 0)
        failure = task.get('failure_count', 0)
        
        status = "✅" if enabled else "❌"
        
        line = f"{task_id:^10} {agent:^12} {content:^33} {cron:^18} {status:^6} {success}/{failure}"
        lines.append(line)
    
    lines.append(separator)
    lines.append(f"\n共 {len(tasks)} 个任务")
    
    return '\n'.join(lines)


def main():
    parser = argparse.ArgumentParser(
        description='MindX Scheduler 管理工具',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  %(prog)s create --agent assistant --content "每日提醒" --cron "0 0 9 * * *"
  %(prog)s list
  %(prog)s show abc12345
  %(prog)s update abc12345 --content "新内容"
  %(prog)s enable abc12345
  %(prog)s disable abc12345
  %(prog)s delete abc12345

环境变量:
  MINDX_SCHEDULER_DIR  Schedules 目录路径
        """
    )
    
    parser.add_argument(
        '--dir',
        default=os.environ.get('MINDX_SCHEDULER_DIR', '/var/lib/mindx/schedules'),
        help='Schedules 目录路径'
    )
    
    subparsers = parser.add_subparsers(dest='command', help='可用命令')
    
    # create 命令
    create_parser = subparsers.add_parser('create', help='创建新任务')
    create_parser.add_argument('--agent', required=True, help='目标智能体')
    create_parser.add_argument('--content', required=True, help='消息内容')
    create_parser.add_argument('--cron', required=True, help='Cron 表达式')
    create_parser.add_argument('--disabled', action='store_true', help='创建后即禁用')
    
    # list 命令
    subparsers.add_parser('list', help='列出所有任务')
    
    # show 命令
    show_parser = subparsers.add_parser('show', help='显示任务详情')
    show_parser.add_argument('task_id', help='任务 ID')
    
    # update 命令
    update_parser = subparsers.add_parser('update', help='更新任务')
    update_parser.add_argument('task_id', help='任务 ID')
    update_parser.add_argument('--content', help='新内容')
    update_parser.add_argument('--cron', help='新 Cron 表达式')
    update_parser.add_argument('--agent', help='新目标智能体')
    
    # enable/disable 命令
    enable_parser = subparsers.add_parser('enable', help='启用任务')
    enable_parser.add_argument('task_id', help='任务 ID')
    
    disable_parser = subparsers.add_parser('disable', help='禁用任务')
    disable_parser.add_argument('task_id', help='任务 ID')
    
    # delete 命令
    delete_parser = subparsers.add_parser('delete', help='删除任务')
    delete_parser.add_argument('task_id', help='任务 ID')
    
    args = parser.parse_args()
    
    if not args.command:
        parser.print_help()
        sys.exit(1)
    
    manager = SchedulerManager(args.dir)
    
    if args.command == 'create':
        manager.create(
            agent=args.agent,
            content=args.content,
            cron_expr=args.cron,
            enabled=not args.disabled
        )
    
    elif args.command == 'list':
        tasks = manager.list_all()
        print(format_table(tasks))
    
    elif args.command == 'show':
        manager.show(args.task_id)
    
    elif args.command == 'update':
        kwargs = {}
        if args.content: kwargs['content'] = args.content
        if args.cron: kwargs['cron_expr'] = args.cron
        if args.agent: kwargs['agent'] = args.agent
        manager.update(args.task_id, **kwargs)
    
    elif args.command == 'enable':
        manager.enable(args.task_id)
    
    elif args.command == 'disable':
        manager.disable(args.task_id)
    
    elif args.command == 'delete':
        manager.delete(args.task_id)


if __name__ == '__main__':
    main()
```

---

## 6. 编程级 API 参考（仅限核心开发者）

> **适用人群：** 正在修改 MindX 源码的开发者、需要扩展 Scheduler 功能的开发者
>
> **前置知识：** 熟悉 Go 语言、了解 MindX 架构

### 6.1 Scheduler 核心组件

#### 组件关系图

```
App (internal/svc/app.go)
  │
  ├── scheduler: *scheduler.Scheduler        (私有字段)
  │     ├── cron: *cron.Cron                 (Cron 引擎)
  │     ├── store: *FileSchedulerStore       (存储后端)
  │     ├── executor: CommandExecutor        (执行函数)
  │     └── entries: map[string]cron.EntryID (内存索引)
  │
  └── schedulerDB: *scheduler.FileSchedulerStore  (公开方法)
        ├── Save(ctx, entry)
        ├── Load(ctx, id)
        ├── Delete(ctx, id)
        ├── List(ctx)
        └── UpdateLastRun(id, runID, err)
```

#### 关键代码位置

| 组件 | 文件路径 | 行号范围 |
|------|---------|---------|
| ScheduleEntry 定义 | `pkg/scheduler/store.go` | L14-L30 |
| FileSchedulerStore | `pkg/scheduler/store.go` | L32-L183 |
| Scheduler 核心 | `pkg/scheduler/scheduler.go` | L17-L154 |
| CommandExecutor 类型 | `pkg/scheduler/scheduler.go` | L15 |
| App 集成 | `internal/svc/app.go` | L37-L38, L97, L126-L132 |
| 命令注册 | `internal/svc/commands.go` | L78-L88 |
| 命令实现 | `internal/svc/commands.go` | L184-L270 |

### 6.2 FileSchedulerStore 接口

**定义位置：** `pkg/scheduler/store.go`

#### 公开方法

##### `NewFileSchedulerStore(dataDir string) (*FileSchedulerStore, error)`

创建存储实例。

```go
store, err := scheduler.NewFileSchedulerStore("./data/schedules")
```

##### `(s *FileSchedulerStore) Save(ctx context.Context, entry *ScheduleEntry) error`

保存或更新任务。

**特性：**
- 自动设置 `created_at`（首次创建时）和 `updated_at`
- 原子性写入（临时文件 + rename）

##### `(s *FileSchedulerStore) Load(ctx context.Context, id string) (*ScheduleEntry, error)`

根据 ID 加载任务。

**向后兼容：** 可自动将旧格式（含 `command` 字段）迁移为新格式（`content` 字段）。

##### `(s *FileSchedulerStore) Delete(ctx context.Context, id string) error`

删除任务。幂等操作（删除不存在的任务不会报错）。

##### `(s *FileSchedulerStore) List(ctx context.Context) ([]ScheduleEntry, error)`

列出所有任务（按 ID 排序）。自动跳过损坏的 JSON 文件。

##### `(s *FileSchedulerStore) UpdateLastRun(id string, runID string, execErr error) error`

更新执行记录。由 Scheduler 内部调用，通常不需要手动调用。

### 6.3 CommandExecutor 接口

**定义位置：** `pkg/scheduler/scheduler.go:L15`

```go
type CommandExecutor func(ctx context.Context, agent string, content string) error
```

**参数说明：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `ctx` | `context.Context` | 上下文（含超时控制，默认 5 分钟） |
| `agent` | `string` | 目标智能体名称 |
| `content` | `string` | 要发送的消息内容 |

**返回值：**
- `error`: 执行是否成功（nil 表示成功）

**默认实现（App 层）：**

```go
// internal/svc/app.go:L134-L149
func (a *App) executeScheduleCommand(
    ctx context.Context,
    agent string,
    content string,
) error {
    resolvedAgent, err := a.resolveAgent(agent)
    if err != nil {
        return fmt.Errorf("resolve agent %q: %w", agent, err)
    }
    
    sessionID := fmt.Sprintf("sched_%s_%s", agent,
        time.Now().Format("20060102"))
    
    _, err = resolvedAgent.Ask(sessionID, content)
    if err != nil {
        return fmt.Errorf("execute scheduled message for @%s: %w", agent, err)
    }
    return nil
}
```

### 6.4 数据流与生命周期

#### 任务创建流程

```
1. WebSocket 接收 CMD 消息
   CMD|job-add|@assistant 内容 expr="..."|||

2. Gateway 解析指令
   → 提取 command name: "job-add"
   → 提取 arguments: "@assistant 内容 expr=\"...\""

3. 调用 CommandHandler (commands.go:jobAddCommand)
   → parseJobAddArgs() 解析参数
   → 验证参数合法性
   → 构建 ScheduleEntry

4. 调用 app.SchedulerDB().Save(entry)
   → 写入 JSON 文件
   → 原子性操作 (tmp + rename)

5. 返回响应给客户端
   {"cmd":"CMD","name":"job-add","data":"✅ ..."}

6. (异步) Scheduler 热更新
   → watchLoop 检测到新文件 (≤5s)
   → reloadAll() 加载任务
   → addJob() 注册到 Cron 引擎
   → 等待触发时间
```

#### 任务执行流程

```
1. Cron 引擎触发 (匹配当前时间)
   → 调用回调函数 s.executeJob(entry)

2. 生成运行 ID
   runID = uuid.New().String()[:8]

3. 设置超时上下文 (5分钟)
   ctx, cancel := context.WithTimeout(...)

4. 调用执行器
   execErr = s.executor(ctx, entry.Agent, entry.Content)
   → App.executeScheduleCommand(ctx, agent, content)
   → resolveAgent(agent) → 获取 Agent 实例
   → resolvedAgent.Ask(sessionID, content) → 发送消息

5. 更新执行记录
   s.store.UpdateLastRun(id, runID, execErr)
   → 更新 JSON 文件中的统计字段

6. 记录日志
   INFO/ERROR 级别日志
```

#### 热更新机制详解

```
Scheduler.Start(ctx)
  │
  ├── 1. reloadAll()  // 初始加载
  │       └── store.List() → 读取所有 JSON
  │       └── 对每个启用的任务 → addJob()
  │
  ├── 2. cron.Start()  // 启动 Cron 引擎
  │
  └── 3. go watchLoop(ctx)  // 启动监听循环
          │
          └── Ticker (每 5 秒)
                  │
                  └── reloadAll()
                      ├── List() 重新扫描文件
                      ├── 对比 entries map
                      ├── 新文件 → addJob()
                      ├── 缺失文件 → removeJob()
                      └── 已禁用 → removeJob()
```

**关键点：**
- 所有文件 I/O 都通过 `FileSchedulerStore` 完成
- 内存状态 (`entries map`) 与文件系统保持最终一致
- 最大延迟 5 秒（Ticker 间隔）
- 幂等操作：重复添加同一任务不会报错

### 6.5 扩展开发指南

#### 场景 1: 添加新的 CMD 指令

**示例：添加 `/job-enable` 和 `/job-disable` 指令**

**步骤 1:** 在 `commands.go` 中注册新命令

```go
func RegisterBuiltinCommands(gw *gateway.Server, app *App) {
    // ... 已有的命令 ...
    
    gw.RegisterCommand("job-enable", func(ctx *gateway.CommandContext) (any, error) {
        return jobEnableCommand(app, ctx)
    }, "启用定时消息")

    gw.RegisterCommand("job-disable", func(ctx *gateway.CommandContext) (any, error) {
        return jobDisableCommand(app, ctx)
    }, "禁用定时消息")
}
```

**步骤 2:** 实现处理函数

```go
func jobEnableCommand(app *App, ctx *gateway.CommandContext) (any, error) {
    args := parseCommandArgs(ctx.Args)
    id := args["id"]
    
    if id == "" {
        return nil, fmt.Errorf("缺少参数: id\n用法: /job-enable id=<任务ID>")
    }
    
    entry, err := app.SchedulerDB().Load(context.Background(), id)
    if err != nil {
        return nil, fmt.Errorf("任务不存在: %s", id)
    }
    
    if entry.Enabled {
        return fmt.Sprintf("ℹ️  任务已是启用状态: %s", id), nil
    }
    
    entry.Enabled = true
    if err := app.SchedulerDB().Save(context.Background(), entry); err != nil {
        return nil, fmt.Errorf("更新失败: %w", err)
    }
    
    return fmt.Sprintf("✅ 任务已启用: %s (5秒内生效)", id), nil
}
```

#### 场景 2: 自定义执行逻辑

如果需要修改任务执行时的行为（例如添加重试、日志增强等），可以实现自定义的 `CommandExecutor`：

```go
// 在 app.go 中
func (a *App) initScheduler() {
    // 自定义执行器：带重试和详细日志
    customExecutor := func(ctx context.Context, agent, content string) error {
        log.Printf("[Scheduler] Executing job for @%s: %s", agent, content)
        
        var lastErr error
        maxRetries := 3
        
        for attempt := 1; attempt <= maxRetries; attempt++ {
            err := a.executeScheduleCommand(ctx, agent, content)
            if err == nil {
                log.Printf("[Scheduler] Job succeeded for @%s (attempt %d)", agent, attempt)
                return nil
            }
            
            lastErr = err
            log.Printf("[Scheduler] Job failed for @%s (attempt %d): %v", agent, attempt, err)
            
            if attempt < maxRetries {
                select {
                case <-ctx.Done():
                    return ctx.Err()
                case <-time.After(5 * time.Second):
                    continue
                }
            }
        }
        
        return fmt.Errorf("after %d attempts: %w", maxRetries, lastErr)
    }
    
    a.scheduler = scheduler.NewScheduler(a.schedulerDB, customExecutor)
}
```

#### 场景 3: 实现监控端点

如果需要通过 HTTP API 暴露任务状态（供监控系统消费）：

```go
// 在 handler.go 或单独的 HTTP handler 文件中
func (a *App) handleSchedulerStats(w http.ResponseWriter, r *http.Request) {
    entries, err := a.SchedulerDB().List(r.Context())
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    
    stats := struct {
        TotalJobs    int     `json:"total_jobs"`
        ActiveJobs   int     `json:"active_jobs"`
        DisabledJobs int     `json:"disabled_jobs"`
        TotalRuns    int     `json:"total_runs"`
        SuccessRate  float64 `json:"success_rate"`
    }{}
    
    for _, e := range entries {
        stats.TotalJobs++
        if e.Enabled {
            stats.ActiveJobs++
        } else {
            stats.DisabledJobs++
        }
        stats.TotalRuns += e.SuccessCnt + e.FailureCnt
    }
    
    if stats.TotalRuns > 0 {
        totalSuccess := sum(e.SuccessCnt for e in entries)
        stats.SuccessRate = float64(totalSuccess) / float64(stats.TotalRuns) * 100
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(stats)
}
```

---

## 7. 最佳实践与故障排查

### 7.1 最佳实践

#### ✅ 推荐做法

**1. 使用明确的任务描述**

```json
// 好：清晰明确
{"content": "请生成今日销售数据分析报告"}

// 差：模糊不清
{"content": "做那个事情"}
```

**2. 合理设置频率**

```json
// 好：适当频率
{"cron_expr": "0 0 */6 * * *"}  // 每6小时

// 差：过于频繁
{"cron_expr": "* * * * * *"}   // 每秒（可能压垮 Agent）
```

**3. 利用 Agent 专业性**

```json
// 好：专业分工
{"agent": "analyst", "content": "分析今日销售趋势"}
{"agent": "writer", "content": "起草周报草稿"}

// 差：全部发给同一个 Agent
{"agent": "assistant", "content": "分析数据、写报告、发邮件"}
```

**4. 使用原子性文件操作**

```python
# 好：原子性写入
with open(tmp_path, 'w') as f:
    json.dump(data, f)
os.replace(tmp_path, final_path)  # 原子操作

# 差：直接覆盖
with open(final_path, 'w') as f:
    json.dump(data, f)  # 如果中途崩溃，文件可能损坏
```

#### ❌ 避免的做法

**1. 不要在内容中包含敏感信息**

```json
// 危险
{"content": "使用密码 123456 登录数据库"}

// 安全
{"content": "请登录数据库并执行备份任务"}
```

**2. 不要创建冲突的任务**

```json
// 冲突：两个任务几乎同时触发同一个 Agent
{"cron_expr": "0 0 9 * * *", "agent": "assistant", "..."}
{"cron_expr": "0 1 9 * * *", "agent": "assistant", "..."}  // 仅差1秒
```

### 7.2 故障排查

#### 常见问题

**问题 1: 创建的任务没有按时触发**

排查步骤：

```bash
# 1. 检查文件是否存在
ls -la <schedules-dir>/<task-id>.json

# 2. 检查文件格式是否正确
cat <schedules-dir>/<task-id>.json> | python -m json.tool

# 3. 检查 enabled 字段
jq '.enabled' <schedules-dir>/<task-id>.json>

# 4. 检查 Cron 表达式
# 使用在线工具验证: https://cronitor.io/cron-expression-debugger

# 5. 检查 Scheduler 日志
grep "scheduler" /var/log/mindx.log
```

**问题 2: 任务执行失败，FailureCnt 不断增加**

排查步骤：

```bash
# 1. 查看任务的错误信息
jq '{last_status, last_error, failure_count}' <schedules-dir>/<task-id>.json

# 2. 检查 Agent 是否存在
# 查看 runtime/agents/ 目录下是否有对应配置

# 3. 检查网络连接
# Agent 可能需要调用外部 LLM API

# 4. 检查日志
grep "schedule job failed" /var/log/mindx.log
```

**问题 3: 修改 JSON 后行为未改变**

原因：热更新有最多 5 秒延迟

解决方法：
- 等待 5-10 秒
- 或重启 MindX 服务（不推荐）

#### 日志关键词

| 日志模式 | 含义 | 处理建议 |
|---------|------|---------|
| `added schedule job` | 任务已注册到 Cron | 正常 |
| `removed schedule job` | 任务已从 Cron 移除 | 正常 |
| `executing schedule job` | 任务正在执行 | 观察 |
| `schedule job completed` | 执行成功 | ✅ 无需处理 |
| `schedule job failed` | 执行失败 | ❌ 需要调查 |
| `failed to add schedule job` | 注册失败（通常 Cron 表达式无效） | 检查表达式 |
| `scheduler reload failed` | 热更新失败 | 检查文件权限 |

---

## 8. 附录

### A. 相关文件索引

| 文件 | 路径 | 说明 |
|------|------|------|
| 数据模型 | `pkg/scheduler/store.go` | ScheduleEntry 定义 + Store 实现 |
| 调度引擎 | `pkg/scheduler/scheduler.go` | Scheduler 核心逻辑 |
| 命令处理 | `internal/svc/commands.go` | CMD 指令实现 |
| 应用集成 | `internal/svc/app.go` | App 层集成 |
| 设置配置 | `internal/svc/settings.go` | 目录路径配置 |
| WS 客户端库 | `gort/pkg/gateway/client.go` | WebSocket CMD 协议实现 |
| 本文档 | `docs/SCHEDULER-GUIDE.md` | 你正在阅读的文件 |

### B. 术语表

| 术语 | 定义 |
|------|------|
| ScheduleEntry | 定时任务的数据模型 |
| Cron Expression | 时间调度表达式（6位，支持秒） |
| CommandExecutor | 任务执行函数类型（签名：func(ctx, agent, content) error） |
| FileSchedulerStore | 基于文件的存储后端（JSON 格式） |
| Hot Reload | 热更新机制（无需重启即可生效） |
| WebSocket CMD Protocol | MindX 的指令协议（格式：CMD\|name\|args\|\|\|） |

### C. 向后兼容性

**旧格式支持：**

Scheduler 的 `unmarshalEntry()` 函数可以自动将旧版 JSON 文件迁移为新格式：

```json
// 旧格式（v1.x）
{
  "id": "xxx",
  "name": "@worker: 同步",
  "command": "请同步数据库",    // ← 旧字段
  "args": "--full",            // ← 旧字段（会被丢弃）
  "agent": "worker",
  ...
}

// 自动迁移后（读取时）
{
  "id": "xxx",
  "agent": "worker",
  "content": "请同步数据库",    // ← 从 command 迁移
  // name 和 args 字段被忽略
  ...
}
```

**注意：** 保存时会使用新格式（`content` 字段），不会保留旧的 `command` 和 `args` 字段。

### D. 版本历史

| 版本 | 日期 | 变更说明 |
|------|------|---------|
| v1.0 | 2026-05-06 | 初始版本，基础 CRUD 功能 |
| v2.0 | 2026-05-06 | 重构数据模型：删除 Name/Args，Command→Content；修正文档定位 |

### E. 快速参考卡

#### WebSocket CMD 指令速查

```bash
# 创建任务
CMD|job-add|@<agent> <content> expr="<cron>"|||

# 列出任务
CMD|job-list|||

# 删除任务
CMD|job-del|id=<task-id>|||
```

#### JSON 文件操作速查

```bash
# 创建
echo '{...}' > <schedules-dir>/<id>.json

# 查询
cat <schedules-dir>/<id>.json | jq .

# 更新
jq '.enabled = false' <file> > <file>.tmp && mv <file>.tmp <file>

# 删除
rm <schedules-dir>/<id>.json
```

#### 常用 Cron 表达式

```cron
每分钟:        * * * * * *
每小时:        0 0 * * * *
每天 9:00:     0 0 9 * * *
工作日 9:00:    0 0 9 * * 1-5
每周五 18:00:   0 0 18 * * 5
每月1号 0:00:   0 0 0 1 * *
```

---

**文档版本:** v2.0  
**最后更新:** 2026-05-06  
**维护者:** MindX 开发团队