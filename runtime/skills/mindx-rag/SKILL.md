---
name: mindx-rag
description: >
  使用 MindX 的条目语义索引（NativeRAG）持久化与检索结构化知识，如技能注册表、
  工具目录、配置事实。当扩展开发需要"按语义取回一条已知存在的条目"（而非全文
  资料问答）时使用。需要精确键值读写用 mindx-kv，需要实体关系查询用 mindx-graph。
allowed-tools:
  - Bash(mindx rag *)
metadata:
  name_zh: MindX 条目语义索引
  name_zh-tw: MindX 條目語義索引
  description_zh: 条目语义索引：注册表、目录与配置事实的存取与语义检索
  description_zh-tw: 條目語義索引：註冊表、目錄與配置事實的存取與語義檢索
---

# MindX 条目语义索引（rag seg）

你正在通过 `mindx rag seg` CLI 管理条目语义索引。索引数据在 daemon 的
`~/.mindx/data/native.db` 中，所有命令要求 daemon 运行中（`mindx start`）。

## 心智模型

- **条目** = (族, 值, meta) 三元组。族是命名空间（如 `skill`、`tool`、`plugin`），
  值是条目主值，meta 是字符串键值对。
- **一条目多向量**：主值向量化为主体，meta 的每个键值对额外生成一条子键向量。
  检索时任一维度相似即可召回整条目——所以 meta 里应放"用户可能怎么提到它"的
  字段（title、description、别名）。
- **覆盖即更新**：同族同值重复 add 视为更新索引（幂等）；不同条目若某子键值
  完全相同，视为同一个东西，后写覆盖。
- **族之间物理隔离**，互不影响检索结果。

## 命令参考

### `mindx rag seg add <族> <值> [--meta JSON]`

添加（或覆盖）一个条目。meta 必须是"字符串值"的 JSON 对象。

```
mindx rag seg add skill "websearch" --meta '{"name":"websearch","title":"网页搜索","description":"搜索网络上内容"}'
```

输出：`{"status":"ok","family":"skill"}`

### `mindx rag seg search <键> <查询> [-k N] [--json]`

语义检索。键可以是族名（主值 + 全部子键联合检索），或 `族:子键`
（仅在该子键上定向检索）。结果按相似度降序、按条目去重。

```
mindx rag seg search skill "搜索网页" --top-k 3
mindx rag seg search skill:title "网页"
```

输出表格列：Value / Field / Score / Key。Field 为空表示主值本身命中；
Score 是 Cosine 相似度（0~1）。注意检索无相似度阈值——库里条目极少时，
零分记录也会作为"最近邻"返回，应结合 Score 自行判断是否采信（建议
Score < 0.3 视为无有效命中）。

### `mindx rag seg list <族> [--limit N] [--json]`

列出族内全部条目（仅主记录视图，不显示子键）。

```
mindx rag seg list skill --limit 50
```

## 使用建议

1. **先 list 后 add**：向一个族写入前先 list 确认是否已有同值条目，避免盲目覆盖。
2. **meta 设计决定召回质量**：title 放简短中文名，description 放一句话功能描述，
   别名可加 aliases 字段；meta 值都是参与向量化的文本，不要放 ID 等无语义内容。
3. **值保持稳定**：值是条目身份（主键由值的哈希构成），更新条目用"同值 + 新 meta"
   覆盖，而不是改值。
4. --json 输出供程序化解析。

## 与另两个存储技能的分工

| 需求 | 用哪个 |
|---|---|
| "我记得有个东西，按语义取回来" | `mindx-rag`（本技能） |
| "精确读写一个状态/配置/缓存" | `mindx-kv` |
| "表达并查询实体之间的关系" | `mindx-graph` |

典型组合：扩展开发中注册一个插件时——`mindx rag seg add plugin "websearch" --meta ...` 登记语义入口；`mindx kv set` 保存运行状态；`mindx graph upsert-nodes/edges` 登记它与其他插件的关系。三者通过约定 ID（如插件名）互相关联。
