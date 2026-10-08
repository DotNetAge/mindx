---
name: luban-skill
slug: luban-skill-pro
displayName: 鲁班.skill
version: "1.0.6"
summary: "鲁班.Skill（luban Skill）：工业级智能体技能优化器，Quick/Full 双模策略，用于 skill 评分、自动优化、质量检查与 review。"
description: "鲁班.Skill（luban Skill）：工业级智能体技能优化器。当用户提及以下关键词时调用：“优化skill”、“skill评分”、“自动优化”、“auto optimize”、“skill质量检查”、“小鲁班”、“luban”、“优化技能”、“帮我改skill”、“skill怎么样”、“提升skill质量”、“skill review”、“skill打分”。"
author: "智慧半岛"
tags: [skill优化, 提示词优化, 智能体, 工作流编排, 工具调用]
---
# 鲁班.Skill

> 天工开物 工匠鲁班

---

## L1: 核心摘要

鲁班.Skill 是工业级技能优化器，基于 autoresearch + SkillOpt 场景自适应。采用 **Quick（轻量 Self-Refine，3 轮 / .bak 回退）** 和 **Full（完整循环 + 多评委 + git 分支 + 仪表盘，5 轮）** 双模策略。

**何时调用**：用户提及"优化 skill / skill 评分 / 自动优化 / 质量检查 / skill review"等关键词时激活。

**五大设计支柱**：棘轮回滚（只留改进）/ 独立评分（子 agent 消除偏差）/ 人在回路（每 skill 确认）/ 学习率预算（编辑 ≤10%）/ ROI 前置（≥85 且最低 ≥7 跳过）。

---

## L2: 核心流程与约束

### 路径约定

- `luban-workspace` = 本 SKILL.md 所在目录（即本 skill 目录本身是一个 git 仓库）
- 分支命名 `auto-optimize/YYYYMMDD-HHMM`
- 被优化技能的数据文件（`diagnostics.tsv` / `rejected_edits.md` / `test-prompts.json` / `results.tsv`）放在**被优化 skill 自己的目录**下
- 鲁班全局文件（`meta_learnings.md` / `luban-profile.json` / `optimization-registry.tsv`）放在 `luban-workspace/`

### 架构底座：L0-L4 分层治理

| 层级 | 名称 | Quick | Full |
|:---|:---|:---|:---|
| L0 | 确定性执行层 | 启用 | 启用 |
| L1 | 多智能体协作层 | 按需 | 按需 |
| L2 | 技能自适应优化层 | 启用（鼓励探索） | 严格审查后启用 |
| L3 | 价值对齐层 | 软性约束（风格对齐） | 硬性约束（合规对齐） |
| L4 | 元认知审计层 | 不激活 | 激活（跨 skill 经验沉淀） |

**L0 原子操作**（所有编辑必须走）：读（`read_text`）→ 改（`edit_file`）→ 验（Rubric 逐项自检）。

---

### 📋 阅读导航

| 如果你想… | 读哪里 |
|:---|:---|
| 快速开始 | [QUICKSTART.md](./QUICKSTART.md) |
| 完整 Rubric / 模块 / 策略 / 数据结构 | [REFERENCE.md](./REFERENCE.md) |
| 模块详细流程 | [references/modules.md](./references/modules.md) |
| 反模式 / FAQ / 架构红线 | [references/faq.md](./references/faq.md) |

---

### 约束规则（9 条 + 4 条红线）

1. **不改变核心功能**：只优化"怎么写"，不改"做什么"
2. **不引入新依赖**：不添加 skill 原本没有的 scripts 或 references 文件
3. **每轮只改一个维度**：避免多变更无法归因；相关簇（dim2/3/4）改其一时观察另两个
4. **保持文件大小合理**：优化后 SKILL.md ≤ 原文件 150%
5. **尊重花叔风格**：中文为主、简洁为上
6. **可回滚**：用 `git checkout` 而非 `git reset --hard`
7. **评分独立性**：效果维度必须用子 agent 独立评分，禁止同 context 自评
8. **Runtime 中立性**：必须能在任何 skills-compatible runtime 运行（skill name 明确绑定单一 runtime 除外）
9. **编辑同源检测**：编辑与评分 agent 同源 → dim8 所有子维度 ×0.5

**架构红线**：
- 禁止 self-edit-self-evaluate → dim8 降权 ×0.5
- 禁止跨维度打包修改 → 整轮回滚
- 禁止 dim8 全部 dry_run → 标记 invalid
- 禁止 P0 未闭合进入下一 phase → 中断流程

---

