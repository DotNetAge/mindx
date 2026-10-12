---
source: https://agentskills.io/
updated: 2026-10-11
---

# SKILL规范

> Agent Skills 的完整格式规范。

> **MindX 差异说明**：本文是 agentskills.io 通用规范。MindX 加载器只消费 `name`、`description`、`allowed-tools` 和 `metadata` 四个字段；`license`、`compatibility` 会被解析后丢弃，写了不生效。MindX 扩展键（`metadata.name_zh`、`metadata.version`、`metadata.requires`、`metadata.install`）以 [schemas.md](schemas.md) 为准，本文描述的通用规范与 schemas.md 冲突时一律以 schemas.md 为准。

## 目录结构

一个 skill 就是一个目录，其中至少要包含一个 `SKILL.md` 文件：

```
skill-name/
├── SKILL.md          # 必需：元数据 + 指令
├── scripts/          # 可选：可执行代码
├── references/       # 可选：文档
├── assets/           # 可选：模板、资源
└── ...               # 任何其他文件或目录
```

## `SKILL.md` 格式

`SKILL.md` 文件必须包含 YAML 前置元数据（frontmatter），后接 Markdown 正文。

### 前置元数据

| 字段            | 必需 | 约束                                                               |
| --------------- | ---- | ------------------------------------------------------------------ |
| `name`          | 是   | 最长 64 字符。仅限小写字母、数字和连字符，不能以连字符开头或结尾。 |
| `description`   | 是   | 最长 1024 字符。非空。描述该 skill 的功能及使用时机。              |
| `license`       | 否   | 许可证名称，或对随附许可证文件的引用。                             |
| `compatibility` | 否   | 最长 500 字符。标明环境要求（目标产品、系统软件包、网络访问等）。  |
| `metadata`      | 否   | 用于附加元数据的任意键值映射（字符串键到字符串值的映射）。         |
| `allowed-tools` | 否   | 该 skill 可使用的预批准工具列表，以空格分隔的字符串。（实验性）    |

<Card>
  **最小示例：**

  ```markdown SKILL.md theme={null}
  ---
  name: skill-name
  description: 说明此技能的功能及使用时机。
  ---
  ```

  **带可选字段的示例：**

  ```markdown SKILL.md theme={null}
  ---
  name: pdf-processing
  description: 提取 PDF 文本、填写表单、合并文件。处理 PDF 时使用。
  license: Apache-2.0
  metadata:
    author: example-org
    version: "1.0"
  ---
  ```
</Card>

#### `name` 字段

必需的 `name` 字段：

* 长度必须为 1-64 个字符
* 只能包含 unicode 小写字母数字字符（`a-z`、`0-9`）和连字符（`-`）
* 不能以连字符（`-`）开头或结尾
* 不能包含连续的连字符（`--`）
* 必须与父目录名一致

<Card>
  **有效示例：**

  ```yaml theme={null}
  name: pdf-processing
  ```

  ```yaml theme={null}
  name: data-analysis
  ```

  ```yaml theme={null}
  name: code-review
  ```

  **无效示例：**

  ```yaml theme={null}
  name: PDF-Processing  # 不允许大写
  ```

  ```yaml theme={null}
  name: -pdf  # 不能以连字符开头
  ```

  ```yaml theme={null}
  name: pdf--processing  # 不允许连续连字符
  ```
</Card>

#### `description` 字段

必需的 `description` 字段：

* 长度必须为 1-1024 个字符
* 应同时描述该 skill 的功能和使用时机
* 应包含能帮助智能体识别相关任务的具体关键词

<Card>
  **好示例：**

  ```yaml theme={null}
  description: 从 PDF 文件中提取文本和表格、填写 PDF 表单，并合并多个 PDF。处理 PDF 文档，或用户提到 PDF、表单或文档提取时使用。
  ```

  **差示例：**

  ```yaml theme={null}
  description: 帮助处理 PDF。
  ```
</Card>

#### `license` 字段

可选的 `license` 字段：

* 指定应用于该 skill 的许可证
* 建议保持简短（许可证名称，或随附许可证文件的名称）

