# MindX 目录体系设计方案 v2.0

> 版本: 2.0  
> 日期: 2026-05-12  
> 状态: 方案草案  
> 关联文件: [PATHS_DEFINE.md](./PATHS_DEFINE.md)

---

## 一、问题背景与本质

### 1.1 现状问题

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

---

## 二、四层目录架构

### 2.1 架构总览

```
┌─────────────────────────────────────────────────────────────┐
│                    MindX 目录体系 v2.0                        │
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
│  │  Layer 3: SESSION_DIR (会话沙箱目录)      │                │
│  │  ═══════════════════════════════════     │                │
│  │  路径:    <HOME>/sessions/<agent>/<id>/  │                │
│  │  生命周期: 随 Session 创建/销毁            │                │
│  │  职责:    会话级别临时文件 & 隔离沙箱      │                │
│  │  决定者:   MindX + GoReact SessionStore   │                │
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

**实现位置**: 待新增 (见第四章 Session 元数据扩展)

#### Layer 3: SESSION_DIR (会话沙箱)

```
路径结构:
  <HOME_DIR>/sessions/
  └── <agent_name>/
      └── <session_id>/
          ├── meta.json           # ★ 新增: 会话元数据 (含 PROJECT_DIR)
          ├── session.yml         # 对话消息
          ├── usages.yml          # Token 用量
          └── tmp/                # 临时文件沙箱
              ├── artifacts/      # 生成的文件
              └── uploads/        # 上传的文件

用途:
  - 存储会话级别的临时文件 (报告、缓存、数据库等)
  - 作为沙箱隔离的安全边界之一
  - 会话结束后可配置自动清理
```

**实现位置**: [pkg/session/file_store.go](../pkg/session/file_store.go) (需扩展)

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

**实现位置**: [goreact/tools/run_script.go](../../goreact/tools/run_script.go) (需增强)

---

## 三、Agent Native 目录哲学

### 3.1 核心理念

> **不要用规则约束 LLM，而是用清晰的语义引导 LLM 自主决策。**

LLM 具备强大的语义理解能力。当它理解了每个目录的"职责"后，就能像人类工程师一样自主判断文件应该存到哪里。

### 3.2 目录角色类比

```
想象你是一个工程师，被邀请到用户的办公室结对编程:

📂 PROJECT_DIR = 用户的工作台 (办公桌)
   - 这里有他们正在进行的项目
   - 你修改的代码要放在这里
   - 这里的一切都是持久的、重要的
   
📂 SESSION_DIR = 你的草稿纸 (白板/笔记本)
   - 你在这里画图、做笔记、写草稿
   - 给用户看的分析报告先写在这里
   - 临时的计算结果、调试信息放这里
   - 会议结束可以擦掉，但当下很有用
```

### 3.3 语义声明 (注入到 System Prompt)

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

## 四、技术实施方案

### 4.1 架构分层：GoReact vs MindX 职责划分

```
┌─────────────────────────────────────────────────────────────┐
│                     MindX (Application Layer)               │
│  ═════════════════════════════════════════════════════      │
│                                                             │
│  ★ 职责: 定义目录语义、注入 Prompt、管理 Session 元数据       │
│                                                             │
│  ┌──────────────────────────────────────────────────┐       │
│  │  internal/core/app.go                           │       │
│  │  ├─ CreateSession() → 捕获 os.Getwd()            │       │
│  │  ├─ BuildSystemPrompt() → 注入目录语义说明 ★      │       │
│  │  └─ ResolveAgentOptions() → 传递上下文给 GoReact  │       │
│  ├──────────────────────────────────────────────────┤       │
│  │  pkg/session/meta.go (新增)                      │       │
│  │  ├─ SessionMeta 结构体定义                       │       │
│  │  └─ 序列化/反序列化 meta.json                    │       │
│  ├──────────────────────────────────────────────────┤       │
│  │  internal/svc/scheduler.go (修改)                │       │
│  │  └─ ExecuteTask() → Chdir 到 Session 的 PROJECT_DIR│       │
│  └──────────────────────────────────────────────────┘       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
                          ↕ 调用
