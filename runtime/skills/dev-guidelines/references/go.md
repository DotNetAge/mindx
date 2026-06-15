# Go Development Guidelines

## Project Structure (Standard Layout)

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

**Key rule from Go community:**
- `internal/` = private, cannot be imported by other projects
- `cmd/` = entry points only, no business logic
- `pkg/` = public libraries intended for external consumption

## Naming Conventions

| Element | Convention | Example |
|---------|-----------|---------|
| Package | `lowercase`, short, no underscores | `user`, `httputil`, `orderrepo` |
| File | `snake_case.go` | `user_service.go`, `http_handler.go` |
| Exported | `PascalCase` | `UserService`, `GetUserByID`, `MaxRetries` |
| Unexported | `camelCase` | `getUser`, `dbConn`, `logger` |
| Interface | `PascalCase` or `-er` suffix | `Reader`, `UserRepository`, `Service` |
| Interface (single method) | Method name + `er` | `Reader`, `Writer`, `Formatter` |
| Constant | `PascalCase` or `camelCase` | `MaxRetries`, `defaultTimeout` |
| Error variables | `Err` prefix | `ErrUserNotFound`, `ErrInvalidInput` |
| Test file | `xxx_test.go` | `user_service_test.go` |
| Test function | `Test` + PascalCase | `TestGetUserByID_Success`, `TestCreateOrder_DuplicateError` |
| Benchmark | `Benchmark` + PascalCase | `BenchmarkGetUserByID` |
| Example | `Example` + PascalCase | `ExampleNewUserService` |

### Package Naming Rules

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

## Code Organization

### Import Grouping (gofumpt/goimports)

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

**Rules:**
- Use `gofumpt` (stricter than gofmt) + `goimports` for formatting
- No unused imports — compiler error anyway, but keep clean
- Alias only to resolve conflicts: `fqdnv1 "github.com/example/api/v1"`

### File Organization

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

## Error Handling Patterns

### Custom Error Types

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

### Error Handling in Practice

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

### Key Go Error Principles

1. **Explicit error handling** — Go forces you to handle every error. Embrace it.
2. **Wrap with context** — Use `%w` in `fmt.Errorf` at each layer boundary.
3. **Sentinel errors for expected cases** — `ErrNotFound`, `ErrConflict`.
4. **Never ignore errors** — `_ = someFunc()` is almost always wrong.
5. **Don't panic in library/business code** — Panic only for truly unrecoverable programmer errors (nil pointer dereference that shouldn't happen).

## Testing Standards

### Table-Driven Tests (The Go Way)

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

**Rules:**
- **Table-driven tests** are the standard pattern in Go
- Subtests via `t.Run()` for readability
- Use `testify` or plain stdlib (both acceptable; prefer stdlib for new projects)
- Mock interfaces with `mockgen` (interface-based mocking)
- Target >80% coverage on services/repositories

## Security Checklist (Go-Specific)

| Check | Rule |
|-------|------|
| SQL Injection | Use `database/sql` parameterized queries (`$1`, `$2`). Never `fmt.Sprintf` into SQL |
| Path Traversal | Use `filepath.Clean` + validate path prefix. Never concatenate user input |
| Command Injection | Never pass user input to `exec.Command` without sanitization. Use allowlists |
| Race Conditions | Run `-race` flag in CI: `go test -race ./...` |
| Memory Safety | Check for goroutine leaks (no `WaitGroup.Done()` calls). Use `errgroup` for structured concurrency |
| Deserialization | Be careful with `encoding/gob` and `encoding/json` (don't call `Unmarshal` on untrusted data with struct fields that have side effects) |

## Performance Patterns

| Pattern | Anti-Pattern |
|---------|-------------|
| `sync.Pool` for frequently allocated objects | Repeated allocation of large structs |
| `strings.Builder` for string concatenation | `+` operator in loops |
| `io.Copy` / `io.CopyBuffer` for I/O | Manual byte-by-byte copy |
| Pre-allocate slices with `make([]T, 0, cap)` when size known | Repeated `append` causing reallocation |
| Buffered channels for throughput | Unbuffered channels in hot paths |
| `context` for cancellation/timeout | Fire-and-forget goroutines without lifecycle management |
| `sync.Map` only for specific cache-like patterns (rare) | Using sync.Map everywhere (usually a regular map + mutex is better) |

## Ecosystem Toolchain

| Tool | Purpose |
|------|---------|
| **gofumpt** | Formatter (stricter gofmt) |
| **golangci-lint** | Linter aggregator (runs ~50 linters) |
| **staticcheck** | Advanced static analysis (included in golangci-lint) |
| **go vet** | Built-in static analysis |
| **go test -race** | Race detector |
| **mockgen** | Interface mock generation |
| **pprof** | CPU/memory profiling |
| **dlv** | Debugger |

### golangci-lint Minimal Config

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
