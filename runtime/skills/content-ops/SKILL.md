---
name: content-ops
description: >
  China-native content operations — plan content calendars, manage multi-platform
  production pipelines (小红书/公众号/抖音/B站/知乎/微博), coordinate specialist
  creators via team orchestration, maintain brand consistency across Chinese media
  ecosystem, A/B test performance, and deliver publish-ready assets on recurring schedules.
  Includes platform-specific writing quality standards loaded from references/.
allowed-tools: bash sub-agent collect-results task-create task-update task-get task-list team-create team-list team-get-tasks find-experts app-promotion content-factory
metadata:
  name_zh: 内容运营
  name_zh-tw: 內容運營
  description_zh: 中国媒体生态内容运营——多平台生产流水线、品牌一致性管理、平台原生写作质量标准（references/按需加载）、A/B测试、定时交付
  description_zh-tw: 中國媒體生態內容運營——多平台生產流水線、品牌一致性管理、平台原生寫作質量標準、A/B測試、定時交付
---

## Trigger Decision

Use this skill when:

- User needs **ongoing content production** for **Chinese media platforms** (小红书/公众号/抖音/B站/知乎/微博)
- User requires **multi-platform output** from a single source topic (一篇内容 → 小红书笔记 + 公众号文章 + 抖音脚本 + B站视频)
- User asks for **editorial calendar management** across Chinese social platforms
- User needs **platform-specific writing quality guidance** — not just "write good content" but "write for 小红书's CES algorithm"
- User mentions **content performance optimization** on Chinese platforms (互动率/完读率/收藏率/转发率)
- User runs a **content team or agency** producing for Chinese audiences at scale
- Task involves **recurring schedules** (日更/周更/月度专题)

**Do NOT use** for a single one-off piece of content — use `copywriting` or `content-factory` directly.

**Composition model:**
```
content-factory  →  REMIXES one asset into N formats (on-demand, generic)
content-ops      →  PRODUCES publish-ready assets for CHINESE PLATFORMS at scale (recurring)
app-promotion     →  DISTRIBUTES and promotes published assets across Chinese channels
```

---

## Domain Knowledge Base

### Language Handling for GraphRAG Queries

> **CRITICAL: Query GraphRAG in Chinese when working with Chinese content.**

- User speaks/writes in **Chinese** → Query `mindx memory query` and `mindx graph query` in **Chinese first**
- When storing via `mindx memory store` → Store in **Chinese**
- Graph node `properties` values → Use Chinese for Chinese content entities
- Cypher string literals → Use Chinese when matching Chinese node properties

### GraphRAG Dual-Engine Architecture

> **Two storage layers — you (the LLM) are the bridge via dynamic Cypher.**

**Layer 1: Graph — Entity Relationship Index**
- Nodes: `id`, `type`, `name`, `properties` (`description`, `confidence`, + custom fields)
- Edges: `type`, `source`, `target`, `predicate`, `properties`
- Write: `mindx graph upsert-nodes/--edges`
- Read: `mindx graph query --cypher "<dynamic Cypher>"`

**Layer 2: NativeRAG — Semantic Overview Index**
- Chunks with vector embeddings: content, title, tags, positions, doc_id
- Write: `mindx memory store --content "..." --title "..."`
- Read: `mindx memory query "<search terms>"`

**The link:** Both layers share `doc_id` — Graph node ↔ NativeRAG chunks.

**When to use which:**
| Need                                                                           | Command                                |
| ------------------------------------------------------------------------------ | -------------------------------------- |
| Find relevant knowledge/documents                                              | `mindx memory query` (semantic search) |
| Store new insights/learnings                                                   | `mindx memory store` (vector index)    |
| Build structured business state (calendar, brand guidelines, performance data) | `mindx graph upsert-nodes/edges`       |
| Query relationships between entities                                           | `mindx graph query --cypher "..."`     |
| Cross-reference: entity → full context                                         | Graph node → doc_id → `memory query`   |

Before any production cycle, load domain knowledge:

```bash
# Load brand voice and audience profiles
mindx memory query "品牌调性 目标用户画像 内容风格指南"

# Load platform-specific performance data
mindx memory query "各平台数据表现 互动率 完读率 爆款分析"

# Query knowledge graph for stored brand assets and templates
mindx graph query --cypher "
  MATCH (b:Brand)-[:HAS_GUIDELINE]->(g:Guideline)
  MATCH (b)-[:HAS_PERSONA]->(p:Persona)
  MATCH (b)-[:HAS_TEMPLATE]->(t:ContentTemplate)
  RETURN b.name, g.type, g.content, p.name, p.segment, t.format, t.structure
  LIMIT 30
"

# Query historical content performance by platform
mindx graph query --cypher "
  MATCH (c:ContentPiece {brand_id:'$BRAND_ID'})
  WHERE c.published_at > datetime('now - 90 days')
  RETURN c.title, c.format, c.platform,
         c.metrics->>'engagement' as engagement,
         c.metrics->>'reads' as reads,
         c.status
  ORDER BY c.metrics->>'engagement' DESC
  LIMIT 20
"
```

Store learnings after each cycle:

```bash
# Store content performance insights
mindx memory store \
  --content "<本周期内容表现分析：什么火了、为什么火、什么扑了、如何改进>" \
  --title "内容运营洞察：<主题> 第<N>周" \
  --source "content-ops-cycle"

# Upsert content piece nodes to graph for tracking
CONTENT_ID=$(mindx utils uuid)
mindx graph upsert-nodes --nodes '[{
  "id":"'"$CONTENT_ID"'",
  "labels":["ContentPiece"],
  "properties":{
    "brand_id":"<brand_id>",
    "title":"<标题>",
    "format":"<格式>",
    "platform":"<平台>",
    "status":"produced",
    "cycle":"week-<N>",
    "created_at":"<时间戳>",
    "metrics":{}
  }
}]'
```

---

## Platform Intelligence Overview

> Each platform is a different content environment with its own algorithm, user psychology, format rules, and success metrics. **Writing "good content" is meaningless without platform context.**
>
> Detailed writing standards per platform live in `references/`. Load them when producing content for a specific platform. This section provides the strategic overview for cross-platform planning.

### Platform Comparison Matrix

| 维度         | 小红书                                           | 公众号                              | 抖音                                 | B站                                    | 知乎                                  | 微博                                 |
| ------------ | ------------------------------------------------ | ----------------------------------- | ------------------------------------ | -------------------------------------- | ------------------------------------- | ------------------------------------ |
| **核心用户** | 18-35岁女性为主（70%），一二线城市，消费力强     | 25-45岁泛人群，职场/知识/生活全覆盖 | 全年龄段，下沉市场占比高，杀时间神器 | 16-30岁 Z世代，二次元/科技/知识向      | 22-40岁高知群体，一线城市，理性讨论   | 全年龄段，娱乐/追星/热点，碎片化消费 |
| **内容形式** | 图文笔记（首图+多图）+ 短视频                    | 长图文（富文本排版）                | 短视频（15s-3min）+ 直播             | 中长视频（5-20min）+ 直播              | 长问答（深度）+ 文章/想法             | 短图文（1400字内）+ 视频/直播        |
| **流量逻辑** | **搜索+推荐双引擎**，CES评分决定分发             | **订阅+算法+社交裂变**三引擎        | **兴趣推荐**为主，完播率是命脉       | **关注+推荐+搜索**，弹幕互动权重高     | **问题匹配+专业权重**，赞同数决定排序 | **热搜+关注+推荐**，时效性第一       |
| **核心指标** | CES = 点赞×1 + 收藏×1 + 评论×4 + 转发×4 + 关注×8 | 打开率 → 完读率 → 分享率 → 关注转化 | 完播率 → 互动率 → 转化率             | 播放量 → 互动率（弹幕/评论/投币）      | 赞同数 → 感谢数 → 收藏数 → 评论质量   | 转发/评论/点赞，传播速度优先         |
| **最佳时长** | 图文：300-800字；视频：30s-3min                  | 800-2000字（完读率最高区间）        | 15s-60s（短视频黄金期）；3min以内    | 5-15min（中视频甜区）；知识类可到20min | 1000-3000字深度回答；文章2000-5000字  | 1400字以内；短视频15-60s             |
| **变现模式** | 种草→电商（蒲公英平台）                          | 流量主广告 + 付费阅读 + 社群变现    | 星图广告 + 商品橱窗 + 直播带货       | 创作激励 + 恰饭视频 + 直播             | 盐选会员 + 品牌提问 + Live            | 微博广告 + 电商 + V+会员             |
| **发布频率** | 日更1-3篇（算法活跃度权重）                      | 周更2-4篇（质量 > 数量）            | 日更1-3条（保持账号活跃）            | 周更1-3条（中视频制作周期长）          | 周更3-5个回答/文章                    | 日更3-5条（热点追踪型）              |

