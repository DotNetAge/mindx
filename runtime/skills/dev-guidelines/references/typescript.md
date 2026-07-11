# TypeScript / Node.js 后端开发指南

## 项目结构（2026 标准）

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

**关键原则：基于特性（modules/）优于基于层级。每个模块都是一个有界上下文，包含自己的 model/service/repository/routes。**

## 命名约定

| 元素 | 约定 | 示例 |
|---------|-----------|---------|
| 文件 | `kebab-case.ts` | `user-service.ts`, `app-error.ts` |
| 类 | `PascalCase` | `UserService`, `AppError` |
| 接口 | `PascalCase` 或 `I` 前缀（团队选择，保持一致） | `IUserRepository` 或 `UserRepository` |
| 类型别名 | `PascalCase` | `CreateUserRequest`, `UserId` |
| 枚举 | `PascalCase` | `UserRole`, `OrderStatus` |
| 函数/方法 | `camelCase` | `getById()`, `calculateTotal()` |
| 变量/const | `camelCase` | `userId`, `is_active`, `maxRetries` |
| 常量（真正的 const） | `UPPER_SNAKE_CASE` | `MAX_RETRY_COUNT`, `DEFAULT_TIMEOUT_MS` |
| 布尔值 | `is/has/can/should` 前缀 | `isValid`, `hasPermission`, `canDelete` |
| 私有成员 | `_prefix`（可选） | `_internalCache` |
| 泛型类型参数 | `T`，单字母 | `Promise<T>`, `Repository<T>` |
| 测试文件 | `*.spec.ts` 或 `*.test.ts` | `user.service.spec.ts` |

### 导入顺序（eslint import 插件）

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

**规则：**
- 导入中始终使用 `.js` 扩展名（Node.js 中 ESM 要求）
- 跨模块导入使用路径别名（`@/`）—— 不使用深层相对路径（`../../../`）
- 仅类型导入使用 `import type`（可 tree-shake）

## 代码组织

### 错误处理模式

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

### Service 模式

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

## 测试标准

### 框架栈

| 层级 | 框架 | 用途 |
|-------|-----------|---------|
| 单元测试 | **Vitest** | 业务逻辑，纯函数 |
| Mock | **vitest mocks** 或 **ts-mockito** | 外部依赖 |
| API 测试 | **supertest** + app factory | 端点契约 |
| 快照测试 | **@snapshot/** 或 vitest snapshot | 输出稳定性 |

### 测试结构

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

**规则：**
- `describe` → `it` 结构（Jest/Vitest 风格）
- `beforeEach` 为每个测试提供干净状态
- 模拟外部依赖，隔离测试业务逻辑
- 测试名称：`"should {expected behavior} when {condition}"`
- 服务/仓库的目标覆盖率 >80%

## 安全检查清单（TypeScript 特定）

| 检查项 | 规则 |
|-------|------|
| SQL 注入 | 使用 ORM 参数化查询（Prisma/drizzle/Knex）。绝不在 SQL 中使用模板字面量 |
| XSS | 渲染前清理用户输入。HTML 上下文使用 DOMPurify |
| 原型污染 | 使用前根据 Zod schema 验证对象。未经验证绝不使用 `Object.assign(req.body, ...)` |
| 依赖混淆 | 锁定 `package-lock`。在 CI 中运行 `npm audit` 或 `pnpm audit`。优先使用 `pnpm` 而非 npm（更好的安全性） |
| 密钥泄漏 | 源代码中不包含密钥。使用 `.env` + 通过 `zod-env-safe` 或类似工具验证 |
| 正则 DoS | 避免对用户输入使用未验证的正则（ReDoS）。使用 safe-regex 或限制输入长度 |
| eval / Function() | 绝不使用 `eval()`、`new Function()`，或带用户输入的 `vm.runInThisContext()` |

## 性能模式

| 模式 | 反模式 |
|---------|-------------|
| 并行 I/O 使用 `Promise.all()` | 循环中顺序 `await` |
| 顺序需求时在 `for...of` 内使用 `await` | 顺序无关时使用 `Promise.all` |
| 大负载使用流式处理（`ReadableStream`） | 将所有内容加载到内存 |
| 重复读取使用响应缓存（Redis/CDN） | 每次都查询数据库 |
| 连接池（pg pool） | 每个请求创建新连接 |
| 动态导入延迟加载（`import()`） | 预加载未使用的模块 |
| 限流 API 使用防抖/节流 | 无限流的 fire-and-forget |
| CPU 密集型任务使用 Worker threads | 阻塞事件循环 |

## 生态工具链

| 工具 | 用途 |
|------|---------|
| **TypeScript**（严格模式） | 类型系统 |
| **Biome**（或 ESLint + Prettier） | Linter + 格式化器（Biome 更快，一体化） |
| **Vitest** | 测试框架 |
| **tsx** | TypeScript 执行（开发/测试） |
| **tsx / tsup** | 构建工具 |
| **Zod** | 运行时 schema 验证 |
| **Prisma** / **drizzle-orm** | 数据库 ORM |
| **tRPC** | 端到端类型化 API（可选但推荐） |
| **pnpm** | 包管理器（快速，磁盘高效） |

### Biome 配置（推荐）

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
