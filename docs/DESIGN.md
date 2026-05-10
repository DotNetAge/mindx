# ProjectTools: OPC 智能工作流系统

> **One-Person-Company (OPC) 的 AI 驱动解决方案**
> 
> 这不是传统的项目管理工具，而是一个**让一个人能够运营一家公司**的智能工作流系统。
> 通过 Agent 自主决策、自动执行、智能协作，实现从目标设定到交付报告的全自动化闭环。

---

## 🎯 核心理念：什么是 OPC？

### 传统模式 vs OPC 模式

```
传统公司 (需要多人团队):
┌─────────────────────────────────────┐
│ CEO/产品经理  → 决策做什么          │
│ 项目经理      → 规划怎么做          │
│ 开发团队      → 具体执行             │
│ 测试团队      → 质量保证             │
│ 文档专员      → 知识沉淀             │
│ 行政助理      → 进度追踪             │
└─────────────────────────────────────┘
成本: 高 | 协调复杂 | 响应慢 | 依赖人力

OPC 模式 (你 + AI Agent 团队):
┌─────────────────────────────────────┐
│ 你 (CEO)     → 设定目标和方向       │
│              ↓                      │
│ MasterAgent  → 分解任务、分配资源    │
│              ↓                      │
│ SubAgents    → 自主执行各自专业领域   │
│              ↓                      │
│ Whisper/Cron → 定时触发、全自动运行   │
│              ↓                      │
│ TUI         → 随时查看进度和结果      │
└─────────────────────────────────────┘
成本: 极低 | 零协调 | 即时响应 | 7×24运行
```

### OPC 的三大支柱

1. **🤖 Agent 自治** - 每个 Agent 都是有推理能力的智能体，不是简单的脚本
2. **📊 GraphDB 记忆** - 所有状态、关系、历史都记录在图数据库中，可追溯可分析
3. **🔄 自动化闭环** - 从触发到执行到汇报，全程无需人工干预

---

## 🏗️ 系统架构全景图

```
┌─────────────────────────────────────────────────────────────────┐
│                    OPC 智能工作流系统架构                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌───────────────┐  ┌───────────────────────────────────────┐  │
│  │  触发层        │  │  引擎层 (持续运行)                     │  │
│  │               │  │                                       │  │
│  │  ┌─────────┐  │  │  ┌─────────────────────────────────┐  │  │
│  │  │ Whisper │  │  │  │        Master Agent (MA)        │  │  │
│  │  │  CLI    │──┼──►│                                 │  │  │
│  │  └─────────┘  │  │  │  职责:                           │  │  │
│  │               │  │  │  ① 目标分解 (WBS)                │  │  │
│  │  Cron 定时器  │  │  │  ② 任务分配 (指派SubAgent)        │  │  │
│  │  Shell 脚本  │  │  │  ③ 进度监控                       │  │  │
│  │  其他程序    │  │  │  ④ 结果收集与汇总                 │  │  │
│  │               │  │  │  ⑤ 报告生成                       │  │  │
│  └───────────────┘  │  └──────────┬──────────────────────┘  │  │
│                      │             │                         │  │
│  ┌───────────────┐  │             ▼                         │  │
│  │  交互层        │  │  ┌─────────────────────────────────┐  │  │
│  │               │  │  │         Skill 层 (工作流编排)     │  │  │
│  │  ┌─────────┐  │  │  │                                 │  │  │
│  │  │MindX TUI│  │  │  │  proj-daily-execution           │  │  │
│  │  │  (终端) │──┼──►│  ├─ 查询今日任务                  │  │  │
│  │  └─────────┘  │  │  ├─ 分配给 MA 或 SubAgent         │  │  │
│  │               │  │  ├─ 更新进度到 GraphDB             │  │  │
│  │  API 接口     │  │  └──────────┬──────────────────────┘  │  │
│  │  Web UI      │  │             │                         │  │
│  └───────────────┘  │             ▼                         │  │
│                      │  ┌─────────────────────────────────┐  │  │
│                      │  │        Tool 层 (能力接口)        │  │  │
│                      │  │                                 │  │  │
│                      │  │  ┌───────────────────────────┐  │  │  │
│                      │  │  │    ProjectTools (6个CRUD)   │  │  │  │
│                      │  │  │    图数据读写               │  │  │  │
│                      │  │  └───────────────────────────┘  │  │  │
│                      │  │                                 │  │  │
│                      │  │  ┌───────────────────────────┐  │  │  │
│                      │  │  │    GoReact 内置 Tools       │  │  │  │
│                      │  │  │    task_create / subagent   │  │  │  │
│                      │  │  │    team_* / send_message    │  │  │
│                      │  │  └───────────────────────────┘  │  │  │
│                      │  └──────────┬──────────────────────┘  │  │
│                      │             │                         │  │
│                      │             ▼                         │  │
│                      │  ┌─────────────────────────────────┐  │  │
│                      │  │        GoGraphDB (持久化存储)     │  │  │
│                      │  │                                 │  │  │
│                      │  │  :Task 节点 (目标/任务)          │  │  │
│                      │  │  :Resource 节点 (工具/角色)      │  │  │
│                      │  │  关系: PARENT_OF / DEPENDS_ON    │  │  │
│                      │  │        REQUIRES / ASSIGNED_TO    │  │  │
│                      │  └─────────────────────────────────┘  │  │
│                      │                                     │  │
│                      └─────────────────────────────────────┘  │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │                    输出层                                │  │
│  │                                                         │  │
│  │  runtime/documents/                                      │  │
│  │  ├── daily-reports/YYYY-MM-DD.md    (每日工作报告)       │  │
│  │  ├── weekly-reports/YYYY-Www.md    (周报)               │  │
│  │  ├── monthly-reports/YYYY-MM.md     (月报)               │  │
│  │  └── milestones/{goal-id}.md        (里程碑报告)          │  │
│  │                                                         │  │
│  └─────────────────────────────────────────────────────────┘  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 👥 核心组件详解

### 1️⃣ Whisper CLI (触发器)

**定位**: Fire-and-forget 的轻量级触发器  
**文件**: `cmd/whisper.go`  
**用途**: 向 Master Agent 发送指令后立即退出

```bash
# 基础用法
mindx whisper "检查并完成今天的任务"

