# Memory, Knowledge Base & Key-Value Store

Three complementary persistence layers:
- **Memory (RAG)**: Semantic vector search for unstructured knowledge — requires daemon
- **Knowledge Base (KB)**: Project file indexing and document search — requires daemon
- **KV Store**: Simple key-value pairs for structured data — requires daemon

> Also available offline: `mindx query <terms>` searches the memory store using the local embedder without a daemon.

## Memory (Long-Term RAG)

Stores semantic content as vector embeddings. Search by meaning, not keywords.
All memory commands **require the daemon** to be running.

### Search

| Task | Command | Notes |
|------|---------|-------|
| Semantic search | `mindx memory query "architecture decisions"` | Vector similarity search |
| Limit results | `mindx memory query "..." --limit 10` | Default varies |
| Minimum relevance score | `mindx memory query "..." --min-score 0.7` | Filter low-quality matches |
| Output as JSON | `mindx memory query "..." --json` | Machine-readable output |

> Also available offline: `mindx query <terms>` — uses local embedder, no daemon needed.  
> Add `--json` for machine-readable output.

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
| Delete by ID | `mindx memory delete --id <uuid>` | Requires exact ID from chunks |
| List chunks (paginated) | `mindx memory chunks --page 1 --page-size 20` | Browse stored content |
| Filter chunks by document | `mindx memory chunks --doc-id <id>` | Only chunks from a specific source doc |
| Output chunks as JSON | `mindx memory chunks --json` | Machine-readable output |
| Get chunks for document | `mindx memory get-chunks --doc-id <id>` | All chunks belonging to a source doc |
| Output document chunks as JSON | `mindx memory get-chunks --doc-id <id> --json` | Machine-readable output |
| Count total records | `mindx memory count` | Quick total |

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

## Knowledge Base (KB)

Project-aware document indexing and search. All `kb` commands **require the daemon**.

| Task | Command | Notes |
|------|---------|-------|
| Semantic search | `mindx kb search "project architecture"` | Search indexed project documents |
| Limit results | `mindx kb search "..." --limit 20` | Default 10 |
| Minimum score | `mindx kb search "..." --min-score 0.5` | Filter by relevance |
| Output as JSON | `mindx kb search "..." --json` | Machine-readable output |
| Index statistics | `mindx kb stats --project-dir /path` | Total records, storage, index info |
| Output stats as JSON | `mindx kb stats --project-dir /path --json` | Machine-readable output |
| Sync project files | `mindx kb sync --project-dir /path/to/project` | Re-index entire project |
| Index single path | `mindx kb index path/to/file.md` | Index one file or directory |
| Force re-index | `mindx kb index --force path/to/file.md` | Skip cache and re-index |
| Check file sync status | `mindx kb file-states --project-dir /path` | Indexed / changed / new / removed |
| Output file states as JSON | `mindx kb file-states --project-dir /path --json` | Machine-readable output |

### Typical Workflow
```bash
# Index a project
mindx kb sync --project-dir ./myproject

# Search indexed documents
mindx kb search "API design decisions"

# Check what changed
mindx kb file-states --project-dir ./myproject
```

## KV Store (Key-Value Persistence)

Simple persistent key-value storage. Used by tools like task management,
agent scoring, team tracking. All `kv` commands **require the daemon**.

### Basic Operations

| Task | Command | Notes |
|------|---------|-------|
| Get a value | `mindx kv get --key <key>` | Returns JSON value |
| Get value as JSON | `mindx kv get --key <key> --json` | Machine-readable output |
| Set a value | `mindx kv set --key <key> --value '<json>'` | Value must be valid JSON |
| Set with TTL | `mindx kv set --key <key> --value '<json>' --ttl 3600` | Auto-delete after N seconds |
| Delete a key | `mindx kv delete --key <key>` | |
| List keys by prefix | `mindx kv list --prefix "tasks_"` | Find all keys matching prefix |
| Limit list results | `mindx kv list --prefix "score:" --limit 10` | Pagination |
| Show values too | `mindx kv list --prefix "config:" --with-values` | Include values in output |
| Output key list as JSON | `mindx kv list --prefix "config:" --json` | Machine-readable output |

### Batch Operations

| Task | Command | Notes |
|------|---------|-------|
| Atomic batch write | `mindx kv batch-set --entries '[{"key":"a","value":1},{"key":"b","value":2}]'` | All succeed or none; supports optional `ttl` per entry |
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

## Memory vs KB vs KV: When to Use Which

| Need | Use | Why |
|------|-----|-----|
| "What did we decide about X?" (chat memory) | `memory query` | Semantic search over stored memory records |
| "Store this meeting note" | `memory store` | Unstructured content, searchable by meaning |
| "Search project documents" | `kb search` | Semantic search over indexed project files |
| "Index project files" | `kb sync` | Project-aware document indexing |
| "Get task #42 status" | `kv get --key tasks_..._task-42` | Exact key lookup — fast and precise |
| "Record that agent scored 8/10" | `kv set --key score:...` | Structured data, not semantic |
| "List all my tasks" | `kv list --prefix tasks_...` | Prefix scan over structured keys |
| "Find anything related to databases" (offline) | `mindx query "database"` | Fuzzy semantic match without daemon |
