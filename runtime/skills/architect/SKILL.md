---
name: architect
description: >
  High-level orchestration for system design and major migrations.
  Use when the user mentions architecture, design, refactoring, or migration.
allowed-tools: glob grep subagent subagent-list subagent-result todo-write
---

# Architect: System Design & Refactoring

High-level orchestration for system design and major migrations.

## Phase 1: Research & Plan
1. **Analyze**: Use 'glob' and 'grep' to understand the project structure and patterns.
2. **Plan**: Define a multi-phase plan. Break it down into independent work units. Output this plan clearly.

## Phase 2: Delegate
1. **Task Decomposition**: Break the work into self-contained units.
2. **Spawn SubAgents**: Use 'subagent' to spawn independent agents for each unit asynchronously. Name each SubAgent using the @{name} format (e.g., @backend-dev, @frontend-dev, @reviewer) for clear identification in skills and prompts. Each SubAgent has its own system prompt and model (optionally specified). Each runs independently with its own timeout (default 5 minutes).
3. **Important**: 'subagent' is asynchronous — it spawns the agent and returns immediately. Use 'subagent_result' to wait for and retrieve a specific SubAgent's result, or 'subagent_list' to check all SubAgents' progress.
4. **For sequential inline tasks**: Use 'task_create' instead — it runs synchronously within the current thread with the same system prompt and model.

## Phase 3: Govern & Integrate
1. **Monitor**: Use 'subagent_list' to check progress, then 'subagent_result' to collect completed results.
2. **Integrate**: Perform the final assembly and cross-module verification to ensure architectural consistency.
