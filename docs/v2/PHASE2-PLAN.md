# Phase 2 重构计划：Skill 系统重构

> 创建日期：2026-03-05
>
> 目标：彻底重构 Skill 系统，解决 TD-001, TD-002, TD-003

---

## 🎯 核心目标

1. **重新定义 Skill** - 从"可执行工具"改为"SOP 知识文档"
2. **实现 SKILL.md 解析** - 支持 agentskills.io 规范
3. **向量化语义匹配** - 替换关键词匹配
4. **动态工具组装** - 运行时根据 SOP 组装 Tools

---

## 📋 技术债清单

### TD-001: Skill 概念完全错误 ❌

**当前错误实现**：
```go
// ❌ 错误：Skill 被定义为可执行函数
type Skill struct {
    GetName     func() string
    Execute     func(name string, params map[string]interface{}) error
    ExecuteFunc func(function ToolCallFunction) error
}
```

**正确实现**：
```go
// ✅ 正确：Skill 是 SOP 知识文档
type Skill struct {
    Name        string
    Description string
    Goal        string      // 技能目标
    Triggers    []string    // 触发条件
    SOP         string      // 标准操作程序（Markdown）
    Tools       []string    // 所需工具列表
    Embedding   []float32   // 向量表示
}
```

### TD-002: SkillMatchProcessor 只是占位符 ❌

**当前问题**：
- 不加载 SKILL.md 内容
- 不解析 SOP
- `assembleTools()` 返回空列表

**需要实现**：
- SKILL.md 解析器
- SOP 内容加载
- 动态工具查找和组装

### TD-003: KeywordIndex 是临时方案 ❌

**当前问题**：
- 只做简单关键词匹配
- 不支持语义理解

**需要实现**：
- 向量化索引
- 语义相似度搜索
- 混合检索（向量 + 关键词）

---

## 🏗️ 重构架构

### 新的 Skill 系统架构

```
┌─────────────────────────────────────────────────────────┐
│                    Skill System                         │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌──────────────┐      ┌──────────────┐               │
│  │ SKILL.md     │─────▶│ SkillParser  │               │
│  │ (agentskills)│      │              │               │
│  └──────────────┘      └──────┬───────┘               │
│                               │                        │
│                               ▼                        │
│                        ┌──────────────┐               │
│                        │ Skill Entity │               │
│                        │ (SOP Doc)    │               │
│                        └──────┬───────┘               │
│                               │                        │
│                ┌──────────────┼──────────────┐        │
│                ▼              ▼              ▼        │
│         ┌───────────┐  ┌───────────┐  ┌──────────┐  │
│         │ Vector    │  │ Keyword   │  │ Tool     │  │
│         │ Indexer   │  │ Indexer   │  │ Assembler│  │
│         └─────┬─────┘  └─────┬─────┘  └────┬─────┘  │
│               │              │              │        │
│               └──────────────┼──────────────┘        │
│                              ▼                        │
│                       ┌──────────────┐               │
│                       │ SkillMatcher │               │
│                       │ (Hybrid)     │               │
│                       └──────────────┘               │
└─────────────────────────────────────────────────────────┘
```

---

## 📝 实施步骤

### Step 0: 现有 SKILL.md 规范化分析（1天）

**文件**：`docs/v2/SKILL-MIGRATION-ANALYSIS.md`

**任务**：
1. 扫描 `skills/` 目录下所有 SKILL.md
2. 分析当前 meta 定义格式
3. 识别不符合 agentskills.io 规范的部分
4. 制定迁移映射规则

**当前格式问题**：
```yaml
---
name: web_search
description: 网页搜索技能
version: 1.0.0
category: general              # ❌ 不符合规范
tags: [search, web]
os: [darwin, linux]            # ❌ 不符合规范
enabled: true                  # ❌ 不符合规范
timeout: 60                    # ❌ 不符合规范
is_internal: true              # ❌ 不符合规范
parameters:                    # ❌ 这是 Tool 的定义，不是 Skill
  terms:
    type: string
    description: 搜索关键词
    required: true
guidance: |                    # ⚠️ 应该在 SOP 中
  当用户要求"搜一下"时使用
---

# 网页搜索技能

使用 DuckDuckGo 搜索...    # ⚠️ 这是 Tool 的说明，不是 Skill SOP
```

