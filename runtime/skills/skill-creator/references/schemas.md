# MindX Skill 数据模型

本文档定义了 MindX skill 所使用的数据结构。

## Skill 存放位置

一个 skill 就是一个目录，其中至少要包含一个 `SKILL.md` 文件。守护进程在内部管理 skill 注册表。通过以下命令将 skill 安装到受管理的注册表中：

```bash
mindx skill add <path-to-skill-directory>
```

安装完成后，运行 `mindx reload skills`，让守护进程加载新版本。

## SKILL.md 前置元数据

MindX skill 是一个 Markdown 文件，文件顶部包含一段 YAML 前置元数据块。

### 必填字段

```yaml
---
name: <kebab-case-name>
description: >
  <One sentence for LLM routing>
---
```

### 可选字段

```yaml
---
name: <kebab-case-name>
description: >
  <One sentence for LLM routing>
allowed-tools: Read Edit Task Bash Grep Glob
metadata:
  requires:
    bins:
      - python3
    env:
      - MY_API_KEY
  name_zh: <中文名>
  description_zh: <中文描述>
---
```

### 字段说明

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是 | skill 的唯一标识符。只能使用小写字母、数字和连字符，且必须以字母开头。 |
| `description` | string | 是 | 供 LLM 路由使用的描述。应说明该 skill 的功能和适用场景，长度不超过 1024 个字符。 |
| `allowed-tools` | string | 否 | 该 skill 加载后可使用的工具列表，以空格分隔。必须是合法的 MindX 工具名称。 |
| `metadata` | object | 否 | 自定义键值元数据。规范扩展（包括 `requires`）也存放在此。 |
| `metadata.requires` | object | 否 | 运行时依赖声明。加载时若依赖未满足，该 skill 会被静默跳过。 |
| `metadata.requires.bins` | list of strings | 否 | PATH 中必须存在的可执行文件。任一缺失则跳过该 skill。示例：`["python3", "git"]`。 |
| `metadata.requires.env` | list of strings | 否 | 必须设置的环境变量。任一为空或未设置则跳过该 skill。示例：`["MY_API_KEY"]`。 |
| `metadata.name_zh` | string | 否 | 简体中文显示名称。 |
| `metadata.name_zh-tw` | string | 否 | 繁体中文显示名称。 |
| `metadata.description_zh` | string | 否 | 简体中文描述。 |
| `metadata.description_zh-tw` | string | 否 | 繁体中文描述。 |

### 校验规则

- `name` 在 skill 注册表中必须唯一
- `description` 不能为空，且长度不得超过 1024 个字符
- `allowed-tools` 中的条目必须属于 MindX 工具集
- 守护进程在加载时通过 `exec.LookPath` 检查 `metadata.requires.bins`；缺失的可执行文件会导致该 skill 被跳过
- 守护进程在加载时通过 `os.Getenv` 检查 `metadata.requires.env`；缺失或为空的环境变量会导致该 skill 被跳过
- 目录名应与 `name` 字段保持一致

## SKILL.md 示例

```markdown
---
name: sql-reviewer
description: >
  Reviews SQL queries for correctness, performance, and security issues.
  Use when the user provides a SQL query and asks for feedback or optimization.
allowed-tools: Read Edit Task
metadata:
  name_zh: SQL 审查者
  description_zh: 审查 SQL 查询的正确性、性能和安全性问题
---

## When to Use

- User asks "review this SQL query"
- User asks "is this query safe?"
- User asks "how can I optimize this query?"

## Workflow

### Step 1: Read the Query

Read the SQL query from the user's message or from a file they referenced.

### Step 2: Check Correctness

Verify that the query is syntactically valid and that table/column names exist.

### Step 3: Check Security

Look for SQL injection risks, missing parameterization, and overly broad permissions.

### Step 4: Check Performance

Identify missing indexes, unnecessary joins, full table scans, and N+1 patterns.

### Step 5: Provide Feedback

Return a concise report with:

- Critical issues (must fix)
- Warnings (should fix)
- Suggestions (optional improvements)
- A corrected query if applicable

## Important Notes

- Do not execute queries against a database unless explicitly asked.
- Do not modify user files unless the user asks for a rewrite.
- Keep feedback focused on SQL-specific concerns.
```
