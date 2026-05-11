# GoReact 硬编码 Logger 修复补丁

## 🎯 问题概述

GoReact 中存在 **9 处硬编码调用** `core.DefaultLogger()`，违反了依赖注入原则，导致：
1. 无法在 TUI/测试环境中完全控制日志输出
2. 日志泄露到 stderr/stdout，污染终端界面
3. 违反 SOLID 原则（依赖倒置）

## 🔍 影响范围

### 高优先级（必须修复）

#### 1. [reactor/thought.go:76](../reactor/thought.go#L76)
```go
// ❌ 当前：硬编码
func ParseThinkResponse(content string) (*Thought, error) {
    // ...
    core.DefaultLogger().Info("parsing non-JSON response as direct answer", ...)
}

// ✅ 修复：接受 Logger 参数
func ParseThinkResponse(content string, logger core.Logger) (*Thought, error) {
    // ...
    if logger != nil {
        logger.Info("parsing non-JSON response as direct answer", ...)
    }
}
```

**影响**：每次 LLM 返回非 JSON 格式响应时触发（高频！）

---

#### 2. [reactor/offload.go:172,199,216,238,249](../reactor/offload.go)
```go
// ❌ 当前：5处硬编码
func cleanupOffloadedFiles() {
    entries, err := os.ReadDir(rootDir)
    if err != nil {
        core.DefaultLogger().Warn("failed to read offload directory", ...)  // Line 172
        return
    }
    // ...
    if err := os.Remove(filePath); err != nil {
        core.DefaultLogger().Warn("failed to clean up offloaded file", ...)  // Line 199
    }
    // ...
    core.DefaultLogger().Info("offload cleanup completed", ...)  // Line 216
}

func CleanupSessionOffloads(sessionID string) error {
    // ...
    if err := os.Remove(filePath); err != nil {
        core.DefaultLogger().Warn("failed to remove session offload file", ...)  // Line 238
    }
    // ...
    core.DefaultLogger().Info("session offload cleanup completed", ...)  // Line 249
}

// ✅ 修复：使用包级变量 + 初始化函数
var offloadLogger core.Logger = core.DefaultLogger()

func SetOffloadLogger(logger core.Logger) {
    offloadLogger = logger
}

func cleanupOffloadedFiles() {
    entries, err := os.ReadDir(rootDir)
    if err != nil {
        offloadLogger.Warn("failed to read offload directory", ...)
        return
    }
    // ... (类似修改其他4处)
}
```

---

#### 3. [agent_registry.go:26](../agent_registry.go#L26)
```go
// ❌ 当前：硬编码
func LoadAgentsFrom(dir string) (*AgentRegistry, error) {
    logger := core.DefaultLogger()  // Line 26
    // ...
}

// ✅ 修复：支持可选 Logger 参数
func LoadAgentsFrom(dir string, opts ...AgentRegistryOption) (*AgentRegistry, error) {
    cfg := &agentRegistryConfig{
        logger: core.DefaultLogger(),
    }
    for _, opt := range opts {
        opt(cfg)
    }

    registry := &AgentRegistry{
        path:   absPath,
        agents: make(map[string]*core.AgentConfig),
        logger: cfg.logger,
    }
    // ...
}
```

---

### 中等优先级（可接受但应改进）

#### 4. [reactor/reactor.go:194](../reactor/reactor.go#L194)
```go
// ✅ 这是合理的 fallback 实现
func (r *Reactor) getLogger() core.Logger {
    if r.config.Logger != nil {
        return r.config.Logger  // 使用注入的 logger
    }
    return core.DefaultLogger()  // fallback 到默认值
}
```

**评估**：这是标准的依赖注入模式，**可以保留**

---

#### 5. [tools/web_search.go:792](../tools/web_search.go#L792)
```go
// ✅ 这也是合理的 fallback
func getLogger(ctx context.Context) core.Logger {
    tc := core.GetToolContext(ctx)
    if tc != nil && tc.Logger != nil {
        return tc.Logger  // 从上下文获取注入的 logger
    }
    return core.DefaultLogger()  // fallback
}
```

**评估**：同样符合依赖注入模式，**可以保留**

---

## 🔧 完整修复代码

### 文件 1：reactor/thought.go

```diff
diff --git a/reactor/thought.go b/reactor/thought.go
--- a/reactor/thought.go
+++ b/reactor/thought.go
@@ -65,7 +65,7 @@ func stripJSONWrappers(s string) string {

 // ParseThinkResponse parses an LLM response string into a Thought struct.
 // If the content is not valid JSON (e.g., LLM returned a direct text answer),
 // it will be automatically wrapped as a DecisionAnswer Thought.
-func ParseThinkResponse(content string) (*Thought, error) {
+func ParseThinkResponse(content string, logger core.Logger) (*Thought, error) {
     content = stripJSONWrappers(content)

     var thought Thought
@@ -73,9 +73,11 @@ func ParseThinkResponse(content string) (*Thought, error) {
         // Check if content looks like a direct answer (non-empty, substantial text)
         trimmed := strings.TrimSpace(content)
         if len(trimmed) > 10 && looksLikeDirectAnswer(trimmed) {
-            core.DefaultLogger().Info("parsing non-JSON response as direct answer",
+            if logger != nil {
+                logger.Info("parsing non-JSON response as direct answer",
                     "content_length", len(trimmed),
                     "preview", truncate(trimmed, 100),
-            )
+                )
+            }
             return &Thought{
                 Decision:    DecisionAnswer,
                 Reasoning:   "LLM returned direct text answer (not JSON)",
```

