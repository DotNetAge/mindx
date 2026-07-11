# 文件操作、文件监控与系统运维

通过守护进程进行文件系统访问、文件变更监控以及其他杂项系统操作。**大多数命令需要守护进程处于运行状态。**

## 文件系统（fs）

通过守护进程的工作目录访问文件。适用于 Agent 需要在受控上下文中读写文件的场景。

| 任务 | 命令 | 说明 |
|------|------|------|
| 列出目录 | `mindx fs ls /path/to/dir` | 也可用 `mindx fs list` |
| 以 JSON 格式列出目录 | `mindx fs list /path/to/dir --json` | 机器可读输出 |
| 读取文件 | `mindx fs read /path/to/file` | 返回文件内容 |
| 写入文件 | `mindx fs write /path/to/file --content "..."` | 创建或覆盖；也支持 stdin 输入 |
| 创建目录 | `mindx fs mkdir /path/new-dir` | 单级目录 |
| 创建多级目录 | `mindx fs mkdir -p /a/b/c/deep` | `--parents` 参数 |
| 删除文件 | `mindx fs rm /path/to/file` | |
| 递归删除 | `mindx fs rm -r /path/to/dir` | `--recurse` 参数 |
| 强制删除 | `mindx fs rm -f /path/to/file` | 不弹确认提示 |
| 移动/重命名 | `mindx fs mv /src/path /dst/path` | 文件和目录均可 |
| 显示主目录 | `mindx fs home` | 守护进程配置的主目录/工作路径 |

### 何时用 `fs`，何时用 bash
- 在守护进程管理的上下文中（Session、项目）操作时，使用 **`mindx fs`**
- 在守护进程范围之外进行通用系统操作时，使用**原生 bash**（`cat`、`ls` 等）

## 文件监控（fw）

实时监控文件变更。用于守护进程的 Session 文件追踪。
**所有 `fw` 命令需要守护进程处于运行状态。**

| 任务 | 命令 | 说明 |
|------|------|------|
| 启动监控 | `mindx fw start` | 开始监控已配置的路径 |
| 停止监控 | `mindx fw stop` | 停止监控 |
| 检查状态 | `mindx fw status` | 是否在运行？正在监控哪些路径？ |
| 以 JSON 格式检查状态 | `mindx fw status --json` | 机器可读输出 |

## 守护进程日志（log API）

通过守护进程获取详细日志（补充 `mindx logs` CLI 命令）。

| 任务 | 命令 | 说明 |
|------|------|------|
| 分页读取（最新在前） | `mindx log read --limit 30` | 按时间倒序 |
| 从偏移量读取 | `mindx log read --offset 200 --limit 30` | 用于翻阅大量日志 |
| 仅错误流 | `mindx log read --limit 50 --stream error` | 只过滤错误 |
| 主/信息流 | `mindx log read --limit 50 --stream main` | 常规日志条目 |
| 以 JSON 格式读取日志 | `mindx log read --limit 30 --json` | 机器可读输出 |
| 清除所有日志 | `mindx log clear --confirm` | **破坏性操作** —— 布尔标志，必须确认才能清除 |
| 按流统计数量 | `mindx log count` | 每个流的条目数 |
| 以 JSON 格式统计 | `mindx log count --json` | 机器可读输出 |

> 注意：`mindx logs -n 50` 直接从磁盘读取日志文件。
> `mindx log read --limit 50` 通过守护进程 API 读取。
> 需要结构化/分页访问时，使用后者。

## 用户配置

显示守护进程中当前生效的用户配置。
**需要守护进程处于运行状态。**

| 任务 | 命令 | 说明 |
|------|------|------|
| 显示用户配置 | `mindx user config` | 当前用户设置的键值对 |
| 以 JSON 格式显示用户配置 | `mindx user config --json` | 机器可读输出 |

## 实体标签

管理 GraphRAG 索引器使用的实体类型定义。
**需要守护进程处于运行状态。**

| 任务 | 命令 | 说明 |
|------|------|------|
| 获取实体标签定义 | `mindx entity-tags get` | 列出所有已定义的实体类型及其描述 |
| 以 JSON 格式获取实体标签 | `mindx entity-tags get --json` | 机器可读输出 |
| 保存实体标签定义 | `mindx entity-tags save --types '[{...}]'` | 定义自定义实体用于图谱提取 |

### 实体标签格式
```json
[
  {
    "name": "Company",
    "title": "公司",
    "desc": "商业组织",
    "category": "core"
  },
  {
    "name": "Product",
    "title": "产品",
    "desc": "商品或服务",
    "category": "core"
  }
]
```

这些定义会被注入到 LLMIndexer 的系统提示词中，让它在 GraphRAG 索引过程中知道该提取哪些实体类型。

## 工具命令

不需要守护进程的本地工具命令。

| 任务 | 命令 | 说明 |
|------|------|------|
| 生成 UUID v4 | `mindx utils uuid` | 随机 UUID |
| 生成 ULID | `mindx utils ulid` | 可排序的唯一标识符 |
| 计算 SHA-256 | `mindx utils sha "text"` | 输入文本的十六进制摘要 |
