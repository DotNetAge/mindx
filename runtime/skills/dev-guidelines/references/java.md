# Java / Spring Boot 开发指南

## 项目结构（标准 Spring Boot 布局）

```
project-name/
├── pom.xml (Maven) or build.gradle.kts (Gradle)
├── README.md
├── src/
│   ├── main/
│   │   ├── java/com/example/project/
│   │   │   ├── ProjectApplication.java       # Spring Boot entry point
│   │   │   ├── config/                       # Configuration classes
│   │   │   │   ├── SecurityConfig.java
│   │   │   │   ├── WebConfig.java
│   │   │   │   └── AppConfig.java
│   │   │   ├── common/                       # Cross-cutting concerns
│   │   │   │   ├── exception/
│   │   │   │   │   └── AppException.java
│   │   │   │   ├── dto/                     # Shared DTOs
│   │   │   │   └── util/
│   │   │   ├── domain/                      # Domain model (DDD)
│   │   │   │   ├── User.java
│   │   │   │   └── Order.java
│   │   │   ├── web/                         # Presentation layer
│   │   │   │   ├── UserController.java      # REST controller
│   │   │   │   ├── UserDTO.java             # Request/response DTOs
│   │   │   │   └── UserMapper.java          # MapStruct mapper
│   │   │   ├── service/                     # Business logic layer
│   │   │   │   ├── UserService.java         # Interface
│   │   │   │   └── impl/
│   │   │   │       └── UserServiceImpl.java # Implementation
│   │   │   ├── repository/                  # Data access layer
│   │   │   │   ├── UserRepository.java      # Spring Data JPA interface
│   │   │   │   └── custom/
│   │   │   │       └── CustomUserRepository.java
│   │   │   └── security/                    # Auth & authorization
│   │   │       ├── JwtTokenProvider.java
│   │   │       └── UserDetailsServiceImpl.java
│   │   └── resources/
│   │       ├── application.yml              # Main config
│   │       ├── application-dev.yml          # Dev profile
│   │       ├── application-prod.yml         # Prod profile
│   │       └── db/migration/                # Flyway migrations
│   └── test/java/com/example/project/
│       ├── unit/service/
│       ├── integration/
│       └── common/
├── Dockerfile
└── .github/workflows/ci.yml
```

**分层规则：**
- `web` → 调用 → `service` → 调用 → `repository`
- 不得跳过层级（web 不能直接调用 repository）
- Domain 实体与层级无关

## 命名约定（Java 标准）

| 元素 | 约定 | 示例 |
|---------|-----------|---------|
| 包名 | `lowercase`，反向 DNS | `com.example.project.service` |
| 类 / 接口 / 枚举 / Record | `PascalCase` | `UserService`, `UserRepository`, `OrderStatus` |
| 接口实现 | `PascalCase + Impl` 后缀或描述性名称 | `UserServiceImpl`, `JpaUserRepository` |
| 方法 | **camelCase**，动词优先表示动作 | `getById()`, `calculateTotal()`, `isValid()` |
| 局部变量 | **camelCase** | `userList`, `isActive` |
| 常量（`static final`） | `UPPER_SNAKE_CASE` | `MAX_RETRY_COUNT`, `DEFAULT_TIMEOUT_MS` |
| 枚举常量 | `UPPER_SNAKE_CASE` | `PENDING`, `IN_PROGRESS`, `COMPLETED` |
| 类型参数 | 单个大写字母 | `<T>`, `<E>`, `<K, V>`, `<R extends Entity>` |
| 测试类 | `{ClassName}Test` | `UserServiceTest` |
| 测试方法 | `should{ExpectedBehavior}WhenCondition` | `shouldReturnUserWhenExists` |
| Bean 名称（Spring） | camelCase | `userService`, `userRepo`, `jwtTokenProvider` |

### 包结构规则

```java
// ✅ GOOD: Clear package boundaries
package com.example.project.service;

// ✅ GOOD: Interface and impl in same package (or impl in subpackage)
public interface UserService { ... }
class UserServiceImpl implements UserService { ... }

// ✅ GOOD: Group by feature, not by technical concern
// com.example.project.user (feature) vs com.example.project.service (layer)
// For large projects, feature-based packaging is preferred.
```

## 代码组织

### Controller 模式（薄层）

```java
@RestController
@RequestMapping("/api/v1/users")
@RequiredArgsConstructor // Lombok generates constructor injection
public class UserController {

    private final UserService userService;
    private final UserMapper userMapper;

    @GetMapping("/{id}")
    public ResponseEntity<UserResponseDTO> getById(@PathVariable String id) {
        User user = userService.getById(id);           // May throw AppException
        return ResponseEntity.ok(userMapper.toDTO(user));
    }

    @PostMapping
    @ResponseStatus(HttpStatus.CREATED)
    public UserResponseDTO create(@Valid @RequestBody CreateUserRequestDTO request) {
        User user = userService.create(request);
        return userMapper.toDTO(user);
    }
}
```