### 评估 Rubric 摘要（10 维度，总分 100）

| # | 维度 | 权重 | 类型 | 一句话评分 |
|:---:|:---|:---:|:---|:---|
| dim1 | Frontmatter 质量 | 7 | 确定性 | name 规范 + description 含做什么/触发词 + ≤1024 字符 + 禁空话。全过=10 |
| dim2 | 工作流清晰度 | 12 | LLM | 步骤明确可执行、有序号、有输入/输出 |
| dim3 | 失败模式编码 | 12 | LLM | 显式编码失败分支 + fallback 路径；无失败分支 ≥−3 分 |
| dim4 | 检查点设计 | 6 | 确定性 | ≥1 处 STOP=10，仅 CHECKPOINT=5，无=0 |
| dim5 | 可执行具体性 | 17 | LLM | 具体参数/格式/示例 + 软化词扫描。HASP 模块产出确定性子分 3/17 |
| dim6 | 资源整合度 | 4 | LLM | 引用正确、路径可达。SkillOps 模块产出确定性子分 3/4 |
| dim7a | 结构合规 | 6 | 确定性 | 标题层级连续 + 含 ≥3/4 必含章节 |
| dim7b | 语义质量 | 6 | LLM | 冗余/AI 腔/重复→一处 −1。Distill 模块产出确定性子分 2/6 |
| dim8 | 实测表现 | 20 | LLM | 8a 意图完成度(8) + 8b 净提升(7) + 8c 副作用(5) |
| dim9 | 反例与黑名单 | 6 | 混合 | 两段式：关键词扫描(≥3 处) → LLM 评估质量 |
| dim10 | 安全与审查门控 | 4 | 公式 | Sentinel 2 + P0/P1 审查 2。Quick 默认满分 |

**评分公式**：`总分 = Σ(dim×权重) / 10`。维度总分 = Module 子分 + Rubric 子分（同维占比加权合并）。dim5/6/7b/10 四维有模块介入。