# 带元数据
mindx whisper --tag daily --priority high "开始Q2发布准备"

# Cron 集成 (crontab)
0 9 * * * mindx whisper "每日例行工作"
0 9 * * 1 mindx whisper --tag weekly "周报生成"
```

**核心特性**:
- ✅ 发送即退出 (<1秒)
- ✅ 零资源占用
- ✅ 支持 tag/priority/timeout 参数
- ✅ 可选等待首次响应 (--wait)

---

### 2️⃣ Master Agent (大脑)

**定位**: 持续运行的 AI 推理引擎  
**职责**: 三件事

| #   | 职责         | 说明                           |
| --- | ------------ | ------------------------------ |
| ①   | **分解目标** | 将高层目标拆解为可执行的子任务 |
| ②   | **指派执行** | 将任务分配给自身或 SubAgent    |
| ③   | **收集总结** | 汇总所有结果，生成报告         |

**工作模式**: 7×24 待机，收到消息立即响应

---

### 3️⃣ Skills (工作流编排)

Skills 定义了 Master Agent 如何串联 Tools 完成复杂的业务流程。

#### 已实现的 Skills:

| Skill 名称               | 文件位置                                       | 用途                 | 优先级 |
| ------------------------ | ---------------------------------------------- | -------------------- | ------ |
| **proj-initialize**      | `runtime/skills/proj-initialize/SKILL.md`       | ⭐ 公司初始化/CEO就职  | **最高** |
| **proj-daily-execution** | `runtime/skills/proj-daily-execution/SKILL.md` | 每日任务执行主流程   | 高     |
| **proj-decompose-goal**  | `runtime/skills/proj-decompose-goal/SKILL.md`  | WBS目标分解+资源分配 | 高     |
| **proj-generate-report** | `runtime/skills/proj-generate-report/SKILL.md` | 多类型工作报告生成   | 中     |

#### Skill 选择逻辑 (更新):

```
用户输入/MasterAgent 收到消息
       │
       ├── 首次使用 / "开公司"/"初始化"/"成立"/"我要做XX"
       │   └──→ 选择: proj-initialize ⭐ (必须先执行!)
       │
       ├── 包含"今天"/"每日"/"routine"
       │   └──→ 选择: proj-daily-execution
       │
       ├── 包含"创建目标"/"分解"/"规划"
       │   └──→ 选择: proj-decompose-goal
       │
       ├── 包含"报告"/"日报"/"周报"/"总结"
       │   └──→ 选择: proj-generate-report
       │
       └── 其他情况
           └──→ MA 自主判断或组合使用多个Skill
```

⚠️ **重要**: `proj-initialize` 是所有其他 Skill 的**前置条件**！  
在系统首次使用时，必须先完成公司初始化流程。
       ├── 包含"报告"/"日报"/"周报"/"总结"
       │   └──→ 选择: proj-generate-report
       │
       └── 其他情况
           └──→ MA 自主判断或组合使用多个Skill
```

---

### 4️⃣ Tools (能力接口)

#### ProjectTools (图数据操作)

| Tool            | 操作     | Cypher命令                     | 用途                   |
| --------------- | -------- | ------------------------------ | ---------------------- |
| **proj_add**    | 创建节点 | `CREATE (n:$label $props)`     | 创建任务/资源节点      |
| **proj_query**  | 查询列表 | `MATCH (n) WHERE ... RETURN n` | 过滤/搜索任务          |
| **proj_get**    | 获取详情 | `MATCH + OPTIONAL MATCH`       | 获取任务+关联数据      |
| **proj_update** | 更新属性 | `SET n += $fields`             | 更新状态/进度/简报     |
| **proj_relate** | 建立关系 | `CREATE (a)-[:REL]->(b)`       | 建立父子/依赖/分配关系 |
| **proj_delete** | 删除节点 | `DETACH DELETE n`              | 删除任务/清理          |

#### GoReact 内置 Tools (执行能力)

| Tool             | 类型     | 用途                         |
| ---------------- | -------- | ---------------------------- |
| **task_create**  | 同步执行 | MA直接执行的简单任务         |
| **subagent**     | 异步启动 | 启动专业SubAgent处理复杂任务 |
| **team_create**  | 团队创建 | 多Agent协作场景              |
| **send_message** | 消息发送 | Agent间通信                  |
| **wait_team**    | 等待完成 | 收集多Agent结果              |

---

### 5️⃣ SubAgents (专业执行者)

SubAgents 不是被动的执行器，而是**有完整推理能力的自治Agent**。

#### SubAgent 工作模式:

```
收到任务 (来自 MA 的 subagent 调用)
       │
       ▼
  Step 1: 任务分析
  ├─ 理解需求和边界
  ├─ 识别技术难点
  ├─ 确定所需工具
  └─ 预估工作量
       │
       ▼
  Step 2: 创建 Todo 列表 (使用 todo-write 工具)
  ├─ 分解为 30min-2h 的独立步骤
  ├─ 设置优先级和依赖
  └─ 输出结构化的执行计划
       │
       ▼
  Step 3: 逐步执行 Todo
  ├─ 取下一个Todo → 标记 in_progress
  ├─ 使用 task_create 或其他工具执行
  ├─ 记录产出和决策
  └─ 标记 completed
       │ (循环直到所有Todo完成)
       ▼
  Step 4: 质量检查
  ├─ 回顾是否满足要求
  ├─ 检查产出物质量
  └─ 确认无遗漏
       │
       ▼
  Step 5: 返回结构化报告
  ├─ 执行摘要
  ├─ 详细过程 (每个Todo)
  ├─ 最终交付物
  ├─ 遇到的问题
  └─ 改进建议
```

