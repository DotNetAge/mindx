---
name: software-dev
description: >
  Plan and manage software product development — from requirements through
  architecture, sprint execution, quality gates, to release and monitoring.
  Supports agile workflows with cross-session persistence for ongoing projects.
allowed-tools: bash sub-agent collect-results task-create task-update task-get task-list team-create team-list team-get-tasks team-delete find-experts
metadata:
  name_zh: 软件开发
  name_zh-tw: 軟體開發
  description_zh: 规划和管理软件产品开发——从需求分析、架构设计、迭代执行、质量门禁到发布监控，支持敏捷工作流和跨会话持久化
  description_zh-tw: 規劃和管理軟體產品開發——從需求分析、架構設計、迭代執行、質量門禁到發布監控，支持敏捷工作流和跨會話持久化
---

## Trigger Decision

Use this skill when:

- User asks to build a software feature, module, or entire product
- User needs sprint planning, iteration management, or release coordination
- Task spans multiple sessions/sprints with dependencies between components
- User mentions "develop", "implement", "build", "sprint", "release", "refactor"

**Do NOT use** for single-file edits or trivial bug fixes (do those directly).

**Do NOT confuse with** `find-experts` — this skill manages the **development lifecycle itself**, while `find-experts` delegates **individual tasks** to specialists. They compose: `software-dev` uses `find-experts` Mode 2 for complex sprints.

## GraphRAG Integration

### Language Handling

> **CRITICAL: This SKILL.md is written in English for token efficiency and global compatibility, but you MUST query GraphRAG in the language matching the stored content.**

**Language detection rule:**
- User speaks/writes in **Chinese** → Query `mindx memory query` and `mindx graph query` in **Chinese first**, then English as fallback
- User speaks/writes in **English** → Query in **English first**, then Chinese as fallback
- When storing via `mindx memory store` → Store in the **same language as the user's current working language**
- Graph node `properties` values → Match the language of stored data
- Cypher string literals → Use the language stored in node properties

### Dual-Engine Architecture

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
| Build structured business state (projects, sprints, tasks with dependencies) | `mindx graph upsert-nodes/edges` (entity graph) |
| Query relationships between entities | `mindx graph query --cypher "..."` (Cypher traversal) |
| Update business state | `mindx graph exec --cypher "SET ..."` (mutation) |
| Cross-reference: entity → full context | Graph node → get source docs → `memory query` |

## Domain Context

This skill follows an **agile-inspired development model** adapted for AI-agent orchestration:

```
┌─────────────────────────────────────────────────────────────┐
│                    Development Lifecycle                     │
│                                                             │
│  Requirements ──→ Architecture ──→ Sprint Plan               │
│       (Phase 1)        (Phase 2)         (Phase 3)          │
│                                             │                │
│                                    ┌──────▼──────┐         │
│                                    │   Sprint    │◄────────┘
│                                    │  Execution   │── repeat
│                                    │  (Phase 4)   │
│                                    └──────┬──────┘         │
│                                           │                  │
│                              ┌────────────┼────────────┐     │
│                              ▼            ▼            ▼     │
│                         Quality      Release    Monitor     │
│                         Gate (5)     (Phase 6)  (Phase 7)  │
└─────────────────────────────────────────────────────────────┘
```

## Workflow

### Phase 1: Requirements — Understand What to Build

Gather requirements before writing any code. Use structured extraction:

| Ask | Why | Good Answer Example |
|-----|-----|---------------------|
| "What problem does this solve?" | Root understanding | "Users can't track their workout history across devices" |
| "Who are the users?" | Scope & personas | "Fitness enthusiasts using iOS/Android" |
| "What's the MVP scope?" | Prevent scope creep | "Sync + basic stats dashboard, social later" |
| "Any technical constraints?" | Architecture input | "Must work offline, existing Go backend" |
| "Definition of done?" | Acceptance criteria | "Data syncs within 5s, no data loss scenario" |

**Output: Requirements Brief**

```
Requirements Brief:
  Problem: <what pain point>
  Users: <who benefits>
  MVP Scope: [feature-1, feature-2, ...]
  Out of scope: [future-feature-a, ...]
  Constraints: [tech/business]
  Definition of Done: [testable criteria]
  Success Metric: <how we know it works>
```

### Phase 2: Architecture — Design Before Coding

