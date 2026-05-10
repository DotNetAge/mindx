# MindX Skill Writing Guideline

Authoritative reference for writing Skills in MindX.
Based on analysis of production-quality Skills: **humanizer**, **file-organizer**, **lead-research-assistant**, **meeting-insights-analyzer**.


## 1. What Is a Skill (and What It Isn't)


### A Skill IS:

- A **pure task instruction** (Prompt) that tells an LLM what to do, how to do it well, and what to avoid
- Read by GoReAct's **T (Thought) phase** — the LLM reads it and reasons about what actions to take
- The **only place** where domain-specific logic, quality rules, and workflow definitions live
- Written in **English** for cross-platform compatibility


### A Skill is NOT:

- A role definition → Agent's SystemPrompt handles identity and role
- A framework explanation → GoReAct's T-A-O loop is infrastructure, not content
- A tool call tutorial → `allowed-tools` in YAML frontmatter declares tools; LLM knows how to call them
- Code → Tools (Go code implementing FuncTool) handle executable logic
- Documentation → It's a working Prompt, not a readme


## 2. Architecture Context


```
┌─────────────────────────────────────────────────────┐
│                   GoReAct Engine                    │
│                                                     │
│   T (Thought) ← reads SKILL.md → reasons internally │
│        │                                            │
│   A (Action)  ← decides which Tool to call          │
│        │                                            │
│   O (Observation) ← receives Tool results           │
│        │                                            │
│        └──→ loops until done                        │
└─────────────────────────────────────────────────────┘

SKILL.md = input to T phase (Prompt text)
Tools = called during A phase (Go code executing FuncTool)
GoGraph = persistence layer (transparent to Skill)
```

**Key implication**: The Skill body never needs to say "call this tool" or "this is your thought phase." The LLM reads the Skill, reasons about it (T), and decides what Action to take based on `allowed-tools` declaration + Skill instructions.


## 3. Mandatory Structure


Every Skill MUST follow this structure:

```markdown
---
name: skill-name
description: >
  When to use this skill (1-3 sentences). Include trigger phrases.
  
  Examples:
  <example>
  Context: [scenario]
  [who]: "[what happened]"
  [response]: "[what agent does]"
  </example>
allowed-tools: [tool1] [tool2]
metadata:
  version: "X.Y.Z"
  category: core|domain|utility
  author: mindx-core-team|...
---

# Skill Title

One-paragraph summary of what this Skill does.

## When to Use This Skill

- [Natural language scenario 1]
- [Natural language scenario 2]
- [Natural language scenario 3]
- ...

## What This Skill Does

1. **[Capability 1]** — Brief description
2. **[Capability 2]** — Brief description
3. **[Capability 3]** — Brief description
...

## How to Use

```
[Example user prompt 1]
```

```
[Example user prompt 2]
```

```
[Example user prompt 3]
```

## Instructions

### 1. [Step Name: Bold Action Title]

[Detailed instructions for this step. Natural language. Tell the LLM what to do,
how to think about it, what to consider.]

#### [Sub-section if needed]

[Quality rules with Before/After examples:]

**Bad:**
> [Example of wrong approach]

**Better:**
> [Example of correct approach]

### 2. [Step Name: Bold Action Title]

[Continue with next step...]

### N. [Final Step Name]

[Last step...]

## Examples

### Example 1: [Descriptive Name]

**User:** "[What user said/did]"

**Process/Output:** [What happens, step by step or final output format]

### Example 2: [Another Scenario]

**User:** "[...]"

**Process/Output:** [...]

## Pro Tips

1. **[Tip 1]** — [Explanation]
2. **[Tip 2]** — [Explanation]
...

## Common [Task/Skill] Requests

```
[Prompt template 1]
```

```
[Prompt template 2]
```

## Related Use Cases

- **[skill-name]** — [How it relates]
- **[skill-name]** — [How it relates]
...

## References

- See [references/xxx.md](references/xxx.md) for details
```


## 4. Section-by-Section Rules


### 4.1 YAML Frontmatter

| Field | Required | Rules |
|-------|----------|-------|
| `name` | YES | lowercase-hyphenated identifier |
| `description` | YES | 1-3 sentences. Must include trigger phrases. May include `<example>` blocks |
| `allowed-tools` | YES | List of tool names this Skill may use. Never empty — if no external tools needed, list relevant built-in tools |
| `metadata.version` | YES | Semantic versioning |
| `metadata.category` | YES | core / domain / utility |
| `metadata.author` | YES | Team or individual |

**Rules:**
- `description` is the PRIMARY activation signal. Make trigger phrases clear and specific.
- `allowed-tools` is the ONLY place tools are declared. Never mention tools in the body.
- No `llm-think`, no framework references, no role definitions in frontmatter.


