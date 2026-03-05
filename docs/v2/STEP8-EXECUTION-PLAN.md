# Step 8 执行计划：删除遗留代码

> 创建日期：2026-03-06
>
> 目标：清理所有旧的 Skill 系统代码

---

## 📋 待删除文件清单

### 核心接口和实现

1. **`internal/core/skillmgr.go`** ❌ 删除
   - 旧的 SkillManager 接口定义
   - 旧的 Skill 结构体

2. **`internal/usecase/skills/skill_mgr.go`** ❌ 删除
   - 旧的 SkillManager 实现

3. **`internal/entity/skill.go`** ⚠️ 标记废弃（暂时保留，添加废弃注释）
   - 旧的 SkillDef 定义
   - 某些地方可能还在使用

### 测试文件

4. **`internal/usecase/skills/skill_mgr_test.go`** ❌ 删除
5. **`internal/usecase/skills/skill_mgr_integration_test.go`** ❌ 删除
6. **`internal/usecase/skills/skill_mgr_precompute_test.go`** ❌ 删除

### 其他相关文件

7. **`internal/usecase/brain/processors/testing.go`** ⚠️ 检查并更新
   - MockSkillManager 定义

---

## 🔍 需要更新的文件

### 1. Pipeline 相关

- `internal/usecase/brain/brain_pipeline.go`
  - 更新 SkillMatchProcessor 的创建方式

### 2. 测试文件

- `internal/usecase/brain/processors/pipeline_e2e_test.go`
- `internal/usecase/brain/processors/pipeline_integration_test.go`
- `internal/usecase/brain/processors/pipeline_test.go`

### 3. Bootstrap 相关

- `internal/infrastructure/bootstrap/assistant.go`
  - 更新 Skill 系统初始化

---

## ✅ 保留的文件

### 1. 新的 Skill 系统

- ✅ `internal/entity/skill_new.go` - 新的 Skill Entity
- ✅ `internal/usecase/skills/parser.go` - SKILL.md 解析器
- ✅ `internal/usecase/skills/vector_index.go` - 向量索引
- ✅ `internal/usecase/skills/hybrid_searcher.go` - 混合检索
- ✅ `internal/usecase/skills/tool_assembler.go` - 工具组装
- ✅ `internal/usecase/skills/keyword_index.go` - 关键词索引（作为混合检索的一部分）

### 2. 处理器

- ✅ `internal/usecase/brain/processors/skill_processor.go` - 重构后的处理器

---

## 🔧 执行步骤

### Step 1: 备份当前代码
```bash
git add .
git commit -m "backup: before removing legacy skill code"
```

### Step 2: 删除核心文件
```bash
rm internal/core/skillmgr.go
rm internal/usecase/skills/skill_mgr.go
rm internal/usecase/skills/skill_mgr_test.go
rm internal/usecase/skills/skill_mgr_integration_test.go
rm internal/usecase/skills/skill_mgr_precompute_test.go
```

### Step 3: 更新引用
- 更新 brain_pipeline.go
- 更新 assistant.go
- 更新测试文件

### Step 4: 运行测试
```bash
go test ./...
```

### Step 5: 验证编译
```bash
go build ./...
```

---

## ⚠️ 风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| 某些地方仍在使用旧代码 | 高 | 先搜索所有引用，逐个更新 |
| 测试失败 | 中 | 逐步删除，每次删除后运行测试 |
| 编译错误 | 中 | 使用 IDE 的重构功能 |
| 功能回归 | 高 | 运行完整的测试套件 |

---

## 📊 预期结果

### 代码减少

- 删除文件：~5 个
- 删除代码行：~2000 行
- 更新文件：~10 个

### 架构改进

- ✅ 无遗留代码
- ✅ 架构清晰
- ✅ 符合规范
- ✅ 易于维护

---

**创建时间**：2026-03-06
**预计完成**：2026-03-06
