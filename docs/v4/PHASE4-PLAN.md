# Phase 4 计划：技术债清理与系统完善

> 创建日期：2026-03-06
>
> 状态：进行中

---

## 🎯 Phase 4 目标

清理 Phase 3 遗留的技术债，完善系统功能，确保整个系统可以正常编译和运行。

**核心目标**：
1. 重构 SkillMgr，分离职责
2. 完成端到端测试
3. 完善文档
4. 系统整体验证

---

## 📋 任务清单

### Step 1: 重构 SkillMgr（2-3天）🔴 高优先级

**问题分析**：
- 旧的 `skills.SkillMgr` 承担了两个职责：
  1. **向大脑提供搜索和执行** - 已被 HybridSearcher 和 ToolAssembler 替代 ✅
  2. **向 UI 提供管理功能** - 仍然需要 ❌

**解决方案**：
1. 创建新的 SkillManager 接口（只负责 UI 管理）
2. 实现轻量级 SkillManager（基于 SKILL.md 文件）
3. 更新 bootstrap 使用新组件
4. 更新 HTTP handlers
5. 处理 builtins 包（删除或重构）

**UI 管理功能需求**（从 handlers/skills.go 分析）：
```go
// 基础查询
GetSkillInfos() []*entity.SkillInfo
GetSkillInfo(name) (*entity.SkillInfo, bool)

// 索引管理
ReIndex() error
IsReIndexing() bool
GetReIndexError() error

// 技能管理
Enable(name) error
Disable(name) error
ConvertSkill(name) error
BatchConvert(names) (success, failed []string)

// 依赖管理
InstallDependency(name, method) error
InstallRuntime(name) error
BatchInstall(names) (success, failed []string)

// 技能执行（可能需要重新设计）
GetSkills() ([]*core.Skill, error)
Execute(skill, params) error
```

**实现策略**：
- 基于 SKILL.md 文件系统
- 使用 HybridSearcher 进行搜索
- 使用 ToolAssembler 进行工具组装
- 简化执行逻辑（委托给 ToolAssembler）

**验收标准**：
- [ ] 系统可以编译
- [ ] UI API 正常工作
- [ ] 所有测试通过
- [ ] 无编译错误

---

### Step 2: 完成端到端测试（1天）

**前置条件**：Step 1 完成

**测试场景**：
- [ ] 完整的对话流程
- [ ] Skill 搜索 → 工具组装 → 工具执行
- [ ] 实际工具执行（calculator）
- [ ] MCP 服务器连接（如果可用）

**文件**：
- `internal/usecase/e2e_test.go`

---

### Step 3: 完善性能测试（0.5天）

**待实现**：
- [ ] BenchmarkToolAssemble
- [ ] BenchmarkToolExecution
- [ ] 内存占用测试

**目标**：
- 工具组装时间 < 100ms
- 工具执行时间 < 5s
- 内存占用 < 100MB

---

### Step 4: 文档更新（1天）

**待更新文档**：
- [ ] README.md - 更新架构说明
- [ ] docs/v4/ARCHITECTURE.md - 完整架构文档
- [ ] docs/v4/MIGRATION-GUIDE.md - 迁移指南
- [ ] docs/v4/API.md - API 文档

---

### Step 5: 系统整体验证（0.5天）

**验证项**：
- [ ] 系统可以正常启动
- [ ] Dashboard 可以访问
- [ ] 所有 API 正常工作
- [ ] 所有测试通过
- [ ] 性能符合预期

---

## 📅 时间规划

| 步骤 | 预计时间 | 优先级 |
|------|---------|--------|
| Step 1: 重构 SkillMgr | 2-3 天 | 🔴 高 |
| Step 2: 端到端测试 | 1 天 | 🟡 中 |
| Step 3: 性能测试 | 0.5 天 | 🟡 中 |
| Step 4: 文档更新 | 1 天 | 🟢 低 |
| Step 5: 系统验证 | 0.5 天 | 🟡 中 |
| **总计** | **5-6 天** | - |

---

## ✅ 验收标准

### 功能验收
- [ ] 系统可以正常编译
- [ ] 所有 UI 功能正常
- [ ] 所有测试通过（单元 + 集成 + 端到端）
- [ ] 性能符合预期

### 质量验收
- [ ] 测试覆盖率 > 90%
- [ ] 无技术债
- [ ] 文档完整
- [ ] 代码质量良好

### 用户验收
- [ ] Dashboard 可以正常使用
- [ ] Skill 管理功能正常
- [ ] 对话功能正常
- [ ] 工具执行正常

---

**创建时间**：2026-03-06
**预计完成**：5-6 天
