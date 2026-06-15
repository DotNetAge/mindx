# Rust Development Guidelines

## Project Structure (Cargo Workspace Standard)

```
project_name/
├── Cargo.toml                 # Workspace root
├── Cargo.lock
├── README.md
├── crates/
│   ├── project-name/          # Main library crate
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs         # Re-exports, public API surface
│   │       ├── main.rs        # Binary entry point (if applicable)
│   │       ├── config.rs      # Configuration
│   │       ├── error.rs       # Error types (thiserror)
│   │       ├── model/         # Domain types
│   │       │   ├── mod.rs
│   │       │   ├── user.rs
│   │       │   └── order.rs
│   │       ├── service/       # Business logic
│   │       │   ├── mod.rs
│   │       │   ├── user_service.rs
│   │       │   └── order_service.rs
│   │       ├── repository/    # Data access
│   │       │   ├── mod.rs
│   │       │   └── user_repo.rs
│   │       └── handler/       # HTTP handlers
│   │           ├── mod.rs
│   │           └── user_handler.rs
│   └── project-name-db/       # DB-specific crate (optional separation)
│       ├── Cargo.toml
│       └── src/
│           └── lib.rs
├── migrations/                # SQL migrations (sqlx or diesel)
├── tests/                     # Integration tests
│   └── integration_test.rs
└── Dockerfile
```

## Naming Conventions (Rust API Guidelines)

| Element | Convention | Example |
|---------|-----------|---------|
| Crate | `kebab-case` | `project-name`, `user-service` |
| Module/file | `snake_case.rs` | `user_service.rs`, `mod.rs` |
| Type/Struct/Enum/Type Alias | `PascalCase` | `UserService`, `UserRepository`, `Result<T>` |
| Function/Method | `snake_case` | `get_user_by_id()`, `calculate_total()` |
| Local variable | `snake_case` | `user_id`, `is_active` |
| Constant (`const`) | `SCREAMING_SNAKE_CASE` | `MAX_RETRY_COUNT`, `DEFAULT_TIMEOUT_MS` |
| Static (`static`) | `SCREAMING_SNAKE_CASE` | `CONFIG_FILE_PATH` |
| Lifetime | Short lowercase, `'a`, `'b` | `fn foo<'a>(x: &'a str)` |
| Type Parameter | `PascalCase`, single letter `T` | `fn parse<T: FromStr>(s: &str) -> Result<T>` |
| Trait | **PascalCase**, often adjective-like | `Send`, `Sync`, `Read`, `Write`, `Iterator`, `FromStr` |
| Error Enum Variant | `PascalCase` or `KebabCase` for long names | `NotFound`, `InvalidInput`, `DatabaseConnectionFailed` |
| Feature flag | kebab-case in Cargo.toml | `postgres`, `redis-cache` |

## Code Organization

### Module Visibility Rule

```rust
// lib.rs — Public API surface (thin re-export layer)
pub mod config;
pub mod error;
pub mod handler;
pub mod model;
pub mod repository;
pub mod service;

// Re-export commonly used types at crate root for convenience
pub use error::{AppError, Result};
pub use model::{User, Order};

// handler/user_handler.rs
use crate::error::Result;           // ✅ Use crate-relative imports
use crate::model::User;
use crate::service::UserService;

// NOT: use super::super::model::User;  // ❌ Avoid deep relative paths
```

**Rules:**
- `lib.rs` is the public API gatekeeper — only re-export what's public
- Internal modules use `pub(crate)` for intra-crate visibility
- Prefer `crate::` absolute paths over `super::` / `self::` relative paths

### Struct Construction Pattern

