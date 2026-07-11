# Rust 开发指南

## 项目结构（Cargo Workspace 标准）

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

## 命名约定（Rust API 指南）

| 元素 | 约定 | 示例 |
|---------|-----------|---------|
| Crate | `kebab-case` | `project-name`, `user-service` |
| 模块/文件 | `snake_case.rs` | `user_service.rs`, `mod.rs` |
| 类型/结构体/枚举/类型别名 | `PascalCase` | `UserService`, `UserRepository`, `Result<T>` |
| 函数/方法 | `snake_case` | `get_user_by_id()`, `calculate_total()` |
| 局部变量 | `snake_case` | `user_id`, `is_active` |
| 常量（`const`） | `SCREAMING_SNAKE_CASE` | `MAX_RETRY_COUNT`, `DEFAULT_TIMEOUT_MS` |
| 静态变量（`static`） | `SCREAMING_SNAKE_CASE` | `CONFIG_FILE_PATH` |
| 生命周期 | 短小写，`'a`, `'b` | `fn foo<'a>(x: &'a str)` |
| 类型参数 | `PascalCase`，单字母 `T` | `fn parse<T: FromStr>(s: &str) -> Result<T>` |
| Trait | **PascalCase**，常为形容词形式 | `Send`, `Sync`, `Read`, `Write`, `Iterator`, `FromStr` |
| 错误枚举变体 | `PascalCase` 或长名称使用 `KebabCase` | `NotFound`, `InvalidInput`, `DatabaseConnectionFailed` |
| Feature flag | Cargo.toml 中使用 kebab-case | `postgres`, `redis-cache` |

## 代码组织

### 模块可见性规则

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

**规则：**
- `lib.rs` 是公共 API 的守门人 —— 仅重新导出需要公开的内容
- 内部模块使用 `pub(crate)` 进行 crate 内可见性控制
- 优先使用 `crate::` 绝对路径而非 `super::` / `self::` 相对路径

### 结构体构造模式

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

## 错误处理模式

### `thiserror` + `anyhow` 模式

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

### 实际错误处理

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

### Rust 错误处理核心原则

1. **`thiserror`** 用于需要程序化处理的类型化、基于枚举的错误
2. **`anyhow`** 用于应用代码中的非类型化错误（通过 `From<anyhow::Error>`）
3. **到处使用 `?` 运算符** —— Rust 错误处理人体工程学最伟大的特性
4. **生产代码中绝不使用 `unwrap()`** —— 使用 `?`、带消息的 `expect()`，或 `ok()/context()`
5. **`panic!` 仅用于不可恢复的程序员错误** —— 不变量违反，而非用户输入

## 测试标准

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

**规则：**
- 异步测试使用 `#[tokio::test]`
- 枚举变体匹配使用 `assert_matches!` 宏（或 `matches!`）
- 通过 mockall 或手写 mock 实现来模拟 trait
- 集成测试放在 `tests/` 目录（crate 外部）
- 单元测试通过源文件中的 `mod tests { ... }` 内联编写
- 目标覆盖率 >80%

## 安全检查清单（Rust 特定）

| 检查项 | 规则 |
|-------|------|
| 内存安全 | Rust 的所有权模型处理了大部分问题。仍需审计 unsafe 块。 |
| Unsafe 块 | 每个 `unsafe {}` 必须有 SAFETY 注释说明为何安全 |
| 反序列化 | 验证前绝不使用 `serde_json::from_str` 处理不受信任的输入。使用 `serde(de deny_unknown_fields)` |
| 命令注入 | 绝不将用户输入传递给 `std::process::Command`。根据白名单验证 |
| 路径遍历 | 规范化后验证路径前缀。不将字符串拼接到路径中 |
| 整数溢出 | Debug 模式下溢出会 panic（好）。Release 模式默认回绕 —— 显式使用 `wrapping_add` 或在 release 中启用 `overflow-checks` |
| 依赖漏洞 | 在 CI 中运行 `cargo audit`。使用 `cargo-deny` 进行策略执行 |

## 性能模式

| 模式 | 反模式 |
|---------|-------------|
| 共享只读数据使用 `Arc` | 到处克隆大型结构体 |
| 条件所有权使用 `Cow<str>` / `Cow<[u8]>` | 始终分配拥有所有权的字符串 |
| 已知大小时使用 `Vec::with_capacity(n)` | 重复重新分配 |
| 动态分发谨慎使用 `Box<dyn Trait>` | 热路径中过度使用 trait 对象（改为单态化） |
| 正确使用 `iter()` / `iter_mut()` / `into_iter()` | 仅为了迭代一次就收集到 Vec |
| 已知追加大小时使用 `String::reserve()` | 字符串构建过程中重复分配 |
| 并行迭代使用 `rayon` | 顺序处理明显可并行的工作负载 |
| 尽可能使用零拷贝解析 | 解析到中间 String 分配 |
| 优先使用 `parking_lot` Mutex/RwLock 而非标准库 | 标准库 mutex 开销更高且不需要 poisoning 保护 |

## 生态工具链

| 工具 | 用途 |
|------|---------|
| **clippy** | Linter（捕获常见错误、性能问题） |
| **rustfmt** | 格式化器（有主见但标准化） |
| **cargo-audit** | 依赖漏洞扫描器 |
| **cargo-deny** | 依赖策略执行 |
| **cargo-nextest** | 更快的测试运行器（默认并行） |
| **mockall** | 为 trait 自动生成 mock 实现 |
| **thiserror** | 类型化错误枚举派生 |
| **anyhow** | 应用代码中的非类型化错误处理 |
| **tokio** | 异步运行时（2026 年的标准选择） |
| **sqlx** | 编译时检查的 SQL（类型安全查询） |
| **axum** / **actix-web** | Web 框架选择 |

### clippy 必备 Lints（默认允许，启用这些）

```toml
# .clippy.toml or in lib.rs
#![warn(clippy::all)]
#![warn(clippy::pedantic)]
#![warn(clippy::nursery)]
#![allow(clippy::module_name_repetitions)]  # Often too noisy
```