**关键点**: 每个 SubAgent 都是独立的项目经理！

---

## 🏢 **流程零: 公司初始化 (CEO 就职仪式)** ⭐ *最重要的前置流程*

> **这是 OPC 系统的"起源故事"——在一切自动化执行之前，必须先完成组织架构搭建。**

### 🎭 角色定义

```
┌─────────────────────────────────────────────┐
│              OPC 组织架构                    │
│                                             │
│   [你: 董事会/老板]                          │
│      │                                      │
│      │ "我要开一家XX公司，目标是YYY"         │
│      │ "你(MA)是这家公司的CEO"               │
│      ▼                                      │
│   [MasterAgent: CEO] ◄── 接受任命           │
│      │                                      │
│      ├── Phase 1: 定义自己的岗位职责        │
│      ├── Phase 2: 规划团队需要哪些专家       │
│      ├── Phase 3: 生成 Agent 配置文件        │
│      ├── Phase 4: 将大目标分解为子目标(Goals)│
│      └── Phase 5: 每个目标 WBS 分解为任务树  │
│                                             │
│   ✅ 公司成立！可以开始运营了！              │
└─────────────────────────────────────────────┘
```

### 📋 初始化的5个阶段

#### Phase 1: CEO 自我定义 (岗位职责)

**MA 收到任命后，首先要明确自己是谁、要做什么：**

```markdown
# Master Agent - CEO 岗位职责

## 基本信息
- 职位: Chief Executive Officer (CEO)
- 汇报对象: 董事会 (用户)
- 核心使命: 将 Vision 转化为可执行计划

## 主要职责
1. 战略规划与目标分解
2. 团队建设与管理  
3. 进度监控与质量控制
4. 利益相关方沟通（向用户汇报）
5. 持续改进

## 权限范围
✅ 可自主决策:
- 任务分配策略和优先级
- 资源调配（预算内）
- 技术方案选择

⚠️ 需请示董事会:
- 变更公司愿景或战略目标
- 超出预算的支出
- 核心技术栈变更
```

**输出文件:** `runtime/agents/master-agent.md`

---

#### Phase 2: 团队架构规划 (专家需求)

**基于业务目标，识别需要哪些专业技能：**

| 角色标识 | 名称 | 适用场景 |
|---------|------|---------|
| `@frontend-dev` | 前端开发工程师 | Web应用/SaaS产品 |
| `@backend-dev` | 后端开发工程师 | 服务端/API/数据处理 |
| `@fullstack` | 全栈工程师 | MVP快速开发 |
| `@architect` | 系统架构师 | 大型项目/技术选型 |
| `@designer` | UI/UX设计师 | 产品界面设计 |
| `@writer` | 文档工程师 | API文档/教程 |
| `@tester` | QA测试工程师 | 质量保证 |
| `@devops` | DevOps工程师 | 部署运维 |

**示例团队配置 (AI SaaS公司):**
```
核心团队 (必须):
├── @architect      [架构师]
├── @fullstack      [全栈开发]
├── @designer       [UI/UX]
└── @writer         [文档]

扩展团队 (按需):
├── @tester         [QA]
├── @devops         [运维]
└── @researcher     [研究]
```

---

#### Phase 3: 生成 Agent 配置文件

**为每个角色创建详细的岗位说明书：**

```
runtime/agents/
├── README.md              # 使用说明
├── master-agent.md        # CEO (你自己)
├── frontend-dev.md        # 前端开发
├── backend-dev.md         # 后端开发
├── fullstack.md           # 全栈开发
├── architect.md           # 系统架构师
├── designer.md            # UI/UX设计师
├── writer.md              # 文档工程师
├── tester.md              # QA测试
└── ... (更多角色)
```

**每个 Agent 文件包含:**
- 身份定位和专业背景
- 核心能力列表
- 主要职责描述
- 工作风格特征
- System Prompt 模板
- 工具权限说明
- KPI 指标

**同时注册到 GraphDB:**
```json
{
  "label": "Resource",
  "id": "agent-frontend-dev",
  "properties": {
    "name": "@frontend-dev",
    "type": "agent",
    "role": "高级前端开发工程师",
    "config_file": "runtime/agents/frontend-dev.md"
  }
}
```

---

#### Phase 4: 大目标分解 (战略规划)

**将用户的宏大 Vision 拆解为 3-7 个阶段性 Goals:**

**示例: "6个月内推出MVP并获取100个付费用户"**

```markdown
Goal 1: 产品定义与技术验证 (Month 1)
├── 目标: 完成PRD和技术架构设计
├── 成功标准: PRD通过评审, POC验证可行
└── 截止时间: Week 4

Goal 2: MVP核心功能开发 (Month 2-3)
├── 目标: 完成80%核心功能
├── 成功标准: 认证系统 + 业务流程 + 管理后台
└── 截止时间: Week 12

Goal 3: 内测与质量保证 (Month 4)
├── 目标: 达到可发布状态
├── 成功标准: 测试覆盖率>80%, 无P0/P1 Bug
└── 截止时间: Week 16

Goal 4: 公测与用户获取 (Month 5-6)
├── 目标: 获取100个付费用户
├── 成功标准: 注册>1000, 转化率>10%
└── 截止时间: Week 24
```

**创建到 GraphDB (is_goal=true)**

---

