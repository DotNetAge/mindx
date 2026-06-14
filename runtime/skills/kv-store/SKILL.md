---
name: kv-store
description: >
  Use when you need to store, retrieve, or manage key-value data persistently.
  Covers CRUD operations, prefix scanning, batch writes, TTL expiry, and namespace cleanup.
  This is the low-level KV storage skill — use kg-manager for knowledge graph workflows that need KV.
metadata:
  name_zh: 键值存储
  name_zh-tw: 鍵值儲存
  description_zh: 持久化键值数据的 CRUD 操作、前缀扫描、批量写入、TTL 过期和命名空间清理
  description_zh-tw: 持久化鍵值資料的 CRUD 操作、前綴掃描、批次寫入、TTL 過期和命名空間清理
---

# Protocol: Key-Value Store Operations

## Trigger

Activate when the user asks to:
- Store or retrieve **key-value data**, **cache** results, **save state**
- Read/write **configuration**, **preferences**, **checkpoints**, **counters**
- Manage **namespaced data** with prefix operations
- Set **time-to-live (TTL)** on stored values
- Clean up or inspect stored keys

**Do NOT use** for: Graph queries (use `graph-db`), RAG memory (use `memory.query`).

## Prerequisite

The daemon must be running: `mindx start`

## Available Operations

Each operation maps to one `mindx kv` subcommand.

### Basic CRUD

#### 1. Get — Read a single key

```bash
mindx kv get --key "app:settings:theme"
```

Params:
- `--key` (required): Full key path (max 512 chars, printable ASCII)

Returns when found:
```json
{
  "found": true,
  "item": {
    "key": "app:settings:theme",
    "value": "dark",
    "created_at": 1749600000,
    "expires_at": 0
  }
}
```

Returns when not found or expired: `{"found": false}`

#### 2. Set — Write a single key

```bash
mindx kv set --key "app:user:last_login" --value '{"ts":"2025-06-10T12:00:00Z"}'
mindx kv set --key "session:abc123:data" --value '{"page":42}' --ttl 3600
```

Params:
- `--key` (required): Full key path
- `--value` (required): Any JSON-serializable value (string, number, object, array, bool, null). Must be valid JSON.
- `--ttl` (optional): Time-to-live in seconds. After this duration, the key auto-expires. Default=0 (no expiry).

Returns: `{"status": "ok", "key": "..."}`

#### 3. Delete — Remove a single key

```bash
mindx kv delete --key "app:cache:stale_entry"
```

Params:
- `--key` (required): Key to delete

Returns: `{"status": "ok", "deleted_key": "..."}`

### Scan & Batch

#### 4. List — Prefix scan (discover keys)

List all keys under a namespace/prefix.

```bash
# List keys only (fast, no values loaded)
mindx kv list --prefix "kg:" --limit 50

# List keys WITH their values (heavier)
mindx kv list --prefix "app:settings:" --limit 20 --with-values
```

Params:
- `--prefix` (required, can be empty `""` for all keys): Key prefix to filter
- `--limit` (optional, default=100): Maximum results (0=no limit)
- `--with-values` (flag): Include value payloads in response

Returns (without values):
```json
{
  "prefix": "kg:",
  "keys": ["kg:entity:ml", "kg:entity:api", ...],
  "count": 150
}
```

Returns (with values):
```json
{
  "prefix": "kg:",
  "items": [{"key": "...", "value": {...}, ...}],
  "count": 150
}
```

#### 5. Batch Set — Atomic multi-write

Write multiple keys in a single transaction. All succeed or all fail.

```bash
mindx kv batch-set --entries '
[
  {"key":"user:1:name","value":"Alice"},
  {"key":"user:1:email","value":"alice@example.com"},
  {"key":"user:1:role","value":"admin","ttl":86400}
]
'
```

Params:
- `--entries` (required): JSON array of `{key, value, ttl?}` objects

