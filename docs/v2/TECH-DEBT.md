# MindX V2 技术债务追踪（更新）

> 最后更新：2026-03-05
>
> 目的：明确追踪所有技术债务，确保在后续版本中解决

---

## 🚨 P0 - 严重技术债（必须在 Phase 2 解决）

### TD-001: Skill 概念完全错误

**位置**：
- `internal/core/skillmgr.go`
- `internal/usecase/skills/`
- `internal/usecase/brain/processors/skill_processor.go`

**问题描述**：
现有 `SkillManager` 将 Skill 等同于可执行工具，完全违背 [agentskills.io](https://agentskills.io/specification) 规范。

**预计工作量**：20 天

**参考文档**：
- `docs/v2/04-skill-system.md`
- `docs/v2/skill-format-spec.md`
- https://agentskills.io/specification

---

### TD-002: SkillMatchProcessor 只是占位符

**位置**：
- `internal/usecase/brain/processors/skill_processor.go`

**问题描述**：
当前 SkillMatchProcessor 只做关键词匹配，不加载 SKILL.md 的 SOP 内容，不动态组装 Tools。

**预计工作量**：5 天

---

### TD-003: KeywordIndex 是临时方案

**位置**：
- `internal/usecase/skills/keyword_index.go`

**问题描述**：
只做简单的关键词匹配，不支持语义理解。

**预计工作量**：5 天

---

### TD-008: 左右脑架构已废弃 ❌❌❌

**位置**：
- `internal/usecase/brain/brain_pipeline.go`
- `internal/infrastructure/bootstrap/assistant.go`

**问题描述**：
**严重错误**：在新 Pipeline 架构中仍然使用 V1 的左右脑概念。

**错误实现**：
```go
// ❌ 错误：新架构不应该有左右脑
leftBrain := NewThinking(leftModel, ...)
rightBrain := NewThinking(rightModel, ...)

return &core.Brain{
    LeftBrain:  leftBrain,
    RightBrain: rightBrain,
    ...
}
```

**正确实现**：
```go
// ✅ 正确：新架构只需要一个 Thinking 实例
thinking := NewThinking(model, ...)

// 降级策略在 IntentProcessor 内部实现
// 不需要两个独立的 Brain
```

**影响**：
- 架构概念混乱
- 违背新架构设计原则
- 增加不必要的复杂度
- 浪费资源（创建两个 LLM 实例）

**解决方案**：
1. 删除 `brain_pipeline.go` 中的左右脑创建
2. 只创建一个 Thinking 实例
3. IntentProcessor 内部实现本地→云端降级
4. 更新 `core.Brain` 接口，移除 LeftBrain/RightBrain 字段

**预计工作量**：2 天

**优先级**：🔴 最高（立即修复）

---

## 🟡 P1 - 重要技术债（Phase 3 解决）

### TD-004: 缺少情感分析

**预计工作量**：5 天

---

### TD-005: 缺少多轮澄清

**预计工作量**：10 天

---

## 🟢 P2 - 优化（Phase 4 解决）

### TD-006: 性能未优化

**预计工作量**：5 天

---

### TD-007: 记忆检索使用关键词匹配

**预计工作量**：3 天

---

## 📊 技术债统计（更新）

| 优先级 | 数量 | 预计工作量 |
|--------|------|-----------|
| P0 | 4 | 32 天 |
| P1 | 2 | 15 天 |
| P2 | 2 | 8 天 |
| **总计** | **8** | **55 天** |

---

## 🚨 立即行动

### TD-008 修复计划（2 天）

#### Day 1: 重构 Brain 创建
1. 删除左右脑概念
2. 只创建一个 Thinking 实例
3. 更新 IntentProcessor 的降级逻辑

#### Day 2: 更新接口和测试
1. 更新 `core.Brain` 接口
2. 修复所有测试
3. 验证集成

---

## 🎓 经验教训（更新）

### 严重错误
1. ❌ **没有理解新架构的核心理念** - 仍然使用 V1 的左右脑概念
2. ❌ **没有仔细阅读设计文档** - 新架构明确废弃了左右脑
3. ❌ **盲目复用旧代码** - 应该从零开始实现新架构

### 正确做法
1. ✅ 先理解新架构的核心理念
2. ✅ 严格按照设计文档实现
3. ✅ 不要盲目复用旧代码

---

**最后更新**：2026-03-05
**下次 Review**：2026-03-06（明天，修复 TD-008）
