# MindX V2 集成方案与技术债务管理

> 日期：2026-03-05
>
> 目的：将新 Pipeline 架构集成到现有系统，同时明确标注技术债务

---

## 🚨 技术债务声明

### 严重技术债（必须在 Phase 2 解决）

#### 1. Skill 概念完全错误 ❌

**问题**：
- 现有 `SkillManager` 将 Skill 等同于可执行工具
- 完全违背 [agentskills.io](https://agentskills.io/specification) 规范
- 与新架构设计文档 `docs/v2/04-skill-system.md` 不一致

**正确定义**：
```
Skill = 声明式 SOP（标准操作程序）
Tool = 可执行工具（Go 函数或 MCP 服务）

Skill 应该：
- 定义"做什么"和"何时使用"
- 包含执行步骤的描述
- 声明所需的 Tools
- 运行时由 LLM 读取并动态组装 Tools
```

**当前错误实现**：
```go
// ❌ 错误：Skill 包含执行逻辑
type Skill struct {
    GetName     func() string
    Execute     func(name string, params map[string]interface{}) error
    ExecuteFunc func(function ToolCallFunction) error
}
```

**应该是**：
```go
// ✅ 正确：Skill 是声明式定义
type Skill struct {
    Metadata      SkillMetadata
    Goal          string
    Trigger       string
    Steps         []Step
    RequiredTools []string
}
```

#### 2. SkillMatchProcessor 只是占位符 ❌

**问题**：
- 当前实现只做关键词匹配
- 不加载 SKILL.md 的 SOP 内容
- 不动态组装 Tools
- MVP 阶段 `thinkCtx.Tools` 为空，导致 ToolExecutionProcessor 被跳过

**影响**：
- 端到端测试中工具执行被跳过
- 无法验证完整的 Pipeline 流程

#### 3. KeywordIndex 是临时方案 ❌

**问题**：
- 只做简单的关键词匹配
- 不支持语义理解
- 不符合 agentskills.io 的向量化匹配要求

---

## ✅ 集成策略：先替换 Brain，后重构 Skill

### Phase 1: 替换旧 Brain（本次完成）

**目标**：让新 Pipeline 跑起来，暂时容忍 Skill 的技术债

**步骤**：
1. 创建新的 `NewBrainWithPipeline` 构造函数
2. 在 `Assistant` 中使用新 Brain
3. 保持对外接口不变（`Ask` 方法）
4. 添加 `// TODO: TECH DEBT` 注释标注所有技术债

### Phase 2: 重构 Skill 系统（下一阶段）

**目标**：按照 agentskills.io 规范重新实现 Skill

**步骤**：
1. 重新定义 `Skill` 结构体
2. 实现 SOP 解析器
3. 实现动态工具组装
4. 实现向量化匹配
5. 迁移现有 Skills

---

## 🔧 实施细节

### 1. 创建新 Brain 构造函数

```go
// internal/usecase/brain/brain_pipeline.go

package brain

import (
    "context"
    "mindx/internal/core"
    "mindx/internal/entity"
    "mindx/internal/usecase/brain/processors"
)

// NewBrainWithPipeline 创建基于 Pipeline 的新 Brain
// TODO: TECH DEBT - 当前 SkillMatchProcessor 使用错误的 Skill 实现
// 需要在 Phase 2 按照 agentskills.io 规范重构整个 Skill 系统
func NewBrainWithPipeline(deps BrainDeps) (*core.Brain, error) {
    // 创建左脑（本地模型）
    leftBrain, err := NewThinking(/* ... */)
    if err != nil {
        return nil, err
    }

    // 创建右脑（云端模型）
    rightBrain, err := NewThinking(/* ... */)
    if err != nil {
        return nil, err
    }

    // 创建处理器管线
    pipeline := NewPipeline(
        processors.NewIntentProcessor(leftBrain, rightBrain),
        processors.NewMemoryRetrievalProcessor(deps.Memory, 5),
        // TODO: TECH DEBT - SkillMatchProcessor 使用错误的 Skill 概念
        // 当前只是占位符，不加载 SOP，不组装 Tools
        processors.NewSkillMatchProcessor(deps.SkillMgr, 3),
        processors.NewToolExecutionProcessor(rightBrain, deps.SkillMgr),
        processors.NewResponseProcessor(rightBrain),
    )

    // 包装成 Brain 接口
    return &core.Brain{
        LeftBrain:  leftBrain,
        RightBrain: rightBrain,
        Pipeline:   pipeline, // 新增字段
        Post: func(req *core.ThinkingRequest) (*core.ThinkingResponse, error) {
            return executePipeline(pipeline, req)
        },
    }, nil
}

// executePipeline 执行 Pipeline 并转换结果
func executePipeline(pipeline *Pipeline, req *core.ThinkingRequest) (*core.ThinkingResponse, error) {
    thinkCtx := entity.NewThinkContext(req.Question, req.SessionID)
    ctx := context.Background()

    if err := pipeline.Execute(ctx, thinkCtx); err != nil {
        return nil, err
    }

    return &core.ThinkingResponse{
        Answer: thinkCtx.Response,
        SendTo: thinkCtx.SendTo,
        // TODO: TECH DEBT - Tools 字段为空，因为 SkillMatchProcessor 不填充
        Tools:  []core.ToolSchema{},
    }, nil
}
```

### 2. 修改 Assistant 使用新 Brain

```go
// internal/infrastructure/bootstrap/assistant.go

func NewAssistant(/* ... */) *Assistant {
    // ... 前面的代码保持不变 ...

    // 创建大脑（使用新 Pipeline）
    // TODO: TECH DEBT - 切换到新 Pipeline，但 Skill 系统仍有问题
    brain, err := brain.NewBrainWithPipeline(brain.BrainDeps{
        Cfg:            cfg,
        Persona:        persona,
        Memory:         mem,
        SkillMgr:       skillMgr, // ❌ 这是错误的 Skill 实现
        ToolsRequest:   toolsRequest,
        CapRequest:     capRequest,
        HistoryRequest: historyRequest,
        Logger:         logger,
        TokenUsageRepo: tokenUsageRepo,
        CronScheduler:  cronScheduler,
    })
    if err != nil {
        logger.Error("创建 Brain 失败", logging.Err(err))
        return nil
    }

    return &Assistant{
        // ... 其他字段 ...
        brain: brain,
    }
}
```

### 3. 添加技术债追踪文件

```markdown
# docs/v2/TECH-DEBT.md

## 技术债务清单

### 🔴 P0 - 严重（必须在 Phase 2 解决）

#### TD-001: Skill 概念完全错误
- **位置**：`internal/core/skillmgr.go`, `internal/usecase/skills/`
- **问题**：Skill 被实现为可执行工具，违背 agentskills.io 规范
- **影响**：无法实现声明式 SOP，无法动态组装工具
- **解决方案**：按照 `docs/v2/04-skill-system.md` 重新实现
- **预计工作量**：15 天
- **负责人**：TBD
- **截止日期**：Phase 2

#### TD-002: SkillMatchProcessor 只是占位符
- **位置**：`internal/usecase/brain/processors/skill_processor.go`
- **问题**：不加载 SOP，不组装 Tools，导致 ToolExecutionProcessor 被跳过
- **影响**：Pipeline 流程不完整，无法验证端到端功能
- **解决方案**：实现 SOP 解析和动态工具组装
- **预计工作量**：7 天
- **负责人**：TBD
- **截止日期**：Phase 2

#### TD-003: KeywordIndex 是临时方案
- **位置**：`internal/usecase/skills/keyword_index.go`
- **问题**：只做关键词匹配，不支持语义理解
- **影响**：Skill 匹配准确率低
- **解决方案**：实现向量化语义匹配
- **预计工作量**：5 天
- **负责人**：TBD
- **截止日期**：Phase 2

### 🟡 P1 - 重要（Phase 3 解决）

#### TD-004: 缺少情感分析
- **位置**：`internal/usecase/brain/processors/`
- **问题**：EmotionProcessor 未实现
- **影响**：无法感知用户情感，响应不够智能
- **解决方案**：实现 EmotionProcessor
- **预计工作量**：5 天

#### TD-005: 缺少多轮澄清
- **位置**：`internal/usecase/brain/processors/`
- **问题**：ClarificationProcessor 未实现
- **影响**：低置信度意图无法澄清
- **解决方案**：实现 ClarificationProcessor
- **预计工作量**：10 天

### 🟢 P2 - 优化（Phase 4 解决）

#### TD-006: 性能未优化
- **位置**：`internal/usecase/brain/pipeline.go`
- **问题**：处理器串行执行，未并行优化
- **影响**：性能可能不达标
- **解决方案**：根据性能测试结果优化
- **预计工作量**：5 天

---

## 技术债务追踪

| ID | 标题 | 优先级 | 状态 | 负责人 | 截止日期 |
|----|------|--------|------|--------|---------|
| TD-001 | Skill 概念错误 | P0 | Open | TBD | Phase 2 |
| TD-002 | SkillMatchProcessor 占位符 | P0 | Open | TBD | Phase 2 |
| TD-003 | KeywordIndex 临时方案 | P0 | Open | TBD | Phase 2 |
| TD-004 | 缺少情感分析 | P1 | Open | TBD | Phase 3 |
| TD-005 | 缺少多轮澄清 | P1 | Open | TBD | Phase 3 |
| TD-006 | 性能未优化 | P2 | Open | TBD | Phase 4 |

---

## 技术债务影响分析

### 当前可用功能
- ✅ 意图识别（本地 + 云端降级）
- ✅ 记忆检索（关键词匹配）
- ✅ 响应生成
- ⚠️ 技能匹配（关键词匹配，不完整）
- ⚠️ 工具执行（因 Skill 问题被跳过）

### 当前不可用功能
- ❌ 声明式 SOP
- ❌ 动态工具组装
- ❌ 语义化 Skill 匹配
- ❌ 情感分析
- ❌ 多轮澄清

### 风险评估
- **高风险**：Skill 系统需要完全重构，工作量大
- **中风险**：现有 Skills 需要迁移到新格式
- **低风险**：Pipeline 架构已验证，可以逐步完善
```

---

## 📝 代码注释规范

所有技术债相关代码必须添加注释：

```go
// TODO: TECH DEBT [TD-001] - Skill 概念错误
// 当前 SkillManager 将 Skill 等同于可执行工具，违背 agentskills.io 规范
// 需要在 Phase 2 按照 docs/v2/04-skill-system.md 重新实现
// 参考：docs/v2/TECH-DEBT.md#TD-001
```

---

## 🎯 Phase 2 重构计划

### 目标
完全按照 agentskills.io 规范重新实现 Skill 系统

### 步骤

#### 1. 重新定义 Skill 结构（3 天）
```go
// internal/entity/skill_v2.go

type SkillSOP struct {
    Metadata      SkillMetadata
    Goal          string
    Trigger       string
    Prerequisites []string
    Steps         []ExecutionStep
    RequiredTools []string
    Examples      []Example
}

type ExecutionStep struct {
    Order       int
    Description string
    ToolCall    *ToolCallSpec
    Condition   string
}
```

#### 2. 实现 SOP 解析器（5 天）
- 解析 SKILL.md 的 Markdown 正文
- 提取执行步骤
- 验证 RequiredTools

#### 3. 实现动态工具组装（4 天）
- 根据 RequiredTools 查找 Tools
- 从 SkillManager 和 MCP 中查找
- 组装成 ToolSchema 列表

#### 4. 实现向量化匹配（5 天）
- 使用 embedding 生成 Skill 向量
- 实现语义相似度搜索
- 替换 KeywordIndex

#### 5. 迁移现有 Skills（3 天）
- 创建迁移工具
- 转换现有 Skills 到新格式
- 验证迁移结果

**总计：20 天**

---

## ✅ 本次集成范围

### 完成的工作
1. ✅ 创建新 Pipeline 架构
2. ✅ 实现 5 个核心处理器
3. ✅ 完整的单元测试和集成测试
4. ✅ 明确标注所有技术债务

### 不完成的工作（Phase 2）
1. ❌ 重构 Skill 系统
2. ❌ 实现 SOP 解析
3. ❌ 实现动态工具组装
4. ❌ 实现向量化匹配

### 集成后的状态
- ✅ 新 Pipeline 可以运行
- ✅ 基础功能可用（意图识别、记忆检索、响应生成）
- ⚠️ Skill 功能不完整（技术债）
- ⚠️ 工具执行被跳过（技术债）

---

## 🚦 集成检查清单

### 代码集成
- [ ] 创建 `brain_pipeline.go`
- [ ] 修改 `assistant.go` 使用新 Brain
- [ ] 添加 `// TODO: TECH DEBT` 注释
- [ ] 创建 `docs/v2/TECH-DEBT.md`

### 测试验证
- [ ] 运行所有单元测试
- [ ] 运行集成测试
- [ ] 手动测试基础对话
- [ ] 验证意图识别
- [ ] 验证记忆检索
- [ ] 验证响应生成

### 文档更新
- [ ] 更新 `CLAUDE.md`
- [ ] 创建技术债文档
- [ ] 更新 README（标注技术债）

---

## 📊 风险评估

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|---------|
| Skill 重构工作量大 | 高 | 高 | 分阶段实施，先保证基础功能 |
| 现有 Skills 迁移困难 | 中 | 中 | 创建自动化迁移工具 |
| 用户体验下降 | 中 | 低 | 保持对外接口不变 |
| 技术债被遗忘 | 高 | 中 | 明确标注，定期 Review |

---

## 🎓 经验教训

### 做对的事
1. ✅ 先实现核心架构，再完善细节
2. ✅ 明确标注技术债务
3. ✅ 保持对外接口稳定

### 需要改进
1. ⚠️ 应该先理解现有 Skill 系统再设计
2. ⚠️ 应该先验证 agentskills.io 规范的可行性
3. ⚠️ 应该更早发现 Skill 概念错误

---

**结论**：本次集成可以让新 Pipeline 跑起来，但必须在 Phase 2 解决 Skill 系统的技术债务。