Returns:
```json
{
  "status": "ok",
  "wrote_keys": ["user:1:name", "user:1:email", "user:1:role"],
  "count": 3
}
```

#### 6. Clear — Prefix deletion (bulk delete)

Delete ALL keys matching a prefix. Irreversible.

```bash
# Dangerous: clears entire session namespace
mindx kv clear --prefix "kg:session-old:"

# Safer: clear only temp cache entries
mindx kv clear --prefix "temp:"
```

Params:
- `--prefix` (required): Delete all keys starting with this prefix

Returns: `{"status": "ok", "prefix": "...", "deleted": 42}`

## Naming Conventions (Recommended)

Use colon (`:`) as separator to create hierarchical namespaces:

| Namespace | Example Keys | Purpose |
|-----------|-------------|---------|
| `kg:` | `kg:{sid}:entity:name`, `kg:{sid}:state` | Knowledge graph sessions |
| `session:` | `session:{id}:page`, `session:{id}:context` | Agent session state |
| `cache:` | `cache:llm:summary:hash`, `cache:rag:result:key` | Computed result caching |
| `config:` | `config:theme`, `config:lang` | User preferences |
| `counter:` | `counter:builds:total`, `counter:errors:today` | Counters / metrics |
| `temp:` | `temp:batch:12345:data` | Ephemeral scratch data |

## Common Patterns

### Pattern A: Cache expensive computation with TTL

```bash
# Check cache first
mindx kv get --key "cache:llm:extract:$(echo 'doc-chunk-hash')"

# If miss → compute → store with short TTL
mindx kv set \
  --key "cache:llm:extract:doc-chunk-hash" \
  --value '{"entities":[...],"relations":[...]}' \
  --ttl 3600
```

### Pattern B: Atomic counter increment

KV Store has no native increment. Use get-modify-set pattern:

```bash
# 1. Read current
mindx kv get --key "counter:builds:total"
# → {"found":true,"item":{"value":42,...}}

# 2. Write new value
mindx kv set --key "counter:builds:total" --value 43
```

For high-concurrency counters, consider using `batch-set` within a transaction boundary.

### Pattern C: Namespace isolation for cleanup

```bash
# All temp data lives under temp:{run_id}:
mindx kv set --key "temp:run-001:x" --value 1
mindx kv set --key "temp:run-001:y" --value 2

# Cleanup entire run in one shot
mindx kv clear --prefix "temp:run-001:"
```

### Pattern D: Structured state checkpoint

Store complex state as a single JSON value:

```bash
mindx kv set \
  --key "build:2025-06-10:checkpoint" \
  --value '{"page":23,"chunks_done":1150,"entities":340,"errors":[]}' \
  --ttl 604800  # 7 days
```

## TTL Reference

| Use Case | Recommended TTL | Example |
|----------|----------------|---------|
| Session state | Duration of session (hours) | `--ttl 7200` (2h) |
| Build checkpoint | Days (survives overnight pause) | `--ttl 604800` (7d) |
| Entity cache summary | Weeks (persists across builds) | `--ttl 864000` (10d) |
| Permanent mapping (entity→ID) | No TTL (never expires) | omit `--ttl` |
| Temp/batch scratch | Minutes | `--ttl 300` (5min) |
| LLM result cache | Hours | `--ttl 3600` (1h) |

## Error Handling

| Error | Meaning | Action |
|-------|---------|--------|
| `cannot connect to daemon` | Daemon not running | Run `mindx start` |
| `key is required` | Missing `--key` argument | Provide the key |
| `--value must be valid JSON` | Invalid JSON value | Wrap strings in quotes: `'"text"'` |
| `entries is required` | Empty batch | Provide at least one entry |

## Data Location

KV Store file: `~/.mindx/data/kvstore.db`
Engine: bbolt (embedded B+tree key-value store, ACID transactions)
Bucket: `default` (all data in one bucket, namespaced by key prefixes)
