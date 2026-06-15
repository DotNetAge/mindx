# Python Development Guidelines

## Project Structure (2026 Standard)

```
project_name/
├── pyproject.toml              # Single source of truth (deps, tooling, metadata)
├── README.md
├── .python-version             # uv or pyenv pin
├── src/
│   └── project_name/
│       ├── __init__.py
│       ├── main.py             # Entry point (FastAPI app factory, CLI)
│       ├── config.py           # Settings via pydantic-settings
│       ├── dependencies.py     # Dependency injection container
│       ├── api/                # Route handlers (thin layer)
│       │   ├── __init__.py
│       │   ├── deps.py         # API-specific dependencies
│       │   └── v1/
│       │       ├── __init__.py
│       │       ├── router.py
│       │       └── endpoints/
│       │           ├── users.py
│       │           └── orders.py
│       ├── core/               # Cross-cutting concerns
│       │   ├── __init__.py
│       │   ├── security.py     # Auth, JWT, password hashing
│       │   ├── exceptions.py   # Custom exception hierarchy
│       │   └── middleware.py    # Request/response middleware
│       ├── models/             # Domain models (pydantic)
│       │   ├── __init__.py
│       │   ├── user.py
│       │   └── order.py
│       ├── services/           # Business logic (the "fat" layer)
│       │   ├── __init__.py
│       │   ├── user_service.py
│       │   └── order_service.py
│       ├── repositories/       # Data access (DB queries)
│       │   ├── __init__.py
│       │   ├── user_repo.py
│       │   └── order_repo.py
│       └── schemas/            # Request/response DTOs
│           ├── __init__.py
│           ├── user_schema.py
│           └── order_schema.py
├── tests/
│   ├── conftest.py             # Shared fixtures
│   ├── unit/
│   │   ├── test_services/
│   │   └── test_repositories/
│   └── integration/
│       └── test_api/
├── scripts/                    # Utility/migration scripts
└── alembic/                    # DB migrations (if using SQLAlchemy)
```

## Naming Conventions

| Element | Convention | Example |
|---------|-----------|---------|
| Module/file | `snake_case.py` | `user_service.py`, `order_repository.py` |
| Class | `PascalCase` | `UserService`, `OrderRepository` |
| Function/method | `snake_case` | `get_user_by_id()`, `calculate_total()` |
| Variable | `snake_case` | `user_list`, `is_active` |
| Constant | `UPPER_SNAKE_CASE` | `MAX_RETRY_COUNT`, `DEFAULT_PAGE_SIZE` |
| Private | `_leading_underscore` | `_internal_cache`, `_validate_input()` |
| "Dunder" | `__double_underscore__` | `__init__`, `__repr__` |
| Type variable | `PascalCase, single char` | `T`, `UserT` |
| Exception | `PascalError` suffix | `UserNotFoundError`, `PaymentFailedError` |

## Code Organization

### Import Order (ruff isort)

```python
# 1. Stdlib (alphabetical)
from datetime import datetime
from typing import Any

# 2. Third-party (alphabetical)
from fastapi import APIRouter, Depends
from pydantic import BaseModel
from sqlalchemy.orm import Session

# 3. Local (absolute imports, alphabetical)
from project_name.config import settings
from project_name.core.exceptions import NotFoundError
from project_name.models.user import User
```

**Rules:**
- No relative imports (`from ..models import user`) — use absolute paths
- No wildcard imports (`from module import *`) — except `__init__.py` re-exports
- Type-only imports in `if TYPE_CHECKING:` blocks to avoid circular imports

### Module Length

- **Max 300 lines** per module — if longer, split
- **Max 50 lines** per function — if longer, decompose
- **Max 7 parameters** — if more, use a dataclass/pydantic model

## Error Handling Patterns

### Custom Exception Hierarchy

```python
# core/exceptions.py
class AppError(Exception):
    """Base for all application errors."""
    def __init__(self, message: str, *, code: str = "UNKNOWN", status_code: int = 500):
        self.message = message
        self.code = code
        self.status_code = status_code
        super().__init__(self.message)


class NotFoundError(AppError):
    def __init__(self, resource: str, id: Any):
        super().__init__(
            f"{resource} with id '{id}' not found",
            code="NOT_FOUND",
            status_code=404,
        )


class ConflictError(AppError):
    def __init__(self, message: str):
        super().__init__(message, code="CONFLICT", status_code=409)


class ValidationError(AppError):
    def __init__(self, message: str):
        super().__init__(message, code="VALIDATION_ERROR", status_code=422)
```

### Error Handling in Services

```python
# ✅ GOOD: Wrap with context, let it propagate
async def get_user(self, user_id: UUID) -> User:
    try:
        return await self.repo.get_by_id(user_id)
    except NotFoundError:
        raise  # Already has context, re-raise as-is
    except DatabaseError as e:
        raise AppError(f"Database error fetching user {user_id}: {e}") from e


# ❌ BAD: Bare except, silent swallowing
async def get_user_bad(self, user_id: UUID) -> User | None:
    try:
        return await self.repo.get_by_id(user_id)
    except Exception:  # NEVER do this
        return None  # Silent failure — bug hidden forever
```

