#!/usr/bin/env python3
"""
KG_Client — Knowledge Graph Builder Client (v2)

CLI helper for KG_Manager Skill. Uses KV Store for persistent state
and document-level chunk fetching for coherent extraction.

Key changes from v1:
  - Entity cache backed by kvstore (survives restarts)
  - Document-by-document processing via memory.get_chunks
  - Batch extraction (N chunks per LLM call)
  - Session-based isolation in KV store

Usage:
    python3 kg_client.py init --session-id SID
    python3 kg_client.py list-docs --session-id SID
    python3 kg_client.py get-doc-chunks --session-id SID --doc-id DID [--batch-size N]
    python3 kg_client.py mark-done --session-id SID --doc-id DID
    python3 kg_client.py status --session-id SID
    python3 kg_client.py stats --session-id SID
    python3 kg_client.py process-batch --session-id SID --doc-id DID --batch-indices I1,I2,I3
    python3 kg_client.py cleanup --session-id SID
"""

import argparse
import json
import os
import sys
import uuid
from pathlib import Path

try:
    import websocket
except ImportError:
    print("ERROR: websocket-client required. pip install websocket-client", file=sys.stderr)
    sys.exit(1)

DEFAULT_HOST = os.environ.get("MINDX_WS_HOST", "localhost")
DEFAULT_PORT = int(os.environ.get("MINDX_WS_PORT", "1314"))
DEFAULT_PATH = os.environ.get("MINDX_WS_PATH", "/ws")
TIMEOUT = 30


# ---------------------------------------------------------------------------
# RPC Client
# ---------------------------------------------------------------------------

def rpc_call(method, params=None):
    """Call MindX Daemon JSON-RPC method, return result dict."""
    url = f"ws://{DEFAULT_HOST}:{DEFAULT_PORT}{DEFAULT_PATH}"
    try:
        ws = websocket.create_connection(url, timeout=TIMEOUT)
    except Exception as e:
        print(f'{{"error": "connection failed: {e}"}}', file=sys.stderr)
        sys.exit(1)

    request_id = str(uuid.uuid4())
    request = {"jsonrpc": "2.0", "id": request_id, "method": method}
    if params is not None:
        request["params"] = params

    try:
        ws.send(json.dumps(request))
        response_str = ws.recv()
        ws.close()
    except Exception as e:
        ws.close()
        print(f'{{"error": "rpc call failed: {e}"}}', file=sys.stderr)
        sys.exit(1)

    response = json.loads(response_str)
    if response.get("error"):
        err = response["error"]
        msg = err.get("message", str(err))
        print(f'{{"error": "rpc error [{method}]: {msg}"}}', file=sys.stderr)
        sys.exit(1)

    return response.get("result")


# ---------------------------------------------------------------------------
# KV Store Helpers (thin wrappers around kvstore.* RPC)
# ---------------------------------------------------------------------------

def kv_get(key):
    """Read a key from KV Store. Returns value or None."""
    result = rpc_call("kvstore.get", {"key": key})
    if result and result.get("found"):
        return result["item"].get("value")
    return None


def kv_set(key, value, ttl=0):
    """Write a key to KV Store."""
    rpc_call("kvstore.set", {"key": key, "value": value, "ttl": ttl})


def kv_batch_set(entries):
    """Atomically write multiple keys to KV Store."""
    rpc_call("kvstore.batch_set", {"entries": entries})


def kv_list(prefix, limit=1000, with_values=False):
    """List keys matching prefix."""
    result = rpc_call("kvstore.list", {
        "prefix": prefix,
        "limit": limit,
        "with_values": with_values,
    })
    return result or {}


def kv_clear(prefix):
    """Delete all keys matching prefix."""
    return rpc_call("kvstore.clear", {"prefix": prefix})


# ---------------------------------------------------------------------------
# Memory / Graph Helpers
# ---------------------------------------------------------------------------

def fetch_chunks_page(page=1, page_size=50):
    """Fetch one page of RAG chunks (for doc discovery)."""
    return rpc_call("memory.chunks", {
        "page": page,
        "page_size": page_size,
        "doc_id": "all",
    })


