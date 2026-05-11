# MindX 项目"工作目录"概念引用归档

> 生成时间: 2026-05-12
> 搜索范围: /Users/ray/workspaces/ai-ecosystem/mindx
> 涉及文件总数: 约 40+ 个文件
> 总引用数量: 200+ 处

---

## 一、中文直接提及 - "工作目录"

### 1. TODO.md - 核心设计思路（最关键）
**文件位置**: `TODO.md:41-55`

```markdown
- [x] 工作目录(用户目录)是否存在，如果不存在就要构建原始的工作目录；
  - [x] (<工作目录>/mindx.json)，检查是否已经进行过初始化

- [ ] Demon 需要Hold住与Client端完全一至的Workspace, 否则对于计划任务的执行就会产生目录的偏移，可能会导致文件找不到或目录不正确的错误；因此，客户端是通过`os.Getwd()`来获取当前工作目录，而需要有一个手段来设置Demon的工作目录，以确保Demon与Client端是完全保持一至。
  - 思路1: 将 SessionID 与 工作目录绑定，一个会话就必须与一个工作目录绑定；
```

### 2. cmd/start.go - Daemon 启动时的初始化提示
**文件位置**: `cmd/start.go:41-45`
```go
fmt.Println("🔧 首次运行，正在初始化工作目录...")
if err := core.ExtractWorkspace(RuntimeFS, workspaceDir); err != nil {
    return fmt.Errorf("初始化工作目录失败: %w", err)
}
fmt.Println("✅ 工作目录初始化完成:", workspaceDir)
```

### 3. internal/core/bootstrap.go - Bootstrap 初始化
**文件位置**: `internal/core/bootstrap.go:12-16`
```go
fmt.Println("🔧 首次运行，正在初始化工作目录...")
if err := ExtractWorkspace(embeddedFS, workspaceDir); err != nil {
    return nil, fmt.Errorf("初始化工作目录失败: %w", err)
}
fmt.Println("✅ 工作目录初始化完成:", workspaceDir)
```

### 4. internal/core/workspace.go - 目录创建错误信息
**文件位置**: `internal/core/workspace.go:12`
```go
return fmt.Errorf("创建工作目录失败 %s: %w", workspaceDir, err)
```

### 5. INSTALL.md - 安装文档中的工作目录说明
**文件位置**: `INSTALL.md:28,66,98,113-114,201-212,224,315,345,354,419`

关键内容：
```markdown
# 安装到系统（会提示选择工作目录）

$MINDX_WORKSPACE/             # 工作目录（默认：~/.mindx）
├── config/                 # 配置文件目录
├── data/                  # 数据存储目录
│   ├── sessions/          # 会话数据
└── logs/                  # 日志目录

| `MINDX_WORKSPACE` | 工作目录路径 | `~/.mindx`         |

运行安装脚本时，会提示选择工作目录：
- 选择 **1** 使用默认工作目录 `~/.mindx`
- 选择 **2** 输入自定义工作目录路径
```

---

## 二、英文 "Working Directory" 引用

### 1. Dockerfile
**文件位置**: `Dockerfile:3,20`
```dockerfile
WORKDIR /app
WORKDIR /mindx
```

### 2. runtime/skills/docker-expert/SKILL.md
**文件位置**: 多处 (L180,186,195,219,235,252,408)
- Docker 容器构建示例中的 WORKDIR 指令

### 3. runtime/skills/doc-coauthoring/SKILL.md
**文件位置**: L144
```markdown
Create a markdown file in the working directory.
```

---

## 三、核心实现层 - Workspace 工作空间体系

### 1. cmd/root.go - 默认工作目录定义 ⭐⭐⭐
**文件位置**: `cmd/root.go:43-49`

```go
func defaultWorkspaceDir() string {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        homeDir = "."
    }
    return filepath.Join(homeDir, ".mindx")  // 默认工作目录: ~/.mindx
}

func runTUI(cmd *cobra.Command, args []string) error {
    workspaceDir := defaultWorkspaceDir()      // 获取默认工作目录
    cfg, err := core.Bootstrap(RuntimeFS, workspaceDir)  // 基于工作目录启动
    // ...
}
```

**关键点**:
- 默认工作目录 = `~/.mindx` (用户主目录下的 .mindx 文件夹)
- 这是整个应用的根目录