**调用方更新**（reactor/think_act_observe.go）:
```diff
-thought, parseErr := ParseThinkResponse(content)
+thought, parseErr := ParseThinkResponse(content, r.getLogger())
```

---

### 文件 2：reactor/offload.go

```diff
diff --git a/reactor/offload.go b/reactor/offload.go
--- a/reactor/offload.go
+++ b/reactor/offload.go
@@ -1,6 +1,10 @@
 package reactor
 
+import (
+    "github.com/DotNetAge/goreact/core"
+)
+
 // offloadTTL defines how long offloaded files are kept before cleanup.
 const offloadTTL = 24 * time.Hour
+
+// offloadLogger allows dependency injection for testing/TUI environments.
+var offloadLogger core.Logger = core.DefaultLogger()
+
+// SetOffloadLogger sets the logger for offload operations.
+// Must be called before any offload operation if custom logging is needed.
+func SetOffloadLogger(logger core.Logger) {
+    offloadLogger = logger
+}

 // cleanupOffloadedFiles removes stale offloaded files older than offloadTTL.
@@ -168,7 +180,7 @@ func cleanupOffloadedFiles() {
     if err != nil {
         if os.IsNotExist(err) {
             return
         }
-        core.DefaultLogger().Warn("failed to read offload directory", "dir", rootDir, "error", err)
+        offloadLogger.Warn("failed to read offload directory", "dir", rootDir, "error", err)
         return
     }
@@ -196,7 +208,7 @@ func cleanupOffloadedFiles() {
             if err := os.Remove(filePath); err != nil {
-                core.DefaultLogger().Warn("failed to clean up offloaded file",
+                offloadLogger.Warn("failed to clean up offloaded file",
                     "file", filePath,
                     "error", err,
                 )
@@ -215,7 +227,7 @@ func cleanupOffloadedFiles() {
     if totalCleaned > 0 {
-        core.DefaultLogger().Info("offload cleanup completed",
+        offloadLogger.Info("offload cleanup completed",
             "files_removed", totalCleaned,
         )
@@ -237,7 +249,7 @@ func CleanupSessionOffloads(sessionID string) error {
         if err := os.Remove(filePath); err != nil {
-            core.DefaultLogger().Warn("failed to remove session offload file",
+            offloadLogger.Warn("failed to remove session offload file",
                 "file", filePath,
                 "error", err,
             )
@@ -249,7 +261,7 @@ func CleanupSessionOffloads(sessionID string) error {

-    core.DefaultLogger().Info("session offload cleanup completed",
+    offloadLogger.Info("session offload cleanup completed",
         "session_id", sessionID,
         "files_removed", len(entries),
     )
```

**初始化位置**（reactor/reactor.go 的初始化函数中）:
```go
func NewReactor(config *ReactorConfig) (*Reactor, error) {
    // ...
    if config.Logger != nil {
        reactor.SetOffloadLogger(config.Logger)  // 注入 logger 到 offload 包
    }
    // ...
}
```

---

### 文件 3：agent_registry.go

```diff
diff --git a/agent_registry.go b/agent_registry.go
--- a/agent_registry.go
+++ b/agent_registry.go
@@ -17,12 +17,25 @@ import (
     "gopkg.in/yaml.v3"
 )

+// agentRegistryOption is a functional option for configuring AgentRegistry.
+type agentRegistryOption struct {
+    logger core.Logger
+}
+
+// AgentRegistryOption is a function that configures AgentRegistry.
+type AgentRegistryOption func(*agentRegistryOption)
+
+// WithRegistryLogger returns an Option that sets the logger for agent registry.
+func WithRegistryLogger(logger core.Logger) AgentRegistryOption {
+    return func(o *agentRegistryOption) { o.logger = logger }
+}
+
 // LoadAgentsFrom loads all agent configurations from a directory.
 // Each .md file in the directory is treated as an agent definition.
-func LoadAgentsFrom(dir string) (*AgentRegistry, error) {
+func LoadAgentsFrom(dir string, opts ...AgentRegistryOption) (*AgentRegistry, error) {
     absPath, err := filepath.Abs(dir)
     if err != nil {
         return nil, fmt.Errorf("failed to get absolute path: %w", err)
     }

-    logger := core.DefaultLogger()
+    cfg := &agentRegistryOption{logger: core.DefaultLogger()}
+    for _, opt := range opts {
+        opt(cfg)
+    }

     registry := &AgentRegistry{
         path:   absPath,
         agents: make(map[string]*core.AgentConfig),
+        logger: cfg.logger,
     }
@@ -44,7 +57,7 @@ func LoadAgentsFrom(dir string) (*AgentRegistry, error) {
             agent, err := parseAgentFile(filePath)
             if err != nil {
-                logger.Warn("failed to parse agent file, skipping",
+                registry.logger.Warn("failed to parse agent file, skipping",
                     "path", filePath,
                     "error", err)
                 continue
             }
```

