# MindX 安装部署指南

## 系统要求

- **Go**: 1.21 或更高版本
- **Node.js**: 18 或更高版本（仅用于构建 Dashboard）
- **Ollama**: 必选（用于本地模型推理）
- **操作系统**: Linux / macOS / Windows

## 快速开始

### 1. 克隆项目
```bash
git clone <repository-url>
cd mindx
```

### 2. 查看可用命令
```bash
make help
```

### 3. 构建和安装
```bash
# 构建（前端 + 后端）
make build

# 安装到系统（会提示选择工作目录）
make install
```

### 4. 检查环境（可选但推荐）
```bash
# 运行环境检查，诊断潜在问题
make doctor
```

### 5. 运行
```bash
# 启动 Dashboard
make run

# 或启动开发模式（后端 + Vite 前端）
make dev
```

### 6. 后续更新（如需）
```bash
# 一键更新到最新版本
make update
```

---

## 目录结构

```
$MINDX_INSTALL_PATH/          # 安装目录（默认：/usr/local/mindx）
├── bin/
│   └── mindx              # Go 二进制文件
├── static/                # Dashboard 静态文件
├── skills/                # 内置技能
├── config/                # 配置模板
└── mindx                 # 符号链接（指向 bin/mindx）

$MINDX_WORKSPACE/             # 工作目录（默认：~/.mindx）
├── config/                 # 配置文件目录
│   ├── server.yml         # 服务器配置
│   ├── models.json         # 模型配置
│   ├── capabilities.json    # 能力配置
│   └── channels.json      # 渠道配置
├── data/                  # 数据存储目录
│   ├── memory/            # 记忆数据
│   ├── sessions/          # 会话数据
│   ├── vectors/           # 向量索引
│   └── training/          # 训练数据
└── logs/                  # 日志目录
```

---

## 安装步骤

### 通用步骤（所有平台）

1. **查看帮助**
   ```bash
   make help
   ```

2. **构建项目**
   ```bash
   make build
   ```

3. **安装到系统**
   
   运行安装脚本时，会提示选择工作目录：
   ```bash
   make install
   ```
   
   安装过程中会出现以下选项：
   ```
   Please choose your workspace directory:
   
     1) Default: ~/.mindx
     2) Custom directory
   
   Enter your choice (1 or 2): 
   ```
   
   - 选择 **1** 使用默认工作目录 `~/.mindx`
   - 选择 **2** 输入自定义工作目录路径

4. **验证安装**
   ```bash
   # 查看版本
   mindx --version
   
   # 测试模型
   mindx model test
   
   # 列出技能
   mindx skill list
   ```

5. **启动服务**
   ```bash
   # 方式1: 使用 make（推荐）
   make run
   
   # 方式2: 直接使用 CLI
   mindx dashboard
   ```

### Linux 安装

1. **安装依赖**
   ```bash
   # Ubuntu/Debian
   sudo apt-get update
   sudo apt-get install -y golang nodejs npm

   # Fedora/RHEL
   sudo dnf install -y golang nodejs npm

   # Arch Linux
   sudo pacman -S go nodejs npm
   ```

2. **构建和安装**
   ```bash
   make build
   make install
   ```

3. **启动 Kernel 服务（可选，用于后台运行）**
   ```bash
   mindx kernel start
   ```

### macOS 安装

1. **安装依赖**
   ```bash
   # 使用 Homebrew
   brew install go node
   ```

2. **构建和安装**
   ```bash
   make build
   make install
   ```

### Windows 安装