**正确格式（agentskills.io）**：
```yaml
---
name: weather_query
description: 查询天气信息的标准操作程序
version: 1.0.0
author: mindx
tags: [weather, query, 天气]
required_tools: [web_search, http_request]
optional_tools: [location_service]
---

# Goal

查询指定地点的天气信息，包括温度、湿度、风速等。

# Triggers

- 用户询问天气
- 用户提到"天气"、"气温"、"下雨"等关键词
- 用户询问是否需要带伞

# SOP

1. 提取地点信息
   - 如果用户未指定地点，询问"您想查询哪里的天气？"
   - 如果用户说"这里"，使用 location_service 获取当前位置

2. 调用天气 API
   - 使用 web_search 工具搜索 "{地点} 天气"
   - 或使用 http_request 调用天气 API

3. 解析结果并生成响应
   - "今天{地点}的天气是{天气状况}，温度{温度}℃..."

# Examples

**用户**: 北京天气怎么样？
**助手**: 今天北京的天气是晴，温度15℃。
```

**迁移映射规则**：
```
当前字段          →  新字段/处理方式
─────────────────────────────────────
name              →  name（保持）
description       →  description（改为 SOP 描述）
version           →  version（保持）
category          →  删除（用 tags 替代）
tags              →  tags（保持）
os                →  删除（移到 Tool 定义）
enabled           →  删除（运行时决定）
timeout           →  删除（移到 Tool 定义）
is_internal       →  删除（移到 Tool 定义）
parameters        →  删除（移到 Tool 定义）
guidance          →  合并到 Triggers 或 SOP
正文内容          →  重写为 Goal + Triggers + SOP + Examples
```

**输出**：
- 迁移分析报告
- 迁移映射规则文档
- 需要手动处理的 Skills 清单

---

### Step 1: 重新定义 Skill Entity（2天）

**文件**：`internal/entity/skill.go`

**任务**：
1. 删除旧的 `core.Skill` 定义
2. 创建新的 `entity.Skill` 结构
3. 定义 SKILL.md 的数据模型

**新结构**：
```go
// Skill 技能定义（SOP 知识文档）
type Skill struct {
    // 基础信息
    Name        string
    Description string
    Version     string
    Author      string

    // 核心内容
    Goal        string      // 技能目标
    Triggers    []string    // 触发条件列表
    SOP         string      // 标准操作程序（Markdown）
    Examples    []string    // 使用示例

    // 工具依赖
    RequiredTools []string  // 必需工具
    OptionalTools []string  // 可选工具

    // 索引
    Tags       []string     // 标签
    Keywords   []string     // 关键词
    Embedding  []float32    // 向量表示

    // 元数据
    FilePath   string       // SKILL.md 文件路径
    UpdatedAt  time.Time
}
```

---

### Step 2: 实现 SKILL.md 解析器（3天）

**文件**：`internal/usecase/skills/parser.go`

**任务**：
1. 解析 YAML frontmatter
2. 解析 Markdown 内容
3. 提取 Goal, Triggers, SOP, Tools

**SKILL.md 格式**：
```markdown
---
name: weather_query
description: 查询天气信息
version: 1.0.0
author: mindx
tags: [weather, query]
required_tools: [web_search, http_request]
optional_tools: [location_service]
---

# Goal

查询指定地点的天气信息，包括温度、湿度、风速等。

# Triggers

- 用户询问天气
- 用户提到"天气"、"气温"、"下雨"等关键词
- 用户询问是否需要带伞

# SOP

1. 提取地点信息
   - 如果用户未指定地点，询问"您想查询哪里的天气？"
   - 如果用户说"这里"或"当前位置"，使用 location_service 获取

2. 调用天气 API
   - 使用 web_search 工具搜索 "{地点} 天气"
   - 或使用 http_request 调用天气 API

3. 解析结果
   - 提取温度、湿度、风速、天气状况
   - 格式化为友好的回复

4. 生成响应
   - "今天{地点}的天气是{天气状况}，温度{温度}℃..."

# Examples

**用户**: 北京天气怎么样？
**助手**: 今天北京的天气是晴，温度15℃，湿度45%，风速3m/s。

**用户**: 明天会下雨吗？
**助手**: 您想查询哪里的天气？
**用户**: 上海
**助手**: 明天上海有小雨，温度12-18℃，建议带伞。
```

