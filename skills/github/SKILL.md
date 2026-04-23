---
name: github
description: "Manages GitHub repositories, issues, pull requests, and CI workflows via the gh CLI. Use when the user asks about PRs, issues, CI status, repo settings, or any GitHub operation."
version: 1.0.0
category: general
tags:
  - github
  - git
  - issue
  - pr
  - ci
  - GitHub
  - 代码仓库
  - 拉取请求
os:
  - darwin
  - linux
enabled: true
timeout: 60
command: gh
requires:
  bins:
    - gh
homepage: https://cli.github.com
---

# GitHub 技能

Uses the `gh` CLI to interact with GitHub. When not inside a git directory, add `--repo owner/repo`.

## Workflow: Debug a Failing PR

1. Check CI status: `gh pr checks <pr-number> --repo owner/repo`
2. List recent runs: `gh run list --repo owner/repo --limit 10`
3. Inspect the failed run: `gh run view <run-id> --repo owner/repo`
4. View only failed logs: `gh run view <run-id> --repo owner/repo --log-failed`
5. Fix the issue, push, and re-check.

## Issue and PR Management

```bash
gh issue list --repo owner/repo --json number,title --jq '.[] | "\(.number): \(.title)"'
gh pr create --repo owner/repo --title "fix: resolve bug" --body "Description"
```

## API Queries

Access data not available through other subcommands:

```bash
gh api repos/owner/repo/pulls/55 --jq '.title, .state, .user.login'
```
