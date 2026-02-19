# MindX CLI 命令参考

MindX 提供完整的命令行工具，用于服务管理、模型测试、技能管理和模型训练等功能。

## 命令概览

```
mindx [command] [subcommand] [flags]
```

| 命令              | 说明             |
| ----------------- | ---------------- |
| `mindx`           | 显示帮助信息     |
| `mindx version`   | 显示版本信息     |
| `mindx dashboard` | 打开 Web 控制台  |
| `mindx tui`       | 启动终端聊天界面 |
| `mindx kernel`    | 服务控制命令     |
| `mindx model`     | 模型管理和测试   |
| `mindx skill`     | 技能管理         |
| `mindx train`     | 模型训练         |

---

## mindx version

显示 MindX 版本信息。

```bash
mindx version
```

**输出示例**：

```
MindX version: 1.0.0
Build time:   2024-01-15T10:30:00Z
Git commit:   abc1234
```

**说明**：
- 显示当前安装的 MindX 版本号
- 显示构建时间和 Git 提交哈希（如果可用）

---

## mindx dashboard

打开 Web 控制台。

```bash
mindx dashboard
```

**说明**：
- 自动打开浏览器访问 `http://localhost:911`
- 需要先启动 kernel 服务

**依赖**：
- kernel 服务必须运行中
- Dashboard 前端已构建

---

## mindx tui

启动终端聊天界面，通过 WebSocket 连接到主服务进行交互。

```bash
mindx tui [flags]
```

**参数**：

| 参数        | 简写 | 默认值 | 说明               |
| ----------- | ---- | ------ | ------------------ |
| `--port`    | `-p` | 1314   | WebSocket 服务端口 |
| `--session` | `-s` | 空     | 会话 ID（可选）    |

**示例**：

```bash
# 使用默认端口连接
mindx tui

# 指定端口
mindx tui --port 8080

# 指定会话 ID
mindx tui --session abc123
```

**操作说明**：

| 按键             | 功能     |
| ---------------- | -------- |
| `Enter`          | 发送消息 |
| `Ctrl+C` / `Esc` | 退出     |
| `Backspace`      | 删除字符 |

**依赖**：
- kernel 服务必须运行中
- WebSocket 服务已启动

---

## mindx kernel

服务控制命令，管理 MindX 服务的启动、停止和状态。

### mindx kernel run

启动 MindX kernel 主进程，阻塞式运行。

```bash
mindx kernel run
```

**说明**：
- 启动后会阻塞运行，监听系统信号
- 收到 `SIGINT`、`SIGTERM`、`SIGQUIT` 信号时优雅关闭
- macOS 下会自动加载 plist 服务文件

**环境变量**：

| 变量                | 说明                      |
| ------------------- | ------------------------- |
| `BOT_DEV_MODE=true` | 开发模式，跳过 plist 加载 |

### mindx kernel start

通过系统服务控制命令启动服务。

```bash
mindx kernel start
```

**说明**：
- macOS: 使用 `launchctl`
- Linux: 使用 `systemctl`
- Windows: 使用 `sc`

### mindx kernel stop

停止服务。

```bash
mindx kernel stop
```

**说明**：
- 发送系统信号触发服务优雅关闭
- 跨平台支持

### mindx kernel restart

重启服务。

```bash
mindx kernel restart
```

**说明**：
- 先执行 stop，再执行 start
- 等待服务完全停止后再启动

### mindx kernel status

查看服务状态。

```bash
mindx kernel status
```

**输出示例**：

```
服务状态: 运行中
```

**状态说明**：

| 状态   | 说明         |
| ------ | ------------ |
| 运行中 | 服务正常运行 |
| 已停止 | 服务已停止   |
| 未安装 | 服务未安装   |

---

## mindx model

模型管理和测试。

### mindx model test

测试模型是否支持函数工具。

```bash
mindx model test [model-name]
```

**参数**：

| 参数         | 说明                       |
| ------------ | -------------------------- |
| `model-name` | 可选，指定要测试的模型名称 |

**示例**：

```bash
# 测试配置中的所有模型
mindx model test

# 测试特定模型
mindx model test qwen3:1.7b
```

**说明**：
- 测试模型是否支持 Function Calling
- 验证模型配置是否正确
- 检查 API 连接是否正常

---

## mindx skill

技能管理命令。

### mindx skill list

列出所有已安装的技能。

```bash
mindx skill list [flags]
```

**参数**：

| 参数         | 简写 | 默认值 | 说明       |
| ------------ | ---- | ------ | ---------- |
| `--category` | `-c` | 空     | 按分类过滤 |

**示例**：

```bash
# 列出所有技能
mindx skill list

# 按分类过滤
mindx skill list --category general
```

**输出说明**：

| 字段         | 说明             |
| ------------ | ---------------- |
| 技能名       | 技能的唯一标识   |
| 版本         | 技能版本号       |
| 描述         | 技能功能描述     |
| 标签         | 技能标签         |
| 缺失二进制   | 缺少的可执行文件 |
| 缺失环境变量 | 缺少的环境变量   |
| 成功/错误    | 执行统计         |
| 平均耗时     | 平均执行时间     |

### mindx skill run

执行指定技能。

```bash
mindx skill run <name> [flags]
```

**参数**：

| 参数   | 说明             |
| ------ | ---------------- |
| `name` | 技能名称（必填） |

