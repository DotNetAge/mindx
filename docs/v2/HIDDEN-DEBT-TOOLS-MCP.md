# 隐含技术债：Tools 与 MCP 架构重构

> 创建日期：2026-03-05
>
> 状态：📝 已识别，待 Phase 2 完成后处理
>
> 优先级：P0（阻塞 Phase 3）

---

## 🚨 问题描述

### 架构根本性变化

**V1 架构（错误）**：
```
Skills 目录
├── skill_name/
│   ├── SKILL.md          ← Skill 定义
│   ├── tool.json         ← Tool 定义（存储在 Skill 目录下）
│   └── main.go/py/js     ← Tool 可执行文件
```

**问题**：
- Skills 和 Tools 耦合在一起
- Tool 存储在 Skills 目录下
- SkillManager 同时管理 Skills 和 Tools
- MCP 工具也混在 Skills 目录中

---

**V2 架构（正确）**：
```
Skills 目录（纯 SOP 知识）
├── weather_query/
│   └── SKILL.md          ← 只有 SOP 文档，声明需要的工具

Tools 目录（独立管理）
├── web_search/
│   ├── tool.json         ← Tool 定义
│   └── main.go           ← Tool 可执行文件
├── http_request/
│   ├── tool.json
│   └── main.py

MCP 配置（独立管理）
└── mcp_servers.json      ← MCP 服务器配置
```

**变化**：
- Skills 和 Tools 完全解耦
- Skills 只是 SOP 文档，声明需要哪些工具
- Tools 独立存储和索引
- MCP 独立配置和管理

---

## 📊 影响范围

### 1. 存储结构变化

**当前（错误）**：
```go
// SkillManager 同时管理 Skills 和 Tools
type SkillManager struct {
    skillsDir string  // skills/ 目录
    skills    map[string]*Skill
    tools     map[string]*Tool  // Tools 存储在 Skills 目录下
}
```

**应该（正确）**：
```go
// SkillManager 只管理 Skills（SOP 文档）
type SkillManager struct {
    skillsDir string
    skills    map[string]*Skill  // 只有 SOP
}

// ToolManager 独立管理 Tools
type ToolManager struct {
    toolsDir string  // tools/ 目录（独立）
    tools    map[string]*Tool
}

// MCPManager 独立管理 MCP
type MCPManager struct {
    configPath string  // mcp_servers.json
    clients    map[string]*MCPClient
}
```

---

### 2. 索引方式变化

**当前（错误）**：
```go
// Skills 和 Tools 混在一起索引
skillManager.SearchSkills(keywords)  // 返回 Skills + Tools
```

**应该（正确）**：
```go
// Skills 独立索引（向量 + 关键词）
skillManager.SearchSkills(query)  // 返回 SOP 文档

// Tools 独立索引（按名称、功能）
toolManager.SearchTools(name)  // 返回 Tool Schema

// MCP 独立查询
mcpManager.GetTool(name)  // 从 MCP 服务器获取
```

---

### 3. 运行时组装变化

**当前（错误）**：
```go
// Skill 直接包含 Tool 的执行逻辑
skill.Execute(params)  // Skill 自己执行
```

**应该（正确）**：
```go
// 1. 匹配 Skill（SOP 文档）
skill := skillManager.SearchSkills(query)[0]

// 2. 解析 SOP，提取所需工具
requiredTools := skill.RequiredTools  // ["web_search", "http_request"]

// 3. 动态查找工具
tools := []ToolSchema{}
for _, toolName := range requiredTools {
    // 先查本地 Tools
    if tool := toolManager.GetTool(toolName); tool != nil {
        tools = append(tools, tool.ToSchema())
        continue
    }

    // 再查 MCP
    if tool := mcpManager.GetTool(toolName); tool != nil {
        tools = append(tools, tool)
        continue
    }

    // 工具未找到
    log.Warn("tool not found", "name", toolName)
}

// 4. 将 SOP + Tools 传给 LLM
thinkCtx.MatchedSkills = []*SkillSOP{skill}
thinkCtx.Tools = tools
```

---

## 🔧 需要重构的组件

### 1. SkillManager

**当前职责（错误）**：
- 加载 Skills
- 加载 Tools（❌ 不应该）
- 执行 Tools（❌ 不应该）
- 管理 MCP（❌ 不应该）

