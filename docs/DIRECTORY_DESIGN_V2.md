# MindX 目录体系设计方案 v2.1

> 版本: 2.1  
> 日期: 2026-05-12  
> 状态: ✅ 已实施 (Agent Native 沙箱架构)  
> 关联文件: [PATHS_DEFINE.md](./PATHS_DEFINE.md)

---

## 🆕 版本更新说明 (v2.0 → v2.1)

### 核心变更：Agent Native 沙箱架构

**实施日期**: 2026-05-12  
**实施范围**: GoReact Framework + MindX Application

#### 主要成就

✅ **SESSION_DIR 作为沙箱根目录** - 彻底解决临时文件隔离问题  
✅ **SessionSandboxManager 集成到 Agent 生命周期** - 自动管理会话级隔离  
✅ **4层目录架构完全对齐** - 每层都有明确的技术实现  
✅ **ValidateFileSafety 增强** - 使用设计时的 ProjectDir 而非运行时 CWD  

#### 技术细节

| 组件 | 变更内容 | 文件位置 |
|------|----------|----------|
| `SandboxConfig` | 新增 `ProjectDir`, `SessionDir` 字段 | [goreact/tools/sandbox.go](../../goreact/tools/sandbox.go) |
| `SessionSandboxManager` | 完全重写，支持 SESSION_DIR 作为根目录 | [goreact/tools/session_sandbox.go](../../goreact/tools/session_sandbox.go) |
| `Agent` 结构体 | 新增 `sandboxMgr` 字段和 `WithSessionBaseDir()` 选项 | [goreact/agent.go](../../goreact/agent.go) |
| `Reactor` 工具初始化 | 根据 sandboxMgr 存在与否选择工具构造函数 | [goreact/reactor/reactor.go](../../goreact/reactor/reactor.go#L370-L475) |
| `ValidateFileSafety` | 新增 `projectDir` 参数，移除对 `os.Getwd()` 的依赖 | [goreact/tools/utils.go](../../goreact/tools/utils.go#L34-L80) |
| MindX App | 自动注入 `WithSessionBaseDir(settings.SessionsDir())` | [mindx/internal/core/app.go#L207-218](../internal/core/app.go#L207-L218) |

---

## 一、问题背景与本质

### 1.1 现状问题（v2.0 已识别）

当前 MindX 与 GoReact 在"工作目录"的使用上存在三重歧义：

| # | 问题描述 | 来源 | 影响 |
|---|---------|------|------|
| P1 | **沙箱隔离边界不清**: 应该按项目目录隔离还是按会话隔离？ | [PATHS_DEFINE.md:18](./PATHS_DEFINE.md#L18) | 安全性与功能性的冲突 |
| P2 | **run_script 执行目录歧义**: 脚本原位置 vs Python 工作目录 vs MindX 工作目录？ | [PATHS_DEFINE.md:19](./PATHS_DEFINE.md#L19) | 脚本执行行为不可预测 |
| P3 | **Daemon 与 Client Workspace 不同步**: `os.Getwd()` 在两端可能不一致 | [PATHS_DEFINE.md:20-21](./PATHS_DEFINE.md#L20-L21) | 定时任务执行失败 |

### 1.2 根本原因

**核心矛盾**："工作目录"这个概念在系统中承载了过多语义：

```
工作目录 = 用户启动位置? 
         = 脚本执行位置?
         = 沙箱隔离边界?
         = 会话存储位置?
         = Daemon 同步目标?
         
→ 语义过载导致行为混乱
```

### 1.3 设计原则

基于 **Agent Native** 理念，确立以下原则：

> **P0 - 语义清晰优先**: 将"目录"概念拆分为独立层级，每层职责单一  
> **P1 - 声明式而非规则式**: 通过语义说明引导 LLM 自主判断，而非硬编码规则引擎  
> **P2 - 框架与应用分离**: GoReact 提供能力，MindX 定义语义  
> **P3 - 最小化侵入**: 不破坏现有架构，渐进式增强  
> **P4 (新增) - 沙箱原生设计**: 安全是基础设施，不是可选功能  

---

## 二、四层目录架构

### 2.1 架构总览

```
┌─────────────────────────────────────────────────────────────┐
│                    MindX 目录体系 v2.1                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Layer 1: HOME_DIR (用户主目录)                               │
│  ════════════════════════════                               │
│  路径:    ~/.mindx                                           │
│  生命周期: 应用安装时创建，全局唯一                             │
│  职责:    应用级数据存储，进程间共享                            │
│  决定者:   MindX (应用层)                                     │
│                                                             │
│  ┌─────────────────────────────────────────┐                │
│  │  Layer 2: PROJECT_DIR (项目工作目录) ⭐    │                │
│  │  ═══════════════════════════════════     │                │
│  │  路径:    os.Getwd() (启动时捕获)        │                │
│  │  生命周期: 与 Session 绑定，可因会话而异    │                │
│  │  职责:    用户当前工作的项目根目录          │                │
│  │  决定者:   MindX Client (运行时动态)       │                │
│  └─────────────────────────────────────────┘                │
│                                                             │
│  ┌─────────────────────────────────────────┐                │
│  │  Layer 3: SESSION_DIR (会话沙箱目录) ⭐⭐  │  ← v2.1 核心!  │
│  │  ═══════════════════════════════════     │                │
│  │  路径:    <HOME>/sessions/<session_id>/ │                │
│  │  生命周期: 随 Session 创建/销毁            │                │
│  │  职责:    会话级别临时文件 & 隔离沙箱      │                │
│  │  决定者:   SessionSandboxManager          │  ← 新组件!     │
│  │                                         │                │
│  │  ★ 内部结构:                             │                │
│  │  ├── tmp/              ← TempDir (自动)  │                │
│  │  │   ├── artifacts/   ← 生成的文件       │                │
│  │  │   └── uploads/     ← 上传的文件       │                │
│  │  ├── meta.json        ← 会话元数据       │                │
│  │  ├── session.yml      ← 对话消息         │                │
│  │  └── usages.yml       ← Token 用量       │                │
│  └─────────────────────────────────────────┘                │
│                                                             │
│  ┌─────────────────────────────────────────┐                │
│  │  Layer 4: SCRIPT_CWD (脚本执行目录)      │                │
│  │  ═══════════════════════════════════     │                │
│  │  路径:    运行时动态决定                  │                │
│  │  生命周期: 单次执行，临时性               │                │
│  │  职责:    单次脚本执行的上下文             │                │
│  │  决定者:   run_script 参数 / Skill 声明    │                │
│  └─────────────────────────────────────────┘                │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 各层详细定义

#### Layer 1: HOME_DIR (`~/.mindx`)

```
用途: 应用生命周期管理
特性: 全局唯一, 进程间共享, 持久化
内容:
  ~/.mindx/
  ├── mindx.json              # 核心配置 (含 LastSessionID, LastProjectDir)
  ├── settings/
  │   ├── models.yml          # 模型配置
  │   └── rules.yml           # 规则配置
  ├── agents/                 # Agent 定义
  ├── skills/                 # 技能文件
  ├── sessions/               # ← Layer 3 的父容器
  └── data/
      └── schedules/          # 定时任务数据
```

**实现位置**: [cmd/root.go:43-49](../cmd/root.go#L43-L49) → `defaultWorkspaceDir()`

#### Layer 2: PROJECT_DIR (动态捕获)

```
用途: 用户当前工作的项目根目录
特性: 每个 Session 可不同, 需要持久化到 Session 元数据
捕获时机: 创建 Session 时通过 os.Getwd() 获取
同步机制: Daemon 加载 Session 时读取并 Chdir

示例:
  TUI 启动于 /Users/ray/workspaces/mindx
  → PROJECT_DIR = /Users/ray/workspaces/mindx
  
  下次可能在 /Users/ray/workspaces/another-project
  → 新 Session 的 PROJECT_DIR = /Users/ray/workspaces/another-project
```

**实现位置**: [goreact/agent.go#L287-308](../../goreact/agent.go#L287-L308) (`WithProjectDir()`)

#### Layer 3: SESSION_DIR (会话沙箱) ⭐ v2.1 核心增强

```
路径结构:
  <HOME_DIR>/sessions/
  └── <session_id>/                    ← 由 SessionSandboxManager 管理
      ├── meta.json                   # 会话元数据 (含 PROJECT_DIR)
      ├── session.yml                 # 对话消息
      ├── usages.yml                  # Token 用量
      └── tmp/                        # ★ 沙箱临时目录 (自动创建)
          ├── artifacts/              # 生成的文件
          └── uploads/                # 上传的文件

用途:
  - 存储会话级别的临时文件 (报告、缓存、数据库等)
  - 作为沙箱隔离的安全边界之一 (★ v2.1 新增)
  - 会话结束后可配置自动清理

★ v2.1 新增能力:
  - 自动作为 Bash/RunScript/PowerShell 的 TempDir
  - 自动加入 AllowedPaths 白名单
  - Session 销毁时完整清理 (不仅是 tmp/)
```

**实现位置**: 
- [goreact/tools/session_sandbox.go](../../goreact/tools/session_sandbox.go) (核心逻辑)
- [mindx/internal/core/app.go#L207-218](../internal/core/app.go#L207-L218) (注入点)

#### Layer 4: SCRIPT_CWD (运行时动态)

```
解析优先级 (从高到低):
  1. run_script 的 working_dir 参数 (显式指定)
  2. SKILL.md 中声明的 execution.working_dir
  3. 默认: Layer 2 PROJECT_DIR

典型场景:
  场景1: npm run build → CWD = PROJECT_DIR (构建项目)
  场景2: python setup.py → CWD = Skill 内 scripts/ (Skill 初始化)
  场景3: python analyze.py → CWD = SESSION_DIR/tmp/ (生成临时结果)
```

**实现位置**: [goreact/tools/run_script.go](../../goreact/tools/run_script.go)

---

## 三、🆕 Agent Native 沙箱架构 (v2.1 核心)

### 3.1 设计哲学

> **安全不是可选功能，而是与生俱来的基础设施。**

在 v2.0 设计中，我们识别出沙箱机制与4层目录架构存在严重不一致：

| 问题 | v2.0 状态 | v2.1 解决方案 |
|------|-----------|---------------|
| TempDir 位置 | `/tmp/goreact-sandbox/` (全局共享) | `${SESSION_DIR}/tmp` (会话隔离) |
| AllowedPaths | 仅允许 `os.Getwd()` | PROJECT_DIR + SESSION_DIR |
| 会话隔离 | ❌ 所有会话共享同一沙箱 | ✅ 每个 Session 独立沙箱 |
| 清理粒度 | 仅删除 tmp 目录 | ✅ 删除整个 SESSION_DIR |
| 路径验证 | 使用运行时 `os.Getwd()` | ✅ 使用设计时 `ProjectDir` |

### 3.2 架构分层

```
┌─────────────────────────────────────────────────────────────┐
│                     MindX (Application Layer)               │
│  ═════════════════════════════════════════════════════      │
│                                                             │
│  ★ 职责: 定义目录语义、注入 Prompt、启用沙箱隔离             │
│                                                             │
│  internal/core/app.go                                       │
│  ├─ getMaster() / ResolveAgent()                            │
│  │  ├─ WithProjectDir(sessionMeta.ProjectDir)               │
│  │  ├─ WithSessionDir(sessionMeta.SessionDir)               │
│  │  └─ WithSessionBaseDir(settings.SessionsDir())  ★ 新增!   │
│  │                                                         │
│  ▼                                                         │
│  goreact.NewAgent(opts...)                                  │
│  ├─ Agent 内部初始化 SessionSandboxManager                  │
│  │  ├─ projectDir = "/Users/ray/my-project"                 │
│  │  └─ sessionBaseDir = "~/.mindx/sessions"                │
│  └─ 通过 reactorOpts 传递给 Reactor                         │
│                                                             │
└─────────────────────────────────────────────────────────────┘
                          ↕ 注入
┌─────────────────────────────────────────────────────────────┐
│                   GoReact (Framework Layer)                  │
│  ═════════════════════════════════════════════════════      │
│                                                             │
│  ★ 职责: 提供沙箱基础设施，不绑定特定应用语义                 │
│                                                             │
│  reactor/reactor.go                                        │
│  ├─ registerBundledTools(setup)                             │
│  │  ├─ if setup.sandboxMgr != nil {                        │
│  │  │   ├─ NewBashToolWithSessionSandbox(sandboxMgr)       │
│  │  │   ├─ NewRunScriptToolWithSessionSandbox(sandboxMgr)  │
│  │  │   └─ NewPowerShellToolWithSessionSandbox(sandboxMgr) │
│  │  │ } else {                                            │
│  │  │   ├─ NewBashTool()  // 向后兼容                      │
│  │  │   └─ ...                                             │
│  │  └─ }                                                   │
│  │                                                         │
│  tools/session_sandbox.go                                 │
│  ├─ SessionSandboxManager                                  │
│  │  ├─ GetConfig(sessionID)                                │
│  │  │  └─ 返回: {                                          │
│  │  │       TempDir: "${sessionBaseDir}/${sid}/tmp",       │
│  │  │       AllowedPaths: [projectDir, sessionDir],        │
│  │  │       ProjectDir: projectDir,                        │
│  │  │       SessionDir: sessionDir                         │
│  │  │     }                                                │
│  │  ├─ RemoveSession(sessionID)                            │
│  │  │  └─ os.RemoveAll(sessionDir) // 完整清理             │
│  │  └─ CleanupStaleSessions()                              │
│  │                                                         │
│  tools/utils.go                                            │
│  └─ ValidateFileSafety(path, projectDir)  ★ 参数化         │
│     └─ 使用设计时的 ProjectDir，而非 os.Getwd()            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 3.3 Session 生命周期中的沙箱管理

#### 完整流程示例

```
1. 用户启动 MindX TUI (位于 /Users/ray/project-a)
   │
   ▼
2. App.CreateSession("personal-assistant")
   │
   ├─ os.Getwd() → "/Users/ray/project-a"
   ├─ sessionID = "sess_abc123"
   ├─ sessionDir = "~/.mindx/sessions/sess_abc123"
   └─ 持久化到 meta.json
   │
   ▼
3. App.getMaster() → 构建 AgentOptions
   │
   ├─ WithProjectDir("/Users/ray/project-a")
   ├─ WithSessionDir("~/.mindx/sessions/sess_abc123")
   └─ WithSessionBaseDir("~/.mindx/sessions")  ★ 新增!
   │
   ▼
4. goreact.NewAgent(opts...)
   │
   └─ 内部执行:
      sandboxMgr = tools.NewSessionSandboxManager(
          projectDir:     "/Users/ray/project-a",
          sessionBaseDir: "~/.mindx/sessions",
      )
      reactorOpts = append(reactorOpts, 
          reactor.WithSessionSandboxManager(sandboxMgr))
   │
   ▼
5. Reactor.registerBundledTools()
   │
   └─ 检测到 setup.sandboxMgr != nil:
      bashTool = NewBashToolWithSessionSandbox(sandboxMgr)
      runScriptTool = NewRunScriptToolWithSessionSandbox(sandboxMgr)
      // PowerShell 同理...
   │
   ▼
6. 用户请求: "帮我运行 npm install"
   │
   ▼
7. LLM 决策 + BashTool.Execute(ctx, {command: "npm install"})
   │
   ├─ sessionID = ExtractSessionID(ctx) → "sess_abc123"
   ├─ config = sandboxMgr.GetConfig("sess_abc123")
   │  └─ 返回:
   │     {
   │       TempDir: "~/.mindx/sessions/sess_abc123/tmp",
   │       AllowedPaths: [
   │         "/Users/ray/project-a",
   │         "~/.mindx/sessions/sess_abc123"
   │       ],
   │       Enabled: true,
   │       Profile: "workspace"
   │     }
   ├─ cmd = sandboxMgr.ApplyToCommand(cmd, sessionID)
   ├─ ensureTempDir(config.TempDir)
   │  └─ os.MkdirAll("~/.mindx/sessions/sess_abc123/tmp", 0755)
   └─ cmd.Run()  ✓ 在安全沙箱中执行
   │
   ▼
8. 用户结束会话
   │
   ▼
9. App.CloseSession()
   │
   └─ sandboxMgr.RemoveSession("sess_abc123")
      └─ os.RemoveAll("~/.mindx/sessions/sess_abc123")
         ├─ 删除 tmp/
         ├─ 删除 artifacts/
         ├─ 删除 meta.json
         └─ ... 完整清理 ✓
```

### 3.4 SandboxConfig 数据结构

```go
type SandboxConfig struct {
    // 基础配置 (原有字段)
    Enabled      bool
    Profile      SandboxProfile  // "sandbox" | "workspace" | "unconfined"
    AllowedPaths []string
    AllowNetwork bool
    TempDir      string
    CustomPolicy string
    
    // ★ v2.1 新增: 4层目录上下文
    ProjectDir string  // Layer 2: 项目工作目录 (始终非空)
    SessionDir string  // Layer 3: 会话沙箱目录 (可选，启用隔离时非空)
}
```

#### 关键方法

```go
// NewSandboxConfigWithDirs 推荐构造函数 (v2.1 新增)
func NewSandboxConfigWithDirs(projectDir, sessionDir string) *SandboxConfig

// 行为:
//   - 当 sessionDir 非空时:
//     * TempDir = ${sessionDir}/tmp
//     * AllowedPaths = [projectDir, sessionDir]
//   - 当 sessionDir 为空时 (向后兼容):
//     * TempDir = /tmp/goreact-sandbox
//     * AllowedPaths = [projectDir]

// HasSessionIsolation 检查是否启用了会话隔离
func (c *SandboxConfig) HasSessionIsolation() bool
// 返回 c.SessionDir != ""
```

### 3.5 SessionSandboxManager API

```go
type SessionSandboxManager struct {
    projectDir     string  // Layer 2 (不可变)
    sessionBaseDir string  // Layer 3 基础路径 (不可变)
    sessions       map[string]*SandboxConfig
}

// 构造函数
func NewSessionSandboxManager(projectDir, sessionBaseDir string) *SessionSandboxManager

// 核心方法
func (m *SessionSandboxManager) GetConfig(sessionID string) *SandboxConfig
//   - 如果 sessionID 已配置: 返回该 session 的配置
//   - 如果未配置: 自动创建默认配置
//     * 当 sessionBaseDir != "":
//       - SessionDir = ${sessionBaseDir}/${sessionID}
//       - TempDir = ${SessionDir}/tmp
//     * 当 sessionBaseDir == "":
//       - Fallback 到 /tmp/goreact-sandbox/${sessionID}

func (m *SessionSandboxManager) SetConfig(sessionID string, config *SandboxConfig)
//   - 自定义某个 session 的配置
//   - 自动填充 ProjectDir 和 SessionDir (如果未提供)
//   - 自动创建 TempDir 目录

func (m *SessionSandboxManager) RemoveSession(sessionID string)
//   - 删除该 session 的 TempDir
//   - 删除整个 SessionDir (如果存在)
//   - 从内存中移除配置

func (m *SessionSandboxManager) ApplyToCommand(cmd *exec.Cmd, sessionID string) *exec.Cmd
//   - 获取 session 配置
//   - 应用沙箱策略到命令

// 辅助方法
func (m *SessionSandboxManager) GetProjectDir() string
func (m *SessionSandboxManager) GetSessionBaseDir() string
func (m *SessionSandboxManager) HasSessionIsolation() bool
//   - 返回 sessionBaseDir != "" (即是否启用了 SESSION_DIR 隔离)
```

---

## 四、Agent Native 目录哲学

### 4.1 核心理念

> **不要用规则约束 LLM，而是用清晰的语义引导 LLM 自主决策。**

LLM 具备强大的语义理解能力。当它理解了每个目录的"职责"后，就能像人类工程师一样自主判断文件应该存到哪里。

### 4.2 目录角色类比

```
想象你是一个工程师，被邀请到用户的办公室结对编程:

📂 PROJECT_DIR = 用户的工作台 (办公桌)
   - 这里有他们正在进行的项目
   - 你修改的代码要放在这里
   - 这里的一切都是持久的、重要的
   
📂 SESSION_DIR = 你的草稿纸 (白板/笔记本) + 安全隔间
   - 你在这里画图、做笔记、写草稿
   - 给用户看的分析报告先写在这里
   - 临时的计算结果、调试信息放这里
   - 会议结束可以擦掉，但当下很有用
   - ★ v2.1: 这个隔间是完全隔离的，不会影响其他会议
```

### 4.3 语义声明 (注入到 System Prompt)

以下是 **MindX 应用层** 应该注入给 LLM 的语义说明（不是 GoReact 层）：

```markdown
## 📁 File Operation Guidelines

You have two primary workspaces with distinct purposes:

### 📂 Project Directory (<PROJECT_DIR>)
**This is the user's actual project — their codebase, their repository.**

**What it represents:**
- The directory where the user invoked `mindx` (captured at session start)
- The user's persistent workspace that exists independently of this conversation
- Files here are version-controlled, shared with team, and long-lived

**Use it for:**
- ✅ Writing or editing source code that belongs in the repo (.go, .py, .js, etc.)
- ✅ Modifying project configuration (package.json, go.mod, Dockerfile, .env.example)
- ✅ Reading existing code to understand the codebase
- ✅ Running build/test/lint commands that operate on the project
- ✅ Creating files that should be committed to git

**Mental model:**  
*"If I close this conversation and come back next week, should this file still be here?"*  
→ **Yes** → Project Directory

---

### 📂 Session Directory (<SESSION_DIR>)
**This is your conversation-specific sandbox — your temporary workspace.**

**What it represents:**
- A directory unique to this session/conversation
- Your ephemeral scratchpad for this specific interaction
- Files here are conversation-scoped, disposable, and not version-controlled
- ★ **Security isolation**: All temporary files from bash/script execution are stored here automatically

**Use it for:**
- ✅ Generating reports, summaries, analyses produced during this conversation
- ✅ Creating temporary/cache files needed for intermediate steps
- ✅ Storing artifacts (diagrams, charts, exported data) generated for the user
- ✅ Saving database files created by skills for this session's context (*.db, *.sqlite)
- ✅ Writing debug logs or investigation output
- ✅ Drafting content before deciding where it ultimately belongs

**Mental model:**  
*"Is this file a byproduct of our conversation — something I'm creating FOR the user right now?"*  
→ **Yes** → Session Directory

---

### 🤔 Quick Decision Framework

When unsure, ask yourself:

1. **Persistence**: Should this file persist after this conversation ends?
   - Yes → **Project Dir**
   - No → **Session Dir**

2. **Ownership**: Who "owns" this file?
   - The project/team/git repo → **Project Dir**
   - This conversation/interaction → **Session Dir**

3. **Purpose**: Why am I creating this file?
   - To add functionality to the project → **Project Dir**
   - To show results/analysis to the user → **Session Dir**

---

### 🔧 Path Syntax (Optional Explicit Prefix)

You can use these prefixes when you want to be extra clear:

| Syntax | Resolves To | Example |
|--------|-------------|---------|
| *(relative path)* | `<PROJECT_DIR>/path` | `src/main.go` |
| `session:<path>` | `<SESSION_DIR>/path` | `session:report.md` |
| `/absolute/path` | Absolute path (sandbox-permitting) | `/tmp/file` |

**Note:** Prefix syntax is optional. Trust your judgment based on the semantics above.

---

### 💡 Common Patterns

**Pattern 1: Code + Report**
```
User: "Refactor auth.go and generate a report"
→ Edit: internal/auth/auth.go              [PROJECT]
→ Write: session:refactoring_report.md     [SESSION]
```

**Pattern 2: Investigation + Fix**
```
User: "Find and fix the login bug"
→ Read: src/**/*.go                       [PROJECT]
→ Write: session:bug_analysis.md           [SESSION]  
→ Edit: src/auth/login.go                 [PROJECT]
→ Write: session:fix_summary.md           [SESSION]
```

**Pattern 3: Generated Artifact**
```
User: "Create an architecture diagram"
→ Read: src/**/*.go                       [PROJECT - understanding]
→ Write: session:arch_diagram.png         [SESSION - artifact]
→ (User can later move to project if desired)
```

---

### ⚠️ Constraints

1. **Sandbox boundaries**: You can only write within PROJECT_DIR and SESSION_DIR
2. **No escape**: Paths like `/etc/passwd`, `~/.ssh/` are blocked by sandbox
3. **Respect explicit intent**: If user says "save to project", honor that
4. **When truly ambiguous**: You may ask the user for clarification

---

### 🎯 Remember

> **You are a skilled engineer working at someone's desk.**  
> The **project directory** is their ongoing work.  
> The **session directory** is your notepad for this pairing session.  
> Use each appropriately, and you'll serve the user best.
```

---

## 五、技术实施方案

### 5.1 架构分层：GoReact vs MindX 职责划分

```
┌─────────────────────────────────────────────────────────────┐
│                     MindX (Application Layer)               │
│  ═════════════════════════════════════════════════════      │
│                                                             │
│  ★ 职责: 定义目录语义、注入 Prompt、管理 Session 元数据       │
│       启用沙箱隔离 (v2.1 新增)                               │
│                                                             │
│  ┌──────────────────────────────────────────────────┐       │
│  │  internal/core/app.go                           │       │
│  │  ├─ CreateSession() → 捕获 os.Getwd()            │       │
│  │  ├─ BuildSystemPrompt() → 注入目录语义说明 ★      │       │
│  │  ├─ ResolveAgentOptions() → 传递上下文给 GoReact  │       │
│  │  └─ ★ WithSessionBaseDir() → 启用沙箱隔离         │       │
│  ├──────────────────────────────────────────────────┤       │
│  │  pkg/session/meta.go                            │       │
│  │  ├─ SessionMeta 结构体定义                       │       │
│  │  └─ 序列化/反序列化 meta.json                    │       │
│  └──────────────────────────────────────────────────┘       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
                          ↕ 调用
┌─────────────────────────────────────────────────────────────┐
│                   GoReact (Framework Layer)                  │
│  ═════════════════════════════════════════════════════      │
│                                                             │
│  ★ 职责: 提供基础设施能力，不绑定特定目录语义                 │
│       提供沙箱隔离基础设施 (v2.1 新增)                       │
│                                                             │
│  ┌──────────────────────────────────────────────────┐       │
│  │  agent.go                                        │       │
│  │  ├─ Agent.sandboxMgr 字段  ★ 新增                 │       │
│  │  └─ WithSessionBaseDir() Option  ★ 新增           │       │
│  ├──────────────────────────────────────────────────┤       │
│  │  tools/session_sandbox.go  ★ v2.1 重写           │       │
│  │  ├─ SessionSandboxManager                       │       │
│  │  ├─ NewSandboxConfigWithDirs()                  │       │
│  │  └─ 完整的会话生命周期管理                        │       │
│  ├──────────────────────────────────────────────────┤       │
│  │  tools/sandbox.go  ★ v2.1 扩展                  │       │
│  │  └─ SandboxConfig 增加 ProjectDir/SessionDir    │       │
│  ├──────────────────────────────────────────────────┤       │
│  │  tools/utils.go  ★ v2.1 改进                    │       │
│  │  └─ ValidateFileSafety(path, projectDir)        │       │
│  ├──────────────────────────────────────────────────┤       │
│  │  reactor/reactor.go  ★ v2.1 改进                │       │
│  │  └─ registerBundledTools() 条件化工具构造       │       │
│  └──────────────────────────────────────────────────┘       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**关键原则**:
- **GoReact 不知道** "什么是项目目录"、"什么是会话目录"的具体语义
- **GoReact 只知道** "有一个字符串叫 ProjectDir，一个叫 SessionDir"
- **GoReact 提供** SessionSandboxManager 作为沙箱基础设施
- **MindX 负责** 赋予这些字符串语义，并通过 Prompt 传达给 LLM
- **MindX 负责** 决定是否启用 SESSION_DIR 隔离（通过 `WithSessionBaseDir()`）

### 5.2 Session 元数据扩展

#### 文件: `pkg/session/meta.go`

```go
package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// SessionMeta 会话元数据，持久化到 <session_dir>/meta.json
// 此结构体由 MindX 定义和使用，GoReact 不依赖此类型
type SessionMeta struct {
	// 基础标识
	SessionID string    `json:"session_id"`
	AgentName string    `json:"agent_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// ★ 目录绑定信息
	HomeDir           string `json:"home_dir"`             // Layer 1: ~/.mindx
	ProjectWorkingDir string `json:"project_working_dir"` // Layer 2: 动态捕获

	// 运行时统计
	MessageCount   int       `json:"message_count"`
	LastActivityAt time.Time `json:"last_activity_at"`
}

// NewSessionMeta 创建新的会话元数据
func NewSessionMeta(sessionID, agentName, projectDir string) (*SessionMeta, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	now := time.Now()
	return &SessionMeta{
		SessionID:         sessionID,
		AgentName:         agentName,
		CreatedAt:         now,
		UpdatedAt:         now,
		HomeDir:           homeDir,
		ProjectWorkingDir: projectDir,
	}, nil
}

// Save 将元数据持久化到指定路径
func (m *SessionMeta) Save(sessionDirPath string) error {
	m.UpdatedAt = time.Now()
	
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	metaPath := filepath.Join(sessionDirPath, "meta.json")
	return os.WriteFile(metaPath, data, 0600)
}

// LoadSessionMeta 从目录加载元数据
func LoadSessionMeta(sessionDirPath string) (*SessionMeta, error) {
	metaPath := filepath.Join(sessionDirPath, "meta.json")
	
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	var meta SessionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	
	return &meta, nil
}
```

### 5.3 System Prompt 注入点 (MindX 层)

#### 文件: `internal/core/prompts.go`

```go
package core

const (
	// DirectorySemanticsPrompt 目录语义引导词
	// 此常量由 MindX 定义，在构建 System Prompt 时注入
	// GoReact 不包含此内容，保持框架的通用性
	DirectorySemanticsPrompt = `
## 📁 File Operation Guidelines

You have two primary workspaces with distinct purposes:

### 📂 Project Directory (%s)
**This is the user's actual project — their codebase, their repository.**

It is the directory where the user invoked \`mindx\`, captured when this session started.
Files here are persistent, version-controlled, and long-lived.

**Use it for:**
- Source code files (.go, .py, .js, .ts, .vue, etc.)
- Project configuration (package.json, go.mod, Dockerfile, .env.example, tsconfig.json)
- Project documentation (README.md at root, CHANGELOG.md, LICENSE)
- Test files (*_test.go, *.test.js, *.spec.py)
- Build/lint operations on the project
- Any file that should be committed to git

**Mental model:** *"If I close this conversation and come back later, should this file still exist here?"* → **Yes** = Project Dir

### 📂 Session Directory (%s)
**This is your conversation-specific sandbox — your temporary workspace.**

A directory unique to this session. Files here are ephemeral, conversation-scoped, and not version-controlled.

**Use it for:**
- Reports, summaries, analyses generated during this conversation (e.g., *report*.md, *analysis*.md)
- Temporary/cache files (tmp/*, cache/*, scratch/*, *.tmp)
- Artifacts generated for the user (diagrams, charts, exported data)
- Database files created by skills (*.db, *.sqlite, *.db3)
- Debug logs and investigation output (*.log, debug*)
- Draft content before deciding final location

**Mental model:** *"Is this a byproduct of our conversation — something I'm creating for the user right now?"* → **Yes** = Session Dir

### 🤔 Quick Decision
- **Persists after chat?** → Project
- **Conversation output?** → Session
- **Truly ambiguous?** Use explicit prefix or ask user

### 🔧 Optional Prefix Syntax
- Relative path (default): writes to Project Dir → \`src/main.go\`
- \`session:<path>\`: writes to Session Dir → \`session:report.md\`
- Absolute path: works as usual (subject to sandbox)

### 💡 Examples
\`\`\`
Edit source code:     { file_path: "api/user.go" }          [PROJECT]
Write report:         { file_path: "session:analysis.md" }  [SESSION]
Read project config:  { file_path: "package.json" }         [PROJECT]
Save temp data:       { file_path: "session:cache.json" }  [SESSION]
\`\`\`

### ⚠️ Constraints
1. Only write within Project Dir and Session Dir
2. Paths like /etc/passwd, ~/.ssh/ are blocked
3. If user explicitly says "save to project", honor that
`
)

// BuildDirectoryGuidelines 构建带实际路径值的目录引导词
// 此函数在 MindX 层调用，将占位符替换为真实路径
func BuildDirectoryGuidelines(projectDir, sessionDir string) string {
	return fmt.Sprintf(DirectorySemanticsPrompt, projectDir, sessionDir)
}
```

### 5.4 GoReact 层的最小化支持

GoReact 只需要提供**极简的基础设施**，不做语义判断：

#### 文件: `goreact/reactor/prompt.go` - 参数化环境信息

```go
// BuildEnvironmentInfo 构建环境信息 (参数化版本)
// 注意: 此函数不再硬编码任何目录语义，只负责格式化输出
func BuildEnvironmentInfo(params EnvironmentInfoParams) string {
	return fmt.Sprintf(`## Environment
- Home Directory: %s
- Project Working Directory: %s
- Session Sandbox Directory: %s
- Platform: %s/%s
- Shell: %s
- Session ID: %s
- App Name: %s
- App Version: %s`,
		params.HomeDir,
		params.ProjectDir,    // ← 由调用方填充
		params.SessionDir,    // ← 由调用方填充
		params.Platform,
		params.OSArch,
		params.Shell,
		params.SessionID,
		params.AppName,
		params.AppVersion,
	)
}

// EnvironmentInfoParams 环境信息参数 (纯数据结构)
type EnvironmentInfoParams struct {
	HomeDir    string
	ProjectDir string
	SessionDir string
	Platform   string
	OSArch     string
	Shell      string
	SessionID  string
	AppName    string
	AppVersion string
}
```

#### 文件: `goreact/tools/write.go` - 支持前缀解析

```go
const sessionPrefix = "session:"

// resolveTargetPath 解析目标路径 (最小化实现)
// 只支持 session: 前缀语法，不做启发式推断
func resolveTargetPath(inputPath string, projectDir, sessionDir string) (absPath string, scope string) {
	// 显式 session: 前缀
	if strings.HasPrefix(inputPath, sessionPrefix) {
		filename := strings.TrimPrefix(inputPath, sessionPrefix)
		return filepath.Join(sessionDir, filename), "session"
	}

	// 默认: 项目目录
	return filepath.Join(projectDir, inputPath), "project"
}
```

### 5.5 Daemon 同步机制

#### 文件: `internal/svc/scheduler.go`

```go
// ExecuteScheduledTask 执行定时任务 (带工作目录恢复)
func (s *Scheduler) ExecuteScheduledTask(task ScheduledTask) error {
	// 1. 加载 Session 元数据
	meta, err := s.sessionStore.GetSessionMeta(task.SessionID)
	if err != nil {
		return fmt.Errorf("load session meta: %w", err)
	}

	// 2. 保存原始 CWD
	originalCWD, _ := os.Getwd()

	// 3. ★ 切换到 Session 绑定的项目工作目录
	if err := os.Chdir(meta.ProjectWorkingDir); err != nil {
		return fmt.Errorf("chdir to project dir %s: %w", meta.ProjectWorkingDir, err)
	}

	// 4. 确保任务完成后恢复
	defer func() {
		if restoreErr := os.Chdir(originalCWD); restoreErr != nil {
			s.logger.Error("failed to restore cwd",
				"original", originalCWD,
				"error", restoreErr,
			)
		}
	}()

	// 5. 设置完整的环境变量上下文
	os.Setenv("MINDX_SESSION_ID", meta.SessionID)
	os.Setenv("MINDX_PROJECT_DIR", meta.ProjectWorkingDir)
	os.Setenv("MINDX_HOME_DIR", meta.HomeDir)
	os.Setenv("MINDX_SESSION_DIR", s.buildSessionDirPath(meta))

	// 6. 记录日志
	s.logger.Info("executing scheduled task with restored environment",
		"session_id", task.SessionID,
		"project_dir", meta.ProjectWorkingDir,
		"original_cwd", originalCWD,
	)

	// 7. 执行任务...
	return s.executeTask(task, meta)
}
```

---

## 六、数据流与交互序列

### 6.1 Session 创建流程 (TUI 启动)

```
User: 在 /Users/ray/project-a/ 目录下运行 mindx
  │
  ▼
[cmd/root.go: runTUI()]
  │
  ├─ workspaceDir = defaultWorkspaceDir()  → "~/.mindx"
  ├─ core.Bootstrap(FS, workspaceDir)     → 初始化 HOME_DIR
  │
  ▼
[client.NewProgram(cfg)]
  │
  ▼
[core.DefaultApp()]
  │
  ├─ settings.SessionsDir() → "~/.mindx/sessions"
  │
  ▼
[App.CreateSession("personal-assistant")]
  │
  ├─ os.Getwd() → "/Users/ray/project-a"   ★ 捕获 PROJECT_DIR
  ├─ sessionID = generateID() → "sess_abc123"
  ├─ meta = SessionMeta{
  │     ProjectWorkingDir: "/Users/ray/project-a",
  │     HomeDir: "~/.mindx",
  │     SessionID: "sess_abc123",
  │     AgentName: "personal-assistant",
  │   }
  ├─ 创建目录: ~/.mindx/sessions/sess_abc123/tmp/
  └─ 写入: meta.json  ★ 持久化
  │
  ▼
[App.getMaster()]  ★ v2.1 增强
  │
  ├─ 读取 meta.json
  ├─ opts = []goreact.AgentOption{
  │     goreact.WithProjectDir("/Users/ray/project-a"),
  │     goreact.WithSessionDir("~/.mindx/sessions/sess_abc123"),
  │     goreact.WithSessionBaseDir("~/.mindx/sessions"),  ★ 新增!
  │   }
  │
  ▼
[goreact.NewAgent(opts...)]  ★ v2.1 内部变化
  │
  ├─ sandboxMgr = tools.NewSessionSandboxManager(
  │     projectDir: "/Users/ray/project-a",
  │     sessionBaseDir: "~/.mindx/sessions",
  │   )
  ├─ reactorOpts = append(reactorOpts,
  │     reactor.WithSessionSandboxManager(sandboxMgr))
  │
  ▼
[Reactor.registerBundledTools()]  ★ v2.1 条件化
  │
  ├─ if sandboxMgr != nil {
  │     bashTool = NewBashToolWithSessionSandbox(sandboxMgr)
  │     runScriptTool = NewRunScriptToolWithSessionSandbox(sandboxMgr)
  │     // ...
  │   }
  │
  ▼
[用户开始对话...]
  │
  └─ AI 已知:
     - PROJECT_DIR = /Users/ray/project-a
     - SESSION_DIR = ~/.mindx/sessions/sess_abc123
     - 各自的职责和使用场景
     - ★ 沙箱已就绪: TempDir=${SESSION_DIR}/tmp
```

### 6.2 文件操作流程 (AI 决策)

```
User: "帮我重构 auth.go 并生成报告"
  │
  ▼
[LLM 推理 (受 System Prompt 引导)]
  │
  ├─ 思考: "auth.go 是源代码 → 应该去 PROJECT_DIR"
  ├─ 决策: Read { file_path: "src/auth/auth.go" }
  │
  ▼
[goreact tools/read.go 执行]
  │
  ├─ resolveTargetPath("src/auth/auth.go", projectDir, sessionDir)
  ├─ 无 session: 前缀 → 默认 PROJECT_DIR
  ├─ ValidateFileSafety(resolvedPath, tc.ProjectDir)  ★ v2.1 使用设计时值
  └─ 实际读取: /Users/ray/project-a/src/auth/auth.go  ✅
  │
  ▼
[LLM 继续推理]
  │
  ├─ 思考: "报告是对话产物 → 应该去 SESSION_DIR"
  ├─ 决策: Write { file_path: "session:refactoring_report.md" }
  │
  ▼
[goreact tools/write.go 执行]
  │
  ├─ resolveTargetPath("session:refactoring_report.md", ...)
  ├─ 检测到 session: 前缀 → SESSION_DIR
  ├─ ValidateFileSafety(resolvedPath, tc.ProjectDir)  ★ v2.1
  └─ 实际写入: ~/.mindx/sessions/sess_abc123/refactoring_report.md  ✅
  │
  ▼
[返回结果给用户]
  │
  └─ Response: {
       "file": ".../refactoring_report.md",
       "scope": "session",  // 透明告知
       "message": "Report saved to session directory"
     }
```

### 6.3 沙箱命令执行流程 (v2.1 新增)

```
User: "运行 npm install"
  │
  ▼
[LLM 决策]
  │
  └─ Bash { command: "npm install" }
  │
  ▼
[BashTool.Execute(ctx, params)]
  │
  ├─ sessionID = ExtractSessionID(ctx) → "sess_abc123"
  │
  ▼
[sessionSandboxMgr.GetConfig("sess_abc123")]  ★ v2.1 核心
  │
  └─ 返回 SandboxConfig {
       Enabled: true,
       Profile: "workspace",
       TempDir: "~/.mindx/sessions/sess_abc123/tmp",  ★ SESSION_DIR-based!
       AllowedPaths: [
         "/Users/ray/project-a",                    ★ PROJECT_DIR
         "~/.mindx/sessions/sess_abc123"             ★ SESSION_DIR
       ],
       AllowNetwork: true,
       ProjectDir: "/Users/ray/project-a",
       SessionDir: "~/.mindx/sessions/sess_abc123",
     }
  │
  ▼
[ApplyToCommand(cmd, sessionID)]
  │
  ├─ 应用沙箱策略 (macOS: Seatbelt, Linux: seccomp/namespace)
  ├─ 设置环境变量限制
  └─ 返回受保护的 cmd 对象
  │
  ▼
[ensureTempDir(config.TempDir)]  ★ v2.1
  │
  └─ os.MkdirAll("~/.mindx/sessions/sess_abc123/tmp", 0755)
     └─ 确保 SESSION_DIR/tmp 存在 (如果不存在)
  │
  ▼
[cmd.Run()]  ★ 在安全沙箱中执行
  │
  ├─ 工作目录: /Users/ray/project-a (PROJECT_DIR)
  ├─ 临时文件: 写入 ~/.mindx/sessions/sess_abc123/tmp/
  ├─ 允许访问:
  │   ✓ /Users/ray/project-a/** (项目文件)
  │   ✓ ~/.mindx/sessions/sess_abc123/** (会话文件)
  │   ✗ /etc/passwd (系统文件 - 被阻止)
  │   ✗ ~/.ssh/ (敏感目录 - 被阻止)
  └─ 结果返回给用户  ✅
```

### 6.4 Daemon 定时任务流程

```
[Scheduler 触发定时任务]
  │
  ▼
[ExecuteScheduledTask(task)]
  │
  ├─ 加载: sessions/.../<task.SessionID>/meta.json
  ├─ meta.ProjectWorkingDir → "/Users/ray/project-a"
  │
  ├─ originalCWD = os.Getwd()  → 可能是 "/" 或其他
  │
  ├─ os.Chdir("/Users/ray/project-a")  ★ 切换!
  │
  ▼
[执行任务 (现在 CWD 正确)]
  │
  ├─ 运行 bash 命令 / 脚本
  ├─ 所有相对路径都基于 /Users/ray/project-a 解析
  │  └─ ★ 如果使用了 SessionSandboxManager:
  │     TempDir = ${SESSION_DIR}/tmp
  │     AllowedPaths 包含 PROJECT_DIR 和 SESSION_DIR
  │
  ▼
[任务完成]
  │
  └─ defer: os.Chdir(originalCWD)  ★ 恢复!
```

---

## 七、✅ 实施状态总览

### Phase 1: 数据模型 ✅ 已完成

**目标**: 建立 Session 元数据基础设施

- [x] 创建 `pkg/session/meta.go`
  - [x] 定义 `SessionMeta` 结构体
  - [x] 实现 `NewSessionMeta()`, `Save()`, `LoadSessionMeta()`
- [x] 修改 `pkg/session/file_store.go`
  - [x] 增加 `GetSessionMeta(sessionID)` 方法
  - [x] 在 `Append()` 时更新 `meta.json` 的 `UpdatedAt`
- [x] 更新 `internal/core/config.go`
  - [x] `MindxConfig` 增加 `LastProjectDir string` 字段
  - [x] 在切换会话时持久化

**验证**: 单元测试覆盖 meta.json 的读写 ✅

---

### Phase 2: Session 创建集成 ✅ 已完成

**目标**: 创建 Session 时自动捕获 PROJECT_DIR

- [x] 修改 `internal/core/app.go`
  - [x] 新增 `CreateSession(agentName)` 方法
  - [x] 集成 `os.Getwd()` 捕获
  - [x] 调用 `meta.Save(sessionDir)`
- [x] 修改 `internal/client/` (TUI 层)
  - [x] 启动时调用 `CreateSession()`
  - [x] 传递 `meta` 给 Agent 初始化流程

**验证**: 手动启动 mindx，检查生成的 `meta.json` 内容正确 ✅

---

### Phase 3: System Prompt 注入 ✅ 已完成

**目标**: MindX 层注入目录语义引导

- [x] 创建 `internal/core/prompts.go`
  - [x] 定义 `DirectorySemanticsPrompt` 常量
  - [x] 实现 `BuildDirectoryGuidelines(projectDir, sessionDir)`
- [x] 修改 `internal/core/app.go`
  - [x] `getMaster()` 中构建 guidelines
  - [x] 通过 `WithSystemPromptAddon()` 传递
- [x] (可选) 修改 `goreact/options.go`
  - [x] 新增 `WithSystemPromptAddon(addon)` Option

**验证**: 启动对话，让 AI 执行文件操作，观察其路径选择是否符合预期 ✅

---

### Phase 4: 工具层最小支持 ✅ 已完成

**目标**: 支持 `session:` 前缀语法

- [x] 修改 `goreact/tools/write.go`
  - [x] 实现 `resolveTargetPath()`
  - [x] 日志中记录 resolved path 和 scope
- [x] 修改 `goreact/tools/read.go`
  - [x] 同上
- [x] 修改 `goreact/tools/edit.go`
  - [x] 同上
- [x] (可选) `goreact/tools/bash.go`
  - [x] 输出重定向感知

**验证**: 测试 `session:` 前缀和默认行为的路径解析 ✅

---

### Phase 5: Daemon 同步 ✅ 已完成

**目标**: Daemon 执行任务时恢复 PROJECT_DIR

- [x] 修改 `internal/svc/scheduler.go`
  - [x] `ExecuteScheduledTask()` 中加载 meta
  - [x] 实现 Chdir/Restore 逻辑
  - [x] 设置环境变量
- [x] 编写集成测试
  - [x] Mock 不同 CWD 场景
  - [x] 验证任务执行前后 CWD 一致性

**验证**: 设置定时任务，在不同目录启动 Daemon，检查任务执行结果 ✅

---

### Phase 6: 🆕 Agent Native 沙箱架构 ✅ 已完成 (v2.1)

**目标**: SESSION_DIR 作为沙箱根目录，完全隔离会话

- [x] 重构 `goreact/tools/sandbox.go`
  - [x] `SandboxConfig` 增加 `ProjectDir`, `SessionDir` 字段
  - [x] 新增 `NewSandboxConfigWithDirs()` 构造函数
  - [x] 新增 `HasSessionIsolation()` 方法
- [x] 完全重写 `goreact/tools/session_sandbox.go`
  - [x] `NewSessionSandboxManager(projectDir, sessionBaseDir)`
  - [x] 每个 Session 获得独立 `${sessionBaseDir}/${sessionID}/`
  - [x] TempDir 自动设置为 `${sessionDir}/tmp`
  - [x] 清理时删除整个会话目录
- [x] 修改 `goreact/agent.go`
  - [x] `Agent` 结构体增加 `sandboxMgr` 字段
  - [x] 新增 `WithSessionBaseDir()` Option
  - [x] Agent 创建时自动初始化 SessionSandboxManager
- [x] 修改 `goreact/reactor/reactor_options.go`
  - [x] 新增 `WithSessionSandboxManager()` ReactorOption
- [x] 修改 `goreact/reactor/reactor.go`
  - [x] `registerBundledTools()` 条件化工具构造
  - [x] 有 sandboxMgr → 使用带隔离的工具
  - [x] 无 sandboxMgr → fallback 到旧行为
- [x] 改进 `goreact/tools/utils.go`
  - [x] `ValidateFileSafety(path, projectDir)` 参数化
  - [x] 移除对 `os.Getwd()` 的依赖
  - [x] 更新所有调用点 (write/read/edit/ls)
- [x] 修改 `mindx/internal/core/app.go`
  - [x] `getMaster()` 和 `ResolveAgent()` 中注入 `WithSessionBaseDir()`
  - [x] 自动从 `settings.SessionsDir()` 获取基础路径
- [x] 更新所有测试用例
  - [x] 适配新的函数签名
  - [x] 验证向后兼容性

**验证**:
- [x] GoReact 编译通过 (`go build ./...`)
- [x] MindX 编译通过 (`go build ./...`)
- [x] 沙箱相关测试全部通过
- [x] 向后兼容性验证 (无 SessionBaseDir 时 fallback 正常)

---

### Phase 7: 文档与迭代 (持续)

- [x] 更新此文档至 v2.1 反映沙箱架构实施
- [ ] 更新 `docs/PATHS_DEFINE.md` 为 v2.1 定义
- [ ] 更新 `INSTALL.md` 中的目录说明
- [ ] 收集真实使用反馈
- [ ] 迭代优化 `DirectorySemanticsPrompt` 措辞

---

## 八、风险与缓解

| 风险 | 影响 | 缓解措施 | 当前状态 |
|------|------|----------|----------|
| LLM 仍然选错目录 | 文件存错位置 | 收集错误案例，优化 Prompt 措辞；增加 `_scope` 显式参数作为逃生舱 | ✅ 已提供清晰语义指南 |
| `session:` 前缀与现有文件名冲突 | 意外路由到会话目录 | 前缀只在工具入口解析，不影响文件系统本身 | ✅ 已实现最小化前缀解析 |
| Daemon Chdir 失败 | 任务无法执行 | Fallback 到 HOME_DIR；记录详细日志；任务状态标记为 failed | ✅ 已实现 Chdir/Restore |
| 元数据文件损坏 | 无法恢复 Session 信息 | 定期备份；损坏时重建元数据（扫描 session.yml） | ✅ 已实现 meta.json 持久化 |
| 性能开销 (每次文件操作都解析) | 微小延迟 | 解析逻辑极轻量（字符串前缀匹配）；可后续缓存 | ✅ 性能影响可忽略 |
| **🆕 沙箱隔离失效** | **安全漏洞** | **SessionSandboxManager 强制隔离；单元测试覆盖；集成测试验证** | **✅ 已实现并测试** |
| **🆕 TempDir 磁盘占用过高** | **磁盘空间耗尽** | **自动清理 stale sessions (>24h)；配额限制 (未来)** | **✅ 已实现基本清理** |
| **🆕 向后兼容性破坏** | **旧代码无法工作** | **所有改动保持向后兼容；无 SessionBaseDir 时 fallback 到旧行为** | **✅ 已验证兼容性** |

---

## 九、成功指标

1. **语义准确率**: AI 自主选择的目录符合预期 > 90% (通过日志统计)
2. **显式使用率**: `session:` 前缀使用率 < 10% (说明大多数时候 AI 能自主判断正确)
3. **Daemon 成功率**: 定时任务的文件操作成功率 > 99%
4. **向后兼容**: 现有不使用 meta.json 的 Session 仍能正常工作 (优雅降级)
5. **🆕 沙箱隔离率**: 100% 的 Bash/RunScript/PowerShell 命令都在会话隔离沙箱中执行
6. **🆕 临时文件清理率**: Session 结束后 100% 清理 SESSION_DIR (包括 tmp/)
7. **🆕 安全事件数**: 0 次路径逃逸攻击 (AllowedPaths 严格限制)

---

## 十、未来展望

### 10.1 多项目并行支持

当前设计已自然支持：每个 Session 可以有不同的 PROJECT_DIR。未来可以扩展 UI：

```
┌─────────────────────────────────────┐
│  Sessions                            │
├─────────────────────────────────────┤
│  📁 project-a (active)              │
│     ~/workspaces/project-a          │
│     🔒 Sandbox: Isolated            │  ← v2.1 可视化
│                                     │
│  📁 project-b                       │
│     ~/workspaces/project-b          │
│     🔒 Sandbox: Isolated            │
│                                     │
│  ➕ New Session (current dir)       │
└─────────────────────────────────────┘
```

### 10.2 Skill 级别的目录声明

SKILL.md frontmatter 可声明其产出物的默认位置：

```yaml
---
name: project-manager
data_files:
  - pattern: "*.db"
    scope: session
outputs:
  - pattern: "report*.*"
    scope: session
---
```

### 10.3 会话迁移

有了明确的 PROJECT_DIR 记录，可以实现：

- 在不同机器上恢复 Session（只要项目路径相同）
- 导出 Session 时包含完整的上下文信息
- 调试时精确定位问题发生的目录环境

### 10.4 🆕 高级沙箱功能 (v2.2+ 展望)

#### 磁盘配额限制
```go
type QuotaConfig struct {
    MaxSessionSize    int64  // 单个 Session 最大磁盘占用 (bytes)
    MaxTempFileSize   int64  // 单个临时文件最大大小 (bytes)
    CleanupPolicy     string // "aggressive" | "conservative" | "manual"
}

mgr := tools.NewSessionSandboxManager(
    projectDir,
    sessionBaseDir,
    tools.WithQuotaConfig(&QuotaConfig{
        MaxSessionSize: 100 * 1024 * 1024, // 100MB per session
        MaxTempFileSize: 10 * 1024 * 1024,  // 10MB per file
        CleanupPolicy: "conservative",
    }),
)
```

#### 网络访问控制精细化
```go
cfg := mgr.GetConfig(sessionID)
cfg.Update(func(c *SandboxConfig) {
    c.AllowNetwork = false  // 禁止网络访问
    c.CustomPolicy = `
        network {
            allow ["github.com:443", "pypi.org:443"]
        }
    `
})
```

#### 审计日志增强
```go
type SandboxAuditLog struct {
    SessionID    string
    Timestamp    time.Time
    Operation    string  // "file_write" | "bash_exec" | "network_access"
    Path         string
    Allowed      bool
    Reason       string  // if blocked
}
```

---

## 十一、总结

### v2.0 核心创新点

1. **四层目录架构**: HOME / PROJECT / SESSION / SCRIPT，各司其职
2. **Agent Native 哲学**: 语义引导优于规则约束
3. **清晰的分层边界**: GoReact 提供能力，MindX 定义语义
4. **最小化实现**: 不需要复杂的启发式引擎，只需一段好的 Prompt + 简单的前缀语法
5. **渐进式演进**: 不破坏现有架构，可以分阶段实施

### v2.1 核心增强 (Agent Native 沙箱架构)

6. **🆕 SESSION_DIR 作为沙箱根目录**: 彻底解决临时文件隔离问题
7. **🆕 SessionSandboxManager 深度集成**: 自动管理会话级隔离，无需手动干预
8. **🆕 4层目录架构完全对齐**: 每层都有明确的技术实现和安全保障
9. **🆕 ValidateFileSafety 增强**: 使用设计时的 ProjectDir，消除运行时不一致性
10. **🆕 向后兼容保证**: 所有改动保持 API 兼容，旧代码无需修改

### 最终目标

> **让 LLM 像人类工程师一样，直觉性地知道"这个文件应该放在哪"。**  
> **同时确保每次操作都在安全的沙箱中进行，保护系统和用户数据。**

---

## 附录 A: 关键文件索引

### GoReact Framework

| 文件 | 角色 | v2.1 变更 |
|------|------|-----------|
| [tools/sandbox.go](../../goreact/tools/sandbox.go) | SandboxConfig 定义 | ✅ 扩展: 增加 ProjectDir/SessionDir |
| [tools/session_sandbox.go](../../goreact/tools/session_sandbox.go) | 会话沙箱管理器 | ✅ 完全重写: SESSION_DIR 作为根目录 |
| [agent.go](../../goreact/agent.go) | Agent 门面 | ✅ 扩展: sandboxMgr + WithSessionBaseDir |
| [reactor/reactor.go](../../goreact/reactor/reactor.go) | Reactor 引擎 | ✅ 改进: 条件化工具构造 |
| [reactor/reactor_options.go](../../goreact/reactor/reactor_options.go) | Reactor 选项 | ✅ 新增: WithSessionSandboxManager |
| [tools/utils.go](../../goreact/tools/utils.go) | 工具辅助函数 | ✅ 改进: ValidateFileSafety 参数化 |
| [tools/bash.go](../../goreact/tools/bash.go) | Bash 工具 | 无变化 (通过 sandboxMgr 集成) |
| [tools/write.go](../../goreact/tools/write.go) | 写入工具 | 无变化 (已支持 session: 前缀) |
| [tools/read.go](../../goreact/tools/read.go) | 读取工具 | 无变化 (已支持 session: 前缀) |
| [tools/edit.go](../../goreact/tools/edit.go) | 编辑工具 | 无变化 (已支持 session: 前缀) |

### MindX Application

| 文件 | 角色 | v2.1 变更 |
|------|------|-----------|
| [internal/core/app.go](../internal/core/app.go) | 应用入口 | ✅ 增强: 注入 WithSessionBaseDir |
| [internal/core/prompts.go](../internal/core/prompts.py) | Prompt 构建 | 无变化 (已包含目录语义) |
| [pkg/session/meta.go](../pkg/session/meta.go) | Session 元数据 | 无变化 (已在 v2.0 实现) |
| [internal/svc/scheduler.go](../internal/svc/scheduler.go) | 调度器 | 无变化 (已在 v2.0 实现) |

---

## 附录 B: API 速查表

### MindX 层 (应用开发者)

```go
// 创建 Agent 时启用沙箱隔离 (推荐)
agent, err := goreact.NewAgent(
    goreact.WithConfig(cfg),
    goreact.WithModel(model),
    goreact.WithProjectDir("/path/to/project"),
    goreact.WithSessionBaseDir("~/.myapp/sessions"),  // ← 启用 SESSION_DIR 隔离
)

// 或者不启用 (向后兼容)
agent, err := goreact.NewAgent(
    goreact.WithConfig(cfg),
    goreact.WithModel(model),
    // 无 WithSessionBaseDir → fallback 到 /tmp/goreact-sandbox/
)
```

### GoReact 层 (框架扩展者)

```go
// 直接使用 SessionSandboxManager
mgr := tools.NewSessionSandboxManager(
    "/path/to/project",      // Layer 2
    "/path/to/sessions",     // Layer 3 base (可选)
)

// 获取某个 Session 的配置
cfg := mgr.GetConfig("sess_abc123")
fmt.Println(cfg.TempDir)        // "/path/to/sessions/sess_abc123/tmp"
fmt.Println(cfg.AllowedPaths)  // ["/path/to/project", "/path/to/sessions/sess_abc123"]
fmt.Println(cfg.HasSessionIsolation())  // true

// 自定义某个 Session 的配置
mgr.SetConfig("sess_custom", &tools.SandboxConfig{
    AllowNetwork: false,  // 禁止网络
})

// 清理 Session (删除整个目录)
mgr.RemoveSession("sess_abc123")

// 清理过期 Sessions (>24h 未活动)
tools.CleanupStaleSessions("/path/to/sessions")
```

---

*文档结束*

**文档版本历史**:
- v1.0 (2026-05-10): 初始版本 - 问题识别与初步方案
- v2.0 (2026-05-12): 四层目录架构 + Agent Native 哲学
- **v2.1 (2026-05-12)**: **✅ Agent Native 沙箱架构实施完成**
