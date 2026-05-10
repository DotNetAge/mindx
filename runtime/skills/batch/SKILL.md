---
name: batch
description: >
  Parallel orchestration of large-scale mechanical changes.
  Use when the user mentions batch, bulk, replace all, or migrate all.
allowed-tools: grep subagent subagent-list subagent-result bash
---

# Batch: Parallel Work Orchestration

You are orchestrating a large, parallelizable change across this codebase.

## Phase 1: Research and Plan
1. **Understand the scope**: deeply research what this instruction touches. Find all the files, patterns, and call sites that need to change.
2. **Decompose into independent units**: Break the work into multiple self-contained units. Each unit must be independently implementable.
3. **Determine the test recipe**: Figure out how a worker can verify its change actually works (e.g. unit tests, e2e recipe).
4. **Write the plan**: Output the numbered list of work units.

## Phase 2: Spawn Workers
Spawn one independent SubAgent per work unit using the 'subagent' tool. Each SubAgent runs asynchronously in a background goroutine with its own timeout (default 5 minutes, configurable via timeout_seconds parameter). You can optionally provide a custom system_prompt and model for each worker.
Name each SubAgent using the @{name} format (e.g., @worker-1, @worker-2) for clear identification in skills and prompts.
You can spawn multiple 'subagent' calls across multiple Think-Act cycles. Each spawn returns immediately with a task ID.
For each SubAgent, the prompt must be fully self-contained including the specific task and codebase conventions.

## Phase 3: Track Progress & Review
Use 'subagent_list' to render an initial status table showing all spawned SubAgents.
Use 'subagent_result' with each task ID to wait for and retrieve results (with configurable wait_seconds).
After all workers finish, parse their results, verify using 'bash', and render a final summary of completed vs failed units.
