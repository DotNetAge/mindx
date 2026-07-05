---
name: app-promotion
description: >
  Plan and execute internet product promotion campaigns — content pipeline,
  social media operations, data-driven optimization, and recurring campaign
  management across Chinese platforms (WeChat, Douyin, Xiaohongshu, etc.).
allowed-tools: bash sub-agent collect-results task-create task-update task-list find-experts
metadata:
  requires:
    bins:
      - python3
  name_zh: 产品推广
  name_zh-tw: 產品推廣
  description_zh: 规划和执行互联网产品推广活动——内容流水线、社媒运营、数据驱动优化、跨平台活动管理
  description_zh-tw: 規劃和執行互聯網產品推廣活動——內容流水線、社群媒體運營、數據驅動優化、跨平台活動管理
---

## Trigger Decision

Use this skill when:

- User asks to plan/run a promotion campaign for an app, product, or service
- User needs a content operation system (选题 → 创作 → 发布 → 数据复盘)
- User mentions social media marketing, user acquisition, growth hacking, or content strategy
- Task is ongoing/recurring (daily posts, weekly reports, monthly reviews)

**Do NOT use** for one-off content creation (use `content-factory` or `copywriting` directly).

## Domain Context

This skill operates in the **Chinese internet ecosystem**. Know these platforms:

### Language Handling for GraphRAG Queries

> **CRITICAL: This SKILL.md is written in English for token efficiency and global compatibility, but you MUST query GraphRAG in the language matching the stored content.**

**Language detection rule:**
- User speaks/writes in **Chinese** → Query `mindx memory query` and `mindx graph query` in **Chinese first**, then English as fallback
- User speaks/writes in **English** → Query in **English first**, then Chinese as fallback
- When storing via `mindx memory store` → Store in the **same language as the user's current working language**
- Graph node `properties` values → Match the language of stored data

**Bilingual query pattern (use when uncertain):**
```bash
# Query primary language first
mindx memory query "<搜索词 / search term>"
# If insufficient results, try secondary language
mindx memory query "<equivalent in other language>"
```

### GraphRAG Dual-Engine Architecture

> **This system has two storage layers that work together — you (the LLM) are the bridge between them via Cypher.**

**Layer 1: Graph — Entity Relationship Index**
- **What it stores:** Nodes (entities) and Edges (relationships)
- **Node structure:** `id`, `type` (entity type from definitions), `name`, `properties` (`description`, `confidence`, + any custom business fields you set)
- **Edge structure:** `type` (relationship), `source`, `target`, `predicate`, `properties`
- **How to write:** `mindx graph upsert-nodes --nodes '[...]'` and `mindx graph upsert-edges --edges '[...]'`
- **How to read:** `mindx graph query --cypher "<your dynamic Cypher>"` or `mindx graph exec --cypher "..."`

**Layer 2: NativeRAG — Semantic Overview Index**
- **What it stores:** Chunks of semantic content with vector embeddings
- **Structure:** content, title, tags, positions, doc_id
- **How to write:** `mindx memory store --content "..." --title "..."`
- **How to read:** `mindx memory query "<search terms>"` (vector similarity search)

**The link:** Both layers share `doc_id` — a Graph node can trace back to its source chunks in NativeRAG, and vice versa.

**Your superpower as LLM:** Humans write fixed hybrid queries. You write **dynamic Cypher** that traverses entity relationships in the Graph, then jumps to NativeRAG for full context via doc_id. This is what makes this architecture flexible.

**When to use which:**
| Need                                   | Command                                               |
| -------------------------------------- | ----------------------------------------------------- |
| Find relevant knowledge/documents      | `mindx memory query` (semantic search)                |
| Store new insights/learnings           | `mindx memory store` (vector index)                   |
| Build structured business state        | `mindx graph upsert-nodes/edges` (entity graph)       |
| Query relationships between entities   | `mindx graph query --cypher "..."` (Cypher traversal) |
| Update business state                  | `mindx graph exec --cypher "SET ..."` (mutation)      |
| Cross-reference: entity → full context | Graph node → get source docs → `memory query`         |

| Platform    | Content Type               | Optimal Posting Window        | Key Metrics                                      |
| ----------- | -------------------------- | ----------------------------- | ------------------------------------------------ |
| WeChat OA   | Long-form article          | 8:00-9:00, 20:00-22:00        | Read rate, share rate, new followers             |
| Douyin      | Short video (15-60s)       | 12:00-13:00, 18:00-21:00      | Views, completion rate, comments, shares         |
| Xiaohongshu | Image + text / short video | 10:00-12:00, 19:00-22:00      | Likes, saves, comments, CTR                      |
| Weibo       | Short text + image         | 9:00-11:00, 17:00-19:00       | Reposts, comments, reach                         |
| Bilibili    | Mid/long video (3-15min)   | 17:00-22:00 (weekend all day) | Play count, bullet comments (danmaku), favorites |

