# Skill 开发指南

本文档面向第三方开发者，介绍如何开发独立的 Skill 并使其与 Skills 系统良好集成。

## 目录结构

一个标准的 Skill 目录结构如下：

```
my-skill/
├── SKILL.md           # 技能定义文件（必需）
├── my-skill_cli.sh    # 命令行入口脚本
├── lib/               # 依赖库（可选）
└── references/        # 参考文档（可选）
    └── API_REFERENCE.md
```

## SKILL.md 文件结构

SKILL.md 是技能的核心定义文件，由 **YAML Frontmatter** 和 **Markdown 文档** 两部分组成：

```markdown
---
# YAML Frontmatter（技能元数据）
name: my-skill
description: 技能的简短描述
version: 1.0.0
category: general
tags:
  - tag1
  - tag2
os:
  - darwin
  - linux
enabled: true
timeout: 30
command: ./my-skill_cli.sh
parameters:
  param1:
    type: string
    description: 参数描述
    required: true
---

# Markdown 文档（技能详细说明）

这里是技能的详细文档...
```

## YAML Frontmatter 字段说明

### 基础字段

| 字段          | 类型     | 必需 | 说明                                                |
| ------------- | -------- | ---- | --------------------------------------------------- |
| `name`        | string   | ✅    | 技能唯一标识符，使用小写字母和连字符，如 `my-skill` |
| `description` | string   | ✅    | 技能的简短描述，用于搜索匹配和展示                  |
| `version`     | string   | ✅    | 版本号，遵循语义化版本规范，如 `1.0.0`              |
| `category`    | string   | ✅    | 技能分类，用于分组展示                              |
| `tags`        | []string | ✅    | 标签列表，用于搜索和分类                            |
| `enabled`     | bool     | ✅    | 是否启用该技能                                      |
| `os`          | []string | ❌    | 支持的操作系统列表，如 `darwin`、`linux`、`windows` |

### 执行配置

| 字段      | 类型   | 必需 | 说明                           |
| --------- | ------ | ---- | ------------------------------ |
| `command` | string | ✅    | 执行命令，相对于技能目录的路径 |
| `timeout` | int    | ❌    | 执行超时时间（秒），默认 30 秒 |

### 参数定义

`parameters` 定义技能接受的参数：

```yaml
parameters:
  city:
    type: string
    description: 城市名称，如"北京"、"New York"
    required: true
  days:
    type: number
    description: 查询天数，默认1天
    required: false
```

参数字段说明：

| 字段          | 类型   | 说明                                                       |
| ------------- | ------ | ---------------------------------------------------------- |
| `type`        | string | 参数类型：`string`、`number`、`boolean`、`array`、`object` |
| `description` | string | 参数描述，用于 LLM 理解参数用途                            |
| `required`    | bool   | 是否必需                                                   |

### 依赖声明

`requires` 声明技能运行所需的外部依赖：

```yaml
requires:
  bins:
    - curl
    - jq
  env:
    - API_KEY
    - API_HOST
```

| 字段   | 类型     | 说明                           |
| ------ | -------- | ------------------------------ |
| `bins` | []string | 需要的二进制文件（命令行工具） |
| `env`  | []string | 需要的环境变量                 |

### 安装方法

`install` 定义依赖的安装方法：

```yaml
install:
  - id: install-curl
    kind: brew
    package: curl
    label: 安装 curl
  - id: install-jq
    kind: brew
    formula: jq
    label: 安装 jq
    os:
      - darwin
```

安装方法字段说明：

| 字段      | 类型     | 说明                                                                     |
| --------- | -------- | ------------------------------------------------------------------------ |
| `id`      | string   | 安装方法唯一标识                                                         |
| `kind`    | string   | 包管理器类型：`brew`、`apt`、`yum`、`dnf`、`npm`、`pip`、`snap`、`choco` |
| `package` | string   | 包名称                                                                   |
| `formula` | string   | Homebrew formula 名称（仅 brew）                                         |
| `label`   | string   | 安装方法的显示名称                                                       |
| `os`      | []string | 该安装方法适用的操作系统                                                 |

## 完整示例

```yaml
---
name: weather
description: 天气查询技能，支持全球城市天气信息查询
version: 1.0.0
category: general
tags:
  - weather
  - forecast
os:
  - darwin
  - linux
enabled: true
timeout: 30
command: ./weather_cli.sh
parameters:
  city:
    type: string
    description: 城市名称，如"北京"、"New York"
    required: true
  days:
    type: number
    description: 查询天数，默认1天
    required: false
---

# 天气技能

## 功能说明

查询全球城市的天气信息，支持未来多天预报。

## 使用示例

```json
{
  "name": "weather",
  "parameters": {
    "city": "北京",
    "days": 3
  }
}
```

## 返回格式

返回 JSON 格式的天气数据：

```json
{
  "city": "北京",
  "forecast": [...]
}
```
```

## 命令行脚本规范

### 输入

参数通过 **标准输入 (stdin)** 以 JSON 格式传入：

```bash
#!/bin/bash

# 读取 JSON 参数
read -r json_input

# 解析参数（使用 jq）
city=$(echo "$json_input" | jq -r '.city // empty')
days=$(echo "$json_input" | jq -r '.days // 1')
```

### 输出

输出结果到 **标准输出 (stdout)**，推荐使用 JSON 格式：

```bash
# 输出 JSON 结果
echo "{\"city\": \"$city\", \"temperature\": 25}"
```

### 错误处理

错误信息输出到 **标准错误 (stderr)**，并以非零退出码退出：

```bash
if [ -z "$city" ]; then
    echo "错误：缺少必需参数 city" >&2
    exit 1
fi
```

## 搜索优化建议