```rust
// ✅ GOOD: Builder pattern for complex structs with many optional fields
#[derive(Debug, Clone)]
pub struct UserService {
    repo: Arc<dyn UserRepository>,
    logger: Logger,
    max_retries: u32,
    timeout: Duration,
}

impl UserService {
    pub fn builder() -> UserServiceBuilder {
        UserServiceBuilder::default()
    }
}

#[derive(Default)]
pub struct UserServiceBuilder {
    repo: Option<Arc<dyn UserRepository>>,
    logger: Option<Logger>,
    max_retries: Option<u32>,
    timeout: Option<Duration>,
}

impl UserServiceBuilder {
    pub fn repo(mut self, repo: Arc<dyn UserRepository>) -> Self {
        self.repo = Some(self);
        self
    }

    pub fn build(self) -> Result<UserService> {
        Ok(UserService {
            repo: self.repo.ok_or_else(|| AppError::config("repo is required"))?,
            logger: self.logger.unwrap_or_default(),
            max_retries: self.max_retries.unwrap_or(3),
            timeout: self.timeout.unwrap_or(Duration::from_secs(30)),
        })
    }
}
```

## Error Handling Patterns

### The `thiserror` + `anyhow` Pattern

```rust
// error.rs
use thiserror::Error;

/// Application-level error type.
/// Use this for business logic errors that callers need to handle.
#[derive(Error, Debug)]
pub enum AppError {
    #[error("not found: {resource} with id '{id}'")]
    NotFound { resource: String, id: String },

    #[error("conflict: {0}")]
    Conflict(String),

    #[error("invalid input: {0}")]
    InvalidInput(String),

    #[error("unauthorized: {0}")]
    Unauthorized(String),

    #[error("internal error: {0}")]
    Internal(#[from] anyhow::Error),

    #[error("database error: {0}")]
    Database(#[from] sqlx::Error),

    #[error("configuration error: {0}")]
    Config(String),
}

impl AppError {
    /// HTTP status code for this error.
    pub fn status_code(&self) -> u16 {
        match self {
            Self::NotFound { .. } => 404,
            Self::Conflict(_) => 409,
            Self::InvalidInput(_) => 422,
            Self::Unauthorized(_) => 401,
            Self::Internal(_) | Self::Database(_) | Self::Config(_) => 500,
        }
    }
}

/// Type alias for Result with AppError.
pub type Result<T> = std::result::Result<T, AppError>;
```

### Error Handling in Practice

```rust
// ✅ GOOD: Use ? operator, let context bubble up naturally
impl UserService {
    pub async fn get_by_id(&self, user_id: &str) -> Result<User> {
        if user_id.is_empty() {
            return Err(AppError::InvalidInput("user ID is required".into()));
        }

        // ? propagates automatically with full context from thiserror derive
        let user = self.repo.get_by_id(user_id).await?;

        Ok(user)
    }

    // For errors that need additional context before propagating:
    pub async fn create(&self, req: CreateUserRequest) -> Result<User> {
        req.validate()?; // Validate first

        let exists = self.repo.exists_by_email(&req.email).await?;
        if exists {
            return Err(AppError::Conflict(format!(
                "email already registered: {}",
                req.email
            )));
        }

        let user = User::new(req);
        self.repo.create(&user).await?;
        Ok(user)
    }
}

// ✅ GOOD: Match on specific error variants when you need different behavior
match service.get_by_id(user_id).await {
    Ok(user) => println!("{:?}", user),
    Err(AppError::NotFound { .. }) => println!("User not found"),
    Err(AppError::InvalidInput(msg)) => eprintln!("Bad input: {}", msg),
    Err(e) => eprintln!("Unexpected error: {}", e), // Catch-all for logging
}
```

### Key Rust Error Principles

1. **`thiserror`** for typed, enum-based errors that need programmatic handling
2. **`anyhow`** for untyped errors in application code (via `From<anyhow::Error>`)
3. **`?` operator everywhere** — Rust's greatest feature for error handling ergonomics
4. **Never use `unwrap()` in production code** — use `?`, `expect()` with message, or `ok()/context()`
5. **`panic!` only for unrecoverable programmer errors** — invariant violations, not user input

## Testing Standards

