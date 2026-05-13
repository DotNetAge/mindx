# Agent Definition Best Practices

Guide for orchestrators who need to create new agent definitions via `create_agent.py`.

## File Format

Every agent is a single `.md` file with YAML frontmatter + optional markdown body:

```markdown
---
name: <identifier>
role: <short-title>
description: >
  Multi-line role description...
model: "<model-name>"
skills:
  - skill-a
  - skill-b
---

## Identity (optional body content)
...system prompt or instruction set...
```

The frontmatter fields are **required** by `create_agent.py`. The body (after `---`) is optional
but recommended for complex agents that need detailed instructions beyond the description.

---

## Field-by-Field Guide

### `name` — Agent Identifier

- **Format**: lowercase, hyphen-separated, no spaces (e.g., `python-engineer`, `code-reviewer`)
- **Purpose**: Used as the filename (`<name>.md`) and as the delegation target name
- **Rules**:
  - Must be unique across all agents — `create_agent.py` rejects duplicates
  - Use a noun-based name that reflects the role, not the task
  - Good: `frontend-engineer`, `security-auditor`, `data-analyst`
  - Bad: `fix-bug`, `write-code`, `helper`

### `role` — Short Role Title

- **Format**: Human-readable title, typically "Senior/Junior [Role]" pattern
- **Purpose**: Displayed in expert rosters and used by LLMs to judge domain fit
- **Rules**:
  - Keep it under ~5 words
  - Include seniority level when relevant (helps with task complexity matching)
  - Good: `Senior Python Engineer`, `Software Architect`, `Security Auditor`
  - Bad: `Python guy`, `does stuff`, `the coder`

### `description` — Role & Capability Description

- **Format**: Folded YAML block scalar (`>`), multi-line, up to ~1024 chars
- **Purpose**: The **primary routing signal** — used by both Orchestrator (to pick experts)
  and Agent itself (for self-judgment of scope). This is the most important field.
- **Content must cover**:
  1. **What this agent does** — core responsibilities and deliverables
  2. **Technical domains** — frameworks, languages, tools it masters
  3. **Output quality standards** — code style, documentation, testing expectations
  4. **Scope boundaries** — what it does NOT do (prevents mis-routing)

**Template:**

```
description: >
  Responsible for [primary responsibility]. Deep expertise in [domain areas]
  including [specific tools/frameworks]. [Quality standard]. Handles [task types]
  but does NOT handle [out-of-scope items].
```

**Example (good):**

```yaml
description: >
  Responsible for designing, developing, and maintaining Python-based
  applications, services, and data pipelines. Deep expertise in the Python
  ecosystem including web frameworks (Django, FastAPI, Flask), data processing
  (pandas, NumPy), testing (pytest), and async programming. Writes clean,
  PEP 8-compliant code with thorough documentation and comprehensive test
  coverage.
```

**Common mistakes:**

- Too vague: `"Helps with coding"` — tells nothing about domain or capabilities
- Too long: >1024 chars bloats the routing context
- Missing boundaries: doesn't say what the agent cannot do, leading to mis-delegation

### `model` — LLM Assignment

- **Format**: String, exact model name from `list_models.py` output
- **Purpose**: Different models have different strengths; match model to task type
- **Selection guide**:

| Task Type                        | Model Characteristics                      |
| -------------------------------- | ------------------------------------------ |
| Complex reasoning / architecture | Strong reasoning capability, large context |
| Code generation / debugging      | Strong coding ability, fast iteration      |
| Creative writing / content       | Good language fluency, creative patterns   |
| Data analysis / math             | Numerical precision, structured output     |
| General-purpose tasks            | Balanced capability, cost-efficient        |

- Always run `list_models.py` first to see available options before choosing
- Consider task complexity — don't assign an expensive heavy model to trivial tasks

### `skills` — Skill Assignments

- **Format**: YAML list of skill names from `list_skills.py` output
- **Purpose**: Activates specific capabilities when the agent spawns
- **Selection rules**:
  - Only assign skills **relevant to the agent's professional domain**
  - Do NOT over-equip — each skill adds context overhead when the agent loads
  - Match skills to the agent's actual responsibilities, not hypothetical needs
  - Run `list_skills.py` first to see all available skills before selecting

**Example mappings:**

| Agent Role         | Typical Skills                  |
| ------------------ | ------------------------------- |
| Python Engineer    | bug-hunter, verify, simplify    |
| Frontend Engineer  | frontend-design, webapp-testing |
| Personal Assistant | file-organizer, xlsx, pdf       |
| Architect          | architect, simplify, batch      |
| Writer             | copywriting, social-content     |

---

## Body Content (Optional System Prompt)

After the closing `---`, add markdown content that serves as the agent's system prompt body.
This is loaded into the ContextWindow only during task execution (T-A-O loop Level 2/3).

### When to include body content:

- The agent needs detailed behavioral instructions beyond what fits in `description`
- The agent has complex scope boundary rules (WITHIN/OUT OF scope lists)
- The agent requires specific output format templates or quality checklists

### When to skip body content:

- Simple agents where `description` + `role` are sufficient for routing and behavior
- Agents whose behavior is fully defined by their assigned skills

### Recommended body structure (if included):

```markdown
## Introduction
Brief self-introduction — who this agent is and what it does in one concise statement.

## Core Responsibilities (My Domain)
Numbered list of tasks handled directly, each with expected output.

## Scope Boundaries (Critical!)
### WITHIN MY SCOPE — I Handle These Myself
- Bullet list of in-scope tasks

### OUT OF MY SCOPE — I Delegate These
- Bullet list of out-of-scope tasks (with suggested expert types)
```

---

## Anti-Patterns

1. **Duplicate names**: Creating an agent with a name that already exists — always run
   `list_agents.py` first to check
2. **Over-scoped descriptions**: One agent that "does everything" — defeats the purpose
   of specialist delegation. Keep agents focused on a coherent domain
3. **Skill hoarding**: Assigning every available skill to one agent — inflates context
   and dilutes specialization signals
4. **Model mismatch**: Using a lightweight model for complex reasoning tasks, or an
   expensive heavy model for trivial formatting tasks
5. **Missing boundaries**: Description that only says what the agent does but not what
   it does NOT do — leads to misrouting and failed delegations
6. **Name-role mismatch**: Name says `python-engineer` but role says "Full-Stack Developer"
   — confuses the routing logic

---

## Creation Checklist

Before running `create_agent.py`, confirm:

- [ ] Ran `list_agents.py` — confirmed no duplicate name exists
- [ ] Ran `list_skills.py` — identified relevant skills for this domain
- [ ] Ran `list_models.py` — selected appropriate model for the task type
- [ ] `name` is lowercase-hyphenated, unique, and reflects the role
- [ ] `role` is concise (~5 words), includes seniority if relevant
- [ ] `description` covers responsibilities, domains, quality standards, AND boundaries (<1024 chars)
- [ ] `model` matches task complexity and domain requirements
- [ ] `skills` list is minimal — only domain-relevant skills included