Skills 系统使用向量搜索和关键词匹配来查找相关技能。以下是优化技能可搜索性的建议：

### 1. 编写精准的 description

`description` 是搜索匹配的核心字段，应该：

- **清晰描述功能**：说明技能能做什么
- **包含关键动词**：如"查询"、"发送"、"管理"、"获取"
- **包含关键名词**：如"天气"、"邮件"、"系统信息"
- **避免冗余**：不要重复 name 字段的内容

**好的示例：**
```yaml
description: 查询全球城市天气信息，支持当前天气和未来预报
```

**不好的示例：**
```yaml
description: 天气技能
```

### 2. 选择合适的 tags

标签用于分类和辅助搜索：

- 使用**通用标签**：如 `system`、`communication`、`productivity`
- 使用**功能标签**：如 `weather`、`email`、`calendar`
- 使用**平台标签**：如 `macos`、`linux`
- **数量适中**：建议 2-5 个标签

### 3. 选择合适的 category

系统预定义的分类：

| 分类            | 说明       |
| --------------- | ---------- |
| `general`       | 通用工具   |
| `system`        | 系统操作   |
| `communication` | 通信相关   |
| `productivity`  | 生产力工具 |
| `media`         | 多媒体处理 |
| `automation`    | 自动化工具 |

### 4. 参数描述的重要性

参数的 `description` 字段会被纳入搜索索引，因此：

- 清晰说明参数用途
- 提供示例值
- 说明默认值（如果有）

**好的示例：**
```yaml
parameters:
  city:
    type: string
    description: 城市名称，支持中文和英文，如"北京"、"New York"、"Tokyo"
    required: true
```

### 5. Markdown 文档内容

Markdown 文档部分也会被索引，建议包含：

- **功能说明**：详细描述技能能做什么
- **使用场景**：什么情况下使用该技能
- **示例**：具体的调用示例
- **注意事项**：使用时需要注意的问题

## 开发注意事项

### 1. 跨平台兼容性

- 使用 `os` 字段声明支持的操作系统
- 在脚本中检查运行环境
- 为不同平台提供不同的安装方法

### 2. 安全性

- 不要在代码中硬编码敏感信息
- 使用环境变量存储 API 密钥等敏感数据
- 在 `requires.env` 中声明所需的环境变量

### 3. 错误处理

- 验证必需参数是否存在
- 提供有意义的错误信息
- 使用适当的退出码

### 4. 性能考虑

- 合理设置 `timeout` 值
- 避免长时间运行的阻塞操作
- 对于耗时操作，考虑提供进度反馈

### 5. 日志输出

- 正常输出到 stdout
- 错误和调试信息输出到 stderr
- 避免在 stdout 输出非结果内容

## 测试技能

### 本地测试

```bash
# 进入技能目录
cd skills/my-skill

# 测试脚本
echo '{"city": "北京"}' | ./my-skill_cli.sh

# 检查输出格式
echo '{"city": "北京"}' | ./my-skill_cli.sh | jq .
```

### 验证 SKILL.md

确保 YAML frontmatter 格式正确：

```bash
# 提取并验证 YAML
sed -n '/^---$/,/^---$/p' SKILL.md | head -n -1 | tail -n +2
```

## MCP 技能开发

除了传统的本地脚本技能外，系统还支持通过 MCP (Model Context Protocol) 协议连接外部工具和服务。

### MCP 技能特点

- **零学习成本**：SKILL.md 格式完全不变，仅通过 metadata 标记
- **统一体验**：MCP 技能在前端显示、搜索、执行等方面与普通技能完全一致
- **强大扩展**：可以连接任何支持 MCP 协议的服务和工具

### MCP 技能定义

MCP 技能通过 `metadata.mcp` 字段标记：

```yaml
---
name: mcp-filesystem
description: 通过 MCP 协议访问文件系统，支持文件读写和目录操作
version: 1.0.0
category: system
tags:
  - mcp
  - filesystem
  - file
os:
  - darwin
  - linux
enabled: true
timeout: 30
parameters:
  path:
    type: string
    description: 文件或目录路径
    required: true
  action:
    type: string
    description: 操作类型：read, write, list
    required: true
  content:
    type: string
    description: 文件内容（write 操作时必需）
    required: false
metadata:
  mcp:
    server: "filesystem"
    tool: "filesystem_operation"
---

# MCP 文件系统技能

通过 MCP 协议提供文件系统操作能力。
```

### metadata.mcp 字段说明

| 字段     | 类型   | 必需 | 说明           |
| -------- | ------ | ---- | -------------- |
| `server` | string | ✅    | MCP 服务器名称 |
| `tool`   | string | ✅    | MCP 工具名称   |

### MCP 技能 vs 本地技能

| 特性          | 本地技能       | MCP 技能     |
| ------------- | -------------- | ------------ |
| SKILL.md 格式 | 完全相同       | 完全相同     |
| 执行方式      | 本地命令行脚本 | MCP 协议调用 |
| 前端显示      | [std] 标记     | [MCP] 标记   |
| 搜索索引      | 支持           | 支持         |
| 统计记录      | 支持           | 支持         |

### 前端显示

MCP 技能在技能管理页面会显示 `[MCP]` 粉色标签，便于识别。格式筛选器也支持专门筛选 MCP 技能。

## 发布清单

在发布技能前，请确认：

- [ ] SKILL.md 包含所有必需字段
- [ ] description 清晰描述技能功能
- [ ] tags 和 category 正确分类
- [ ] 参数定义完整且描述清晰
- [ ] （本地技能）命令行脚本可执行
- [ ] （本地技能）错误处理完善
- [ ] （MCP 技能）metadata.mcp.server 和 metadata.mcp.tool 正确配置
- [ ] 本地测试通过
- [ ] 跨平台兼容性已考虑