Decompose the MVP into technical components:

```
Architecture Blueprint:
  Components:
    - <component-A>: <responsibility> → <tech choice>
    - <component-B>: <responsibility> → <tech choice>
    - <component-C>: <responsibility> → <tech choice>

  Data Model:
    - <entity-1>: {fields, relationships}
    - <entity-2>: {fields, relationships}

  API Surface:
    - POST /api/<resource> — <purpose>
    - GET /api/<resource>/:id — <purpose>

  Dependencies:
    - component-B depends on component-A (must build A first)
    - component-C is independent (can parallelize with B)
```

If the project is complex enough, delegate architecture design:

```
sub-agent(
  agent_name="architect",
  task="Design the architecture for: {requirements brief}. "+
       "Output: component diagram, data model, API endpoints, "+
       "dependency graph, tech stack rationale. Format as structured markdown."
)
collect-results(task_ids=[...])
→ Review output, adjust if needed
```

### Phase 3: Sprint Planning — Break Into Executable Tasks

Map architecture components to a sprint backlog. Each task must be **executable by a specialist agent in one session**.

**Sprint template:**

```
Sprint N: {sprint goal} ({start} → {end})

Wave 1 (no dependencies):
  Task: {name}          | Owner: {agent} | Est: {size} | Deps: none
  Task: {name}          | Owner: {agent} | Est: {size} | Deps: none

Wave 2 (depends on Wave 1):
  Task: {name}          | Owner: {agent} | Est: {size} | Deps: wave1-task-x

Wave 3 (depends on Wave 2):
  Task: {name}          | Owner: {agent} | Est: {size} | Deps: wave2-task-y
```

**Create tasks via TaskCreate, wire dependencies via TaskUpdate(addBlockedBy):**

```
# Create all tasks first
TaskCreate(subject="API: user sync endpoint", description="...")
TaskCreate(subject="Client: sync service layer", description="...)
TaskCreate(subject="Tests: sync integration tests", description="...)

# Wire: tests depend on both API and client
TaskUpdate(task_id=tests_task, addBlockedBy=[api_task, client_task])
```

For multi-sprint projects, also create a **release milestone** in the graph:

```bash
PROJ_ID=$(mindx utils uuid)
SPRINT_ID=$(mindx utils uuid)

mindx graph upsert-nodes --nodes '[{
  "id":"'"$PROJ_ID"'",
  "labels":["SoftwareProject"],
  "properties":{
    "name":"{project-name}",
    "status":"active","sprint":1,"progress":0.0,
    "requirements":"{req-brief-summary}"
  }
}]'
mindx graph upsert-nodes --nodes '[{
  "id":"'"$SPRINT_ID"'",
  "labels":["Sprint"],
  "properties":{"number":1,"goal":"{sprint-goal}","status":"planning"}
}]'
mindx graph upsert-edges --edges '[{
  "from_node_id":"'"$PROJ_ID"'","to_node_id":"'"$SPRINT_ID"'",
  "type":"CURRENT_SPRINT"
}]'
```

### Phase 4: Sprint Execution — Build Wave by Wave

Use **find-experts Mode 2 (Team Orchestration)** for sprints with multiple components.

**Team composition for typical software project:**

| Role | When Included | Responsibility |
|------|--------------|----------------|
| `architect` | Phase 2 only | Design, code review for structural issues |
| `backend-dev` | API, services, DB work | Server-side implementation |
| `frontend-dev` | UI, mobile client | Client-side implementation |
| `qa-engineer` | After each wave | Test planning, integration testing, regression |
| `devops` | Phase 6-7 | CI/CD pipeline, deployment, monitoring setup |

**Execution pattern per wave:**

```
// Wave 1: No-dependency tasks
task-update(wave1_tasks, status="in_progress")
sub-agent(backend-dev, task="Implement user sync API endpoint:...")
sub-agent(frontend-dev, task="Implement sync service client:...")
// Both run in parallel (same response)
collect-results(...)
task-update(wave1_tasks, status="completed")

// Wave 2: Depends on Wave 1
task-update(wave2_tasks, status="in_progress")
sub-agent(qa-engineer, task="Write integration tests for sync flow:...")
// Can start once Wave 1 completes
collect-results(...)
task-update(wave2_tasks, status="completed")

// Wave 3: Final validation
task-update(wave3_tasks, status="in_progress")
[execute final tasks...]
task-update(wave3_tasks, status="completed")
```

