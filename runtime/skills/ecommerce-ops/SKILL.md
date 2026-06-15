---
name: ecommerce-ops
description: >
  End-to-end cross-border e-commerce operations — product research, listing optimization,
  multi-platform publishing, advertising management, order operations, customer service,
  inventory control, and financial reconciliation across Amazon, Shopify, TikTok Shop,
  Temu, Shein, and other platforms.
allowed-tools: bash sub-agent collect-results task-create task-update task-get task-list team-create team-list team-get-tasks find-experts
metadata:
  name_zh: 跨境电商运营
  name_zh-tw: 跨境電商運營
  description_zh: 端到端跨境电商运营——选品、Listing优化、多平台发布、广告管理、订单处理、客服、库存管控、财务对账
  description_zh-tw: 端到端跨境電商運營——選品、Listing優化、多平台發布、廣告管理、訂單處理、客服、庫存管控、財務對帳
---

## Trigger Decision

Use this skill when:

- User runs a cross-border e-commerce business (Amazon, Shopify, TikTok Shop, Temu, Shein, etc.)
- User needs daily store operations automation (pricing, listing, inventory, ads)
- User asks about e-commerce analytics, GMV tracking, or profit calculation
- Task involves multi-platform synchronization or cross-border logistics coordination

**Do NOT use** for single-product content creation (use `app-promotion` or `content-factory`).
**Do NOT confuse with** `app-promotion` — that skill handles brand/content marketing; this skill handles **transactional store operations** centered on GMV, conversion rate, and fulfillment.

## Domain Knowledge Base

### Language Handling for GraphRAG Queries

> **CRITICAL: This SKILL.md is written in English for token efficiency and global compatibility, but you MUST query GraphRAG in the language matching the stored content.**

**Language detection rule:**
- User speaks/writes in **Chinese** → Query `mindx memory query` and `mindx graph query` in **Chinese first**, then English as fallback
- User speaks/writes in **English** → Query in **English first**, then Chinese as fallback
- When storing via `mindx memory store` → Store in the **same language as the user's current working language**
- Graph node `properties` values → Match the language of stored data
- Cypher string literals (e.g., `p.name = 'Amazon'`) → Use the language stored in node properties

**Bilingual query pattern (use when uncertain):**
```bash
# Primary language query
mindx memory query "<搜索词 / search term>"
# Fallback if insufficient results
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
| Need | Command |
|------|---------|
| Find relevant knowledge/documents | `mindx memory query` (semantic search) |
| Store new insights/learnings | `mindx memory store` (vector index) |
| Build structured business state (stores, products, campaigns) | `mindx graph upsert-nodes/edges` (entity graph) |
| Query relationships between entities | `mindx graph query --cypher "..."` (Cypher traversal) |
| Update business state | `mindx graph exec --cypher "SET ..."` (mutation) |
| Cross-reference: entity → full context | Graph node → get source docs → `memory query` |

Before any operation, load domain knowledge from GraphRAG:

```bash
# Load platform-specific rules and best practices
mindx memory query "Amazon listing optimization best practices 2026"
mindx memory query "TikTok Shop algorithm ranking factors"
mindx memory query "cross-border e-commerce tax compliance"
mindx memory query "ecommerce advertising ROAS benchmarks"

# Query knowledge graph for stored operational patterns
mindx graph query --cypher "
  MATCH (p:Platform)-[:HAS_RULE]->(r:Rule)
  WHERE p.name IN ['Amazon', 'Shopify', 'TikTokShop']
  RETURN p.name, r.category, r.content
  LIMIT 20
"
```

Store new learnings back after each cycle:
```bash
mindx memory store --content "<what we learned>" --title "Ecommerce Insight: <topic>" --source "ecommerce-ops-cycle"
```

## Platform Profiles

| Platform          | Business Model       | Key Metrics                            | Listing Format               | Ad System                        | Fulfillment              |
| ----------------- | -------------------- | -------------------------------------- | ---------------------------- | -------------------------------- | ------------------------ |
| **Amazon**        | Marketplace          | BSR, conversion rate, TOS              | A+ content + images + video  | Sponsored Products/Brand/Display | FBA / FBM                |
| **Shopify**       | DTC store            | AOV, LTV, CAC, repeat rate             | Full custom storefront       | FB/Google/Meta ads + email       | Self / 3PL               |
| **TikTok Shop**   | Social commerce      | GPM, completion rate, share rate       | Short video + live           | Promote product / Shopping Ads   | Fulfilled by seller      |
| **Temu**          | Discount marketplace | Price competitiveness, review velocity | Basic listing (price-driven) | Platform-managed                 | Temu logistics           |
| **Shein**         | Fast fashion         | Trend alignment, return rate           | Image-heavy + size matrix    | Limited                          | Shein logistics          |
| **Shopee/Lazada** | SEA marketplace      | Chat response time, shop score         | Mobile-first listing         | In-app ads                       | Self / logistics partner |

## Workflow

### Phase 1: Store Diagnosis & Goal Setting

Understand the current state before planning anything:

```bash
# Pull current store metrics into knowledge graph for tracking
STORE_ID=$(mindx utils uuid)

