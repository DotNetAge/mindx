# Python 开发指南

## 项目结构（2026 标准）

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

## 命名约定

| 元素 | 约定 | 示例 |
|---------|-----------|---------|
| 模块/文件 | `snake_case.py` | `user_service.py`, `order_repository.py` |
| 类 | `PascalCase` | `UserService`, `OrderRepository` |
| 函数/方法 | `snake_case` | `get_user_by_id()`, `calculate_total()` |
| 变量 | `snake_case` | `user_list`, `is_active` |
| 常量 | `UPPER_SNAKE_CASE` | `MAX_RETRY_COUNT`, `DEFAULT_PAGE_SIZE` |
| 私有成员 | `_leading_underscore` | `_internal_cache`, `_validate_input()` |
| "Dunder" 方法 | `__double_underscore__` | `__init__`, `__repr__` |
| 类型变量 | `PascalCase, 单字符` | `T`, `UserT` |
| 异常 | `PascalError` 后缀 | `UserNotFoundError`, `PaymentFailedError` |

## 代码组织

### 导入顺序（ruff isort）

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

**规则：**
- 不使用相对导入（`from ..models import user`）—— 使用绝对路径
- 不使用通配符导入（`from module import *`）—— 除非是 `__init__.py` 的重新导出
- 仅在类型导入时使用 `if TYPE_CHECKING:` 块，避免循环导入

### 模块长度

- **每个模块最多 300 行** —— 超过则拆分
- **每个函数最多 50 行** —— 超过则分解
- **最多 7 个参数** —— 超过则使用 dataclass/pydantic model

## 错误处理模式

### 自定义异常层次结构

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

### Service 中的错误处理

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

### 使用上下文管理器清理资源

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

## 测试标准

### 框架栈

| 层级 | 框架 | 用途 |
|-------|-----------|---------|
| 单元测试 | `pytest` + `pytest-asyncio` | 业务逻辑，纯函数 |
| Mock | `unittest.mock` / `pytest-mock` | 外部依赖 |
| API 测试 | `httpx.AsyncClient` + FastAPI `TestClient` | 端点契约 |
| Fixtures | `conftest.py` 中的 `pytest` fixtures | 共享测试状态 |

### 测试命名与结构

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

**规则：**
- 测试名称 = `test_{expected_behavior}_when_{condition}`
- 每个测试尽可能只有一个断言（Arrange-Act-Assert 模式）
- 使用 fixtures，而非 setup/teardown 方法
- 服务/仓库的目标覆盖率 >80%；核心逻辑 >90%

## 安全检查清单（Python 特定）

| 检查项 | 规则 |
|-------|------|
| SQL 注入 | 始终使用 SQLAlchemy ORM 参数化查询。绝不使用 `f"SELECT * FROM users WHERE id = {user_id}"` |
| 路径遍历 | 使用 `pathlib.Path.resolve()` 并验证前缀。绝不将用户输入拼接到文件路径 |
| 反序列化 | 绝不使用 `pickle.loads()` 处理不受信任的数据。改用 JSON 或 msgpack |
| 依赖漏洞 | 在 CI 中定期运行 `pip-audit` 或 `uv pip audit` |
| 密钥泄漏 | 源代码中不包含密钥。使用 `python-dotenv` + `.env` 并添加到 `.gitignore` |
| YAML 安全 | 使用 `yaml.safe_load()`，绝不使用 `yaml.load()`（会执行任意 Python 代码） |
| 模板注入 | Jinja2：设置 `autoescape=True`。绝不在未转义的情况下在模板中渲染用户输入 |

## 性能模式

| 模式 | 反模式 |
|---------|-------------|
| 并行 I/O 使用 `asyncio.gather(*tasks)` | 循环中顺序 `await` |
| 纯函数记忆化使用 `functools.lru_cache` | 重复计算相同值 |
| 大数据集使用生成器（`yield`） | 将所有内容加载到内存（`list()`） |
| 多实例类使用 `__slots__` | 数百万实例的数据类使用常规 `__dict__` |
| 排序查找使用 `bisect` | 对排序列表进行线性扫描 |
| 字符串拼接使用 `str.join(list)` | 循环中使用 `+` 运算符（O(n^2)） |

## 生态工具链

| 工具 | 用途 | 配置位置 |
|------|---------|-----------------|
| **ruff** | Linter + 格式化器（替代 flake8 + black + isort） | `pyproject.toml \[tool.ruff\]` |
| **mypy** 或 **pyright** | 静态类型检查 | `pyproject.toml \[tool.mypy\]` |
| **pytest** + **pytest-cov** | 测试 + 覆盖率 | `pyproject.toml \[tool.pytest\]` |
| **pre-commit** | Git hooks（ruff, mypy 等） | `.pre-commit-config.yaml` |
| **uv** | 包管理器（快速，基于 Rust） | `pyproject.toml` |
| **alembic** | 数据库迁移 | `alembic.ini` |

### ruff 配置（最小推荐配置）

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
```