> 完整 Rubric 评分方式、模块子分合并规则、dry_run 降权、十维执行模式 → [REFERENCE.md §1](./REFERENCE.md#1-十维-rubric-评分体系)

---

### 双模策略

**模式选择（前置网关）**：

```
if 用户明确要求"完整/深度/全面/工业/生产" → Full
elif baseline 分 < 70 → Full
elif results.tsv 有 revert 记录 → Full
elif delta > 5 且连续 2 轮保持 → Full
else → Quick（默认）
```

| | Quick | Full |
|:---|:---|:---|
| **触发** | 默认 | 用户明确 / ROI>5 / 曾被 revert |
| **评分** | 结构评分 + dry_run 推演 | 全维度 + full_test + 多评委 |
| **优化** | self-refine，`.bak` 回退 | git 分支 + 独立 judge + 仪表盘 |
| **审查** | dim10 默认 100 | P0/P1/P2 全量门控 |
| **停止** | MAX_ROUNDS=3 | MAX_ROUNDS=5 |
| **Meta** | 不激活 L4 | 激活，输出 meta_learnings.md |

触顶信号：连续 2 轮 Δ < 2 分 → break。双轨反馈：Quick 下 3 轮内 Δ>5→升级 Full；Full 下连续 3 skill 稳定 delta<3→降级 Quick。

> 完整双模对比、审查流程、多评委与多角色、子 Agent 降级 → [REFERENCE.md §3](./REFERENCE.md#3-双模策略) / [REFERENCE.md §5](./REFERENCE.md#5-多评委与多角色审查)

---

### 优化流程（Phase 0–3）

```
Phase 0: 初始化
  确认范围 → git 分支（Quick 跳过）→ 检查/创建 results.tsv + diagnostics.tsv
  → ROI 前置评估（≥85 且最低 ≥7 跳过）→ 读取 revert 历史标记绕行

Phase 0.3: 模块缺陷检测
  六模块按序执行：SkillOps → EvoSkill → HASP → CASCADE → Distill → Sentinel
  产出确定性子分到 diagnostics.tsv。EvoSkill 仅标注 [oscillation]，不产生子分。
  🔴 CHECKPOINT：展示子分摘要 → Runtime 中立性 Gate → Phase 0.5

Phase 0.5: 测试 Prompt 设计
  为每个 skill 设计 2-3 个 prompt（典型 + 歧义场景），保存到 test-prompts.json
  🔴 CHECKPOINT · 🛑 STOP

Phase 1: 基线评估
  逐维评分 → 同维合并 Module 子分 + Rubric 子分 → spawn 子 agent 跑 full_test
  → 加权计算总分 → 记录 results.tsv
  Full 额外：消费者能力基线测试（baseline-skill.md < 60 分阻断）
  🔴 CHECKPOINT

Phase 2: 优化循环（按基线分升序，先优化最弱）
  round = 0; while round < MAX_ROUNDS:
    Step 0: 重新运行六模块
    Step 1: 诊断找最弱维度（EvoSkill [oscillation] 维度跳过）
    Step 2: 提 1 个具体方案（对照反例黑名单 8 条 + rejected_edits + oscillation_guard）
    Step 3: 编辑前备份（git commit 或 .bak）
    Step 4: 执行改进（字符变化 ≤10%）→ 自检 dim1/4/7a/9
    Step 5: spawn 独立子 agent 重评
    Step 6: 新分 > 旧分 → commit + 触顶检测；否则回滚 + 记录 rejected_edits.md
  🔴 CHECKPOINT：每个 skill 展示改动摘要，等用户确认

Phase 2.5: 探索性重写（按需触发）
  连续 2 skill round 1 break 或单 skill 连续 2 轮 round 1 break → git stash → 重写 → 评估
  🛑 STOP：必须征得用户同意

Phase 3: 汇总报告
  全局战绩表 + 分数变化 + 主要改进摘要 + 健康度仪表盘
  Full 模式：Epoch Meta-Review → meta_learnings.md + luban-profile.json oscillation_guard
  备份轮转：保留最近 5 轮，baseline 和首轮永久保留
```

> 完整 Phase 0–3 详细步骤、Sentinel 执行方式、备份轮转 → [REFERENCE.md §4](./REFERENCE.md#4-优化流程phase-0-3)

---

### 反例黑名单（每轮 Phase 2 Step 2 对照）

| # | 反模式 | 替代做法 |
|---|--------|----------|
| 1 | 同 context 自评自改 | spawn 独立子 agent 评分 |
| 2 | `git reset --hard` 当回滚 | 用 `git checkout` 保留追溯链 |
| 3 | 为凑分增冗余 | 触顶信号 → break |
| 4 | 跳过 test-prompts | Phase 0.5 强制设计 2-3 prompts |
| 5 | 轮内改多个维度 | 每轮 1 个维度 |
| 6 | dry_run 比例 > 30% | 强制至少 1 个 full_test |
| 7 | 静默跳过异常 | 异常表 fallback 必须先告知 |
| 8 | 忽视维度相关性单独优化 | 看相关簇短板再决定 |

---

### 关键数据结构速查

| 文件 | 位置 | 核心字段 |
|:---|:---|:---|
| `results.tsv` | `{skill目录}/` | timestamp / commit / skill / round / old_score / new_score / status / dim_changed / delta / note / eval_mode / judge_count |
| `diagnostics.tsv` | `{skill目录}/` | 模块 / 维度 / 子分 / 文件 / 行号 / 详情。Phase 0.3 每次清空重建 |
| `optimization-registry.tsv` | `luban-workspace/` | skill_name / timestamp / score_before / score_after / rounds / eval_mode。只增不删 |
| `rejected_edits.md` | `{skill目录}/` | REJ 记录：目标维度 / 改动段落 / 方案摘要 / 被拒原因 / 绕行建议 |
| `meta_learnings.md` | `luban-workspace/` | ML 记录：规律 / 来源 skill / 置信度 / 可复用场景 |
| `luban-profile.json` | `luban-workspace/` | oscillation_guard 数组 |

> 完整 TSV 示例与数据结构详情 → [REFERENCE.md §6](./REFERENCE.md#6-关键数据结构)

---

### 异常与边界条件速查

| 场景 | 处理动作 |
|:---|:---|
| 不在 git 仓库 | 询问：`git init` 或文件备份 |
| results.tsv 缺失/损坏 | 新建或备份后重建 |
| 分支已存在 | 末尾加 `-2`/`-3`；3 次后询问 |
| git revert 失败 | 先 stash；仍失败则从 commit 手动恢复 SKILL.md |
| MAX_ROUNDS 触顶 | 展示最弱维度，问用户：加 1 轮 / 探索性重写 / 收工 |
| 优化后超 150% 体积 | 拒绝提交，回精简后重评 |
| test-prompts.json 已存在 | 复用 / 重写 / 追加，三选一 |
| SKILL.md 找不到 | 终止，记 `status=error` |
| 消费者基线失败 | 输出能力不足报告，阻断 |
| 子 Agent 不可用 | 触发降级模式（见 REFERENCE.md §5.4） |
| 分数精度漂移 | 总分差 < 0.05 不算提升 |

> 完整异常表含触发条件+通俗解释 → [REFERENCE.md §8](./REFERENCE.md#8-异常与边界条件)

---

### 优化策略库（按优先级 P0→P3，每轮只做最高优先级一个）

**P0: 适配性与效果（gate，必须先修）**
Runtime 绑定（单 runtime 措辞/路径写死）→ 替换为 runtime-neutral；效果倒退 → 精简指令；Sentinel 安全告警 → 移除恶意指令/凭据替换/增加 guards。

**P1: 结构性问题**
Frontmatter 缺触发词 → 补充；无 Phase/Step 结构 → 重组线性流程；无检查点 → 插入 🔴 CHECKPOINT / 🛑 STOP；标题跳跃 → 补层级；无错误处理 → 补三段式 fallback。

**P2: 具体性问题**
步骤模糊 → 加参数/工具名/格式；软化词过多 → 改"建议"为"必须"；资源引用断裂 → 删除或补建。

**P3: 可读性问题**
段落过长 → 拆分/表格；重复描述 → 合并；缺反例标注 → 加 ≥3 处；缺速查入口 → 添加 TL;DR。

**优先级公式**：`弱点深度 = (10 − 当前维度分) × 权重`。相关簇提醒：dim2/3/4 联动。

> 完整策略库含识别特征+优化策略+关联维度 → [REFERENCE.md §7](./REFERENCE.md#7-优化策略库)

---

### HL 操作速查

- **HL-1（dim4）**：加 🔴 CHECKPOINT / 🛑 STOP。4 行改动撬动 +3 分
- **HL-2（dim2/3/4 相关簇）**：修 dim3（三段式 fallback）→ dim2 跟涨 1-2 分，dim4 顺便补检查点
- **HL-3（Phase 2 退出）**：+0.15 是停手信号，触顶自动 break

---

## L3: 详见参考文件

### 资源文件速查

| 路径 | 用途 |
|------|------|
| `scripts/skillops_scanner.py` | SkillOps 路径/YAML/引用链扫描 |
| `scripts/evo_skill_patcher.py` | EvoSkill 失败驱动补丁生成 |
| `scripts/hasp_hardener.py` | HASP 软化词检测→Must/PF 升级 |
| `scripts/cascade_updater.py` | CASCADE 外部引用过时检测 |
| `scripts/distill_analyzer.py` | Distill 引用矩阵+F_approx 计算 |
| `scripts/muse_generator.py` | MUSE 测试用例生成与回归 |
| `scripts/security_audit.py` | Sentinel 安全审计 |
| `references/SA-DM.md` | SkillOps 设计方法论完整论文 |
| `references/baseline-skill.md` | 消费者能力基线测试参考 skill |

### 完整文档索引

| 内容 | 位置 |
|:---|:---|
| 十维 Rubric 评分体系（完整表格+评分公式+合并规则+dry_run 降权+执行模式） | [REFERENCE.md §1](./REFERENCE.md#1-十维-rubric-评分体系) |
| 六模块缺陷检测（模块清单+Sentinel 五类+HASP 软化词+Distill F_approx） | [REFERENCE.md §2](./REFERENCE.md#2-六模块缺陷检测) |
| 双模策略（网关+对比+双轨反馈+触顶信号） | [REFERENCE.md §3](./REFERENCE.md#3-双模策略) |
| 优化流程 Phase 0-3（全部步骤+备份轮转） | [REFERENCE.md §4](./REFERENCE.md#4-优化流程phase-0-3) |
| 多评委与多角色审查（同质/异质/并行审查+降级） | [REFERENCE.md §5](./REFERENCE.md#5-多评委与多角色审查) |
| 关键数据结构（TSV 示例+rejected_edits+meta_learnings+luban-profile） | [REFERENCE.md §6](./REFERENCE.md#6-关键数据结构) |
| 优化策略库（P0-P3 完整表格+HL 操作） | [REFERENCE.md §7](./REFERENCE.md#7-优化策略库) |
| 异常与边界条件（完整表格） | [REFERENCE.md §8](./REFERENCE.md#8-异常与边界条件) |
| 12 条设计原则（完整解释） | [REFERENCE.md §9](./REFERENCE.md#9-12-条设计原则) |
| 架构红线 + 反例黑名单 | [REFERENCE.md](#架构红线运行时检测) |
| 模块详细流程（CASCADE/HASP/Distill/MUSE/调度器） | [references/modules.md](./references/modules.md) |
| 反模式 / FAQ / 架构红线 / 指令映射 / 异常场景 | [references/faq.md](./references/faq.md) |