def fetch_doc_chunks(doc_id):
    """Fetch ALL chunks for a single document."""
    result = rpc_call("memory.get_chunks", {"doc_id": doc_id})
    if result:
        return result.get("chunks", []), result.get("count", 0)
    return [], 0


def graph_upsert_nodes(nodes):
    """Batch write nodes to knowledge-graph."""
    return rpc_call("graph.upsert_nodes", {"nodes": nodes})


def graph_upsert_edges(edges):
    """Batch write edges to knowledge-graph."""
    return rpc_call("graph.upsert_edges", {"edges": edges})


def graph_query(cypher, params=None):
    """Execute Cypher read query."""
    p = {"query": cypher}
    if params:
        p["params"] = params
    return rpc_call("graph.query", p)


# ---------------------------------------------------------------------------
# Session / Key Helpers
# ---------------------------------------------------------------------------

class SessionKeys:
    """Generates consistent KV store key prefixes for a session."""

    def __init__(self, session_id):
        self.sid = session_id
        self.prefix = f"kg:{session_id}:"

    def state(self):
        return f"{self.prefix}state"

    def done(self, doc_id):
        return f"{self.prefix}done:{doc_id}"

    def entity(self, name_normalized):
        return f"{self.prefix}entity:{name_normalized}"

    def cache_summary(self):
        return f"{self.prefix}entity_cache_summary"

    def all_session_keys(self):
        return self.prefix


# ---------------------------------------------------------------------------
# Entity Cache (KV Store backed)
# ---------------------------------------------------------------------------

class EntityCacheKV:
    """Entity name → node_id mapping, persisted in KV Store.

    Lookup pattern:
      - Local in-memory cache for speed during a batch
      - Falls back to kvstore.get for cross-batch/session persistence
      - Writes back to both memory + kvstore on new registrations
    """

    def __init__(self, sk: SessionKeys):
        self.sk = sk
        self._local = {}       # normalized_name -> node_id (fast path)
        self._aliases = {}     # alias -> normalized_name

    @staticmethod
    def normalize(name):
        return name.strip().lower()

    def lookup(self, name):
        """Return (node_id, normalized_key) or (None, None)."""
        key = self.normalize(name)
        # Fast: local cache
        if key in self._local:
            return self._local[key], key
        if key in self._aliases:
            norm = self._aliases[key]
            return self._local[norm], norm
        # Slow: KV store fallback
        stored = kv_get(self.sk.entity(key))
        if stored and isinstance(stored, dict):
            node_id = stored.get("node_id")
            if node_id:
                self._local[key] = node_id
                return node_id, key
        return None, None

    def register(self, node_id, name, labels=None, level="core"):
        """Register a new entity. Persists to both local cache and KV store."""
        key = self.normalize(name)
        self._local[key] = node_id
        # Persist to KV store for cross-session reuse (no TTL = permanent)
        kv_set(
            self.sk.entity(key),
            {"node_id": node_id, "labels": labels or ["Concept"], "level": level},
        )

    def size(self):
        return len(self._local)


# ---------------------------------------------------------------------------
# Commands
# ---------------------------------------------------------------------------

def cmd_init(args):
    """Initialize a build session in KV Store."""
    sk = SessionKeys(args.session_id)
    state = {
        "session_id": args.session_id,
        "status": "in_progress",
        "created_at": _iso_now(),
        "docs_done": [],
        "total_chunks": 0,
        "total_entities": 0,
        "total_relations": 0,
    }
    kv_set(sk.state(), state, ttl=864000)  # 10 days
    print(json.dumps({"status": "ok", "session_id": args.session_id}, indent=2))