#### Phase 5: 任务树分解 (执行计划)

**对每个 Goal 调用 `proj-decompose-goal` Skill 进行WBS分解:**

```
Goal 1 (验证)
  └── proj-decompose-goal → 任务树 (Level 1-3)
      ├── 建立关系: PARENT_OF, DEPENDS_ON
      └── 分配资源: REQUIRES (@architect, @fullstack)

Goal 2 (MVP)
  └── proj-decompose-goal → 任务树
      └── ...

(对所有 Goals 重复此过程)
```

---

### ✅ 初始化完成标志

当所有5个Phase完成后，MA 向用户输出:

```markdown
# 🎊 公司成立报告

尊敬的董事会:

我就任 {公司名} 的 CEO。

## ✅ 我已完成的工作
1. ✅ CEO岗位职责已定义 → runtime/agents/master-agent.md
2. ✅ 团队架构已规划 → N 个专家角色
3. ✅ Agent配置已生成 → runtime/agents/*.md (N个文件)
4. ✅ 战略目标已分解 → M 个 Goals
5. ✅ 执行计划已制定 → K 个 Tasks

## 🚀 准备就绪!
公司已正式成立，我可以开始指挥团队工作了。

您的指示?
_您的 CEO: Master Agent_
```

**此时，完整的 OPC 系统已经可以开始运营了！**

---

## 🔄 完整工作流程 (初始化之后)

### 流程一: 创建新目标并自动化执行

```
[你] 在 TUI 中输入:
> 帮我创建一个新目标: 完成 Q2 产品发布，截止日期 6月30日

[MasterAgent] 选择 Skill: proj-decompose-goal

Step 1: 记录目标到 GraphDB
  proj_add {
    label: "Task",
    id: "goal-q2-2026",
    properties: {
      description: "完成Q2产品发布",
      is_goal: true,
      status: "pending",
      priority: "urgent",
      deadline: "2026-06-30",
      recurrence: { type: "quarterly", next_due: "2026-04-01" }
    }
  }

Step 2: WBS分解 (AI推理)
  Level 1: 5个主要交付物
    ├── 1. 需求分析与PRD
    ├── 2. UI/UX设计
    ├── 3. 后端API开发
    ├── 4. 前端页面实现
    └── 5. 测试与上线
  
  Level 2: 18个工作包 (每个2-8h)
  
  Level 3: 42个具体活动 (可选细化)

Step 3: 建立依赖关系
  proj_relate (建立 DEPENDS_ON 关系, ~15个)

Step 4: 资源分配
  为每个叶子任务:
  - proj_add (Resource节点)
  - proj_relate (REQUIRES关系)
  
  示例:
  task-L2-03-api-dev → requires → [@backend-dev, bash, graph-query]

Step 5: 输出结果
  返回完整的任务树可视化 + 执行计划建议

[你] 看到:
  📋 任务分解完成!
  总计: 5个交付物 → 18个工作包 → 42个活动
  关键路径: 需求分析 → API设计 → 后端开发 → 集成测试
  预估总工时: 120人天
  建议: 立即启动? [Y/n]
```

---

### 流程二: 每日自动化执行 (Cron触发)

```
[Cron] 09:00 触发:
$ mindx whisper "检查并完成今天的任务"

[Whisper] 连接Gateway → 发送消息 → 立即退出 (耗时0.3秒)

[MasterAgent] 收到消息 → 选择 Skill: proj-daily-execution

═════════════════════════════════════
Phase 1: 查询今日任务 (09:00-09:01)
═════════════════════════════════════

proj_query: 查找 status=pending/in_progress 的任务

返回今日待办清单:
┌──────────────────────────────────────────────┐
│ 今日任务: Q2产品发布 (进度: 25%)            │
│                                              │
│ [P0] 🔴 API文档编写 - 截止: 今天          │
│       状态: pending | 预计: 3h             │
│                                              │
│ [P1] 🟡 登录页实现 - 截止: 明天           │
│       状态: pending | 负责: @frontend-dev  │
│                                              │
│ [P1] 🟡 认证模块修复 - 截止: 明天         │
│       状态: pending | 负责: @backend-dev   │
│                                              │
│ [P2] 🟢 性能优化 - 截止: 下周             │
│       状态: blocked (依赖: API文档)        │
└──────────────────────────────────────────────┘

═════════════════════════════════════
Phase 2: 分配并执行 (09:01-12:00)
═════════════════════════════════════

任务1: API文档编写
  判断: 简单任务 (文档类)
  执行: task_create (MA同步执行)
  结果: ✅ 完成 (耗时20分钟)
  更新: proj_update {status: completed, summary: "完成API v2.0文档"}

任务2: 登录页实现
 判断: 复杂任务 (需要前端专业技能)
  执行: subagent @frontend-dev
  prompt: |
   你是前端开发专家，负责实现登录页面...
    
    ## 工作流程要求
    Step 1: 分析任务
    Step 2: todo-write 创建列表
    Step 3: 逐步执行每个Todo
    ...
  
  [@frontend-dev 内部]:
    ① 分析需求 → 识别需要: React + Tailwind + 表单验证
    ② todo-write([
        {id:"1", content:"分析现有代码结构", status:"in_progress"},
        {id:"2", content:"实现登录表单组件", status:"pending"},
        {id:"3", content:"添加表单验证逻辑", status:"pending"},
        {id:"4", content:"样式调整和响应式", status:"pending"},
        {id:"5", content:"测试和修复bug", status:"pending"}
      ])
    ③ 执行Todo[1]: 直接分析 → completed
    ④ 执行Todo[2]: task_create("实现登录表单") → waiting... → completed
    ⑤ 执行Todo[3]: task_create("添加验证") → waiting... → completed
    ⑥ 执行Todo[4]: write_file(Login.jsx) → completed
    ⑦ 执行Todo[5]: task_create("测试登录流程") → waiting... → completed
    ⑧ 质量检查通过
    ⑧ 返回报告给MA

  MA收到结果:
  更新: proj_update {status: completed, progress: 1.0, summary: "完成登录页实现，包含表单验证和响应式布局"}

任务3: 认证模块修复
  执行: subagent @backend-dev (类似流程...)
  结果: ✅ 完成 (耗时1.5h)

═════════════════════════════════════
Phase 3: 更新目标进度 (12:00-12:05)
═════════════════════════════════════

查询 goal-q2-2026 的所有子任务状态:
- 已完成: 8/18 (44%)
- 进行中: 3/18 (17%)
- 待处理: 7/18 (39%)

计算新进度:
  旧进度: 25%
  新进度: 48% (+23% 今日)

更新: proj_update {progress: 0.48}

═════════════════════════════════════
Phase 4: 生成日报 (12:05-12:10)
═════════════════════════════════════

调用 Skill: proj-generate-report

收集今日完成的任务简报 (从GraphDB):
- task-L3-xxx: "完成API文档v2.0" (@writer)
- task-L2-xxx: "实现登录页面" (@frontend-dev)
- task-L2-xxx: "修复认证bug #123" (@backend-dev)

分析数据:
- 完成任务数: 5个
- 平均工时: 2.3h
- 按时率: 100%
- 效率提升: +15% vs 上周

生成Markdown报告:

# 每日工作报告 - 2026-04-26

## 📊 总体进展
| 目标       | 当前进度 | 今日变化 |
| ---------- | -------- | -------- |
| Q2产品发布 | 48%      | +23% 🚀   |

## ✅ 今日完成任务 (5个)
1. **API文档编写** - @MasterAgent
   > 完成RESTful API v2.0文档，包含认证、用户、订单模块
   
2. **登录页实现** - @frontend-dev
   > 实现响应式登录表单，支持邮箱/手机号登录
   
3. **认证模块修复** - @backend-dev
   > 修复JWT token刷新逻辑，解决过期问题

## 💡 AI洞察
- 今日效率高于平均15%，主要因为任务分解合理
- @frontend-dev 和 @backend-dev 配合默契
- 建议: 明天可以启动集成测试

写入文件: runtime/documents/daily-reports/2026-04-26.md

═════════════════════════════════════
✅ 全部完成! MA进入待机状态
═════════════════════════════════════
```

