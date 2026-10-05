# 鲁班.Skill 技术参考

Rubric 评分体系、优化流程（Phase 0-3）、六模块、数据结构、双模策略。

---

## 目录

1. [十维 Rubric 评分体系](#1-十维-rubric-评分体系)
2. [六模块缺陷检测](#2-六模块缺陷检测)
3. [双模策略](#3-双模策略)
4. [优化流程（Phase 0-3）](#4-优化流程phase-0-3)
5. [多评委与多角色审查](#5-多评委与多角色审查)
6. [关键数据结构](#6-关键数据结构)
7. [优化策略库](#7-优化策略库)
8. [异常与边界条件](#8-异常与边界条件)
9. [12 条设计原则](#9-12-条设计原则)

---

## 1. 十维 Rubric 评分体系

总分 100。确定性维度覆盖 25%，LLM 承担 71 分，dim10 公式计算 4 分。

### 1.1 维度总览

| 维度 | 权重 | 类型 | 评分方式 | Quick | Full |
|:---|:---:|:---|:---|:---|:---|
| dim1 Frontmatter 质量 | 7 | 确定性 | name 规范、description 含做什么+何时用+触发词、≤1024 字符、禁结尾空话。全过=10，缺一=0 | agent 检查 | 同 Quick |
| dim2 工作流清晰度 | 12 | LLM | 步骤明确可执行、有序号、每步有明确输入/输出 | LLM judge | 多评委中位数 |
| dim3 失败模式编码 | 12 | LLM | 显式编码失败模式（"X 失败→Y"）；有 fallback 路径和错误恢复 | LLM judge | 多评委中位数 |
| dim4 检查点设计 | 6 | 确定性 | 正则 `/CHECKPOINT\|STOP\|🔴\|⛔/`：≥1 处 STOP=10，仅 CHECKPOINT=5，无=0 | agent 扫描 | 同 Quick |
| dim5 可执行具体性 | 17 | LLM | 有具体参数/格式/示例；前置软化词扫描。HASP 子分 3/17 | LLM judge + 软化词扫描 | 多评委中位数 + HASP 3 |
| dim6 资源整合度 | 4 | LLM | references/assets 引用正确、路径可达。SkillOps 子分 3/4 | LLM judge | SkillOps 3 + LLM 1 |
| dim7a 结构合规 | 6/12 | 确定性 | 标题层级连续不跳跃 + 含 ≥3/4 必含章节→6 分，每缺一项 −2 | agent 检查 | 同 Quick |
| dim7b 语义质量 | 6/12 | LLM | 冗余/AI 腔/重复描述→一处 −1。Distill 子分 2/6 | LLM judge | Distill 2 + LLM 4 |
| dim8 实测表现 | 20 | LLM | 8a 意图完成度(8) + 8b 净提升(7) + 8c 副作用(5) | dry_run 推演 | full_test（子 agent） |
| dim9 反例与黑名单 | 6 | 混合 | ①关键词扫描 → ②LLM 评估质量（独立章节/含反模式+替代做法/覆盖核心风险） | 同 Full | 同 Quick |
| dim10 安全与审查门控 | 4 | 公式 | Sentinel 2 + P0/P1 审查 2。Quick 默认满分 | 默认 100 | Sentinel + 审查合并 |

### 1.2 评分公式

```
加权原始分 = (dim1×7 + dim2×12 + dim3×12 + dim4×6 + dim5×17 + dim6×4 + dim7a×6 + dim7b×6 + dim8×20 + dim9×6 + dim10×4) / 10
```

### 1.3 模块子分合并规则

| 维度 | 权重 | Module 子分 | Rubric 子分 | 合并公式 |
|:---|:---:|:---|:---|:---|
| dim5 | 17 | HASP 3 | LLM 14 | `(HASP×3 + Rubric×14) / 17 × 10` |
| dim6 | 4 | SkillOps 3 | LLM 1 | `(SkillOps×3 + Rubric×1) / 4 × 10` |
| dim7b | 6 | Distill 2 | LLM 4 | `(Distill×2 + Rubric×4) / 6 × 10` |
| dim10 | 4 | Sentinel 2 | 审查 2 | `(Sentinel×2 + 审查×2) / 4 × 10` |

### 1.4 dry_run 降权规则

| 维度 | 处理 |
|:---|:---|
| dim2/3/5/6/7b/9 | 标注 `[confidence: degraded]`，不降分 |
| dim8a 意图完成度 | 原始分 ×0.5 |
| dim8b 净提升幅度 | 原始分 ×0.3 |
| dim8c 副作用 | 正则扫描可用，不降权 |
| dim10 | 默认 100 |

---

## 2. 六模块缺陷检测

Phase 0.3 执行，产出子分到 `diagnostics.tsv`，模块不独立评分。

### 2.1 模块清单

| 模块 | 检测内容 | 关联维度 | 子分范围 |
|:---|:---|:---|:---:|
| **SkillOps** | 引用路径可达性、YAML/Frontmatter 合法性 | dim6 | 0-3 |
| **EvoSkill** | 历史振荡检测（同维度 2+ 轮反复涨跌） | dim3 | 不产子分（仅标注 `[oscillation]`） |
| **HASP** | 软化词计数（建议/可考虑/根据情况等） | dim5 | 0-3 |
| **CASCADE** | 外部引用过期 > 180 天 | dim6 | 标注，子分由 SkillOps 反映 |
| **Distill** | 完全未被引用的 references、F_approx ≥0.7 文件 | dim7b | 0-2 |
| **Sentinel** | 恶意指令/硬编码凭据/Prompt 注入/数据外泄/权限越权 | dim10 | 0-2（每类独立） |

### 2.2 Sentinel 五类检测

每类独立计分（0 命中=2 分，≥2 处=0 分，1 处=1 分），5 类取均值。

| 类别 | 匹配模式 |
|:---|:---|
| 恶意指令 | `exec(` / `system(` / `subprocess` / `rm -rf` / `format` / `del /f` / `reg delete` |
| 硬编码凭据 | `api_key=` / `password=` / `token=` / `secret=` / 私钥 PEM |
| Prompt 注入 | `DAN` / `jailbreak` / `simulate` / `system override` / `ignore.*instructions` |
| 数据外泄 | `smtp` / `upload.*external` / `scp` / `ftp upload` |
| 权限越权 | `chmod` / `chown` / `sudo` / `su -` / `icacls` |

### 2.3 HASP 软化词列表

检测词：`建议` / `可考虑` / `根据情况` / `灵活把握` / `视情况而定` / `可能` / `大概`

计分：0 处=3 分，≥5 处=0 分，1-4 处线性映射。

### 2.4 Distill F_approx 公式

```
F_approx = 1 - (模块被规则引用的次数 / 模块总字符数归一化)
归一化：模块总字符数 / 所有模块总字符数均值（分母取 max(均值, 100)）
```

- F ≥ 0.7 → 标记「可精简」
- F ≤ 0.3 → 标记「核心资产」

---

## 3. 双模策略

### 3.1 模式选择网关

```
if 用户明确要求"完整/深度/全面/工业/生产" → Full
elif baseline 分 < 70 → Full
elif results.tsv 有 revert 记录 → Full
elif delta > 5 且连续 2 轮保持 → Full
elif 用户说"看看/评一下/扫一眼" → Quick
else → Quick（默认）
```

### 3.2 Quick vs Full

| | Quick | Full |
|:---|:---|:---|
| **触发** | 默认 | 用户明确 / ROI>5 分 / 曾被 revert |
| **评分** | 结构评分 + dry_run 推演 | 全维度 + full_test + 多评委 |
| **优化** | self-refine，`.bak` 回退 | git 分支 + 独立 judge + 仪表盘 |
| **审查** | dim10 默认 100 | P0/P1/P2 全量门控 |
| **基线** | 跳过 | 消费者能力基线测试 |
| **停止** | MAX_ROUNDS=3 | MAX_ROUNDS=5 |
| **Meta** | 不激活 L4 | 激活 L4，输出 meta_learnings.md |

### 3.3 双轨升降

- **Quick→Full**：3 轮内 Δ > 5 分自动升级
- **Full→Quick**：连续 3 个 skill 稳定 delta < 3，后续降级

### 3.4 触顶信号

连续 2 轮 Δ < 2 分 → break。

---

## 4. 优化流程（Phase 0-3）

### 4.1 Phase 0: 初始化

```
1. 确认优化范围（全部 / 指定列表）
2. git checkout -b auto-optimize/YYYYMMDD-HHMM（Quick 跳过）
3. 检查/创建 results.tsv（12 列表头）
4. 读取历史评分
5. ROI 前置评估：基线分 ≥85 且最低维度分 ≥7 → 跳过
6. 读取 revert 历史，标记绕行维度
7. 检查/创建 diagnostics.tsv
```

### 4.2 Phase 0.3: 六模块缺陷检测

六模块按优先级顺序执行：SkillOps → EvoSkill → HASP → CASCADE → Distill → Sentinel，追加 diagnostics.tsv。🔴 CHECKPOINT：展示子分摘要。

### 4.3 Runtime 中立性 Gate

扫描 SKILL.md 全文：

| 检测项 | 判定 | 动作 |
|:---|:---|:---|
| 单 runtime 措辞 | ❌ 不通过 | 强制 Phase 2 P0 修复 |
| 安装路径写死单一工具链 | ❌ 不通过 | 同上 |
| 单一 badge/标识语 | ❌ 不通过 | 同上 |
| skill name 含单 runtime 标识 | ✅ 豁免 | — |

### 4.4 Phase 0.5: 测试 Prompt 设计

为每个 skill 设计 2-3 个测试 prompt（典型 + 歧义），保存到 `{skill目录}/test-prompts.json`。🔴 CHECKPOINT · 🛑 STOP。

### 4.5 Phase 1: 基线评估

```
1. 按 Rubric 逐维评分，得 Rubric 子分
2. 读 diagnostics.tsv，取 Module 子分
3. 同维合并（dim5/6/7b/10）
4. spawn 子 agent 跑 test-prompts 测效果
5. 加权总分 → results.tsv
```

Full 模式额外执行消费者能力基线测试（baseline-skill.md 测目标模型裸能力，< 60 分阻断）。

### 4.6 Phase 2: 优化循环

```
for each skill（按基线分升序）:
  round = 0
  while round < MAX_ROUNDS:
    round += 1
    Step 0: 重跑六模块检测 → 更新 diagnostics.tsv
    Step 1: 诊断 → 找最弱维度
    Step 2: 提方案（1 个具体改进，对照反例黑名单+rejected_edits+luban-profile）
    Step 3: 编辑前备份（git commit 或 .bak）
    Step 4: 执行改进（字符变化 ≤10%）→ 自检 dim1/4/7a/9
    Step 5: 重新评分（spawn 独立子 agent）
    Step 6: 决策
      if 新分 > 旧分: commit + 触顶检测
      else: 回滚 + 记录 rejected_edits.md
```

🔴 CHECKPOINT：每个 skill 优化完展示摘要，等用户确认。

### 4.7 Phase 2.5: 探索性重写

触发：连续 2 个 skill round 1 break 或单 skill 连续 2 轮 round 1 break。流程：git stash → 重写 → 评估 → 优于 stash 版则采用。🛑 STOP：必须用户同意。

### 4.8 Phase 3: 汇总报告

| 项目 | 内容 |
|:---|:---|
| 优化 skill 数 | N 个，保留 M 个 |
| 分数变化 | 表格（skill 名 / 旧分 / 新分 / Δ / 主要改进维度） |
| 主要改进摘要 | 按维度聚类 |
| 健康度仪表盘 | dry_run 比例、revert 率、oscillation 告警 |

**Epoch Meta-Review**（Full 模式）：

1. 汇总优化记录
2. 提炼可迁移规律 → `meta_learnings.md`
3. 识别 oscillation 模式 → `luban-profile.json`

### 4.9 备份轮转

- 每个 skill 保留最近 5 轮备份
- baseline 和首轮备份永久保留
- 超出 5 轮移至 `luban-backups-archive/{skill_name}/`

---

## 5. 多评委与多角色审查

### 5.1 同质多评委

Full 模式 2 个独立 file-agent 评委（dispatch_task），取中位数。

### 5.2 异质评委（按需触发）

| 触发条件 | 异质评委 | 复核焦点 |
|:---|:---|:---|
| dim1≥9 且 dim8a≤5 | search-agent | dim1/dim3 真实性抽查 |
| dim8c=5 且 dim9≤3 | computer-agent | 副作用复核 |
| dim4≥9 但从未触发 revert | computer-agent | dim4/dim10 架构抽查 |

### 5.3 多角色并行审查（P0/P1/P2）

dim8c < 5 或 dim10 原始分 < 60 时触发。

**审查严重度**：

| 级别 | 定义 | 审查影响 | 门控 |
|:---|:---|:---|:---|
| P0 | 影响正确性或安全性 | −30/项 | 任一未闭合→阻断，dim10 上限锁死 ≤40 |
| P1 | 影响可靠性或可维护性 | −5/项 | ≥3 个未闭合→阻断 |
| P2 | 影响一致性或可读性 | 不扣分 | 不设门控 |

**审查流程（阶段 0-4）**：

1. 阶段 0 — 前置：识别文档类型 → 选派角色 → 公开分级声明
2. 阶段 1 — 独立审查：角色间绝对隔离，输出结构化问题清单
3. 阶段 2 — 问题归一化：同类合并、侧面互补、独立保留
4. 阶段 3 — 编辑修复：按 P0→P1→P2 逐项修复，禁止顺手改
5. 阶段 4 — 复审闭环：P0 原角色逐条确认，P1 自检+30% 抽查，P2 自检

### 5.4 子 Agent 不可用降级

| 受影响功能 | 降级行为 | 标记 |
|:---|:---|:---|
| 同质多评委 | 主 agent 单次 LLM 评分 | `judge_count=1`, `eval_mode=fallback` |
| 异质评委 | 跳过 | — |
| 多角色审查 | 跳过，dim10 默认 100 | `eval_mode=fallback` |
| dim8 效果维度 | 降为 dry_run 推演 | `eval_mode=fallback_dry` |

---

## 6. 关键数据结构

### 6.1 results.tsv

位置：`{skill目录}/results.tsv`，12 列 TSV。

```
timestamp	commit	skill	round	old_score	new_score	status	dim_changed	delta	note	eval_mode	judge_count
```

- `status`: `baseline` / `keep` / `revert` / `error`
- `eval_mode`: `full_test` / `dry_run` / `fallback`

### 6.2 diagnostics.tsv

位置：`{skill目录}/diagnostics.tsv`，Phase 0.3 产出，每次运行清空重建。

```
模块	维度	子分	文件	行号	详情
```

### 6.3 optimization-registry.tsv

位置：`luban-workspace/optimization-registry.tsv`，鲁班全局登记表。

```
skill_name	timestamp	score_before	score_after	rounds	eval_mode
```

### 6.4 rejected_edits.md

位置：`{skill目录}/rejected_edits.md`，被回滚的编辑方案。

```markdown
## REJ-{序号} | {时间戳} | {skill名}
- **目标维度**: dim5
- **改动段落**: L120-L135
- **方案摘要**: ...
- **被拒原因**: ...
- **绕行建议**: ...
```

### 6.5 meta_learnings.md

位置：`luban-workspace/meta_learnings.md`，跨 skill 可迁移规律。

```markdown
## ML-{序号} | {时间戳}
- **规律**: ...
- **来源 skill**: ...
- **置信度**: 高/中/低
- **可复用场景**: ...
```

### 6.6 luban-profile.json

位置：`luban-workspace/luban-profile.json`，oscillation guard。

```json
{
  "oscillation_guard": [
    {"dimension": "dim5", "skills": ["skill-a", "skill-b"], "pattern": "...", "recommendation": "..."}
  ]
}
```

---

## 7. 优化策略库

按优先级排序，每轮只做最高优先级的一个。

### 7.1 P0: 适配性与效果（gate 项，必须先修）

| 类型 | 识别特征 | 策略 | 维度 |
|:---|:---|:---|:---|
| Runtime 绑定 | 单 runtime 措辞、安装路径写死 | 替换为 runtime-neutral 措辞 | dim6/dim8 |
| 效果倒退 | 带 skill 比不带还差 | 精简指令 | dim8 |
| 输出偏离 | 测试输出不符合预期 | 补充明确输出模板 | dim8 |
| 副作用触发 | dim8c 命中 | 逐项检查副作用来源 | dim8/dim10 |
| Sentinel 告警 | 安全审计命中 | 移除恶意指令/凭据替换/增加 guards | dim10 |

### 7.2 P1: 结构性问题

| 类型 | 策略 | 维度 |
|:---|:---|:---|
| Frontmatter 缺触发词 | 补充中英文触发词 | dim1 |
| 无 Phase/Step 结构 | 重组为线性流程 | dim2 |
| 无检查点 | 插入 🔴 CHECKPOINT / 🛑 STOP | dim4 |
| 标题跳跃 | 补中间层级，合并重复章节 | dim7 |
| 无错误处理 | 补三段式 fallback | dim3 |

### 7.3 P2: 具体性问题

| 类型 | 策略 | 维度 |
|:---|:---|:---|
| 步骤模糊 | 改为具体操作+参数 | dim5 |
| 缺输入/输出规格 | 补充格式（JSON Schema/示例） | dim5 |
| 缺异常处理 | 补 if-then 兜底路径 | dim3 |
| 软化词过多 | 改为"必须"，补具体数值 | dim5 |
| 资源引用断裂 | 删除死链接或补建文件 | dim6 |

### 7.4 P3: 可读性问题

| 类型 | 策略 | 维度 |
|:---|:---|:---|
| 段落过长 | 拆分或改用表格 | dim7 |
| 重复描述 | 合并去重 | dim7 |
| 缺反例标注 | 加 ≥3 处反例标注 | dim9 |
| 缺速查入口 | 添加 TL;DR 或决策树 | dim5/dim7 |

### 7.5 高杠杆操作（HL）

- **HL-1**：加 🔴 CHECKPOINT / 🛑 STOP，4 行撬动 dim4 +3
- **HL-2**：三段式 fallback，一石三鸟（dim3→dim2 跟涨 + dim4 补检查点）
- **HL-3**：触顶自动 break，+0.15 是停手信号

---

## 8. 异常与边界条件

| 场景 | 触发条件 | 处理动作 |
|:---|:---|:---|
| 不在 git 仓库 | `git rev-parse` 失败 | 询问用户：`git init` 或文件备份 |
| results.tsv 缺失 | 文件不存在 | 新建并写表头 |
| results.tsv 损坏 | 列数不匹配 | 备份后重建 |
| 分支已存在 | `git checkout -b` 失败 | 末尾加 `-2`/`-3`；3 次后询问 |
| git revert 失败 | 冲突/工作树脏 | 先 stash；仍失败则从 commit 读 SKILL.md 手动恢复 |
| MAX_ROUNDS 触顶 | 已达上限仍有短板 | 展示最弱维度，问用户选择 |
| 优化后超 150% 体积 | 新文件 > 原 ×1.5 | 拒绝提交，回精简后重评 |
| test-prompts.json 已存在 | 文件已在 skill 目录 | 复用/重写/追加，三选一 |
| SKILL.md 找不到 | 目录存在但无 SKILL.md | 终止，results.tsv 记 `status=error` |
| 消费者基线失败 | 目标模型裸能力不足 | 输出能力不足报告，阻断 |
| 子 Agent 不可用 | dispatch_task 返回错误 | 触发降级模式 |
| 分数精度漂移 | 总分差 < 0.05 | 改进需严格 > 旧分（不靠四舍五入） |

原则：异常先告知用户再按规则处理；绝不静默跳过。

---

## 9. 12 条设计原则

1. **单一可编辑资产**：每次只改一个 SKILL.md
2. **双重评估**：结构评分 + 效果验证
3. **棘轮机制**：只保留改进，自动回滚退步
4. **独立评分**：用子 agent 消除偏差
5. **人在回路**：每个 skill 优化完暂停确认
6. **文本学习率预算**：每次编辑字符变化 ≤10%
7. **拒绝编辑缓冲区**：回滚方案留作负反馈
8. **Epoch Meta-Review**：跨 skill 汇总，沉淀可迁移经验
9. **场景自适应双模**：Quick / Full 自动选择
10. **ROI 前置评估**：基线分 ≥85 且最低维度分 ≥7 跳过
11. **消费者能力基线**：Full 模式先测目标模型裸能力
12. **全链路审计**：Full 模式所有操作记录 git commit 可追溯

---

## 架构红线

| # | 红线 | 后果 |
|---|------|------|
| 1 | 禁止 self-edit-self-evaluate | dim8 降权 ×0.5 |
| 2 | 禁止跨维度打包修改 | 整轮回滚 |
| 3 | 禁止 dry_run 冒充 full_test | results.tsv 标记 invalid |
| 4 | 禁止 bypass gate | 中断流程 |

### 反例黑名单

| # | 反模式 | 替代做法 |
|---|--------|----------|
| 1 | 同 context 自评自改 | spawn 独立子 agent 评分 |
| 2 | `git reset --hard` 当回滚 | 用 `git checkout` 保留追溯链 |
| 3 | 为凑分增冗余 | 触顶信号 → break |
| 4 | 跳过 test-prompts | Phase 0.5 强制设计 |
| 5 | 轮内改多个维度 | 每轮 1 个维度 |
| 6 | dry_run 比例 > 30% | 强制至少 1 个 full_test |
| 7 | 静默跳过异常 | 异常表 fallback 必须先告知 |
| 8 | 忽视维度相关性 | 看相关簇短板再决定 |
