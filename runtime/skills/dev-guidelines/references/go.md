# Go 开发指南

## 项目结构（标准布局）

```
project_name/
├── go.mod
├── go.sum
├── README.md
├── cmd/
│   └── server/
│       └── main.go              # Entry point, wire up dependencies
├── internal/                    # Private code — never importable from outside
│   ├── config/
│   │   └── config.go            # Config loading (env/viper/cleanenv)
│   ├── handler/                 # HTTP handlers (thin)
│   │   ├── user.go
│   │   └── order.go
│   ├── model/                   # Domain models + validation
│   │   ├── user.go
│   │   └── order.go
│   ├── repository/              # Data access
│   │   ├── user_repo.go
│   │   └── order_repo.go
│   ├── service/                 # Business logic
│   │   ├── user_service.go
│   │   └── order_service.go
│   ├── middleware/
│   │   ├── auth.go
│   │   └── logging.go
│   └── pkg/                     # Internal packages used within internal/
│       └── errors/
│           └── errors.go        # Custom error types
├── pkg/                         # Public packages importable by external code
│   └── project_name/
│       └── public_api.go
├── api/                         # API definitions (OpenAPI/proto)
│   └── openapi.yaml
├── migrations/                  # DB migrations (golang-migrate/migrate)
├── scripts/                     # Utility scripts
├── test/
│   ├── integration/
│   └── mocks/                   # Generated mocks (mockgen)
└── Dockerfile
```

**Go 社区的核心规则：**
- `internal/` = 私有代码，其他项目无法导入
- `cmd/` = 仅存放入口点，不包含业务逻辑
- `pkg/` = 公共库，供外部代码使用

## 命名约定

| 元素 | 约定 | 示例 |
|---------|-----------|---------|
| 包名 | `lowercase`，简短，无下划线 | `user`, `httputil`, `orderrepo` |
| 文件名 | `snake_case.go` | `user_service.go`, `http_handler.go` |
| 导出标识符 | `PascalCase` | `UserService`, `GetUserByID`, `MaxRetries` |
| 非导出标识符 | `camelCase` | `getUser`, `dbConn`, `logger` |
| 接口 | `PascalCase` 或 `-er` 后缀 | `Reader`, `UserRepository`, `Service` |
| 接口（单方法） | 方法名 + `er` | `Reader`, `Writer`, `Formatter` |
| 常量 | `PascalCase` 或 `camelCase` | `MaxRetries`, `defaultTimeout` |
| 错误变量 | `Err` 前缀 | `ErrUserNotFound`, `ErrInvalidInput` |
| 测试文件 | `xxx_test.go` | `user_service_test.go` |
| 测试函数 | `Test` + PascalCase | `TestGetUserByID_Success`, `TestCreateOrder_DuplicateError` |
| 基准测试 | `Benchmark` + PascalCase | `BenchmarkGetUserByID` |
| 示例 | `Example` + PascalCase | `ExampleNewUserService` |

### 包命名规则

```go
// ✅ GOOD: Short, clear package names
package user          // user domain
package orderrepo     // repository for orders
package httputil      // HTTP utilities

// ❌ BAD: Too long, redundant with imports
package userservice    // "user.UserService" is redundant
package util            // too vague
package myhelpers       // useless prefix
```

## 代码组织

### 导入分组（gofumpt/goimports）

```go
package handler

import (
	// 1. Stdlib
	"encoding/json"
	"fmt"
	"net/http"

	// 2. Third-party
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	// 3. Internal (project packages)
	"project_name/internal/config"
	"project_name/internal/model"
	"project_name/internal/pkg/errors"
)

// Blank line between groups. No blank lines within a group.
```

**规则：**
- 使用 `gofumpt`（比 gofmt 更严格）+ `goimports` 进行格式化
- 不允许未使用的导入——编译器会报错，保持代码整洁
- 仅在解决冲突时使用别名：`fqdnv1 "github.com/example/api/v1"`

### 文件组织

