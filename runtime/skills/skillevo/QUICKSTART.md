# 鲁班.Skill 快速上手

5 分钟掌握鲁班的核心用法。

---

## 触发鲁班

说出以下任一词即可：

```
小鲁班 / luban / 优化skill / skill评分 / skill打分 / skill review
```

---

## 常用命令

### 评分（只读，不改文件）

```
"给 prompt-optimizer 评个分"
"评估所有 skills 质量"
"看看 knowledge-engineering 怎么样"
```

输出：十维度评分卡 + 诊断摘要。

### 优化（评分 + 自动改进）

```
"帮我优化 make-to-markdown"
"优化所有 skills"
"完整优化 tiangong-skill"    # 强制 Full 模式
```

流程：模块检测 → 基线评估 → 编辑优化 → 重评 → 保留改进 / 回滚退步。

### 专项检查

| 指令 | 触发模块 | 说明 |
|------|----------|------|
| "检查技能健康" | SkillOps | 引用断裂 / YAML 合法性 / 规则冲突扫描 |
| "这个技能有问题" | EvoSkill | 失败驱动修补，分析缺口并生成补丁 |
| "更新技能知识" | CASCADE | 检查外部引用是否过时（> 90 天） |
| "精简技能" | Skill Distill | 检测死引用、冗余段落，生成精简方案 |
| "规则硬化" | HASP | 软规则升级为 Must 或 PF（可执行程序函数） |
| "安全审计" | Sentinel | 恶意指令 / 硬编码凭据 / Prompt 注入扫描 |
| "看看优化历史" | — | 读取 optimization-registry.tsv |

---

## Quick vs Full 模式

鲁班默认使用 **Quick 模式**（轻量快速），满足以下条件自动升级 **Full 模式**：

| 条件 | 说明 |
|------|------|
| 用户明确要求"完整/深度/全面" | 强制 Full |
| 基线分 < 70 | 需要完整优化 |
| 有历史 revert 记录 | 曾退化，需更谨慎 |
| delta > 5 且连续 2 轮保持 | 有金矿，值得深度挖掘 |

| | Quick | Full |
|---|---|---|
| 评分 | 结构评分 + dry_run 推演 | 全维度 + full_test + 多评委 |
| 优化 | self-refine，`.bak` 回退 | git 分支 + 独立 judge |
| MAX_ROUNDS | 3 | 5 |
| 安全审查 | dim10 默认满分 | P0/P1/P2 全量门控 |

---

## 看懂评分卡

鲁班用 **10 个维度、总分 100** 评估技能质量：

| 维度 | 权重 | 含义 |
|------|:---:|------|
| dim1 Frontmatter 质量 | 7 | name / description / 触发词是否规范 |
| dim2 工作流清晰度 | 12 | 步骤是否有序号、有明确输入/输出 |
| dim3 失败模式编码 | 12 | 是否有 fallback 路径和错误恢复 |
| dim4 检查点设计 | 6 | 是否有关键 STOP / CHECKPOINT 标记 |
| dim5 可执行具体性 | 17 | 参数/格式/示例是否具体，软化词是否过多 |
| dim6 资源整合度 | 4 | references/assets 引用是否正确 |
| dim7 整体架构 | 12 | 标题层级 + 语义质量（冗余/AI 腔） |
| dim8 实测表现 | 20 | 带 skill vs 不带 skill 效果对比 |
| dim9 反例与黑名单 | 6 | 是否标注"不要/禁止/反例"并有替代做法 |
| dim10 安全与审查门控 | 4 | 恶意指令/凭据/注入/外泄/越权检测 |

---

## 关键约束

优化过程中鲁班严格遵守：

1. **不改变技能核心功能**——只优化"怎么写"，不改"做什么"
2. **不引入新依赖**——不添加原本没有的 scripts 或 references
3. **每轮只改一个维度**——避免无法归因
4. **文件大小 ≤ 原 150%**——防止膨胀
5. **棘轮机制**——改进保留，退步自动回滚
6. **评分独立**——效果维度用子 agent 评分，不「自己改自己评」

---

## 端到端实战示例

以"给 prompt-optimizer 评个分"为例，完整展示从输入到报告的流程。

### 用户输入
```
给 prompt-optimizer 评个分
```

### 鲁班做了什么