**解析器接口**：
```go
type SkillParser interface {
    // Parse 解析 SKILL.md 文件
    Parse(filePath string) (*entity.Skill, error)

    // ParseContent 解析 SKILL.md 内容
    ParseContent(content []byte) (*entity.Skill, error)
}
```

---

### Step 3: 实现向量化索引（5天）

**文件**：`internal/usecase/skills/vector_index.go`

**任务**：
1. 生成 Skill 的向量表示（Goal + Triggers）
2. 存储到 BadgerDB
3. 实现向量相似度搜索

**向量生成策略**：
```go
// 组合 Goal + Triggers 生成向量
text := skill.Goal + "\n" + strings.Join(skill.Triggers, "\n")
embedding := embeddingService.Embed(text)
skill.Embedding = embedding
```

**索引结构**：
```go
type VectorIndex struct {
    db              *badger.DB
    embeddingService embedding.Service
    dimension       int
}

// Index 索引单个 Skill
func (idx *VectorIndex) Index(skill *entity.Skill) error

// Search 向量相似度搜索
func (idx *VectorIndex) Search(query string, topK int) ([]*SkillMatch, error)

// SearchByEmbedding 直接使用向量搜索
func (idx *VectorIndex) SearchByEmbedding(embedding []float32, topK int) ([]*SkillMatch, error)
```

---

### Step 4: 实现混合检索（3天）

**文件**：`internal/usecase/skills/hybrid_searcher.go`

**任务**：
1. 结合向量搜索和关键词搜索
2. 实现分数融合策略
3. 优化检索性能

**混合检索策略**：
```go
type HybridSearcher struct {
    vectorIndex  *VectorIndex
    keywordIndex *KeywordIndex

    // 权重配置
    vectorWeight  float64  // 向量搜索权重（默认 0.7）
    keywordWeight float64  // 关键词搜索权重（默认 0.3）
}

// Search 混合检索
func (s *HybridSearcher) Search(query string, topK int) ([]*SkillMatch, error) {
    // 1. 向量搜索
    vectorMatches := s.vectorIndex.Search(query, topK*2)

    // 2. 关键词搜索
    keywords := extractKeywords(query)
    keywordMatches := s.keywordIndex.Search(keywords, topK*2)

    // 3. 分数融合
    finalMatches := s.mergeResults(vectorMatches, keywordMatches)

    // 4. 返回 TopK
    return finalMatches[:topK], nil
}
```

---

### Step 5: 实现动态工具组装（4天）

**文件**：`internal/usecase/skills/tool_assembler.go`

**任务**：
1. 根据 Skill.RequiredTools 查找工具
2. 支持本地工具和 MCP 工具
3. 生成 OpenAI Tools Schema

**工具组装器**：
```go
type ToolAssembler struct {
    localTools  map[string]*LocalTool   // 本地工具注册表
    mcpClients  map[string]*MCPClient   // MCP 客户端
}

// AssembleTools 组装工具
func (a *ToolAssembler) AssembleTools(skill *entity.Skill) ([]entity.ToolSchema, error) {
    schemas := []entity.ToolSchema{}

    // 1. 查找必需工具
    for _, toolName := range skill.RequiredTools {
        schema, err := a.findTool(toolName)
        if err != nil {
            return nil, fmt.Errorf("required tool %s not found: %w", toolName, err)
        }
        schemas = append(schemas, schema)
    }

    // 2. 查找可选工具（失败不影响）
    for _, toolName := range skill.OptionalTools {
        schema, err := a.findTool(toolName)
        if err != nil {
            log.Warn("optional tool not found", "tool", toolName)
            continue
        }
        schemas = append(schemas, schema)
    }

    return schemas, nil
}

// findTool 查找工具（本地 + MCP）
func (a *ToolAssembler) findTool(name string) (entity.ToolSchema, error) {
    // 1. 查找本地工具
    if tool, ok := a.localTools[name]; ok {
        return tool.ToSchema(), nil
    }

    // 2. 查找 MCP 工具
    for _, client := range a.mcpClients {
        if schema, err := client.GetTool(name); err == nil {
            return schema, nil
        }
    }

    return entity.ToolSchema{}, fmt.Errorf("tool %s not found", name)
}
```

