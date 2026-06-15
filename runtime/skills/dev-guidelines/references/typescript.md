# TypeScript / Node.js Backend Development Guidelines

## Project Structure (2026 Standard)

```
project_name/
├── package.json
├── tsconfig.json
├── tsconfig.build.json          # Build config (strict)
├── eslint.config.mjs
├── .prettierrc
├── .env.example
├── .gitignore
├── src/
│   ├── index.ts                 # Entry point
│   ├── app.ts                   # App factory (Express/Fastify/NestJS)
│   ├── config/                  # Configuration
│   │   └── index.ts             # zod/env-safe validated config
│   ├── modules/                 # Feature modules (bounded contexts)
│   │   ├── user/
│   │   │   ├── index.ts         # Module barrel export
│   │   │   ├── user.model.ts    # Type/interface definitions
│   │   │   ├── user.service.ts  # Business logic
│   │   │   ├── user.repository.ts # Data access
│   │   │   ├── user.dto.ts      # Request/response types
│   │   │   ├── user.routes.ts   # Route definitions
│   │   │   └── user.controller.ts # Route handlers (thin)
│   │   └── order/
│   │       └── ...
│   ├── common/                  # Shared utilities
│   │   ├── errors/
│   │   │   └── app-error.ts     # Custom error hierarchy
│   │   ├── middleware/
│   │   │   ├── auth.ts
│   │   │   └── validation.ts
│   │   └── utils/
│   │       └── logger.ts
│   └── types/                   # Global type declarations
│       └── index.d.ts
├── tests/
│   ├── unit/
│   ├── integration/
│   └── fixtures/
├── prisma/                      # Or drizzle ORM schema
│   └── schema.prisma
└── scripts/
```

**Key principle: Feature-based (modules/) over layer-based. Each module is a bounded context with its own model/service/repository/routes.**

## Naming Conventions

| Element | Convention | Example |
|---------|-----------|---------|
| File | `kebab-case.ts` | `user-service.ts`, `app-error.ts` |
| Class | `PascalCase` | `UserService`, `AppError` |
| Interface | `PascalCase` or `I` prefix (team choice, be consistent) | `IUserRepository` or `UserRepository` |
| Type alias | `PascalCase` | `CreateUserRequest`, `UserId` |
| Enum | `PascalCase` | `UserRole`, `OrderStatus` |
| Function/method | `camelCase` | `getById()`, `calculateTotal()` |
| Variable/const | `camelCase` | `userId`, `is_active`, `maxRetries` |
| Constant (true const) | `UPPER_SNAKE_CASE` | `MAX_RETRY_COUNT`, `DEFAULT_TIMEOUT_MS` |
| Boolean | `is/has/can/should` prefix | `isValid`, `hasPermission`, `canDelete` |
| Private | `_prefix` (optional) | `_internalCache` |
| Generic type parameter | `T`, single letter | `Promise<T>`, `Repository<T>` |
| Test file | `*.spec.ts` or `*.test.ts` | `user.service.spec.ts` |

### Import Order (eslint import plugin)

```typescript
// 1. Node builtins (rare in modern TS)
import path from "node:path";

// 2. External packages
import { Router } from "express";
import { z } from "zod";

// 3. Internal aliases (@/ prefix configured in tsconfig paths)
import { AppError } from "@/common/errors/app-error.js";
import { logger } from "@/common/utils/logger.js";
import { UserService } from "@/modules/user/user.service.js";

// 4. Relative imports (sibling files only)
import { validateEmail } from "./user.validators.js";
import type { CreateUserDTO } from "./user.dto.js";
```

**Rules:**
- Always use `.js` extension in imports (required by ESM in Node.js)
- Use path aliases (`@/`) for cross-module imports — no deep relative paths (`../../../`)
- Use `import type` for type-only imports (tree-shakeable)

## Code Organization

### Error Handling Pattern

```typescript
// common/errors/app-error.ts

export class AppError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly statusCode: number = 500,
    public readonly cause?: Error,
  ) {
    super(message);
    this.name = "AppError";
    // Maintains proper stack trace in V8
    Error.captureStackTrace(this, this.constructor);
  }

  static NotFound(resource: string, id: string) {
    return new AppError("NOT_FOUND", `${resource} with id '${id}' not found`, 404);
  }

  static Conflict(message: string) {
    return new AppError("CONFLICT", message, 409);
  }

  static InvalidInput(message: string) {
    return new AppError("INVALID_INPUT", message, 422);
  }

  static Unauthorized(message: string = "Unauthorized") {
    return new AppError("UNAUTHORIZED", message, 401);
  }
}

// Usage in service:
// ✅ GOOD: Early returns, typed throws
async getById(userId: string): Promise<User> {
  if (!userId) throw AppError.InvalidInput("userId is required");

  try {
    const user = await this.repo.findById(userId);
    if (!user) throw AppError.NotFound("User", userId);
    return user;
  } catch (error) {
    if (error instanceof AppError) throw error; // Re-throw as-is
    throw new AppError("INTERNAL", `Failed to fetch user: ${error}`, 500, error);
  }
}

// ❌ BAD: Bare catch, silent failure
async getByIdBad(userId: string): Promise<User | null> {
  try {
    return await this.repo.findById(userId);
  } catch {
    return null; // Silent failure — bug hidden forever
  }
}
```

### Service Pattern