### 2. internal/core/workspace.go - 工作目录创建与管理 ⭐⭐⭐
**文件位置**: `internal/core/workspace.go` (完整文件)

```go
// ExtractWorkspace 从嵌入的 FS 中提取运行时资源到工作目录
func ExtractWorkspace(embeddedFS fs.FS, workspaceDir string) error {
    if err := os.MkdirAll(workspaceDir, 0755); err != nil {
        return fmt.Errorf("创建工作目录失败 %s: %w", workspaceDir, err)
    }

    return fs.WalkDir(embeddedFS, "runtime", func(path string, d fs.DirEntry, err error) error {
        relPath, _ := filepath.Rel("runtime", path)
        if relPath == "." { return nil }
        
        targetPath := filepath.Join(workspaceDir, relPath)  // 目标路径 = 工作目录 + 相对路径
        
        if d.IsDir() {
            return os.MkdirAll(targetPath, 0755)
        }
        // ... 复制文件到工作目录
    })
}

// WorkspaceExists 检查工作目录是否已存在
func WorkspaceExists(workspaceDir string) bool {
    info, err := os.Stat(workspaceDir)
    if err != nil { return false }
    return info.IsDir()
}
```

**功能**:
- 将嵌入的 `runtime/` 目录内容解压到用户的工作目录
- 首次运行时自动创建工作目录结构
- 包括 agents/, skills/, settings/ 等子目录

### 3. internal/core/bootstrap.go - 启动引导流程 ⭐⭐⭐
**文件位置**: `internal/core/bootstrap.go` (完整文件)

```go
func Bootstrap(embeddedFS fs.FS, workspaceDir string) (*MindxConfig, error) {
    // Step 1: 检查/创建工作目录
    if !WorkspaceExists(workspaceDir) {
        fmt.Println("🔧 首次运行，正在初始化工作目录...")
        if err := ExtractWorkspace(embeddedFS, workspaceDir); err != nil {
            return nil, fmt.Errorf("初始化工作目录失败: %w", err)
        }
        fmt.Println("✅ 工作目录初始化完成:", workspaceDir)
    }

    // Step 2: 设置环境变量 MINDX_WORKSPACE
    os.Setenv("MINDX_WORKSPACE", workspaceDir)

    // Step 3: 加载配置文件 (位于 <工作目录>/mindx.json)
    cfg, err := LoadMindxConfig(workspaceDir)

    // Step 4: 如果首次运行，进入配置向导
    if !cfg.Initialized {
        settingsDir := filepath.Join(workspaceDir, "settings")
        modelsPath := filepath.Join(settingsDir, "models.yml")
        agentsDir := filepath.Join(workspaceDir, "agents")
        // ... 运行首次配置向导
    }

    return cfg, nil
}
```

**关键点**:
- 工作目录是整个应用的生命周期起点
- 通过环境变量 `MINDX_WORKSPACE` 全局传递
- 所有子目录都基于工作目录派生

### 4. internal/core/config.go - 配置文件管理
**文件位置**: `internal/core/config.go`

```go
type MindxConfig struct {
    Version       int          `json:"version"`
    Initialized   bool         `json:"initialized"`
    LastAgent     string       `json:"last_agent,omitempty"`
    LastSessionID string       `json:"last_session_id,omitempty"`  // ← 记录最后会话ID
    DefaultModel  string       `json:"default_model,omitempty"`
    Daemon        DaemonConfig `json:"daemon"`
    filePath      string       `json:"-"`
}

// 配置文件路径 = <工作目录>/mindx.json
func DefaultMindxConfig(workspaceDir string) *MindxConfig {
    return &MindxConfig{
        Version:  1,
        filePath: filepath.Join(workspaceDir, "mindx.json"),
    }
}

func LoadMindxConfig(workspaceDir string) (*MindxConfig, error) {
    filePath := filepath.Join(workspaceDir, "mindx.json")
    // ... 加载配置
}
```

### 5. internal/core/settings.go - 路径解析中心 ⭐⭐⭐
**文件位置**: `internal/core/settings.go` (完整文件)

