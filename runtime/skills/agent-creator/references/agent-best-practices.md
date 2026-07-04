# Agent Definition Reference

An agent definition is a Markdown file with YAML frontmatter. The frontmatter contains routing metadata; the Markdown body is the agent's system prompt / working instructions.

## Field Requirements

| Field | Format | Rule |
| --- | --- | --- |
| `name` | lowercase-hyphen | Unique, noun-based, reflects role (e.g. `python-engineer`) |
| `role` | ~2-5 words | Human-readable role title, include seniority if helpful |
| `description` | <1024 chars | For **LLM routing** — helps other LLMs decide whether to delegate to this agent |
| `skills` | list from `skill.list` | Only domain-relevant skills — each adds context overhead |
| `meta.name_zh` | 2-6 Chinese characters | Concise Chinese display name |
| `meta.name_zh_tw` | 2-6 Traditional Chinese characters | Traditional Chinese display name |
| `meta.role_zh` | 2-6 Chinese characters | Chinese role title |
| `meta.description_zh` | 1-2 sentences | Chinese description, ending with “从...角度分析问题” |
| body | system prompt | The agent's **full working instructions** — must follow the four-section format below |

---

## `description` — LLM Routing Description

Written for **LLM routing**: another LLM reads this to decide whether to delegate to this agent. Not marketing copy.

- Start with domain and role
- List concrete responsibilities
- State scope boundaries implicitly
- Keep under 1024 characters

Example (from `backend-engineer`):

```
Designs, develops, and maintains server-side applications, APIs, and data pipelines
across multiple languages. Delivers production-grade code with thorough test coverage.
```

---

## Body — Full System Prompt (Mandatory Four-Section Format)

The body defines three things:

- **Role** (定岗) — what position the agent holds
- **Domain** (定领域) — which field the agent belongs to
- **Responsibilities** (定责) — what scope the agent covers and what it does NOT handle

The LLM reads this to know exactly what the agent is, what it does, and where its boundaries lie. Follow this exact structure:

```markdown
I am a **{Role}**. {one-liner: what I do and my value} — I do **not** {boundary}.
{second sentence extending the intro}

## Professional Areas

- **{Area 1}** — {specific capability}
- **{Area 2}** — {specific capability}
- **{Area 3}** — {specific capability}

## Core Deliverables

- **{Deliverable 1}** — {what it contains}
- **{Deliverable 2}** — {what it contains}

## Behavior Rules

### {Imperative Rule 1}

{Specific, enforceable standard.}

### {Imperative Rule 2}

{Specific, enforceable standard.}

### Don't {Overstep}

{Clear boundary of what this agent does NOT do.}
```

### Section Rules

- `**{Role}**` must match the agent's `role` field exactly.
- **Identity Statement**: one or two sentences; state who you are and what you do NOT do.
- **Professional Areas**: bullet list; format `**Title** — explanation`.
- **Core Deliverables**: bullet list of named outputs; format `**Name** — contents`.
- **Behavior Rules**: imperative titles with specific, enforceable standards. Include explicit boundary rules (`Don't...`).
- Use "NOT" for emphasis when the agent deliberately does NOT do something.

### Example

From `backend-engineer`:

```markdown
I am a **Backend Engineer**. My quality comes from rigorous standards, not raw capability.

## Professional Areas

- **API Development** — REST/GraphQL/gRPC
- **Business Logic** — Core rules, service orchestration
- **Database** — Data modeling, indexing, query tuning
- **Data Pipelines** — Async processing, message queues, batch tasks
- **Auth & Security** — Authentication, permission models, OWASP Top 10
- **Caching** — Redis/Memcached
- **Testing** — Unit, integration, E2E

## Core Deliverables

- **Data Model Definitions** — Output first when data storage is involved
- **Database Migration Plans** — Forward + rollback scripts
- **API Interface Documentation** — Request/response, error codes, boundary conditions
- **Implementation Code** — With corresponding tests

## Behavior Rules

### Design First, Code Later

For new features with data models or APIs: design first (data model → interface → business logic), implement after confirmation.

### Interface Completeness

Every interface defines: structure, required/optional fields, validation, error responses, rate limits. No hidden boundary behaviors.

### Database Change Safety

Schema changes include forward + rollback. Always define index strategy.

### External Calls Need Error Handling

All calls to DB, API, filesystem handle errors.
```

---

## `skills` — Skill Assignment

Skills are **LLM operating instructions**, not feature flags. Each skill activates specific tools, behaviors, or domain knowledge.

- Choose skills that **implement behaviors** the agent needs
- Do NOT hoard — each skill adds context overhead
- Query `skill.list` to see available options
- Example: A security auditor needs `bug-hunter`, `verify`, `find-experts`

---

## Style Rules

- Use direct, short, imperative language.
- Prefer absolute terms: `Every`, `All`, `Always`, `Never`, `No`, `must not`.
- Every proposal or deliverable must state what it includes and what it excludes.
- The definition is a **constraint list**, not a capability brag.
- Chinese `description_zh` should end with the perspective phrase: “从...角度分析问题”.

---

## Anti-Patterns

- Duplicate names — check `agent.list` first
- Over-scoped descriptions — defeats specialist delegation
- Skill hoarding — each skill adds context overhead
- Missing out-of-scope boundaries — leads to misrouting
- Name-role mismatch — `python-engineer` with role "Full-Stack Developer" confuses routing
- `description` written as marketing blurb — it's for LLM routing
- Body missing any of the four required sections
- Generic Professional Areas without specific capabilities
- Behavior Rules that are vague or not enforceable

---

## Creation Checklist

Before running `mindx agent add`:

- [ ] Confirmed name, domain, work scope with user
- [ ] Checked `mindx agent list --json` — no duplicate or overlapping agent exists
- [ ] Checked `mindx skill list --json` — identified relevant skills
- [ ] `name` lowercase-hyphenated, unique, noun-based
- [ ] `role` concise (~2-5 words), seniority if relevant
- [ ] `description` written for LLM routing, <1024 chars, includes scope
- [ ] `skills` list is minimal — only domain-relevant skills included
- [ ] Body follows the four-section format: Identity, Professional Areas, Core Deliverables, Behavior Rules
- [ ] Behavior Rules include explicit boundaries (`Don't...`)
