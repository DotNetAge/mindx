---
name: kg-manager
description: >
  This skill should be used when the user asks to "build knowledge graph",
  "construct knowledge graph from documents", "extract entities and relations",
  "create KG from RAG chunks", "knowledge graph extraction",
  or wants to turn documents into a structured knowledge graph.
---

# Protocol: Document-by-Document Knowledge Graph Construction

## Trigger

Activate when user requests building a knowledge graph from documents,
extracting entities and relations from RAG chunks.
Do NOT use for: plain semantic search (use `memory.query`), project management (use `project-manager`).

## Step 1: Load Ontology

Read `references/ontology.md` for the full entity type whitelist and relation type whitelist.
All extraction must stay within this framework.

**Hard constraints — enforce on every extraction:**

1. Entities: only 5 categories x 18 subtypes defined in ontology.md. No custom types.
2. Relations: only 14 whitelisted relation names. No custom relations.
3. Every entity must carry a `level` field: `basic` / `core` / `advanced` / `practical`
4. Same-named entities normalize to one graph node (case-insensitive match)
5. Skip pronouns, filler words, generic terms with no informational value
6. Max 8 entities and 8 relations per batch (batch may contain multiple chunks)
7. Return empty `{"entities":[],"relations":[]}` when content has no extractable value

## Step 2: Initialize Build Session

```bash
python3 scripts/kg_client.py init --session-id {SESSION_ID}
```

Generate a unique `SESSION_ID` for this build run (e.g., `kg-20250610-001`).
This creates a KV store namespace under prefix `kg:{SESSION_ID}:`.

To resume an interrupted build:

```bash
python3 scripts/kg_client.py status --session-id {SESSION_ID}
```

If status shows `"in_progress"`, proceed with resume flow in Step 3.

## Step 3: Document-by-Document Loop (Core)

The build processes **one document at a time**, fetching all its chunks in one call.
This preserves document-level context coherence — chunks from the same document are always processed together.

### Phase A: Get Document List

```bash
python3 scripts/kg_client.py list-docs --session-id {SESSION_ID}
```

This calls `memory.chunks?page=1&page_size=1` to discover available documents,
then returns the unique `doc_id` list. Record it as your working queue.

### Phase B: Process One Document

For each document in the queue (skip if already marked done):

```bash
python3 scripts/kg_client.py get-doc-chunks --session-id {SESSION_ID} --doc-id {DOC_ID}
```

This calls `memory.get_chunks` with the doc_id and returns ALL chunks for that document.
Chunks are already sorted by their original order (`chunk_meta.index`).

Then split the document's chunks into **batches** of N chunks each (default N=3).
Process each batch through Steps B1–B4 below.

#### B1: Batch Extraction via LLM

Send the System Prompt and User Prompt (below) to the LLM.
The User Prompt contains **all N chunk texts from the current batch** at once.

##### System Prompt (identical every call)

```
You are a knowledge graph construction assistant. Extract structured entities and relations
from one or more text fragments following strict rules.

## Allowed Entity Types (5 categories, 18 subtypes)
Concept: CoreTheory, Term, Definition, Principle, Model
KnowledgeUnit: Method, Process, Technique, Formula, Framework
Resource: Document, Section, Chunk
Practice: Tool, Step, Problem, Solution, Note
Association: Person, Reference, Version, Tag

## Allowed Relation Types (14 total)
Hierarchy: IS_A, PART_OF, CONTAINS, CLASSIFIED_AS
Content: DESCRIBES, CITES, EXEMPLIFIES
Logic: IMPLIES, EQUIVALENT_TO, CONTRADICTS, EXTENDS
Dependency: PRECEDES, DEPENDS_ON, COMPLEMENTS
Practice: APPLIES_TO, SOLVES, DEMONSTRATES

## Knowledge Levels
basic = definitions, terminology, introductory concepts
core = principles, main workflows, core mechanisms
advanced = techniques, optimizations, edge cases
practical = hands-on, examples, troubleshooting

## Hard Constraints
- Use ONLY entity types and relation types listed above. No custom types.
- Normalize aliases/abbreviations to one standard name per concept.
- No pronouns, fillers, or meaningless generic words.
- Maximum 8 entities and 8 relations per batch.
- Entities found across multiple fragments share ONE node ID (deduplicate by name).
- Return empty JSON if all fragments have no extractable value.

## Output Format (JSON only, no other text)
{
  "entities": [
    {"name":"StandardName","labels":["Category","Subtype"],"level":"core",
     "aliases":["Alias"],"summary":"One sentence"}
  ],
  "relations": [
    {"from":"EntityA","to":"EntityB","type":"RELATION_TYPE","properties":{}}
  ]
}
```

