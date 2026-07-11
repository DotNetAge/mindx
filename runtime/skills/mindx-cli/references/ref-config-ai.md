# AI 能力配置

用于配置 Provider、模型、Agent、Skill 和权限规则的相关命令。
这些命令定义了 **MindX 系统能做什么** —— 调用哪些 LLM、有哪些 Agent 配置、可用哪些 Skill，以及哪些工具被允许或禁止。

## Provider（LLM API 端点）

定义 MindX 连接哪个端点来获取语言模型的响应。

### 离线命令（无需守护进程）

| 任务 | 命令 | 说明 |
|------|------|------|
| 列出所有 Provider | `mindx provider list` | 表格展示：名称 / 标题 / Base URL / API Key（脱敏）/ 是否本地 |
| 以 JSON 格式列出 | `mindx provider list --json` | 机器可读输出 |
| 添加或更新 Provider | `mindx provider add --name <name> --base-url <url>` | 写入 `providers.yml` |
| 设置标题 | `mindx provider add --name openai --title "OpenAI"` | 便于识别的名称 |
| 存储 API Key 引用 | `mindx provider add --api-key OPENAI_API_KEY` | 存储环境变量名（而非 Key 本身） |
| 标记为本地模型 | `mindx provider add --local` | 适用于 Ollama、LM Studio 等 |
| 移除 Provider | `mindx provider rm <name>` | 从配置中删除 |
| 存储实际 API Key | `mindx provider setkey <provider> <key>` | 存入 macOS 钥匙串或加密文件 |

### 守护进程命令（需要运行守护进程）

| 任务 | 命令 | 说明 |
|------|------|------|
| 通过守护进程创建 | `mindx provider create --name <n> --title <t> --base-url <url> --api-key <key>` | 由守护进程管理存储 |
| 通过守护进程更新 | `mindx provider update --name <n> --base-url <new-url>` | 更新已有配置 |
| 通过守护进程删除 | `mindx provider delete --name <name>` | 由守护进程管理删除 |

### 示例

```bash
# 云端 Provider（DashScope/阿里云）
mindx provider add --name dashscope \
  --title "DashScope" \
  --base-url https://dashscope.aliyuncs.com/compatible-mode/v1 \
  --api-key DASHSCOPE_API_KEY
mindx provider setkey dashscope sk-xxxxxxxxxxxx

# 本地 Provider（Ollama）
mindx provider add --name ollama \
  --title "Ollama Local" \
  --base-url http://localhost:11434 \
  --local
```

## 模型（LLM 定义）

定义具体可用的模型，需关联到某个 Provider。

| 任务 | 命令 | 说明 |
|------|------|------|
| 列出模型 | `mindx model list` | 表格展示：名称 / Provider / 上下文长度 / 最大 Token 数 / 函数调用 / 是否启用 |
| 以 JSON 格式列出 | `mindx model list --json` | 机器可读；需要守护进程 |
| 添加模型 | `mindx model add --name <name> --provider <prov>` | 最少需要这些字段 |
| 设置显示名称 | `mindx model add ... --title "Qwen Max"` | 便于识别的名称 |
| 设置上下文长度 | `mindx model add ... --context-length 32000` | 默认值因模型而异 |
| 设置最大输出 Token 数 | `mindx model add ... --max-tokens 4096` | 响应 Token 上限 |
| 启用函数调用 | `mindx model add ... --func-calling` | 工具调用型模型必须开启 |
| 启用网络搜索 | `mindx model add ... --web-searching` | 模型具备搜索能力 |
| 设置生成参数 | `mindx model add ... --temperature 0.7 --top-p 0.9 --repetition-penalty 1.0` | 采样参数 |
| 启用/禁用模型 | `mindx model add ... --enabled=false` | 默认启用；使用 `--enabled=false` 禁用 |
| 设为默认模型 | `mindx model set <model-name>` | 新建 Session 时使用 |
| 切换当前模型 | `mindx model switch --name <model>` | **需要守护进程** —— 更改当前 Session 的模型 |
| 切换时指定 Provider | `mindx model switch --name <model> --provider <prov>` | 不同 Provider 下有同名模型时用于区分 |
| 移除模型 | `mindx model rm <name>` | 从配置中删除 |

### 示例
```bash
mindx model add --name qwen-max \
  --provider dashscope \
  --context-length 32000 \
  --max-tokens 4096 \
  --func-calling \
  --temperature 0.7
mindx model set qwen-max
```

## Agent（AI Agent 配置）

定义 Agent 的角色、描述、技能和模型等配置。

### 离线命令