### Service 模式（业务逻辑）

```java
@Service
@RequiredArgsConstructor
@Transactional(readOnly = true) // Default read-only; override for writes
public class UserServiceImpl implements UserService {

    private final UserRepository userRepository;
    private final PasswordEncoder passwordEncoder;
    private final ApplicationEventPublisher eventPublisher;

    @Override
    public User getById(String id) {
        return userRepository.findById(id)
            .orElseThrow(() -> new NotFoundException("User", id));
    }

    @Override
    @Transactional // Override: this method writes
    public User create(CreateUserRequestDTO dto) {
        // 1. Validate
        validateEmail(dto.getEmail());

        // 2. Check uniqueness
        if (userRepository.existsByEmail(dto.getEmail())) {
            throw new ConflictException("Email already registered: " + dto.getEmail());
        }

        // 3. Create entity
        User user = new User();
        user.setEmail(dto.getEmail());
        user.setName(dto.getName());
        user.setPassword(passwordEncoder.encode(dto.getPassword()));
        user = userRepository.save(user);

        // 4. Publish event
        eventPublisher.publishEvent(new UserCreatedEvent(user.getId()));

        return user;
    }

    private void validateEmail(String email) {
        if (email == null || !email.matches("^[\\w.-]+@[\\w.-]+\\.\\w+$")) {
            throw new ValidationException("Invalid email format");
        }
    }
}
```

## 错误处理模式

### 自定义异常层次结构

```java
// common/exception/AppException.java
public abstract class AppException extends RuntimeException {
    private final String code;
    private final HttpStatus status;

    protected AppException(String code, String message, HttpStatus status) {
        super(message);
        this.code = code;
        this.status = status;
    }

    public String getCode() { return code; }
    public HttpStatus getStatus() { return status; }
}

// Specific exceptions
public class NotFoundException extends AppException {
    public NotFoundException(String resource, Object id) {
        super("NOT_FOUND",
              resource + " with id '" + id + "' not found",
              HttpStatus.NOT_FOUND);
    }
}

public class ConflictException extends AppException {
    public ConflictException(String message) {
        super("CONFLICT", message, HttpStatus.CONFLICT);
    }
}

public class ValidationException extends AppException {
    public ValidationException(String message) {
        super("VALIDATION_ERROR", message, HttpStatus.UNPROCESSABLE_ENTITY);
    }
}
```

### 全局异常处理器

```java
@RestControllerAdvice
public class GlobalExceptionHandler {

    @ExceptionHandler(NotFoundException.class)
    public ResponseEntity<ErrorResponse> handleNotFound(NotFoundException e) {
        return ResponseEntity.status(e.getStatus())
            .body(new ErrorResponse(e.getCode(), e.getMessage()));
    }

    @ExceptionHandler(MethodArgumentNotValidException.class)
    public ResponseEntity<ErrorResponse> handleValidation(
            MethodArgumentNotValidException e) {
        String details = e.getBindingResult().getFieldErrors().stream()
            .map(f -> f.getField() + ": " + f.getDefaultMessage())
            .collect(Collectors.joining(", "));
        return ResponseEntity.status(HttpStatus.UNPROCESSABLE_ENTITY)
            .body(new ErrorResponse("VALIDATION_ERROR", details));
    }

    @ExceptionHandler(Exception.class) // Catch-all
    public ResponseEntity<ErrorResponse> handleGeneric(Exception e) {
        log.error("Unexpected error", e);
        return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR)
            .body(new ErrorResponse("INTERNAL", "An unexpected error occurred"));
    }
}

public record ErrorResponse(String code, String message) {}
```

## 测试标准

### 框架栈

| 层级 | 框架 | 用途 |
|-------|-----------|---------|
| 单元测试 | **JUnit 5** + **Mockito** | 业务逻辑，纯函数 |
| API 测试 | **MockMvc** + **@WebMvcTest** | 端点契约 |
| 集成测试 | **@SpringBootTest** + **Testcontainers** | 完整栈与真实数据库 |
| 断言 | **AssertJ** | 流式断言（比 JUnit 内置更好） |

### 单元测试示例

