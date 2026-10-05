# 模块与调度器参考

## CASCADE 知识更新

### 触发条件

- 定时任务：每季度自动执行
- 用户指令："更新技能知识""补最新""刷新 references"
- 技能 references 中引用外部知识且距上次更新 > 90 天

### 执行流程

```
Step 1: 扫描 references/ 目录
  - 执行 `scripts/cascade_updater.py <skill_dir> --threshold 90` 提取外部引用并判定过时
  - 记录最后更新日期（优先 git log，回退至 mtime）

Step 2: 筛选过时引用
  - 距上次更新 > 90 天 → 标记待更新
  - 优先高频使用的技能

Step 3: 知识检索
  - 论文：搜索 arXiv ID，检查新版本
  - API：抓取最新文档，对比 changelog
  - 标准：搜索新版本

Step 4: 自我反思（内省）
  - 对比新旧知识差异
  - 仅在有实质性变化时生成更新

Step 5: 追加式更新
  - 追加新知识（不删除旧内容，标注版本号）
  - 格式：## [YYYY-MM-DD] 更新：xxx → 新内容

Step 6: 写入诊断记录
  - 确实有更新的引用追加 diagnostics.tsv
  - 格式：CASCADE | dim6 | 子分 | 文件 | 行号 | 详情
```

### 关键设计

- 只追加不删除：旧知识的废弃留给 SkillOps 的 `retire` 动作
- 标注版本：每次更新附带日期和版本号
- 不自动修改规则：仅更新 references，不自动改 SKILL.md 中的规则引用

---

## Distill 精简

### F_approx 计算公式

```
F_approx = 1 - (模块被规则引用的次数 / 模块总字符数归一化)

归一化方式：模块总字符数 / 所有模块总字符数的均值。分母过小时取 max(均值, 100)。
```

- F ≈ 1：模块内容庞大但很少被引用 → 可精简
- F ≈ 0：模块内容紧凑且多处引用 → 保留
- F_approx ≥ 0.7：标记「可精简」；≤ 0.3：标记「核心资产」

### 精简优先级

| 优先级 | 类型 | 处理 |
|--------|------|------|
| P0 | 完全未被引用的 references | 直接建议删除 |
| P1 | 文件大但仅 1-2 处引用 | 提取引用段落到 SKILL.md，删原文件 |
| P2 | 多处重复的示例代码块 | 合并为一个 reference |
| P3 | 历史版本累积的旧内容 | 归档到 archive/ 子目录 |

### 执行流程

```
Step 1: 构建引用矩阵
  - 执行 `scripts/distill_analyzer.py <skill_dir>` 自动扫描引用
  - 计算每个 references 文件的「有效引用密度」

Step 2: 计算 F_approx，分级标记

Step 3: 生成精简方案
  - 展示「删除后文件大小变化」预估
  - 标注「保留的核心内容」
  - 🔴 CHECKPOINT：待用户确认后执行

Step 4: 写入诊断记录
  - 确认执行后，P0 删除 / P1-P3 精简项追加 diagnostics.tsv
  - 格式：Distill | dim7b | 子分 | 文件 | 行号 | 详情
```

---

## HASP 硬化规则