| 任务 | 命令 | 说明 |
|------|------|------|
| 列出 Agent | `mindx agent list` | 表格展示：名称 / 角色 / 模型 / Skill 数量 |
| 以 JSON 格式列出 | `mindx agent list --json` | 完整详情，需通过守护进程 |
| 创建新 Agent | `mindx agent add <name> --role "<role>"` | 创建 `.md` 文件 + YAML frontmatter |
| 设置描述 | `mindx agent add ... --description "..."` | 描述该 Agent 的功能 |
| 分配 Skill | `mindx agent add ... --skills find-experts,introspect` | 逗号分隔的 Skill 名称 |
| 删除 Agent | `mindx agent rm <name>` | 移除 Agent 文件 |

> 注意：Agent 的默认模型通过 `mindx agent update --model <model>` 设置，不在 `agent add` 时设置。

### 守护进程命令

| 任务 | 命令 | 说明 |
|------|------|------|
| 获取完整 Agent 配置 | `mindx agent get <name>` | 以 JSON 返回完整的 YAML frontmatter |
| 部分更新 Agent | `mindx agent update --agent-name <name>` | 无需重写即可修改任意字段 |
| 更新角色 | `mindx agent update ... --role "Senior CSM"` | |
| 更新描述 | `mindx agent update ... --description "..."` | |
| 更新自我介绍 | `mindx agent update ... --introduction "..."` | Agent 的自我介绍提示词 |
| 更换模型 | `mindx agent update ... --model gpt-4o` | |
| 替换 Skill 集合 | `mindx agent update ... --skills s1,s2,s3` | 全量替换，非追加 |
| 排除工具 | `mindx agent update --exclude-tools fs.write,bash` | 禁止使用特定工具 |
| 为 Agent 评分 | `mindx agent score --agent-name <n> --task "<desc>" --score 8 --notes "..."` | 1-10 分制，存入 KVStore |
| 获取 Agent 评分 | （通过 `mindx kv list --prefix score:`） | 查询已存储的评分 |
| 从磁盘重新加载 Agent | `mindx reload agents` | 重新扫描 `~/.mindx/agents/`，无需重启 |

### 示例
```bash
mindx agent add csm-lead \
  --role "Customer Success Lead" \
  --description "Manages enterprise accounts, runs QBRs" \
  --skills find-experts,customer-success,content-ops

# 然后单独设置模型
mindx agent update --agent-name csm-lead --model gpt-4o
```

## Skill（已安装的 Skill 检查器）

查看、安装和验证从 `~/.mindx/skills/` 加载的 Skill。

| 任务 | 命令 | 说明 |
|------|------|------|
| 列出已安装的 Skill | `mindx skill list` | 名称 / 描述 / 允许使用的工具 |
| 以 JSON 格式列出 | `mindx skill list --json` | 包含完整的 frontmatter 元数据；需要守护进程 |
| 查看 Skill 详情 | `mindx skill get <name>` | 显示 SKILL.md 的内容 |
| 从本地目录安装/更新 | `mindx skill add <path>` | 将 Skill 复制到 `~/.mindx/skills/` |
| 验证 Skill 结构 | `mindx skill validate <name>` | 检查 frontmatter 和目录结构 |
| 验证评测测试集 | `mindx skill eval <name>` | 检查 `evals/evals.json` 的 Schema |
| 从磁盘重新加载 Skill | `mindx reload skills` | 重新扫描 `~/.mindx/skills/`，无需重启 |

## 权限规则（工具访问控制）

定义 Agent 可以使用哪些工具。规则会被注入到系统提示词中。

| 任务 | 命令 | 说明 |
|------|------|------|
| 列出所有规则 | `mindx rule list` | 当前的 allow/deny/ask 规则 |
| 获取规则详情 | `mindx rule get --id <tool-name>` | 完整的规则定义 |
| 创建新规则 | `mindx rule create --id <id> --intro "<instruction>"` | 新建工具权限规则 |
| 设置作用域 | `mindx rule create ... --scope global|local|conversation` | 规则的生效范围 |
| 设置优先级 | `mindx rule create ... --priority <int>` | 整数优先级，数值越大越优先 |
| 创建时不启用 | `mindx rule create ... --enabled false` | 创建但暂不激活 |
| 更新规则 | `mindx rule update --id <id> --intro "..."` | 修改已有规则 |
| 切换启用状态 | `mindx rule update --id <id> --enabled true/false` | |
| 删除规则 | `mindx rule delete --id <id>` | 永久移除 |

## 典型配置流程

```bash
# 1. 先配置 Provider
mindx provider add --name dashscope --base-url <url> --api-key DASHSCOPE_KEY
mindx provider setkey dashscope sk-real-key-here

# 2. 再配置模型
mindx model add --name qwen-max --provider dashscope --context-length 32000 --func-calling
mindx model set qwen-max

# 3. 然后配置 Agent
mindx agent add executive-assistant --role "Executive Assistant" \
  --model qwen-max --skills find-experts,introspect

# 4. 验证配置
mindx status
mindx provider list
mindx model list
mindx agent list
```