```go
type Settings struct {
    Test        bool
    MasterAgent string
}

// UserPreferences 返回工作目录基础路径
func (s *Settings) UserPreferences() string {
    if s.Test {
        return "./tmp/mindx-test"
    }
    path, _ := filepath.Abs("~/.mindx")
    return path
}

// 所有子目录都基于工作目录派生:
func (s *Settings) SkillsDir() string     { return filepath.Join(s.UserPreferences(), "skills") }
func (s *Settings) ModelsFile() string    { return filepath.Join(s.UserPreferences(), "settings", "models.yml") }
func (s *Settings) DataDir() string       { return filepath.Join(s.UserPreferences(), "data") }
func (s *Settings) AgentsDir() string     { return filepath.Join(s.UserPreferences(), "agents") }
func (s *Settings) RulesFile() string     { return filepath.Join(s.UserPreferences(), "settings", "rules.yml") }
func (s *Settings) SessionsDir() string   { return filepath.Join(s.UserPreferences(), "sessions") }  // ← 会话存储目录
func (s *Settings) SchedulesDir() string  { return filepath.Join(s.DataDir(), "schedules") }
```

**目录结构映射**:
```
~/.mindx/ (工作目录)
├── skills/          → SkillsDir()
├── agents/          → AgentsDir()
├── settings/
│   ├── models.yml   → ModelsFile()
│   └── rules.yml    → RulesFile()
├── sessions/        → SessionsDir()  ← 会话数据存储
├── data/
│   └── schedules/   → SchedulesDir()
└── mindx.json       → 配置文件
```

---

## 四、Session 与目录的关系体系

### 1. pkg/session/file_store.go - 会话文件存储 ⭐⭐⭐
**文件位置**: `pkg/session/file_store.go` (完整文件, 480行)

```go
type FileSessionStore struct {
    rootDir        string  // 会话存储根目录 (= Settings.SessionsDir())
    slideMu        sync.RWMutex
    slideHandler   core.SlideHandler
    tokenEstimator core.TokenEstimator
}

func NewFileSessionStore(rootDir string) (*FileSessionStore, error) {
    absPath, err := filepath.Abs(rootDir)
    // 创建会话存储目录
    if err := os.MkdirAll(absPath, 0755); err != nil {
        return nil, fmt.Errorf("create session store directory %s: %w", absPath, err)
    }
    return &FileSessionStore{rootDir: absPath, ...}, nil
}

// 目录结构:
// rootDir/
// └── <agentName>/
//     └── <sessionID>/
//         ├── session.yml    # 会话消息
//         └── usages.yml     # Token 用量

func (s *FileSessionStore) agentDir(agentName string) string {
    return filepath.Join(s.rootDir, agentName)
}

func (s *FileSessionStore) sessionDir(agentName, sessionID string) string {
    return filepath.Join(s.agentDir(agentName), sessionID)
}

func (s *FileSessionStore) sessionFilePath(agentName, sessionID string) string {
    return filepath.Join(s.sessionDir(agentName, sessionID), "session.yml")
}

func (s *FileSessionStore) usageFilePath(agentName, sessionID string) string {
    return filepath.Join(s.sessionDir(agentName, sessionID), "usages.yml")
}
```

**关键发现**:
- 当前 Session 存储是 **二维结构**: `<rootDir>/<agentName>/<sessionID>/`
- Session 目录已经按 Agent 分组
- 但 **尚未实现 Session 与独立工作目录的绑定**

### 2. internal/core/app.go - 应用初始化与会话加载
**文件位置**: `internal/core/app.go:77-78`

```go
logger.Info("Loading sessions", "dir", settings.SessionsDir())
sessDB, err := session.NewFileSessionStore(settings.SessionsDir())
```

---

## 五、os.Getwd() 的使用场景

### 1. cmd/intercept/main.go - LLM 请求拦截工具
**文件位置**: `cmd/intercept/main.go:144-145`

```go
cwd, _ := os.Getwd()
outPath := filepath.Join(cwd, ".tmp", "llm_requests.json")
```
**用途**: 将拦截的 LLM 请求保存到当前工作目录下的 `.tmp/` 子目录

### 2. TODO.md:54 - Daemon 工作目录同步问题
```markdown
客户端是通过`os.Getwd()`来获取当前工作目录，而需要有一个手段来设置Demon的工作目录，
以确保Demon与Client端是完全保持一至。
```

