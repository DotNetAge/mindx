---
name: customer-success
description: >
  Manage the full customer lifecycle for SaaS and subscription businesses — onboarding,
  health scoring, proactive engagement, QBRs, renewal/upsell orchestration, churn prevention,
  and expansion revenue growth. Turns passive accounts into engaged, growing partnerships.
allowed-tools: bash sub-agent collect-results task-create task-update task-get task-list team-create team-list team-get-tasks find-experts
metadata:
  name_zh: 客户经营
  name_zh-tw: 客戶經營
  description_zh: 管理 SaaS 和订阅制业务的完整客户生命周期——入驻引导、健康评分、主动触达、QBR、续约/扩购编排、流失预防
  description_zh-tw: 管理 SaaS 和訂閱制業務的完整客戶生命週期——入駐引導、健康評分、主動觸達、QBR、續約/擴購編排、流失預防
---

## Trigger Decision

Use this skill when:

- User manages SaaS/subscription customers and needs lifecycle management
- User needs customer health monitoring, renewal forecasting, or churn prevention
- Task involves onboarding new customers, running QBRs (Quarterly Business Reviews), or executing expansion plays
- Customer success manager (CSM) workload exceeds capacity — need systematic coverage

**Do NOT use** for one-off support tickets (use `customer-support` or handle directly).
**Do NOT confuse with** `ecommerce-ops` — that handles transactional e-commerce; this manages **relationship-driven subscription lifecycles**.

## Domain Knowledge Base

### Language Handling for GraphRAG Queries

> **CRITICAL: This SKILL.md is written in English for token efficiency and global compatibility, but you MUST query GraphRAG in the language matching the stored content.**

**Language detection rule:**
- User speaks/writes in **Chinese** → Query `mindx memory query` and `mindx graph query` in **Chinese first**, then English as fallback
- User speaks/writes in **English** → Query in **English first**, then Chinese as fallback
- When storing via `mindx memory store` → Store in the **same language as the user's current working language**
- Graph node `properties` values → Match the language of stored data
- Cypher string literals → Use the language stored in node properties

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
| Build structured business state (customers, accounts, health scores) | `mindx graph upsert-nodes/edges` (entity graph) |
| Query relationships between entities | `mindx graph query --cypher "..."` (Cypher traversal) |
| Update business state | `mindx graph exec --cypher "SET ..."` (mutation) |
| Cross-reference: entity → full context | Graph node → get source docs → `memory query` |

```bash
# Load CSM best practices and playbook patterns
mindx memory query "customer success health scoring methodology"
mindx memory query "SaaS churn prevention strategies 2026"
mindx memory query "QBR template best practices enterprise"
mindx memory query "expansion revenue playbooks net dollar retention"

# Query knowledge graph for stored customer data and patterns
mindx graph query --cypher "
  MATCH (c:Customer)-[:HAS_HEALTH]->(h:HealthScore)
  WHERE h.score < 60
  RETURN c.name, c.tier, c.arr, h.score, h.risk_factors
  ORDER BY h.score ASC
  LIMIT 20
"
```

Store insights after each interaction:
```bash
mindx memory store --content "<interaction summary + insight>" \
  --title "CSM Note: <customer> - <date>" \
  --source "customer-success-cycle"
```

## Workflow

### Phase 1: Account Portfolio Setup

Build the customer portfolio in the graph:

```bash
PORTFOLIO_ID=$(mindx utils uuid)

mindx graph upsert-nodes --nodes '[{
  "id":"'"$PORTFOLIO_ID"'",
  "labels":["CSMPortfolio"],
  "properties":{
    "csm_name":"<your-name>",
    "total_accounts":0,
    "total_arr":0,
    "updated_at":"<now>"
  }
}]'
```

For each managed customer, create an account node:

```bash
# Per-customer account setup
CUSTOMER_ID=$(mindx utils uuid)
mindx graph upsert-nodes --nodes '[{
  "id":"'"$CUSTOMER_ID"'",
  "labels":["Customer","Account"],
  "properties":{
    "company":"<company-name>",
    "tier":"enterprise|mid-market|smb|growth",
    "arr":<annual-revenue>,
    "contract_start":"<date>",
    "contract_end":"<renewal-date>",
    "product_tier":"<plan-name>",
    "seat_count":<n>,
    "industry":"<sector>",
    "primary_contact":"<name>",
    "status":"active|at-risk|churned",
    "health_score":0,
    "last_touch":null,
    "nrr":1.0,
    "notes":""
  }
}]'
mindx graph upsert-edges --edges '[{
  "from_node_id":"'"$PORTFOLIO_ID"'",
  "to_node_id":"'"$CUSTOMER_ID"'",
  "type":"MANAGES"
}]'
```

### Phase 2: Health Scoring Model

Define and continuously calculate a **multi-dimensional health score** (0-100) for each customer:

| Dimension              | Weight | Data Sources                                               | Score Logic                                              |
| ---------------------- | ------ | ---------------------------------------------------------- | -------------------------------------------------------- |
| **Product Adoption**   | 30%    | Login frequency, feature usage depth, seat utilization     | Active users / purchased seats × feature breadth score   |
| **Engagement Quality** | 20%    | Meeting attendance, response rate, NPS/promoter status     | Weighted composite of engagement signals                 |
| **Support Sentiment**  | 15%    | Ticket volume trend, CSAT scores, escalation rate          | Inverse of ticket severity + positive sentiment bonus    |
| **Value Realization**  | 20%    | Goal achievement vs stated outcomes, time-to-value         | How much of promised value is the customer experiencing? |
| **Contract Risk**      | 15%    | Days to renewal, executive sponsor changes, budget signals | Time decay + risk flag penalties                         |

**Health tiers:**

```
90-100  🟢 Healthy      → Focus on expansion and advocacy
70-89   🟡 Engaged      → Maintain relationship, identify growth opportunities
50-69   🟠 At-Risk      → Immediate intervention required, create save plan
0-49    🔴 Critical     → Escalate to leadership, daily touch points
```

**Automated health check (daily):**
```bash
mindx schedule add \
  --agent "health-monitor" \
  --content "Run daily health score recalculation for all active customers. Flag any score drop >10 points from previous day. Output: tier changes, at-risk alerts, recommended actions." \
  --cron "0 8 * * *" \           # Daily 08:00
  --session-id "$PORTFOLIO_ID" \
  --enabled true
```

### Phase 3: Onboarding — The First 90 Days

Onboarding is the single biggest predictor of long-term retention. Structure it as:

```
Week 1:  Foundation (Days 1-7)
  Day 1:   Welcome call + stakeholder mapping
  Day 2-3: Technical setup + access provisioning
  Day 4-5: Admin training (key user training)
  Day 6-7: Initial data import + configuration validation
  
Week 2:  Activation (Days 8-14)
  Day 8-10: End-user rollout + adoption tracking begins
  Day 11-12: First use-case walkthrough with team
  Day 13-14: Address early friction points + quick wins

Week 3:  Value Discovery (Days 15-21)
  Identify 3 success metrics aligned with customer's goals
  Set baseline measurements
  Create first value milestone plan

Week 4:  Handoff & First Review (Days 22-30)
  Transition from high-touch to cadenced engagement
  30-day review meeting (mini-QBR)
  Document learnings → store in GraphRAG

Month 2-3: Steady State Building
  Bi-weekly check-ins
  Expand to secondary use cases
  Build internal champion network

Day 90:  Full Onboarding Complete
  Transition to standard engagement model
  Measure: activation rate, time-to-value, initial NPS
```

**Create onboarding tasks via TaskCreate with dependencies:**
```
TaskCreate(subject="Welcome call: {company}", description="Stakeholder mapping + goal setting")
TaskCreate(subject="Technical setup: {company}", description="Provision access + validate")
TaskUpdate(tech_task, addBlockedBy=[welcome_task])  // tech after welcome
TaskCreate(subject="30-day review: {company}", ...)
TaskUpdate(review_task, addBlockedBy=[tech_task])  // review after setup complete
```

### Phase 4: Cadenced Engagement Model

After onboarding, move to a **tier-based engagement cadence**:

| Tier           | ARR Range  | Touch Frequency       | Meeting Type                     | Owner              |
| -------------- | ---------- | --------------------- | -------------------------------- | ------------------ |
| **Enterprise** | >$100k     | Weekly + ad-hoc       | Strategic QBR + working sessions | Senior CSM         |
| **Mid-Market** | $25k-$100k | Bi-weekly             | Working sessions + quarterly QBR | CSM                |
| **Growth**     | $5k-$25k   | Monthly               | Check-in calls + email nurture   | CSM / Automated    |
| **SMB**        | <$5k       | Quarterly / automated | Digital touchpoints only         | Automated + pooled |

**Standard touch point types:**

| Touch Point                         | Purpose                                                | Frequency by Tier                        | Format               |
| ----------------------------------- | ------------------------------------------------------ | ---------------------------------------- | -------------------- |
| **Executive Business Review (EBR)** | Strategic alignment, roadmap, ROI proof                | Enterprise: Quarterly; MM: Semi-annually | 45-60 min video      |
| **Working Session**                 | Tactical execution, feature deep-dive, problem-solving | Enterprise: Weekly; MM: Bi-weekly        | 30 min video         |
| **Health Check-In**                 | Pulse check, blockers, quick wins                      | All tiers per cadence above              | 15 min call or async |
| **Success Story Capture**           | Document ROI, case study material                      | Post major milestone                     | Async (email + doc)  |
| **Proactive Outreach**              | Product updates, best practices, relevant tips         | As triggered by events                   | Email / in-app       |

### Phase 5: QBR Execution Protocol

QBRs are the highest-leverage CSM activity. Follow this structure every time:

```
QBR Template (45-60 minutes):

┌─ PREP (CSM does before meeting) ─────────────────────┐
│ 1. Pull latest health score + trend (graph query)    │
│ 2. Gather usage analytics from product                │
│ 3. Review open tickets + resolutions                 │
│ 4. Compile product roadmap items relevant to them    │
│ 5. Prepare 1-2 expansion/upsell suggestions          │
│ 6. Send pre-read deck 48h in advance                 │
└──────────────────────────────────────────────────────┘

┌─ MEETING AGENDA ─────────────────────────────────────┐
│ [5 min]  Welcome + agenda alignment                  │
│ [10 min] Success recap — what we've achieved together │
│           (metrics vs goals set at last QBR)         │
│ [10 min] Product usage deep-dive — what's working,  │
│           what's underutilized, recommendations       │
│ [10 min] Roadmap preview — what's coming, gather     │
│           input on priorities                         │
│ [10 min] Expansion discussion — upsell/cross-sell    │
│           opportunities identified                    │
│ [5 min]  Action items + next steps + date next QBR   │
└──────────────────────────────────────────────────────┘

┌─ POST-MEETING (within 24h) ─────────────────────────┐
│ 1. Send meeting notes + action items to all attendees │
│ 2. Update customer graph node with QBR outcomes      │
│ 3. Create follow-up tasks for each action item      │
│ 4. Store key decisions in GraphRAG                   │
│ 5. Schedule next QBR                                 │
└──────────────────────────────────────────────────────┘
```

### Phase 6: Renewal & Expansion Orchestration

**Renewal Playbook (90 days before contract end):**

```
D-90: Renewal forecast entry — confirm contract dates, decision maker mapping
D-60: Value recap document — compile all ROI evidence, success metrics
D-45: Preliminary renewal discussion — gauge sentiment, surface objections early
D-30: Formal proposal — pricing, terms, any requested changes
D-15: Executive alignment — get champion buy-in, address final concerns
D-7:  Paperwork + signature — send contract, track signature progress
D+0:  Renewal celebration! → Update graph node, log outcome
```

**Expansion Playbook (triggered by signals):**

| Signal             | Trigger Condition                | Action                              |
| ------------------ | -------------------------------- | ----------------------------------- |
| Seat ceiling hit   | Usage ≥ 80% of purchased seats   | Propose seat expansion              |
| Feature request    | Repeated ask for premium feature | Demo + trial of higher tier         |
| New department     | Logo detected from new domain    | Reach out about org-wide deployment |
| Budget increase    | Customer mentions growth/hiring  | Discuss scaling options             |
| Champion promotion | Key contact gets promoted        | Leverage for executive sponsorship  |

