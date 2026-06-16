# 存储层诊断

> 诊断 MindX 的持久化存储层：GraphRAG 图数据库、KVStore、Session 存储、模型文件。

## 存储架构概览

```
~/.mindx/
├── data/
│   ├── models/           # Embedder ONNX 模型 (~169 MB)
│   │   └── model_q4.onnx
│   ├── graph/            # GraphRAG bbolt 图数据库
│   │   └── *.db
│   └── rules.yml         # 规则配置
├── *.db                  # KVStore (bbolt) — score/token-usage 等
├── sessions/             # 会话数据（对话历史、backup）
│   └── {agent}/{session_id}/
│       ├── backup/
│       └── ...
├── logs/                 # 日志（zap JSON + launchd 重定向）
├── mindx.json            # 主配置
└── settings/             # providers/models 配置
```

## 各存储组件诊断

### 1. GraphRAG 图数据库 (`data/graph/`)

**健康指标：**

| 指标 | 健康值 | 检查命令 |
|------|--------|---------|
| 目录存在 | ✅ | `test -d ~/.mindx/data/graph && echo OK` |
| .db 文件可读 | ✅ | `file ~/.mindx/data/graph/*.db` |
| 文件大小合理 | < 100 MB | `du -sh ~/.mindx/data/graph/` |
| 无 lock 残留 | ✅ | `ls ~/.mindx/data/graph/*.lock 2>/dev/null \| wc -l` 应为 0 |

**常见问题：**

| 问题 | 日志证据 | 修复 |
|------|---------|------|
| 目录缺失 | `"knowledge-graph database unavailable"` | `mkdir -p && chmod 755 && restart` |
| 文件损坏 | bbolt read 返回 corruption error | 备份后删除 .db，restart 重建 |
| 文件过大 (>500MB) | 磁盘占用高 | 导出关键实体后清空重建 |
| lock 文件残留 | 上次 crash 后未清理 | `rm *.lock && restart` |

**重建 GraphRAG 数据库（慎用）：**
```bash
# 1. 备份
cp -r ~/.mindx/data/graph/ ~/mindx-graph-backup-$(date +%Y%m%d)/

# 2. 停止 daemon
mindx stop

# 3. 删除图数据库文件
rm -f ~/.mindx/data/graph/*.db

# 4. 重启（daemon 会在首次查询时自动创建新库）
mindx start

# 注意：重建后所有之前索引的实体关系都会丢失，
# 需要重新通过 research-pipeline 或手动添加文档来建立知识图谱
```

---

### 2. KVStore (`~/.mindx/*.db`)

**用途：** Agent score、Token usage 统计、Translate cache、Entity tags

**健康指标：**

| 指标 | 健康值 |
|------|--------|
| 文件存在 | ✅（首次使用后自动创建） |
| 文件大小 | 通常 < 10 MB |
| 可读写 | ✅ |

**常见问题：**

| 问题 | 证据 | 修复 |
|------|------|------|
| 初始化失败 | `"failed to initialize kvstore"` | 检查磁盘空间+权限；备份后删除重建 |
| 文件损坏 | KV 操作返回 error | 同上 |
| 文件过大 | > 50 MB | 检查是否有异常大量的 key（如 token usage 记录爆炸） |

**查看 KVStore 内容（调试用）：**
```bash
# 用 bbolt CLI 查看（如果安装了）
# 或通过 MindX 的 RPC 接口查询
# 目前没有直接的 CLI 工具，需通过 daemon API
```

**数据影响评估：**
- **Agent score 丢失**: 可重新评分，影响不大
- **Token usage 丢失**: 统计数据归零，不影响功能
- **Translate cache 丢失**: 已翻译的内容需要重新翻译
- **Entity tags 丢失**: 自定义标签丢失

---

### 3. Session 存储 (`sessions/`)

**健康指标：**

| 指标 | 健康值 | 检查命令 |
|------|--------|---------|
| 会话总数 | < 200 | `find ~/.mindx/sessions/ -type d -mindepth 2 \| wc -l` |
| 总大小 | < 500 MB | `du -sh ~/.mindx/sessions/` |
| 最大单个会话 | < 10 MB | `du -sh ~/.mindx/sessions/*/* \| sort -rh \| head -5` |
| 无 orphan 会话 | ✅ | 会话 ID 应在 daemon memory 中有记录 |

**Session 垃圾回收策略：**

```bash
# 清理 14 天前的会话（保留最近的）
find ~/.mindx/sessions/ -type d -mindepth 2 -mtime +14 -print -exec rm -rf {} + 2>/dev/null

# 清理空目录
find ~/.mindx/sessions/ -type d -empty -delete 2>/dev/null
```

**注意：** 删除活跃会话会导致用户下次打开项目时创建新 session（旧对话历史丢失）。只清理确定已过期的。

---

### 4. Embedder 模型 (`data/models/model_q4.onnx`)

**健康指标：**

| 指标 | 健康值 |
|------|--------|
| 文件存在 | ✅ |
| 文件大小 | ~169 MB |
| 文件完整性 | `file` 命令显示 "data" (非 corrupt) |

**如果模型文件缺失或损坏：**
```bash
# 重新下载
mindx install  # 会检测并下载缺失的模型
```

---

## 存储层关联诊断矩阵

当一个存储组件出问题时，检查其级联影响：

```
KVStore 故障
├── Agent score 不可用 → introspect 技能部分失效
├── Token usage 丢失   → 成本统计归零
├── Translate cache 失效 → 翻译功能变慢（需重新调用 API）
└── Entity tags 丢失   → 自定义标签重置

Graph DB 故障
├── GraphRAG 查询全部失败 → research-pipeline 的图谱能力失效
├── Entity extraction 不可用 → 新文档无法提取实体
└── Cypher 查询报错       → 所有图操作返回 error

Session 异常
├── 对话历史丢失           → 用户感知：之前的对话找不到了
├── Backup 不可用          → 文件 diff 功能失效
└── Project context 丢失   → 新 session 需要重新建立上下文
```