### 平台间内容流转策略

```
                    ┌─────────────┐
                    │   原创深度   │
                    │  （知乎/公众号）│
                    └──────┬──────┘
                           │ 拆解提炼
              ┌────────────┼────────────┐
              ▼            ▼            ▼
       ┌──────────┐  ┌──────────┐  ┌──────────┐
       │ 小红书   │  │  抖音    │  │   B站    │
       │ 图文/短视 │  │ 短视频   │  │ 中长视频 │
       │ 视觉种草  │  │ 节奏爽感  │  │ 深度讲解  │
       └─────┬────┘  └─────┬────┘  └─────┬────┘
             │             │             │
             └──────┬──────┘             │
                    ▼                    │
              ┌──────────┐               │
              │  微博    │◄──────────────┘
              │ 热点/切片│  碎片化传播 + 引流
              └──────────┘
```

**流转原则：**
- **知乎/公众号** = 深度原创源（一次创作，多次复用）
- **小红书** = 提炼视觉化要点 + 步骤拆解（重搜索流量）
- **抖音** = 提取最抓人的片段做快节奏剪辑（重完播率）
- **B站** = 深度展开讲解 + 弹幕互动设计（重专业性）
- **微博** = 金句切片 + 热点追踪 + 引导回主阵地

### Reference File Loading Protocol

When producing content for a specific platform, load the corresponding reference:

```bash
# Load platform-specific writing standard before drafting
# Example: producing 小红书 content
READ references/xiaohongshu.md   # ← 标题公式 / 正文模板 / CES优化 / SEO布局 / 质量检查卡

# Example: producing 公众号 article
READ references/wechat-oa.md     # ← 4U标题原则 / 黄金开头模板 / 呼吸感排版 / 双引擎分发

# Example: producing 抖音 video script
READ references/douyin.md        # ← 3秒Hook法则 / 脚本分段结构 / 完播率优化 / CTA设计

# Example: producing B站 video script
READ references/bilibili.md      # ← 弹幕工程设计 / 三连引导 / 深度内容结构

# Example: producing 知乎 answer
READ references/zhihu.md         # ← 5层结构法 / 专业权重建设 / 长尾流量策略

# Example: producing 微博 post
READ references/weibo.md         # ← 时效性写作 / 碎片化表达 / 话题运营
```

**Cross-platform production:** When one source topic needs to adapt to multiple platforms, load all relevant reference files and apply each platform's native standards independently.

---

## Content Principles (All Platforms)

> These principles apply regardless of target platform. They are the floor, not the ceiling.

### Write This

- **一个平台一个样**：同一件事，小红书说"姐妹们看过来"，公众号说"本文将为您详细解析"，抖音说"注意听好了！" —— 每个平台的"原生语感"完全不同
- **具体 > 抽象 > 空洞**：不说"效果显著"，说"3天涨粉2000"；不说"很多人都在用"，说"月活超过500万"
- **先给结论再展开**：中国用户耐心有限，先把结果/结论/核心观点亮出来
- **设计互动钩子**：每篇内容的结尾都必须有让用户想要评论/转发/收藏的理由
- **搜索思维**：即使是为推荐算法写内容，也要考虑"如果用户搜这个词，能不能找到我"
- **情绪价值 > 信息价值**：纯粹的信息到处都有，加了情绪（共鸣/愤怒/惊喜/温暖）才会被传播
- **视觉先行**：在中国社交媒体，"好看"和"好用"同等重要，甚至更重要
- **人设一致**：跨平台的人设内核要统一（你是谁、你相信什么、你的说话方式），表现形式可以差异化
- **数据说话**：能用数字就不用形容词，能用案例就不光说道理
- **迭代思维**：发布后24小时内观察数据，快速调整下一期的策略

### Never Write This