def cmd_list_docs(args):
    """Discover all available documents and show which are done/pending."""
    sk = SessionKeys(args.session_id)

    # Fetch first page to discover doc_ids
    result = fetch_chunks_page(page=1, page_size=200)
    chunks = result.get("chunks", []) if result else []
    has_more = result.get("has_more", False) if result else False

    # Collect unique doc_ids
    doc_ids = set()
    for ch in chunks:
        did = ch.get("doc_id")
        if did:
            doc_ids.add(did)

    # If there might be more pages, we need to scan more.
    # For now report what we found on page 1; the Agent can paginate further.
    done_docs = []
    pending_docs = []

    for did in sorted(doc_ids):
        is_done = kv_get(sk.done(did))
        if is_done:
            done_docs.append(did)
        else:
            pending_docs.append(did)

    print(json.dumps({
        "session_id": args.session_id,
        "discovered_on_first_page": len(doc_ids),
        "has_more_pages": has_more,
        "pending": pending_docs,
        "done": done_docs,
    }, indent=2))


def cmd_get_doc_chunks(args):
    """Get all chunks for a document and format them for batch extraction."""
    sk = SessionKeys(args.session_id)
    batch_size = args.batch_size or 3

    chunks, count = fetch_doc_chunks(args.doc_id)
    if count == 0:
        print(json.dumps({
            "doc_id": args.doc_id, "count": 0, "batches": [],
            "message": "No chunks found for this document",
        }, indent=2))
        return

    # Split into batches
    batches = []
    for i in range(0, len(chunks), batch_size):
        batch = chunks[i:i + batch_size]
        batches.append({
            "batch_index": len(batches),
            "chunk_indices": [ch.get("chunk_meta", {}).get("index", j) for j, ch in enumerate(batch)],
            "chunks": [
                {
                    "id": ch.get("id"),
                    "content": ch.get("content", ""),
                    "index": ch.get("chunk_meta", {}).get("index", 0),
                }
                for ch in batch
            ],
        })

    # Load current entity cache summary from KV store
    cache_summary_raw = kv_get(sk.cache_summary())
    cache_summary = cache_summary_raw if isinstance(cache_summary_raw, list) else []

    print(json.dumps({
        "doc_id": args.doc_id,
        "count": count,
        "batch_size": batch_size,
        "total_batches": len(batches),
        "cache_summary": cache_summary[-40:],  # last 40 entries
        "batches": batches,
    }, indent=2, ensure_ascii=False))


def cmd_mark_done(args):
    """Mark a document as processed."""
    sk = SessionKeys(args.session_id)
    kv_set(sk.done(args.doc_id), True)

    # Update state's docs_done list
    state = kv_get(sk.state())
    if state and isinstance(state, dict):
        docs_done = state.get("docs_done", [])
        if args.doc_id not in docs_done:
            docs_done.append(args.doc_id)
            state["docs_done"] = docs_done
            kv_set(sk.state(), state, ttl=864000)

    print(json.dumps({"status": "ok", "doc_id": args.doc_id, "marked_done": True}, indent=2))


