# QUICKSTART — 天工.skill 快速上手

## 1. 30 秒体验：创建你的第一个专家

**用户输入**：

> "帮我创建一个税务顾问智能体"

**Agent 自动完成 J1→J5 全流程**：

| 步骤 | 动作 | 产出 |
|------|------|------|
| **J1 需求萃取** | 提取岗位名称、部门、核心职责 | 需求卡：税务顾问 / 财务部 / 筹划+合规+风险 |
| **J2 模板填充** | 套用 `examples/job-template.md`，逐字段填充身份5字段、DO/DON'T、KPI | 完整模板骨架 |
| **J3 技法注入** | 升级关键规则（合规/流程/质量/沟通四组）、沟通风格标签化 | 充实后的高质量内容 |
| **J4 质量内控** | 5维评估 + 2个压力场景验证 + `verify-skill.py` 结构校验 | 通过/回退标记 |
| **J5 交付** | 输出完整 SKILL.md 到 `output/` 目录 | 最终文件 + 产出物声明 |

最终产出包含：身份与记忆、核心使命+反使命、DO/DON'T 规则（四组分类）、结构化交付物模板、工作流、可量化 KPI、沟通风格。完整端到端示例见 [`examples/tax-advisor-example.md`](examples/tax-advisor-example.md)。

---

## 2. 选择正确的范式

### 一句话诊断

> **这个智能体在模仿一个人，还是在承担一个岗位？**

### 范式选择流程图

```
你想要的智能体…
  ├─ 是某个真实人物的「数字分身」？
  │   └─ 是 → 人格蒸馏（蒸馏 0→5 全流程）
  ├─ 是某个岗位上的「靠谱专家」？
  │   └─ 是 → 岗位型流水线（J1→J5）
  └─ 两者都要？（既要岗位能力，又要人物风格）
      └─ 范式融合：先岗位型 J1→J5，再注入人物蒸馏的思维特征
```

### 快速诊断表

| 问题 | 回答"是"偏向 | 回答"否"偏向 |
|------|------------|------------|
| 有没有一个具体的人名或角色原型？ | 人格蒸馏 | 岗位型 |
| 用户描述的是职责和能力，而不是"像谁"？ | 岗位型 | 人格蒸馏 |
| 需要复刻特定的表达风格和思维路径？ | 人格蒸馏 | 岗位型 |
| 场景有明确的行业规范、合规要求？ | 岗位型 | 人格蒸馏 |
| 一手素材（著作、访谈）是否充足？ | 人格蒸馏可行 | 降级为岗位型 |

---

## 3. 人物蒸馏快速上手

### 最小可用示例

```
用户: "蒸馏 Charlie Munger"
```

Agent 执行流程：

| 阶段 | 动作 | 输出示例 |
|------|------|---------|
| 蒸馏 0 | 澄清：确认蒸馏目标、使用场景、深度要求 | 需求卡：芒格 / 投资决策辅助 / 深度蒸馏 |
| 蒸馏 1 | 并行采集 7 维度素材（著作、访谈、表达DNA…） | 素材清单（区分一手/二手/推断） |
| 🔴 Gate 1 | 素材门槛验证（深度蒸馏需 7 维度全覆盖 + 一手≥30%） | PASS / 回退补素材 |
| 蒸馏 2 | 四重验证提取心智模型 + 决策启发式 + 表达DNA | 3-7 个心智模型，每条含具体引述 |
| 🔴 Gate 2 | 提炼质量验证（≥3 心智模型通过四重验证） | PASS / 回退蒸馏 1 |
| 蒸馏 3 | 结构化构建：YAML frontmatter + 16 必选模块 | 完整 SKILL.md 骨架 |
| 蒸馏 4 | 质量验证 + verify-persona.py + 压力测试 | 退出码 0 = 通过 |
| 🔴 Gate 3 | 交付就绪门禁 | PASS / 回退修复 |
| 蒸馏 5 | 交付 + 下一步引导 | 最终 SKILL.md 写入 output/ |

### 素材不足时的逃生舱

如果一手来源 < 3 篇，会触发蒸馏 0 的负向筛选警告。有三个选项：

1. **换目标**：选择素材更充足的人物
2. **降级模式**：从深度蒸馏降级为标准/精简模式
3. **岗位型兜底**：放弃人物蒸馏，改为岗位型设计

---

## 4. 岗位型快速上手

### 从模板开始

```bash
# 1. 复制岗位型空白模板
copy examples\job-template.md my-expert.md

# 2. 按照 12 模块逐字段填充（见 references/job-details.md）
# 3. 运行验证
python scripts/verify-skill.py --skill my-expert.md

# 4. 退出码 0 = 质量合格，可以交付
```

### 快捷触发命令

快捷命令是自然语言的简写形式，和完整描述效果完全相同：