### 3. Skill 脚本中的 cwd 引用
**文件位置**: 
- `runtime/skills/skill-creator/scripts/run_eval.py:L23,L28,L89` - Python 脚本使用 `Path.cwd()`
- `runtime/skills/hook-development/scripts/test-hook.sh:L35,51,64,76,88` - Shell 测试脚本使用 `"cwd": "/tmp/test-project"`
- `runtime/skills/skill-creator/scripts/package_skill.py:L48,L85` - 打包脚本使用 `Path.cwd()`

---

## 六、TUI/UI 层面的 Workspace 显示

### 1. internal/client/component_root.go - 状态栏显示
**文件位置**: `internal/client/component_root.go:683-688`

```go
workspace := os.Getenv("MINDX_WORKSPACE")
if workspace == "" {
    workspace = "default"
}
m.contentPanel.ShowWelcome(appTitle, version, workspace, sessionID, "本地模式")
```

### 2. internal/client/component_content.go - Welcome 页面
**文件位置**: `internal/client/component_content.go:23,162,167,182-183,213-214`

```go
type welcomeView struct {
    appTitle  string
    version   string
    workspace string  // ← 工作区路径显示
    sessionID string
    agentName string
    mode      string
}

func (p *ContentPanel) ShowWelcome(appTitle, version, workspace, sessionID, agentName string) {
    p.welcome = &welcomeView{
        workspace: workspace,
        // ...
    }
}

// 渲染时显示:
if workspace != "" {
    lines = append(lines, fmt.Sprintf("Workspace: %s", workspace))
}
```

---

## 七、文档中的 Workspace 设计规范

### 1. docs/TUI-REFACTORING-SPEC.md - TUI 重构规范
**文件位置**: `docs/TUI-REFACTORING-SPEC.md:425-426,438,957,1183,1289-1290`

```go
type Settings struct {
    Workspace   string // 工作区根目录 (e.g., ~/.mindx)  ← 明确定义
    Path        string // PWD 路径                        ← 当前工作目录(?)
    MasterAgent string
}

// Precondition: MINDX_WORKSPACE 环境变量可访问（或使用默认值 ~/.mindx）

// 初始化示例:
Settings{
    Workspace:   os.Getenv("MINDX_WORKSPACE"),
    Path:        os.Getenv("MINDX_PWD_PATH"),  // ← 这个字段暗示了 PWD 概念
}
```

**重要发现**: 规范中定义了两个不同的目录概念：
- `Workspace`: 工作区根目录 (`~/.mindx`)
- `Path`: PWD 路径 (可能对应 `os.Getwd()` 或会话级别的工作目录)

### 2. docs/TUI.md - TUI 设计文档
**文件位置**: `docs/TUI.md:151,167`

```go
type AppState struct {
    workspace  string  // 由环境变量或启动参数指定
}

| Workspace | 由环境变量或启动参数指定 |
```

### 3. docs/SCHEDULER-GUIDE.md - 调度器指南
**文件位置**: `docs/SCHEDULER-GUIDE.md:475,482-483,489`

```markdown
<MINDX_WORKSPACE>/schedules/

MINDX_WORKSPACE=/path/to/workspace
# 则 schedules 目录为: /path/to/workspace/schedules/

# 查看 MindX 进程的工作目录
```

### 4. README.md - 安全配置说明
**文件位置**: `README.md:774,839`

```yaml
# 只能访问工作目录
- "./workspace/**"

| **文件系统** | 路径级别 | 允许 `/workspace/`，禁止 `~/.ssh/` |
```

---

## 八、Daemon 服务层的 Workspace 处理

### 1. cmd/start.go - Daemon 启动命令
**文件位置**: `cmd/start.go:37-47` (已在上面引用)

```go
func runStart(cmd *cobra.Command, args []string) error {
    workspaceDir := defaultWorkspaceDir()  // 获取默认工作目录
    
    // 初始化工作目录（如果不存在）
    if !core.WorkspaceExists(workspaceDir) {
        // ... 创建工作目录
    }
    
    os.Setenv("MINDX_WORKSPACE", workspaceDir)  // 设置环境变量
    
    cfg, err := core.LoadMindxConfig(workspaceDir)
    // ... 启动服务
}
```

