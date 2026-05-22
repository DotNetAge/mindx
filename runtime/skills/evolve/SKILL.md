---
name: evolve
description: >
  Analyzes conversation archives to extract recurring workflows and user preferences.
  Use this when the user says "进化", "反省", "学习", "提取模式", "evolve", "自我提升",
  "总结工作经验", or otherwise asks you to learn from past conversations and improve yourself.
  Trigger proactively when you notice the user is repeatedly doing similar multi-step tasks
  — you can suggest evolving those patterns into reusable skills.
allowed-tools: bash glob grep read write
---

# Evolve — Self-Improvement Through Reflection

Extract experience from past conversations. Each evolve run produces two kinds of knowledge:

| Output | Purpose | Storage | Memory pipeline |
|--------|---------|---------|-----------------|
| **Workflow skills** | Repeated multi-step operations as reusable SKILL.md | `$MINDX_WORKSPACE/skills/evolved/<name>/SKILL.md` | Loaded by FileSystemSkillLoader on next agent load |
| **User preferences** | Coding style, tool habits, communication patterns | `$MINDX_WORKSPACE/evolved/preferences.md` | FileWatchService indexes into LongTerm RAG memory → retrievable via MemorySearch |

**Memory pipeline:** When you write to `preferences.md`, the Daemon's `FileWatchService` detects the change → indexes content into the LongTerm `HybridIndexer` (vector + fulltext). Future sessions find these preferences via `MemorySearch` — no manual reload needed.

Every evolve run is "past-you teaching future-you." You are both the analyst and the beneficiary.

---

## Phase 1: Gather Session Data

### List analyzable sessions

The LLM knows the correct paths from the SystemPrompt — always pass them explicitly.

```bash
python scripts/evolve list --sessions-dir <workspace>/sessions
```

Output example:

```
SESSION ID              AGENT        MSGS   SIZE     LAST ACTIVE
sess_a1b2c3d4           developer    47     12.3 KB  2026-05-21 14:32
sess_e5f6g7h8           developer    23     5.1 KB   2026-05-20 09:15
```

**Selection strategy:**
- Default: analyze the 3-5 most recent active sessions (preferring those with more messages)
- If the user specifies a session ID, analyze only that one
- Skip sessions with fewer than 5 messages — insufficient data for pattern extraction

### Get session content — progressive disclosure

The script defaults to **outline mode** (one line per message) to protect context budget. Read the outline to understand the conversation flow, then drill down.

**Step 1 — Outline (default):**

```bash
python scripts/evolve get <session-id> --sessions-dir <workspace>/sessions
```

Output shows a compact header (message count, role distribution, tools used) then one line per message:

```
Session: sess_a1b2c3  |  Agent: developer
Messages: 47 (23 user, 24 assistant)  |  09:15 ~ 14:32
Tools: grep(12), read(8), edit(5), bash(6)

#1   09:15 user     Fix the login validation bug in auth.go
#2   09:16 assistant [grep] Searching for the login function...
#3   09:18 assistant [read] Reading auth.go...
...
```

**Step 2 — Drill down into a specific section:**

```bash
python scripts/evolve get <session-id> --range 5-10 --mode expand --sessions-dir <workspace>/sessions
```

This shows full decoded content for messages 5-10 only, truncated at 1500 chars per message.

**Step 3 — Single message:**

```bash
python scripts/evolve get <session-id> --msg 3 --sessions-dir <workspace>/sessions
```

Shows exactly one message in full detail.

**Why progressive disclosure?** A 200-message session decoded at 2000 chars each = 400K chars into context. Outline mode uses ~100 chars per message = 20K. Only expand what you need.

---

## Phase 2: Pattern Analysis

Use your own reasoning to analyze the conversation. Ask yourself three questions:

### Question A: Are there recurring workflows?

Look for **multi-step action sequences that appear more than once**. Example:

```
User: "fix this function for me"
  → you grep to find the function location
  → read the file
  → write the fix
  → bash to test
  → Later, user says: "fix another function"
  → same grep → read → write → test flow
```

This pattern deserves extraction.

**Judgment criteria:**
- Same sequence appears **≥ 2 times** to count as a pattern
- Minimum 3 steps with a clear order
- Trigger scenario is well-defined (e.g., "whenever the user asks to modify code")

### Question B: Are there user preferences?

Look for **consistent behavioral tendencies**. Examples:

- "You always run tests before committing" → preference: tests must pass before commit
- "User emphasized tabs over spaces multiple times" → preference: tab indentation
- "Every time I explain something, user says 'keep it simple'" → preference: concise explanations

**Judgment criteria:**
- Same preference appears **≥ 2 times** (possibly in different wording)
- Don't extract overly generic conclusions ("the user likes AI-assisted coding")
- Annotate each preference with confidence 0.0–1.0; skip anything below 0.7

