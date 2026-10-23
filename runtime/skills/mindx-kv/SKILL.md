---
name: mindx-kv
description: >
  使用 MindX daemon 的持久键值存储读写扩展状态：配置项、缓存、检查点、计数器。
  支持 TTL 过期、批量原子写、前缀浏览与清理。当需要精确键值读写（而非语义检索
  mindx-rag，或关系查询 mindx-graph）时使用。
allowed-tools:
  - Bash(mindx kv *)
metadata:
  version: 1.0.0
  author: Ray
  name_zh: MindX 键值存储
  name_zh-tw: MindX 鍵值存儲
  description_zh: daemon 持久键值存储：状态、缓存与配置的精确读写
  description_zh-tw: daemon 持久鍵值存儲：狀態、快取與配置的精確讀寫
---

# MindX 键值存储（kv）

你正在通过 `mindx kv` CLI 读写 daemon 的全局键值存储（bbolt，位于
`~/.mindx/data/kv.db`）。所有命令要求 daemon 运行中（`mindx start`）。

## 心智模型

- **键是精确匹配的**：不做语义检索、不做模糊匹配。键名设计建议用
  `域:对象:属性` 冒号分层（如 `plugin:websearch:state`），list/clear 的
  前缀过滤按此前缀生效。
- **值是 JSON**：任何合法 JSON（字符串要带引号：`--value '"dark"'`）。
- **TTL 可选**：ttl 秒后自动过期；0 表示永不过期。

## 命令参考

### `mindx kv get --key <键>`

取单个键。输出 `{"found":true,"item":{...}}` 或 Key not found。

```
mindx kv get --key "plugin:websearch:state"
```

### `mindx kv set --key <键> --value <JSON值> [--ttl 秒]`

写单个键。

```
mindx kv set --key "plugin:websearch:state" --value '{"enabled":true}' --ttl 86400
mindx kv set --key "app:theme" --value '"dark"'
```

### `mindx kv delete --key <键>`

删除单个键。

### `mindx kv list --prefix <前缀> [--limit N] [--with-values] [--json]`

按前缀列出键（默认只列键名，--with-values 附带值）。

```
mindx kv list --prefix "plugin:" --with-values
```

### `mindx kv batch-set --entries <JSON数组>`

原子批量写。任一条目失败则全部不生效。

```
mindx kv batch-set --entries '[{"key":"user:1","value":"Alice"},{"key":"user:2","value":"Bob","ttl":86400}]'
```

### `mindx kv clear --prefix <前缀>`

删除该前缀下全部键。破坏性操作，先 list 确认范围再执行。

## 使用建议

1. **写前先 get/list**：确认现状再覆盖；批量写用 batch-set 保证原子性。
2. **前缀即命名空间**：扩展用自己的前缀（如 `plugin:<名称>:`）隔离数据，
   清理时 `clear --prefix` 一次摘除。
3. **临时数据务必带 TTL**：缓存、锁、运行中标记都应设过期，避免残留。

## 与另两个存储技能的分工

| 需求 | 用哪个 |
|---|---|
| 精确读写一个状态/配置/缓存 | `mindx-kv`（本技能） |
| "我记得有个东西，按语义取回来" | `mindx-rag` |
| 表达并查询实体之间的关系 | `mindx-graph` |