- **AI味浓重的表达**："在当今数字化转型的浪潮中""随着科技的飞速发展""值得一提的是""不容忽视" —— 用户瞬间判断为 AI 生成/营销号
- **正确的废话**："我们要坚持用户导向的理念""质量是我们的生命线""不忘初心砥砺前行" —— 没有任何信息量
- **跨平台照搬**：把公众号文章原封不动发小红书，把抖音脚本直接念成B站视频 —— 每个平台有自己的语法
- **标题党空心化**：「震惊！」「必看！！」「转疯了！」—— 2026年的用户早就免疫了，反而会降低信任度
- **自嗨型内容**：只写自己想写的，不考虑"用户为什么要看这个""用户看完能得到什么"
- **长篇大论无结构**：一大段文字没有任何分段、小标题、重点标记 —— 手机端直接划走
- **违规擦边球**：涉及医疗/金融/法律等敏感领域的绝对化表述、夸大宣传、诱导行为
- **搬运/洗稿**：直接复制他人内容或简单换措辞 —— 中国平台打击力度越来越大，且损害长期品牌
- **忽略评论区**：发布即结束的心态 —— 在中国社交媒体，评论区才是内容的"第二战场"

### The Human Test

> 发布前最后一步：大声读出来（或想象读给朋友听）。如果听起来像营销号/AI/教科书，**重写**。如果听起来像一个真人在跟朋友分享有用的信息，**通过**。

---

## Workflow

### Phase 1: Content Brief & Calendar Planning

#### Step 1A: Gather Context

| 问                                | 为什么         | 好的回答示例                                        |
| --------------------------------- | -------------- | --------------------------------------------------- |
| "我们为哪个品牌/账号做内容运营？" | 确定范围和身份 | "小红书账号'XX好物'，专注家居收纳领域"              |
| "主要运营哪些平台？"              | 格式映射       | "小红书主阵地 + 公众号深度内容 + 抖音引流"          |
| "目标用户是谁？"                  | 基调和话题方向 | "25-35岁一线城市租房女性，预算有限但追求品质"       |
| "核心目标是什么？"                | 成功定义       | "小红书月均3篇千赞爆款 + 公众号打开率高于5%"        |
| "目前各平台数据基线是多少？"      | 改进参照       | "小红书均赞200 / 公众号均阅读3000 / 抖音均播放5000" |
| "有没有已有的品牌调性文档？"      | 品牌一致性     | "有的，偏向温暖实用风，不要太网红也不要太高冷"      |
| "内容更新频率要求？"              | 容量规划       | "小红书日更、公众号周更2篇、抖音周更3条"            |
| "竞品账号有哪些？谁做得好？"      | 差异化参考     | "@XXX @YYY，我觉得XXX的封面做得特别好"              |

#### Step 1B: Build Content Calendar

Output a **4-week rolling content calendar** stored as graph nodes:

```bash
CALENDAR_ID=$(mindx utils uuid)

mindx graph upsert-nodes --nodes '[{
  "id":"'"$CALENDAR_ID"'",
  "labels":["ContentCalendar"],
  "properties":{
    "brand_id":"<brand_id>",
    "project_name":"<name>",
    "cycle_start":"<date>",
    "cycle_end":"<date>+28 days",
    "status":"planning",
    "total_pieces":<N>,
    "volume_target":"<target>"
  }
}]'
```

**Calendar view template (4-week rolling):**

```
┌─────────────────────────────────────────────────────────────────────┐
│  CONTENT CALENDAR — {Brand} — Week {N} of {M}                       │
│  Cycle: {start_date} → {end_date}                                   │
├──────┬──────────┬──────────┬──────────┬──────────┬──────────┬─────────┤
│      │   Mon    │   Tue    │   Wed    │   Thu    │   Fri    │  Status │
├──────┼──────────┼──────────┼──────────┼──────────┼──────────┼─────────┤
│ W1   │ XHS:{topic}│ WX:{topic}  │ Research │ DY:{topic}  │ Edit +   │         │
│      │          │          │ + Outline│          │ Review   │         │
├──────┼──────────┼──────────┼──────────┼──────────┼──────────┼─────────┤
│ W2   │ XHS:{topic}│ DY Script│ WX:{topic}  │ B站 Script│ Social:  │         │
│      │          │ :{topic} │          │ :{topic}  │{topic}   │         │
├──────┼──────────┼──────────┼──────────┼──────────┼──────────┼─────────┤
│ W3   │ ZH:{topic}│ XHS:{topic}│ WX:{topic}  │ DY:{topic}  │ Approve │         │
├──────┼──────────┼──────────┼──────────┼──────────┼──────────┼─────────┤
│ W4   │ B站:{topic}│ Weibo:{topic}│ Monthly  │ Next Mo.│ Plan    │         │
│      │          │          │ Review   │ Planning │ Refresh │         │
└──────┴──────────┴──────────┴──────────┴──────────┴──────────┴─────────┘
```