def cmd_process_batch(args):
    """Process LLM extraction results for one batch of chunks.

    This command handles C1-C4 from SKILL.md Step B2:
      - Deduplicate entities against KV store cache
      - Write new nodes/edges to GraphDB
      - Persist new entity mappings to KV store
      - Update cache summary
    """
    sk = SessionKeys(args.session_id)
    cache = EntityCacheKV(sk)

    # Read extraction JSON from stdin
    extraction_text = sys.stdin.read().strip()
    if not extraction_text:
        print(json.dumps({"error": "no stdin input — pipe LLM output to this command"}, indent=2))
        sys.exit(1)

    try:
        data = json.loads(extraction_text)
    except json.JSONDecodeError as e:
        print(json.dumps({"error": f"Invalid JSON: {e}"}, indent=2))
        sys.exit(1)

    # Parse batch info
    batch_indices = [int(x.strip()) for x in args.batch_indices.split(",") if x.strip()]
    chunk_ids = json.loads(args.chunk_ids) if hasattr(args, 'chunk_ids') and args.chunk_ids else []
    doc_id = args.doc_id

    entities_raw = data.get("entities", [])
    relations_raw = data.get("relations", [])
    new_nodes = []
    new_edges = []
    created_entities = 0

    # C1: Deduplicate & prepare nodes
    for e in entities_raw:
        name = e.get("name", "")
        if not name:
            continue
        existing_id, _ = cache.lookup(name)
        if existing_id:
            continue

        node_id = f"ent-{uuid.uuid4().hex[:12]}"
        labels = e.get("labels", ["Concept"])
        if not labels:
            labels = ["Concept"]

        props = {
            "name": name,
            "level": e.get("level", "core"),
            "summary": e.get("summary", ""),
            "source_chunk_ids": chunk_ids,
        }
        aliases = e.get("aliases", [])
        if aliases:
            props["aliases"] = aliases

        new_nodes.append({"id": node_id, "labels": labels, "properties": props})
        cache.register(node_id, name, labels, e.get("level", "core"))
        created_entities += 1

    # C2: Resolve relations via cache
    for r in relations_raw:
        from_name = r.get("from", "")
        to_name = r.get("to", "")
        rel_type = r.get("type", "")
        if not from_name or not to_name or not rel_type:
            continue
        from_id, _ = cache.lookup(from_name)
        to_id, _ = cache.lookup(to_name)
        if from_id and to_id:
            new_edges.append({
                "from_node_id": from_id, "to_node_id": to_id,
                "type": rel_type, "properties": r.get("properties", {}),
            })

    # C3: Auto-link chunk -> entity
    for cid in chunk_ids:
        for e in entities_raw:
            name = e.get("name", "")
            eid, _ = cache.lookup(name)
            if eid:
                new_edges.append({
                    "from_node_id": cid, "to_node_id": eid,
                    "type": "DESCRIBES", "properties": {},
                })

    # Batch write to GraphDB
    errors = []
    if new_nodes:
        try:
            graph_upsert_nodes(new_nodes)
        except SystemExit:
            raise
        except Exception as ex:
            errors.append(f"upsert_nodes: {ex}")
    if new_edges:
        try:
            graph_upsert_edges(new_edges)
        except SystemExit:
            raise
        except Exception as ex:
            errors.append(f"upsert_edges: {ex}")

    # C4: Update cache summary in KV store
    summary_raw = kv_get(sk.cache_summary())
    summary = summary_raw if isinstance(summary_raw, list) else []
    for e in entities_raw:
        name = e.get("name", "")
        _, key = cache.lookup(name)
        if key:
            summary.append({
                "name": name,
                "labels": e.get("labels", ["Concept"]),
                "level": e.get("level", "core"),
                "summary": e.get("summary", "")[:80],
            })
    # Trim to 40
    summary = summary[-40:]
    kv_set(sk.cache_summary(), summary, ttl=864000)  # 10 days TTL

    print(json.dumps({
        "entities_created": created_entities,
        "relations_created": len(new_edges),
        "errors": errors,
        "cache_size": cache.size(),
    }, indent=2))


def cmd_status(args):
    """Show current build progress from KV Store."""
    sk = SessionKeys(args.session_id)
    state = kv_get(sk.state())

    if not state:
        print(json.dumps({"status": "not_found", "session_id": args.session_id}, indent=2))
        return

    # Count done docs
    done_result = kv_list(sk.done(""), limit=10000)
    done_count = done_result.get("count", 0) if done_result else 0

    # Count cached entities
    entity_result = kv_list(sk.entity(""), limit=10000)
    entity_count = entity_result.get("count", 0) if entity_result else 0

    print(json.dumps({
        "status": state.get("status", "unknown"),
        "session_id": args.session_id,
        "created_at": state.get("created_at"),
        "docs_done": state.get("docs_done", []),
        "docs_done_count": done_count,
        "cached_entities": entity_count,
        "total_chunks": state.get("total_chunks", 0),
        "total_entities": state.get("total_entities", 0),
        "total_relations": state.get("total_relations", 0),
    }, indent=2))