1. **安装依赖**
   
   下载并安装：
   - [Go](https://golang.org/dl/)
   - [Node.js](https://nodejs.org/)

2. **构建和安装**
   ```bash
   make build
   make install
   ```

---

## 开发模式

```bash
# 启动开发环境（后端 + Vite 前端热重载）
make dev
```

开发模式会：
- 使用 **`.dev` 目录** 作为临时工作目录（不会污染用户实际工作区）
- 启动后端服务（端口 911）
- 启动 Vite 开发服务器（端口 5173）
- 支持前端代码热重载

访问：
- 前端开发界面：http://localhost:5173
- 后端 API：http://localhost:911

**临时目录说明：**
- `.dev/` - 开发模式使用的临时工作目录
- `.test/` - 测试使用的临时工作目录
- 这些目录已添加到 `.gitignore`，不会被提交到 git
- 运行 `make clean` 会清理这些临时目录

---

## Makefile 命令参考

### 核心命令
| 命令             | 说明                                 |
| ---------------- | ------------------------------------ |
| `make build`     | 构建 MindX（前端 + 后端）            |
| `make install`   | 安装到系统（交互式选择工作目录）     |
| `make update`    | 更新到最新版本                       |
| `make uninstall` | 从系统卸载                           |
| `make run`       | 启动 Dashboard                       |
| `make dev`       | 启动开发模式（使用 `.dev` 临时目录） |
| `make clean`     | 清理构建产物（含临时目录）           |
| `make test`      | 运行测试（使用 `.test` 临时目录）    |
| `make doctor`    | 检查环境问题                         |
| `make help`      | 显示帮助                             |

### 构建相关
| 命令                  | 说明         |
| --------------------- | ------------ |
| `make build-frontend` | 仅构建前端   |
| `make build-backend`  | 仅构建后端   |
| `make build-all`      | 构建所有平台 |

### 运行相关
| 命令                  | 说明             |
| --------------------- | ---------------- |
| `make run-dashboard`  | 启动 Dashboard   |
| `make run-tui`        | 启动 TUI 聊天    |
| `make run-kernel`     | 启动 Kernel 服务 |
| `make run-train`      | 运行一次训练     |
| `make run-model-test` | 测试模型兼容性   |
| `make run-skill-list` | 列出所有技能     |

### 开发辅助
| 命令           | 说明         |
| -------------- | ------------ |
| `make fmt`     | 格式化代码   |
| `make lint`    | 代码检查     |
| `make deps`    | 更新依赖     |
| `make version` | 显示版本信息 |

---

## CLI 命令参考

### Dashboard 和 Web UI
```bash
mindx dashboard              # 打开 Dashboard
```

### TUI 终端聊天
```bash
mindx tui                    # 启动终端聊天界面
mindx tui -p 8080           # 指定端口
mindx tui -s my-session      # 指定会话 ID
```

### 模型管理
```bash
mindx model test              # 测试所有模型兼容性
mindx model test qwen3:1.7b   # 测试指定模型
```

### Kernel 服务管理
```bash
mindx kernel run              # 运行 Kernel 服务（阻塞式）
mindx kernel start            # 启动 Kernel 系统服务
mindx kernel stop             # 停止 Kernel 系统服务
mindx kernel restart          # 重启 Kernel 系统服务
mindx kernel status           # 查看 Kernel 服务状态
```

### 训练系统
```bash
mindx train                  # 启动训练守护进程
mindx train --run-once        # 运行一次训练
```

### 技能管理
```bash
mindx skill list             # 列出所有技能
mindx skill run github       # 运行指定技能
mindx skill validate weather  # 验证指定技能
mindx skill enable github     # 启用指定技能
mindx skill disable github    # 禁用指定技能
mindx skill reload           # 重新加载所有技能
```

---

## 配置说明

### 环境变量

| 变量名            | 说明         | 默认值             |
| ----------------- | ------------ | ------------------ |
| `MINDX_PATH`      | 安装目录路径 | `/usr/local/mindx` |
| `MINDX_WORKSPACE` | 工作目录路径 | `~/.mindx`         |

### 配置文件

所有配置文件位于 `$MINDX_WORKSPACE/config/` 目录：

- **server.yml**: 服务器配置
- **models.json**: 模型配置
- **capabilities.json**: 能力配置
- **channels.json**: 渠道配置

---

## 卸载步骤

### 使用 Makefile（推荐）
```bash
make uninstall
```

### 手动卸载

#### Linux / macOS
```bash
# 删除符号链接
sudo rm /usr/local/bin/mindx

# 删除安装目录
sudo rm -rf /usr/local/mindx

# 删除工作目录（可选）
rm -rf ~/.mindx
```

#### Windows
```cmd
# 删除安装目录
rmdir /s /q "C:\Program Files\MindX"

# 删除工作目录（可选）
rmdir /s /q "%USERPROFILE%\.mindx"
```

---

## 环境检查与故障排查

### 使用 `make doctor` 检查环境

```bash
make doctor
```

`make doctor` 会检查以下项目：

| 检查项目    | 说明                                 |
| ----------- | ------------------------------------ |
| 系统依赖    | Go、Node.js、Ollama                  |
| Ollama 模型 | qwen3:0.6b、qwen3:1.7b、bge-small-zh |
| 安装状态    | mindx 是否在 PATH、安装目录是否存在  |
| 工作区状态  | 配置文件、数据目录是否存在           |
| 权限        | 工作区是否可写                       |
| 端口        | 911、1314 端口是否可用               |

检查完成后会显示：
- ✅ Passed: 通过的检查数量
- ⚠ Warnings: 警告数量
- ✗ Errors: 错误数量

并给出具体的修复建议。

---

## 验证安装

```bash
# 查看帮助
make help

# 检查版本
make version

# 测试模型
mindx model test

# 启动 Dashboard
make run

# 访问 Web 界面
# 打开浏览器访问 http://localhost:911
```

---

## 故障排查

### 常见问题

1. **端口被占用**
   - Dashboard 默认端口: 911
   - WebSocket 默认端口: 1314
   - 修改端口: 编辑 `$MINDX_WORKSPACE/config/server.yml`

2. **权限问题**
   - 确保工作目录有读写权限
   - Linux/macOS: `chmod -R 755 ~/.mindx`
   - Windows: 以管理员身份运行

3. **模型连接失败**
   - 检查 API 密钥是否正确
   - 检查网络连接
   - 检查 `base_url` 是否正确

4. **Dashboard 静态文件找不到**
   - 确保已运行 `make build`
   - 开发模式使用 `make dev`

### 日志位置

- **系统日志**: `$MINDX_WORKSPACE/logs/system.log`
- **对话日志**: `$MINDX_WORKSPACE/logs/YYYY/MM/DD/`

---

## 更新

### 使用 `make update`（推荐）

一键更新到最新版本：

```bash
make update
```

`make update` 会自动执行以下步骤：
1. **git pull** - 拉取最新代码
2. **make build** - 重新构建
3. **make install** - 重新安装

**重要说明：**
- ✅ 用户工作区文件（配置、数据、日志）会被完整保留
- ✅ 不会删除任何用户数据
- ✅ 配置文件只会在不存在时创建新模板

### 手动更新

如果需要手动执行每个步骤：

```bash
# 1. 拉取最新代码
git pull

# 2. 重新构建
make build

# 3. 重新安装
make install
```

---

## 获取帮助

```bash
# Makefile 帮助
make help

# CLI 帮助
mindx --help
mindx dashboard --help
mindx tui --help
mindx kernel --help
mindx model --help
mindx train --help
mindx skill --help
```