---

### 流程三: 按需查看进度 (TUI交互)

```
[你] 启动: mindx tui

[你]> 今天的工作进展如何?

[MasterAgent] 读取 GraphDB 数据:

📊 今日实时状态 (截至 14:30):

┌────────────────────────────────────────┐
│ 🎯 Q2产品发布                           │
│ 进度: 62% | 今日变化: +37% 🎉          │
│ 状态: 🟢 正常推进                        │
└────────────────────────────────────────┘

已完成 (7/18 tasks): ████████████████░░░░ 72%
进行中 (3/18 tasks):  ████░░░░░░░░░░░░░░ 22%
待处理 (8/18 tasks): ░░░░░░░░░░░░░░░░░░   6%

🕐 时间线:
09:00  ✅ API文档编写 (MA)
10:30  🔄 登录页实现 (@frontend-dev) - 进行中
11:00  ✅ 认证修复 (@backend-dev)
11:45  🔄 用户管理页 (@frontend-dev) - 进行中
13:00  ✅ 单元测试套件 (@tester)
14:00  🔄 API集成测试 (@backend-dev) - 进行中

💡 预计剩余任务可在 17:00 前完成

[你]> 展示完整的今日工作报告

[MasterAgent] 读取并格式化报告文件:
runtime/documents/daily-reports/2026-04-26.md

(展示完整Markdown报告...)

[你]> 明天的工作计划是什么?

[MasterAgent] 基于 GraphDB 数据预测:

📋 明日待办 (优先级排序):
1. [P0] 前后端联调集成 (依赖: 今日完成的API和前端)
2. [P1] 用户验收测试 (UAT)
3. [P1] 性能优化 (数据库查询)
4. [P2] 部署文档编写

👥 资源分配建议:
- @fullstack: 负责联调 (熟悉前后端)
- @qa: 负责UAT
- @backend-dev: 性能优化
- @writer: 部署文档

⚠️ 注意事项:
- 联调任务可能遇到兼容性问题，建议预留缓冲时间
- UAT需要真实环境数据，请提前准备

[你] 满意地关闭 TUI 😊
```

---

## 🎬 场景模拟: OPC 一周实战

让我们模拟一个真实的 OPC (一人公司) 如何使用这套系统运营一周。

### 背景

**你**: 独立开发者/创业者，正在构建一个 SaaS 产品  
**产品**: AI 驱动的项目管理工具 (就是我们现在设计的这个!)  
**团队**: 只有你 + AI Agent 团队  
**目标**: 2周内完成 MVP 并上线

---

### Day 1 (周一): 系统初始化与目标设定