### Context Managers for Resource Cleanup

```python
# Always use context managers for I/O resources
async with aiofiles.open(path, mode="r") as f:
    content = await f.read()

# For database sessions — always scoped
async def get_db() -> AsyncGenerator[Session, None]:
    async with async_session_factory() as session:
        yield session
        # Auto rollback on exception, auto commit on success (or explicit control)
```

## Testing Standards

### Framework Stack

| Layer | Framework | Purpose |
|-------|-----------|---------|
| Unit | `pytest` + `pytest-asyncio` | Business logic, pure functions |
| Mocking | `unittest.mock` / `pytest-mock` | External dependencies |
| API testing | `httpx.AsyncClient` + FastAPI `TestClient` | Endpoint contracts |
| Fixtures | `pytest` fixtures in `conftest.py` | Shared test state |

### Test Naming & Structure

```python
# tests/unit/test_user_service.py

import pytest
from project_name.core.exceptions import NotFoundError
from project_name.services.user_service import UserService


class TestGetUserById:
    """One test class per method under test."""

    async def test_returns_user_when_exists(
        self,
        user_service: UserService,
        sample_user: User,
    ) -> None:
        result = await user_service.get_by_id(sample_user.id)
        assert result.id == sample_user.id

    async def test_raises_not_found_when_absent(
        self,
        user_service: UserService,
    ) -> None:
        with pytest.raises(NotFoundError) as exc_info:
            await user_service.get_by_id(UUID("00000000-0000-0000-0000-000000000000"))
        assert exc_info.value.status_code == 404

    async def test_raises_validation_error_for_invalid_uuid(
        self,
        user_service: UserService,
    ) -> None:
        with pytest.raises(ValidationError):
            await user_service.get_by_id("not-a-uuid")  # type: ignore[arg-type]
```

**Rules:**
- Test name = `test_{expected_behavior}_when_{condition}`
- One assertion per test when possible (Arrange-Act-Assert pattern)
- Use fixtures, not setup/teardown methods
- Target >80% coverage on services/repositories; >90% on core logic

## Security Checklist (Python-Specific)

| Check | Rule |
|-------|------|
| SQL Injection | Always use SQLAlchemy ORM parameterized queries. Never `f"SELECT * FROM users WHERE id = {user_id}"` |
| Path Traversal | Use `pathlib.Path.resolve()` and validate prefix. Never concatenate user input into file paths |
| Deserialization | Never use `pickle.loads()` on untrusted data. Use JSON or msgpack instead |
| Dependency Vulnerabilities | Run `pip-audit` or `uv pip audit` regularly in CI |
| Secret Leakage | No secrets in source code. Use `python-dotenv` + `.env` in `.gitignore` |
| YAML Safety | Use `yaml.safe_load()`, never `yaml.load()` (executes arbitrary Python) |
| Template Injection | Jinja2: set `autoescape=True`. Never render user input in templates without escaping |

## Performance Patterns

| Pattern | Anti-Pattern |
|---------|-------------|
| `asyncio.gather(*tasks)` for parallel I/O | Sequential `await` in a loop |
| `functools.lru_cache` for pure function memoization | Recalculating same value repeatedly |
| Generators (`yield`) for large datasets | Loading everything into memory (`list()`) |
| `__slots__` on classes with many instances | Regular `__dict__` for data classes with millions of instances |
| `bisect` for sorted lookups | Linear scan through sorted list |
| `str.join(list)` for string concatenation | `+` operator in loops (O(n^2)) |

## Ecosystem Toolchain

| Tool | Purpose | Config Location |
|------|---------|-----------------|
| **ruff** | Linter + formatter (replaces flake8 + black + isort) | `pyproject.toml \[tool.ruff\]` |
| **mypy** or **pyright** | Static type checking | `pyproject.toml \[tool.mypy\]` |
| **pytest** + **pytest-cov** | Testing + coverage | `pyproject.toml \[tool.pytest\]` |
| **pre-commit** | Git hooks (ruff, mypy, etc.) | `.pre-commit-config.yaml` |
| **uv** | Package manager (fast, rust-based) | `pyproject.toml` |
| **alembic** | DB migrations | `alembic.ini` |

### ruff Configuration (Minimal Recommended)

```toml
[tool.ruff]
line-length = 100
target-version = "py312"

[tool.ruff.lint]
select = [
    "E",      # pycodestyle errors
    "W",      # pycodestyle warnings
    "F",      # Pyflakes
    "I",      # isort
    "N",      # pep8-naming
    "UP",     # pyupgrade
    "B",      # flake8-bugbear
    "SIM",    # flake8-simplify
    "TCH",    # flake8-type-checking
    "RUF",    # Ruff-specific rules
]
ignore = ["E501"]  # ruff formatter handles line length