### 2. internal/svc/ - 服务层测试
**文件位置**: `internal/svc/app_integration_test.go:182,193-194`

```go
if os.Getenv("MINDX_WORKSPACE") == "" {
    // 测试时设置临时工作目录
}
if os.Getenv("MINDX_WORKSPACE") == "" {
    return fmt.Errorf("MINDX_WORKSPACE not set")
}
```

---

## 九、Skill 中的 Workspace 相关引用

### 1. skill-creator SKILL.md - 技能评估工作空间
**文件位置**: `runtime/skills/skill-creator/SKILL.md:167,180,186,229,239`

```markdown
Put results in `<skill-name>-workspace/` as a sibling to the skill directory.

Within the workspace, organize results by iteration (`iteration-1/`, `iteration-2/`, etc.)

- Save outputs to: <workspace>/iteration-<N>/eval-<ID>/with_skill/outputs/
- **Improving an existing skill**: snapshot the skill (`cp -r <skill-path> <workspace>/skill-snapshot/`)

python -m scripts.aggregate_benchmark <workspace>/iteration-N --skill-name <name>
```

### 2. hook-development SKILL.md - Hook 开发技能
**文件位置**: `runtime/skills/hook-development/SKILL.md:308,327`

```json
{
  "cwd": "/current/working/dir"
}
```
```markdown
- `$CLAUDE_PLUGIN_ROOT` - Plugin directory (use for portable paths)
```

### 3. generate_review.py - 评估查看器
**文件位置**: `runtime/skills/skill-creator/eval-viewer/generate_review.py` (多处)

```python
def find_runs(workspace: Path) -> list[dict]:
    """Reads the workspace directory, discovers runs (directories with outputs/)"""

class ReviewHandler(http.server.BaseHTTPRequestHandler):
    def __init__(self, workspace: Path, skill_name, feedback_path, previous, benchmark_path):
        self.workspace = workspace

# CLI 参数
parser.add_argument("workspace", type=Path, help="Path to workspace directory")
parser.add_argument("--previous-workspace", type=Path, help="Path to previous iteration's workspace")
```

---

## 十、其他相关引用

### 1. internal/client/chat_session.go - 聊天会话初始化
**文件位置**: `internal/client/chat_session.go:32-36`

```go
homeDir, err := os.UserHomeDir()
if err != nil {
    homeDir = "."
}
mindxDir := filepath.Join(homeDir, ".mindx")  // 构建默认 mindx 目录
```

### 2. embed.go - 嵌入式文件系统
项目使用 Go 的 `embed` 功能将 `runtime/` 目录打包进二进制文件，然后在首次运行时解压到工作目录。

### 3. 各平台沙箱脚本中的目录引用
- `runtime/skills/content-repurposer/scripts/setup.sh:L8` - `SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"`
- `runtime/skills/content-repurposer/scripts/repurpose.sh:L9` - 同上
- `runtime/skills/web-artifacts-builder/scripts/init-artifact.sh:L46` - SCRIPT_DIR 获取
- `runtime/skills/valyu-best-practices/scripts/valyu:L6` - 同上
- `runtime/skills/hook-development/scripts/test-hook.sh:L172` - `export CLAUDE_PLUGIN_ROOT="$(pwd)"`

---

## 十一、统计汇总

| 类别 | 数量 | 关键程度 |
|------|------|----------|
| 中文"工作目录"直接提及 | **26 处** | ⭐⭐⭐ |
| 英文 Working Directory | **10 处** | ⭐⭐ |
| `os.Getwd()` / `Path.cwd()` | **17 处** | ⭐⭐⭐ |
| Workspace / MINDX_WORKSPACE | **100+ 处** | ⭐⭐⭐⭐⭐ |
| Session + Dir 组合引用 | **39 处** | ⭐⭐⭐⭐ |
| rootDir / homeDir 引用 | **53 处** | ⭐⭐⭐ |
| **涉及源码文件数** | **~25 个** | - |
| **涉及文档/配置文件** | **~15 个** | - |

---

## 十二、核心架构分析

