---
name: simplify
description: >
  Post-implementation cleanup to ensure code quality and simplicity.
  Use when the user mentions simplify, cleanup, refactor, or polish.
allowed-tools: file-edit bash read replace
metadata:
  name_zh: 代码简化
  name_zh-tw: 程式碼簡化
  description_zh: 实现后的代码质量检查与清理，确保代码复用性、质量和效率
  description_zh-tw: 實作後的程式碼品質檢查與清理，確保程式碼複用性、品質和效率
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