| 命令 | 等价自然语言（任意措辞均可） | 效果 |
|:---|:---|:---|
| `设计 律师` | 「帮我做一个律师专家」「创建一个法律顾问 Agent」「写一个律师角色的 prompt」 | 直接进入岗位型流水线 J1→J5 |
| `蒸馏 芒格` | 「复刻查理·芒格的思维方式」「做一个像芒格一样思考的助手」「芒格风格的数字分身」 | 直接进入人格蒸馏流程 |
| `优化 税务顾问` | 「帮我看看这个税务顾问有什么问题」「这个 Agent 质量怎么样，怎么改进」「审查一下这个角色」 | 靶向编辑已有角色 |
| `评估 代码审查员` | 「给这个 Agent 打个分」「检查一下规则有没有冲突」「验证这个技能质量」 | 5 维质量评估，不修改内容 |

**不需要记忆这些命令**。直接说「帮我做一个税务顾问」完全等同于 `设计 税务顾问`。详细触发规则见 [references/trigger-guide.md](references/trigger-guide.md)。

---

## 5. 常见问题速查

| 问题 | 答案 |
|------|------|
| 素材不足怎么办？ | 检查蒸馏 0 负向筛选三条件。命中任一 → 换人或降级。详见 [FAQ #1](references/faq.md) |
| 蒸馏到一半发现素材不够要重来？ | 先走外部搜索降级策略（蒸馏 1）。仍然不够 → 缺失 >3 维度暂停，否则标记"外部来源待补充"继续。详见 [FAQ #3](references/faq.md) |
| verify-skill.py 报错了？ | 按报错定位问题模块（YAML / CHECKPOINT / 规则冲突），回退修复。脚本不可用时回退人工矩阵检测。详见 [FAQ #4](references/faq.md) |
| verify-persona.py 报错了？ | 检查 16 项逐条对照，最常见的失败是：心智模型 < 3 个、表达DNA 某维度"未确认"过多、诚实边界 < 5 条。详见 [验证脚本文档](README.md) §验证脚本 |
| 角色回复像通用 AI？ | 三排查：身份 5 字段是否只填了名称没填性格；表达 DNA 是否机械拼接（防 NPC 铁律）；心智模型是否通过了四重验证。详见 [FAQ #5](references/faq.md) |
| DO 规则注入了但效果不明显？ | 检查每条 DO 是否满足"具体动作 + 为什么 + 反例"三要素。缺反例的 DO 是空泛口号。详见 [FAQ #6](references/faq.md) |
| 人物存在公开矛盾怎么办？ | 命中负向筛选条件 3，提示该人物不适合蒸馏。坚持则取最新立场，旧立场标记为"已修正/已演变"。详见 [FAQ #7](references/faq.md) |
| 对话时经常崩人设？ | 跑蒸馏 4 压力测试。崩人设原因通常是价值观排序不清晰或诚实边界未定义。详见 [FAQ #8](references/faq.md) |
| 天工能帮我执行 Agent 吗？ | 不能。天工只设计+验证 Agent 的 prompt，不运行 Agent。运行任务需派发给对应 Sub Agent。详见 [能力边界矩阵](SKILL.md#-能力边界矩阵) |
| 天工能帮我找现成的 Agent 吗？ | 不能。天工是创作工具，不推荐/搜索已有工具。找现成的用 app-agent 或 web_search。 |
| 我的需求不确定该不该用天工？ | 看两件事：① 是否有「创造/改进/评估 Agent」的意图？② 是否有明确角色/岗位身份？两者都满足 → 用天工。详见 [触发方式详解](references/trigger-guide.md)。 |

---

## 6. 下一步阅读指引

| 想了解… | 阅读… |
|---------|------|
| 人格蒸馏的完整方法论细节 | [references/extraction-framework.md](references/extraction-framework.md) |
| 岗位型 12 模块的逐字段填充规则 | [references/job-details.md](references/job-details.md) |
| 如何做规则冲突检测 | [references/quality-design-process.md](references/quality-design-process.md) §规则冲突检测 |
| 五场景回测怎么做 | [references/quality-verification.md](references/quality-verification.md) §回测验证 |
| 完整输出格式要求 | [references/quality-output-spec.md](references/quality-output-spec.md) |
| 触发机制的配置方式 | [references/trigger-guide.md](references/trigger-guide.md) |
| 人格蒸馏产物模板（16 模块） | [examples/persona-template.md](examples/persona-template.md) |
| 岗位型产物模板（12 模块） | [examples/job-template.md](examples/job-template.md) |
| 岗位型端到端完整示例 | [examples/tax-advisor-example.md](examples/tax-advisor-example.md) |

---

## 7. 验证命令速查

```bash
# 岗位型 / 通用结构验证（7 大维度 + 岗位型深度 11 项）
python scripts/verify-skill.py [--skill path/to/SKILL.md] [--json] [--quiet]

# 人格蒸馏产物验证（16 项检查）
python scripts/verify-persona.py <path/to/SKILL.md> [--json]
```

退出码：`0` = PASS 可交付，`1` = FAIL/NEEDS_REVIEW 需修复。
