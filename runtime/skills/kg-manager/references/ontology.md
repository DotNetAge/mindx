# Knowledge Graph Ontology Reference

Pure data reference. Loaded by SKILL.md Step 1. Contains no instructions — only whitelists.

---

## Entity Taxonomy (5 Categories, 18 Subtypes)

### Concept — Abstract top-level knowledge

| Label        | Description                 | Example                     |
| ------------ | --------------------------- | --------------------------- |
| `CoreTheory` | Core theory / paradigm      | Relativity, Agile Manifesto |
| `Term`       | Domain term / noun          | Microservice, Vector DB     |
| `Definition` | Formal definition           | CAP Theorem                 |
| `Principle`  | Principle / axiom           | DRY, KISS                   |
| `Model`      | Theoretical model / pattern | MVC, Hexagonal Architecture |

### KnowledgeUnit — Independent knowledge points (main body)

| Label       | Description           | Example                             |
| ----------- | --------------------- | ----------------------------------- |
| `Method`    | Methodology           | TDD, User Story Mapping             |
| `Process`   | Workflow / step chain | CI/CD Pipeline, Code Review         |
| `Technique` | Technique / trick     | Breakpoint Debugging, Cache Warming |
| `Formula`   | Formula / algorithm   | TF-IDF, Cosine Similarity           |
| `Framework` | Framework / toolset   | React, Spring Boot                  |

### Resource — Document / chunk layer (bridges to RAG)

| Label      | Description       | Example                     |
| ---------- | ----------------- | --------------------------- |
| `Document` | Original document | A PDF, Markdown file        |
| `Section`  | Chapter / section | Chapter 3: Deployment Guide |
| `Chunk`    | RAG chunk unit    | chunk-abc123                |

### Practice — Ground-level actionable knowledge

| Label      | Description      | Example                              |
| ---------- | ---------------- | ------------------------------------ |
| `Tool`     | Tool / software  | Docker, Postman                      |
| `Step`     | Operation step   | Step 1: Clone repo                   |
| `Problem`  | Issue / error    | OOM Error, Connection Timeout        |
| `Solution` | Resolution       | Increase heap memory, Configure pool |
| `Note`     | Warning / caveat | Never enable DEBUG in production     |

### Association — Auxiliary knowledge

| Label       | Description           | Example                      |
| ----------- | --------------------- | ---------------------------- |
| `Person`    | Person / author       | Martin Fowler                |
| `Reference` | Literature / citation | RFC 791, GoF Design Patterns |
| `Version`   | Version identifier    | v2.0.0, JDK 17               |
| `Tag`       | Category tag          | #frontend, #backend          |

---

## Relation Whitelist (14 Types, 5 Groups)

### Hierarchy

| Relation        | Reverse      | Meaning           | Example                           |
| --------------- | ------------ | ----------------- | --------------------------------- |
| `IS_A`          | IS_A         | Is-a / subtype-of | Microservice IS_A Architecture    |
| `PART_OF`       | HAS_PART     | Part-whole        | Ch3 PART_OF Book                  |
| `CONTAINS`      | CONTAINED_BY | Contains          | Doc CONTAINS Chunk                |
| `CLASSIFIED_AS` | —            | Classified as     | Method CLASSIFIED_AS BestPractice |

### Content

| Relation      | Reverse        | Meaning     | Example                 |
| ------------- | -------------- | ----------- | ----------------------- |
| `DESCRIBES`   | DESCRIBED_BY   | Describes   | Chunk DESCRIBES Concept |
| `CITES`       | CITED_BY       | Cites       | Paper CITES Reference   |
| `EXEMPLIFIES` | EXEMPLIFIED_BY | Exemplifies | Case EXEMPLIFIES Method |

### Logic

| Relation        | Reverse       | Meaning     | Example                                 |
| --------------- | ------------- | ----------- | --------------------------------------- |
| `IMPLIES`       | IMPLIED_BY    | Implies     | A IMPLIES B                             |
| `EQUIVALENT_TO` | EQUIVALENT_TO | Equivalent  | REST EQUIVALENT_TO ROA                  |
| `CONTRADICTS`   | CONTRADICTS   | Contradicts | Strong CONSISTENCY CONTRADICTS Eventual |
| `EXTENDS`       | EXTENDED_BY   | Extends     | React EXTENDS ComponentThinking         |

### Dependency

| Relation      | Reverse         | Meaning     | Example                           |
| ------------- | --------------- | ----------- | --------------------------------- |
| `PRECEDES`    | SUCCEEDS        | Precedes    | UnitTest PRECEDES IntegrationTest |
| `DEPENDS_ON`  | REQUIRED_BY     | Depends on  | Deploy DEPENDS_ON Build           |
| `COMPLEMENTS` | COMPLEMENTED_BY | Complements | Doc COMPLEMENTS API               |

### Practice

| Relation       | Reverse         | Meaning      | Example                           |
| -------------- | --------------- | ------------ | --------------------------------- |
| `APPLIES_TO`   | APPLIED_IN      | Applies to   | Docker APPLIES_TO ContainerDeploy |
| `SOLVES`       | SOLVED_BY       | Solves       | Fix SOLVES Bug                    |
| `DEMONSTRATES` | DEMONSTRATED_BY | Demonstrates | Demo DEMONSTRATES Theory          |

---

## Knowledge Level Tags

| Level     | Value       | Signal                                      |
| --------- | ----------- | ------------------------------------------- |
| Basic     | `basic`     | "What is X", "Definition of X"              |
| Core      | `core`      | "How X works", "Principle of X"             |
| Advanced  | `advanced`  | "Best practice for X", "Performance tuning" |
| Practical | `practical` | "Step: ...", "Error: ... → Solution: ..."   |

---

## Extraction Rules Summary

- Extract only within the 5x18 entity taxonomy above
- Use only the 14 relation names above
- Short chunks → extract Terms only; paragraphs → Concept + KnowledgeUnit + Practice
- Normalize aliases/abbreviations to standard name (e.g. "k8s" → "Kubernetes")
- Max 5 entities, max 5 relations per chunk
- Always create `Chunk DESCRIBES Entity` edge for each extracted entity