mindx graph upsert-nodes --nodes '[{
  "id":"'"$STORE_ID"'",
  "labels":["EcommerceStore"],
  "properties":{
    "platform":"<platform>",
    "store_name":"<name>",
    "status":"active",
    "currency":"<currency>",
    "timezone":"<tz>",
    "started_at":"<date>"
  }
}]'
```

Ask the user these diagnostic questions:

| Ask                                            | Why                | Good Answer Example                            |
| ---------------------------------------------- | ------------------ | ---------------------------------------------- |
| "Which platforms are you active on?"           | Scope              | "Amazon US + TikTok Shop US"                   |
| "What's your monthly GMV target?"              | Success definition | "$50k/month within 90 days"                    |
| "What's your current GMV and main bottleneck?" | Baseline           | "$12k/mo, conversion rate stuck at 1.2%"       |
| "What's your product category and ASP?"        | Strategy input     | "Home fitness equipment, $30-80 ASP"           |
| "What's your ad budget range?"                 | Constraints        | "$3k-5k/month total across platforms"          |
| "Who handles fulfillment? FBA? 3PL? Self?"     | Ops model          | "Amazon FBA, Shopify via 3PL"                  |
| "Any compliance issues? (tax, certification)"  | Risk check         | "FCC needed for electronics, no tax nexus yet" |

Output: `Store Profile` document — store as graph node properties.

### Phase 2: Product Research & Selection

For ongoing stores, run a weekly/bi-weekly research cycle:

**Step 2A: Market Intelligence**

```bash
# Use GraphRAG to find trending products in your category
mindx memory query "trending products <category> <region> 2026 Q2"

# Store findings as research nodes
RESEARCH_ID=$(mindx utils uuid)
mindx graph upsert-nodes --nodes '[{
  "id":"'"$RESEARCH_ID"'",
  "labels":["ProductResearch"],
  "properties":{
    "cycle":"weekly-<N>",
    "category":"<category>",
    "date":"<today>",
    "status":"analyzing"
  }
}]'
```

Research dimensions:

| Dimension    | Data Source                                       | What to Look For          |
| ------------ | ------------------------------------------------- | ------------------------- |
| Demand trend | Platform search volume, Google Trends             | Rising vs declining       |
| Competition  | Number of sellers, review count distribution      | Blue ocean vs red ocean   |
| Pricing      | Price range analysis, margin calculator           | Can you make 25%+ margin? |
| Seasonality  | Historical sales pattern                          | Is this a seasonal spike? |
| Compliance   | Certification requirements, restricted categories | Any showstoppers?         |

**Step 2B: Product Scoring Matrix**

Score each candidate on:

| Factor                    | Weight | Score 1-5                            |
| ------------------------- | ------ | ------------------------------------ |
| Demand velocity           | High   | Search growth rate                   |
| Competition gap           | High   | Low seller count + high demand       |
| Profit margin             | High   | After all fees (FBA/refunds/ads/tax) |
| Sourcing feasibility      | Medium | Can you reliably source?             |
| Differentiation potential | Medium | Room for branding/bundling?          |
| **Weighted Total**        |        | **Cut-off: ≥3.5 to proceed**         |

### Phase 3: Listing Creation & Optimization

This is where domain expertise matters most. Each platform has different requirements:

#### 3A: Amazon Listing

```
Listing Checklist:
  Title: Brand + Model + Key Features + Size/Color + Compatibility (≤200 chars)
  Bullet points: 5 bullets, lead with benefit not feature
  Backend keywords: 249 bytes max, comma-separated, no brand repetition
  A+ Content (Brand Registry required): Rich HTML comparison charts
  Images: Main + 6-8 lifestyle/infographic/size chart
  Video: 30-60 sec demo or unboxing
  Price: Position against top 3 competitors (±15%)
  Category: Most specific leaf node possible
```

#### 3B: TikTok Shop Listing

```
Listing Checklist:
  Product title: Concise, keyword-rich, mobile-friendly
  Cover video: First 3 seconds = hook (problem → solution)
  Detail images: Before/after, size guide, usage scenarios
  Price: Competitive with platform average (check GPM benchmarks)
  Inventory: Set safety stock (account for viral spike risk)
  Shipping template: Accurate processing time
