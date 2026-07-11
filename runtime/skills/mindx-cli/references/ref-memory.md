# 记忆、知识库与键值存储

三个互补的持久化层：
- **Memory（RAG）**：基于语义向量的非结构化知识搜索 —— 需要守护进程
- **知识库（KB）**：项目文件索引与文档搜索 —— 需要守护进程
- **KV 存储**：简单的键值对，用于结构化数据 —— 需要守护进程

> 也支持离线使用：`mindx query <terms>` 可通过本地 Embedder 搜索记忆存储，无需守护进程。

## Memory（长期记忆 RAG）

将语义内容存储为向量嵌入。按语义搜索，而非关键词匹配。
所有 memory 命令**需要守护进程**处于运行状态。

### 搜索

| 任务 | 命令 | 说明 |
|------|------|------|
| 语义搜索 | `mindx memory query "architecture decisions"` | 向量相似度搜索 |
| 限制结果数量 | `mindx memory query "..." --limit 10` | 默认值因情况而异 |
| 最低相关性分数 | `mindx memory query "..." --min-score 0.7` | 过滤低质量匹配 |
| 以 JSON 格式输出 | `mindx memory query "..." --json` | 机器可读输出 |

> 也支持离线使用：`mindx query <terms>` —— 使用本地 Embedder，无需守护进程。
> 添加 `--json` 可获得机器可读输出。

### 存储

| 任务 | 命令 | 说明 |
|------|------|------|
| 存储新内容 | `mindx memory store --content "..."` | 最少需要此字段 |
| 设置标题 | `mindx memory store ... --title "Meeting Notes"` | 用于展示和提升搜索相关性 |
| 设置描述 | `mindx memory store ... --description "QBR with Acme"` | 补充上下文信息 |
| 标记来源 | `mindx memory store ... --source "customer-success-cycle"` | 追踪数据来源 |

### 管理

| 任务 | 命令 | 说明 |
|------|------|------|
| 按 ID 删除 | `mindx memory delete --id <uuid>` | 需要提供 chunk 的精确 ID |
| 列出 chunk（分页） | `mindx memory chunks --page 1 --page-size 20` | 浏览已存储的内容 |
| 按文档过滤 chunk | `mindx memory chunks --doc-id <id>` | 仅显示特定来源文档的 chunk |
| 以 JSON 输出 chunk | `mindx memory chunks --json` | 机器可读输出 |
| 获取文档的 chunk | `mindx memory get-chunks --doc-id <id>` | 获取某来源文档的所有 chunk |
| 以 JSON 输出文档 chunk | `mindx memory get-chunks --doc-id <id> --json` | 机器可读输出 |
| 统计总记录数 | `mindx memory count` | 快速查看总数 |

### 典型工作流
```bash
# 重要会议结束后：
mindx memory store \
  --content "Decided to use PostgreSQL for the analytics DB. Migration planned for Q3." \
  --title "Architecture Decision: Analytics DB" \
  --source "meeting-2026-06-15" \
  --description "Database selection decision from engineering review"

# 之后有人问起数据库选型时：
mindx memory query "database decision architecture"
```

## 知识库（KB）

面向项目的文档索引与搜索。所有 `kb` 命令**需要守护进程**。

| 任务 | 命令 | 说明 |
|------|------|------|
| 语义搜索 | `mindx kb search "project architecture"` | 搜索已索引的项目文档 |
| 限制结果数量 | `mindx kb search "..." --limit 20` | 默认 10 |
| 最低分数 | `mindx kb search "..." --min-score 0.5` | 按相关性过滤 |
| 以 JSON 格式输出 | `mindx kb search "..." --json` | 机器可读输出 |
| 索引统计 | `mindx kb stats --project-dir /path` | 总记录数、存储量、索引信息 |
| 以 JSON 输出统计 | `mindx kb stats --project-dir /path --json` | 机器可读输出 |
| 同步项目文件 | `mindx kb sync --project-dir /path/to/project` | 重新索引整个项目 |
| 索引单个路径 | `mindx kb index path/to/file.md` | 索引单个文件或目录 |
| 强制重新索引 | `mindx kb index --force path/to/file.md` | 跳过缓存，强制重新索引 |
| 检查文件同步状态 | `mindx kb file-states --project-dir /path` | 已索引 / 已变更 / 新增 / 已移除 |
| 以 JSON 输出文件状态 | `mindx kb file-states --project-dir /path --json` | 机器可读输出 |