```go
// Order within a file:
//  1. Package declaration
//  2. Imports
//  3. Type definitions (structs, interfaces)
//  4. Constants
//  5. Variables
//  6. Constructor / factory functions (NewXxx)
//  7. Methods (grouped by receiver)
//  8. Helper functions (unexported)

package service

import (
	"context"
	"fmt"

	"project_name/internal/model"
	"project_name/internal/repository"
	"project_name/internal/pkg/errors"
)

// UserService handles business logic for users.
type UserService struct {
	repo   repository.UserRepository
	logger *zap.Logger
}

// NewUserService creates a new UserService.
func NewUserService(repo repository.UserRepository, logger *zap.Logger) *UserService {
	return &UserService{repo: repo, logger: logger}
}

// GetByID retrieves a user by their ID.
func (s *UserService) GetByID(ctx context.Context, id string) (*model.User, error) {
	if id == "" {
		return nil, errors.ErrInvalidInput.Wrap("user ID is required")
	}

	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errors.ErrUserNotFound.WithDetail(fmt.Sprintf("id=%s", id))
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	return user, nil
}
```

## 错误处理模式

### 自定义错误类型

```go
// internal/pkg/errors/errors.go
package errors

import (
	"errors"
	"fmt"
)

// AppError wraps errors with context.
type AppError struct {
	Code    string
	Message string
	Cause   error
	Status  int // HTTP status code
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Cause }

// Predefined sentinel errors.
var (
	ErrNotFound       = &AppError{Code: "NOT_FOUND", Message: "resource not found", Status: 404}
	ErrInvalidInput   = &AppError{Code: "INVALID_INPUT", Message: "invalid input", Status: 400}
	ErrUnauthorized  = &AppError{Code: "UNAUTHORIZED", Message: "unauthorized", Status: 401}
	ErrForbidden      = &AppError{Code: "FORBIDDEN", Message: "forbidden", Status: 403}
	ErrConflict       = &AppError{Code: "CONFLICT", Message: "resource conflict", Status: 409}
	ErrInternal       = &AppError{Code: "INTERNAL", Message: "internal error", Status: 500}
)

// Wrap adds detail context to an error.
func (e *AppError) Wrap(detail string) *AppError {
	return &AppError{
		Code:    e.Code,
		Message: fmt.Sprintf("%s: %s", e.Message, detail),
		Cause:   e.Cause,
		Status:  e.Status,
	}
}

// Is checks if err matches target using errors.Is.
func Is(err error, target *AppError) bool {
	var appErr *AppError
	if errors.As(err, &appErr) && appErr.Code == target.Code {
		return true
	}
	return false
}
```

### 实际错误处理

```go
// ✅ GOOD: Check specific sentinels, wrap with context on unexpected errors
func (s *UserService) GetByID(ctx context.Context, id string) (*model.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound.WithDetail(id)
		}
		return nil, fmt.Errorf("service.get_user[%s]: %w", id, err) // always wrap with context
	}
	return user, nil
}

// ✅ GOOD: Early returns, no deep nesting
func (s *UserService) Create(ctx context.Context, req *model.CreateUserRequest) (*model.User, error) {
	if req.Email == "" {
		return nil, ErrInvalidInput.Wrap("email is required")
	}
	if !isValidEmail(req.Email) {
		return nil, ErrInvalidInput.Wrap("invalid email format")
	}

	exists, err := s.repo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("check email existence: %w", err)
	}
	if exists {
		return nil, ErrConflict.Wrap(fmt.Sprintf("email already registered: %s", req.Email))
	}

	user := &model.User{
		ID:        uuid.New().String(),
		Email:     req.Email,
		Name:      req.Name,
		CreatedAt: time.Now(),
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// ❌ BAD: Panic in normal flow, bare recover
func badHandler() (string, error) {
	defer func() {
		if r := recover(); r != nil { // Don't use panic for control flow!
			log.Printf("recovered: %v", r)
		}
	}()
	// ... panic somewhere inside
	return "", nil
}
```

### Go 错误处理核心原则

1. **显式错误处理** —— Go 强制你处理每个错误，接受这一点。
2. **带上下文包装** —— 在每个层级边界使用 `fmt.Errorf` 的 `%w`。
3. **哨兵错误用于预期情况** —— `ErrNotFound`, `ErrConflict`。
4. **绝不忽略错误** —— `_ = someFunc()` 几乎总是错的。
5. **库/业务代码中不要 panic** —— 只在真正不可恢复的程序员错误时使用 panic（如不应该发生的空指针解引用）。

