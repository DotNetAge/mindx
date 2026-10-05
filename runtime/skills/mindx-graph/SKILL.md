---
name: mindx-graph
description: >
  使用 MindX daemon 的知识图谱存储与查询扩展中的实体关系：批量写节点/边、
  按 ID 取节点、多跳邻居查询、Cypher 读写。当扩展开发需要表达并查询
  "谁依赖谁、谁属于谁、谁关联谁"这类关系时使用。精确键值用 mindx-kv，
  语义取回条目用 mindx-rag。
allowed-tools:
  - Bash(mindx graph *)
metadata:
  name_zh: MindX 知识图谱
  name_zh-tw: MindX 知識圖譜
  description_zh: 知识图谱：实体关系的写入、遍历与 Cypher 查询
  description_zh-tw: 知識圖譜：實體關係的寫入、遍歷與 Cypher 查詢
---

# MindX 知识图谱（graph）

你正在通过 `mindx graph` CLI 操作 daemon 的知识图谱（gograph，位于
`~/.mindx/data/kb.db`）。所有命令要求 daemon 运行中（`mindx start`）。

## 心智模型

- **节点** = (id, labels, properties)，**边** = (from_node_id, to_node_id, type)。
  upsert 语义：同 ID 重复写即更新。
- **关系靠边表达**：type 用大写下划线动语（如 `DEPENDS_ON`、`BELONGS_TO`、
  `CALLS`），与 MindX 内置图谱（Region/Document/实体）风格一致。
- **扩展数据用自己的 ID 约定**（如 `plugin:websearch`），与 mindx-kv 的键、
  mindx-rag 的族通过同一命名互相关联。

## 命令参考

### `mindx graph upsert-nodes --nodes <JSON数组>`

批量创建/更新节点。

```
mindx graph upsert-nodes --nodes '[
  {"id":"plugin:websearch","labels":["Plugin"],"properties":{"name":"网页搜索","version":"1.2.0"}}
]'
```

### `mindx graph upsert-edges --edges <JSON数组>`

批量创建/更新边。

```
mindx graph upsert-edges --edges '[
  {"from_node_id":"plugin:websearch","to_node_id":"plugin:browser","type":"DEPENDS_ON"}
]'
```

### `mindx graph get-node --id <ID>`

取单个节点。

### `mindx graph neighbors --id <ID> [--depth N] [--limit N] [--types 类型列表]`

从节点出发遍历多跳邻居。depth 默认 1（直接邻居）；--types 逗号分隔过滤边类型。

```
mindx graph neighbors --id "plugin:websearch" --depth 2 --types "DEPENDS_ON,CALLS"
```

### `mindx graph query --cypher <Cypher> [--params JSON]`

只读 Cypher 查询。

```
mindx graph query --cypher "MATCH (n:Plugin) RETURN n LIMIT 10"
```

### `mindx graph exec --cypher <Cypher> [--params JSON]`

写入型 Cypher（CREATE / SET / DELETE / MERGE）。破坏性操作（DELETE）先 query 确认。

## 使用建议

1. **先 query 后 exec**：写入型操作前用只读查询确认现状。
2. **属性放 properties，关系放边**：不要把关系编码进节点属性字符串。
3. **增量维护**：upsert 幂等，扩展每次安装/升级时重写自己的节点与边即可。

## 与另两个存储技能的分工

| 需求 | 用哪个 |
|---|---|
| 表达并查询实体之间的关系 | `mindx-graph`（本技能） |
| 精确读写一个状态/配置/缓存 | `mindx-kv` |
| "我记得有个东西，按语义取回来" | `mindx-rag` |