#### Step 1C: Content Brief Template (Platform-Aware)

Every content piece gets a brief before production starts. The brief must reference the appropriate platform writing standard from `references/`.

```
Content Brief — 平台版
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
ID:             {brief_id}
Project:        {project_name}
Platform:       {小红书 / 公众号 / 抖音 / B站 / 知乎 / 微博}
Content Type:   {该平台对应的内容形式}

Title (working):  {工作标题}
Target Audience:  {具体用户画像，不是泛泛的"年轻女性"}

Key Message:      {用户看完后必须记住的一句话}
Core Keyword:     {核心搜索词/话题词}
Interaction Goal: {希望用户做什么：收藏/评论/转发/关注/三连?}

Tone/Voice:       {该平台的原生语感参考}
Reference:        {对标账号或爆款链接}

Platform Standard: {加载 references/{platform}.md 中的质量检查卡}
Deadline:         {date}
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### Phase 2: Production Pipeline Setup

Discover and coordinate specialists via `find-experts`. Do NOT hardcode role assignments — let the expert discovery system match capabilities to needs.

#### Step 2A: Capability Requirements Per Platform

When creating tasks, specify what capability is needed (not who fills it):

| 内容类型        | 所需能力                              | 通过 find-experts 匹配                               |
| --------------- | ------------------------------------- | ---------------------------------------------------- |
| 小红书图文笔记  | 图文策划能力、SEO关键词布局、视觉文案 | find experts with 小红书/种草/SEO experience         |
| 公众号深度文章  | 长文写作能力、标题工程、排版设计      | find experts with 公众号/深度写作/编辑 experience    |
| 抖音短视频脚本  | 脚本撰写、Hook设计、节奏把控          | find experts with 抖音/短视频/脚本 experience        |
| B站中长视频脚本 | 深度内容结构、弹幕互动设计            | find experts with B站/视频脚本/知识类内容 experience |
| 知乎深度问答    | 专业论证、结构化写作、权威建立        | find experts with 知乎/专业写作/行业经验 experience  |
| 微博碎片内容    | 时效性写作、热点捕捉、精炼表达        | find experts with 微博/热点/社交媒体 experience      |
| 跨平台编辑      | 品牌一致性审核、质量终检              | find experts with 编辑/内容审核/多平台 experience    |

#### Step 2B: Create Team via find-experts Mode 2

```bash
# Discover available specialists for this project's platform mix
find-experts(
  mode="multi-expert",
  requirements=[
    "小红书内容创作 / 图文笔记策划 / SEO优化",
    "公众号深度文章 / 标题工程 / 排版",
    "抖音短视频脚本 / Hook设计 / 完播率优化",
    (根据实际需要的平台组合动态调整)
  ],
  project_context="{项目背景、品牌调性、目标平台}"
)

# Assemble the content production team based on discovered experts
team-create(
  team_name="content-{project-name}",
  leader="<orchestrator>",           # You (coordinating agent)
  members=[<从find-experts结果中选取>],
  tasks=[
    "Weekly content brief generation and calendar maintenance",
    "Platform-native content production per brief (loading refs/{platform}.md)",
    "Visual asset production (thumbnails, covers, social graphics)",
    "Cross-platform editorial review and quality gate",
    "Performance tracking and iteration recommendations"
  ]
)
```

#### Step 2C: Set Up Tasks with Dependencies

For each content piece in the calendar, create a task chain. Each task brief includes a pointer to the platform reference file:

```bash
# Example: Create task chain for one 小红书 note
TaskCreate(
  subject="Draft 小红书笔记: {title}",
  description="Write 小红书 note following references/xiaohongshu.md standard. Target audience: {persona}. Key message: {message}. Core keyword: {keyword}. Interaction goal: {goal}. MUST pass the quality checklist at end of xiaohongshu.md.",
  active_form="Drafting 小红书笔记: {title}"
)

TaskCreate(
  subject="Editorial review: {title} ({platform})",
  description="Run full editorial review against brand voice guide AND the platform quality checklist in references/{platform}.md. Flag any deviations.",
  active_form="Editing: {title} ({platform})"
)

