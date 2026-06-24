---
name: research-pipeline
description: >
  Structured research skill — define the real question first, then gather evidence, synthesize findings, and
  produce a verified Markdown report. Designed for deep-dive analysis that aligns with the user's actual decision
  needs. Supports multi-agent scaling for large studies.
allowed-tools: bash sub-agent collect-results task-create task-update task-get task-list team-create team-list team-get-tasks find-experts firecrawl
metadata:
  name_zh: 研究管线
  name_zh-tw: 研究管線
  description_zh: 结构化研究技能——先定义真正的问题，再收集证据、综合发现，输出经过验证的 Markdown 报告
  description_zh-tw: 結構化研究技能——先定義真正的問題，再收集證據、綜合發現，輸出經過驗證的 Markdown 報告
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

**Do NOT use this skill for:** quick fact-checking, single-question lookups, or trivial information retrieval. For those cases, use a direct query or simple web search. This skill is designed for research that produces structured, citable, decision-grade outputs.

---

## Core Principle: Question Before Method

> The most expensive mistake in research is answering the wrong question perfectly.

This skill is built around one core idea: **the definition of the question determines the value of the answer**. The tools (web search, knowledge graph search, document fetching) are innate capabilities of the LLM — they don't need to be taught. What needs to be taught is how to **ask the right questions, plan the right depth, and verify that the output truly answers what was asked.**

This skill operates in three layers:

| Layer             | What                                                         | Who                                |
| ----------------- | ------------------------------------------------------------ | ---------------------------------- |
| **Methodology**   | How to define the question, choose depth, structure analysis | This skill (the prompt)            |
| **Tools**         | WebSearch, LocalSearch, WebFetch                             | LLM innate (not taught here)       |
| **Orchestration** | Multi-agent coordination for large studies                   | Optional, triggered by depth level |

---

## Phase 1: Problem Definition

**Goal:** Transform a vague request into a precise, verifiable research objective. This is the most important phase — invest 10-15% of total time here.

### Step 1.1 — Four-Question Elicitation Framework

Do NOT start with "what do you want to research?" Instead, use this structured sequence to extract the user's real need:

#### Q1 — Decision Anchor (决策锚定)

Ask the user:

> "What decision will this research inform? What action will you take based on the results?"

This is the single most important question. It determines:
- **Precision required** — a $10M investment decision vs. a casual curiosity need very different rigor
- **Output format** — board deck, internal memo, personal knowledge
- **Confidence threshold** — how sure do you need to be?

Examples of anchoring answers:
- "I need to decide whether to invest in company X" → Full Study, high confidence required
- "I'm exploring if we should enter the APAC market" → Deep Dive, directional confidence
- "I want to understand how vector databases work" → Quick Scan, low stakes

#### Q2 — Known/Unknown Separation (已知与未知分离)

Ask the user:

> "What do you already know about this topic? And what are the specific unknowns you want to resolve?"

This prevents wasted effort on re-discovering known information and focuses research on the real gaps.

Document the output as:

```
Known:
- [fact 1]
- [fact 2]

Unknowns (research targets):
- [gap 1]
- [gap 2]
```

#### Q3 — First Principles Decomposition (一阶原理拆解)

Ask the user:

> "Let's strip away the assumptions. What is this fundamentally about?"

Guide the user to decompose the problem into its irreducible elements. For example, "AI agents in healthcare" decomposes to:
- Who is the buyer? (hospital systems? insurers? patients?)
- What specific problem is solved? (cost reduction? accuracy? access?)
- Why now? (regulatory change? technology maturity? market pressure?)
- What would need to be true for this to work?

This step prevents the user's own framing from biasing the research direction.

#### Q4 — Depth Calibration (深度校准)

Ask the user:

> "What's the cost of being wrong? If this research led you to the wrong conclusion, what would happen?"

Use the answer to determine the research level:

| If wrong answer means...             | Research Level             | Approach                                  |
| ------------------------------------ | -------------------------- | ----------------------------------------- |
| Minor inconvenience                  | **Level 1 — Quick Scan**   | Single agent, <10 sources, half-day       |
| Wasted effort/resources              | **Level 2 — Deep Dive**    | Single agent, 10-30 sources, 1-2 days     |
| Significant financial/strategic loss | **Level 3 — Full Study**   | Multi-agent, 30+ sources, cross-validated |
| Ongoing competitive disadvantage     | **Level 4 — Living Intel** | Multi-agent + periodic refresh cycles     |

### Step 1.2 — Document the Research Protocol

After the four questions, synthesize into a concise protocol:

```markdown
## Research Protocol

**Decision:** [what will this inform?]
**Primary Question:** [single, precise question]
**Sub-Questions:** [2-5 specific unknowns]
**Scope:** [geographic, temporal, domain boundaries]
**Level:** [Quick Scan / Deep Dive / Full Study / Living Intel]
**Confidence Required:** [high / medium / directional]
**Output:** [report, memo, brief, etc.]
**Deadline:** [time constraint]
```

Present this to the user for confirmation **before proceeding**.

---

## Phase 2: Evidence Collection

**Goal:** Gather diverse, high-quality sources that directly address the research questions.

### Guiding Principles

1. **Tools are innate** — Use `WebSearch`, `LocalSearch`, and `WebFetch` as needed. No special instruction is required for how to use them.
2. **Diversify sources** — Don't rely on a single type of source. Mix web articles, academic papers, official documents, analyst reports.
3. **Track provenance** — For every source, record: URL/DOI, publisher, author, publish date, access date.
4. **Quality over quantity** — 5 high-quality sources beat 50 blog posts.

### Source Quality Scoring

Score each source on 4 dimensions (1-5 scale):

| Dimension       | What It Measures                          | Red Flags                                        |
| --------------- | ----------------------------------------- | ------------------------------------------------ |
| **Credibility** | Publisher authority, methodology rigor    | Anonymous blog, no methodology cited, known bias |
| **Recency**     | How current is the information?           | Data >2 years old for fast-moving domains        |
| **Relevance**   | Directly addresses the research question? | Tangential mention only                          |
| **Bias Check**  | Balanced perspective or advocacy?         | Vendor-sponsored "research", no counter-evidence |

**Minimum threshold:** Average score ≥ 3.5 for inclusion in final synthesis. Flag low-scoring sources but don't discard — they may still provide useful context.

### Collection Checklist

