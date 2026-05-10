---
name: simplify
description: >
  Post-implementation cleanup to ensure code quality and simplicity.
  Use when the user mentions simplify, cleanup, refactor, or polish.
allowed-tools: file-edit bash read replace
---

# Simplify: Code Review and Cleanup

Review all changed files for reuse, quality, and efficiency. Fix any issues found.

## Phase 1: Identify Changes
Run 'git diff' to see what changed, or review the recently modified files.

## Phase 2: Review (Simulated Parallel)
Review the changes across three dimensions:
1. **Code Reuse**: Search for existing utilities and helpers that could replace newly written code. Flag logic that duplicates existing utilities.
2. **Code Quality**: Look for hacky patterns: redundant state, parameter sprawl, copy-paste variations, stringly-typed code, unnecessary comments.
3. **Efficiency**: Look for unnecessary work (redundant file reads, N+1 patterns), missed concurrency, memory leaks, and overly broad operations.

## Phase 3: Fix Issues
Aggregate the findings and fix each issue directly using 'replace_in_file' or 'file_edit'.
Briefly summarize what was fixed.
