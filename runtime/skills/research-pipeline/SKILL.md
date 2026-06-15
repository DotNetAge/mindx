---
name: research-pipeline
description: >
  End-to-end research pipeline for deep-dive analysis — define research questions, execute multi-source
  information gathering (web search, document analysis, expert interviews), synthesize findings via
  GraphRAG knowledge graphs, produce structured reports, and maintain living knowledge bases for
  ongoing intelligence.
allowed-tools: bash sub-agent collect-results task-create task-update task-get task-list team-create team-list team-get-tasks find-experts firecrawl
metadata:
  name_zh: 研究管线
  name_zh-tw: 研究管線
  description_zh: 端到端深度研究管线——问题定义、多源信息采集、GraphRAG 知识图谱合成、结构化报告输出、活知识库维护
  description_zh-tw: 端到端深度研究管線——問題定義、多源資訊採集、GraphRAG 知識圖譜合成、結構化報告輸出、活知識庫維護
---

# Research Pipeline — Deep Research & Knowledge Production Skill

## Trigger Decision

**Use this skill when:** the user needs in-depth, structured research including but not limited to:

- **Market analysis** — market sizing, TAM/SAM/SOM, growth trends, segment dynamics
- **Competitive intelligence** — competitor mapping, feature comparison, positioning analysis, win/loss patterns
- **Academic literature review** — systematic survey of papers, citation networks, research gap identification
- **Investment due diligence** — target company deep-dive, financial + technical + team assessment
- **Technology assessment** — tech stack evaluation, emerging technology radar, build-vs-buy analysis
- **Industry report** — sector overview, regulatory landscape, ecosystem mapping, future outlook

**Do NOT use this skill for:** quick fact-checking, single-question lookups, or trivial information retrieval. For those cases, use `mindx query` directly or a simple web search. This skill is designed for research that produces structured, citable, graph-backed outputs.

---

## Domain Knowledge Base — GraphRAG Integration

### Language Handling for GraphRAG Queries

> **CRITICAL: This SKILL.md is written in English for token efficiency and global compatibility, but you MUST query GraphRAG in the language matching the stored content.**

**Language detection rule:**
- User speaks/writes in **Chinese** → Query `mindx memory query` and `mindx graph query` in **Chinese first**, then English as fallback
- User speaks/writes in **English** → Query in **English first**, then Chinese as fallback
- When storing via `mindx memory store` → Store in the **same language as the user's current working language**
- Graph node `properties` values (entity names, descriptions) → Match the language of stored data
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
| Build structured knowledge base (entities, relationships, sources) | `mindx graph upsert-nodes/edges` (entity graph) |
| Query relationships between entities | `mindx graph query --cypher "..."` (Cypher traversal) |
| Update business state | `mindx graph exec --cypher "SET ..."` (mutation) |
| Cross-reference: entity → full context | Graph node → get source docs → `memory query` |

This skill's superpower is its **heavy GraphRAG integration**. Every piece of research feeds into a living knowledge graph that compounds over time. The graph becomes an intelligence asset — not just a one-off report.

### Core Graph Operations

| Command | Purpose | When to Use |
|---------|---------|-------------|
| `mindx memory store` | Save source documents, interview transcripts, data points as searchable memory entries | After every source is collected |
| `mindx memory query` | Semantic search across all prior research artifacts | When checking if we already have relevant data |
| `mindx graph upsert-nodes` | Create or update entity nodes (companies, people, technologies, concepts) | During entity extraction from sources |
| `mindx graph upsert-edges` | Create or update relationship edges between entities | After nodes are created, to wire the graph together |
| `mindx graph query` | Run Cypher queries for pattern discovery across the full graph | Analysis phase — finding hidden connections |
| `mindx graph neighbors` | Explore 1-2 hop connections around any entity | Entity deep-dives and relationship mapping |
| `mindx graph get-node` | Pull full detail on any single entity node | Fact-checking and detail retrieval |

### Concrete Cypher Query Examples

**Find all AI companies in healthcare that raised Series B+ funding:**

