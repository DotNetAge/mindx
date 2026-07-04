# Knowledge Graph (GraphRAG)

The graph stores **entity relationships** as nodes and edges. This is where
structured knowledge lives — customers, projects, tasks, campaigns, and their
interconnections. The LLM's superpower is writing **dynamic Cypher queries** that
humans cannot easily compose.

**All commands require the daemon to be running.**

## Core Operations

### Query (Read)

| Task | Command | Notes |
|------|---------|-------|
| Execute Cypher (read) | `mindx graph query --cypher "MATCH (n) RETURN n LIMIT 10"` | SELECT-style, read-only |
| Pass parameters | `mindx graph query --cypher "MATCH (n {id:$id}) RETURN n" --params '{"id":"abc"}'` | Parameterized queries |
| Get single node | `mindx graph get-node --id <node-id>` | Quick lookup by ID |
| Find neighbors | `mindx graph neighbors --id <node-id> --depth 2` | Traverse relationships |
| Limit neighbor results | `mindx graph neighbors ... --limit 20` | Cap result count |
| Filter by edge types | `mindx graph neighbors ... --types MANAGES,HAS_GOAL` | Only specific relationship types |

### Write (Mutate)

| Task | Command | Notes |
|------|---------|-------|
| Execute Cypher (write) | `mindx graph exec --cypher "MATCH (n {id:'x'}) SET n.status='active'"` | CREATE/SET/DELETE/MERGE |
| Batch upsert nodes | `mindx graph upsert-nodes --nodes '[{...},{...}]'` | Create or update multiple nodes |
| Batch upsert edges | `mindx graph upsert-edges --edges '[{...},{...}]'` | Create or update multiple edges |

## Node & Edge Structure

### Node JSON Format
```json
{
  "id": "unique-id",
  "labels": ["EntityTypeName"],     // e.g., ["Customer", "Account"]
  "properties": {
    "name": "Display Name",
    "description": "...",           // Always present
    "confidence": 0.9,              // Always present (0-1)
    // Custom business fields:
    "status": "active",
    "health_score": 72,
    "arr": 50000,
    "tier": "enterprise"
  }
}
```

### Edge JSON Format
```json
{
  "from_node_id": "source-node-id",
  "to_node_id": "target-node-id",
  "type": "RELATIONSHIP_TYPE",      // e.g., MANAGES, HAS_GOAL, DEPENDS_ON
  "predicate": "human-readable description of this relationship",
  "properties": {
    "description": "...",
    "confidence": 0.9,
    // Custom fields:
    "since": "2026-01-15",
    "weight": 1.0
  }
}
```

## Common Patterns

### Build a Project Structure
```bash
PROJ_ID=$(mindx utils uuid)
GOAL_ID=$(mindx utils uuid)

# Create project node
mindx graph upsert-nodes --nodes "[{
  \"id\":\"$PROJ_ID\",
  \"labels\":[\"Project\"],
  \"properties\":{\"name\":\"App Launch\",\"status\":\"active\",\"progress\":0.0}
}]"

# Create goal under project
mindx graph upsert-nodes --nodes "[{
  \"id\":\"$GOAL_ID\",
  \"labels\":[\"Goal\"],
  \"properties\":{\"title\":\"100k Users\",\"weight\":1.0,\"status\":\"pending\"}
}]"
mindx graph upsert-edges --edges "[{
  \"from_node_id\":\"$PROJ_ID\",\"to_node_id\":\"$GOAL_ID\",\"type\":\"HAS_GOAL\"
}]"

echo "Project: $PROJ_ID  Goal: $GOAL_ID"
```

### Track Customer Health (customer-success skill pattern)
```bash
# Create customer account
CUSTOMER_ID=$(mindx utils uuid)
mindx graph upsert-nodes --nodes "[{
  \"id\":\"$CUSTOMER_ID\",
  \"labels\":[\"Customer\",\"Account\"],
  \"properties\":{
    \"company\":\"Acme Corp\",
    \"tier\":\"enterprise\",
    \"arr\":120000,
    \"health_score\":78,
    \"status\":\"active\"
  }
}]"

# Later — update health score
mindx graph exec --cypher "
  MATCH (c:Customer {id:'$CUSTOMER_ID'})
  SET c.health_score = 72, c.updated_at = timestamp()
  RETURN c.company, c.health_score
"

# Query at-risk customers
mindx graph query --cypher "
  MATCH (c:Customer)
  WHERE c.health_score < 60 AND c.status = 'active'
  RETURN c.company, c.tier, c.health_score, c.arr
  ORDER BY c.health_score ASC
  LIMIT 20
"
```

### Cross-reference Graph → Memory
```bash
# 1. Find a node in the graph
mindx graph query --cypher "MATCH (p:Project {name:'App Launch'}) RETURN p.id"

# 2. Use its source docs to search memory
mindx memory query "App Launch requirements decisions" --min-score 0.7
```

## Cypher Tips for LLM

Since you (the LLM) write Cypher dynamically:

1. **Always use parameterized values for user input** (`--params`) to avoid injection
2. **Prefer `query` for reads and `exec` for writes** — they have different permission levels
3. **Use `upsert-nodes/upsert-edges for bulk operations** — more efficient than individual Cypher SET/MERGE
4. **Node IDs should be unique and stable** — use `mindx utils uuid` for new entities
5. **Labels are your index** — query by label first, then filter properties
6. **`neighbors` is optimized for graph traversal** — prefer it over manual MATCH chains for depth-first exploration