---

### Step 6: 重构 SkillMatchProcessor（3天）

**文件**：`internal/usecase/brain/processors/skill_processor.go`

**任务**：
1. 使用新的混合检索器
2. 加载完整的 SOP 内容
3. 动态组装工具

**新实现**：
```go
type SkillMatchProcessor struct {
    searcher      *HybridSearcher
    toolAssembler *ToolAssembler
    topK          int
    logger        logging.Logger
}

func (p *SkillMatchProcessor) Process(ctx context.Context, thinkCtx *entity.ThinkContext) error {
    // 1. 检查意图
    if thinkCtx.Intent == nil {
        return nil
    }

    // 2. 混合检索 Skills
    query := thinkCtx.Input
    matches, err := p.searcher.Search(query, p.topK)
    if err != nil {
        p.logger.Warn("skill search failed", logging.Err(err))
        return nil
    }

    if len(matches) == 0 {
        p.logger.Debug("no skills matched")
        return nil
    }

    // 3. 选择最优 Skill
    bestMatch := matches[0]
    skill := bestMatch.Skill

    // 4. 加载完整 SOP
    thinkCtx.MatchedSkills = []*entity.SkillSOP{
        {
            Name:          skill.Name,
            Description:   skill.Description,
            Keywords:      skill.Keywords,
            RequiredTools: skill.RequiredTools,
            SOPContent:    skill.SOP,  // ✅ 加载完整 SOP
        },
    }

    // 5. 动态组装工具
    tools, err := p.toolAssembler.AssembleTools(skill)
    if err != nil {
        p.logger.Error("tool assembly failed", logging.Err(err))
        return fmt.Errorf("failed to assemble tools: %w", err)
    }

    thinkCtx.Tools = tools  // ✅ 动态组装的工具

    p.logger.Info("skill matching completed",
        logging.String("skill", skill.Name),
        logging.Float64("score", bestMatch.Score),
        logging.Int("tools_count", len(tools)),
    )

    return nil
}
```

---

### Step 7: 迁移现有 SKILL.md（5天）

**文件**：`scripts/migrate_skills.go`

**任务**：
1. 实现自动迁移工具
2. 批量迁移所有 SKILL.md
3. 人工审核和调整
4. 生成迁移报告

**迁移工具**：
```go
type SkillMigrator struct {
    parser      *OldSkillParser   // 解析旧格式
    generator   *NewSkillGenerator // 生成新格式
    validator   *SkillValidator    // 验证新格式
}

// Migrate 迁移单个 Skill
func (m *SkillMigrator) Migrate(oldPath string) (*MigrationResult, error) {
    // 1. 解析旧格式
    oldSkill := m.parser.Parse(oldPath)

    // 2. 转换为新格式
    newSkill := m.convertToNewFormat(oldSkill)

    // 3. 验证新格式
    if err := m.validator.Validate(newSkill); err != nil {
        return nil, err
    }

    // 4. 生成新 SKILL.md
    newContent := m.generator.Generate(newSkill)

    // 5. 写入文件
    newPath := strings.Replace(oldPath, ".md", ".new.md", 1)
    os.WriteFile(newPath, []byte(newContent), 0644)

    return &MigrationResult{
        OldPath: oldPath,
        NewPath: newPath,
        Status:  "success",
    }, nil
}

// convertToNewFormat 转换格式
func (m *SkillMigrator) convertToNewFormat(old *OldSkill) *NewSkill {
    return &NewSkill{
        Name:        old.Name,
        Description: fmt.Sprintf("%s的标准操作程序", old.Description),
        Version:     old.Version,
        Author:      "mindx",
        Tags:        old.Tags,

        // 从 guidance 提取 Triggers
        Triggers: extractTriggers(old.Guidance),

        // 从正文生成 SOP
        SOP: generateSOP(old.Content),

        // 从 parameters 提取 required_tools
        RequiredTools: extractRequiredTools(old.Parameters),
    }
}
```