def cmd_stats(args):
    """Query graph database statistics."""
    try:
        nodes = graph_query(
            "MATCH (n) RETURN labels(n)[0] as label, count(*) as cnt ORDER BY cnt DESC"
        )
        rels = graph_query(
            "MATCH ()-[r]->() RETURN type(r) as type, count(*) as cnt ORDER BY cnt DESC"
        )
        total_nodes = graph_query("MATCH (n) RETURN count(n) as cnt")
        total_rels = graph_query("MATCH ()-[r]->() RETURN count(r) as cnt")

        print(json.dumps({
            "nodes_by_label": nodes.get("rows", []) if nodes else [],
            "relations_by_type": rels.get("rows", []) if rels else [],
            "total_nodes": total_nodes.get("rows", [[0]])[0][0] if total_nodes else 0,
            "total_relations": total_rels.get("rows", [[0]])[0][0] if total_rels else 0,
        }, indent=2))
    except SystemExit:
        raise
    except Exception as e:
        print(json.dumps({"error": str(e)}, indent=2))


def cmd_cleanup(args):
    """Remove all KV store data for a session."""
    sk = SessionKeys(args.session_id)
    result = kv_clear(sk.all_session_keys())
    print(json.dumps({
        "status": "ok",
        "session_id": args.session_id,
        "cleaned": result.get("deleted", 0) if result else 0,
    }, indent=2))


# ---------------------------------------------------------------------------
# Utils
# ---------------------------------------------------------------------------

def _iso_now():
    from datetime import datetime, timezone
    return datetime.now(timezone.utc).isoformat()


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(
        description="KG_Manager Skill Client v2 — Document-by-document KG builder with KV Store",
        epilog="Examples:\n"
               "  kg_client.py init --session-id kg-001\n"
               "  kg_client.py list-docs --session-id kg-001\n"
               "  kg_client.py get-doc-chunks --session-id kg-001 --doc-id doc-abc\n"
               "  kg_client.py mark-done --session-id kg-001 --doc-id doc-abc\n"
               "  kg_client.py status --session-id kg-001\n"
               "  kg_client.py stats --session-id kg-001\n"
               "  kg_client.py cleanup --session-id kg-001\n",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    sub = parser.add_subparsers(dest="command")

    # init
    p_init = sub.add_parser("init", help="Initialize a build session")
    p_init.add_argument("--session-id", required=True)

    # list-docs
    p_list = sub.add_parser("list-docs", help="Discover documents and show progress")
    p_list.add_argument("--session-id", required=True)

    # get-doc-chunks
    p_get = sub.add_parser("get-doc-chunks", help="Fetch all chunks for a document, split into batches")
    p_get.add_argument("--session-id", required=True)
    p_get.add_argument("--doc-id", required=True)
    p_get.add_argument("--batch-size", type=int, default=3)

    # mark-done
    p_done = sub.add_parser("mark-done", help="Mark a document as fully processed")
    p_done.add_argument("--session-id", required=True)
    p_done.add_argument("--doc-id", required=True)

    # process-batch
    p_proc = sub.add_parser("process-batch", help="Process LLM extraction results (reads JSON from stdin)")
    p_proc.add_argument("--session-id", required=True)
    p_proc.add_argument("--doc-id", required=True)
    p_proc.add_argument("--batch-indices", required=True, help="Comma-separated chunk indices, e.g., 0,1,2")
    p_proc.add_argument("--chunk-ids", default="[]", help='JSON array of source chunk IDs, e.g., \'["c1","c2","c3"]\'')

    # status
    p_status = sub.add_parser("status", help="Show build session progress")
    p_status.add_argument("--session-id", required=True)

    # stats
    sub.add_parser("stats", help="Query graph statistics")

    # cleanup
    p_clean = sub.add_parser("cleanup", help="Remove all session data from KV store")
    p_clean.add_argument("--session-id", required=True)

    args = parser.parse_args()
    cmds = {
        "init": cmd_init,
        "list-docs": cmd_list_docs,
        "get-doc-chunks": cmd_get_doc_chunks,
        "mark-done": cmd_mark_done,
        "process-batch": cmd_process_batch,
        "status": cmd_status,
        "stats": cmd_stats,
        "cleanup": cmd_cleanup,
    }
    fn = cmds.get(args.command)
    if fn:
        fn(args)
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