## 测试标准

### 表驱动测试（Go 的标准方式）

```go
// test/user_service_test.go
package service_test

import (
	"testing"

	"project_name/internal/model"
	"project_name/internal/pkg/errors"
)

func TestUserService_GetByID(t *testing.T) {
	svc := setupTestService(t) // helper to create service with mock repo

	tests := []struct {
		name       string
		userID     string
		want       *model.User
		wantErr    error
	}{
		{
			name:    "returns user when exists",
			userID:  "existing-id",
			want:    &model.User{ID: "existing-id", Email: "test@example.com"},
			wantErr: nil,
		},
		{
			name:    "returns not found for unknown id",
			userID:  "unknown-id",
			want:    nil,
			wantErr: errors.ErrUserNotFound,
		},
		{
			name:    "returns invalid input for empty id",
			userID:  "",
			want:    nil,
			wantErr: errors.ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.GetByID(context.Background(), tt.userID)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got error %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.ID != tt.want.ID {
				t.Errorf("ID = %q, want %q", got.ID, tt.want.ID)
			}
		})
	}
}
```

**规则：**
- **表驱动测试** 是 Go 的标准模式
- 通过 `t.Run()` 使用子测试提高可读性
- 使用 `testify` 或纯标准库（两者都可接受；新项目优先使用标准库）
- 使用 `mockgen` 模拟接口（基于接口的模拟）
- 服务/仓库的目标覆盖率 >80%

## 安全检查清单（Go 特定）

| 检查项 | 规则 |
|-------|------|
| SQL 注入 | 使用 `database/sql` 参数化查询（`$1`, `$2`）。绝不在 SQL 中使用 `fmt.Sprintf` |
| 路径遍历 | 使用 `filepath.Clean` + 验证路径前缀。绝不拼接用户输入 |
| 命令注入 | 未经清理绝不将用户输入传递给 `exec.Command`。使用白名单 |
| 竞态条件 | 在 CI 中运行 `-race` 标志：`go test -race ./...` |
| 内存安全 | 检查 goroutine 泄漏（没有 `WaitGroup.Done()` 调用）。使用 `errgroup` 进行结构化并发 |
| 反序列化 | 小心使用 `encoding/gob` 和 `encoding/json`（不要对不受信任的数据调用 `Unmarshal`，特别是当结构体字段有副作用时） |

## 性能模式

| 模式 | 反模式 |
|---------|-------------|
| 频繁分配的对象使用 `sync.Pool` | 重复分配大型结构体 |
| 字符串拼接使用 `strings.Builder` | 循环中使用 `+` 运算符 |
| I/O 使用 `io.Copy` / `io.CopyBuffer` | 手动逐字节复制 |
| 已知大小时使用 `make([]T, 0, cap)` 预分配切片 | 重复 `append` 导致重新分配 |
| 吞吐量使用带缓冲的 channel | 热路径中使用无缓冲 channel |
| 取消/超时使用 `context` | 没有生命周期管理的 fire-and-forget goroutine |
| `sync.Map` 仅用于特定的类缓存模式（罕见） | 到处使用 sync.Map（通常普通 map + mutex 更好） |

## 生态工具链

| 工具 | 用途 |
|------|---------|
| **gofumpt** | 格式化器（更严格的 gofmt） |
| **golangci-lint** | Linter 聚合器（运行约 50 个 linter） |
| **staticcheck** | 高级静态分析（包含在 golangci-lint 中） |
| **go vet** | 内置静态分析 |
| **go test -race** | 竞态检测器 |
| **mockgen** | 接口模拟生成 |
| **pprof** | CPU/内存分析 |
| **dlv** | 调试器 |

### golangci-lint 最小配置

```yaml
# .golangci.yml
linters:
  enable:
    - errcheck       # unchecked errors
    - gosimple       # simplify code
    - govet          # vet checks
    - ineffassign    # ineffective assignments
    - staticcheck    # advanced analysis
    - unused         # dead code
    - bodyclose      # unclosed HTTP bodies
    - noctx          # http request without context
    - errname        # error naming conventions
    - nakedret       # naked returns
    - unconvert      # unnecessary conversions
    - unparam        # unused parameters
```
