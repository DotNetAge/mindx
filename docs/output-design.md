# QuickSearch 输出格式设计

## 限制
- 每个 GraphIndex 搜索最多显示 3 条结果

## 输出结构

```
## Search Result

[chunk.summary] - [file:chunk.source_file][POS:L{lineStart},{lineEnd}][ID:chunk.id][TAGS:tag1, tag2]

### Relevant Nodes

(entity table: schema keys 作为列头)

| ID  | Name | confidence | deadline | owner |
| --- | ---- | ---------- | -------- | ----- |
| ... | ...  | ...        | ...      | ...   |

### Relations
entityA --relation--> entityB
```

## 详细字段

### Chunk 行
```
[summary] - [file:source_path][POS:LstartLine,endLine][ID:chunk_id][TAGS:tag1, tag2]
```

| 字段      | 数据来源                                             | 说明                                     |
| --------- | ---------------------------------------------------- | ---------------------------------------- |
| `summary` | `hit.Metadata["summary"]` → `hit.Title`              | LLM 生成的 chunk 摘要，或 chunk title    |
| `file:`   | `hit.Metadata["source_file"]`                        | 原始文件路径                             |
| `POS:`    | `hit.ChunkMeta.StartPos`, `EndPos` (行号，0-indexed) | 显示为 `L{start+1},{end+1}`（1-indexed） |
| `ID:`     | `hit.ID`                                             | chunk 节点 ID                            |
| `TAGS:`   | `hit.Metadata["tags"]`                               | 标签列表，逗号分隔                       |

### Relevant Nodes
- 与该 chunk 关联的实体节点列表
- 列头: `ID | Name | <entity.Properties 的所有 schema key>`
- Properties 缺失值显示 `—`
- 每行一个实体

### Relations
- 跨所有 hits 合并去重后的边
- 格式: `entityName --predicate/type--> entityName`