```
08:50  [你] 启动 MindX 服务
       $ mindx start
       
       Gateway starting on ws://localhost:1314/ws...
       Master Agent initialized ✅
       GraphDB connected ✅
       Ready to serve...

09:00  [你] 打开 TUI，创建第一个目标
       $ mindx tui
       
       > 我要在2周内完成 ProjectTools MVP 开发，
       > 包括: 6个ProjectTools, 3个核心Skills,
       > Whisper CLI, 以及完整的文档。

[MA] 选择 Skill: proj-decompose-goal

[MA] 开始分析和分解...

10分钟后输出:

📋 目标已创建: goal-mvp-v1
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
WBS 分解结果:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Level 1: 6个主要阶段
├── Phase 1: 架构设计与技术选型 (Day 1-2)
├── Phase 2: ProjectTools 实现 (Day 3-6)
│   ├── Tool 1: proj_add
│   ├── Tool 2: proj_query
│   ├── Tool 3: proj_get
│   ├── Tool 4: proj_update
│   ├── Tool 5: proj_relate
│   └── Tool 6: proj_delete
├── Phase 3: Skills 实现 (Day 7-9)
│   ├── Skill 1: proj-daily-execution
│   ├── Skill 2: proj-decompose-goal
│   └── Skill 3: proj-generate-report
├── Phase 4: Whisper CLI (Day 10)
├── Phase 5: 集成测试 (Day 11-12)
└── Phase 6: 文档与上线 (Day 13-14)

总计: 28个工作包 | 预估: 14天 | 关键路径清晰

💡 建议: 立即配置 Cron 自动化?
[你]> 是的，配置每日自动执行

[MA] 配置建议:

# 编辑 crontab:
crontab -e

# 添加以下行:
0 9 * * 1-5 /usr/local/bin/mindx whisper "继续MVP开发工作"

# 这将在工作日每天早上9点自动触发

[你] 完美！让我先手动触发一次试试
> whisper "开始Phase 1: 架构设计"

[Whisper] ✅ 消息已发送 (0.2秒)

[MA] 收到指令，开始执行...
```

---

### Day 1-2 (周一-周二): 架构设计阶段

```
Day 1 上午:
[MA] 自动执行架构设计任务
  ├── task_create "设计系统架构图" → ✅ 完成
  ├── task_create "选择技术栈确认" → ✅ 完成
  └── task_create "编写DESIGN.md初稿" → ✅ 完成

Day 1 下午:
[你] 想查看进展
> 今天的设计成果有哪些?

[MA] 展示:
📄 已生成文档:
- architecture.md (系统架构图)
- tech-stack.md (技术选型说明)
- DESIGN-v0.1.md (初始设计文档)

💡 设计决策:
- 采用 GoGraph + GoReact 技术栈
- WebSocket 实现实时通信
- GraphDB 存储项目数据

Day 2:
[你] 继续让系统自动工作
(无需干预，Cron会在明天9点自动触发)

[Cron 09:00] whisper "继续MVP开发" → 触发

[MA] 执行 Phase 1 剩余任务
  ├── subagent @architect "详细设计API接口"
  │   └── SA: 分析→TodoWrite(5步)→逐步执行→完成 ✅
  ├── subagent @writer "完善设计文档"
  │   └── SA: 分析→TodoWrite(4步)→逐步执行→完成 ✅
  └── task_create "设计评审检查" → ✅ 完成

[MA] 生成 Day 2 日报:
# MVP开发日报 - Day 2
## 进展: Phase 1 完成 95%
## 今日完成任务: 6个
## 明日: 进入 Phase 2 (Tools实现)
```

---

### Day 3-6 (周三-周六): 核心开发阶段

```
这段时间完全自动化运行!

每天早上9点，Whisper自动触发 → MA自主安排工作

Day 3 (周三):
[Cron] → [MA] → Phase 2 开始
  ├── subagent @go-expert "实现 proj_add Tool"
  │   └── 完成Tool代码 + 单元测试 ✅
  ├── subagent @go-expert "实现 proj_query Tool"
  │   └── 完成查询逻辑 + 分页支持 ✅
  └── ...

Day 4 (周四):
  ├── proj_get ✅
  ├── proj_update ✅
  └── 开始 proj_relate

[你] 中午想看看进度
> 本周开发进度?

[MA]:
📊 MVP 开发进度 (Day 4)
━━━━━━━━━━━━━━━━━━━━
Phase 1: 架构设计    ████████████████████ 100% ✅
Phase 2: Tools开发   ██████████████░░░░░░ 65% 🔄
  ├── proj_add     ✅
  ├── proj_query    ✅
  ├── proj_get      ✅
  ├── proj_update   ✅
  ├── proj_relate   🔄 (80%)
  └── proj_delete   ⏳ (未开始)
Phase 3: Skills      ░░░░░░░░░░░░░░░░░░░   0%
Phase 4: Whisper     ░░░░░░░░░░░░░░░░░░░   0%

总体进度: 48% | 按计划推进中 ✅

Day 5-6:
  ├── proj_relate ✅
  ├── proj_delete ✅
  ├── Phase 2 完成! 🎉
  └── 进入 Phase 3: Skills 实现

[周末] 系统休息，你也可以休息 😊
```

---

### Day 7-9 (下周一周): Skills 开发

```
现在 ProjectTools 已经就绪，开始实现 Skills

Day 7:
[MA] 实现 proj-daily-execution Skill
  ├── 编写 SKILL.md 工作流定义 ✅
  ├── 测试基本流程 ✅
  └── 优化异常处理 ✅

Day 8:
[MA] 实现 proj-decompose-goal Skill
  ├── WBS分解算法 ✅
  ├── 依赖检测 ✅
  └── 资源分配策略 ✅

Day 9:
[MA] 实现 proj-generate-report Skill
  ├── 日报模板 ✅
  ├── 周报模板 ✅
  └── 数据分析逻辑 ✅
```

---

### Day 10: Whisper CLI

```
[MA] 自动实现 Whisper
  ├── cmd/whisper.go ✅
  ├── 测试 fire-and-forget 模式 ✅
  └── 验证 Cron 集成 ✅

[你] 测试 Whisper:
$ mindx whisper --verbose "测试Whisper功能"
🚀 连接到 ws://localhost:1314/ws...
✅ 消息已发送
⚡ 退出 (耗时0.15s)

完美！
```

---

### Day 11-12: 集成测试

