---
name: stuck
description: >
  Strategy to break free when the agent is repeating actions or failing.
  Use when the user mentions stuck, loop, repeat, or failing.
allowed-tools: grep glob bash
---

# Stuck: Diagnose Frozen/Slow State

Strategy to break free when the agent is repeating actions, failing, or stuck in a loop.

## Phase 1: Diagnosis
1. **Analyze**: Identify why the previous attempts failed by reading recent conversation history.
2. **Identify Loops**: Are you repeatedly calling the same tool with the same arguments and getting the same error?

## Phase 2: Pivot
1. Change your approach. If Grep failed, try Glob. If Bash failed, try to write a small script and execute it.
2. Simplify the problem. Isolate the failing component and write a minimal reproduction test.

## Phase 3: Action
1. If the environment is hung (e.g., high CPU, stuck processes), use 'bash' to run 'ps' or 'top' and kill hung processes if necessary.
2. If still stuck after pivoting, formulate a clear question and ask the user for guidance rather than continuing to loop.