# Wire dependency: Edit after Write
TaskUpdate(edit_task, addBlockedBy=[write_task])

# Visual task can start once draft exists (parallel with edit)
TaskCreate(
  subject="Create visuals for: {title} ({platform})",
  description="Design cover image and supporting visuals for '{title}' per {platform} visual specs in references/{platform}.md.",
  active_form="Creating visuals for: {title}"
)
TaskUpdate(design_task, addBlockedBy=[write_task])

TaskCreate(
  subject="Final approval: {title} ({platform})",
  description="Assemble final package. Run platform-specific quality gate card from references/{platform}.md before signoff.",
  active_form="Preparing final package: {title}"
)
TaskUpdate(approval_task, addBlockedBy=[edit_task, design_task])
```

**Dependency graph:**

```
Write ──→ Editorial Review ──┐
   │                         ├──→ Final Approval → Publish
   └────────→ Visual Assets ──┘
```

### Phase 3: Execution — Wave-Based Production

Execute production in coordinated waves. **Each wave loads the appropriate platform reference file before producing content.**

#### Wave 1: Research + Outline (Parallel Across All Pieces)

**Timing:** Day 1–2 of each production cycle

For each piece, identify target platform → load corresponding `references/{platform}.md` → produce outline that conforms to that platform's structure template.

```bash
sub-agent(
  task="Research and outline '{title}' for {platform}. Load references/{platform}.md first, then produce structured outline following that platform's content structure template."
)
# Repeat for each piece in parallel
```

#### Wave 2: First Draft Creation (Parallel, One SubAgent Per Piece)

**Timing:** Day 2–4

Each sub-agent loads the target platform's reference file and drafts according to its specific standards:

```bash
sub-agent(
  task="Write full {platform} draft for '{title}'. Load and follow references/{platform}.md completely — title formulas, body structure, interaction design, SEO layout. Output must pass that platform's quality checklist."
)
```

#### Wave 3: Editing + Revision

**Timing:** Day 4–6

Editor reviews against BOTH brand voice AND the platform-specific quality checklist:

```bash
sub-agent(
  task="Edit '{title}' ({platform} draft). Check against: 1) Brand voice guide 2) references/{platform}.md quality checklist. Flag any non-compliant items. Output edited version with edit report."
)
```

#### Wave 4: Visual Asset Creation (Parallel With Text Editing)

**Timing:** Day 3–6 (overlaps with Wave 3)

```bash
sub-agent(
  task="Create visual asset package for '{title}' on {platform}. Follow visual specs from references/{platform}.md (cover requirements, aspect ratio, text readability)."
)
```

#### Wave 5: Final Approval + Scheduling

**Timing:** Day 6–7

Assemble final package. **Run the platform-specific quality check card from references/{platform}.md as the final gate.** Then hand off to `app-promotion`.

### Phase 4: Quality Gates (5 Gates + Platform Card)

Every piece of content must pass ALL 5 gates **plus** its platform-specific quality check card from `references/`.

#### Gate 1: Brand Voice Check
- Tone matches brand voice document ✓
- Platform-native expression (not copy-pasted from another platform) ✓
- No generic AI boilerplate phrases ✓

#### Gate 2: Factual Accuracy
- Statistics include sources ✓
- Product claims match current features/pricing ✓
- No exaggerated/misleading claims (especially critical for 小红书种草 and 抖音测评) ✓

#### Gate 3: Platform-Native Quality (**Load references/{platform}.md**)
- Passes the **platform-specific quality checklist** ✓
- Format/length/tone matches target platform's best practices ✓
- Interaction hooks designed and appropriate for the platform ✓

#### Gate 4: Visual Consistency
- Colors match brand palette exactly ✓
- Images meet platform specs (cover requirements, aspect ratio, text readability) ✓
- Accessibility where applicable ✓

#### Gate 5: Performance Readiness
- CTA clear, specific, actionable ✓
- Publishing time optimized for target platform's user activity patterns ✓
- SEO keywords in place (for search-heavy platforms like 小红书/知乎) ✓

**Gate summary report format:**

```
Quality Gate Report — {Title} ({Platform})
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Gate 1 — Brand Voice:      ✅ PASS / ❌ FAIL — {notes}
Gate 2 — Factual Accuracy: ✅ PASS / ❌ FAIL — {notes}
Gate 3 — Platform Quality:  ✅ PASS / ❌ FAIL — {notes} (per refs/{platform}.md)
Gate 4 — Visual Consistency: ✅ PASS / ❌ FAIL — {notes}
Gate 5 — Performance Ready: ✅ PASS / ❌ FAIL — {notes}

