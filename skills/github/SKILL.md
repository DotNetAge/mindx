---
name: github
description: GitHub管理技能，管理仓库、Issue、Pull Request和CI工作流
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

使用 `gh` CLI 与 GitHub 交互。当不在 git 目录中时，请指定 `--repo owner/repo`。

## Pull Request

检查 PR 的 CI 状态:

```bash
gh pr checks 55 --repo owner/repo
```

列出最近的工作流运行:

```bash
gh run list --repo owner/repo --limit 10
```

查看运行和失败的步骤:

```bash
gh run view <run-id> --repo owner/repo
```

仅查看失败步骤的日志:

```bash
gh run view <run-id> --repo owner/repo --log-failed
```

## API 高级查询

`gh api` 命令用于访问其他子命令不可用的数据:

```bash
gh api repos/owner/repo/pulls/55 --jq '.title, .state, .user.login'
```

## JSON 输出

大多数命令支持 `--json` 进行结构化输出，可使用 `--jq` 过滤:

```bash
gh issue list --repo owner/repo --json number,title --jq '.[] | "\(.number): \(.title)"'
```