### 4.2 ## When to Use This Skill

**Purpose**: Tells the LLM (and humans reading the Skill) when this Skill should activate.

**Rules:**
- Write as **natural language scenarios**, not technical conditions
- Use 5-10 bullet points covering different trigger contexts
- Include both user-initiated and system-triggered scenarios
- Be specific enough for pattern matching but not overly technical

**Good:**
```
- The user sets a new goal that needs breaking down into tasks
- Someone asks to "plan" or "decompose" a project
- A goal feels too vague to act on directly ("improve performance")
```

**Bad:**
```
- When WPS decomposition is required
- If goal.status == NEW and goal.type == OBJECTIVE
```


### 4.3 ## What This Skill Does

**Purpose**: High-level capability overview. Numbered list of what the Skill accomplishes.

**Rules:**
- 3-7 items maximum
- Each item: **Bold action verb** — one-line description
- Covers full scope without diving into implementation


### 4.4 ## How to Use

**Purpose**: Example user prompts that would trigger this Skill. Serves as pattern-matching reference for the LLM.

**Rules:**
- 3-6 example prompts in code blocks
- Show variety: formal, casual, specific, vague
- These are NOT usage instructions — they're **activation patterns**

**Why this matters**: The LLM sees these patterns and learns to recognize when users are asking for this Skill's capability, even if they don't use exact trigger words.


### 4.5 ## Instructions

**Purpose**: The core of the Skill. Step-by-step instructions telling the LLM exactly what to do.

**Rules:**