```rust
// tests/integration_test.rs or src/service/user_service_test.rs
mod tests {
    use super::*;
    use crate::error::AppError;
    use crate::model::User;

    #[tokio::test]
    async fn test_get_user_returns_user_when_exists() {
        let svc = setup_test_service().await;
        let user = svc.get_by_id("existing-id").await.unwrap();
        assert_eq!(user.id, "existing-id");
    }

    #[tokio::test]
    async fn test_get_user_returns_not_found_for_unknown_id() {
        let svc = setup_test_service().await;
        let err = svc.get_by_id("unknown-id").await.unwrap_err();
        assert!(matches!(err, AppError::NotFound { .. }));
    }

    #[tokio::test]
    async fn test_get_user_returns_invalid_input_for_empty_id() {
        let svc = setup_test_service().await;
        let err = svc.get_by_id("").await.unwrap_err();
        assert!(matches!(err, AppError::InvalidInput(_)));
    }
}
```

**Rules:**
- Use `#[tokio::test]` for async tests
- Use `assert_matches!` macro for enum variant matching (or `matches!`)
- Mock traits via mockall or hand-written mock impls
- Integration tests go in `tests/` directory (outside crate)
- Unit tests inline via `mod tests { ... }` within source files
- Target >80% coverage

## Security Checklist (Rust-Specific)

| Check | Rule |
|-------|------|
| Memory Safety | Rust's ownership model handles most issues. Still audit unsafe blocks. |
| Unsafe Blocks | Every `unsafe {}` must have a SAFETY comment explaining why it's safe |
| Deserialization | Never call `serde_json::from_str` on untrusted input without validation. Use `serde(de deny_unknown_fields)` |
| Command Injection | Never pass user input to `std::process::Command`. Validate against allowlist |
| Path Traversal | Validate path prefix after canonicalization. No string concatenation into paths |
| Integer Overflow | Debug mode panics on overflow (good). Release mode wraps by default — use `wrapping_add` explicitly or enable `overflow-checks` in release |
| Dependency Vulnerabilities | Run `cargo audit` in CI. Use `cargo-deny` for policy enforcement |

## Performance Patterns

| Pattern | Anti-Pattern |
|---------|-------------|
| `Arc` for shared read-only data | Cloning large structs everywhere |
| `Cow<str>` / `Cow<[u8]>` for conditional ownership | Always allocating owned strings |
| `Vec::with_capacity(n)` when size known | Repeated reallocation |
| `Box<dyn Trait>` for dynamic dispatch sparingly | Overusing trait objects in hot paths (monomorphize instead) |
| `iter()` / `iter_mut()` / `into_iter()` correctly | Collecting to Vec just to iterate once |
| `String::reserve()` for known append sizes | Repeated allocation during string building |
| `rayon` for parallel iteration | Sequential processing of embarrassingly parallel workloads |
| Zero-copy parsing where possible | Parsing into intermediate String allocations |
| `parking_lot` Mutex/RwLock over stdlib | stdlib mutex has higher overhead and no poisoning protection needed |

## Ecosystem Toolchain

| Tool | Purpose |
|------|---------|
| **clippy** | Linter (catches common mistakes, performance issues) |
| **rustfmt** | Formatter (opinionated but standard) |
| **cargo-audit** | Dependency vulnerability scanner |
| **cargo-deny** | Dependency policy enforcement |
| **cargo-nextest** | Faster test runner (parallel by default) |
| **mockall** | Auto-generated mock implementations for traits |
| **thiserror** | Typed error enum derivation |
| **anyhow** | Untyped error handling in app code |
| **tokio** | Async runtime (standard choice for 2026) |
| **sqlx** | Compile-time checked SQL (type-safe queries) |
| **axum** / **actix-web** | Web framework choices |

### clippy Must-Have Lints (allow by default, enable these)

```toml
# .clippy.toml or in lib.rs
#![warn(clippy::all)]
#![warn(clippy::pedantic)]
#![warn(clippy::nursery)]
#![allow(clippy::module_name_repetitions)]  # Often too noisy
```
