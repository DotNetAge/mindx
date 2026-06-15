# File Operations, File Watcher & System Ops

File system access through the daemon, file change monitoring, and miscellaneous
system operations. **Most commands require the daemon to be running.**

## File System (fs)

Access files through the daemon's working directory. Useful when the agent needs
to read/write files in a controlled context.

| Task | Command | Notes |
|------|---------|-------|
| List directory | `mindx fs ls /path/to/dir` | Also: `mindx fs list` |
| Read file | `mindx fs read /path/to/file` | Returns file content |
| Write file | `mindx fs write /path/to/file --content "..."` | Creates or overwrites |
| Create directory | `mindx fs mkdir /path/new-dir` | Single level |
| Create nested dirs | `mindx fs mkdir -p /a/b/c/deep` | `--parents` flag |
| Delete file | `mindx fs rm /path/to/file` | |
| Delete recursively | `mindx fs rm -r /path/to/dir` | `--recurse` flag |
| Force delete | `mindx fs rm -f /path/to/file` | No confirmation prompt |
| Move/rename | `mindx fs mv /src/path /dst/path` | Works for files and directories |
| Show home dir | `mindx fs home` | Daemon's configured home/working path |

### When to use `fs` vs direct bash
- Use **`mindx fs`** when operating within the daemon's managed context (sessions, projects)
- Use **direct bash** (`cat`, `ls`, etc.) for general system operations outside daemon scope

## File Watcher (fw)

Monitor file changes in real-time. Used by the daemon's session file tracking.

| Task | Command | Notes |
|------|---------|-------|
| Start watcher | `mindx fw start` | Begins monitoring configured paths |
| Stop watcher | `mindx fw stop` | Stops monitoring |
| Check status | `mindx fw status` | Running? Watching which paths? |

## Daemon Logs (log API)

Detailed log access through the daemon (complements `mindx logs` CLI command).

| Task | Command | Notes |
|------|---------|-------|
| Paginated read (newest first) | `mindx log read --limit 30` | Reverse chronological order |
| Read from offset | `mindx log read --offset 200 --limit 30` | For pagination through large logs |
| Error stream only | `mindx log read --limit 50 --stream error` | Filter to errors |
| Main/info stream | `mindx log read --limit 50 --stream main` | Normal log entries |
| Clear all logs | `mindx log clear --confirm yes` | **Destructive** — requires explicit confirm |
| Count by stream | `mindx log count` | How many entries per stream |

> Note: `mindx logs -n 50` reads log files directly from disk.
> `mindx log read --limit 50` reads through the daemon API.
> Use the latter when you need structured/paginated access.

## User Configuration

| Task | Command | Notes |
|------|---------|-------|
| Show user config | `mindx user config` | Key-value pairs of current user settings |

## Entity Tags

Manage entity type definitions used by the GraphRAG indexer.

| Task | Command | Notes |
|------|---------|-------|
| Get entity tag definitions | `mindx entity-tags get` | Lists all defined entity types with descriptions |
| Save entity tag definitions | `mindx entity-tags save --types '[{...}]'` | Define custom entities for graph extraction |

### Entity Tags Format
```json
[
  {
    "name": "Company",
    "title": "公司",
    "desc": "商业组织"
  },
  {
    "name": "Product",
    "title": "产品",
    "desc": "商品或服务"
  }
]
```

These definitions are injected into the LLMIndexer's system prompt so it knows
what entity types to extract from documents during GraphRAG indexing.