```

#### 3C: Multi-Platform Sync Rules

```
DO sync:
  · Core product data (SKU, specs, price baseline)
  · Brand assets (logo, banner, about page)
  · Customer service templates (FAQ, return policy)

DON'T blindly copy:
  · Titles (each platform has different SEO rules)
  · Descriptions (tone and length vary)
  · Pricing (factor in each platform's fee structure)
  · Images (aspect ratios differ)
```

### Phase 4: Advertising & Traffic Management

**Daily ad ops rhythm:**

```
Morning (09:00):
  └─ Check yesterday's performance per campaign
     mindx graph query --cypher "
       MATCH (c:Campaign {store_id:'$STORE_ID'})
       WHERE c.yesterday = true
       RETURN c.name, c.spend, c.impressions, c.clicks,
              c.orders, c.revenue, c.acos
       ORDER BY c.spend DESC
     "
  └─ Identify underperformers (ACOS > target) and pausing candidates
  └─ Identify winners (ROAS > 2x target) → consider budget increase

Afternoon (14:00):
  └─ Adjust bids on high-intent keywords
  └─ Add negative keywords from search term report
  └─ Launch/test new creative variations

Evening (19:00):
  └─ Prime-time monitoring (especially TikTok Shop live sessions)
  └─ Respond to urgent customer messages
  └─ Log today's actions to graph for tomorrow's baseline
```

**Ad performance thresholds (by platform):**

| Metric          | Amazon             | TikTok Shop    | Shopify (Meta) |
| --------------- | ------------------ | -------------- | -------------- |
| Target ACOS     | 15-25%             | N/A (use ROAS) | N/A (use ROAS) |
| Target ROAS     | 3-4x               | 2-3x           | 2.5-4x         |
| CTR benchmark   | 0.3-0.5%           | 1-3%           | 0.8-1.5%       |
| CPC warning     | >$2.00 (most cats) | >$0.50         | >$3.00         |
| Daily min spend | $30/campaign       | $20/campaign   | $50/campaign   |

### Phase 5: Order Operations & Fulfillment

**Order processing workflow:**

```
New Order → Verify stock → Print label → Hand to carrier → Upload tracking → Confirm to customer
                                                                    ↓
                                              Exception Branch: Out of stock / Address issue / Return
                                                                    ↓
                                                    Sub-agent(customer-service) handles resolution
```

**Daily order checklist:**

```bash
# Query pending orders
mindx graph query --cypher "
  MATCH (o:Order {store_id:'$STORE_ID', status:'pending'})
  RETURN o.order_id, o.sku, o.quantity, o.customer, o.shipping_method
  ORDER BY o.created_at ASC
  LIMIT 50
"

# Check inventory alerts
mindx graph query --cypher "
  MATCH (p:Product {store_id:'$STORE_ID'})
  WHERE p.stock < p.reorder_point
  RETURN p.sku, p.title, p.stock, p.reorder_point,
         p.days_of_supply as dos
  ORDER BY dos ASC
"
```

### Phase 6: Customer Service

**Response SLAs by channel:**

| Channel                         | Target Response Time |   Resolution Target | Escalation Trigger                     |
| ------------------------------- | -------------------: | ------------------: | -------------------------------------- |
| Amazon Buyer Messages           |            < 2 hours |          < 24 hours | Negative sentiment / A-to-Z claim      |
| TikTok Shop chat                |         < 30 minutes |           < 4 hours | Refund request / fake review suspicion |
| Shopify email                   |            < 4 hours |          < 24 hours | Chargeback / legal threat              |
| Platform cases (return/request) |             < 1 hour | Per platform policy | Any case affecting account health      |

**Common scenario handling:**

| Scenario             | First Action                      | Resolution Path                                |
| -------------------- | --------------------------------- | ---------------------------------------------- |
| "Item not arrived"   | Check tracking → confirm location | If lost: immediate replacement or refund       |
| "Wrong item sent"    | Apologize → verify SKU mismatch   | Send correct + prepaid return label            |
| "Not as described"   | Review listing accuracy           | If listing misleading: fix it + partial refund |
| "Want discount"      | Check purchase history            | LTV-based decision (first-time vs repeat)      |
| Fake/negative review | Document evidence                 | Report to platform + public response draft     |

### Phase 7: Analytics & Financial Reconciliation

**Weekly financial close:**

```
Revenue:
  Platform gross sales
  − Platform fees (commission + FBA/referral/shipping)
  − Advertising spend
  − Returns & refunds
  − Cost of goods sold (COGS)
  − Storage/pick&pack fees
  ──────────────────────
  = Net Profit
  ÷ Total units sold
  = Profit per unit
```

**Key ratios to track:**

| Ratio                     | Formula                                    | Healthy Range        | Action if Below                          |
| ------------------------- | ------------------------------------------ | -------------------- | ---------------------------------------- |
| **Net Margin**            | Net Profit / Revenue                       | 15-30%               | Review COGS or pricing                   |
| **Ad Efficiency (TACOS)** | Total Ad Spend / Total Revenue             | 8-15%                | Pause low-performing campaigns           |
| **Return Rate**           | Returns / Units Sold                       | < 5% (varies by cat) | Investigate quality/description mismatch |
| **LTV:CAC**               | Lifetime Value / Customer Acquisition Cost | > 3:1                | Improve retention or reduce ad cost      |
| **Inventory Turnover**    | COGS / Average Inventory                   | 4-12x/year           | Clear slow movers                        |

**Store health dashboard (graph query):**

```bash
mindx graph query --cypher "
  MATCH (s:EcommerceStore {id:'$STORE_ID'})-[:HAS_METRIC]->(m:Metric)
  WHERE m.period = 'week-<N>'
  RETURN m.gmv, m.orders, m.units, m.avg_order_value,
         m.ad_spend, m.tacos, m.return_rate, m.net_margin,
         m.top_sku, m.worst_sku
"
```

### Phase 8: Recurring Schedule Setup

For long-running stores, automate routine tasks:

```bash
# Daily morning ops brief (09:00 every day)
mindx schedule add \
  --agent "ops-coordinator" \
  --content "Run daily store ops for store {store_id}. Check overnight orders, ad performance, inventory levels, and urgent CS tickets. Output structured morning brief following the standard template." \
  --cron "0 9 * * *" \
  --session-id "$STORE_ID" \
  --enabled true

# Weekly product research (Monday 10:00)
mindx schedule add \
  --agent "product-researcher" \
  --content "Execute weekly product research cycle for {store_name}. Analyze demand trends, competition gaps, and scoring matrix. Recommend 3-5 new product candidates." \
  --cron "0 10 * * 1" \
  --session-id "$STORE_ID" \
  --enabled true

# Weekly financial close (Friday 17:00)
mindx schedule add \
  --agent "finance-analyst" \
  --content "Generate weekly financial report for {store_name}. Reconcile revenue, calculate net margin per SKU, flag anomalies. Output P&L summary." \
  --cron "0 17 * * 5" \
  --session-id "$STORE_ID" \
  --enabled true
```

> Every scheduled prompt must include: "When finished, use AgentTalk to report results to session '{session_id}'."

## Team Composition

| Role                 | Responsibility                                                 | When Needed            |
| -------------------- | -------------------------------------------------------------- | ---------------------- |
| `product-researcher` | Market analysis, competitor monitoring, scoring                | Weekly research cycles |
| `listing-specialist` | Multi-platform listing creation, A+ content, SEO optimization  | New product launches   |
| `ad-manager`         | Campaign setup, bid management, creative testing               | Daily ad ops           |
| `customer-service`   | Message handling, case resolution, review management           | Ongoing (SLA-driven)   |
| `data-analyst`       | Financial reports, KPI dashboards, anomaly detection           | Weekly closes          |
| `ops-coordinator`    | Order processing, inventory management, logistics coordination | Daily operations       |

**Team setup via find-experts Mode 2:**
```
team-create(
  team_name="ecom-{store-name}",
  leader="store-manager",
  members=[researcher, lister, ad-cs, cs, analyst, coordinator],
  tasks=[
    "Weekly product research and scoring",
    "New product listing creation for {N} SKUs",
    "Daily ad performance monitoring and optimization",
    "Customer service SLA compliance",
    "Weekly P&L and health report"
  ]
)
```

## Anti-Patterns

- Do NOT set and forget ad campaigns — daily optimization is non-negotiable on Amazon/TikTok
- Do NOT ignore negative reviews — respond within 24 hours, they directly affect conversion rate
- Do NOT let inventory drop below 14 days of supply without a replenishment plan — stockouts kill rankings
- Do NOT copy listings verbatim across platforms — each has unique SEO algorithms and format norms
- Do NOT chase vanity metrics (total views, follower count) without tying to revenue — focus on conversion and margin
- Do NOT skip the weekly financial close — small discrepancies compound into large losses
- Do NOT use hard-sell language in TikTok content — native, entertaining content outperforms direct selling by 3-5x
- Do NOT neglect tax compliance across jurisdictions — VAT/GST obligations differ wildly by country/platform
- Do NOT launch more than 10 new SKUs simultaneously — test and iterate before scaling