- [ ] At least 2 independent sources per key claim
- [ ] Sources span multiple perspectives (bull, bear, neutral)
- [ ] Primary sources preferred over secondary (original data > someone's interpretation)
- [ ] Sources are documented with full citation metadata

---

## Phase 3: Multi-Agent Orchestration

**Goal:** Scale research effort proportionally to the decision at stake.

### When to Scale

| Research Level         | Agent Count   | When to Deploy Additional Agents                                     |
| ---------------------- | ------------- | -------------------------------------------------------------------- |
| Level 1 — Quick Scan   | 1 (lead only) | Never                                                                |
| Level 2 — Deep Dive    | 1-2           | If sources span very different domains (e.g., technical + financial) |
| Level 3 — Full Study   | 2-4           | Always — parallel workstreams                                        |
| Level 4 — Living Intel | 2-4 + ongoing | Always                                                               |

### Role Definitions

| Role                      | Responsibility                                                           | Trigger                                      |
| ------------------------- | ------------------------------------------------------------------------ | -------------------------------------------- |
| **Lead Researcher**       | Question design, source coordination, synthesis, quality control         | Always                                       |
| **Data Analyst**          | Quantitative analysis, statistical work, data cleaning, chart production | When quantitative data is a major component  |
| **Subject Matter Expert** | Domain-specific validation, methodology review, sanity-check             | When domain-specific expertise is required   |
| **Writer**                | Report drafting, narrative structure, executive summary                  | When output is a formal report (Full Study+) |

Use `team-create` and `task-create` / `task-update` to coordinate parallel workstreams.

---

## Phase 4: Analysis & Synthesis

**Goal:** Turn collected evidence into insights, patterns, and actionable conclusions.

### Step 4.1 — Evidence Mapping

For each sub-question from the protocol, map the evidence:

```
Sub-Question: [from protocol]

Supporting Evidence:
- [claim] → [source citation] (confidence: high/medium/low)
- [claim] → [source citation] (confidence: high/medium/low)

Contradicting Evidence:
- [claim] → [source citation] (confidence: high/medium/low)

Gaps:
- [what we still don't know or have insufficient evidence for]
```

### Step 4.2 — Pattern Detection

Look for:
- **Convergence** — Multiple independent sources reaching similar conclusions (increases confidence)
- **Divergence** — Sources disagreeing (note the disagreement, don't smooth it over)
- **Gaps** — Questions with insufficient evidence (flag as uncertainty)
- **Temporal trends** — How things change over time

### Step 4.3 — Hypothesis Formation

For each major finding:

1. State the hypothesis clearly
2. Identify supporting evidence (with source citations)
3. Identify contradictory evidence (actively seek disconfirmation)
4. Assign confidence level based on evidence strength
5. Note remaining uncertainties

### Step 4.4 — Fact vs. Assumption Separation

**Every conclusion must be tagged as one of:**

| Tag                     | Meaning                                                                                                    | Example                                                                                                                  |
| ----------------------- | ---------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| **Fact**                | Directly supported by 2+ independent, high-quality sources                                                 | "Company X raised $500M Series D in March 2025 (Source A, Source B)"                                                     |
| **Synthesized Finding** | Derived from combining multiple data points; each data point is factual but the conclusion is interpretive | "Company X is expanding into APAC based on hiring patterns and partnership announcements (Source C, Source D)"           |
| **Assumption**          | Reasonable inference with limited direct support; explicitly labeled                                       | "Assuming current growth rate continues, Company X will need another round in 18 months (extrapolation from Source E)"   |
| **Speculation**         | Informed opinion with insufficient evidence; must be flagged                                               | "Company X could be an acquisition target for Big Tech (no direct evidence; inferred from market consolidation pattern)" |

**Never present an Assumption or Speculation as a Fact.** If the user needs higher confidence, flag which sources would resolve the uncertainty.

### Confidence Levels

| Level      | Criteria                                                                                                      |
| ---------- | ------------------------------------------------------------------------------------------------------------- |
| **High**   | Supported by 3+ independent, high-quality sources; quantitative data available; no significant contradictions |
| **Medium** | Supported by 2+ sources with some limitations; qualitative consensus; minor contradictions exist              |
| **Low**    | Single source or limited corroboration; significant contradictions; largely speculative                       |

---

## Phase 5: Report Production

**Goal:** Produce a structured Markdown file saved to the current working directory.

### Output Location

Save the report as a `.md` file in the current working directory with a clear, descriptive filename:

```
<project-dir>/research-<topic-slug>-<YYYY-MM-DD>.md
```

For example: `research-ai-healthcare-diagnostics-2025-06-24.md`

### Required Report Structure

```markdown
# Research: [Title]

**Date:** YYYY-MM-DD
**Research Level:** Quick Scan / Deep Dive / Full Study / Living Intel
**Decision Context:** [what decision this research informs]

---

## Executive Summary

[One-paragraph bottom line. 3-5 key findings with confidence levels. Recommendations if applicable.]

---

## 1. Research Methodology

- **Primary Question:** [from protocol]
- **Scope:** [geographic, temporal, domain boundaries]
- **Sources:** [count by type, quality distribution]
- **Limitations:** [what this research does NOT cover]

## 2. Findings

[Organized by theme, not by source. Each finding includes claim, supporting evidence with citations, confidence level.]

## 3. Analysis

[Cross-finding synthesis, pattern interpretation, implications.]

## 4. Conclusions

[Direct answers to the primary question and sub-questions. Tagged by fact/synthesized finding/assumption/speculation.]

## 5. Recommendations

[Actionable, prioritized, with supporting evidence references.]

---

## Appendix

### A. Source List

| #   | Title | Publisher | Date | Credibility | Recency | Relevance | Bias | URL |
| --- | ----- | --------- | ---- | ----------- | ------- | --------- | ---- | --- |

### B. Uncertainties & Gaps

[What remains unknown. What would resolve it.]

### C. Protocol (Original)

[Copy of the research protocol from Phase 1 for traceability.]
```

### Citation Rules

1. **Every factual claim must have ≥1 source citation** — format: `[Source N]`
2. **Conflicting sources must be acknowledged** — don't hide disagreement
3. **Distinguish between** direct quotes, paraphrases, and synthesized conclusions
4. **Assumptions and speculations must be labeled** — use the tags from Phase 4.4

---

## Phase 6: Verification & Reflection

**Goal:** Before delivering the output, verify that the report genuinely answers the original research question with integrity.

### Step 6.1 — Goal Alignment Check

Re-read the original Research Protocol and ask:

- [ ] Does the report directly answer the primary question? (not a different question, not a nearby question)
- [ ] Does it address every sub-question from the protocol?
- [ ] Are all conclusions within the agreed scope? (no scope creep that wasn't agreed upon)
- [ ] If the decision anchor was "should I invest in X", does the report give a clear answer or actionable framework for that decision?

If any check fails, the report is incomplete — revise or note the gap explicitly.

### Step 6.2 — Evidence Integrity Check

- [ ] Is every factual claim backed by at least one source citation?
- [ ] Are confidence levels assigned to every major finding?
- [ ] Are assumptions and speculations clearly labeled as such? (no "conclusion dressing" — don't write an assumption like it's a fact)
- [ ] Are contradictory or dissenting sources acknowledged?
- [ ] Are any claims made with "no evidence" tag? (if so, remove or flag as speculation)

### Step 6.3 — Reasoning Integrity Check

- [ ] Does the reasoning chain from evidence → conclusion hold? Or are there logical leaps?
- [ ] Are all numerical claims traceable to their source calculation? (e.g., "market will reach $X by 2030" — which analyst, what methodology?)
- [ ] Are correlations distinguished from causations?
- [ ] If the report makes a prediction or forecast, is the underlying model/assumption stated?

### Step 6.4 — Final Confidence Statement

End with an honest assessment:

> **Confidence in this report:** [High / Medium / Low]
>
> **Key uncertainties:** [what would change the conclusions if new evidence emerged]
>
> **Recommended follow-up:** [what to do if higher confidence is needed — e.g., "commission a primary survey", "interview 3 industry executives", "purchase Gartner report X"]

---

## Anti-Patterns

1. **Confirmation Bias / Cherry-Picking**
   Don't selectively include sources that support a pre-existing conclusion while ignoring contradicting evidence. Actively seek out disconfirming sources. If you can't find them, note that absence as a limitation.

2. **Answering a Different Question**
   The most common failure mode. A user asks about "market opportunity" but gets a "technology overview." Stay anchored to the protocol. If new questions emerge during research, separate them into "additional findings" rather than letting them redirect the report.

3. **Presenting Assumptions as Facts**
   Never write "Company X will enter this market" when what the evidence supports is "Company X's job postings suggest they are exploring this market." The difference matters. Label it.

4. **Confusing Correlation with Causation**
   Just because two trends co-occur doesn't mean one caused the other. Be explicit about what the evidence shows vs. what you infer.

5. **Ignoring Source Bias**
   Every source has a perspective. A vendor's whitepaper promotes their solution. A short seller's report highlights risks. An academic paper may have funding conflicts. Characterize and disclose source bias.

6. **Over-Generalizing from Limited Samples**
   "We analyzed 5 companies and found..." is not the same as "Across the industry...". Be precise about scope and sample size. Extrapolation should always be flagged as such.

7. **Skipping the Problem Definition Phase**
   Jumping straight to web searches without defining the research question leads to scope creep, wasted effort, and unfocused outputs. Spend 10-15% of total time on Phase 1.

8. **Producing a Report Without Verification**
   Skipping Phase 6 means delivering a report that may not actually answer the question. Always run the verification checks before presenting the output.