┌─────────────────────────────────────────────────────────────┐
│                   GoReact (Framework Layer)                  │
│  ═════════════════════════════════════════════════════      │
│                                                             │
│  ★ 职责: 提供基础设施能力，不绑定特定目录语义                 │
│                                                             │
│  ┌──────────────────────────────────────────────────┐       │
│  │  reactor/prompt.go                               │       │
│  │  ├─ BuildEnvironmentInfo() → 接收参数，不硬编码语义│       │
│  │  └─ 返回环境信息 (HOME/PROJECT/SESSION 由调用方填) │       │
│  ├──────────────────────────────────────────────────┤       │
│  │  tools/*.go                                      │       │
│  │  ├─ write.go, read.go, edit.go, bash.go          │       │
│  │  ├─ 支持 session: 前缀解析 (最小化语法糖)         │       │
│  │  └─ 不做启发式推断 (信任调用方的 Prompt 引导)     │       │
│  ├──────────────────────────────────────────────────┤       │
│  │  core/session.go (接口层)                         │       │
│  │  └─ SessionStore 接口 (不关心内部目录结构)        │       │
│  └──────────────────────────────────────────────────┘       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**关键原则**:
- **GoReact 不知道** "什么是项目目录"、"什么是会话目录"
- **GoReact 只知道** "有一个字符串叫 ProjectDir，一个叫 SessionDir"
- **MindX 负责** 赋予这些字符串语义，并通过 Prompt 传达给 LLM

### 4.2 Session 元数据扩展

#### 新增文件: `pkg/session/meta.go`

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

#### 修改: `internal/core/app.go` - 创建 Session 时捕获 CWD

```go
// CreateSession 创建新会话并捕获当前工作目录
func (a *App) CreateSession(agentName string) (*session.SessionMeta, error) {
	// ★ 关键: 捕获当前工作目录作为 PROJECT_DIR
	projectCWD, err := os.Getwd()
	if err != nil {
		// Fallback: 使用 HOME_DIR
		projectCWD = a.Settings().UserPreferences()
		a.logger.Warn("failed to get cwd, using home dir as fallback",
			"error", err,
		)
	}

	sessionID := generateSessionID()
	
	// 创建元数据
	meta, err := session.NewSessionMeta(sessionID, agentName, projectCWD)
	if err != nil {
		return nil, fmt.Errorf("create session meta: %w", err)
	}

	// 构建会话目录路径
	sessionBaseDir := a.Settings().SessionsDir()
	agentDir := filepath.Join(sessionBaseDir, agentName)
	sessionDir := filepath.Join(agentDir, sessionID)
	tmpDir := filepath.Join(sessionDir, "tmp")

	// 创建目录结构
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	// ★ 持久化元数据
	if err := meta.Save(sessionDir); err != nil {
		return nil, fmt.Errorf("save session meta: %w", err)
	}

	a.logger.Info("session created",
		"session_id", sessionID,
		"agent", agentName,
		"project_dir", projectCWD,
		"session_dir", sessionDir,
	)

	return meta, nil
}
```

### 4.3 System Prompt 注入点 (MindX 层)

#### 修改: `internal/core/app.go` 或新增 `internal/core/prompts.go`

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

#### 集成到 Agent 初始化流程:

```go
// internal/core/app.go - GetMaster() 方法中集成
func (a *App) getMaster() (*goreact.Agent, error) {
	// ... 已有的 agent/model/rules 加载逻辑 ...

	// ★ 获取或创建当前 Session 的元数据
	meta, err := a.GetOrCreateCurrentSession(masterAgent.Name)
	if err != nil {
		a.logger.Warn("failed to get session meta, using defaults", "error", err)
	}

	// ★ 构建目录语义引导词 (MindX 层!)
	directoryGuidelines := ""
	if meta != nil {
		directoryGuidelines = core.BuildDirectoryGuidelines(
			meta.ProjectWorkingDir,
			filepath.Join(a.Settings().SessionsDir(), masterAgent.Name, meta.SessionID),
		)
	}

	opts := []goreact.AgentOption{
		goreact.WithSkillDir(a.settings.SkillsDir()),
		goreact.WithConfig(masterAgent),
		goreact.WithModel(masterModel),
		goreact.WithLogger(a.logger),
		
		// ★ 传递自定义 System Prompt 片段
		goreact.WithSystemPromptAddon(directoryGuidelines),
		
		// ... 其他选项 ...
	}

	m, err := goreact.NewAgent(opts...)
	// ...
}
```

### 4.4 GoReact 层的最小化支持

GoReact 只需要提供**极简的基础设施**，不做语义判断：

#### 修改: `goreact/reactor/prompt.go` - 参数化环境信息

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

#### 新增 (可选): `goreact/options.go` - WithSystemPromptAddon

```go
// WithSystemPromptAddon 追加自定义 System Prompt 片段
// 允许应用层注入自己的语义说明，而不修改 GoReact 核心代码
func WithSystemPromptAddon(addon string) AgentOption {
	return func(a *Agent) error {
		a.systemPromptAddons = append(a.systemPromptAddons, addon)
		return nil
	}
}
```

#### 修改: `goreact/tools/write.go` - 支持前缀解析

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

### 4.5 Daemon 同步机制

#### 修改: `internal/svc/scheduler.go`

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

## 五、数据流与交互序列

### 5.1 Session 创建流程 (TUI 启动)

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
[App.CreateSession("personal-assistant")]  ★ 新增
  │
  ├─ os.Getwd() → "/Users/ray/project-a"   ★ 捕获 PROJECT_DIR
  ├─ sessionID = generateID() → "sess_abc123"
  ├─ meta = SessionMeta{
  │     ProjectWorkingDir: "/Users/ray/project-a",
  │     HomeDir: "~/.mindx",
  │     SessionID: "sess_abc123",
  │     AgentName: "personal-assistant",
  │   }
  ├─ 创建目录: ~/.mindx/sessions/personal-assistant/sess_abc123/tmp/
  └─ 写入: meta.json  ★ 持久化
  │
  ▼
[App.getMaster()]  ★ 修改
  │
  ├─ 读取 meta.json
  ├─ guidelines = BuildDirectoryGuidelines(
  │     "/Users/ray/project-a",           // PROJECT_DIR
  │     "~/.mindx/sessions/.../sess_abc123"  // SESSION_DIR
  │   )
  ├─ opts = append(opts, WithSystemPromptAddons(guidelines))  ★ 注入
  │
  ▼
[goreact.NewAgent(opts...)]
  │
  └─ Agent 的 System Prompt 包含:
     ├─ GoReact 基础环境信息 (参数化的)
     └─ ★ MindX 目录语义引导词 (应用特定的)
  │
  ▼
[用户开始对话...]
  │
  └─ AI 已知:
     - PROJECT_DIR = /Users/ray/project-a
     - SESSION_DIR = ~/.mindx/sessions/.../sess_abc123
     - 各自的职责和使用场景
```

### 5.2 文件操作流程 (AI 决策)

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
  └─ 实际写入: ~/.mindx/sessions/.../sess_abc123/refactoring_report.md  ✅
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

### 5.3 Daemon 定时任务流程

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
  │
  ▼
[任务完成]
  │
  └─ defer: os.Chdir(originalCWD)  ★ 恢复!
```

---

## 六、实施路线图

### Phase 1: 数据模型 (预计 1-2 天)

**目标**: 建立 Session 元数据基础设施

- [ ] 创建 `pkg/session/meta.go`
  - [ ] 定义 `SessionMeta` 结构体
  - [ ] 实现 `NewSessionMeta()`, `Save()`, `LoadSessionMeta()`
- [ ] 修改 `pkg/session/file_store.go`
  - [ ] 增加 `GetSessionMeta(sessionID)` 方法
  - [ ] 在 `Append()` 时更新 `meta.json` 的 `UpdatedAt`
- [ ] 更新 `internal/core/config.go`
  - [ ] `MindxConfig` 增加 `LastProjectDir string` 字段
  - [ ] 在切换会话时持久化

**验证**: 单元测试覆盖 meta.json 的读写

---

### Phase 2: Session 创建集成 (预计 1 天)

**目标**: 创建 Session 时自动捕获 PROJECT_DIR

- [ ] 修改 `internal/core/app.go`
  - [ ] 新增 `CreateSession(agentName)` 方法
  - [ ] 集成 `os.Getwd()` 捕获
  - [ ] 调用 `meta.Save(sessionDir)`
- [ ] 修改 `internal/client/` (TUI 层)
  - [ ] 启动时调用 `CreateSession()`
  - [ ] 传递 `meta` 给 Agent 初始化流程

**验证**: 手动启动 mindx，检查生成的 `meta.json` 内容正确

---

### Phase 3: System Prompt 注入 (预计 1 天)

**目标**: MindX 层注入目录语义引导

- [ ] 创建 `internal/core/prompts.go`
  - [ ] 定义 `DirectorySemanticsPrompt` 常量
  - [ ] 实现 `BuildDirectoryGuidelines(projectDir, sessionDir)`
- [ ] 修改 `internal/core/app.go`
  - [ ] `getMaster()` 中构建 guidelines
  - [ ] 通过 `WithSystemPromptAddon()` 传递
- [ ] (可选) 修改 `goreact/options.go`
  - [ ] 新增 `WithSystemPromptAddon(addon)` Option

**验证**: 启动对话，让 AI 执行文件操作，观察其路径选择是否符合预期

---

### Phase 4: 工具层最小支持 (预计 1 天)

**目标**: 支持 `session:` 前缀语法

- [ ] 修改 `goreact/tools/write.go`
  - [ ] 实现 `resolveTargetPath()`
  - [ ] 日志中记录 resolved path 和 scope
- [ ] 修改 `goreact/tools/read.go`
  - [ ] 同上
- [ ] 修改 `goreact/tools/edit.go`
  - [ ] 同上
- [ ] (可选) `goreact/tools/bash.go`
  - [ ] 输出重定向感知

**验证**: 测试 `session:` 前缀和默认行为的路径解析

---

### Phase 5: Daemon 同步 (预计 2 天)

**目标**: Daemon 执行任务时恢复 PROJECT_DIR

- [ ] 修改 `internal/svc/scheduler.go`
  - [ ] `ExecuteScheduledTask()` 中加载 meta
  - [ ] 实现 Chdir/Restore 逻辑
  - [ ] 设置环境变量
- [ ] 编写集成测试
  - [ ] Mock 不同 CWD 场景
  - [ ] 验证任务执行前后 CWD 一致性

**验证**: 设置定时任务，在不同目录启动 Daemon，检查任务执行结果

---

### Phase 6: 文档与迭代 (持续)

- [ ] 更新 `docs/PATHS_DEFINE.md` 为 v2.0 定义
- [ ] 更新 `INSTALL.md` 中的目录说明
- [ ] 收集真实使用反馈
- [ ] 迭代优化 `DirectorySemanticsPrompt` 措辞

---

## 七、风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| LLM 仍然选错目录 | 文件存错位置 | 收集错误案例，优化 Prompt 措辞；增加 `_scope` 显式参数作为逃生舱 |
| `session:` 前缀与现有文件名冲突 | 意外路由到会话目录 | 前缀只在工具入口解析，不影响文件系统本身 |
| Daemon Chdir 失败 | 任务无法执行 | Fallback 到 HOME_DIR；记录详细日志；任务状态标记为 failed |
| 元数据文件损坏 | 无法恢复 Session 信息 | 定期备份；损坏时重建元数据（扫描 session.yml） |
| 性能开销 (每次文件操作都解析) | 微小延迟 | 解析逻辑极轻量（字符串前缀匹配）；可后续缓存 |

---

## 八、成功指标

1. **语义准确率**: AI 自主选择的目录符合预期 > 90% (通过日志统计)
2. **显式使用率**: `session:` 前缀使用率 < 10% (说明大多数时候 AI 能自主判断正确)
3. **Daemon 成功率**: 定时任务的文件操作成功率 > 99%
4. **向后兼容**: 现有不使用 meta.json 的 Session 仍能正常工作 (优雅降级)

---

## 九、未来展望

### 9.1 多项目并行支持

当前设计已自然支持：每个 Session 可以有不同的 PROJECT_DIR。未来可以扩展 UI：

```
┌─────────────────────────────────────┐
│  Sessions                            │
├─────────────────────────────────────┤
│  📁 project-a (active)              │
│     ~/workspaces/project-a          │
│                                     │
│  📁 project-b                       │
│     ~/workspaces/project-b          │
│                                     │
│  ➕ New Session (current dir)       │
└─────────────────────────────────────┘
```

### 9.2 Skill 级别的目录声明

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

### 9.3 会话迁移

有了明确的 PROJECT_DIR 记录，可以实现：

- 在不同机器上恢复 Session（只要项目路径相同）
- 导出 Session 时包含完整的上下文信息
- 调试时精确定位问题发生的目录环境

---

## 十、总结

本方案的核心创新点：

1. **四层目录架构**: HOME / PROJECT / SESSION / SCRIPT，各司其职
2. **Agent Native 哲学**: 语义引导优于规则约束
3. **清晰的分层边界**: GoReact 提供能力，MindX 定义语义
4. **最小化实现**: 不需要复杂的启发式引擎，只需一段好的 Prompt + 简单的前缀语法
5. **渐进式演进**: 不破坏现有架构，可以分阶段实施

**最终目标**: 让 LLM 像人类工程师一样，直觉性地知道"这个文件应该放在哪"。

---

*文档结束*