**Churn Prevention (when health drops below 60):**

```
Immediate actions:
  1. Executive escalation — inform your leadership within 24h
  2. Save plan creation — root cause analysis + remediation steps
  3. Executive sponsor meeting — get C-level alignment on path forward
  4. 30-day intensive cadence — daily/every-other-day touch points
  4. Commercial flexibility — if appropriate, offer concession (discount, extension)

Save plan template:
  Root cause: <why they're leaving>
  Proposed remedy: <what we'll do differently>
  Timeline: <specific milestones>
  Success criteria: <how we know it worked>
  Concession (if any): <what we're offering>
  Next review: <date>
```

### Phase 7: Portfolio Analytics

**Weekly portfolio dashboard:**

```bash
mindx graph query --cypher "
  MATCH (p:CSMPortfolio)-[:MANAGES]->(c:Customer)
  WHERE c.status = 'active'
  RETURN 
    count(c) as total_accounts,
    sum(c.arr) as total_arr,
    avg(c.health_score) as avg_health,
    count(CASE WHEN c.health_score >= 90 END) as healthy,
    count(CASE WHEN c.health_score >= 70 AND c.health_score < 90 END) as engaged,
    count(CASE WHEN c.health_score >= 50 AND c.health_score < 70 END) as at_risk,
    count(CASE WHEN c.health_score < 50 END) as critical,
    sum(CASE WHEN c.contract_end < date('now + 90d') THEN c.arr ELSE 0 END) as arr_at_risk_renewal
"
```

**Key CSM KPIs:**

| KPI                             | Formula                                                    | Target                 | Why It Matters                                           |
| ------------------------------- | ---------------------------------------------------------- | ---------------------- | -------------------------------------------------------- |
| **Gross Retention Rate**        | (Start ARR - Churned ARR) / Start ARR                      | > 90%                  | Baseline retention health                                |
| **Net Revenue Retention (NRR)** | (Start ARR + Expansion - Churn - Contraction) / Start ARR  | > 110%                 | Best single CSM metric — shows growth from existing base |
| **Time to Value (TTV)**         | Days from contract start to first value milestone achieved | < 30 days              | Faster TTV = lower churn                                 |
| **Health Score Trend**          | Avg health score change over quarter                       | Positive or flat       | Leading indicator of future churn                        |
| **QBR Completion Rate**         | QBRs held / QBRs scheduled                                 | > 95%                  | Discipline metric — missed QBRs = blind spots            |
| **Expansion Pipeline**          | $ value of identified expansion opportunities              | > 15% of portfolio ARR | Feed for sales team                                      |

## Team Composition

| Role                    | Responsibility                                        | When Needed             |
| ----------------------- | ----------------------------------------------------- | ----------------------- |
| `csm-lead`              | Enterprise accounts, EBRs, strategic escalations      | Always (team lead)      |
| `onboarding-specialist` | New customer 90-day journey                           | During onboarding waves |
| `health-analyst`        | Score calculation, trend analysis, alert generation   | Daily/weekly cycles     |
| `renewal-manager`       | Contract negotiations, paperwork, timeline management | 90 days before renewals |
| `content-writer`        | QBR decks, case studies, success stories              | Per QBR cycle           |

## Anti-Patterns

- Do NOT wait until 30 days before renewal to start the conversation — begin at D-90
- Do NOT skip QBRs because "the customer seems happy" — happy customers still churn when contracts auto-renew without review
- Do NOT let health scores silently decline — act on the first drop below 70, not the first cancellation notice
- Do NOT treat all customers the same — enterprise needs strategic partnership, SMB needs scalable digital touch
- Do NOT hide problems from leadership during escalations — early visibility enables rescue; late visibility ensures failure
- Do NOT over-promise in save plans — under-promise and over-deliver, or you burn credibility for the next crisis
- Do NOT confuse activity with outcomes — 10 calls mean nothing if health score didn't improve
- Do NOT forget the economic buyer — even if your day-to-day contact loves you, the person signing the check needs their own business case