> 论文：[arXiv:2605.17734](https://arxiv.org/abs/2605.17734)
> 核心理念：技能升格为可执行程序函数（PF），含 should_activate + intervene，从"建议"变"硬纠正"。

### 触发条件

- 同一规则在同一场景下连续 2 次以上被忽略
- 用户指令："规则硬化""硬一点""这个规则总被忽略"

### 硬化层级

#### 层级 1：Should → Must（措辞强化）

```
原文：建议在生成 SKILL.md 时控制文件大小在 30KB 以内

硬化后：强制约束：SKILL.md 文件大小不得超过 30KB。
超限时，必须将详细内容拆分到 references/，SKILL.md 仅保留导航链接。
```

#### 层级 2：Should → PF（可执行程序函数）

在 SKILL.md frontmatter 中追加硬规则元数据：

```yaml
hard_rules:
  - id: rule_001
    should_activate: "SKILL.md 文件大小 > 30KB"
    intervene:
      type: "block_and_restructure"
      action: "禁止继续在 SKILL.md 追加内容，将超出部分写入新 reference 文件"
    severity: "critical"
    last_violated: "2026-06-10"
    violation_count: 3
```

### 执行流程

```
Step 1: 执行日志分析
  - 执行 `scripts/hasp_hardener.py <skill_dir> [--results <results.tsv>]` 提取软规则、匹配违规
  - 识别「规则被忽略」的实例

Step 2: 分级处理
  - 工具自动判定 T0（基线）/ T1（措辞强化，违规 2 次）/ T2（PF 硬化，违规 ≥3 次）
  忽略 1 次 → 暂不处理

Step 3: 硬化规则注入
  - 定义 should_activate 条件 + intervene 动作
  - 🔴 CHECKPOINT：待用户确认后执行

Step 4: 写入诊断记录
  - 确认执行后追加 diagnostics.tsv
  - 违规 ≥2 次：subtype=wording_harden；≥3 次：subtype=pf_harden
  - 格式：HASP | dim5 | 子分 | 文件 | 行号 | 详情
```

### 硬化适用性

| 规则类型 | 适合硬化 | 原因 |
|----------|---------|------|
| 输出格式约束 | ✅ | 可精确检测和修正 |
| 文件大小限制 | ✅ | 可精确检测 |
| 必须包含的章节/字段 | ✅ | 结构化检查 |
| 语义风格约束 | ❌ | 难以精确检测 |
| 创造性建议 | ❌ | 无法形式化 |

---

## MUSE 回归测试

> 论文：[arXiv:2605.27366](https://arxiv.org/abs/2605.27366)
> 核心理念：双驱动评估（单元测试 + 运行反馈），自动触发修补和重测，首次实证跨智能体技能迁移。

### 触发条件

- 任何对技能文件（SKILL.md 或 references/）的编辑操作完成后
- 自动触发，无需用户指令

### 测试用例生成维度

| 维度 | 生成方法 | 示例 |
|------|----------|------|
| 触发词识别 | 从 SKILL.md 提取所有触发词，逐一构造输入 | 输入"小鲁班" → 预期触发 |
| 输出格式 | 提取格式约束，构造验收条件 | 输出必须包含 YAML frontmatter |
| 关键规则遵守 | 提取"必须"/"禁止"语句，构造边界测试 | 输入超限请求 → 预期拒绝 |
| 流程完整性 | 按技能 Step 列表逐项模拟 | Step 3 依赖 Step 2 的输出 → 断链测试 |
| references 可达性 | 遍历所有文件路径引用 | 逐条检查文件是否存在 |
| 反例测试 | 构造明确不在范围内的输入 | "帮我写操作系统" → 预期不触发 |

### 执行流程

```
Step 1: 修改前快照
  - 执行 `scripts/muse_generator.py <skill_dir>` 保存 hash 并生成 5-10 条测试用例（6 维度）

Step 2: 执行修改

Step 3: 回归测试 → 逐条检查修改后行为

Step 4: 结果判定
  全部通过 → 「回归测试通过」
  部分失败 → 列出失败项 + 偏差 + 建议回滚
  全部失败 → 强制建议回滚

Step 5: 测试用例沉淀
  - 通过的用例追加到 tests.yaml（如不存在则创建）
```

### tests.yaml 格式

```yaml
skill: {skill_name}
generated_at: {date}
tests:
  - id: trigger_001
    dimension: "触发词识别"
    input: "{test_input}"
    expected: "{expected_behavior}"
    status: pass | fail
    last_run: {date}
    
  - id: format_001
    dimension: "输出格式"
    condition: "{constraint}"
    check: "{check_method}"
    status: pass | fail
    last_run: {date}
```

---

## 调度器触发策略

### 事件驱动 / 按需触发 / 定时驱动

```
事件驱动（立即响应）
  ├── P0: 用户反馈技能错误         → EvoSkill
  ├── P0: 技能编辑完成              → MUSE 回归测试
  ├── P1: 规则连续忽略 3 次         → HASP 层级 2
  └── P2: 规则连续忽略 2 次         → HASP 层级 1

按需触发（用户指令）→ 详见 FAQ 指令映射表

定时驱动（周期扫描）
  ├── 每周：SkillOps 健康巡检
  ├── 每月：Skill Distill 精简检查（仅当冗余评分低时）
  └── 每季度：CASCADE 知识更新检查（仅当有外部引用时）
```

### 并发控制与冲突仲裁

| 场景 | 仲裁规则 |
|------|----------|
| 任何模块 vs 用户正在手动编辑 | 用户优先，模块排队 |
| Phase 2 编辑中 vs 事件模块同时触发 | Phase 2 优先（按需触发等同用户指令），事件模块排队 |
| SkillOps 巡检 vs 用户正在编辑 | 巡检只读执行，仅输出报告 |
| Skill Distill vs HASP 同时触发 | HASP 优先（质量保障），精简让步 |
| MUSE 回归测试运行中 | 锁定技能文件，其他模块等待 |
| 同一技能多个任务排队 | FIFO 顺序执行 |