Overall: ✅ APPROVED / ❌ REVISIONS REQUIRED
Confidence Score: {X}/10
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### Phase 5: Publish & Distribute

Once content passes all gates, hand off to `app-promotion` for platform-specific distribution.

**Composition model:**

```
content-ops (this skill)            app-promotion (partner skill)
┌─────────────────────┐             ┌─────────────────────┐
│  PRODUCES            │ hands off  │  DISTRIBUTES         │
│  · Edited articles   │ ─────────→ │  · Platform publishing│
│  · Video scripts     │             │  · Scheduling        │
│  · Social captions   │             │  · Campaign mgmt     │
│  · Newsletters       │             │  · Community engage  │
│  · Visual assets     │             │  · Ad coordination   │
│  · Landing pages     │             │  · Analytics track   │
│  · Case studies      │             │  · A/B test execute  │
└─────────────────────┘             └─────────────────────┘
```

**Note:** For one-to-many format remuxing (take 1 article → output 10 formats), delegate to `content-factory` as a sub-task within a wave. Use `content-ops` for planning/quality control; use `content-factory` for the remix execution.

### Phase 6: Analytics & Optimization Loop

Weekly review with **platform-specific KPIs**, referencing each platform's success metrics from `references/{platform}.md`:

```bash
mindx graph query --cypher "
  MATCH (c:ContentPiece {brand_id:'$BRAND_ID'})
  WHERE c.published_at > datetime('now - 7 days') AND c.status = 'published'
  RETURN c.title, c.format, c.platform,
         c.metrics->>'engagement_rate' as eng_rate,
         c.metrics->>'conversions' as convos
  ORDER BY c.metrics->>'engagement_rate' DESC
"
```

**Output format:**

```
📊 Content Performance Report — {Brand} — Week {N}

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📕 小红书：
  本周发布 {N} 篇 | 总互动 {N} | 平均CES {N}
  🔥最佳：「{标题}」— CES {N} — 原因：{分析}
  💧最差：「{标题}」— CES {N} — 改进：{action}

📮 公众号：
  本周发布 {N} 篇 | 平均打开率 {N}% | 平均完读率 {N}%
  🔥最佳：「{标题}」— 打开率 {N}% — 原因：{分析}
  💧最差：「{标题}」— 打开率 {N}% — 改进：{action}

🎵 抖音：
  本周发布 {N} 条 | 平均完播率 {N}% | 平均互动率 {N}%
  🔥最佳：「{标题}」— 完播率 {N}% — 原因：{analysis}
  💧最差：「{标题}」— 完播率 {N}% — 改进：{action}

📺 B站 / 💡 知乎 / 📢 微博：(同理，各平台用其核心指标)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🔄 跨平台流转计划：
  本周最佳内容「{标题}」({平台}) → 下周改编为 {目标平台}格式
  （参考 references/{target_platform}.md 进行适配）

📋 下周策略：
  加大 {X平台} 投入 | 测试 {新内容类型} | 复盘 {失败案例}
```

#### A/B Testing Framework

| Test Variable        | What to Test                  | Duration | Success Metric (per platform)                        |
| -------------------- | ----------------------------- | -------- | ---------------------------------------------------- |
| **Headlines/Titles** | 2–3 variants                  | 7 days   | 小红书:CES diff / 公众号:CTR diff / 抖音:完播率 diff |
| **CTAs**             | Wording/placement             | 14 days  | Conversion lift ≥ 10%                                |
| **Formats**          | Article vs carousel vs video  | 30 days  | Engagement comparison across platforms               |
| **Publish Times**    | Same content, different times | 4 weeks  | Reach by time slot per platform                      |
| **Cover/Images**     | Different visual treatments   | 7 days   | Click-through / 完播率 impact                        |

#### Content Recycling Strategy