### 典型工作流
```bash
# 索引一个项目
mindx kb sync --project-dir ./myproject

# 搜索已索引的文档
mindx kb search "API design decisions"

# 查看变更情况
mindx kb file-states --project-dir ./myproject
```

## KV 存储（键值持久化）

简单的持久化键值存储。被任务管理、Agent 评分、团队追踪等工具使用。所有 `kv` 命令**需要守护进程**。

### 基本操作

| 任务 | 命令 | 说明 |
|------|------|------|
| 获取值 | `mindx kv get --key <key>` | 返回 JSON 值 |
| 以 JSON 格式获取值 | `mindx kv get --key <key> --json` | 机器可读输出 |
| 设置值 | `mindx kv set --key <key> --value '<json>'` | 值必须是合法的 JSON |
| 设置 TTL | `mindx kv set --key <key> --value '<json>' --ttl 3600` | N 秒后自动删除 |
| 删除键 | `mindx kv delete --key <key>` | |
| 按前缀列出键 | `mindx kv list --prefix "tasks_"` | 查找匹配前缀的所有键 |
| 限制列表结果数 | `mindx kv list --prefix "score:" --limit 10` | 分页 |
| 同时显示值 | `mindx kv list --prefix "config:" --with-values` | 输出中包含值 |
| 以 JSON 输出键列表 | `mindx kv list --prefix "config:" --json` | 机器可读输出 |

### 批量操作

| 任务 | 命令 | 说明 |
|------|------|------|
| 原子批量写入 | `mindx kv batch-set --entries '[{"key":"a","value":1},{"key":"b","value":2}]'` | 全部成功或全部失败；支持每条设置可选的 `ttl` |
| 按前缀批量删除 | `mindx kv clear --prefix "cache:"` | **破坏性操作** —— 删除所有匹配的键 |

### 键名约定（内置工具使用的键前缀）

系统使用特定的键前缀。了解这些前缀有助于高效查询：

| 前缀 | 所属模块 | 格式 | 示例 |
|------|---------|------|------|
| `tasks_` | 任务工具 | `tasks_{sessionID}_{taskID}` | 包含 status/owner/metadata 的任务 JSON |
| `teams_` | 团队工具 | `teams_{sessionID}_{teamName}` | 包含 members/taskIDs 的团队 JSON |
| `score:` | Agent 评分 | `score:{agent_name}:{unix_nano}` | `{agent_name, task, score, timestamp, notes}` |
| `tran:` | 翻译缓存 | `tran:{hash}` | 缓存的翻译结果 |
| `kg:` | 知识图谱缓存 | `kg:{query_hash}` | 图谱查询结果缓存 |

### 示例
```bash
# 查看当前 Session 有哪些任务
mindx kv list --prefix "tasks_abc123"

# 获取 Agent 评分用于绩效回顾
mindx kv list --prefix "score:csm-lead" --with-values

# 清除过期缓存
mindx kv clear --prefix "cache:"
```

## Memory vs KB vs KV：如何选择

| 需求 | 使用 | 原因 |
|------|------|------|
| "我们关于 X 做了什么决定？"（对话记忆） | `memory query` | 对存储的记忆记录进行语义搜索 |
| "存储这条会议笔记" | `memory store` | 非结构化内容，按语义搜索 |
| "搜索项目文档" | `kb search` | 对已索引的项目文件进行语义搜索 |
| "索引项目文件" | `kb sync` | 面向项目的文档索引 |
| "获取任务 #42 的状态" | `kv get --key tasks_..._task-42` | 精确键查找，快速准确 |
| "记录 Agent 得分 8/10" | `kv set --key score:...` | 结构化数据，非语义搜索 |
| "列出我的所有任务" | `kv list --prefix tasks_...` | 对结构化键进行前缀扫描 |
| "找跟数据库相关的内容"（离线） | `mindx query "database"` | 无需守护进程的模糊语义匹配 |
