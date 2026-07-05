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

## Principles

### Hypothetical Options First

When collecting requirements, do NOT ask open-ended questions. Instead:

0. **If the user gives no specifics** (e.g., "create a skill" without detail),
   default to extracting content and experience from the current conversation
   as the skill's subject matter. Capture patterns, knowledge, or workflow shown.
   Skip to [Prerequisite](#prerequisite-collect-required-information).
1. **Interpret intent** and generate 2-4 specific hypothetical options
2. **Present for confirmation** — let the user pick or refine
3. Only ask open-ended if none of the options fit

**Example**: User says "I need a skill for working with databases":

> I can create a database skill. Which type fits best?
>
> - **SQL Reviewer** — checks queries for correctness, performance, and injection risks
> - **Schema Designer** — helps design tables, indexes, and migrations
> - **Query Optimizer** — suggests indexes and rewrites for slow queries
> - **Other** — describe your needs

### Ground in Real Expertise

Effective skills come from domain-specific knowledge, not generic advice.
Source material can come from:

- **Current conversation** — extract corrections the user made, steps that worked,
  conventions the agent didn't already know, input/output data shapes
- **Project artifacts** — internal docs, API specs, code review comments, runbooks,
  incident reports, version control history (patches reveal real patterns)
- **Domain knowledge** — schemas, failure modes, configuration files, edge cases

If the user hasn't provided enough context, ask for relevant source material.
The skill is only as good as the context it's built from.

### Spend Context Wisely

Once loaded, the full SKILL.md enters the agent's context alongside everything else.
Every token competes.

- **Add what the agent lacks, omit what it knows** — don't explain what a PDF is
- **Prefer procedures over declarations** — teach *how to approach*, not *what to produce*
- **Provide defaults, not menus** — pick one approach, mention alternatives briefly
- **Match specificity to fragility** — prescriptive for fragile ops, flexible for creative tasks

## Prerequisite: Collect Required Information

Before writing, verify ALL of the following are clear. Use hypothetical options to clarify.

### (a) Skill Name

Lowercase-hyphen, noun-based, unique in registry. Example: `git-commit-helper`.

### (b) Trigger Condition → `description` field

When should the skill activate? What user query patterns indicate relevance?
This drives LLM routing.

### (c) Work Scope & Boundaries

What does the skill handle? What is OUT of scope? Output format?

### (d) Required Tools → `allowed-tools` field

Which MindX tools does the skill need? Keep the list minimal.

### (e) Runtime Requirements → `metadata.requires`

Does the skill need executables on PATH (e.g. `python3`, `git`)?
Does it need environment variables (e.g. `API_KEY`)?

## Workflow

### Design

#### Step 1: Check for Existing Skills

```bash
mindx skill list --json
```
Check if a skill with the same name or overlapping domain exists.
If yes, inform the user and let them decide.

```bash
mindx skill get <proposed-name>
```

#### Step 2: Confirm Requirements

Walk through all items in [Prerequisite](#prerequisite-collect-required-information).
Do NOT proceed until (a) through (e) are clear.

### Create

#### Step 3: Create the Skill Directory

```
<skill-name>/
  SKILL.md
```

#### Step 4: Read Schema Reference

Read `references/schemas.md` for full frontmatter schema details.

#### Step 5: Write SKILL.md

Write the skill body. Key focus areas:

**`description` field** (controls triggering):
- Use imperative phrasing: "Use this skill when..."
- Focus on user intent, not implementation
- Err on the side of being pushy about when the skill applies
- Keep under 1024 characters

**`metadata.requires`**: Declare needed bins and env vars so the runtime
skips the skill if the environment doesn't meet requirements.

**Workflow**: Numbered steps with concrete, executable instructions.

**Gotchas section**: The highest-value content in many skills. Document
corrections to mistakes the agent will make without being told:

```markdown
## Gotchas

- The `users` table uses soft deletes. Queries must include
  `WHERE deleted_at IS NULL`.
- The `/health` endpoint returns 200 even if the DB is down;
  use `/ready` for full health check.
```

When testing reveals an agent mistake, add the correction as a gotcha.

**Plan-Validate-Execute** pattern for destructive or batch operations:

```markdown
1. Create a plan in `plan.json`
2. Validate: `script/validate.py plan.json`
3. If validation fails, revise and re-validate
4. Execute: `script/apply.py plan.json`
```

**Checklists** for multi-step workflows to track progress.
**Validation loops**: "do work → validate → fix → re-validate → proceed."

For designing scripts the skill bundles, see [Script Design Guidelines](#script-design-guidelines).

### Install

#### Step 6: Install the Skill

```bash
mindx skill add <path-to-skill-directory>
```

Verify installation:

```bash
mindx skill get <skill-name>
```

#### Step 7: Validate

```bash
mindx skill validate <skill-name>
```

This catches frontmatter errors using the same loader as the daemon.

### Optimize Description

The `description` field determines whether the skill triggers. Optimize it systematically.

#### Step 8a: Create Trigger Eval Queries

Create `evals/trigger_queries.json` with ~20 queries:

```json
[
  { "query": "review this SQL query for injection risks", "should_trigger": true },
  { "query": "what's the weather today?", "should_trigger": false }
]
```

- **Should-trigger**: vary by phrasing, explicitness, detail level
- **Should-not-trigger**: use near-misses — prompts that share keywords but need a different skill
- Split into **train (60%)** and **validation (40%)** to prevent overfitting

#### Step 8b: Test Trigger Rate

Attach the skill to an agent. For each query, run 3 times and observe
whether the `Skill` tool was invoked. A should-trigger query passes if
trigger rate >= 0.5. A should-not-trigger passes if rate < 0.5.

#### Step 8c: Optimize

- Should-trigger failing → description too narrow — broaden scope
- Should-not-trigger false-triggering → description too broad — add specificity
- Avoid adding specific keywords from failed queries (overfitting)
- Re-test with train set, then check validation set for generalization

Iterate until train queries pass or improvement plateaus (~5 iterations).

### Evaluate Output Quality

#### Step 9a: Create Test Cases

Create `evals/evals.json` with 2-3 test cases:

```json
{
  "skill_name": "my-skill",
  "evals": [
    {
      "id": 1,
      "prompt": "Realistic user prompt with file paths and details...",
      "expected_output": "Description of what success looks like",
      "files": ["evals/files/input.csv"],
      "assertions": [
        "The output includes a chart image",
        "Both axes are labeled"
      ]
    }
  ]
}
```

Use realistic context (file paths, column names). Cover edge cases.

#### Step 9b: Run Baseline Comparison

Run each test case twice — **with the skill** and **without it**.
Save outputs to `evals/workspace/iteration-N/eval-ID/{with,without}_skill/`.
Record tokens and duration for each run.

#### Step 9c: Grade Assertions

Evaluate each assertion as PASS or FAIL with specific evidence:

```json
{
  "assertion_results": [
    { "text": "Output includes a chart", "passed": true, "evidence": "Found chart.png" },
    { "text": "Both axes are labeled", "passed": false, "evidence": "Y-axis labeled, X-axis missing" }
  ],
  "summary": { "passed": 1, "failed": 1, "total": 2, "pass_rate": 0.5 }
}
```

Use LLM judgment for subjective checks, scripts for mechanical checks
(file exists, valid JSON, row count).

#### Step 9d: Aggregate

Compute pass rate, token cost, and duration delta between with-skill
and without-skill. A skill that adds 50% pass rate for minor token
overhead is valuable.

Remove assertions that pass in both configs (not informative).
Investigate assertions that always fail (broken or too hard).

### Iterate

#### Step 10: Improve from Signal

Three signal sources:

- **Failed assertions** — specific gaps: missing step, unclear instruction
- **Execution transcripts** — *why* things went wrong: ambiguous instruction,
  unnecessary steps, wasted work
- **Trigger failures** — wrong queries triggered or missed

Feed all signals plus the current SKILL.md to an LLM for improvements:

- Generalize from feedback — fix underlying issues, not narrow patches
- Keep the skill lean — fewer, better instructions beat exhaustive rules
- Explain the why — reasoning-based instructions work better than rigid directives
- Bundle repeated work — if the agent writes the same helper each run,
  move it into `scripts/`

After changes, re-install and re-run relevant phases. Stop when feedback
is empty or improvement plateaus.

## File Structure Conventions

```
<skill-name>/
  SKILL.md              # Required. The skill definition.
  scripts/              # Optional. Reusable scripts for agentic use.
    validate.sh
    process.py
  references/           # Optional. Schemas, examples, reference docs.
    schemas.md
  evals/                # Optional. Test cases and evaluation artifacts.
    trigger_queries.json
    evals.json
    files/              # Test input files
    workspace/          # Eval run outputs
```

## Script Design Guidelines

Skills can bundle scripts in `scripts/` that the agent runs during execution.

### One-Off Commands

When an existing package does what's needed, reference it directly in SKILL.md:

```bash
npx eslint@9 --fix .
uvx ruff@0.8.0 check .
go run golang.org/x/tools/cmd/goimports@v0.28.0 .
```

Pin versions. State prerequisites (Node.js 18+, Python 3.10+).

### Self-Contained Scripts

Scripts can declare dependencies inline — no separate manifest:

**Python (PEP 723)** — run with `uv run`:
```python
# /// script
# dependencies = ["beautifulsoup4>=4.12,<5"]
# ///
from bs4 import BeautifulSoup
```

**Deno** — `npm:` import specifiers:
```typescript
#!/usr/bin/env -S deno run
import * as cheerio from "npm:cheerio@1.0.0";
```

**Bun** — version in import path:
```typescript
#!/usr/bin/env bun
import * as cheerio from "cheerio@1.0.0";
```

### Designing for Agentic Use

- **No interactive prompts** — agents can't TTY. All input via flags/env/stdin
- **`--help` is the primary interface** — description, flags, examples
- **Structured output** — JSON over free-form text; data on stdout,
  diagnostics on stderr
- **Meaningful exit codes** — distinct codes for different failure types
- **Idempotency** — "create if not exists" safer than "fail on duplicate"
- **Dry-run support** — `--dry-run` for destructive operations
- **Predictable output size** — truncate or use `--offset` for large output

## Writing Style & Patterns

- **Direct and imperative**: "Do X", "Check Y", "Return Z"
- **Specific over vague**: "List the files" is better than "Handle the files"
- **Example-driven**: Include examples for input/output formats
- **Progressive**: Most important instructions first; details later
- **Templates for output** — provide markdown or JSON templates.
  Agents pattern-match better against concrete structures than prose
- **Checklists** — track progress with `- [ ]` items
- **Validation loops** — do work → validate → fix → re-validate
- **Gotchas** — document specific mistakes the agent will make without instruction
- **Progressive disclosure** — move deep reference material to separate files
  in `references/` and tell the agent *when* to load each one

## Anti-Patterns

- Generic names (`helper`, `utils`, `assistant`)
- Descriptions that are marketing copy instead of routing signals
- Skills that try to do everything — split them
- Missing boundaries — leads to misrouting and overstepping
- Declaring tools in `allowed-tools` the skill never uses
- Overly prescriptive — let the agent exercise judgment for flexible tasks
- Generic skills with no domain-specific context
- Too many equal options without a clear default
- Overfitting the description to specific test queries instead of generalizing

## Important Notes

- **All fields are for LLM consumption unless stated otherwise.** Write clearly.
- **Skills are operating instructions**, not feature flags.
- **Less is more.** A focused skill beats a broad one.
- **Propose options before asking open-ended questions** — speeds up requirements.
- **Test before declaring done.** Validation + trigger test + at least one manual eval.
- **Description optimization ≠ output evaluation.** A skill can trigger correctly
  but produce bad output, or produce good output but never trigger. Test both.
- **Start small.** 2-3 test cases for the first eval round. Expand as you go.
- **Run each eval with clean context** — no leftover state from prior runs.