1. **Phase 0 初始化**：识别目标 skill `prompt-optimizer`，检查 `results.tsv` 历史记录
2. **Phase 0.3 模块检测**：六模块静态扫描（SkillOps/HASP/Sentinel 等），产出诊断子分
3. **Phase 0.5 测试设计**：生成 2-3 个测试 prompt
4. **Phase 1 基线评估**：十维评分 + 效果验证，输出完整评分卡

### 用户看到的输出

```
📊 prompt-optimizer 评分卡

总分: 72.3 / 100

| 维度 | 得分 | 权重 | 加权 | 状态 |
|:---|:---:|:---:|:---:|:---|
| dim1 Frontmatter质量 | 10.0 | 7 | 7.0 | ✅ |
| dim2 工作流清晰度 | 7.5 | 12 | 9.0 | ⚠️ |
| dim3 失败模式编码 | 4.0 | 12 | 4.8 | 🔴 |
| dim4 检查点设计 | 10.0 | 6 | 6.0 | ✅ |
| dim5 可执行具体性 | 8.0 | 17 | 13.6 | ⚠️ |
| dim6 资源整合度 | 5.0 | 4 | 2.0 | ⚠️ |
| dim7 整体架构 | 8.0 | 12 | 9.6 | ✅ |
| dim8 实测表现 | 6.0 | 20 | 12.0 | ⚠️ |
| dim9 反例与黑名单 | 5.0 | 6 | 3.0 | ⚠️ |
| dim10 安全与审查门控 | 6.0 | 4 | 2.4 | ⚠️ |

模块诊断摘要:
- HASP: 软化词 5 处（"建议"×3、"可考虑"×2）→ dim5 子分 1/3
- SkillOps: 引用路径全部可达 → dim6 子分 3/3
- Sentinel: 无安全告警 → dim10 子分 2/2

主要短板:
🔴 dim3 (4.0): 缺少失败分支，只写了正向流程
⚠️ dim5 (8.0): 软化词过多，部分步骤缺具体参数
```

---

### 优化前后对比示例

#### 示例 1：dim5 软化词修正

**优化前**（dim5 = 6.0）：
```markdown
建议在生成报告时控制篇幅在 2000 字以内。
可根据情况选择表格或列表展示数据。
```

**优化后**（dim5 = 8.5）：
```markdown
必须控制报告篇幅 ≤ 2000 字。超出时截断并标注省略。
数据 ≥ 3 列 → 用表格；数据 ≤ 2 列 → 用列表。
```

变化：2 处"建议/可"改为"必须"并补充具体阈值，软化词从 5 处减至 1 处。

#### 示例 2：dim3 补 fallback 路径

**优化前**（dim3 = 4.0）：
```markdown
Step 2: 读取目标文件内容
Step 3: 分析内容并生成摘要
```

**优化后**（dim3 = 8.0，dim2 跟涨 +1.5）：
```markdown
Step 2: 读取目标文件内容
  → 如果文件不存在: 提示用户检查路径，等待重新输入
  → 如果文件无法解析: 尝试用 legac-doc-parser 兜底
  → 如果以上均失败: 输出原始文件路径供用户手动查看
Step 3: 分析内容并生成摘要
```

变化：每个步骤补了三段式 fallback（触发条件 / 一线修复 / 兜底），dim2 因流程更清晰而跟涨。

---

## 自检清单

运行以下命令确认鲁班各脚本可用：

```bash
# 安全审计
python scripts/security_audit.py --help

# 技能健康巡检
python scripts/skillops_scanner.py --help

# 软化词检测与规则硬化
python scripts/hasp_hardener.py --help

# 外部引用过时检测
python scripts/cascade_updater.py --help

# 引用矩阵分析（精简检测）
python scripts/distill_analyzer.py --help

# 回归测试生成
python scripts/muse_generator.py --help

# 失败补丁建议
python scripts/evo_skill_patcher.py --help
```

全部脚本输出 `--help` 信息（而非报错）即表示可用。如有 `ModuleNotFoundError`，执行 `pip install -r requirements.txt`。

---

## 下一步

- 完整 Rubric 细则 → [REFERENCE.md](./REFERENCE.md)
- 设计方法论 → [references/SA-DM.md](./references/SA-DM.md)
- 技能主文件 → [SKILL.md](./SKILL.md)