```java
@ExtendWith(MockitoExtension.class)
class UserServiceImplTest {

    @Mock
    private UserRepository userRepository;

    @Mock
    private PasswordEncoder passwordEncoder;

    @Mock
    private ApplicationEventPublisher eventPublisher;

    @InjectMocks
    private UserServiceImpl userService;

    @Test
    void shouldReturnUser_whenExists() {
        // Given
        String userId = "abc";
        User expectedUser = new User();
        expectedUser.setId(userId);
        expectedUser.setEmail("test@example.com");

        given(userRepository.findById(userId)).willReturn(Optional.of(expectedUser));

        // When
        User result = userService.getById(userId);

        // Then
        assertThat(result).isNotNull();
        assertThat(result.getId()).isEqualTo(userId);
        then(userRepository).should().findById(userId);
    }

    @Test
    void shouldThrowNotFound_whenAbsent() {
        // Given
        given(userRepository.findById("unknown")).willReturn(Optional.empty());

        // When & Then
        assertThatThrownBy(() -> userService.getById("unknown"))
            .isInstanceOf(NotFoundException.class)
            .hasFieldOrPropertyWithValue("code", "NOT_FOUND")
            .hasMessageContaining("not found");
    }

    @Test
    void shouldThrowConflict_whenEmailAlreadyRegistered() {
        // Given
        CreateUserRequestDTO dto = new CreateUserRequestDTO();
        dto.setEmail("existing@example.com");
        dto.setPassword("password123");

        given(userRepository.existsByEmail("existing@example.com")).willReturn(true);

        // When & Then
        assertThatThrownBy(() -> userService.create(dto))
            .isInstanceOf(ConflictException.class)
            .hasMessageContaining("already registered");

        then(userRepository).shouldHaveNoMoreInteractions(); // Never saved
    }
}
```

**规则：**
- BDD 风格命名：`shouldXxx_whenYyy`
- 测试体中使用 Given/When/Then 注释
- 使用 AssertJ 流式断言而非 JUnit 的 `assertEquals`
- 仅模拟被测类的直接依赖
- 服务/仓库的目标覆盖率 >80%

## 安全检查清单（Java 特定）

| 检查项 | 规则 |
|-------|------|
| SQL 注入 | 始终使用 Spring Data JPA / JDBC 参数化查询。绝不在 JPQL/HQL 中拼接字符串 |
| XSS | 如果使用 Thymeleaf，设置 `escapeHtml=true`。通过 OWASP Java HTML sanitizer 清理输出 |
| CSRF | 在 Spring Security 中启用 CSRF 保护（默认）。仅对使用 token 的无状态 API 禁用 |
| 反序列化 | 绝不接受来自不受信任来源的 `ObjectInputStream`。使用带有严格类型绑定的 JSON |
| 路径遍历 | 根据允许的目录验证文件路径。使用 `Path.normalize()` 并检查前缀 |
| 依赖漏洞 | 在 CI Maven/Gradle 阶段运行 OWASP Dependency Check 或 Snyk |
| 密钥管理 | VCS 中不包含带密钥的 properties 文件。使用 Vault、AWS Secrets Manager 或 Kubernetes secrets |
| 日志记录 | 绝不记录敏感数据（密码、token、PII）。在日志模式中使用脱敏 |

## 性能模式

| 模式 | 反模式 |
|---------|-------------|
| 读操作使用 `@Transactional(readOnly = true)` | 读操作使用不必要的写事务 |
| 使用 `@EntityGraph` / `JOIN FETCH` 防止 N+1 | 延迟加载导致 N+1 查询 |
| 列表端点使用分页（`Pageable`） | 返回无界列表 |
| 频繁读取的数据使用缓存（`@Cacheable`） | 对静态/参考数据重复查询数据库 |
| 非关键路径使用异步处理（`@Async`） | 在慢 I/O 上阻塞 HTTP 线程 |
| 连接池（HikariCP —— Spring Boot 默认） | 手动创建连接 |
| 大负载使用流式响应 | 将整个结果集加载到内存 |
| 部分数据需求使用 DTO 投影 | 只需要 2 个字段时获取完整实体图 |

## 生态工具链

| 工具 | 用途 |
|------|---------|
| **Spotless** 或 **Google Java Format** | 代码格式化器 |
| **Checkstyle** + **SpotBugs** | 静态分析 |
| **Error Prone**（Google） | 编译时错误检测 |
| **JUnit 5** + **Mockito** + **AssertJ** | 测试 |
| **Testcontainers** | 使用真实数据库的集成测试 |
| **MapStruct** | 类型安全的 Bean 映射 |
| **Lombok** | 减少样板代码（谨慎使用 —— 优先使用 record） |
| **Spring Boot Actuator** | 健康检查、指标、可观测性 |
| **Micrometer** + **Prometheus/Grafana** | 指标收集 |
| **Flyway** 或 **Liquibase** | 数据库版本化迁移 |
| **Gradle（Kotlin DSL）** 或 **Maven** | 构建工具（新项目优先使用 Gradle） |
