# 服务生命周期与安装

用于安装、升级、运行和维护 MindX 守护进程服务的命令。
其中大部分命令支持离线使用（无需守护进程）。

## 安装与设置

| 任务 | 命令 | 说明 |
|------|------|------|
| 全新安装 | `mindx install` | 复制二进制文件、配置 PATH、注册系统服务 |
| 不安装守护进程 | `mindx install --no-daemon` | 适用于容器或使用自定义进程管理的服务器 |
| 跳过 PATH 配置 | `mindx install --no-path` | PATH 已配置好时使用 |
| 不创建桌面快捷方式 | `mindx install --no-shortcut` | 无图形界面环境 |
| 自定义安装目录 | `mindx install --dir /opt/mindx` | 非默认安装位置 |
| 强制复制二进制文件 | `mindx install --force-copy` | 即使从包管理器管理的位置也强制复制 |
| **完全卸载** | `mindx uninstall` | 停止守护进程、移除服务、清理 PATH、删除二进制文件、移除快捷方式 |
| 卸载（保留二进制文件） | `mindx uninstall --keep-binary` | 仅移除集成项，保留二进制文件 |
| 跳过守护进程清理 | `mindx uninstall --no-daemon` | 卸载时不处理守护进程服务 |
| 跳过 PATH 清理 | `mindx uninstall --no-path` | 保持 PATH 不变 |
| 跳过快捷方式移除 | `mindx uninstall --no-shortcut` | 保留桌面快捷方式 |
| 查看版本 | `mindx version` | 显示构建信息、Go 运行时、平台信息 |
| 检查更新 | `mindx upgrade --check` | 试运行 —— 不实际安装 |
| 升级到最新版 | `mindx upgrade` | 从 GitHub 下载并安装 |
| 健康诊断 | `mindx doctor` | 检查配置、路径、权限、连通性 |
| 自动修复问题 | `mindx doctor -f` | 尝试修复检测到的问题 |
| 打开 WebUI | `mindx web` | 在默认端口 :1313 打开浏览器；需要守护进程提供 UI 服务 |
| 自定义端口打开 WebUI | `mindx web -p :8080` | 覆盖默认端口 |

## macOS 应用包

| 任务 | 命令 | 说明 |
|------|------|------|
| 创建 .app 应用包 | `mindx app create` | 在 /Applications 中生成带嵌入图标的 .app |
| 自定义输出路径 | `mindx app create -o ~/Desktop` | 覆盖目标路径 |
| 导出图标 | `mindx app icon ./icon.png` | 提取嵌入的应用图标 |

## Shell 自动补全

为主流 Shell 生成 Tab 补全脚本。

| 任务 | 命令 | 说明 |
|------|------|------|
| Bash 补全 | `mindx completion bash` | 输出到 `/etc/bash_completion.d/mindx` 或 `$(brew --prefix)/etc/bash_completion.d/mindx` |
| Zsh 补全 | `mindx completion zsh` | 输出到 `${fpath[1]}/_mindx` 或 `$(brew --prefix)/share/zsh/site-functions/_mindx` |
| Fish 补全 | `mindx completion fish` | 输出到 `~/.config/fish/completions/mindx.fish` |
| PowerShell 补全 | `mindx completion powershell` | 通过管道写入 profile |
| 禁用描述信息 | `mindx completion bash --no-descriptions` | 生成的脚本中不包含命令描述 |

## 守护进程服务管理

| 任务 | 命令 | 说明 |
|------|------|------|
| 启动守护进程 | `mindx start` | 通过系统服务管理器启动（launchctl/systemd/schtasks） |
| 停止守护进程 | `mindx stop` | 优雅关闭 |
| 重启守护进程 | `mindx restart` | 配置变更或升级后使用 |
| 检查状态 | `mindx status` | 显示二进制文件路径、配置、守护进程状态、平台信息 |
| 重新加载 Agent | `mindx reload agents` | 热重载 Agent 配置，无需完全重启 |
| 重新加载 Skill | `mindx reload skills` | 热重载 Skill 配置，无需完全重启 |

### 直接运行守护进程（开发模式）

| 任务 | 命令 | 说明 |
|------|------|------|
| 直接运行守护进程 | `mindx daemon` | 前台进程 —— 仅用于开发/容器环境 |
| 自定义 WebSocket 端口 | `mindx daemon -p :1314` | 默认端口 :1314 |
| 自定义 WebSocket 路径 | `mindx daemon --path /ws` | 默认 WebSocket 端点路径 |
| 查看守护进程版本 | `mindx daemon version` | 服务端版本信息 |
| 以 JSON 查看守护进程版本 | `mindx daemon version --json` | 机器可读输出 |
| 检查守护进程更新 | `mindx daemon check-update` | 服务端自更新检查 |
| 应用守护进程更新 | `mindx daemon apply-update` | 热重载新二进制文件 |
| 从内部重启 | `mindx daemon restart` | 进程内重启 |
| 查看守护进程配置 | `mindx daemon config` | 显示当前生效的配置 |
| 以 JSON 查看守护进程配置 | `mindx daemon config --json` | 机器可读输出 |

> **重要提示**：生产环境请使用 `mindx start/stop/restart`。`mindx daemon` 仅用于开发环境或自行管理进程的容器化环境。

## 日志

| 任务 | 命令 | 说明 |
|------|------|------|
| 查看最近日志 | `mindx logs -n 50` | 所有日志文件的最后 50 行 |
| 实时跟踪日志 | `mindx logs -f` | 跟踪模式（类似 tail -f） |
| 检查的日志文件： | `daemon.log`、`daemon.err`、`mindx.log` | — |

### 守护进程日志 API（需要守护进程）

| 任务 | 命令 | 说明 |
|------|------|------|
| 分页读取（最新在前） | `mindx log read --limit 30` | 按时间倒序 |
| 仅读取错误流 | `mindx log read --limit 30 --stream error` | 按流过滤 |
| 从偏移量读取 | `mindx log read --offset 100 --limit 30` | 用于分页 |
| 通过 API 流式读取 | `mindx log read --stream main --limit 10` | 通过守护进程实时跟踪 |
| 清除所有日志 | `mindx log clear --confirm` | **破坏性操作** —— 布尔标志，必须确认才能清除 |
| 统计日志条目数 | `mindx log count` | 按流分类统计 |

## 常见工作流

### 首次安装
```bash
mindx install
mindx doctor
mindx status
mindx web
```

### 升级后
```bash
mindx upgrade --check
mindx upgrade
mindx restart
mindx doctor
```

### 故障排查
```bash
mindx version
mindx status
mindx doctor -f
mindx logs -n 50
mindx log read --limit 30 --stream error
```

### 完全移除
```bash
mindx uninstall              # 移除所有系统集成
rm -rf ~/.mindx             # 可选：移除所有数据（日志、Session、配置）
```

### 开发模式
```bash
mindx daemon --port :1314
# 在另一个终端：
mindx logs -f
```