**Each sub-agent task prompt must include:**
- Clear acceptance criteria from Phase 1 DoD
- Relevant files/paths to work on
- Coding standards (language, framework conventions)
- Testing requirement (unit test coverage threshold)

### Phase 5: Quality Gates — Don't Ship Broken Code

Before declaring anything complete, verify against quality checklist:

| Gate | Check | How | Pass Criteria |
|------|-------|-----|----------------|
| **Code completeness** | All planned features implemented? | TaskList(status_filter="completed") | All wave tasks completed |
| **Unit tests** | Core logic covered? | Run test command | ≥70% coverage on new code |
| **Integration smoke** | Happy path works? | Manual or automated smoke test | Primary user flow executes end-to-end |
| **Code review** | No obvious issues? | Read changed files or sub-agent review | No critical bugs, style consistent |
| **No regressions** | Existing features still work? | Run existing test suite | All pre-existing tests still pass |

**If a gate fails:**

```
Quality gate FAILED: {which gate}
  Issue: {what went wrong}
  Action:
    Option A: Fix now (create fix task, assign, execute)
    Option B: Defer to next sprint (document as known issue, update task status)
    Option C: Adjust scope (remove failing feature from this sprint's DoD)
```

**Every 3rd completion triggers nudge from TaskUpdate** — act on it: run tests, review files, check coverage.

### Phase 6: Release — Ship It

When all quality gates pass:

```bash
# Update sprint status in graph
mindx graph exec --cypher "
  MATCH (s:Sprint {id: '$SPRINT_ID'})
  SET s.status = 'completed', s.completed_at = timestamp()
  RETURN s.number, s.goal
"

# If there's a next sprint, link it
mindx graph upsert-edges --edges '[{
  "from_node_id":"'"$PROJ_ID"'",
  "to_node_id":"'$NEXT_SPRINT_ID'",
  "type":"CURRENT_SPRINT"
}]'
```

**Release communication:**

```
🚀 Sprint {N} Complete — {Project Name}

Delivered:
  ✅ {feature-1}: {one-line summary}
  ✅ {feature-2}: {one-line summary}
  ✅ {feature-3}: {one-line summary}

Quality:
  Tests: {pass}/{total} passing | Coverage: {coverage}%
  Known issues: {n} (tracked)

Next sprint preview:
  · {next-item-1}
  · {next-item-2}

Deploy: {deployment status / instructions}
```

### Phase 7: Monitor — Watch After Shipping

Post-release checks:

| Check | Frequency | Command / Method |
|-------|----------|------------------|
| Error rate spike | Daily after release | Check logs / monitoring dashboards |
| Performance regression | Within 24h | Benchmark key endpoints vs baseline |
| User-reported bugs | Ongoing | Triage incoming reports, file if new |
| Technical debt accumulation | Per sprint | Review TODO/FIXME/HACK counts in new code |

## Sprint Anti-Patterns

- Do not skip Phase 1 (requirements) — "just start coding" always leads to rework
- Do not make sprints longer than 2 weeks — long sprints hide progress problems
- Do not add tasks mid-sprint without removing others of equal size — scope must be fixed at sprint start
- Do not mark tasks complete without running them (or having a sub-agent run them) — trust but verify
- Do not let tasks sit in `in_progress` for more than 2 days — investigate or cancel
- Do not skip quality gates for "MVP speed" — technical debt compounds faster than you think
- Do not push to production without at least a smoke test — even "small changes" can break things
- Do not ignore the nudge from TaskUpdate (every 3 completions) — it exists for a reason

## Quick Reference: Task Status Lifecycle

```
pending → in_progress → completed
                        ↓ (quality gate failed)
                     blocked → in_progress (after fix)
                        
pending → cancelled (deprioritized / out of scope)

Valid transitions only. Invalid transitions will be rejected.
```

## Quick Reference: Dependency Declaration Syntax

```
// "B depends on A" (A must finish before B starts):
TaskUpdate(task_id=B, addBlockedBy=[A])

// Equivalent alternative:
TaskUpdate(task_id=A, addBlocks=[B])

// Circular dependencies are auto-detected and rejected.
// Transitive cycles (A→B→C→A) are also caught.
```
