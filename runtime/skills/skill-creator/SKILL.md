---
name: skill-creator
description: >
  Creates and improves MindX skills. Use when the user wants a new reusable
  capability, when an existing skill needs refinement, or when you need to
  structure domain knowledge into a skill that can be attached to agents.
metadata:
  name_zh: 技能创建者
  name_zh-tw: 技能建立者
  description_zh: 创建和改进 MindX 技能，将领域知识封装为可挂载到智能体的复用能力
  description_zh-tw: 建立和改進 MindX 技能，將領域知識封裝為可掛載到智慧體的複用能力
---

# Skill Creator

Create and improve MindX skills.

## When to Use

- User says "I need a skill for X", "create a skill that does X", "help me write a skill"
- A workflow would benefit from reusable instructions that can be attached to multiple agents
- An existing skill is unclear, triggers incorrectly, or needs better boundaries

**Do NOT use** when a suitable skill already exists.

## Guiding Principle: Hypothetical Options First

When collecting requirements, do NOT ask open-ended questions. Instead:

0. **If the user gives no specifics** (e.g., "帮我生成一个技能" without further detail),
   default to extracting the content and experience from the current conversation session
   as the skill's subject matter. The skill should capture the patterns, knowledge, or
   workflow demonstrated in this session. Skip to Prerequisite to collect remaining details.
1. **Interpret the user's intent** and generate 2-4 specific hypothetical options
2. **Present them for confirmation** — let the user pick or refine
3. Only ask open-ended if none of the options fit

**Example**: If the user says "I need a skill for working with databases", respond with:

> I can create a database skill. Which type fits best?
>
> - **SQL Reviewer** — checks queries for correctness, performance, and injection risks
> - **Schema Designer** — helps design tables, indexes, and migrations
> - **Query Optimizer** — suggests indexes and rewrites for slow queries
> - **Other** — describe your specific needs
>
> Or do you have something else in mind?

Apply this technique to all data collection below.

## Prerequisite: Collect Required Information

Before writing anything, verify that ALL of the following are clear. If any item is missing, use hypothetical options to clarify.

### (a) Skill Name

- Lowercase-hyphen format, noun-based, reflects the capability (e.g. `git-commit-helper`, `api-reviewer`)
- Must be unique within the skill registry

### (b) Trigger Condition

- When should this skill activate?
- What user query patterns indicate the skill is relevant?
- This becomes the `description` field and guides the LLM routing decision

### (c) Work Scope & Boundaries

- What specific tasks will the skill handle?
- What is explicitly OUT of scope?
- What output format or structure should it produce?
- This information feeds into the Markdown body

### (d) Required Tools

- Which MindX tools does the skill need? (e.g. `Read`, `Edit`, `Task`, `Bash`)
- List them as a space-separated string in `allowed-tools` so the runtime grants access when the skill is loaded
- Keep the list minimal — each additional tool adds context overhead

## MindX Skill Format

A MindX skill is a single Markdown file named `SKILL.md` with YAML frontmatter.

```markdown
---
name: <kebab-case-name>
description: >
  <One sentence for LLM routing: what this skill does and when to use it>
allowed-tools: Read Edit Task
metadata:
  name_zh: <中文名>
  description_zh: <中文描述>
---

## When to Use

- <trigger condition 1>
- <trigger condition 2>

## Workflow

### Step 1: <First action>

<Specific instructions>

### Step 2: <Second action>

<Specific instructions>

## Important Notes

- <boundary or quality rule>
- <boundary or quality rule>
```

### Frontmatter Fields

| Field                        | Required | Purpose                                                            |
| ---------------------------- | -------- | ------------------------------------------------------------------ |
| `name`                       | Yes      | Unique machine ID, lowercase-hyphen                                |
| `description`                | Yes      | For LLM routing — helps the agent decide when to invoke this skill |
| `allowed-tools`              | No       | Tools the skill is allowed to use when loaded                      |
| `metadata.name_zh`           | No       | Simplified Chinese display name                                    |
| `metadata.name_zh-tw`        | No       | Traditional Chinese display name                                   |
| `metadata.description_zh`    | No       | Simplified Chinese description                                     |
| `metadata.description_zh-tw` | No       | Traditional Chinese description                                    |

### Body Conventions

- Start with `## When to Use` — clear trigger conditions
- Use `## Workflow` with numbered steps
- End with `## Important Notes` — boundaries, anti-patterns, quality rules
- Use direct, imperative language
- Prefer constraints over bragging: say what the skill does NOT do
- Progressive disclosure: only load detailed instructions after the skill is invoked

## Workflow

### Step 1: Check for Existing Skills

```bash
mindx skill list --json
```

- If a skill with the same name or overlapping domain exists, inform the user and stop
- Show which existing skill overlaps and let the user decide

You can also check a specific name:

```bash
mindx skill get <proposed-name>
```

### Step 2: Create the Skill Directory

Create a new skill directory. The directory name must match the `name` frontmatter field. The daemon will manage where this directory lives in the user's environment.

```
<skill-name>/
  SKILL.md
```

### Step 3: Read the Schema Reference

Read `references/schemas.md` for the frontmatter schema.

### Step 4: Write SKILL.md

Use the MindX Skill Format above. Focus on:

- A routing description that is precise enough for the LLM to trigger correctly
- A workflow that is concrete and executable
- Clear boundaries so the skill does not overstep

### Step 5: Install the Skill

Copy the skill into the managed skill registry:

```bash
mindx skill add <path-to-skill-directory>
```

This validates the skill, installs it, and reloads the registry.

Verify the skill is loaded:

```bash
mindx skill get <skill-name>
```

### Step 6: Validate the Installed Skill

Run the built-in validator against the installed skill:

```bash
mindx skill validate <skill-name>
```

This uses the same loader as the daemon, so it catches frontmatter errors.

### Step 7: Test the Skill

Test by attaching the skill to an agent and running relevant queries.

For each test case, check:

- Did the agent invoke the `Skill` tool for this skill?
- Did the skill produce the expected output?
- Did the skill stay within its declared scope?

### Step 8: Iterate

Based on test results, refine:

- `description` — if triggering is wrong
- body workflow — if output is wrong
- `allowed-tools` — if the skill needs different tools
- boundaries in `## Important Notes` — if the skill oversteps

Re-validate and reload after each change.

## File Structure Conventions

A well-organized skill may include:

```
<skill-name>/
  SKILL.md              # Required. The skill definition.
  references/           # Optional. Schemas, examples, or reference docs.
    schemas.md
```

Keep the root minimal. Put detailed reference material under `references/`.

## Writing Style

- **Direct and imperative**: "Do X", "Check Y", "Return Z"
- **Specific over vague**: "List the files" is better than "Handle the files"
- **Constrained over broad**: Explicitly state what is NOT in scope
- **Example-driven**: Include examples for input/output formats
- **Progressive**: Put the most important instructions first; details later

## Anti-Patterns

- Names that are too generic (`helper`, `utils`, `assistant`)
- Descriptions that are marketing copy instead of routing signals
- Skills that try to do everything — split them instead
- Missing boundaries — leads to misrouting and overstepping
- Declaring tools in `allowed-tools` that the skill never uses

## Important Notes

- **All fields are for LLM consumption unless stated otherwise.** Write clearly and precisely.
- **Skills are operating instructions**, not feature flags. They tell the LLM what behaviors to activate.
- **Less is more.** A focused skill with a tight scope is more useful than a broad one.
- **Always propose options before asking open-ended questions.** This speeds up requirement gathering.
- **Test before declaring done.** At minimum, run the validation command and a few manual test queries.
