---
name: dev-guidelines
description: >
  Universal coding standards and language-specific best practices that define
  how production-quality code should be written. Inject this into any developer
  agent to establish a consistent quality baseline — code quality is not about
  model capability, it is about following disciplined standards.
allowed-tools: bash read glob grep
metadata:
  name_zh: 开发守则
  name_zh-tw: 開發守則
  description_zh: 通用编码标准与各语言最佳实践，定义生产级代码的编写规范。注入任何开发者 Agent 以建立一致的质量基线——代码质量取决于遵循的标准，而非模型能力本身
  description_zh-tw: 通用編碼標準與各語言最佳實踐，定義生產級程式碼的編寫規範。注入任何開發者 Agent 以建立一致的品質基線——程式碼品質取決於遵循的標準，而非模型能力本身
---

## When to Use

This skill is **always active** when writing or reviewing code. It is not triggered
by user request — it is injected as a baseline discipline.

**Do NOT use** for architecture decisions (use `architect`), deployment ops (use `sysops`),
or non-code tasks.

## Core Principle

> Code quality = Standards compliance, not model intelligence.
>
> A model without standards produces inconsistent code.
> A model WITH standards produces predictable, maintainable output.

## Language Selection

Before coding, identify the target language and load its specific guidelines:

| If user asks about... | Load reference | Key framework context |
|----------------------|---------------|----------------------|
| Python backend | `references/python.md` | FastAPI / Django / Flask |
| Go service | `references/go.md` | Standard lib / Gin / Echo |
| Rust service | `references/rust.md` | Actix-web / Axum |
| Node.js/TS backend | `references/typescript.md` | Express / Fastify / NestJS |
| Java service | `references/java.md` | Spring Boot / Quarkus |
| Any language | `references/universal.md` | Cross-cutting principles |

**Always load `universal.md` first**, then the language-specific reference.

## Quality Gates

Every piece of code must pass these gates before delivery:

### Gate 1: Style Compliance
- Follows the language's official style guide (PEP 8, effective go, rustfmt, etc.)
- Consistent naming conventions throughout the codebase
- No TODO/FIXME/HACK comments in delivered code

### Gate 2: Error Handling
- Every fallible operation has explicit error handling
- Errors are wrapped with context (not bare re-raises)
- No silent error swallowing (`except: pass`, empty catch blocks)
- Panic/recover or equivalent used only for unrecoverable states

### Gate 3: Security Basics
- User input is always validated and sanitized
- No hardcoded secrets, credentials, or tokens
- SQL queries use parameterized statements (no string concatenation)
- Authentication/authorization checks on every endpoint

### Gate 4: Testability
- Functions are small, single-responsibility, side-effect-free where possible
- Dependencies are injectable (no global singletons in business logic)
- Public APIs have corresponding test cases

### Gate 5: Performance Awareness
- No O(n^2) where O(n) suffices (obvious algorithmic choices)
- No unnecessary allocations in hot paths
- I/O operations are async/non-blocking where the runtime supports it
- Database queries are minimized (N+1 awareness)

## Anti-Patterns (Language-Agnostic)

These patterns are **never acceptable** regardless of language:

| Anti-Pattern | Why | Correct Approach |
|-------------|-----|-----------------|
| Copy-paste programming | DRY violation, double maintenance | Extract to function/module |
| God objects/classes | SRD violation, untestable | Decompose by responsibility |
| Magic numbers/strings | Unreadable, error-prone | Named constants or enums |
| Deeply nested control flow | Cognitive overload >7 levels | Early returns, guard clauses |
| Comments explaining *what* | Code should be self-explanatory | Rename variables/functions; comment *why* only |
| Catch-all exception handlers | Swallows bugs | Catch specific exceptions; let unknowns propagate |
| Global mutable state | Untestable, race-prone | Dependency injection, immutable data |
| Synchronous I/O in async context | Blocks event loop, kills throughput | Use async variants consistently |
| Hardcoded environment values | Works on dev, fails in prod | Config files, env vars, feature flags |

## Reference Files

Each language reference contains:

1. **Project Structure** — Standard directory layout
2. **Naming Conventions** — File/class/function/variable naming rules
3. **Code Organization** — Module/package boundaries, import ordering
4. **Error Handling Patterns** — Idiomatic error handling for that language
5. **Testing Standards** — Framework choice, coverage expectations, fixture patterns
6. **Security Checklist** — Language-specific security gotchas
7. **Performance Patterns** — Common optimizations and anti-patterns
8. **Ecosystem Toolchain** — Linters, formatters, type checkers to use

Load the appropriate reference before writing any code.
