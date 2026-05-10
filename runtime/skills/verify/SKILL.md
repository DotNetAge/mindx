---
name: verify
description: >
  Rigorous verification of changes through testing and execution.
  Use when the user mentions verify, test, check, or QA.
allowed-tools: bash todo-write
---

# Verify: Code Change QA

Verify a code change does what it should by running the app and tests.

## Phase 1: Locate tests
1. Check for 'package.json', 'Makefile', 'go.mod', or other build files to find test commands.
2. Find related test files for the recently modified code.

## Phase 2: Execution
1. **Unit Tests**: Run relevant tests using 'bash'.
2. **Lint & Static Analysis**: Run the project's linter.
3. **E2E/Integration**: If applicable, start the dev server and hit endpoints using 'web_fetch' or curl in 'bash'.

## Phase 3: Report & Cleanup
1. Document which tests passed and which failed.
2. If tests failed, automatically attempt to fix the code or the tests if it's a simple discrepancy.
3. Remove any side effects or temporary files.