## Workflow

### Phase 1: Diagnose & Define Campaign

Talk to the user before planning anything. Extract concrete parameters:

| Ask                                    | Why                | Example Answer                 |
| -------------------------------------- | ------------------ | ------------------------------ |
| "What product/app are we promoting?"   | Scope              | "Our fitness app FitTrack"     |
| "What's the primary goal?"             | Success definition | "Get 10k downloads in 30 days" |
| "Which platforms?"                     | Channel selection  | "Douyin + Xiaohongshu mainly"  |
| "What's the budget (time/money)?"      | Constraints        | "No ad budget, organic only"   |
| "Who is the target audience?"          | Content direction  | "25-35 urban professionals"    |
| "What content formats can we produce?" | Capability check   | "Short videos and articles"    |

From answers, define:

```
Campaign spec:
  Product: <name>
  Goal: <measurable target>
  Platforms: [<platforms>]
  Audience: <persona>
  Formats: [<types>]
  Timeline: <start> to <end>
  Budget: <constraints>
```

### Phase 2: Design Content Pipeline

Build a **weekly content rhythm**. A typical week looks like:

```
Monday:   选题会（Topic meeting）— 分析上周数据 + 确定本周主题
Tuesday:  内容创作日（Creation）— 图文/视频制作
Wednesday: 审核与排期（Review & Schedule）— 质检 + 设定发布时间
Thursday: 发布日 A（Publish A）— 高峰时段发布
Friday:   发布日 B（Publish B）— 次高峰发布 + 互动维护
Saturday: 轻量内容（Light content）— 用户生成内容/互动帖
Sunday:   周报与下周规划（Weekly report + Next week plan）
```

For each piece of content, define a **content card**:

```
Content Card:
  Title: <标题>
  Platform: <平台>
  Format: <图文/短视频/直播>
  Topic: <所属话题/系列>
  Target KPI: <预期指标范围>
  Publish time: <具体时间窗口>
  Status: pending → creating → reviewing → scheduled → published → analyzed
```

### Phase 3: Assemble Team

Use `find-experts` skill to discover and delegate to specialists.

**Typical team for app promotion:**

| Role             | Responsibility                        | When Needed                     |
| ---------------- | ------------------------------------- | ------------------------------- |
| `content-writer` | Article writing, script drafting      | Every creation cycle            |
| `video-editor`   | Short video editing, captioning       | If Douyin/Bilibili is a channel |
| `data-analyst`   | Weekly metrics analysis, A/B insight  | After each publish cycle        |
| `copywriter`     | Headlines, CTAs, ad copy              | For high-conversion touchpoints |
| `designer`       | Cover images, infographics, templates | Visual-heavy platforms          |

**Team setup pattern** (using find-experts Mode 2):

```
mindx agent list --json          # Discover available experts

# Then use find-experts workflow:
team-create(
  team_name="promo-{campaign-name}",
  leader="coordinator",
  members=["content-writer", "video-editor", "data-analyst"],
  tasks=[
    "Write weekly content calendar with topics and angles",
    "Create and edit 5 short videos for Douyin campaign",
    "Analyze weekly metrics and produce optimization report"
  ]
)
```

### Phase 4: Execute — Create & Track Tasks

For each content piece, create a task for tracking:

```bash
# Example: Track a Douyin video production task
# (via TaskCreate tool, not bash — this is for LLM's own tracking)
TaskCreate(
  subject="抖音短视频：FitTrack 减肥误区合集",
  description="制作一条 45 秒的抖音短视频，主题为「5个常见减肥误区」，含口播脚本+封面图+字幕",
  active_form="正在制作抖音减肥误区视频"
)
→ 记录 task_id 用于后续状态跟踪
```

**Execution modes by complexity:**

| Scenario                               | Mode               | Tools Used                                                |
| -------------------------------------- | ------------------ | --------------------------------------------------------- |
| Single post, do it yourself            | Direct execution   | Write/edit/publish directly                               |
| Need specialist content (e.g., script) | Single SubAgent    | `sub-agent(content-writer, task=...)` → `collect-results` |
| Full weekly cycle with team            | Team orchestration | `find-experts` Mode 2 + `task-create/update`              |

### Phase 5: Analyze & Optimize (The Loop)

After each publish cycle (daily or weekly), run the analysis loop:

```bash
# Gather platform data (example: query graph for campaign progress)
mindx graph query --cypher "
  MATCH (c:Campaign {id: '$CAMPAIGN_ID'})-[:HAS_CONTENT]->(co:Content)
  WHERE co.published_at > datetime('now - 7 days')
  RETURN co.platform, co.format,
         co.metrics->>'views' as views,
         co.metrics->>'likes' as likes,
         co.metrics->>'shares' as shares,
         co.status
  ORDER BY co.published_at DESC
"
```

**Analysis framework — ask these questions every cycle:**

| Question                      | Good Signal                   | Bad Signal        | Action                                |
| ----------------------------- | ----------------------------- | ----------------- | ------------------------------------- |
| Which content performed best? | Top 20% by engagement         | Bottom 20%        | Double down on winning topics/formats |
| Which platform has best ROI?  | Highest conversion per effort | Lowest            | Reallocate effort                     |
| Is the audience growing?      | Follower trend up             | Flat or declining | Check content-audience fit            |
| Are we hitting the goal?      | On track to target            | Behind            | Intensify or pivot                    |

**Output format for weekly report:**

```
📊 推广周报 — {Campaign Name}（第 N 周）

🎯 目标进度：{current}/{target}（{percentage}%）

各平台表现：
  抖音：发布 {N} 条 | 总播放 {views} | 平均完播 {rate}% | 涨粉 {n}
  小红书：发布 {N} 条 | 总阅读 {reads} | 收藏/点赞比 {ratio} | 涨粉 {n}

Top 3 内容：
  1. 「{title}」— {platform} | {key_metric} | ✅ 原因：{why it worked}
  2. ...
  3. ...

需关注：
  ⚠️ {issue} — 建议：{action}

下周计划：
  · {topic-1}
  · {topic-2}
  · 调整：{what changes based on data}
```

### Phase 6: Recurring Schedule (Optional)

For long-running campaigns, set up automated tasks:

```bash
# Weekly data analysis reminder
mindx schedule add \
  --agent "data-analyst" \
  --content "Analyze last 7 days of campaign data for {campaign}. Output weekly report following the standard template." \
  --cron "0 10 * * 1" \           # Every Monday 10:00
  --session-id "$CAMPAIGN_ID" \
  --enabled true

# Daily content publishing (if fully automated)
mindx schedule add \
  --agent "content-publisher" \
  --content "Publish today's scheduled content. Report back which items were published and any issues." \
  --cron "0 9 * * 2-6" \          # Tue-Sat 09:00
  --session-id "$CAMPAIGN_ID" \
  --enabled true
```

> Each scheduled task's prompt must include: "When you finish, use AgentTalk to report to project coordinator in session '{session_id}'."

## Gotchas

- **Platform algorithm changes are silent and instant.** A content format that performed well yesterday may get zero reach today. Never guarantee a specific outcome — describe what worked recently and note that platforms change.
- **Content compliance is platform-specific.** WeChat, Douyin, and Xiaohongshu each have different rules on external links, calls-to-action, and commercial content. Always check platform content policies before publishing.
- **Brand voice is harder to maintain at scale.** When generating content across multiple platforms and 10+ pieces per week, review for tone consistency. Flag any content that deviates from the brand guidelines.
- **Data-driven optimization is only as good as the data source.** Platform analytics APIs can have 24-48 hour latency. Decisions based on "this week's data" may actually be "last week's data."

## Anti-Patterns

- Do not post identical content across all platforms — each platform has different format norms and audience expectations
- Do not chase viral trends that don't align with the product's target audience
- Do not ignore negative comments/feedback — address them or document the pattern
- Do not measure vanity metrics (raw views) without conversion context — always tie back to the primary goal
- Do not skip the analysis phase — publishing without reviewing data is operating blind
- Do not over-commit to posting frequency — consistent 3x/week beats sporadic 7x then silence
- Do not use hard-sell language on Xiaohongshu/Douyin — native, value-first content outperforms ads

## Quick Reference: Content Types by Platform

| Platform    | Best Performing Content                                                            | Avoid                                                  |
| ----------- | ---------------------------------------------------------------------------------- | ------------------------------------------------------ |
| WeChat OA   | Deep guides (2000-3000 words), industry insights, how-to series                    | Pure ads, clickbait without substance                  |
| Douyin      | Problem-solution hooks (first 3 sec critical), behind-the-scenes, tips compilation | Long intros, no hook, low-energy delivery              |
| Xiaohongshu | Personal experience reviews, aesthetic flat-lays, "before/after", actionable lists | Overt selling, generic stock photos, no personal voice |
| Weibo       | Timely hot takes, event live-blogging, conversation starters                       | Evergreen content (dies in feed instantly)             |
