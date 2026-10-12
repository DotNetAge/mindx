---
updated: 2026-10-11
---

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
  name_zh: <中文展示名>
  version: "<semver>"
  requires:
    bins:
      - python3
    envs:
      - MY_API_KEY
    runtime:
      python: ">=3.10"
    oauth:
      provider: <client-provider 注册表名>
      env: <令牌落库凭据名>
  install:
    darwin:
      - brew install <pkg>
    linux:
      - apt-get install -y <pkg>
  description_zh: <中文描述>
---
```

### 字段说明

| 字段                         | 类型   | 必填 | 说明                                                                                                                                                 |
| ---------------------------- | ------ | ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| `name`                       | string | 是   | skill 的唯一标识符。只能使用小写字母、数字和连字符，且必须以字母开头。                                                                               |
| `description`                | string | 是   | 供 LLM 路由使用的描述。应说明该 skill 的功能和适用场景，长度不超过 1024 个字符。                                                                     |
| `allowed-tools`              | string | 否   | 该 skill 加载后可使用的工具列表，以空格分隔。必须是合法的 MindX 工具名称。                                                                           |
| `metadata`                   | object | 否   | 自定义键值元数据。MindX 扩展键（`name_zh`、`version`、`requires`、`install` 等）都存放在此。                                                         |
| `metadata.requires`          | object | 否   | 运行时依赖声明，是技能安装、依赖检查与运行前准备流程的输入。见下方 requires 详表。                                                                   |
| `metadata.install`           | object | 否   | 依赖修复动作命令块（与 `requires` 平级）。key 为平台名（`darwin` / `linux` 等），value 为命令列表。安装/准备时展示命令清单，用户确认一次后自动执行。 |
| `metadata.name_zh`           | string | 否   | 简体中文显示名。技能导出打包时写入市场清单，市场卡片展示用；未声明时回退技能原名，不构成错误。                                                       |
| `metadata.version`           | string | 否   | 技能版本号（如 `"1.0.1"`）。打包为市场分发包时写入包清单供展示。MindX 目前没有版本管理策略，仅作展示元数据，非必填。                                 |
| `metadata.name_zh-tw`        | string | 否   | 繁体中文显示名称。                                                                                                                                   |
| `metadata.description_zh`    | string | 否   | 简体中文描述。                                                                                                                                       |
| `metadata.description_zh-tw` | string | 否   | 繁体中文描述。                                                                                                                                       |

### `metadata.requires` 详表

| 字段      | 类型            | 说明                                                                                                                                                                                                                                                                                            |
| --------- | --------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bins`    | list of strings | PATH 中必须存在的可执行文件（如 `lark-cli`、`ffmpeg`）。加载时按三层查找链探测：进程 PATH → 用户 shell（`zsh -lic which`，覆盖 nvm/Homebrew 等用户级安装）→ 常见安装目录扫描。                                                                                                                  |
| `envs`    | list            | 必须设置的环境变量。**定稿键为 `envs`**，兼容存量 `env` 写法。条目两种形态：字符串（仅变量名）；对象 `{name, prompt, url, sensitive}`——`name` 必填（缺失整条忽略），`prompt` 为首用收集说明（缺失告警），`url` 为申请入口（可选），`sensitive: true` 表示掩码输入且正文禁明文赋值样例（告警）。缺失时首用触发收集，落库键 `<skill_name>:<env>`。 |
| `runtime` | map             | 生态级运行时版本要求（跨 skill 共享的运行时，不要与 `bins` 重复声明）。key 为运行时名（`node` / `python` / `dotnet` / `go` / `rust` / `java`），value 为版本约束字符串，支持 `>=18`、`>18`、`^18`、`18.x`、`18`。检查时映射到对应二进制（`python` → `python3`，`rust` → `rustc`）读取版本比较。 |
| `oauth`   | object          | 技能运行所需 OAuth 授权，含 `provider` 和 `env` 两个字段。`provider` 引用 client-provider.yml 注册表名；`env` 是授权成功后令牌落库的凭据名（存储键 `<skill_name>:<env>`）。缺失时触发授权准备流程。                                                                                             |

### 校验规则

- `name` 在 skill 注册表中必须唯一
- `description` 不能为空，且长度不得超过 1024 个字符
- `allowed-tools` 中的条目必须属于 MindX 工具集
- 目录名应与 `name` 字段保持一致
- `name` 不规范时系统会自动清洗一次（空格转连字符、转小写），清洗后仍不合规才报错
- **依赖未满足不阻断装载**：`requires` 检查失败（二进制缺失、环境变量未设置、版本不满足）时技能照常注册，仅生成告警；真正的硬卡点在技能运行前的准备流程（结合 `metadata.install` 引导用户补齐环境）
- frontmatter 硬错误（缺 `name`/`description`、YAML 语法错误、`---` 未闭合）才会导致技能装载失败

### 正文模板变量

SKILL.md 正文中的以下变量在加载时自动替换：

| 变量           | 替换值                                                            |
| -------------- | ----------------------------------------------------------------- |
| `{base_dir}`   | 技能目录的绝对路径（引用同目录 `scripts/`、`references/` 时使用） |
| `{skill_name}` | 技能名                                                            |

示例：`python3 {base_dir}/scripts/process.py`

## SKILL.md 示例

```markdown
---
name: sql-reviewer
description: >
  审查 SQL 查询的正确性、性能和安全性问题。
  当用户提供 SQL 查询并请求反馈或优化时使用。
allowed-tools: Read Edit Task
metadata:
  name_zh: SQL 审查者
  version: "1.0.0"
  requires:
    envs:
      - DATABASE_URL
      - name: DATABASE_PASSWORD
        prompt: 在数据库控制台的凭证页获取
        url: https://console.example.com/credentials
        sensitive: true
  description_zh: 审查 SQL 查询的正确性、性能和安全性问题
---
```

## 何时使用

- 用户问"审查这条 SQL 查询"
- 用户问"这个查询安全吗？"
- 用户问"如何优化这个查询？"

## 工作流

### 步骤 1：读取查询

从用户的消息或他们引用的文件中读取 SQL 查询。

### 步骤 2：检查正确性

验证查询语法是否有效，以及表/列名是否存在。

### 步骤 3：检查安全性

查找 SQL 注入风险、缺少参数化以及过于宽泛的权限。

### 步骤 4：检查性能

识别缺失的索引、不必要的连接、全表扫描和 N+1 模式。

### 步骤 5：提供反馈

返回一份简洁的报告，包含：

- 关键问题（必须修复）
- 警告（应该修复）
- 建议（可选改进）
- 如适用，附上修正后的查询

## 重要说明

- 除非明确要求，否则不要对数据库执行查询。
- 除非用户要求重写，否则不要修改用户文件。
- 让反馈聚焦于 SQL 特有的问题。
```