### Question C: Are there capability gaps?

Notice user requests that recur but none of your current skills cover. For example, the user keeps asking "walk me through this code" but you don't have a code-review skill — that's a capability gap worth flagging.

---

## Phase 3: Generate Skills

For each workflow with confidence ≥ 0.7:

### 3.1 Dedup check

```bash
python scripts/evolve check <workflow-name> --skills-dir <workspace>/skills
```

Returns `{"exists": true}` if a skill with that name already exists → skip and report.

Get a complete list of previously evolved skills for reference:

```bash
python scripts/evolve dedup --skills-dir <workspace>/skills
```

### 3.2 Create skill directory and SKILL.md

Use the `write` tool to create the file:

```markdown
---
name: evolved-<workflow-name>
description: >
  [Auto-evolved] One-sentence description of what this workflow does and when it triggers.
  Include triggering user intents so the skill gets matched reliably.
allowed-tools: [space-separated tool names]
---

# Evolved: [Workflow Name]

Auto-generated by `evolve` skill on [YYYY-MM-DD].
Based on analysis of sessions: [session-id], etc.

## Trigger
[When does this workflow activate?]

## Steps
1. [Step 1]
2. [Step 2]
3. [Step 3]
```

**Naming conventions:**
- Lowercase, hyphen-separated
- Prefix `evolved-` to distinguish auto-generated from hand-written skills
- Name should reflect the trigger scenario (e.g., `evolved-pr-review-flow`)

**Tool selection:**
- Only list tools actually used in the workflow
- Don't over-equip — unnecessary tools waste context

**Instructions quality:**
- Explain **why** each step matters, not just what to do
- Include trigger context so future-you knows when to use this

---

## Phase 4: Store Preferences

For each preference with confidence ≥ 0.7, write to the preferences archive.

Ensure the directory exists:

```bash
mkdir -p <workspace>/evolved
```

Then use `write` to append to `<workspace>/evolved/preferences.md`.

Each entry follows Markdown format with timestamp, category, and confidence:

```markdown
## 2026-05-21

- **coding_style**: Tab indentation (confidence: 0.95)
- **workflow**: Always run tests before modifying code (confidence: 0.88)
- **communication**: Prefer concise answers without explaining fundamentals (confidence: 0.82)
```

Follow the format `- **category**: description (confidence: 0.XX)` for consistency.

**Memory integration:** These entries are automatically indexed into LongTerm RAG memory. The Daemon's `FileWatchService` monitors `<workspace>/evolved/` — when you write to `preferences.md`, the `ProjectIndexer` detects the change and syncs it into the `HybridIndexer` (vector + fulltext). Future agents retrieve these preferences via `MemorySearch` without any manual import step.

Format notes for optimal retrieval:
- Use `## YYYY-MM-DD` date headings — the indexer chunks by section boundaries
- Keep each bullet as a self-contained statement — the chunk might be retrieved in isolation
- Include `(confidence: 0.XX)` — the fulltext index makes scores searchable
- Use consistent category keys (`coding_style`, `workflow`, `communication`, `tool_preference`, `domain_knowledge`) — enables filtering by tag later

---

## Phase 5: Report

Present the evolution results clearly to the user.

### When new patterns are found

```
Evolution complete!

New skills (2):
  evolved-pr-review-flow  — PR review checklist workflow (confidence 0.92)
  evolved-bug-hunt-flow   — Debugging standard procedure (confidence 0.85)

User preferences (3):
  coding_style:    Tab indentation
  workflow:        Test before code changes
  communication:   Prefer concise answers

Tip: Run `introspect` to check whether these new skills match your role.
```

### When nothing is found

```
No significant patterns detected. Need more conversation data (at least 3-5 complete
dialog rounds) for meaningful analysis.
```

---

## Edge Cases

| Situation | Handling |
|-----------|----------|
| No session data | Report "need conversations before evolution" |
| Too few messages (< 5) | Skip, explain why |
| All workflows already exist | Report "no new patterns found" |
| Concurrent writes to preferences.md | Append-only, no conflict |
| User interrupts mid-analysis | Stop and respond to user first |

---

## Anti-Patterns

- **Don't regenerate existing skills** — always check with `evolve check` first
- **Don't overwrite built-in skills** in `mindx/runtime/skills/` — only write to `evolved/`
- **Don't over-extract** — two occurrences is a trend, three+ is a pattern. Be conservative
- **Don't assume tool availability** — if an extracted workflow depends on specific tools (internal APIs, platform-specific CLIs), note prerequisites in the generated skill's description
- **Don't record trivial preferences** — "user likes AI" is useless. Record specific tendencies that help future-you serve the user better