**迁移步骤**：
```bash
# 1. 备份原始文件
cp -r skills/ skills.backup/

# 2. 运行迁移工具
go run scripts/migrate_skills.go --input skills/ --output skills.new/

# 3. 人工审核
# 检查 skills.new/ 中的文件，调整 SOP 内容

# 4. 替换原文件
rm -rf skills/
mv skills.new/ skills/

# 5. 生成迁移报告
go run scripts/migrate_skills.go --report
```

**需要手动处理的 Skills**：
- 复杂的 guidance 逻辑
- 多步骤的操作流程
- 特殊的参数处理

**验收标准**：
- [ ] 所有 SKILL.md 符合 agentskills.io 规范
- [ ] 迁移成功率 > 90%
- [ ] 人工审核通过
- [ ] 生成详细的迁移报告

---

### Step 8: 删除遗留代码（2天）

**要删除的文件**：
- `internal/core/skillmgr.go` - 旧的 Skill 接口定义
- `internal/usecase/skills/skill_mgr.go` - 旧的 SkillManager 实现
- `internal/usecase/skills/keyword_index.go` - 临时关键词索引（保留作为混合检索的一部分）

**要重构的文件**：
- `internal/infrastructure/bootstrap/assistant.go` - 更新 Skill 系统初始化
- `internal/adapters/cli/skill.go` - 更新 CLI 命令
- `internal/adapters/http/skill.go` - 更新 HTTP API

**迁移策略**：
1. 先实现新系统
2. 并行运行新旧系统（灰度）
3. 验证新系统稳定后删除旧代码

**注意**：
- ⚠️ 不要删除 `keyword_index.go`，它将作为混合检索的一部分保留
- ⚠️ Tools 和 MCP 相关代码暂时保留，等待 Phase 3 处理

---

## 🧪 测试策略

### 单元测试

```go
// parser_test.go
func TestSkillParser_Parse(t *testing.T) {
    parser := NewSkillParser()
    skill, err := parser.Parse("testdata/weather_query.md")

    assert.NoError(t, err)
    assert.Equal(t, "weather_query", skill.Name)
    assert.NotEmpty(t, skill.Goal)
    assert.NotEmpty(t, skill.SOP)
    assert.Contains(t, skill.RequiredTools, "web_search")
}

// vector_index_test.go
func TestVectorIndex_Search(t *testing.T) {
    idx := NewVectorIndex(db, embeddingService)

    // 索引测试数据
    skill := &entity.Skill{
        Name: "weather_query",
        Goal: "查询天气信息",
        Triggers: []string{"天气", "气温"},
    }
    idx.Index(skill)

    // 搜索
    matches, err := idx.Search("今天天气怎么样", 3)

    assert.NoError(t, err)
    assert.Len(t, matches, 1)
    assert.Equal(t, "weather_query", matches[0].Skill.Name)
}

// tool_assembler_test.go
func TestToolAssembler_AssembleTools(t *testing.T) {
    assembler := NewToolAssembler(localTools, mcpClients)

    skill := &entity.Skill{
        RequiredTools: []string{"web_search"},
        OptionalTools: []string{"location_service"},
    }

    tools, err := assembler.AssembleTools(skill)

    assert.NoError(t, err)
    assert.Len(t, tools, 2)
}
```

### 集成测试

```go
// skill_processor_integration_test.go
func TestSkillMatchProcessor_Integration(t *testing.T) {
    // 1. 准备测试环境
    db := setupTestDB(t)
    embeddingService := setupEmbeddingService(t)

    // 2. 加载测试 Skills
    parser := NewSkillParser()
    skills := loadTestSkills(t, parser)

    // 3. 构建索引
    vectorIndex := NewVectorIndex(db, embeddingService)
    for _, skill := range skills {
        vectorIndex.Index(skill)
    }

    // 4. 创建处理器
    searcher := NewHybridSearcher(vectorIndex, keywordIndex)
    assembler := NewToolAssembler(localTools, mcpClients)
    processor := NewSkillMatchProcessor(searcher, assembler, 3)

    // 5. 测试处理
    thinkCtx := &entity.ThinkContext{
        Input: "北京天气怎么样",
        Intent: &entity.IntentContext{
            Type: "weather_query",
            Keywords: []string{"北京", "天气"},
        },
    }

    err := processor.Process(context.Background(), thinkCtx)

    // 6. 验证结果
    assert.NoError(t, err)
    assert.Len(t, thinkCtx.MatchedSkills, 1)
    assert.Equal(t, "weather_query", thinkCtx.MatchedSkills[0].Name)
    assert.NotEmpty(t, thinkCtx.MatchedSkills[0].SOPContent)
    assert.NotEmpty(t, thinkCtx.Tools)
}
```

