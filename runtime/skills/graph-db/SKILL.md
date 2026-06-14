---
name: graph-db
description: >
  Use when you need to query, read from, write to, or manage a graph database (gograph).
  Covers Cypher queries, node/edge CRUD, and neighbor traversal.
  This is the low-level graph DB skill — use kg-manager for knowledge graph construction workflows.
metadata:
  name_zh: 图数据库
  name_zh-tw: 圖資料庫
  description_zh: 查询、读取、写入和管理图数据库，支持 Cypher 查询、节点/边 CRUD 和邻居遍历
  description_zh-tw: 查詢、讀取、寫入和管理圖資料庫，支援 Cypher 查詢、節點/邊 CRUD 和鄰居遍歷
---

# Protocol: Graph Database Operations

## Trigger

Activate when the user asks to:
- Query or search the **knowledge graph** / **graph database**
- Create, update, or delete **nodes** or **edges** in a graph
- Find neighbors, paths, or connections between entities
- Run **Cypher** queries against the graph
- Inspect graph schema, count nodes/edges, check what's stored

**Do NOT use** for: RAG memory queries (use `memory.query`), key-value storage (use `kv-store`).

## Prerequisite

The daemon must be running: `mindx start`

## Available Operations

Each operation maps to one `mindx graph` subcommand.

### Read Operations

#### 1. Query (Cypher READ)

Execute a read-only Cypher query.

```bash
mindx graph query --cypher "MATCH (n) RETURN labels(n), count(*)"
```

Params:
- `--cypher` (required): Cypher SELECT/MATCH...RETURN query
- `--params` (optional): JSON object for parameterized query variables

Returns: JSON with `columns` and `rows`.

#### 2. Get Node

Fetch a single node by ID.

```bash
mindx graph get-node --id "ent-abc123def456"
```

Params:
- `--id` (required): Node ID

Returns: Node object with id, labels, properties.

#### 3. Get Neighbors

Find connected nodes around a given node.

```bash
mindx graph neighbors --id "ent-abc123def456" --depth 2 --limit 20
```

Params:
- `--id` (required): Center node ID
- `--depth` (optional, default=1): Hop depth (1=direct, 2=friends-of-friends)
- `--limit` (optional, default=50): Max neighbors to return
- `--types` (optional, comma-separated): Filter by edge types, e.g., "DESCRIBES,DEPENDS_ON"

Returns: List of neighbor nodes with connecting edges.

### Write Operations

#### 4. Exec (Cypher WRITE)

Execute a write Cypher query (CREATE, SET, DELETE, MERGE).

```bash
mindx graph exec --cypher "MATCH (n) WHERE n.name = 'old' SET n.name = 'new'"
```

Params:
- `--cypher` (required): Cypher write query
- `--params` (optional): JSON object for parameterized variables

Returns: `{nodes_created: N, rels_created: M, affected_nodes: P, affected_rels: Q}`

#### 5. Upsert Nodes

Create or update nodes in batch. If a node with same ID exists, its properties are merged.

```bash
mindx graph upsert-nodes --nodes '[{"id":"n1","labels":["Concept","Term"],"properties":{"name":"ML","level":"core"}}]'
```

Params:
- `--nodes` (required): JSON array of node objects. Each node:
  - `id` (string, required): Unique node identifier
  - `labels` (string array): Type labels (e.g., ["Concept", "CoreTheory"])
  - `properties` (object): Key-value properties (e.g., name, level, summary)

Returns: `{status: "ok", upserted: N}`

#### 6. Upsert Edges

Create or update edges in batch. If an edge with same (from, to, type) exists, properties are merged.

```bash
mindx graph upsert-edges --edges '[{"from_node_id":"n1","to_node_id":"n2","type":"DEPENDS_ON","properties":{}}]'
```

Params:
- `--edges` (required): JSON array of edge objects. Each edge:
  - `from_node_id` (string, required): Source node ID
  - `to_node_id` (string, required): Target node ID
  - `type` (string, required): Edge/relation type
  - `properties` (object, optional): Key-value properties on the edge

Returns: `{status: "ok", upserted: N}`

## Common Patterns

### Pattern A: Check if entity exists before creating

```bash
# Step 1: Try to find by property
mindx graph query --cypher "MATCH (n {name:'Microservice'}) RETURN n.id, labels(n)"

# Step 2: If empty result → upsert; else → use existing ID
mindx graph upsert-nodes --nodes '[...]'
```

### Pattern B: Explore neighborhood

```bash
# What is this entity connected to?
mindx graph neighbors --id "ent-xxx" --depth 1 --limit 30

# What connects two specific entities?
mindx graph query --cypher "MATCH p=(a)-[*1..3]-(b) WHERE a.id='x' AND b.id='y' RETURN relationships(p)"
```

### Pattern C: Aggregate statistics

```bash
# Count everything
mindx graph query --cypher "MATCH (n) RETURN labels(n)[0] as label, count(*) as cnt ORDER BY cnt DESC"

# Relation type distribution
mindx graph query --cypher "MATCH ()-[r]->() RETURN type(r) as t, count(*) as cnt ORDER BY cnt DESC"

# Entities at each knowledge level
mindx graph query --cypher "MATCH (n) WHERE n.level IS NOT NULL RETURN n.level as lvl, count(*) as cnt ORDER BY cnt DESC"
```

### Pattern D: Bulk delete (use with caution)

```bash
# Delete all edges of a specific type
mindx graph exec --cypher "MATCH ()-[r:DESCRIBES]->() DELETE r"

# Delete nodes matching criteria (edges must be deleted first)
mindx graph exec --cypher "MATCH (n:Concept) WHERE n.name = 'Obsolete' DETACH DELETE n"
```

## Error Handling

| Error | Meaning | Action |
|-------|---------|--------|
| `cannot connect to daemon` | Daemon not running | Run `mindx start` |
| `--cypher is required` | Missing query | Provide the Cypher string |
| `node not found` | ID does not exist | Verify ID or use query to find it |

## Data Location

Graph data file: `~/.mindx/data/knowledge-graph.db`
Engine: gograph (Pebble-based embedded graph store)