**Structure:**
- Numbered steps (1, 2, 3... N)
- Each step has **bold action title** as heading
- Detailed natural-language instructions under each heading
- Sub-sections (####) for quality rules, examples, decision frameworks within a step

**Content style:**
- Write like you're explaining to a smart colleague who hasn't done this before
- Include **decision criteria** ("if X then Y, otherwise Z")
- Include **quality rules** with **Before/After examples** for common mistakes
- Include **edge cases** and what to do about them
- Be specific about outputs: what format, what fields, what structure

**What goes here vs What doesn't:**
- ✅ How to analyze, evaluate, decide, classify
- ✅ What good output looks like vs bad output
- ✅ Decision frameworks and criteria
- ❌ "Call graph-query tool now"
- ❌ "This is your Thought phase"
- ❌ Framework explanations (T-A-O, ReAct, etc.)

**Before/After pattern for quality rules:**

```markdown
#### Common [Something] Mistakes

**Too [adjective]:**
> [Bad example]

**Better ([adjective]):**
> [Good example]
```

This pattern appears throughout humanizer, file-organizer, meeting-insights-analyzer, and all our v3.0 Skills. It's the standard way to teach quality.


### 4.6 ## Examples

**Purpose**: Demonstrate the Skill in action with real scenarios.

**Rules:**
- Minimum 2 examples, ideally 3+
- Each example shows: User input → Process → Output
- Cover different complexity levels (simple, medium, complex)
- Include at least one edge case or error scenario
- Show actual output format (JSON, markdown table, etc.) not just description

**Example types to include:**
1. **Happy path**: Normal successful execution
2. **Edge case**: Vague input, error recovery, partial success
3. **Complex scenario**: Multiple steps, interactions with other Skills


### 4.7 ## Pro Tips

**Purpose**: Hard-won practical wisdom that doesn't fit into step-by-step instructions.

**Rules:**
- 5-10 tips maximum
- Each tip: **Bold summary** — Explanation
- Tips should be non-obvious insights from experience
- Not a rehash of Instructions section — these are the "pro level" additions


### 4.8 ## Common [X] Requests

**Purpose**: Quick-reference prompt templates. Users (and LLMs) can scan this to see typical invocations.

**Rules:**
- 4-8 prompt templates in code blocks
- Short, varied, covering main use cases
- Different from `## How to Use`? Slightly — this is more of a cheat sheet / quick reference


### 4.9 ## Related Use Cases

**Purpose**: Shows how this Skill connects to the broader ecosystem.

**Rules:**
- List related Skills and how they interact
- Note data flow between Skills
- Mention what triggers calls TO other skills
- Mention what this skill produces FOR other skills


### 4.10 ## References

**Purpose**: Link to deeper documentation.

**Rules:**
- 1-3 links maximum
- Only link to documents that actually exist or are planned
- Format: `- See [path](path) for [brief description]`


## 5. Anti-Patterns (Never Do These)


### 5.1 Role Definition in Skill

**Wrong:**
```markdown
## Role
You are a task decomposition engine that breaks down goals...
```

**Why**: Agent already has a SystemPrompt defining its role. Skill defines **tasks**, not identity.

**Fix**: Remove role definition. Start with `# Title` + one-paragraph summary.


### 5.2 Framework Explanation in Skill

**Wrong:**
```markdown
This skill operates within the ReAct T-A-O loop:
- T (Thought): Your internal reasoning...
- A (Action): Call graph-query tool...
```

**Why**: T-A-O is GoReAct's execution mechanism. Skill is the INPUT to T phase. Explaining the framework to the LLM is like explaining how an engine works to a driver — unnecessary and confusing.

**Fix**: Remove all framework mentions. Just tell the LLM what to do.


### 5.3 Tool Call Instructions in Body

**Wrong:**
```markdown
**Persistence Action** (call graph-query tool):
CREATE (g:Goal { ... })
```

**Why**: `allowed-tools` in YAML frontmatter declares available tools. The LLM knows how to call them. Repeatedly reminding it to "call the tool" adds noise and suggests the LLM wouldn't know to persist data otherwise.

**Right approach**: If Cypher/data schemas are needed for reference, put them in a dedicated `#### Graph Schema Reference` subsection as informational reference material, not as instructional "call this now" commands.


### 5.4 "Analysis Process" / "Persistence Action" Split

**Wrong:**
```markdown
**Analysis Process**:
Analyze the goal...

**Persistence Action**:
Call graph-query to store result...
```

**Why**: This split is a relic of misunderstanding the T-A-O architecture. It artificially divides natural reasoning into "thinking" vs "acting" when the LLM does both fluidly. It also reintroduces framework awareness through the back door.

**Fix**: Write natural step-by-step instructions. Let the LLM reason and decide when to call tools organically.


### 5.5 Vague or Empty Content

**Wrong:**
```markdown
## Instructions
1. Do the thing
2. Check the result
3. Return output
```

**Why**: A Skill with generic instructions forces the LLM to guess. The whole point of a Skill is to encode domain expertise so the LLM doesn't have to figure it out from scratch.

**Fix**: Every step should have detailed guidance, quality rules, examples, and decision criteria.


## 6. Quality Checklist


Before considering a Skill complete, verify:

### Must Have (Blocking)

- [ ] YAML frontmatter with name, description (with examples), allowed-tools, metadata
- [ ] `## When to Use This Skill` with 5+ natural language scenario bullets
- [ ] `## What This Skill Does` with 3-7 numbered capability items
- [ ] `## How to Use` with 3-6 example user prompts
- [ ] `## Instructions` with numbered steps, each having bold title + detailed content
- [ ] `## Examples` with 2+ complete scenarios showing input → process → output
- [ ] `## Pro Tips` with 5+ practical tips
- [ ] `## Common X Requests` with 4+ prompt templates
- [ ] `## Related Use Cases` listing connected Skills
- [ ] Zero occurrences of "call [tool]", "use [tool] tool", "this is your thought phase", "T-A-O", "ReAct loop"
- [ ] Zero role definitions ("You are a...", "As a...")
- [ ] Written entirely in English

### Should Have (Strongly Recommended)

- [ ] Before/After quality rule examples in at least 2 instruction steps
- [ ] Error handling covered in Instructions (what to do when things go wrong)
- [ ] At least one edge-case example in Examples section
- [ ] Output format specification (JSON schema, markdown template, etc.)
- [ ] Decision tables or frameworks where applicable
- [ ] Quality rules section (thresholds, scoring criteria, etc.)

### Nice to Have

- [ ] Data/schema reference appendix (for Skills that use GoGraph)
- [ ] Lifecycle state diagram (for Skills managing entities with state)
- [ ] Quick reference table (for Skills with many options/modes)


## 7. Reference Skills

These Skills exemplify the standard. Study them before writing your own:

| Skill | Strengths to Study |
|-------|-------------------|
| **humanizer** | Pattern-based rules with Before/After examples; soul/personality guidance; comprehensive coverage list |
| **file-organizer** | Clear When/What/How structure; multiple scenario examples; Pro Tips + Best Practices separation |
| **lead-research-assistant** | Clean numbered instructions; output format template; Tips for Best Results |
| **meeting-insights-analyzer** | Multi-dimensional analysis framework; timestamped examples with quotes; Setup Tips + Best Practices |

Our v3.0 Skills (**wps-decompose**, **find-experts**, **self-reflection**) follow this same standard and serve as additional reference implementations for core MindX Skills.


## 8. File Conventions


| Convention | Rule |
|-----------|------|
| Location | `runtime/skills/{skill-name}/SKILL.md` |
| Naming | lowercase-hyphenated directory and name |
| Language | English only (for cross-platform compatibility) |
| Encoding | UTF-8 |
| Line length | No hard limit, but prefer <120 chars for readability |
| No subdirectories | All skills flat under `runtime/skills/` |
| Version | Start at 1.0.0, increment per significant rewrite |