**示例**：

```bash
# 执行技能
mindx skill run github --repo owner/repo --pr 55
```

### mindx skill validate

验证技能配置和依赖。

```bash
mindx skill validate <name>
```

**参数**：

| 参数   | 说明             |
| ------ | ---------------- |
| `name` | 技能名称（必填） |

**示例**：

```bash
mindx skill validate github
```

**输出说明**：

| 字段         | 说明             |
| ------------ | ---------------- |
| 已启用       | 技能是否启用     |
| 可运行       | 技能是否可以执行 |
| 缺失二进制   | 缺少的可执行文件 |
| 缺失环境变量 | 缺少的环境变量   |
| 最后错误     | 最近一次执行错误 |

### mindx skill enable

启用技能。

```bash
mindx skill enable <name>
```

**示例**：

```bash
mindx skill enable github
```

### mindx skill disable

禁用技能。

```bash
mindx skill disable <name>
```

**示例**：

```bash
mindx skill disable github
```

### mindx skill reload

重新加载所有技能。

```bash
mindx skill reload
```

**说明**：
- 重新扫描技能目录
- 重新加载技能配置
- 更新技能索引

---

## mindx train

模型训练命令，基于记忆系统中的数据创建个性化模型。

### 基本用法

```bash
mindx train --run-once [flags]
```

**参数**：

| 参数             | 默认值                 | 说明                             |
| ---------------- | ---------------------- | -------------------------------- |
| `--run-once`     | false                  | 执行一次训练后退出（必须）       |
| `--mode`         | message                | 训练模式：message 或 lora        |
| `--model`        | qwen3:0.6b             | 基础模型名称                     |
| `--min-corpus`   | 50                     | 最小训练语料量                   |
| `--data-dir`     | data                   | 数据目录                         |
| `--config`       | config/models.json     | 模型配置文件路径                 |
| `--training-dir` | training               | Python 微调脚本目录（lora 模式） |
| `--workspace`    | 当前目录               | 工作目录                         |
| `--ollama`       | http://localhost:11434 | Ollama 服务地址                  |

### 训练模式

#### Message 模式（默认）

通过 Modelfile 的 MESSAGE 指令注入对话历史。

```bash
mindx train --run-once
```

**特点**：
- 速度快（秒级完成）
- 无需额外依赖
- 效果为上下文记忆

#### LoRA 模式

通过 Python 脚本进行真正的 LoRA 微调。

```bash
mindx train --run-once --mode lora
```

**特点**：
- 效果持久，改变模型行为
- 需要 Python 环境
- CPU 训练速度较慢

**前置条件**：
```bash
cd training
./setup.sh
```

### 示例

```bash
# 消息注入模式（默认，快速）
mindx train --run-once

# LoRA 微调模式
mindx train --run-once --mode lora

# 自定义基础模型
mindx train --run-once --model qwen3:1.7b

# 指定最小语料量
mindx train --run-once --min-corpus 100

# 完整参数
mindx train --run-once \
    --mode lora \
    --model qwen3:0.6b \
    --min-corpus 50 \
    --data-dir data \
    --config config/models.json
```

### 训练报告

训练完成后会输出训练报告：

```
========== 训练报告 ==========
训练ID: train_20260214_030000
状态: success
训练模式: message
基础模型: qwen3:0.6b
新模型: qwen3:0.6b-personal
原始语料: 150 条
筛选后: 120 条
耗时: 2.5s
基础模型分数: 0.72
新模型分数: 0.85
效果提升: true
==============================
```

---

## 常见使用场景

### 启动服务

```bash
# 前台运行（开发调试）
mindx kernel run

# 后台服务（生产环境）
mindx kernel start
```

### 日常交互

```bash
# Web 界面
mindx dashboard

# 终端界面
mindx tui
```

### 模型管理

```bash
# 测试模型
mindx model test

# 训练个性化模型
mindx train --run-once
```

### 技能管理

```bash
# 查看所有技能
mindx skill list

# 验证技能
mindx skill validate github

# 启用/禁用技能
mindx skill enable github
mindx skill disable github
```

### 服务管理

```bash
# 查看状态
mindx kernel status

# 重启服务
mindx kernel restart

# 停止服务
mindx kernel stop
```

---

## 环境变量

| 变量               | 默认值            | 说明         |
| ------------------ | ----------------- | ------------ |
| `MINDX_WORKSPACE`  | `~/.mindx`        | 工作目录路径 |
| `MINDX_SKILLS_DIR` | `~/.mindx/skills` | 技能目录路径 |
| `BOT_DEV_MODE`     | 空                | 开发模式标志 |

---

## 配置文件

配置文件位于 `~/.mindx/config/` 目录：

| 文件                | 说明     |
| ------------------- | -------- |
| `models.json`       | 模型配置 |
| `capabilities.json` | 能力配置 |
| `channels.json`     | 通道配置 |
| `general.json`      | 通用配置 |

---

## 日志位置

| 日志     | 路径                                       |
| -------- | ------------------------------------------ |
| 对话日志 | `~/.mindx/logs/YYYY/MM/DD/conv_*.json`     |
| 监控日志 | `~/.mindx/logs/monitor/monitor_*.log`      |
| 系统日志 | `/tmp/mindx.out.log`、`/tmp/mindx.err.log` |
