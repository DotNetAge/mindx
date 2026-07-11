---
name: find-skills
description: 帮助用户发现和安装智能体技能，当用户问"如何做某事"、"找一个能做 X 的技能"、"有没有能...的技能"，或表达扩展能力的兴趣时使用。当用户寻找可能作为可安装技能存在的功能时，应使用此技能。
metadata:
  name_zh: 查找技能
  name_zh-tw: 查找技能
  description_zh: 帮助用户发现和安装智能体技能，当用户问"如何做某事"或表达扩展能力兴趣时使用
  description_zh-tw: 幫助使用者發現和安裝智慧體技能，當使用者問「如何做某事」或表達擴展能力興趣時使用
---

# 查找技能

本技能帮你从开放的智能体技能生态系统中发现和安装技能。

## 何时使用本技能

用户出现以下情况时使用：

- 问"如何做 X"，而 X 可能是有现成技能的常见任务
- 说"找一个能做 X 的技能"或"有没有能做 X 的技能"
- 问"你能做 X 吗"，而 X 需要专业能力
- 想扩展智能体的能力
- 想搜索工具、模板或工作流
- 提到需要特定领域的帮助（设计、测试、部署等）

## 什么是 Skills CLI？

Skills CLI (`npx skills`) 是开放智能体技能生态系统的包管理器。技能是模块化包，通过专业知识、工作流和工具来扩展智能体能力。

**关键命令：**

- `npx skills find [query]` - 交互式搜索技能或按关键词搜索
- `npx skills add <package>` - 从 GitHub 或其他来源安装技能
- `npx skills check` - 检查技能更新
- `npx skills update` - 更新所有已安装技能

**浏览技能：** https://skills.sh/

## 如何帮助用户查找技能

### 步骤 1：了解他们的需求

当用户请求帮助时，识别：

1. 领域（例如，React、测试、设计、部署）
2. 具体任务（例如，编写测试、创建动画、审查 PR）
3. 这是否是足够常见的任务，可能已有技能存在

### 步骤 2：先查看排行榜

在运行 CLI 搜索前，查看 [skills.sh 排行榜](https://skills.sh/) 以了解该领域是否已有知名技能。排行榜按总安装量排名，展示最受欢迎和经过实战检验的选项。

例如，Web 开发的顶级技能包括：
- `vercel-labs/agent-skills` — React、Next.js、Web 设计（各 100K+ 安装）
- `anthropics/skills` — 前端设计、文档处理（100K+ 安装）

### 步骤 3：搜索技能

如果排行榜没有覆盖用户需求，运行 find 命令：

```bash
npx skills find [query]
```

例如：

- 用户问"如何让我的 React 应用更快？" → `npx skills find react performance`
- 用户问"你能帮我审查 PR 吗？" → `npx skills find pr review`
- 用户问"我需要创建变更日志" → `npx skills find changelog`

### 步骤 4：推荐前验证质量

**不要仅凭搜索结果就推荐技能。** 一定要验证：

1. **安装量** — 优先选择 1K+ 安装的技能。100 以下的要谨慎。
2. **来源声誉** — 官方来源（`vercel-labs`、`anthropics`、`microsoft`）比未知作者更可信。
3. **GitHub 星标** — 检查源仓库。来自 <100 星标仓库的技能要持怀疑态度。

### 步骤 5：向用户展示选项

找到相关技能后，向用户展示：

1. 技能名称和功能
2. 安装量和来源
3. 可直接运行的安装命令
4. skills.sh 上的详情链接

示例响应：

```
我找到一个可能有用的技能！"react-best-practices" 技能提供
来自 Vercel Engineering 的 React 和 Next.js 性能优化指南。
（185K 安装）

安装它：
npx skills add vercel-labs/agent-skills@react-best-practices

了解更多：https://skills.sh/vercel-labs/agent-skills/react-best-practices
```

### 步骤 6：提供安装帮助

用户想继续的话，可以帮他们安装技能：

```bash
npx skills add <owner/repo@skill> -a claude-code -g -y
```

`-a claude-code` 指定 Claude Code 智能体目录，`-g` 全局安装（用户级），`-y` 跳过确认提示。

## 常见技能类别

搜索时考虑这些常见类别：

| 类别        | 示例查询                          |
| --------------- | ---------------------------------------- |
| Web 开发 | react, nextjs, typescript, css, tailwind |
| 测试         | testing, jest, playwright, e2e           |
| DevOps          | deploy, docker, kubernetes, ci-cd        |
| 文档   | docs, readme, changelog, api-docs        |
| 代码质量    | review, lint, refactor, best-practices   |
| 设计          | ui, ux, design-system, accessibility     |
| 生产力    | workflow, automation, git                |

## 搜索技巧

1. **用具体关键词**："react testing" 比单用 "testing" 效果更好
2. **尝试不同术语**：如果 "deploy" 搜不到，试试 "deployment" 或 "ci-cd"
3. **查看热门来源**：很多技能来自 `vercel-labs/agent-skills` 或 `ComposioHQ/awesome-claude-skills`

## 没找到技能时

如果没有相关技能：

1. 告知用户没找到现有技能
2. 用通用能力直接帮忙完成任务
3. 建议用户可以用 `npx skills init` 创建自己的技能

示例：

```
我搜索了与 "xyz" 相关的技能，但没有找到匹配项。
我仍然可以直接帮你完成这个任务！要我继续吗？

如果这是你经常做的事情，你可以创建自己的技能：
npx skills init my-xyz-skill
```