##### User Prompt (different each batch)

```
## Document: {DOC_TITLE_OR_ID}
## Batch [{BATCH_INDEX}/{TOTAL_BATCHES}] — Chunks {CHUNK_RANGE}

---

### Fragment 1 [Chunk index={IDX1}]
{CONTENT_1}

### Fragment 2 [Chunk index={IDX2}]
{CONTENT_2}

### Fragment 3 [Chunk index={IDX3}]
{CONTENT_3}

---

## Global Entity Cache (previously extracted across all documents)

{CACHE_SUMMARY}

Extract entities and relations from ALL fragments above. Output JSON only.
Cross-fragment relationships within this batch are especially valuable.
```

Variable substitution rules:
- `{DOC_TITLE_OR_ID}` = current document's `doc_id`
- `{BATCH_INDEX}` = 1-based batch number within this document
- `{TOTAL_BATCHES}` = total batches for this document
- `{CHUNK_RANGE}` = e.g., "0–2" meaning chunk indices covered by this batch
- `{CONTENT_N}` = raw value of each chunk's `content` field
- `{CACHE_SUMMARY}` = loaded from KV store via `kvstore.get` with key `kg:{SESSION_ID}:entity_cache_summary`.
  Format: bullet list of up to 40 most recent entities:

```
- [core] Microservice (Concept/CoreTheory) Distributed architecture style
- [core] API Gateway (KnowledgeUnit/Framework) Unified entry gateway
- [practical] Docker (Practice/Tool) Container deployment tool
...
```

On very first batch of a new session, set `{CACHE_SUMMARY}` to `(no prior context)`.

#### B2: Deduplicate & Write to GraphDB

Parse the LLM JSON output. Execute C1–C4:

**C1. Deduplicate Entities**

For each item in `entities[]`:
1. Check KV store: `kvstore.get` with key `kg:{SESSION_ID}:entity:{normalized_name}`
2. If found → reuse existing `node_id` from stored value, skip creation
3. If not found → generate `node_id = "ent-{12-char-hex}"`, write via RPC:

```bash
graph.upsert_nodes '{"nodes":[{"id":"ent-abc123def456","labels":["Concept","CoreTheory"],"properties":{"name":"Microservice","level":"core","aliases":["微服务"],"summary":"Distributed architecture style","source_chunk_ids":["chunk-xxx"]}}]}'
```

Then persist the mapping to KV store:

```bash
kvstore.set '{"key":"kg:{SESSION_ID}:entity:microservice","value":{"node_id":"ent-abc123def456","labels":["Concept","CoreTheory"],"level":"core"}}'
```

**C2. Resolve & Create Relations**

For each item in `relations[]`:
1. Look up both endpoint names via `kvstore.get` (same key pattern as C1)
2. Create edge only if both endpoints exist; discard otherwise

```bash
graph.upsert_edges '{"edges":[{"from_node_id":"ent-aaa","to_node_id":"ent-bbb","type":"DEPENDS_ON","properties":{}}]}'
```

**C3. Auto-link Chunk → Entity**

For every entity extracted from any chunk in this batch, create a `DESCRIBES` edge
from each source chunk to that entity:

```bash
graph.upsert_edges '{"edges":[{"from_node_id":"chunk-xxx","to_node_id":"ent-abc123","type":"DESCRIBES","properties":{}}]}'
```

**C4. Update Entity Cache Summary in KV Store**

Append newly created/confirmed entities to the cache summary.
Trim to last 40 entries. Persist back to KV store:

```bash
kvstore.set '{"key":"kg:{SESSION_ID}:entity_cache_summary","value":[...updated summary array...],"ttl":864000}'
```

(TTL = 10 days so stale caches auto-expire.)

#### B3: Mark Document Complete

After processing all batches of the current document:

```bash
python3 scripts/kg_client.py mark-done --session-id {SESSION_ID} --doc-id {DOC_ID}
```

This writes `kvstore.set` with key `kg:{SESSION_ID}:done:{DOC_ID}` → `true`.

#### B4: Advance to Next Document

Return to Phase A, pick next unprocessed document from the queue.
When all documents are done → exit loop, proceed to Step 4.

## Step 4: Verify and Report

```bash
python3 scripts/kg_client.py stats --session-id {SESSION_ID}
```

Report to user:

```
Knowledge Graph Build Complete
================================
Session ID:          {SESSION_ID}
Documents processed: {D}/{D_TOTAL}
Chunks processed:    {N}
Entities created:    {M} (unique after dedup: {U})
Relations created:   {R}
Graph DB path:       ~/.mindx/data/knowledge-graph.db
KV Store prefix:     kg:{SESSION_ID}:

Entity distribution: (by label counts from graph.query)
Relation distribution: (by type counts from graph.query)
```

Optional cleanup after successful build:

```bash
python3 scripts/kg_client.py cleanup --session-id {SESSION_ID}
```

This runs `kvstore.clear` with prefix `kg:{SESSION_ID}:` to free session data.

## Command Reference

| Action | Command |
|--------|---------|
| Init session | `python3 scripts/kg_client.py init --session-id SID` |
| List docs | `python3 scripts/kg_client.py list-docs --session-id SID` |
| Get doc chunks | `python3 scripts/kg_client.py get-doc-chunks --session-id SID --doc-id DID` |
| Mark doc done | `python3 scripts/kg_client.py mark-done --session-id SID --doc-id DID` |
| Check progress | `python3 scripts/kg_client.py status --session-id SID` |
| Graph statistics | `python3 scripts/kg_client.py stats --session-id SID` |
| Cleanup session | `python3 scripts/kg_client.py cleanup --session-id SID` |

## RPC Endpoint Reference

| Method | Key Params | Purpose |
|--------|-----------|---------|
| `memory.chunks` | page, page_size | Paginated RAG chunk listing (for doc discovery) |
| `memory.get_chunks` | doc_id | Fetch ALL chunks for one document |
| `graph.upsert_nodes` | nodes[] | Batch write nodes |
| `graph.upsert_edges` | edges[] | Batch write edges |
| `graph.query` | query, params? | Cypher read queries |
| `graph.exec` | query, params? | Cypher write queries |
| `graph.get_node` | id | Get node by ID |
| `graph.get_neighbors` | id, depth, limit, types? | Neighbor traversal |
| `kvstore.get` | key | Read single value (with TTL expiry check) |
| `kvstore.set` | key, value, ttl? | Write single value (with optional TTL) |
| `kvstore.delete` | key | Delete single key |
| `kvstore.list` | prefix, limit, with_values? | Prefix scan |
| `kvstore.batch_set` | entries[] | Atomic batch write |
| `kvstore.clear` | prefix | Prefix deletion |

## Incremental Builds

After new documents are indexed into RAG, start a fresh session with a new SESSION_ID.
Entity name-to-node_id mappings persist in KV store (under `kg:*:entity:` keys) across sessions,
so global deduplication works automatically — existing entities are reused, only new ones are created.
New documents' chunks are appended to existing graph with correct DESCRIBES edges.
