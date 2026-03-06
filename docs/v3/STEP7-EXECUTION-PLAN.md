# Step 7 执行计划：测试和验证

> 创建日期：2026-03-06
>
> 状态：部分完成（受技术债阻塞）

---

## 🎯 目标

全面测试和验证新架构，确保：
1. 所有单元测试通过
2. 集成测试覆盖核心流程
3. 端到端测试验证实际功能
4. 性能符合预期
5. 文档完整

---

## 📋 任务清单

### 1. 单元测试补充（1天）

**当前状态**：
- ToolManager: 8/8 ✅
- MCPManager: 12/12 ✅
- ToolAssembler: 8/8 ✅
- SkillMatchProcessor: 11/11 ✅

**已完成**：
- [x] 所有新组件单元测试通过
- [x] 测试覆盖率 > 85%

---

### 2. 集成测试（1天）

**测试场景**：
- [x] ToolManager + ToolAssembler 集成
- [x] MCPManager + ToolAssembler 集成（跳过，需要真实 MCP 服务器）
- [x] 完整的 Skill 匹配流程
- [x] 工具加载 → 组装流程

**文件**：
- `internal/usecase/integration_test.go` ✅

**测试结果**：
```
✅ TestToolManagerIntegration - 通过
✅ TestToolAssemblerIntegration - 通过
✅ TestFullPipeline - 通过
✅ TestToolPriority - 通过
⏭️  TestMCPManagerIntegration - 跳过（需要真实 MCP 服务器）
```

---

### 3. 端到端测试（0.5天）

**状态**：❌ 受技术债阻塞

**原因**：
- 旧的 `skills.SkillMgr` 已在 Phase 2 中删除
- 但 bootstrap、HTTP handlers、brain 等模块仍在使用
- 导致编译错误，无法运行完整系统测试

**技术债**：
- `internal/infrastructure/bootstrap/app.go` - 使用 SkillMgr
- `internal/adapters/http/handlers/skills.go` - 使用 SkillMgr
- `internal/usecase/brain/brain.go` - 使用 SkillMgr
- `internal/usecase/skills/builtins/` - 已删除，依赖 SkillMgr

---

### 4. 性能测试（0.5天）

**测试指标**：
- [x] 工具加载时间 < 1s ✅ (~419µs for 10 tools)
- [ ] 工具执行时间 < 5s（待实现）
- [ ] 内存占用 < 100MB（待实现）

**文件**：
- `internal/usecase/benchmark_test.go` ✅

**测试结果**：
```
BenchmarkToolManagerLoad-12    3    418968 ns/op
```

---

### 5. 文档更新（1天）

**待更新文档**：
- [ ] README.md
- [ ] docs/v3/ARCHITECTURE.md
- [ ] docs/v3/MIGRATION-GUIDE.md
- [ ] docs/v3/PHASE3-COMPLETION-REPORT.md
- [x] docs/v3/STEP7-EXECUTION-PLAN.md（本文档）

---

## ⚠️ 技术债务

### TD-001: SkillMgr 重构

**问题描述**：
- 旧的 `skills.SkillMgr` 在 Phase 2 中被删除
- 但它承担了两个职责：
  1. 向大脑提供搜索和执行功能（已被 HybridSearcher 和 ToolAssembler 替代）
  2. 向 UI 提供 Skills 管理功能（仍然需要）

**影响范围**：
- `internal/infrastructure/bootstrap/` - 初始化依赖 SkillMgr
- `internal/adapters/http/handlers/` - UI API 依赖 SkillMgr
- `internal/usecase/brain/` - 大脑组件依赖 SkillMgr
- `internal/usecase/skills/builtins/` - 内置技能依赖 SkillMgr

**解决方案**（Phase 4）：
1. 创建新的 SkillManager 接口，只负责 UI 管理功能
2. 实现轻量级的 SkillManager（基于 SKILL.md 文件）
3. 更新 bootstrap 使用新的组件
4. 更新 HTTP handlers 使用新的 SkillManager
5. 重构或删除 builtins 包

**优先级**：高（阻塞完整系统测试）

---

## ✅ 验收标准

### 测试验收
- [x] 新组件单元测试通过（39 个测试）
- [x] 集成测试通过（4 个测试）
- [ ] 端到端测试通过（受技术债阻塞）
- [x] 性能测试达标（工具加载 < 1s）
- [x] 测试覆盖率 > 85%

### 文档验收
- [ ] README 更新
- [ ] 架构文档完整
- [ ] 迁移指南清晰
- [ ] 完成报告详细

---

## 📊 当前进度

**已完成**：
- ✅ 新组件单元测试（39 个）
- ✅ 集成测试（4 个通过，1 个跳过）
- ✅ 性能测试（工具加载）
- ✅ 修复编译错误（integration_test.go, benchmark_test.go）

**受阻**：
- ❌ 端到端测试（需要先解决 SkillMgr 技术债）
- ❌ 完整系统编译（需要先解决 SkillMgr 技术债）

**建议**：
1. 将 SkillMgr 重构作为 Phase 4 的首要任务
2. 先完成 Phase 3 文档更新
3. 创建 Phase 3 完成报告（标注技术债）

---

**创建时间**：2026-03-06
**更新时间**：2026-03-06
**预计完成**：待 Phase 4 解决技术债后
