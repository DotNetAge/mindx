---
name: software-dev
description: >
  Follow this agile development process: requirements analysis → architecture design → sprint planning →
  sprint execution → quality gates → release → monitoring. Each phase produces specific artifacts.
allowed-tools: bash sub-agent collect-results task-create task-update task-get task-list team-create team-list team-get-tasks find-experts
metadata:
  name_zh: 软件开发
  name_zh-tw: 軟體開發
  description_zh: 遵循敏捷开发流程：需求分析 → 架构设计 → Sprint 规划 → Sprint 执行 → 质量门禁 → 发布 → 监控
  description_zh-tw: 遵循敏捷開發流程：需求分析 → 架構設計 → Sprint 規劃 → Sprint 執行 → 質量門禁 → 發佈 → 監控
---

## When to Use

Activate this skill when:
- User wants to build a software feature, module, or entire product
- User mentions: develop, implement, build, sprint, release, refactor, architecture
- The task spans multiple files or multiple sessions

Do NOT activate for: single-file edits, trivial bug fixes — do those directly.

---

## Phase 1 — Requirements Analysis

**Mandatory first phase. Do not skip to coding.**

### Step 1.1 — Elicit Requirements from the User

Ask the user these five questions in order. Wait for each answer before proceeding.

1. **Core problem** — "What problem are you trying to solve? What scenario are you implementing?"
2. **Users & scope** — "Who will use this? At what scale?"
3. **Specific features** — "What exactly needs to be built? List the core features. Which are MVP vs. future?"
4. **Constraints** — "Any technical, platform, timeline, or resource constraints?"
5. **Definition of done** — "How do we verify it's complete? What are the acceptance criteria?"

During the conversation, actively guide:

- User is vague → ask clarifying follow-ups
- User lists too many features → help prioritize ("Which 3 are the most critical?")
- User describes a solution instead of a problem → dig deeper ("What underlying problem does this solve?")

### Step 1.2 — Output Requirements Brief

Synthesize answers into this document. Save to `docs/requirements.md`. Show to the user for confirmation before proceeding:

```markdown
# Requirements Brief

## Overview
[One-line summary of what to build]

## Core Problem
[The pain point or goal]

## Target Users
[Who and at what scale]

## MVP Features
1. [Feature A]
2. [Feature B]
3. [Feature C]

## Future Scope (excluded from this round)
- [Feature D]
- [Feature E]

## Constraints
- Tech stack: [e.g. Go + React + PostgreSQL]
- Platform: [e.g. Web / iOS / Android]
- Timeline: [e.g. 2 weeks]
- Other:

## Acceptance Criteria
- [ ] [Verifiable criterion 1]
- [ ] [Verifiable criterion 2]
- [ ] [Verifiable criterion 3]

## Success Metric
[How to measure success]
```

> **Phase 1 is complete when `docs/requirements.md` is saved to the project directory and confirmed by the user.**

---

## Phase 2 — Architecture Design

Based on the confirmed requirements brief, produce the architecture blueprint. Save to `docs/architecture.md`.

### Output: Architecture Blueprint

```markdown
# Architecture Blueprint

## 1. System Architecture Diagram

[Use Mermaid to draw: Client → API Gateway → Service Layer → Data Layer, etc.]

## 2. Project Directory Structure

```
project-root/
├── cmd/              # Entry points
├── internal/
│   ├── api/          # HTTP/gRPC handlers
│   ├── service/      # Business logic
│   ├── repository/   # Data access
│   └── model/        # Data models
├── pkg/              # Shared libraries
├── migrations/       # Database migrations
├── configs/          # Configuration
└── deploy/           # Deployment files
```

## 3. Core Data Models

- [Entity 1]: {fields, relationships}
- [Entity 2]: {fields, relationships}

## 4. API Surface

- `POST /api/resource` — Create resource
- `GET /api/resource/:id` — Get resource
- ...

## 5. Component Dependencies

- Component B depends on A (build A first)
- Component C is independent (can parallelize with A/B)
```

> **Phase 2 is complete when `docs/architecture.md` is saved to the project directory.**

---

## Phase 3 — Sprint Planning

Every sprint is requirement-driven. Select requirement(s) from the Requirements Brief to deliver in this sprint. The sprint goal is to deliver those specific requirements. Do not create tasks from the architecture alone — tasks serve requirements.

### Step 3.1 — Select Sprint Requirements

Pick a subset of requirements from the Requirements Brief that can be delivered in one sprint cycle. The sprint goal must reference specific requirements by name.

### Step 3.2 — Discover Available Agents

Run `mindx agent list` to see available agents and their capabilities. Use this to determine who can handle each task.

### Step 3.3 — Break Requirements Into Tasks

For each selected requirement, decompose into executable tasks organized by dependency waves. Every task must map back to a requirement from the sprint goal.

### Step 3.4 — Output Sprint Plan

Save sprint plan to `docs/sprint-plan.md`.