```
[MA] 自动执行全面测试
  ├── 端到端流程测试 ✅
  ├── 多SubAgent协作测试 ✅
  ├── 压力测试 (并发10个任务) ✅
  └── 异常恢复测试 ✅

发现并修复:
  - Bug #1: 循环依赖检测误报 → 已修复
  - Bug #2: 大数据量查询性能问题 → 添加索引
  - Bug #3: SubAgent超时处理 → 增加重试机制
```

---

### Day 13-14: 文档与上线

```
[MA] 生成完整文档集
  ├── README.md (快速开始) ✅
  ├── ARCHITECTURE.md (架构说明) ✅
  ├── USER_GUIDE.md (用户手册) ✅
  └── API_REFERENCE.md (API参考) ✅

[你] 最终验收:
> 生成项目总结报告

[MA] 调用 proj-generate-report (里程碑报告):

# 🎉 MVP 发布里程碑报告
=====================================

项目: ProjectTools OPC 系统
周期: 2026-04-26 ~ 2026-05-09 (14天)

## 📊 成果统计
━━━━━━━━━━━━━━━━━━━━━━
总任务数:     156 个
完成任务:     148 个 (94.9%) ⭐
按时完成率:   92.3%
平均效率提升: 35% vs 手动开发

## 🏗️ 交付物
━━━━━━━━━━━━━━━━━━━━━━
✅ 6个 ProjectTools (Go代码)
✅ 3个核心 Skills (SKILL.md)
✅ 1个 Whisper CLI (Go代码)
✅ 完整文档集 (4份核心文档)
✅ GraphDB Schema 设计
✅ Cron 自动化配置
✅ 集成测试报告

## 💰 成本对比
━━━━━━━━━━━━━━━━━━━━━━
传统开发团队:
  - 1名项目经理:  ¥25,000 × 0.5月 = ¥12,500
  - 2名全栈开发:  ¥40,000 × 0.5月 = ¥20,000
  - 1名测试工程师: ¥15,000 × 0.5月 = ¥7,500
  - 总计: ¥40,000

OPC 模式 (你 + AI):
  - 你的时间: 约 4小时/天 × 14天 = 56小时
  - 云服务器: ¥200 × 0.5月 = ¥100
  - LLM API费用: 约 ¥500
  - 总计: 你的时间 + ¥600

**节省: 98.5% 的金钱成本**
**获得: 7×24 不知疲倦的AI团队**

## 🌟 创新亮点
━━━━━━━━━━━━━━━━━━━━━━
1. 🤖 Agent自治: SubAgents自主管理任务
2. 📊 GraphDB记忆: 完整的状态追溯
3. 🔄 全自动闭环: 从触发到汇报零干预
4. ⚡ Whisper触发器: 极简的自动化入口
5. 📈 智能报告: AI洞察而非数据罗列

## 🚀 下一步规划
━━━━━━━━━━━━━━━━━━━━━━
- Week 3-4: 用户反馈收集与迭代
- Month 2: 功能增强 (风险管理/质量门禁)
- Quarter 1: 商业化版本发布

=====================================
🎊 MVP 开发成功! OPC 模式验证通过!
=====================================
```

---

## 📋 工具与技能完整清单

### ProjectTools (必须实现)

| #   | Tool名称        | 函数签名                           | 核心Cypher                                                   | 复杂度 |
| --- | --------------- | ---------------------------------- | ------------------------------------------------------------ | ------ |
| 1   | **proj_add**    | `Add(ctx, params) (any, error)`    | `CREATE (n:$label {id:$id}+$props) RETURN n`                 | ⭐⭐     |
| 2   | **proj_query**  | `Query(ctx, params) (any, error)`  | `MATCH (n{$label}) WHERE {$filters} RETURN n LIMIT $limit`   | ⭐⭐⭐    |
| 3   | **proj_get**    | `Get(ctx, params) (any, error)`    | `MATCH (n{id:$id}) OPTIONAL MATCH paths RETURN n, relations` | ⭐⭐⭐    |
| 4   | **proj_update** | `Update(ctx, params) (any, error)` | `MATCH (n{id:$id}) SET n += $fields RETURN n`                | ⭐⭐     |
| 5   | **proj_relate** | `Relate(ctx, params) (any, error)` | `MATCH (a),(b) CREATE (a)-[:$rel $props]->(b)`               | ⭐⭐     |
| 6   | **proj_delete** | `Delete(ctx, params) (any, error)` | `MATCH (n{id:$id}) DETACH DELETE n`                          | ⭐⭐     |

**参考实现**: `internal/tools/graphquery/graph_query.go`

### Skills (已实现) - 4个核心Skills

| Skill                    | 文件                                           | 行数   | 核心功能          | 触发时机        |
| ------------------------ | ---------------------------------------------- | ------ | ----------------- | --------------- |
| **proj-initialize** ⭐    | `runtime/skills/proj-initialize/SKILL.md`       | ~600行 | 公司初始化/CEO就职  | **首次使用(必须)** |
| **proj-daily-execution** | `runtime/skills/proj-daily-execution/SKILL.md` | ~350行 | 4阶段每日执行流程   | Cron/每日触发     |
| **proj-decompose-goal**  | `runtime/skills/proj-decompose-goal/SKILL.md`  | ~450行 | 5阶段WBS分解      | 创建目标时       |
| **proj-generate-report** | `runtime/skills/proj-generate-report/SKILL.md` | ~550行 | 5种报告类型       | 每日/周/月触发   |

### Agent 模板体系 (已实现)

