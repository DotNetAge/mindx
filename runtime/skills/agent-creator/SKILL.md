---
name: agent-creator
description: >
  Creates and registers a new agent with a specific role, expertise,
  or capability. Use when you need a specialist in a particular domain
  and no existing agent fits the requirement.
metadata:
  name_zh: 创建智能体
  name_zh-tw: 建立智慧體
  description_zh: 创建和注册具有特定角色、专业知识或能力的新智能体
  description_zh-tw: 建立和註冊具有特定角色、專業知識或能力的新智慧體
---

## When to Use

- User says "I need a XXX expert", "I need someone who knows XXX", "create an agent for XXX"
- A workflow requires a specialist and no existing agent is suitable

**Do NOT use** when a suitable agent already exists.

## Guiding Principle: Hypothetical Options First

When collecting information, do NOT simply ask open-ended questions. Instead:

1. **Interpret the user's intent** and generate 2-4 specific hypothetical options
2. **Present them for confirmation** — let the user pick or refine
3. Only ask open-ended if none of the options fit

**Example**: If the user says "I need a project manager", respond with:

> I can create a project management agent. Which type fits best?
> 
> - **Software Project Manager** — manages development sprints, task tracking, agile workflows, and team coordination
> - **Construction Project Manager** — oversees building projects, timelines, resource allocation, and compliance
> - **Marketing Campaign Manager** — plans and executes marketing initiatives, tracks KPIs, manages content calendars
> - **Other** — describe your specific needs
>
> Or do you have something else in mind?

Apply this technique to all data collection below. It reduces back-and-forth and helps the user articulate their needs faster.

## Prerequisite: Collect Required Information

Before proceeding, verify whether ALL of the following information has been collected. If any item is unclear or missing, use the hypothetical-options technique above to clarify with the user.

### (a) Agent Name

- Lowercase-hyphen format, noun-based, reflects the role (e.g. `python-engineer`, `security-auditor`)

### (b) Domain / Role

- What domain does this expert belong to?
- This becomes the human-readable role title (e.g. "Senior Frontend Engineer")
- Include seniority if helpful

### (c) Work Scope & Responsibilities

- What specific tasks will this expert handle?
- What are the boundaries (IN scope / OUT of scope)?
- What quality standards should they follow?
- This information feeds into the Markdown body (the system prompt content)

### (d) Required Skills

- Based on domain and responsibilities, run `mindx skill list --json` to see available skills
- Pre-select the skills this expert needs
- Skills are **LLM operating instructions** — each skill tells the LLM what behaviors to activate
- Keep the list minimal — each skill adds context overhead

> If the user's description is vague, do not guess blindly — propose specific role categories and let them choose.

## Agent Definition Writing Guide

The agent definition is a Markdown file with YAML frontmatter. It must match the exact format used by existing agents in `runtime/agents/`.

### File Structure

```markdown
---
name: <kebab-case-id>
role: <English Role Title>
description: >
  <One sentence for LLM routing: responsibility, output, boundary>
skills:
  - <skill-1>
  - <skill-2>
meta:
  name_zh: <中文名>
  role_zh: <中文角色>
  description_zh: |
    <一句话职责>，从<xxx>角度分析问题。
---

I am a **<Role>**. I focus on "..." and "..."

## Professional Areas

...

## Core Deliverables

...

## Behavior Rules

...
```

### Frontmatter Fields

| Field                 | Format           | Purpose                                                       |
| --------------------- | ---------------- | ------------------------------------------------------------- |
| `name`                | lowercase-hyphen | Unique machine ID                                             |
| `role`                | ~2-5 words       | Human-readable role title                                     |
| `description`         | <1024 chars      | For LLM routing; include responsibility, output, and boundary |
| `skills`              | list             | Only domain-relevant skills; each adds context overhead       |
| `meta.name_zh`        | 2-6 chars        | Chinese display name                                          |
| `meta.role_zh`        | 2-6 chars        | Chinese role title                                            |
| `meta.description_zh` | 1-2 sentences    | Chinese description, ending with “从...角度分析问题”          |

### Body: Four-Section Format

Every agent body follows this exact structure:

1. **Identity Statement** — One or two sentences. State who you are and what you do NOT do. Use bold for the role name and `**not**` for boundaries.
2. **Professional Areas** — Bullet list of domain capabilities. Format: `**Title** — explanation`.
3. **Core Deliverables** — Bullet list of named outputs. Format: `**Deliverable Name** — what it contains`.
4. **Behavior Rules** — Imperative rules. Each rule has a bold title and a specific, enforceable standard. Include explicit boundary rules (`Don't...`).

### Style Rules

- Use direct, short, imperative language.
- Prefer absolute terms: `Every`, `All`, `Always`, `Never`, `No`, `must not`.
- Every proposal or deliverable must state what it includes and what it excludes.
- The definition is a **constraint list**, not a capability brag.
- Chinese `description_zh` should end with the perspective phrase: “从...角度分析问题”.

### Full Template

```markdown
---
name: <kebab-case-id>
role: <Role Title>
description: >
  <Responsibility>. <Concrete outputs>. <Scope boundary>.
skills:
  - <skill-1>
  - <skill-2>
meta:
  name_zh: <中文名>
  role_zh: <中文角色>
  description_zh: |
    <一句话职责>，从<xxx>角度分析问题。
---

I am a **<Role>**. I focus on "<...>" and "<...>."

## Professional Areas

- **<Area 1>** — <brief description>
- **<Area 2>** — <brief description>
- **<Area 3>** — <brief description>

## Core Deliverables

- **<Deliverable 1>** — <what it contains>
- **<Deliverable 2>** — <what it contains>

## Behavior Rules

### <Imperative Rule 1>

<Specific, enforceable standard.>

### <Imperative Rule 2>

<Specific, enforceable standard.>

### Don't <Overstep>

<Clear boundary of what this agent does NOT do.>
```

### Examples

See existing agents such as `runtime/agents/backend-engineer.md` and `runtime/agents/product-manager.md`.

## Workflow

### Step 1: Check for Existing Agents

```bash
mindx agent list --json
```

- If an agent with the **same name** or **overlapping domain** already exists, **inform the user and stop**
- Show which existing agent overlaps and let the user decide whether to proceed with a different role

You can also check a specific name:

```bash
mindx agent get <proposed-name>
```

### Step 2: Review Writing Guidelines

Read the **Agent Definition Writing Guide** above and `references/agent-best-practices.md` before writing anything. They contain the exact format, field rules, and style constraints.

### Step 3: Query Available Skills and Models

```bash
mindx skill list --json
mindx model list --json
```

- Select only domain-relevant skills that **implement the behaviors this agent needs**
- Match model complexity to task — don't waste expensive models on trivial work

### Step 4: Write the Agent Definition

Use the template and style rules in the **Agent Definition Writing Guide** to write the YAML frontmatter and the Markdown body. The body becomes the agent's system prompt / working instructions.

### Step 5: Create the Agent

```bash
mindx agent add <agent-name> \
    --role "Senior Role Title" \
    --description "description for LLM routing" \
    --skills "skill1,skill2"
```

### Step 6: Verify

```bash
mindx agent list --json
```

The agent is now registered and ready for delegation.

## Important Notes

- **All fields are for LLM consumption unless explicitly stated otherwise.** Write clearly and precisely — vague descriptions lead to misrouting.
- **Skills are operating instructions** that tell the LLM what behaviors to exhibit, not feature flags for human users.
- **Less is more** — an overly broad agent with too many skills will be less effective than a focused specialist.
- **Always propose options before asking open-ended questions.** This makes the interaction faster and helps users clarify their own needs.
