# Phase 4 Step 1 进度报告：SkillMgr 重构遇到的挑战

> 更新日期：2026-03-06
>
> 状态：⚠️ 遇到技术挑战，需要重新评估

---

## 📊 当前进度

**已完成**：30%
- ✅ 新 SkillManager 核心实现（320 行）
- ✅ 9 个单元测试通过
- ✅ 基础 UI 管理功能

**遇到的问题**：
- ❌ 旧 SkillMgr 有 20+ 个方法需要实现
- ❌ 类型不匹配（entity.Skill vs core.Skill）
- ❌ 涉及 10+ 个文件需要修改
- ❌ MCP 相关方法需要重新设计

---

## 🔍 问题分析

### 1. 旧 SkillMgr 的复杂性

**缺失的方法**（需要实现）：
```go
// 执行相关
ExecuteByName(name, params) error
ExecuteFunc(name) (func, error)

// MCP 相关
GetMCPServers() []MCPServer
AddMCPServer(server) error
RemoveMCPServer(name) error
RestartMCPServer(name) error
GetMCPServerTools(name) []Tool

// 其他
SearchSkills(keywords) ([]string, error)  // 已实现
InitMCPServers(ctx, config)               // 已实现（no-op）
IsVectorTableEmpty() bool                 // 已实现
StartReIndexInBackground()                // 已实现
```

### 2. 类型不匹配问题

**问题**：
- 新代码使用 `entity.Skill`
- 旧代码使用 `core.Skill`
- 两者不兼容

**影响范围**：
- `internal/adapters/http/handlers/skills.go`
- `internal/infrastructure/bootstrap/assistant.go`
- `internal/usecase/brain/tool_caller.go`

### 3. 接口不匹配

**core.SkillManager 接口**：
```go
type SkillManager interface {
    Execute(skill *core.Skill, params map[string]interface{}) error
    // ... 其他方法
}
```

**新 SkillManager 实现**：
```go
func (sm *SkillManager) Execute(skill *entity.Skill, params map[string]any) error
```

**问题**：参数类型不匹配

---

## 💡 解决方案评估

### 方案 1：完整重构（预计 3-5 天）

**工作量**：
1. 实现所有缺失的方法（20+ 个）
2. 解决类型不匹配问题
3. 更新所有引用（10+ 个文件）
4. 重新设计 MCP 集成
5. 更新所有测试

**优点**：
- 彻底解决技术债
- 架构更清晰

**缺点**：
- 工作量大
- 风险高
- 可能引入新 bug

### 方案 2：渐进式重构（推荐）

**阶段 1**：保留旧 SkillMgr，新增 SkillManager 用于 UI
- 两者并存
- UI 使用新的 SkillManager
- Brain 继续使用旧的 SkillMgr
- 工作量：1-2 天

**阶段 2**：逐步迁移功能
- 先迁移简单功能
- 逐个文件更新
- 充分测试
- 工作量：2-3 天

**阶段 3**：删除旧代码
- 确保所有功能正常
- 删除旧 SkillMgr
- 工作量：1 天

### 方案 3：先完成其他工作（最实际）

**优先级调整**：
1. ✅ Phase 3 文档更新（1 天）
2. ✅ Phase 3 完成报告（已完成）
3. ⏳ SkillMgr 重构作为独立 Phase 5（3-5 天）

**理由**：
- Phase 3 核心架构已完成
- 文档更重要
- SkillMgr 重构需要更多时间规划

---

## 📈 工作量对比

| 方案 | 预计时间 | 风险 | 优先级 |
|------|---------|------|--------|
| 方案 1：完整重构 | 3-5 天 | 高 | 低 |
| 方案 2：渐进式重构 | 4-6 天 | 中 | 中 |
| 方案 3：先完成文档 | 1 天 | 低 | 高 |

---

## 🎯 建议

**立即行动**：
1. 先完成 Phase 3 文档更新
2. 创建 Phase 3 完成报告（已完成）
3. 将 SkillMgr 重构作为独立的 Phase 5

**Phase 5 规划**：
- 详细分析旧 SkillMgr 的所有功能
- 设计新的接口和架构
- 制定详细的迁移计划
- 分阶段实施

**理由**：
- Phase 3 的核心目标已达成（Tools/Skills 解耦）
- 文档对用户更重要
- SkillMgr 重构需要更充分的规划

---

## ✅ 已完成的工作

**新 SkillManager**：
- ✅ 320 行核心代码
- ✅ 9 个单元测试
- ✅ 基础 UI 管理功能
- ✅ 部分兼容方法

**价值**：
- 为未来重构打下基础
- 验证了新架构的可行性
- 提供了清晰的职责分离

---

## 🚀 下一步

**推荐路径**：
1. 完成 Phase 3 文档更新（1 天）
2. 规划 Phase 5：SkillMgr 完整重构（3-5 天）
3. 或者：先进行端到端测试（需要解决编译问题）

**决策点**：
- 是否继续强行完成 SkillMgr 重构？（3-5 天，高风险）
- 还是先完成文档，将重构作为独立 Phase？（推荐）

---

**更新时间**：2026-03-06
**状态**：等待决策