### MindX 项目的"工作目录"三层架构:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Layer 1: Workspace (工作空间)                  │
│  ─────────────────────────────────────────────────────────────  │
│  定义: ~/.mindx (通过 MINDX_WORKSPACE 环境变量)                   │
│  职责: 应用根目录，包含所有运行时资源                               │
│  实现: cmd/root.go → defaultWorkspaceDir()                       │
│  内容: agents/, skills/, settings/, sessions/, data/            │
├─────────────────────────────────────────────────────────────────┤
│                    Layer 2: Session Dir (会话目录)                 │
│  ─────────────────────────────────────────────────────────────  │
│  定义: <workspace>/sessions/<agent>/<session_id>/                │
│  职责: 存储单个会话的消息和用量数据                                 │
│  实现: pkg/session/file_store.go                                 │
│  内容: session.yml, usages.yml                                   │
├─────────────────────────────────────────────────────────────────┤
│                    Layer 3: CWD (当前工作目录)                     │
│  ─────────────────────────────────────────────────────────────  │
│  定义: os.Getwd() 返回值                                          │
│  职责: 进程级别的当前目录，用于命令执行和文件操作                    │
│  问题: ⚠️ Client 和 Daemon 可能不一致                              │
│  待实现: Session 级别的 CWD 绑定                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 十三、待解决问题 (来自 TODO.md)

### 🔴 问题: Daemon 与 Client 的 Workspace 同步

**现状描述** (TODO.md:54):
```
Demon 需要Hold住与Client端完全一致的Workspace, 否则对于计划任务的执行就会产生目录的偏移，
可能会导致文件找不到或目录不正确的错误。

客户端是通过 os.Getwd() 来获取当前工作目录，而需要有一个手段来设置Demon的工作目录，
以确保Demon与Client端是完全保持一致。
```

### 💡 提出的解决方案

**思路1: 将 SessionID 与 工作目录绑定**
```
一个会话就必须与一个工作目录绑定
```

这意味着:
1. 每个 Session 应该有自己独立的 working directory
2. 当 Daemon 执行定时任务时，应该能够恢复到该 Session 创建时的 CWD
3. Session 元数据中需要增加 `working_dir` 字段

---

## 十四、与 Goreact 项目的关系

### MindX 依赖 Goreact 的能力:

| MindX 组件 | 对应 Goreact 组件 | 工作目录相关 |
|------------|------------------|-------------|
| `pkg/session/file_store.go` | `goreact/core` SessionStore 接口 | Session 存储在 workspace 下 |
| `internal/core/app.go` | `goreact.AgentRegistry`, `goreact.ModelRegistry` | Agent/Skill 从 workspace 加载 |
| TUI Client | goreact Agent 运行时 | Agent 接收 workspace 作为参数 |

### Goreact 已有的工作目录支持 (参考上一份归档):

✅ `BuildEnvironmentInfo()` - 向 Agent 暴露 Primary working directory  
✅ `run_script.working_dir` - 脚本执行支持指定工作目录  
✅ `ValidateFileSafety()` - 基于工作目录的安全校验  
✅ `ProfileWorkspace` - 沙箱的 Workspace 隔离模式  
✅ TODO 确认: "运行的工作目录就是会话的存储目录"  

### MindX 需要补充的实现:

⚠️ Session ↔ Working Directory 的显式绑定逻辑  
⚠️ Daemon 端的 CWD 设置/恢复机制  
⚠️ Session 元数据中的 `working_dir` 字段持久化  

---

## 十五、建议的实施路径

基于以上分析，如果要实现 **"SessionID 与 工作目录绑定"**：

### Phase 1: 数据模型扩展
1. 在 `MindxConfig` 或新建 `SessionMeta` 中添加 `WorkingDir string` 字段
2. 在 `FileSessionStore` 中扩展 session 目录结构，增加 `meta.json`

### Phase 2: 运行时绑定
1. 在创建 Session 时记录当前的 `os.Getwd()` 值
2. 将该值写入 session 的元数据文件
3. 在 Agent 执行前设置 CWD 为 Session 绑定的目录

### Phase 3: Daemon 同步
1. Daemon 加载 Session 时读取 `WorkingDir`
2. 在执行计划任务前 `os.Chdir(session.WorkingDir)`
3. 任务完成后恢复原始 CWD

### Phase 4: UI 显示
1. TUI 的 Welcome 页面显示当前 Session 的工作目录
2. 状态栏增加工作目录指示器

---

*文档结束*