```cypher
MATCH (c:Company)-[:LOCATED_IN]->(s:Sector {name: "Healthcare"})
WHERE c.tags CONTAINS "AI" OR c.description CONTAINS "AI"
MATCH (c)-[r:FUNDING_ROUND]->(f)
WHERE r.stage IN ["Series B", "Series C", "Series D", "Series E", "IPO"]
RETURN c.name, r.stage, r.amount, r.date, f.investor_name
ORDER BY r.amount DESC
```

**Show competitive landscape for a target technology:**

```cypher
MATCH (t:Technology {name: $tech_name})<-[:USES_TECH]-(c:Company)
OPTIONAL MATCH (c)-[:COMPETES_WITH]-(competitor)
WITH c, collect(DISTINCT competitor.name) AS competitors
OPTIONAL MATCH (c)-[:INVESTED_IN]-(inv:Investor)
RETURN c.name, c.founded_year, c.funding_total, competitors,
       [i IN collect(inv) | i.name] AS investors
ORDER BY c.funding_total DESC
```

**Trace acquisition chain for a target domain:**

```cypher
MATCH path = (acquirer:Company)-[:ACQUIRED_BY*0..3]->(target:Company)
WHERE ANY(n IN nodes(path) WHERE n.domain = $domain OR n.tags CONTAINS $domain)
UNWIND relationships(path) AS rel
RETURN startNode(rel).name AS from, type(rel) AS relation,
       endNode(rel).name AS to, rel.date AS date, rel.amount AS amount
ORDER BY date DESC
```

**Identify key opinion leaders / experts in a field:**

```cypher
MATCH (p:Person)-[:PUBLISHED_IN]->(pub:Publication)
WHERE pub.topic CONTAINS $topic OR pub.keywords CONTAINS $topic
WITH p, count(pub) AS publication_count
OPTIONAL MATCH (p)<-[:LED_BY]-(org:Organization)
RETURN p.name, p.title, p.organization, publication_count,
       org.name AS affiliated_org
ORDER BY publication_count DESC
LIMIT 20
```

**Find investment patterns around a technology cluster:**

```cypher
MATCH (t:Technology)-[:RELATED_TO*1..2]-(related_tech)
WITH collect(t) + collect(related_tech) AS tech_cluster
UNWIND tech_cluster AS tc
MATCH (c:Company)-[:USES_TECH]->(tc)
MATCH (c)-[r:INVESTED_IN]->(i:Investor)
RETURN tc.name AS technology, count(DISTINCT c) AS company_count,
       count(DISTINCT i) AS investor_count, sum(r.amount) AS total_invested
ORDER BY total_invested DESC
```

### Memory Store Pattern

Every source document should be stored with consistent metadata:

```bash
mindx memory store \
  --content "$(cat source_document.pdf | text-extract)" \
  --title "Source: Gartner 2025 AI Platforms Magic Quadrant" \
  --source "research-pipeline" \
  --tags "ai-platforms,magic-quadrant,gartner,2025,market-analysis" \
  --metadata '{"type": "industry-report", "publisher": "Gartner", "date": "2025-03", "credibility_score": 9, "access_date": "2025-06-15"}'
```

This ensures every claim in every report can be traced back to its original source.

---

## Research Type Classification

Not all research requests are equal. Classify upfront to set expectations:

| Type | Timeline | Depth | Output Format | Example Request |
|------|----------|-------|---------------|-----------------|
| **Quick Scan** | 2–4 hours | Surface-level, publicly available data only | 1-page memo with key findings + source list | "Who are the top 5 competitors in X space?" |
| **Deep Dive** | 1–3 days | Multi-source synthesis with cross-validation | 5–15 page report with executive summary, findings, implications | "Market sizing for AI agents in APAC region, 2024–2030" |
| **Full Study** | 1–4 weeks | Primary + secondary research, interviews, data analysis | 20–50 page report + supporting dataset + presentation deck | "Comprehensive due diligence on target company Y" |
| **Living Intel** | Ongoing / Continuous | Continuous monitoring with periodic refresh cycles | Living knowledge base + weekly/bi-weekly briefings + alert system | "Competitive landscape monitoring for our product category" |