<Card>
  **示例：**

  ```yaml theme={null}
  license: 专有。完整条款见 LICENSE.txt
  ```
</Card>

#### `compatibility` 字段

可选的 `compatibility` 字段：

* 如提供，长度必须为 1-500 个字符
* 仅当你的 skill 有特定环境要求时才应包含
* 可标明目标产品、必需的系统软件包、网络访问需求等

<Card>
  **示例：**

  ```yaml theme={null}
  compatibility: 为 Claude Code（或类似产品）设计
  ```

  ```yaml theme={null}
  compatibility: 需要 git、docker、jq 以及网络访问
  ```

  ```yaml theme={null}
  compatibility: 需要 Python 3.14+ 和 uv
  ```
</Card>

<Note>
  大多数 skill 不需要 `compatibility` 字段。
</Note>

#### `metadata` 字段

可选的 `metadata` 字段：

* 字符串键到字符串值的映射
* 客户端可用它存储 Agent Skills 规范未定义的其他属性
* 建议让键名保持足够独特，以避免意外冲突

<Card>
  **示例：**

  ```yaml theme={null}
  metadata:
    author: example-org
    version: "1.0"
  ```
</Card>

#### `allowed-tools` 字段

可选的 `allowed-tools` 字段：

* 预批准可运行的工具列表，以空格分隔的字符串
* 实验性。不同智能体实现对该字段的支持可能不同

<Card>
  **示例：**

  ```yaml theme={null}
  allowed-tools: Bash(git:*) Bash(jq:*) Read
  ```
</Card>

### 正文内容

前置元数据之后的 Markdown 正文包含技能指令。格式没有限制，写任何有助于智能体高效执行任务的内容即可。

推荐包含的部分：

* 分步指令
* 输入与输出示例
* 常见边界情况

注意：智能体一旦决定激活某个 skill，就会加载整个文件。考虑把较长的 `SKILL.md` 内容拆分到被引用的文件中。

## 可选目录

skill 目录除必需的 `SKILL.md` 外，还可以包含任何文件和目录。以下约定是对常见类型内容的组织建议。

### `scripts/`

包含智能体可运行的可执行代码。脚本应当：

* 自包含，或清楚说明依赖
* 包含有用的错误信息
* 优雅地处理边界情况

支持的语言取决于智能体实现。常见选项包括 Python、Bash 和 JavaScript。

### `references/`

包含智能体需要时可读取的补充文档：

* `REFERENCE.md` - 详细的技术参考
* `FORMS.md` - 表单模板或结构化数据格式
* 领域特定文件（`finance.md`、`legal.md` 等）

保持各个[参考文件](#file-references)聚焦。智能体按需加载这些文件，因此文件越小，占用的上下文越少。

### `assets/`

包含静态资源：

* 模板（文档模板、配置模板）
* 图片（图表、示例）
* 数据文件（查找表、schema）

## 渐进式披露

智能体*渐进式*地加载 skill，只在任务需要时拉取更多细节。skill 的结构应充分利用这一点：

1. **元数据**（约 100 token）：所有 skill 的 `name` 和 `description` 字段在启动时加载
2. **指令**（建议小于 5000 token）：完整的 `SKILL.md` 正文在 skill 被激活时加载
3. **资源**（按需）：文件（如 `scripts/`、`references/` 或 `assets/` 中的文件）仅在需要时加载

让主 `SKILL.md` 保持在 500 行以内。把详细的参考材料移到单独的文件中。

## 文件引用

在 skill 中引用其他文件时，使用相对于 skill 根目录的路径：

```markdown SKILL.md theme={null}
详细内容参见[参考指南](references/REFERENCE.md)。

运行提取脚本：
scripts/extract.py
```

保持文件引用在 `SKILL.md` 一层以内。避免深层嵌套的引用链。

## 校验

使用 [skills-ref](https://github.com/agentskills/agentskills/tree/main/skills-ref) 参考库来校验你的 skill：

```bash theme={null}
skills-ref validate ./my-skill
```

这会检查你的 `SKILL.md` 前置元数据是否有效，以及是否遵循所有命名约定。
