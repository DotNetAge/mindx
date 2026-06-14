# Agent Definition Reference

## Field Requirements

| Field             | Format                             | Rule                                                                                             |
| ----------------- | ---------------------------------- | ------------------------------------------------------------------------------------------------ |
| `name`            | lowercase-hyphen                   | Unique, noun-based, reflects role (e.g. `python-engineer`)                                       |
| `role`            | ~5 words                           | Human-readable role title, include seniority if helpful (e.g. `Senior Python Engineer`)          |
| `description`     | <1024 chars                        | For **LLM routing** — helps other LLMs decide whether to delegate to this agent                  |
| `model`           | exact name from `model.list`       | Match complexity to task — don't waste expensive models on trivial work                          |
| `skills`          | list from `skill.list`             | Only domain-relevant skills — each adds context overhead                                         |
| `meta.name_zh`    | 2-6 Chinese characters             | Concise Chinese display name (stored in meta map)                                                |
| `meta.name_zh_tw` | 2-6 Traditional Chinese characters | Traditional Chinese display name (stored in meta map)                                            |
| `introduction`    | system prompt                      | The agent's **full system prompt / working instructions** — the only content field that persists |

---

## `description` — LLM Routing Description

Written for **LLM routing**: another LLM reads this to decide whether to delegate to this agent. Not marketing copy.

- Start with domain and role
- List concrete responsibilities
- State scope boundaries implicitly
- Keep under 1024 characters

Example (from `frontend-engineer`):
```
Responsible for building modern, responsive, and accessible web interfaces
using React, Vue, TypeScript, CSS frameworks, and build tools. Implements pixel-perfect
UI components, manages application state, optimizes performance, and ensures
cross-browser compatibility. Focuses on user experience, component architecture,
and frontend testing strategies.
```

---

## `meta.name_zh` / `meta.name_zh_tw` — Chinese / Traditional Chinese Name

2-6 characters, concise and descriptive. Examples: "frontend engineer", "personal assistant", "architect", "code reviewer" in Chinese. `name_zh_tw` is the Traditional Chinese conversion of `name_zh`.

---

## `introduction` — Full System Prompt (Mandatory Format)

An agent definition defines three things:
- **Role** (定岗) — what position the agent holds
- **Domain** (定领域) — which field the agent belongs to
- **Responsibilities** (定责) — what scope the agent covers and what it does NOT handle

The LLM reads this to know exactly what the agent is, what it does, and where its boundaries lie. Follow this exact structure:

```
I am a **{Role}** — {one-liner: what I do and my value}
{second sentence extending the intro}

**Domain**: {comma-separated domain categories with tools/tech in parentheses}

**Out of scope**: {what I do NOT handle}
```

### Examples

```
I am a **Compliance Officer** — I ensure regulatory adherence across financial operations, not execute trades or manage portfolios.
I review transactions, flag risks, and enforce policy — I do NOT make investment decisions.

**Domain**: Regulatory compliance monitoring (KYC/AML, MiFID II, SOX), transaction surveillance
(flagging, reporting, audit trails), policy enforcement (restricted lists, insider trading controls),
risk assessment (enhanced due diligence, sanctions screening, PEP checks), compliance training
(material development, attestation tracking).

**Out of scope**: Portfolio management, trade execution, financial advising, tax preparation.
```

```
I am a **Clinical Research Coordinator** — I manage the operational execution of clinical trials, not diagnose or treat patients.
I coordinate sites, track data, and ensure protocol adherence — I do NOT interpret clinical outcomes.

**Domain**: Trial site management (IRB submissions, patient enrollment tracking, visit scheduling),
data collection & integrity (Case Report Forms, source document verification, query resolution),
regulatory documentation (informed consent, protocol amendments, serious adverse event reporting),
budget & contract administration (site payments, clinical trial agreements, scope changes).

**Out of scope**: Patient diagnosis, treatment decisions, statistical analysis, drug formulation.
```

```
I am a **Brand Strategist** — I define how a brand looks, sounds, and feels across every touchpoint.
I build positioning, shape identity, and guide creative direction — I do NOT execute production design.

**Domain**: Brand positioning & strategy (competitive analysis, target audience definition,
brand architecture, messaging hierarchy), identity development (visual language, tone of voice,
brand guidelines), campaign strategy (channel planning, creative briefs, content calendars),
market research (perception audits, focus groups, brand health tracking).

**Out of scope**: Graphic design execution, copywriting, media buying, social media posting.
```

### Rules

- `**{Role}**` must match the agent's `role` field exactly
- Domain categories: `Category A (tools), Category B (tools), ...` — parentheses for specifics
- Out of scope is critical for preventing misrouting — be explicit
- Use "NOT" for emphasis when the agent deliberately does NOT do something

---

## `skills` — Skill Assignment

Skills are **LLM operating instructions**, not feature flags. Each skill activates specific tools, behaviors, or domain knowledge.

- Choose skills that **implement behaviors** the agent needs
- Do NOT hoard — each skill adds context overhead
- Query `skill.list` to see available options
- Example: A security auditor needs `bug-hunter`, `verify`, `find-experts`

---

## Anti-Patterns

- Duplicate names — check `agent.list` first
- Over-scoped descriptions — defeats specialist delegation
- Skill hoarding — each skill adds context overhead
- Model mismatch — heavy model for trivial tasks, light for complex reasoning
- Missing out-of-scope boundaries — leads to misrouting
- Name-role mismatch — `python-engineer` with role "Full-Stack Developer" confuses routing
- `description` written as marketing blurb — it's for LLM routing
- Generic Domain list without specific tools in parentheses
- Missing Domain or Out of scope sections

---

## Creation Checklist

Before running `mindx agent add`:
- [ ] Confirmed name, domain, work scope with user
- [ ] Checked `mindx agent list --json` — no duplicate or overlapping agent exists
- [ ] Checked `mindx skill list --json` — identified relevant skills
- [ ] `name` lowercase-hyphenated, unique, noun-based
- [ ] `role` concise (~5 words), seniority if relevant
- [ ] `description` written for LLM routing, <1024 chars, includes scope
- [ ] `skills` list is minimal — only domain-relevant skills included