**Decision heuristic:** If the answer requires synthesizing 5+ sources and producing a structured output → use this skill. If it's a single Google-away question → don't.

---

## Workflow

### Phase 1: Research Design

**Goal:** Transform a vague request into a precise, executable research protocol.

#### Step 1.1 — Define the Research Question (SMART)

The research question must be:
- **S**pecific — narrow enough to be answerable
- **M**easurable — success criteria are quantifiable
- **A**chievable — within time/resource constraints
- **R**elevant — directly addresses the stakeholder's need
- **T**ime-bound — has a clear deadline

**Bad:** "Tell me about AI in healthcare."
**Good:** "Map the competitive landscape of FDA-cleared AI diagnostic imaging companies in the US, identify their funding status (through 2025), key clinical validation approaches, and hospital adoption barriers. Deliver a 10-page report with market size estimates."

#### Step 1.2 — Determine Scope & Constraints

Document these explicitly:
- **Time budget:** How many hours/days?
- **Geographic scope:** Global? Regional? Country-specific?
- **Source constraints:** Public only? Can we purchase reports? Access to internal data?
- **Language requirements:** English only? Multi-language?
- **Stakeholder audience:** Executive? Technical? Investor? Legal?

#### Step 1.3 — Choose Methodology

| Approach | Best For | Methods |
|----------|----------|---------|
| Quantitative | Market sizing, financial analysis, trend quantification | Financial filings, market data, surveys, statistical modeling |
| Qualitative | Competitive positioning, strategy assessment, sentiment analysis | Interviews, expert opinions, content analysis, case studies |
| Mixed | Most real-world research projects | Combine both — quantify where possible, qualify where needed |

#### Step 1.4 — Define Success Criteria

Before starting collection, define what "done" looks like:
- Minimum number of sources per claim
- Required confidence threshold for key findings
- Mandatory sections in the output
- Review/approval process

#### Output of Phase 1: **Research Protocol Document**

Store it in the graph so it's traceable:

```bash
mindx graph upsert-nodes --nodes '[
  {
    "id": "protocol:<project_id>",
    "labels": ["ResearchProtocol"],
    "properties": {
      "project_id": "<project_id>",
      "title": "<research_title>",
      "question": "<smart_research_question>",
      "scope": {...},
      "methodology": "<quant|qual|mixed>",
      "timeline": "<start_date> to <end_date>",
      "success_criteria": [...],
      "status": "designed",
      "created_at": "<timestamp>"
    }
  }
]'
```

---

### Phase 2: Source Planning & Collection

**Goal:** Gather high-quality, diverse sources with full provenance tracking.

#### Source Taxonomy

| Category | Examples | Access Method | Quality Considerations |
|----------|----------|---------------|----------------------|
| **Public** | Web articles, press releases, patents, SEC filings, academic papers, government data, GitHub repos | Web search, `firecrawl`, API access | High accessibility; variable quality; check recency and publisher credibility |
| **Proprietary** | Expert interviews, internal documents, survey responses, customer data | Interview scheduling, internal access | Highest uniqueness value; requires access; may have bias |
| **Purchased** | Gartner / Forrester reports, IBISWorld, PitchBook, Crunchbase Pro, Statista | Subscription / pay-per-use | Generally high quality; cost consideration; check if already available |

#### Collection Methods Per Source Type

**Web Sources:**
1. Use targeted web searches with site-specific operators
2. Use `firecrawl` skill for scraping when bulk extraction is needed
3. Extract structured data (tables, lists) where possible
4. Capture URL, access date, page title, author, publish date

**Documents (PDFs, Reports):**
1. Download and store locally
2. Extract text content
3. Index key sections, figures, tables
4. Note methodology and sample size (for surveys/studies)

**Interviews / Expert Input:**
1. Prepare structured interview guide based on research questions
2. Record (with permission) and transcribe
3. Tag transcript with topics and entities
4. Store as proprietary source with expert credentials