**应该职责（正确）**：
- 加载 SKILL.md（SOP 文档）
- 解析 SOP 内容
- 索引 Skills（向量 + 关键词）
- 搜索 Skills

---

### 2. ToolManager（新增）

**职责**：
- 加载本地 Tools（从 `tools/` 目录）
- 索引 Tools（按名称、功能）
- 提供 Tool Schema
- 执行本地 Tool

**接口**：
```go
type ToolManager interface {
    // LoadTools 加载所有本地工具
    LoadTools(toolsDir string) error

    // GetTool 获取单个工具
    GetTool(name string) (*Tool, error)

    // SearchTools 搜索工具
    SearchTools(query string) ([]*Tool, error)

    // ExecuteTool 执行工具
    ExecuteTool(name string, params map[string]interface{}) (string, error)
}
```

---

### 3. MCPManager（新增）

**职责**：
- 加载 MCP 配置（`mcp_servers.json`）
- 连接 MCP 服务器
- 获取 MCP Tools
- 执行 MCP Tools

**接口**：
```go
type MCPManager interface {
    // LoadConfig 加载 MCP 配置
    LoadConfig(configPath string) error

    // GetTool 获取 MCP 工具
    GetTool(name string) (*MCPTool, error)

    // ListTools 列出所有 MCP 工具
    ListTools() ([]*MCPTool, error)

    // ExecuteTool 执行 MCP 工具
    ExecuteTool(name string, params map[string]interface{}) (string, error)
}
```

---

### 4. ToolAssembler（新增）

**职责**：
- 根据 Skill.RequiredTools 动态查找工具
- 优先查找本地 Tools
- 回退到 MCP Tools
- 生成 OpenAI Tools Schema

**接口**：
```go
type ToolAssembler interface {
    // AssembleTools 组装工具
    AssembleTools(skill *Skill) ([]ToolSchema, error)

    // FindTool 查找单个工具（本地 + MCP）
    FindTool(name string) (ToolSchema, error)
}
```

---

## 📁 目录结构变化

### 当前结构（错误）

```
mindx/
├── skills/                    ← Skills 和 Tools 混在一起
│   ├── web_search/
│   │   ├── SKILL.md          ← Skill 定义
│   │   ├── tool.json         ← Tool 定义（❌ 不应该在这里）
│   │   └── main.go           ← Tool 可执行文件（❌ 不应该在这里）
│   ├── calculator/
│   │   ├── SKILL.md
│   │   ├── tool.json
│   │   └── main.py
```

### 新结构（正确）

```
mindx/
├── skills/                    ← 只有 SOP 文档
│   ├── weather_query/
│   │   └── SKILL.md          ← 纯 SOP，声明需要 web_search, http_request
│   ├── code_review/
│   │   └── SKILL.md          ← 纯 SOP，声明需要 github, file_search
│
├── tools/                     ← 本地工具（独立）
│   ├── web_search/
│   │   ├── tool.json         ← Tool 定义
│   │   └── main.go           ← Tool 可执行文件
│   ├── http_request/
│   │   ├── tool.json
│   │   └── main.py
│   ├── github/
│   │   ├── tool.json
│   │   └── main.js
│
├── config/
│   └── mcp_servers.json      ← MCP 配置（独立）
```

---

## 🔄 迁移策略

### Phase 1: 识别和分类（当前阶段）

1. ✅ 识别问题
2. ✅ 记录隐含债务
3. ⏳ 等待 Skill 重构完成

### Phase 2: Skill 重构（进行中）

1. 重新定义 Skill（SOP 文档）
2. 实现 SKILL.md 解析器
3. 实现向量化索引
4. 实现动态工具组装

### Phase 3: Tools 与 MCP 重构（待 Phase 2 完成）

#### Step 1: 创建新目录结构（1天）

```bash
# 创建 tools/ 目录
mkdir -p tools/

# 迁移工具文件
for skill_dir in skills/*/; do
    if [ -f "$skill_dir/tool.json" ]; then
        tool_name=$(basename "$skill_dir")
        mkdir -p "tools/$tool_name"
        mv "$skill_dir/tool.json" "tools/$tool_name/"
        mv "$skill_dir/main."* "tools/$tool_name/" 2>/dev/null || true
    fi
done
```

