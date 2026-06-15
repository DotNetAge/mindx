# Universal Coding Principles

Language-agnostic principles that apply to ALL code, regardless of language.

## SOLID Principles

| Principle | Definition | Code Smell When Violated |
|-----------|-----------|------------------------|
| **S** — Single Responsibility | One class/module/function = one reason to change | "God class" with 1000+ lines doing 5 things |
| **O** — Open/Closed | Open for extension, closed for modification | Adding a feature requires editing 10 existing files |
| **L** — Liskov Substitution | Subtypes must be substitutable for base types | Override breaks parent contract (throws unexpected errors) |
| **I** — Interface Segregation | Many specific interfaces > one fat interface | Client forced to implement methods it doesn't use |
| **D** — Dependency Inversion | Depend on abstractions, not concretions | `new ConcreteService()` scattered everywhere |

## DRY / KISS / YAGNI

| Principle | Rule |
|-----------|------|
| **DRY** (Don't Repeat Yourself) | Every piece of knowledge has a single, unambiguous representation. Duplication = bug multiplier. |
| **KISS** (Keep It Simple, Stupid) | The simplest solution that works is almost always the best. Complexity accumulates interest. |
| **YAGNI** (You Aren't Gonna Need It) | Don't build abstraction for hypothetical future needs. Build it when you actually need it, not before. |

## Clean Code Heuristics

### Functions
- **20-30 lines max** — if longer, decompose
- **3-4 parameters max** — if more, use an options object/struct
- **One level of indentation** per function — nested logic → extract
- **Verb-noun naming**: `getUserById()`, `calculateTotal()`, `validateInput()`
- **No side effects in pure functions** — separate query from mutation

### Naming

| Type | Convention | Examples |
|------|-----------|---------|
| Classes/Types | PascalCase | `UserService`, `OrderProcessor`, `HttpClient` |
| Functions/Methods | camelCase | `getUserById`, `calculateTotal`, `formatDate` |
| Constants | UPPER_SNAKE_CASE | `MAX_RETRY_COUNT`, `DEFAULT_TIMEOUT_MS` |
| Variables | camelCase | `userList`, `orderTotal`, `isActive` |
| Private members | _prefix or language convention | `_internalCache`, `m_connectionPool` |
| Boolean variables | is/has/can/should prefix | `isValid`, `hasPermission`, `canDelete`, `shouldRetry` |
| Collections | Plural noun | `users`, `items`, `errorMessages` |

### Comments

```plaintext
// BAD: Explains WHAT (code should be self-explanatory)
// Increment i by 1
i++;

// GOOD: Explains WHY (non-obvious business rule)
// We use >= here because the legacy API returns inclusive upper bounds,
// unlike the new spec which uses exclusive bounds. See ticket #4231.
if (pageOffset >= totalItems) { ... }
```

## Error Handling Philosophy

```
Errors are NOT exceptional — they are a normal part of program flow.
Treat them as values, not as control flow disruptions.

Golden Rules:
  1. Handle errors at the boundary (API edge, I/O operations)
  2. Wrap errors with context as they propagate up the stack
  3. Never silently swallow errors
  4. Log at the point of detection, handle at the point of decision
  5. Distinguish between retryable and non-retryable errors
```

## Security Fundamentals (All Languages)

| Rule | Detail |
|------|--------|
| **Validate input** | Never trust client data. Validate schema, type, range, encoding at every boundary. |
| **Sanitize output** | Escape for context (HTML, SQL, shell, JSON). Use parameterized APIs. |
| **Least privilege** | Run with minimum permissions. No root/admin in production services. |
| **Secrets management** | Never hardcode credentials. Use env vars, secret managers, vaults. |
| **Defense in depth** | Multiple layers of security. Auth + rate limiting + input validation + audit logging. |
| **Fail secure** | Default deny. Error paths should not expose information or grant access. |
| **Log security events** | Auth failures, permission changes, admin actions — always log, never log secrets. |

## Testing Philosophy

```
The test pyramid:

         ╱╲
        ╱ E2E╲          ← Few, slow, high-confidence (critical paths only)
       ╱──────╲
      ╱ Integration ╲   ← Medium number, medium speed (API contracts, DB integration)
     ╱──────────────╲
    ╱    Unit Tests   ╲  ← Many, fast, isolated (business logic, pure functions)
   ╱──────────────────╲

Rules:
  - Unit tests: Test ONE behavior per test. Name should read like a sentence:
    "should_return_error_when_user_not_found"
  - Integration tests: Test contracts between components, not internals
  - E2E tests: Test user journeys, not implementation details
  - Tests must be deterministic — no random data, no time-dependent assertions
  - Tests must be fast — unit suite runs in <30 seconds
  - Tests must be independent — no shared state between tests
```