#### Source Quality Scoring

Score each source on 4 dimensions (1–5 scale):

| Dimension | What It Measures | Red Flags |
|-----------|-----------------|-----------|
| **Credibility** | Publisher authority, methodology rigor | Anonymous blog, no methodology cited, known bias |
| **Recency** | How current is the information? | Data >2 years old for fast-moving domains |
| **Relevance** | Directly addresses the research question? | Tangential mention only |
| **Bias Check** | Balanced perspective or advocacy? | Vendor-sponsored "research", no counter-evidence |

**Minimum threshold:** Average score ≥ 3.5 for inclusion in final synthesis. Flag low-scoring sources but don't discard — they may still provide useful context.

#### Storing Every Source

After collecting each source, immediately persist it:

```bash
# For web articles
mindx memory store \
  --content "$ARTICLE_CONTENT" \
  --title "Source: <Article Title> (<Publisher>, <Date>)" \
  --source "research-pipeline" \
  --tags "<project_id>,<topic_tags>" \
  --metadata '{
    "type": "web-article",
    "url": "<original_url>",
    "author": "<author_name>",
    "publisher": "<publisher>",
    "publish_date": "<date>",
    "quality_score": 4.2,
    "credibility": 4,
    "recency": 5,
    "relevance": 4,
    "bias_check": 4
  }'

# For documents/reports
mindx memory store \
  --content "$(pdftotext report.pdf -)" \
  --title "Source: <Report Title> (<Firm>, <Date>)" \
  --source "research-pipeline" \
  --tags "<project_id>,<topic_tags>" \
  --metadata '{
    "type": "industry-report",
    "file_path": "/path/to/report.pdf",
    "firm": "<analyst_firm>",
    "publish_date": "<date>",
    "page_count": 45,
    "quality_score": 4.8
  }'
```

This creates a complete audit trail. Every claim in the final report traces back to a stored source.

---

### Phase 3: Entity Extraction & Knowledge Graph Construction

**Goal:** Transform unstructured research into a queryable, structured knowledge graph. This is the core differentiator of this skill.

#### Entity Types to Extract

| Entity Label | Examples | Key Properties |
|--------------|----------|----------------|
| `Company` | OpenAI, Anthropic, DeepMind | name, founded_year, headquarters, funding_total, employee_count, stage, description, tags, url |
| `Person` | Sam Altman, Demis Hassabis | name, title, organization, bio, linkedin, expertise_areas |
| `Technology` | LLM, RAG, Vector DB | name, category, maturity_stage, description, key_vendors, adoption_rate |
| `Product` | GPT-4, Claude, Gemini | name, vendor, category, launch_date, pricing, features, market_position |
| `Investor` | Sequoia, a16z, Microsoft | name, type (VC/PE/Corporate/Angel), focus_sectors, aum, notable_investments |
| `Publication` | ArXiv papers, news articles, analyst reports | title, authors, date, venue, topic, doi/url, type |
| `Sector` | Healthcare AI, Fintech, EdTech | name, description, market_size, growth_rate, key_players |
| `Metric` | TAM, CAGR, NPS scores | name, value, unit, date, source, confidence |
| `Event` | Funding round, Acquisition, Product launch | type, date, participants, amount, description |

#### Building Nodes

Extract entities systematically from each source, then upsert into the graph:

```bash
# Example: Upserting companies found in research
mindx graph upsert-nodes --nodes '[
  {
    "id": "company:openai",
    "labels": ["Company"],
    "properties": {
      "name": "OpenAI",
      "founded_year": 2015,
      "headquarters": "San Francisco, CA",
      "funding_total": "$13B+",
      "employee_count": "~1500",
      "stage": "unicorn",
      "description": "AI research lab focused on safe AGI development",
      "tags": ["llm", "agi", "research", "api", "chatgpt"],
      "url": "https://openai.com",
      "last_updated": "2025-06-15",
      "source_project": "<project_id>"
    }
  },
  {
    "id": "company:anthropic",
    "labels": ["Company"],
    "properties": {
      "name": "Anthropic",
      "founded_year": 2021,
      "headquarters": "San Francisco, CA",
      "funding_total": "$7.3B+",
      "employee_count": "~1000",
      "stage": "unicorn",
      "description": "AI safety company building Claude and Constitutional AI",
      "tags": ["llm", "ai-safety", "claude", "constitutional-ai"],
      "url": "https://anthropic.com",
      "last_updated": "2025-06-15",
      "source_project": "<project_id>"
    }
  },
  {
    "id": "tech:llm",
    "labels": ["Technology"],
    "properties": {
      "name": "Large Language Model (LLM)",
      "category": "Generative AI",
      "maturity_stage": "growth",
      "description": "Foundation models trained on large text corpora for natural language understanding and generation",
      "key_vendors": ["OpenAI", "Anthropic", "Google", "Meta"],
      "adoption_rate": "rapidly accelerating"
    }
  }
]'
```

#### Building Relationships (Edges)

Wire entities together with semantic, typed relationships:

```bash
# Standard edge types used by this skill
mindx graph upsert-edges --edges '[
  {
    "from": "company:openai",
    "to": "tech:llm",
    "type": "USES_TECH",
    "props": {
      "since": "2020",
      "primary_product": "GPT series",
      "notes": "Pioneer in commercial LLM deployment"
    }
  },
  {
    "from": "company:anthropic",
    "to": "tech:llm",
    "type": "USES_TECH",
    "props": {
      "since": "2021",
      "primary_product": "Claude",
      "notes": "Focus on safety-aligned LLMs"
    }
  },
  {
    "from": "company:openai",
    "to": "company:anthropic",
    "type": "COMPETES_WITH",
    "props": {
      "intensity": "high",
      "markets": ["enterprise-api", "consumer-chatbot", "developer-tools"],
      "notes": "Primary competitors in generative AI space"
    }
  },
  {
    "from": "investor:microsoft",
    "to": "company:openai",
    "type": "INVESTED_IN",
    "props": {
      "round": "Series D+ (multiple)",
      "total_amount": "$13B+",
      "first_investment": "2019",
      "strategic_note": "Cloud partnership + exclusive inference rights"
    }
  },
  {
    "from": "person:sam-altman",
    "to": "company:openai",
    "type": "LED_BY",
    "props": {
      "role": "CEO",
      "since": "2019",
      "previous_role": "Y Combinator President"
    }
  }
]'
```

#### Standard Edge Type Reference

| Edge Type | From → To | Example | Use Case |
|-----------|-----------|---------|----------|
| `COMPETES_WITH` | Company → Company | OpenAI ←→ Anthropic | Competitive mapping |
| `INVESTED_IN` | Investor → Company | Sequoia → OpenAI | Investment tracing |
| `ACQUIRED_BY` | Company → Company | DeepMind → Google | M&A chain analysis |
| `USES_TECH` | Company → Technology | OpenAI → LLM | Tech stack mapping |
| `PARTNERS_WITH` | Company → Company/Partner | OpenAI ↔ Microsoft | Partnership network |
| `LED_BY` | Company/Org → Person | OpenAI → Sam Altman | Leadership mapping |
| `PUBLISHED_IN` | Person → Publication | Researcher → Paper | Citation network |
| `LOCATED_IN` | Entity → Region/sector | OpenAI → San Francisco | Geographic/sector placement |

#### Why This Matters