```markdown
# Sprint 1: [derived from requirements — e.g. "user authentication & profile"]

Requirement(s) addressed:
  - [Requirement A from Requirements Brief]
  - [Requirement B from Requirements Brief]

Wave 1 (no dependencies, parallel):
  Task: [Feature A] — Requirement: [Req A] — Owner: [agent] — Est: M
  Task: [Feature B] — Requirement: [Req A] — Owner: [agent] — Est: M

Wave 2 (depends on Wave 1):
  Task: [Integration A+B] — Requirement: [Req A] — Owner: [agent] — Est: S
  Task: [Tests A+B] — Requirement: [Req B] — Owner: [agent] — Est: S

Wave 3 (depends on Wave 2):
  Task: [E2E tests] — Requirement: [Req A, Req B] — Owner: [agent] — Est: M
```

Create tasks and wire dependencies:

```
task-create(subject="[Req X] Task name", description="AC from requirements: {criteria}. Files to modify: {paths}.")
task-update(task_id=B, addBlockedBy=[A])
```

> **Phase 3 is complete when `docs/sprint-plan.md` is saved and all tasks are created in the system with dependencies wired.**

---

## Phase 4 — Sprint Execution

Execute tasks wave by wave. Run parallel tasks concurrently using sub-agents.

### Step 4.1 — Agent Discovery

Use `mindx agent list` to find the right agent for each task. Match task requirements against each agent's capabilities.

### Step 4.2 — Sub-Agent Task Template

Every delegated task must include these five elements, with ACCEPTANCE pulled directly from the Requirements Brief:

```
subject: clear task name
description: |
  GOAL: what to achieve
  ACCEPTANCE: [copied from requirements — these are the same criteria used to mark this task complete]
  SCOPE: exact files/paths to modify
  CONSTRAINTS: tech stack, conventions, patterns to follow
  DELIVERABLE: expected output format
```

### Step 4.3 — Execution Pattern

```
# Wave 1: parallel
task-update(wave1_tasks, status="in_progress")
sub-agent([agent_name], task="{goal}. Accept when: {acceptance}. Scope: {paths}. Standards: {tech}.")
sub-agent([agent_name], task="{goal}. Accept when: {acceptance}. Scope: {paths}. Standards: {tech}.")
collect-results(...)
→ Verify each result against its acceptance criteria. Run tests. Mark complete only when all pass.
task-update(wave1_tasks, status="completed")

# Wave 2: sequential
task-update(wave2_tasks, status="in_progress")
sub-agent([agent_name], task="{goal}. Accept when: {acceptance}. Scope: {paths}.")
collect-results(...)
→ Verify against acceptance criteria. Run integration tests.
task-update(wave2_tasks, status="completed")
```

---

## Phase 5 — Quality Gates

Before marking any task or sprint complete, run these checks in order:

| Gate                 | Action                                    | Fail →             |
| -------------------- | ----------------------------------------- | ------------------ |
| 1. Code complete     | Verify all planned code is written        | Fix or defer       |
| 2. Unit tests        | Run test suite, coverage ≥70% on new code | Fix failing tests  |
| 3. Integration smoke | Run primary user flow end-to-end          | Debug and fix      |
| 4. Code review       | Read key files for structural issues      | Refactor if needed |
| 5. No regressions    | Run all pre-existing tests                | Fix regressions    |

After passing all gates, save a quality report to `docs/quality-report.md` documenting results for each gate.

> **Phase 5 is complete when all gates pass and `docs/quality-report.md` is saved.**

---

## Phase 6 — Release

When all quality gates pass, save release notes to `docs/release-notes.md` and communicate:

```
Sprint 1 Complete — [Project]

Delivered:
  ✅ [Feature]: [summary]
  ✅ [Feature]: [summary]

Quality:
  Tests: X/Y passing | Coverage: X%
  Known issues: N (tracked)

Next sprint:
  · [item 1]
  · [item 2]
```

> **Phase 6 is complete when `docs/release-notes.md` is saved.**

---

## Phase 7 — Monitor

Post-release checks:

| Check        | When       | How                          |
| ------------ | ---------- | ---------------------------- |
| Error spikes | Daily      | Check logs                   |
| Performance  | Within 24h | Benchmark vs baseline        |
| User bugs    | Ongoing    | Triage reports               |
| Tech debt    | Per sprint | Count TODO/FIXME in new code |

---

## Hard Rules

- **Each phase is complete only when its deliverable document is saved.** No document = incomplete phase.
- Phase 1 complete → `docs/requirements.md` saved and user confirmed.
- Phase 2 complete → `docs/architecture.md` saved.
- Phase 3 complete → `docs/sprint-plan.md` saved and tasks wired.
- Phase 5 complete → all quality gates passed and `docs/quality-report.md` saved.
- Phase 6 complete → `docs/release-notes.md` saved.
- Phase 1 must complete before Phase 2. Phase 2 before Phase 3. Phase 5 before Phase 6. No exceptions.
- Every task must have a clear acceptance criterion before execution.
- Do not modify files outside the agreed scope without re-confirming with the user.
- All output documents must be written in the same language as the user's input.
- **The acceptance criteria defined in Phase 1 are the sole standard for verifying all outputs. If the criteria are insufficient to verify a result, the deficiency is in Phase 1 — go back and refine requirements before proceeding.**
