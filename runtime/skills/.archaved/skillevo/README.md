# 鲁班.Skill

> 天工开物 工匠鲁班 —— 工业级智能体技能优化器
> 任何的技能自进化原则上依然是线性回归动作，高度依赖模型的自身能力
> 请尽量在可使用的模型里使用最高的模型去优化技能，尽量使用Claude GPT-5.4 Pro GLM-5.1这种高性能模型作为基础优化底座模型

鲁班.Skill 是一套技能自进化系统，基于 **EvoSkill / SkillOps / CASCADE / Skill Distill / HASP / MUSE-Autoskill** 六篇论文构建。它为 AI Agent 技能（SKILL.md）提供全生命周期优化：从静态质量评估、自动化缺陷检测、多评委评分，到规则硬化、知识更新、精简瘦身和回归测试。

---

## 核心能力

| 能力 | 说明 | 依赖说明 |
|------|------|----------|
| **十维 Rubric 评分** | 100 分制量化技能质量，确定性维度 + LLM 维度混合评估 | 无脚本依赖，纯 agent 逻辑 |
| **六模块缺陷检测** | SkillOps / EvoSkill / HASP / CASCADE / Distill / Sentinel 静态扫描产出子分 | 依赖 `scripts/skillops_scanner.py` / `hasp_hardener.py` / `cascade_updater.py` / `distill_analyzer.py` / `security_audit.py` |
| **双模优化引擎** | Quick（轻量 Self-Refine）和 Full（多评委 + git 分支 + 仪表盘）自适应 | 无额外脚本依赖；Full 模式需 git 环境 |
| **安全审计** | Sentinel 检测恶意指令、硬编码凭据、Prompt 注入、数据外泄、权限越权 | 依赖 `scripts/security_audit.py` |
| **棘轮回滚** | 自动保留改进，退步回滚，反例黑名单防止重复踩坑 | Quick 模式靠 `.bak` 文件，Full 模式靠 git |
| **跨技能经验沉淀** | Epoch Meta-Review 提炼可迁移优化规律 | 纯 agent 逻辑，无脚本依赖 |

---

## 文档导航

| 文档 | 内容 |
|------|------|
| [README.md](./README.md) | 项目概览（当前文档） |
| [QUICKSTART.md](./QUICKSTART.md) | 5 分钟快速上手 |
| [REFERENCE.md](./REFERENCE.md) | 完整技术参考（Rubric / Phase 流程 / 模块 / 数据结构） |
| [SKILL.md](./SKILL.md) | 技能主文件（Agent 执行指令） |
| [references/SA-DM.md](./references/SA-DM.md) | 设计方法论论文 |

---

## 触发词

说以下任意关键词即可激活鲁班：
"优化skill"、"skill评分"、"自动优化"、"skill质量检查"、"小鲁班"、"luban"、"优化技能"、"帮我改skill"、"skill怎么样"、"提升skill质量"、"skill review"、"skill打分"

---

## 快速使用

```
# 评估一个技能
"给 prompt-optimizer 评个分"

# 优化一个技能
"帮我优化 make-to-markdown"

# 批量优化
"优化所有 skills"

# 安全审计
"安全审计 luban-skill"
```

详细命令见 [QUICKSTART.md](./QUICKSTART.md)。

---

## 架构总览

```
                          ┌──────────────────────────────────────┐
                          │      鲁班.Skill 技能自进化调度器       │
                          └────────────────┬─────────────────────┘
                                           │
              ┌────────────────────────────┼────────────────────────────┐
              │                            │                            │
    ┌─────────┴──────────┐      ┌─────────┴──────────┐      ┌─────────┴────────────┐
    │   模块层（缺陷检测）   │    │ 核心引擎（评分+修复）│      │   事件钩子（旁路）    │
    │ SkillOps / EvoSkill │      │  Phase 0  初始化   │      │  MUSE 回归 / 错误反馈 │
    │ HASP / CASCADE      │───→  │  Phase 0.3 模块检测│      │  HASP 硬化 / 规则忽略│
    │ Distill / Sentinel  │      │  Phase 1  基线评估 │      └──────────────────────┘
    └────────────────────┘      │  Phase 2  优化循环 │
                                │  Phase 3  汇总报告 │
                                └────────────────────┘
```

---

## 学术依据

| 论文 | arXiv | 核心贡献 |
|------|-------|---------|
| EvoSkill | [2603.02766](https://arxiv.org/abs/2603.02766) | 失败驱动的技能缺口发现与修补 |
| SkillOps | [2605.13716](https://arxiv.org/abs/2605.13716) | 技能库运维框架，五维健康诊断 |
| CASCADE | [2512.23880](https://arxiv.org/abs/2512.23880) | 持续学习 + 自我反思驱动进化 |
| Skill Distill | [2604.01608](https://arxiv.org/abs/2604.01608) | 指标自由度 F 驱动的精简决策 |
| HASP | [2605.17734](https://arxiv.org/abs/2605.17734) | 技能升格为可执行程序函数 |
| MUSE-Autoskill | [2605.27366](https://arxiv.org/abs/2605.27366) | 全生命周期管理 + 回归测试 |
| SkillLens | [2605.23899](https://arxiv.org/abs/2605.23899) | 九维 rubric 实证基础 |
| SkillOpt | [2605.23904](https://arxiv.org/abs/2605.23904) | validation-gated edits 形式化框架 |

---

## 文档结构

```
luban-skill/
│
├── README.md              ← 项目概览、核心能力、学术依据（你在看这里）
│
├── QUICKSTART.md          ← 5 分钟快速上手
│   ├── 触发词
│   ├── 常用命令（评分 / 优化 / 专项检查）
│   ├── Quick vs Full 模式
│   ├── 端到端实战示例        ★ 新增
│   ├── 优化前后对比示例      ★ 新增
│   └── 自检清单              ★ 新增
│
├── SKILL.md               ← Agent 执行指令（核心）
│   ├── 设计哲学 + 阅读导航   ★ 新增导航
│   ├── 评估 Rubric（10 维 100 分）
│   ├── 双模策略（Quick / Full）
│   ├── 约束规则（9 条 + 白话解释）★ 新增白话
│   ├── 多评委与多角色评分
│   ├── 优化流程（Phase 0-3）
│   ├── 异常与边界条件（+ 通俗解释）★ 新增通俗解释列
│   ├── 关键数据结构
│   ├── 反例黑名单
│   ├── 模块详解（CASCADE / Distill / HASP / MUSE）
│   ├── 优化策略库
│   ├── 调度器 + 使用方式
│   └── 反模式与FAQ            ★ 新增（集中速查）
│
├── REFERENCE.md            ← 完整技术参考（Rubric 细则 / Phase 流程 / 模块 / 数据结构）
│
├── references/
│   └── SA-DM.md            ← 设计方法论论文
│
└── scripts/                ← 底层工具脚本
    ├── skillops_scanner.py
    ├── evo_skill_patcher.py
    ├── hasp_hardener.py
    ├── cascade_updater.py
    ├── distill_analyzer.py
    ├── muse_generator.py
    └── security_audit.py
```

| 文档 | 阅读顺序 | 适合人群 |
|:---|:---|:---|
| README.md | 第 1 步 | 所有人，了解这是什么 |
| QUICKSTART.md | 第 2 步 | 新用户，5 分钟上手 |
| SKILL.md | 第 3 步 | 深度用户、开发者，了解全部规则 |
| REFERENCE.md | 参考查阅 | 需要查具体评分标准或流程细节时 |
| references/SA-DM.md | 扩展阅读 | 对设计方法论感兴趣的读者 |