The knowledge graph is not just storage — it's an **analysis engine**. Once built, you can:
- Ask questions that weren't anticipated at design time ("Which investors also funded OpenAI's competitors?")
- Discover non-obvious patterns ("Three of the top five AI healthcare startups share the same seed investor")
- Maintain intelligence that grows more valuable with each new research project
- Reuse the graph as a foundation for future research (don't start from zero)

---

### Phase 4: Analysis & Synthesis

**Goal:** Turn raw collected data and graph structure into insights, patterns, and actionable conclusions.

#### Pattern Detection via Graph Queries

Use the graph to find patterns that aren't visible in individual sources:

```cypher
# Example: Find white space opportunities — underserved sectors
MATCH (t:Technology)<-[:USES_TECH]-(c:Company)
WITH t, count(c) AS adoption_count
WHERE adoption_count < 3 AND t.maturity_stage = "emerging"
RETURN t.name, t.category, adoption_count
ORDER BY adoption_count ASC
```

```cypher
# Example: Identify investment concentration risk
MATCH (i:Investor)-[r:INVESTED_IN]->(c:Company)
WITH i, count(c) AS portfolio_size, sum(r.amount) AS total_deployed
WHERE portfolio_size > 5
RETURN i.name, portfolio_size, total_deployed,
       [c IN [(i)-[:INVESTED_IN]->(comp) | comp.name] | c][0..5] AS sample_portfolio
ORDER BY total_deployed DESC
```

#### Trend Analysis

Look for temporal patterns in node properties:
- Funding amounts over time (are rounds getting bigger?)
- Hiring trends (which skills are companies competing for?)
- Technology mentions in publications (what's rising/falling in interest?)
- Geographic shifts (where is innovation concentrating?)

#### Gap Identification

Systematically check what's missing:
- Which key players have no recent coverage?
- Are there claims without sufficient source support?
- Does the graph have orphan nodes (entities with no relationships)?
- Are there temporal gaps in the data (e.g., missing Q4 data)?

#### Hypothesis Formation & Testing

For each major finding:
1. State the hypothesis clearly
2. Identify supporting evidence (with source citations)
3. Identify contradictory evidence (actively look for disconfirmation)
4. Assign confidence level based on evidence strength
5. Note remaining uncertainties

#### Team Roles for Complex Research

For Deep Dive and Full Study projects, distribute work using sub-agents:

| Role | Responsibility | When to Deploy |
|------|---------------|----------------|
| `researcher` | Primary source collection, entity extraction, initial synthesis | Always — the lead analyst |
| `data-analyst` | Quantitative analysis, statistical work, data cleaning, chart production | When there's numerical data to crunch |
| `subject-matter-expert` | Domain-specific validation, methodology review, sanity-check conclusions | When the domain requires specialized knowledge |
| `writer` | Report drafting, narrative structure, executive summary writing | Final phase — turning analysis into readable output |

Use `team-create` and `task-create` / `task-update` to coordinate parallel workstreams.

---

### Phase 5: Report Production

**Goal:** Produce a polished, structured deliverable that communicates findings clearly with full provenance.

#### Standard Report Structure

```
1. Executive Summary (1 page)
   - One-paragraph bottom line
   - Key findings (3-5 bullet points)
   - Recommendations (if applicable)
   - Confidence overview

2. Research Methodology (0.5-1 page)
   - Research question
   - Scope and constraints
   - Sources used (count by type, quality distribution)
   - Methodology and limitations

3. Findings (main body, 60-70% of report)
   - Organized by theme (not by source)
   - Each finding includes:
     * Claim statement
     * Supporting evidence (with inline citations)
     * Confidence level
     * Visual element reference (chart/table)

4. Analysis (10-20% of report)
   - Cross-finding synthesis
   - Pattern interpretation
   - Comparison to prior benchmarks
   - Implications discussion

5. Implications & Recommendations (1-3 pages)
   - So what? Why does this matter?
   - Actionable recommendations (prioritized)
   - Risk factors and caveats
   - Next steps / follow-up research suggestions

6. Appendix
   - Full source list (every source with citation details)
   - Detailed data tables
   - Methodology notes
   - Glossary of terms
   - Query logs (Cypher queries run during analysis)
```

#### Data Visualization Guidance

| Visualization Type | Best For | Tool Suggestion |
|--------------------|----------|-----------------|
| Bar chart | Comparing values across categories (market shares, funding amounts) | Python matplotlib / seaborn, or export to spreadsheet |
| Line chart | Trends over time (funding trends, adoption curves) | Same as above |
| Scatter plot | Correlation between two variables (funding vs. headcount) | Same as above |
| Network/graph diagram | Relationship maps (competitive landscape, investment network) | Export graph data, use Gephi / D3.js / Mermaid |
| Heatmap | Matrix comparisons (feature comparison tables) | Table-based with color coding |
| Treemap | Hierarchical breakdowns (market segmentation by size) | Plotly or similar |

#### Citation Management Rules

1. **Every factual claim must have ≥1 source citation**
2. **Citation format:** `[Source N]` where N indexes the source list in the appendix
3. **Each source in the appendix links back to its graph node ID** — enabling click-through to the original content
4. **Conflicting sources must be acknowledged** — don't hide disagreement
5. **Distinguish between direct quotes, paraphrases, and synthesized conclusions**

Example inline citation format:
> The enterprise AI orchestration market is projected to reach $18.7B by 2028, growing at a 42% CAGR [Source 3, Source 7]. However, two analyst firms use different market definitions resulting in a 30% variance in absolute figures [Source 3 vs Source 12].

#### Confidence Levels

Assign to every major finding:

| Level | Criteria | Display |
|-------|----------|---------|
| **High** | Supported by 3+ independent, high-quality sources; quantitative data available; no significant contradictions | 🟢 Solid evidence base |
| **Medium** | Supported by 2+ sources with some limitations; qualitative consensus; minor contradictions exist | 🟡 Likely directionally correct; verify before critical decisions |
| **Low** | Single source or limited corroboration; significant contradictions; largely speculative | 🔴 Preliminary finding; treat as hypothesis requiring further investigation |

---

### Phase 6: Knowledge Base Maintenance

**Goal:** Ensure the knowledge graph remains accurate, current, and useful beyond the initial research project.

#### Periodic Refresh Schedule (Living Intel Projects)

| Data Type | Refresh Frequency | Trigger |
|-----------|-------------------|---------|
| Company fundamentals | Monthly | Or on material events (funding, acquisition, leadership change) |
| Market sizing data | Quarterly | New quarterly reports released |
| Competitive features | Bi-weekly | Product release cadence |
| News / press | Daily / Weekly | RSS feed monitoring |
| Pricing data | Weekly | Public pricing page changes |

Implement using scheduled tasks (`task-create` with recurrence):

```bash
task-create \
  --title "Monthly refresh: AI company funding data" \
  --schedule "0 9 1 * *" \
  --action "run-research-pipeline-refresh --scope=funding --domain=ai-companies"
```

#### Stale Data Detection

Query for potentially outdated information:

```cypher
// Find nodes updated >90 days ago in active projects
MATCH (n)
WHERE n.last_updated < datetime() - duration('P90D')
AND n.source_project IS NOT NULL
RETURN labels(n) AS type, n.name, n.last_updated, n.source_project
ORDER BY n.last_updated ASC
LIMIT 50
```

Flag these for re-validation or removal.

#### Graph Pruning

Periodically clean up deprecated entities:
- Companies that shut down or pivoted away from the domain
- Technologies that were superseded
- People who changed roles significantly
- Events that are no longer relevant

Before deleting, consider archiving rather than removing — historical data has value for trend analysis.

#### Reusable Cypher Templates

Save common query patterns as templates for reuse across projects:

**Template 1 — Competitive Landscape Snapshot:**
```cypher
// Parameters: $sector (string), $min_funding (number)
MATCH (c:Company)-[:LOCATED_IN]->(s:Sector)
WHERE s.name CONTAINS $sector
OPTIONAL MATCH (c)-[fr:FUNDING_ROUND]->()
WITH c, sum(fr.amount) AS total_funding
WHERE total_funding >= $min_funding OR total_funding IS NULL
OPTIONAL MATCH (c)-[:COMPETES_WITH]-(peer)
RETURN c.name, c.stage, total_funding,
       count(DISTINCT peer) AS competitor_count
ORDER BY total_funding DESC NULLS LAST
```

**Template 2 — Investment Activity Heatmap:**
```cypher
// Parameters: $domain (string), $months_back (int)
MATCH (i:Investor)-[r:INVESTED_IN]->(c:Company)
WHERE r.date >= date() - duration('P' + toString($months_back) + 'M')
AND (c.tags CONTAINS $domain OR c.description CONTAINS $domain)
WITH i, count(r) AS deal_count, sum(r.amount) AS total_amount
RETURN i.name AS investor, deal_count, total_amount
ORDER BY deal_count DESC
LIMIT 20
```

**Template 3 — Technology Adoption Radar:**
```cypher
// Parameters: $category (string)
MATCH (t:Technology {category: $category})
OPTIONAL MATCH (c:Company)-[u:USES_TECH]->(t)
WITH t, count(c) AS adopter_count,
     collect(DISTINCT c.name)[0..10] AS sample_adopters
RETURN t.name, t.maturity_stage, adopter_count, sample_adopters
ORDER BY adopter_count DESC
```

Store templates in the project's protocol node or as named saved queries.

---

## Team Composition

For complex research projects, assemble a focused team:

| Role | Count | Responsibilities | Skills Needed |
|------|-------|-----------------|---------------|
| **Lead Researcher** | 1 | Research design, source coordination, entity extraction, graph construction, synthesis, quality control | Analytical thinking, domain breadth, graph/structured data literacy |
| **Data Analyst** | 0–1 | Quantitative analysis, statistical modeling, data visualization, metric validation | Statistics, Python/R, data viz tools, numerical reasoning |
| **Subject Matter Expert** | 0–1 | Domain-specific validation, methodology review, expert interviews, conclusion sanity-check | Deep domain expertise, industry experience, professional network |
| **Technical Writer** | 0–1 | Report drafting, narrative structure, executive summary writing, presentation deck creation | Clear writing, visual communication, stakeholder awareness |

**Team sizing guidelines:**
- Quick Scan: Lead Researcher only
- Deep Dive: Lead Researcher + optionally Data Analyst or SME
- Full Study: Full 4-person team recommended
- Living Intel: Lead Researcher ongoing + rotating specialists per refresh cycle

Use `team-create` to instantiate the team, then `team-get-tasks` and `task-create` / `task-update` to assign and track work.

---

## Anti-Patterns

Avoid these common research pitfalls:

1. **Confirmation Bias / Cherry-Picking**
   Don't selectively include sources that support a pre-existing conclusion while ignoring contradicting evidence. Actively seek out disconfirming sources. If you can't find them, note that absence as a limitation.

2. **Presenting Findings Without Confidence Levels**
   Never present all findings as equally certain. A claim backed by 5 independent analyst reports is fundamentally different from a claim based on a single blog post. Always label confidence explicitly.

3. **Letting the Graph Grow Without Curation**
   An uncurated knowledge graph becomes noisy and unreliable. Set quality thresholds for node/edge creation. Regularly prune stale or low-quality entries. Garbage in = garbage out, even in a graph.

4. **Confusing Correlation with Causation**
   Just because two entities are connected in the graph doesn't mean one caused the other. Be explicit about what the relationship represents (observed correlation, reported causation, inferred association).

5. **Ignoring Source Bias**
   Every source has a perspective. A vendor's whitepaper promotes their solution. A short seller's report highlights risks. An academic paper may have funding conflicts. Characterize and disclose source bias — don't treat all sources as neutral.

6. **Over-Generalizing from Limited Samples**
   "We analyzed 5 companies and found..." is not the same as "Across the industry...". Be precise about scope and sample size. Extrapolation should always be flagged as such.

7. **Skipping the Research Design Phase**
   Jumping straight to Google searches without defining the research question leads to scope creep, wasted effort, and unfocused outputs. Spend 10-15% of total time on design — it saves 30-50% downstream.

8. **Producing a Report That Dies**
   The worst outcome is a PDF that nobody reads again after delivery. Structure research so it feeds the living knowledge graph. Build queries that stakeholders can re-run. Make the graph the primary artifact, with the report being a snapshot view.