```typescript
// modules/user/user.service.ts
import { injectable, inject } from "tsyringe"; // or your DI choice
import type { UserRepository } from "./user.repository.js";
import type { CreateUserDTO } from "./user.dto.js";
import { AppError } from "@/common/errors/app-error.js";

@injectable()
export class UserService {
  constructor(
    @inject("UserRepository") private readonly repo: UserRepository,
  ) {}

  async create(dto: CreateUserDTO): Promise<User> {
    // 1. Validate
    if (!dto.email || !this.isValidEmail(dto.email)) {
      throw AppError.InvalidInput("Invalid email format");
    }

    // 2. Check uniqueness
    const exists = await this.repo.existsByEmail(dto.email);
    if (exists) {
      throw AppError.Conflict(`Email already registered: ${dto.email}`);
    }

    // 3. Create
    const user = await this.repo.create({
      ...dto,
      id: crypto.randomUUID(),
      createdAt: new Date(),
    });

    return user;
  }

  private isValidEmail(email: string): boolean {
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
  }
}
```

## Testing Standards

### Framework Stack

| Layer | Framework | Purpose |
|-------|-----------|---------|
| Unit | **Vitest** | Business logic, pure functions |
| Mocking | **vitest mocks** or **ts-mockito** | External dependencies |
| API testing | **supertest** + app factory | Endpoint contracts |
| Snapshot | **@snapshot/** or vitest snapshot | Output stability |

### Test Structure

```typescript
// tests/unit/modules/user/user.service.spec.ts
import { describe, it, expect, beforeEach, vi } from "vitest";
import { UserService } from "@/modules/user/user.service.js";
import { AppError } from "@/common/errors/app-error.js";
import type { UserRepository } from "@/modules/user/user.repository.js";

describe("UserService", () => {
  let service: UserService;
  let mockRepo: Mocked<UserRepository>;

  beforeEach(() => {
    mockRepo = {
      findById: vi.fn(),
      findByEmail: vi.fn(),
      create: vi.fn(),
      existsByEmail: vi.fn(),
    };
    service = new UserService(mockRepo as unknown as UserRepository);
  });

  describe("getById", () => {
    it("should return user when user exists", async () => {
      const mockUser = { id: "abc", email: "test@example.com" };
      mockRepo.findById.mockResolvedValue(mockUser);

      const result = await service.getById("abc");

      expect(result).toEqual(mockUser);
      expect(mockRepo.findById).toHaveBeenCalledWith("abc");
    });

    it("should throw NotFound when user does not exist", async () => {
      mockRepo.findById.mockResolvedValue(null);

      await expect(service.getById("unknown")).rejects.toThrow(
        expect.objectContaining({ code: "NOT_FOUND" }),
      );
    });

    it("should throw InvalidInput when userId is empty", async () => {
      await expect(service.getById("")).rejects.toThrow(
        expect.objectContaining({ code: "INVALID_INPUT" }),
      );
    });
  });
});
```

**Rules:**
- `describe` → `it` structure (Jest/Vitest style)
- `beforeEach` for clean state per test
- Mock external dependencies, test business logic in isolation
- Test name: `"should {expected behavior} when {condition}"`
- Target >80% coverage on services/repositories

## Security Checklist (TypeScript-Specific)

| Check | Rule |
|-------|------|
| SQL Injection | Use ORM parameterized queries (Prisma/drizzle/Knex). Never template literals for SQL |
| XSS | Sanitize user input before rendering. Use DOMPurify for HTML context |
| Prototype Pollution | Validate objects against Zod schemas before use. Never `Object.assign(req.body, ...)` without validation |
| Dependency Confusion | Lock `package-lock`. Run `npm audit` or `pnpm audit` in CI. Use `pnpm` over npm (better security) |
| Secret Leakage | No secrets in source. Use `.env` + validation via `zod-env-safe` or similar |
| Regex DoS | Avoid unvalidated regex on user input (ReDoS). Use safe-regex or limit input length |
| eval / Function() | NEVER use `eval()`, `new Function()`, or `vm.runInThisContext()` with user input |

## Performance Patterns

| Pattern | Anti-Pattern |
|---------|-------------|
| `Promise.all()` for parallel I/O | Sequential `await` in a loop |
| `for...of` with `await` inside for sequential needs | `Promise.all` when order doesn't matter |
| Streaming for large payloads (`ReadableStream`) | Loading everything into memory |
| Response caching (Redis/CDN) for repeated reads | Hitting DB every time |
| Connection pooling (pg pool) | Creating new connections per request |
| Lazy loading dynamic imports (`import()`) | Eager loading unused modules |
| Debounce/throttle for rate-limited APIs | Fire-and-forget without rate limiting |
| Worker threads for CPU-intensive tasks | Blocking the event loop |

## Ecosystem Toolchain

| Tool | Purpose |
|------|---------|
| **TypeScript** (strict mode) | Type system |
| **Biome** (or ESLint + Prettier) | Linter + formatter (Biome is faster, all-in-one) |
| **Vitest** | Testing framework |
| **tsx** | TypeScript execution (dev/test) |
| **tsx / tsup** | Build tooling |
| **Zod** | Runtime schema validation |
| **Prisma** / **drizzle-orm** | Database ORM |
| **tRPC** | End-to-end typed APIs (optional but recommended) |
| **pnpm** | Package manager (fast, disk-efficient) |

### Biome Config (Recommended)

```jsonc
// biome.json
{
  "$schema": "https://biomejs.dev/schemas/2.0.0/schema.json",
  "organizeImports": { "enabled": true },
  "linter": {
    "enabled": true,
    "rules": {
      "recommended": true,
      "correctness": { "noUnusedVariables": "error" },
      "security": { "noDangerousSetInnerHtml": "error" },
      "style": { "useConst": "error" },
      "suspicious": { "noExplicitAny": "warn" }
    }
  },
  "formatter": { "enabled": true },
  "javascript": { "formatter": { "quoteStyle": "single", "semicolons": "always" } },
  "typescript": { "formatter": { "quoteStyle": "single" } }
}
```