| Source             | Recycle Into | Method (load target platform ref)              |
| ------------------ | ------------ | ---------------------------------------------- |
| Top 公众号 article | 小红书笔记   | 提炼要点 → 加载 xiaohongshu.md → 按CES优化改写 |
| Top 公众号 article | 抖音脚本     | 提取Hook → 加载 douyin.md → 按完播率改写       |
| Top 小红书 note    | 公众号文章   | 扩展深度 → 加载 wechat-oa.md → 按双引擎改写    |
| Top B站视频        | 知乎回答     | 提取论点 → 加载 zhihu.md → 按5层结构改写       |
| Any top performer  | 微博碎片     | 金句切片 → 加载 weibo.md → 按时效性改写        |

**Recycling rule:** Always load the **target platform's reference file** when adapting content. Generate 3–5 derivative pieces per top performer before retiring the topic.

---

## Recurring Schedule Setup

```bash
# Daily: Content queue check (09:00 weekdays)
mindx schedule add \
  --agent "content-coordinator" \
  --content "Check today's content queue. Review due pieces, blocked tasks, approval items." \
  --cron "0 9 * * 1-5" \
  --session-id "$PROJECT_ID" --enabled true

# Weekly: Performance review (Friday 16:00)
mindx schedule add \
  --agent "content-analyst" \
  --content "Run weekly content performance review. Generate report with TOP/BOTTOM performers per platform, using each platform's KPIs from references/." \
  --cron "0 16 * * 5" \
  --session-id "$PROJECT_ID" --enabled true

# Weekly: Calendar planning (Friday 17:00)
mindx schedule add \
  --agent "content-planner" \
  --content "Plan next 4-week content calendar. Incorporate insights from review. Cross-reference platform reference files for format diversity." \
  --cron "0 17 * * 5" \
  --session-id "$PROJECT_ID" --enabled true

# Monthly: Full audit + strategy refresh (1st of month)
mindx schedule add \
  --agent "content-strategist" \
  --content "Run monthly content audit. Analyze 30-day trends, compare with competitors, recommend strategic adjustments. Review all platform reference files for any needed updates." \
  --cron "0 10 1 * *" \
  --session-id "$PROJECT_ID" --enabled true
```

> Every scheduled prompt must include: "When finished, use AgentTalk to report results to session '{session_id}'."

---

## Anti-Patterns

### 流程层面

- **Do NOT skip the content brief** — 没有brief就是瞎写。每篇至少要有：目标平台、核心信息、互动目标。Brief 必须指向对应的 `references/{platform}.md` 质量标准
- **Do NOT publish without passing all 5 gates + platform quality card** — 紧急内容可以做精简版3-pass（voice + facts + CTA），但不能零审核。平台质量卡来自 `references/{platform}.md`
- **Do NOT ignore platform differences** — 把公众号原文贴小红书 = 两头不讨好。每个平台必须原生适配，**加载对应 reference 文件**
- **Do NOT over-commit volume over quality** — 10篇 mediocre 不如 3篇精品。尤其小红书和知乎，质量 >> 数量
- **Do NOT silo content from distribution** — 和 `app-promotion` 协同，完美内容没人看 = 零值
- **Do NOT let the calendar go stale** — 每周根据数据和热点重新评估，替换过期选题
- **Do NOT skip the dependency chain** — Write → Edit + Design → Approve
- **Do NOT hardcode expert roles** — 使用 `find-experts` 动态发现所需能力，不要在技能中预设"专家名单"

### 内容质量层面（中国媒体生态特有）

- **Do NOT write AI-flavored Chinese** — "在当今...的大背景下""不得不说""总的来说" = 用户瞬间划走。要像真人说话
- **Do NOT chase every hot topic** — 每个热点都蹭 = 没有品牌辨识度。只在热点与你的专业领域有真实交集时才介入
- **Do NOT ignore comment sections** — 在中国社交媒体，评论区运营 = 内容运营的一半。发布后24小时的评论区互动直接影响算法推荐
- **Do NOT fake engagement** — 买赞/买粉/互刷在2026年不仅无效，还会被平台降权甚至封号
- **Do NOT violate platform rules** — 各平台对引流、营销、敏感词的规则不同且经常更新。发布前务必检查当前规则
- **Do NOT treat followers as numbers** — 中国社交媒体的核心是"信任关系"。每一次内容都是在存取或消耗信任
- **Do NOT copy domestic success blindly** — 别人的爆款方法论不一定适合你的领域和人设。理解底层逻辑后再适配
- **Do NOT produce without loading the platform reference** — 为小红书写内容却不加载 `references/xiaohongshu.md` = 盲写。每次生产前必须加载对应平台的写作规范