#### Step 2: 实现 ToolManager（3天）

```go
// internal/usecase/tools/tool_manager.go
type ToolManager struct {
    toolsDir string
    tools    map[string]*Tool
    logger   logging.Logger
}

func (m *ToolManager) LoadTools(toolsDir string) error {
    // 扫描 tools/ 目录
    // 加载 tool.json
    // 索引工具
}

func (m *ToolManager) GetTool(name string) (*Tool, error) {
    // 查找工具
}

func (m *ToolManager) ExecuteTool(name string, params map[string]interface{}) (string, error) {
    // 执行工具
}
```

#### Step 3: 实现 MCPManager（3天）

```go
// internal/usecase/mcp/mcp_manager.go
type MCPManager struct {
    configPath string
    clients    map[string]*MCPClient
    logger     logging.Logger
}

func (m *MCPManager) LoadConfig(configPath string) error {
    // 加载 mcp_servers.json
    // 连接 MCP 服务器
}

func (m *MCPManager) GetTool(name string) (*MCPTool, error) {
    // 从 MCP 服务器获取工具
}
```

#### Step 4: 实现 ToolAssembler（2天）

```go
// internal/usecase/skills/tool_assembler.go
type ToolAssembler struct {
    toolManager *ToolManager
    mcpManager  *MCPManager
}

func (a *ToolAssembler) AssembleTools(skill *Skill) ([]ToolSchema, error) {
    // 根据 skill.RequiredTools 查找工具
    // 优先本地，回退 MCP
}
```

#### Step 5: 更新 SkillMatchProcessor（1天）

```go
// 使用 ToolAssembler
tools, err := p.toolAssembler.AssembleTools(skill)
thinkCtx.Tools = tools
```

#### Step 6: 删除旧代码（2天）

- 删除 SkillManager 中的 Tool 管理逻辑
- 删除 Skills 目录下的 tool.json 和可执行文件
- 更新所有引用

#### Step 7: 测试和验证（3天）

- 单元测试
- 集成测试
- 端到端测试

**总计**：15 天

---

## 📊 时间估算

| 阶段 | 任务 | 工作量 |
|------|------|--------|
| Phase 2 | Skill 重构 | 22 天 |
| Phase 3 | Tools 与 MCP 重构 | 15 天 |
| **总计** | | **37 天** |

---

## ✅ 验收标准

### 架构验收

- [ ] Skills 和 Tools 完全解耦
- [ ] Skills 目录只包含 SKILL.md（SOP 文档）
- [ ] Tools 目录独立管理本地工具
- [ ] MCP 配置独立管理

### 功能验收

- [ ] SkillManager 只管理 Skills（SOP）
- [ ] ToolManager 正确加载和执行本地工具
- [ ] MCPManager 正确连接和执行 MCP 工具
- [ ] ToolAssembler 正确动态组装工具

### 代码质量

- [ ] 无遗留的耦合代码
- [ ] 测试覆盖率 > 80%
- [ ] 所有测试通过

---

## 🚨 风险

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| 现有 Skills 依赖旧结构 | 高 | 提供自动迁移脚本 |
| Tool 查找失败 | 中 | 优雅降级，记录警告 |
| MCP 连接失败 | 中 | 回退到本地工具 |
| 迁移工作量大 | 高 | 分阶段实施，先并行运行 |

---

## 📚 相关文档

- `docs/v2/PHASE2-PLAN.md` - Phase 2 Skill 重构计划
- `docs/v2/04-skill-system.md` - Skill 系统设计
- `docs/v2/TECH-DEBT.md` - 技术债务追踪

---

**结论**：这是一个根本性的架构变化，必须在 Phase 2 完成后立即处理，否则会阻塞后续开发。

**建议**：
1. ✅ 先完成 Phase 2（Skill 重构）
2. ⏳ 立即启动 Phase 3（Tools 与 MCP 重构）
3. 🎯 两个阶段总计 37 天

---

**创建时间**：2026-03-05
**预计开始**：Phase 2 完成后（约 2026-03-27）
**预计完成**：2026-04-11（Phase 2 + Phase 3）