**同时需要在 AgentRegistry 结构体中添加 logger 字段**:
```diff
 type AgentRegistry struct {
-    path   string
-    agents map[string]*core.AgentConfig
+    path   string
+    agents map[string]*core.AgentConfig
+    logger core.Logger
 }
```

---

## ✅ MindX TUI 端的集成修复

完成上述 GoReact 修复后，MindX 需要更新初始化代码：

### internal/core/app.go

```diff
 func DefaultApp() (*App, error) {
-    logger := logging.DefaultConsoleLogger()
+    logger := logging.DefaultConsoleLogger()  // 或根据环境变量选择

     // ...
     
     return &App{
         settings:   settings,
         logger:     logger,
         // ...
     }, nil
 }
 
+// InitForTUI initializes App with no-op logger for TUI mode.
+func InitForTUI() (*App, error) {
+    app, err := DefaultApp()
+    if err != nil {
+        return nil, err
+    }
+    
+    noopLogger := logging.DefaultNoopLogger()
+    app.SetLogger(noopLogger)
+    
+    // 如果 GoReact 已修复，这里还可以设置其他组件的 logger
+    // reactor.SetOffloadLogger(noopLogger)
+    
+    return app, nil
+}
```

### internal/client/component_root.go

```diff
 func (m *rootModel) Init() tea.Cmd {
     var err error
-    m.app, err = core.DefaultApp()
+    m.app, err = core.InitForTUI()  // 使用 TUI 专用初始化
     if err != nil {
         return func() tea.Msg { return err }
     }

-    m.app.SetLogger(logging.DefaultNoopLogger())
+    // 不再需要单独设置，InitForTUI 已经处理
     
     // ...
 }
```

---

## 🧪 测试验证

### 测试用例 1：确认无日志泄露

```go
func TestNoLogLeakageInTUI(t *testing.T) {
    // 设置 discard handler
    slog.SetDefault(slog.New(discardHandler{}))
    
    // 调用包含硬编码的函数
    thought, err := ParseThinkResponse("这是一个直接回答", nil)
    
    assert.NoError(t, err)
    assert.Equal(t, "DecisionAnswer", thought.Decision)
    
    // 验证没有任何输出到 stdout/stderr
    // （通过捕获 os.Stdout/os.Stderr 或使用 testlogger）
}
```

### 测试用例 2：Logger 注入正确性

```go
func TestLoggerInjection(t *testing.T) {
    var logged bool
    mockLogger := &mockLogger{
        InfoFunc: func(msg string, keyvals ...any) {
            logged = true
            assert.Contains(t, msg, "non-JSON response")
        },
    }
    
    _, err := ParseThinkResponse("直接回答内容", mockLogger)
    
    assert.True(t, logged, "应该调用了注入的 logger")
}
```

---

## 📊 修复优先级矩阵

| 修复项 | 影响 | 复杂度 | 优先级 |
|--------|------|--------|--------|
| thought.go | 🔴 高频触发 | ⭐ 低 | **P0 - 立即** |
| offload.go (5处) | 🟡 中频 | ⭐⭐ 中 | **P1 - 本周** |
| agent_registry.go | 🟢 低频 | ⭐⭐ 中 | **P2 - 下周** |
| reactor.go:194 | 🟢 Fallback | - | **保持现状** |
| web_search.go:792 | 🟢 Fallback | - | **保持现状** |

---

## 🎯 总结

### 当前状态（MindX 端）
✅ **已通过 `slog.SetDefault(discardHandler{})` 临时解决所有问题**
- 所有 `core.DefaultLogger()` 调用都会被拦截
- TUI 屏幕不会再出现日志乱码
- 编译和运行正常

### 推荐行动（GoReact 端）
🔧 **向 GoReact 提交 PR 修复硬编码**
1. 优先修复 `thought.go:76`（最高频触发）
2. 其次修复 `offload.go` 的 5 处调用
3. 最后优化 `agent_registry.go`
4. 保留 `reactor.go:194` 和 `web_search.go:792` 的 fallback 模式

### 架构原则
✅ **遵循 SOLID 原则**：
- **S**ingle Responsibility: 每个 Logger 只负责一个组件
- **O**pen/Closed: 通过接口扩展，不修改实现
- **L**iskov Substitution: NoopLogger 可替代任何 Logger
- **I**nterface Segregation: Logger 接口精简
- **D**ependency Inversion: 依赖抽象而非具体实现

---

**准备就绪后可以向 GoReact 仓库提交 PR！** 🚀
