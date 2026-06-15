# Memory & Key-Value Store

Two complementary persistence layers:
- **Memory (RAG)**: Semantic vector search for unstructured knowledge — requires daemon
- **KV Store**: Simple key-value pairs for structured data — works locally or via daemon

## Memory (Long-Term RAG)

Stores semantic content as vector embeddings. Search by meaning, not keywords.
All memory commands **require the daemon** to be running.

### Search

| Task | Command | Notes |
|------|---------|-------|
| Semantic search | `mindx memory query "architecture decisions"` | Vector similarity search |
| Limit results | `mindx memory query "..." --limit 10` | Default varies |
| Minimum relevance score | `mindx memory query "..." --min-score 0.7` | Filter low-quality matches |

> Also available offline: `mindx query <terms>` — uses local embedder, no daemon needed.

### Store

| Task | Command | Notes |
|------|---------|-------|
| Store new content | `mindx memory store --content "..."` | Minimum required field |
| Set title | `mindx memory store ... --title "Meeting Notes"` | For display and search relevance |
| Set description | `mindx memory store ... --description "QBR with Acme"` | Additional context |
| Tag source | `mindx memory store ... --source "customer-success-cycle"` | Track where data came from |

### Manage

| Task | Command | Notes |
|------|---------|-------|
| Delete by ID | `mindx memory delete --id <uuid>` | Requires exact ID from stats/chunks |
| View statistics | `mindx memory stats` | Total records, storage size, index info |
| List chunks (paginated) | `mindx memory chunks --page 1 --page-size 20` | Browse stored content |
| List chunks for document | `mindx memory get-chunks --doc-id <id>` | All chunks belonging to a source doc |
| Count total records | `mindx memory count` | Quick total |
| Sync project files | `mindx memory sync --project-dir /path/to/project` | Index project files into memory |
| Check file sync status | `mindx memory file-states --project-dir /path` | Which files are indexed, which changed |

### Typical Workflow
```bash
# After an important meeting:
mindx memory store \
  --content "Decided to use PostgreSQL for the analytics DB. Migration planned for Q3." \
  --title "Architecture Decision: Analytics DB" \
  --source "meeting-2026-06-15" \
  --description "Database selection decision from engineering review"

# Later, when someone asks about database choices:
mindx memory query "database decision architecture"
```

## KV Store (Key-Value Persistence)

Simple persistent key-value storage. Used by tools like task management,
agent scoring, team tracking. Works without daemon for local operations.

### Basic Operations

| Task | Command | Notes |
|------|---------|-------|
| Get a value | `mindx kv get --key <key>` | Returns JSON value |
| Set a value | `mindx kv set --key <key> --value '<json>'` | Value must be valid JSON |
| Set with TTL | `mindx kv set --key <key> --value '<json>' --ttl 3600` | Auto-delete after N seconds |
| Delete a key | `mindx kv delete --key <key>` | |
| List keys by prefix | `mindx kv list --prefix "tasks_"` | Find all keys matching prefix |
| Limit list results | `mindx kv list --prefix "score:" --limit 10` | Pagination |
| Show values too | `mindx kv list --prefix "config:" --with-values` | Include values in output |

### Batch Operations

| Task | Command | Notes |
|------|---------|-------|
| Atomic batch write | `mindx kv batch-set --entries '[{"k":"a","v":1},{"k":"b","v":2}]'` | All succeed or none |
| Bulk delete by prefix | `mindx kv clear --prefix "cache:"` | **Destructive** — deletes all matching keys |

### Key Conventions (used by built-in tools)

The system uses specific key prefixes. Know these to query effectively:

| Prefix | Owner | Format | Example |
|--------|-------|--------|---------|
| `tasks_` | Task tools | `tasks_{sessionID}_{taskID}` | Task JSON with status/owner/metadata |
| `teams_` | Team tools | `teams_{sessionID}_{teamName}` | Team JSON with members/taskIDs |
| `score:` | Agent scoring | `score:{agent_name}:{unix_nano}` | `{agent_name, task, score, timestamp, notes}` |
| `tran:` | Translation cache | `tran:{hash}` | Cached translation result |
| `kg:` | Knowledge graph cache | `kg:{query_hash}` | Graph query result cache |

### Examples
```bash
# Check what tasks exist for current session
mindx kv list --prefix "tasks_abc123"

# Get agent scores for performance review
mindx kv list --prefix "score:csm-lead" --with-values

# Clear stale cache
mindx kv clear --prefix "cache:"
```

## Memory vs KV: When to Use Which

| Need | Use | Why |
|------|-----|-----|
| "What did we decide about X?" | `memory query` | Semantic search — you don't know the exact words |
| "Store this meeting note" | `memory store` | Unstructured content, searchable by meaning |
| "Get task #42 status" | `kv get --key tasks_..._task-42` | Exact key lookup — fast and precise |
| "Record that agent scored 8/10" | `kv set --key score:...` | Structured data, not semantic |
| "List all my tasks" | `kv list --prefix tasks_...` | Prefix scan over structured keys |
| "Find anything related to databases" | `memory query "database"` | Fuzzy semantic match across all content |