---

## 📊 时间估算

| 步骤 | 任务 | 工作量 |
|------|------|--------|
| Step 0 | 现有 SKILL.md 规范化分析 | 1 天 |
| Step 1 | 重新定义 Skill Entity | 2 天 |
| Step 2 | 实现 SKILL.md 解析器 | 3 天 |
| Step 3 | 实现向量化索引 | 5 天 |
| Step 4 | 实现混合检索 | 3 天 |
| Step 5 | 实现动态工具组装 | 4 天 |
| Step 6 | 重构 SkillMatchProcessor | 3 天 |
| Step 7 | 迁移现有 SKILL.md | 5 天 |
| Step 8 | 删除遗留代码 | 2 天 |
| **总计** | | **28 天** |

**注意**：
- Step 7（迁移现有 SKILL.md）是新增步骤，需要 5 天
- 迁移工具可以自动化大部分工作，但需要人工审核
- 复杂的 Skills 可能需要手动重写 SOP

---

## ✅ 验收标准

### 功能验收

- [ ] 支持 agentskills.io 规范的 SKILL.md 格式
- [ ] 所有现有 SKILL.md 已迁移到新格式
- [ ] 迁移成功率 > 90%
- [ ] 向量相似度搜索准确率 > 85%
- [ ] 混合检索效果优于纯关键词匹配
- [ ] 动态工具组装成功率 > 95%
- [ ] SkillMatchProcessor 正确加载 SOP 内容
- [ ] 所有单元测试通过
- [ ] 所有集成测试通过

### 性能验收

- [ ] Skill 索引构建时间 < 1s/skill
- [ ] 向量搜索响应时间 < 100ms
- [ ] 工具组装时间 < 200ms
- [ ] 内存占用合理（< 500MB for 1000 skills）

### 代码质量

- [ ] 测试覆盖率 > 80%
- [ ] 无遗留的旧代码
- [ ] 无版本号命名（v1, v2）
- [ ] 代码审查通过

---

## 🚨 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| 向量化性能不达标 | 高 | 使用缓存，优化索引结构 |
| SKILL.md 格式不统一 | 中 | 提供迁移工具和验证器 |
| 工具查找失败 | 中 | 优雅降级，记录警告 |
| 旧系统依赖难以解耦 | 高 | 先并行运行，逐步迁移 |

---

## 📚 参考文档

- [agentskills.io Specification](https://agentskills.io/specification)
- `docs/v2/04-skill-system.md` - Skill 系统设计
- `docs/v2/skill-format-spec.md` - Skill 格式规范
- `docs/v2/TECH-DEBT.md` - 技术债务追踪

---

## 🔗 相关文档

- `docs/v2/HIDDEN-DEBT-TOOLS-MCP.md` - Tools 与 MCP 隐含技术债
- `docs/v2/TECH-DEBT.md` - 技术债务追踪
- `docs/v2/04-skill-system.md` - Skill 系统设计
- `docs/v2/skill-format-spec.md` - Skill 格式规范

---

## ⚠️ 重要提醒

### Phase 2 完成后立即启动 Phase 3

Phase 2 重构 Skill 系统后，会暴露 Tools 与 MCP 的架构问题：
- Skills 和 Tools 需要完全解耦
- Tools 需要独立的目录和管理器
- MCP 需要独立的配置和管理器

**详见**：`docs/v2/HIDDEN-DEBT-TOOLS-MCP.md`

**时间估算**：
- Phase 2（Skill 重构）：28 天
- Phase 3（Tools 与 MCP 重构）：15 天
- **总计**：43 天

---

**创建时间**：2026-03-05
**更新时间**：2026-03-05（新增 Step 0 和 Step 7）
**预计完成**：2026-04-02（28 个工作日）