| 组件          | 路径                                  | 说明           |
| ------------- | ------------------------------------- | -------------- |
| **README**    | `runtime/agents/README.md`             | 使用说明和规范 |
| **CEO模板**   | `runtime/agents/master-agent.md`         | MA自身角色定义  |
| **前端开发**  | `runtime/agents/frontend-dev.md`         | 前端工程师配置  |
| **后端开发**  | `runtime/agents/backend-dev.md`          | 后端工程师配置  |
| *(更多模板)*  | `runtime/agents/{role-name}.md`         | 按需生成       |

### CLI 工具 (已实现)

| 工具        | 文件             | 功能   |
| ----------- | ---------------- | ------ |
| **whisper** | `cmd/whisper.go` | ~250行 | Fire-and-forget触发器 |

### 配套基础设施

| 组件         | 配置                        | 说明           |
| ------------ | --------------------------- | -------------- |
| **Cron**     | crontab                     | 定时触发自动化 |
| **目录结构** | runtime/documents/          | 报告输出目录   |
| **环境变量** | MINDX_WS_ADDR, MINDX_PROJDB | 连接配置       |

---

## 🎯 实现优先级路线图

### Phase 1: 核心工具 (本周)

- [ ] 实现 `proj_add` - 创建节点
- [ ] 实现 `proj_query` - 查询列表
- [ ] 实现 `proj_update` - 更新属性
- [ ] 实现 `proj_relate` - 建立关系
- [ ] 实现 `proj_get` - 获取详情
- [ ] 实现 `proj_delete` - 删除节点

**验收标准**: 能在 TUI 中手动创建/查询/更新任务

### Phase 2: 集成测试 (下周)

- [ ] 注册 Tools 到 MindX Gateway
- [ ] 测试 Skill 与 Tool 的配合
- [ ] 验证 SubAgent 工作流程
- [ ] 配置第一个 Cron 任务

**验收标准**: `whisper "测试"` 能触发完整流程

### Phase 3: 生产优化 (后续)

- [ ] 性能优化 (索引、缓存)
- [ ] 错误处理增强
- [ ] 监控和日志
- [ ] 更多 Skills (风险/质量/排期)

---

## 💡 OPC 使用最佳实践

### 1️⃣ 目标设定的艺术

```bash
# ❌ 太模糊
> 做一个网站

# ✅ SMART目标
> 在2周内完成SaaS产品的MVP开发，包括：
> - 6个后端API端点
> - 3个核心前端页面
> - 用户认证系统
> - 基础的管理后台
> 截止日期: 2026-05-10
```

### 2️⃣ Cron 策略

```bash
# 🌅 工作日节奏
0 9  * * 1-5   mindx whisper "开始每日工作"        # 早上启动
0 12 * * 1-5   mindx whisper --tag noon "午间进度检查"  # 中午检查
0 18 * * 1-5   mindx whisper --tag eod "下班前汇总"    # 晚上收尾

# 📅 周期性任务
0 9  * * 1     mindx whisper --tag weekly "周报生成"   # 周一早上
0 10 1 * *     mindx whisper --tag monthly "月度回顾"  # 每月1号

# 🚨 应急触发
# 手动: mindx whisper --priority urgent "紧急: 线上bug"
```

### 3️⃣ TUI 交互技巧

```bash
# 常用查询
> 今天做了什么?              # 快速摘要
> 本周进展如何?              # 周度视图
> XXX目标的详细任务树?        # 深入查看
> 生成一份完整的里程碑报告    # 导出报告

# 管理操作
> 暂停任务 XXX              # 紧急暂停
> 重新规划目标 YYY           # 调整方向
> 给 @frontend-dev 增加任务   # 动态调配
```

### 4️⃣ SubAgent 团队建设

```bash
# 推荐的专业 Agent 配置:

@frontend-dev
  System Prompt: "你是高级前端工程师，精通 React/Vue/TypeScript..."
  擅长: UI实现、性能优化、响应式设计

@backend-dev
  System Prompt: "你是后端架构师，精通 Go/Python/数据库..."
  擅长: API设计、数据库优化、并发处理

@writer
  System Prompt: "你是技术文档专家..."
  擅长: README、API文档、架构文档

@tester
  System Prompt: "你是QA工程师..."
  擅长: 自动化测试、边界情况、性能测试

@architect
  System Prompt: "你是系统架构师..."
  擅长: 技术选型、架构设计、性能规划
```

---

## 🔮 未来展望

### 短期 (1-2月)

- [ ] 更多 Skills (风险管理/质量门禁/智能排期)
- [ ] Agent 间协商机制 (资源冲突解决)
- [ ] 历史数据分析 (效率趋势/瓶颈识别)
- [ ] Web UI 界面 (替代TUI的可视化界面)

### 中期 (3-6月)

- [ ] 多项目管理 (同时运营多个项目/产品)
- [ ] Agent 学习系统 (从历史任务中学习优化)
- [ ] 第三方集成 (GitHub/Jira/Slack通知)
- [ ] 移动端 App (随时查看和管理)

### 长期 (6月+)

- [ ] Agent 市场 (购买/出售专业Agent)
- [ ] OPC 协作网络 (多个OPC协同工作)
- [ ] 自主进化 (系统能自我改进)
- [ ] 通用AGI平台 (不限于项目管理)

---

## 📚 参考资源

- [GoReact Framework](https://github.com/DotNetAge/goreact) - Agent框架
- [GoGraph](https://github.com/DotNetAge/gochat) - 图数据库
- [WBS方法论](https://www.pmi.org/pmbok-guidestandards/projects/project-scope-management/wbs) - 工作分解结构
- [OPC概念](https://en.wikipedia.org/wiki/One-person_company) - 一人公司模式

---

## 📝 变更日志

| 日期       | 版本   | 内容                               | 作者 |
| ---------- | ------ | ---------------------------------- | ---- |
| 2026-04-26 | v1.0.0 | 初始完整设计 (OPC架构+全流程+场景) | Ray  |
