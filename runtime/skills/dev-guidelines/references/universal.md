# 通用编码原则

与语言无关的原则，适用于所有代码，无论使用何种语言。

## SOLID 原则

| 原则 | 定义 | 违反时的代码异味 |
|-----------|-----------|------------------------|
| **S** —— 单一职责 | 一个类/模块/函数 = 一个变更的理由 | 1000+ 行做 5 件事的"上帝类" |
| **O** —— 开闭原则 | 对扩展开放，对修改关闭 | 添加一个特性需要编辑 10 个现有文件 |
| **L** —— 里氏替换 | 子类型必须能替换基类型 | 重写破坏父类契约（抛出意外错误） |
| **I** —— 接口隔离 | 多个特定接口 > 一个臃肿接口 | 客户端被迫实现不使用的方法 |
| **D** —— 依赖倒置 | 依赖抽象，而非具体实现 | `new ConcreteService()` 到处散布 |

## DRY / KISS / YAGNI

| 原则 | 规则 |
|-----------|------|
| **DRY**（不要重复自己） | 每片知识都有单一、明确的表示。重复 = 错误倍增器。 |
| **KISS**（保持简单，傻瓜） | 能工作的最简单方案几乎总是最好的。复杂性会累积利息。 |
| **YAGNI**（你不会需要它） | 不要为假设的未来需求构建抽象。实际需要时再构建，而非提前。 |

## 整洁代码启发式

### 函数
- **最多 20-30 行** —— 超过则分解
- **最多 3-4 个参数** —— 超过则使用选项对象/结构体
- **每个函数一级缩进** —— 嵌套逻辑 → 提取
- **动词-名词命名**：`getUserById()`, `calculateTotal()`, `validateInput()`
- **纯函数中无副作用** —— 将查询与变更分离

### 命名

| 类型 | 约定 | 示例 |
|------|-----------|---------|
| 类/类型 | PascalCase | `UserService`, `OrderProcessor`, `HttpClient` |
| 函数/方法 | camelCase | `getUserById`, `calculateTotal`, `formatDate` |
| 常量 | UPPER_SNAKE_CASE | `MAX_RETRY_COUNT`, `DEFAULT_TIMEOUT_MS` |
| 变量 | camelCase | `userList`, `orderTotal`, `isActive` |
| 私有成员 | _prefix 或语言约定 | `_internalCache`, `m_connectionPool` |
| 布尔变量 | is/has/can/should 前缀 | `isValid`, `hasPermission`, `canDelete`, `shouldRetry` |
| 集合 | 复数名词 | `users`, `items`, `errorMessages` |

### 注释

```plaintext
// BAD: Explains WHAT (code should be self-explanatory)
// Increment i by 1
i++;

// GOOD: Explains WHY (non-obvious business rule)
// We use >= here because the legacy API returns inclusive upper bounds,
// unlike the new spec which uses exclusive bounds. See ticket #4231.
if (pageOffset >= totalItems) { ... }
```

## 错误处理哲学

```
错误并非例外情况——它们是程序流程的正常组成部分。
将它们视为值，而非控制流的中断。

黄金法则：
  1. 在边界处处理错误（API 边缘、I/O 操作）
  2. 错误向上传播时，用上下文包装
  3. 绝不默默吞掉错误
  4. 在检测点记录，在决策点处理
  5. 区分可重试和不可重试的错误
```

## 安全基础（所有语言）

| 规则 | 详情 |
|------|--------|
| **验证输入** | 绝不信任客户端数据。在每个边界验证 schema、类型、范围、编码。 |
| **清理输出** | 根据上下文转义（HTML、SQL、shell、JSON）。使用参数化 API。 |
| **最小权限** | 以最小权限运行。生产服务中不使用 root/admin。 |
| **密钥管理** | 绝不硬编码凭证。使用环境变量、密钥管理器、保险库。 |
| **纵深防御** | 多层安全。认证 + 限流 + 输入验证 + 审计日志。 |
| **安全失败** | 默认拒绝。错误路径不应暴露信息或授予访问权限。 |
| **记录安全事件** | 认证失败、权限变更、管理员操作 —— 始终记录，绝不记录密钥。 |

## 测试哲学

```
测试金字塔：

         ╱╲
        ╱ E2E╲          ← 少量、缓慢、高置信度（仅限关键路径）
       ╱──────╲
      ╱ 集成测试 ╲       ← 中等数量、中等速度（API 契约、数据库集成）
     ╱──────────────╲
    ╱   单元测试     ╲   ← 大量、快速、隔离（业务逻辑、纯函数）
   ╱──────────────────╲

规则：
  - 单元测试：每个测试测试一个行为。命名应该像句子一样易读：
    "should_return_error_when_user_not_found"
  - 集成测试：测试组件之间的契约，而非内部实现
  - E2E 测试：测试用户旅程，而非实现细节
  - 测试必须是确定性的——不使用随机数据，不依赖时间的断言
  - 测试必须快速——单元测试套件在 30 秒内运行完毕
  - 测试必须独立——测试之间不共享状态
```
