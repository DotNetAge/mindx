# Java / Spring Boot Development Guidelines

## Project Structure (Standard Spring Boot Layout)

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

**Layering rule:**
- `web` → calls → `service` → calls → `repository`
- Never skip layers (web must not call repository directly)
- Domain entities are layer-agnostic

## Naming Conventions (Java Standard)

| Element | Convention | Example |
|---------|-----------|---------|
| Package | `lowercase`, reverse DNS | `com.example.project.service` |
| Class / Interface / Enum / Record | `PascalCase` | `UserService`, `UserRepository`, `OrderStatus` |
| Interface implementation | `PascalCase + Impl` suffix or descriptive name | `UserServiceImpl`, `JpaUserRepository` |
| Method | **camelCase**, verb-first for actions | `getById()`, `calculateTotal()`, `isValid()` |
| Local variable | **camelCase** | `userList`, `isActive` |
| Constant (`static final`) | `UPPER_SNAKE_CASE` | `MAX_RETRY_COUNT`, `DEFAULT_TIMEOUT_MS` |
| Enum constant | `UPPER_SNAKE_CASE` | `PENDING`, `IN_PROGRESS`, `COMPLETED` |
| Type parameter | Single uppercase letter | `<T>`, `<E>`, `<K, V>`, `<R extends Entity>` |
| Test class | `{ClassName}Test` | `UserServiceTest` |
| Test method | `should{ExpectedBehavior}WhenCondition` | `shouldReturnUserWhenExists` |
| Bean name (Spring) | camelCase | `userService`, `userRepo`, `jwtTokenProvider` |

### Package Structure Rules

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

## Code Organization

### Controller Pattern (Thin)

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

### Service Pattern (Business Logic)

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

## Error Handling Patterns

### Custom Exception Hierarchy

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

### Global Exception Handler

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

## Testing Standards

### Framework Stack

| Layer | Framework | Purpose |
|-------|-----------|---------|
| Unit | **JUnit 5** + **Mockito** | Business logic, pure functions |
| API testing | **MockMvc** + **@WebMvcTest** | Endpoint contracts |
| Integration | **@SpringBootTest** + **Testcontainers** | Full stack with real DB |
| Assertions | **AssertJ** | Fluent assertions (better than JUnit built-in) |

### Unit Test Example

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

**Rules:**
- BDD-style naming: `shouldXxx_whenYyy`
- Given/When/Then comments in test body
- Use AssertJ fluent assertions over JUnit's `assertEquals`
- Mock only direct dependencies of the class under test
- Target >80% coverage on services/repositories

## Security Checklist (Java-Specific)

| Check | Rule |
|-------|------|
| SQL Injection | Always use Spring Data JPA / JDBC parameterized queries. Never string concatenation in JPQL/HQL |
| XSS | Set `escapeHtml=true` on Thymeleaf if used. Sanitize output via OWASP Java HTML sanitizer |
| CSRF | Enable CSRF protection in Spring Security (default). Disable only for stateless APIs using tokens |
| Deserialization | Never accept `ObjectInputStream` from untrusted sources. Use JSON with strict type binding |
| Path Traversal | Validate file paths against allowed directories. Use `Path.normalize()` and check prefix |
| Dependency Vulnerabilities | Run OWASP Dependency Check or Snyk in CI Maven/Gradle phase |
| Secret Management | No properties files with secrets in VCS. Use Vault, AWS Secrets Manager, or Kubernetes secrets |
| Logging | Never log sensitive data (passwords, tokens, PII). Use masking in log patterns |

## Performance Patterns

| Pattern | Anti-Pattern |
|---------|-------------|
| `@Transactional(readOnly = true)` for read operations | Unnecessary write transactions for reads |
| `@EntityGraph` / `JOIN FETCH` for N+1 prevention | Lazy loading causing N+1 queries |
| Pagination (`Pageable`) for list endpoints | Returning unbounded lists |
| Caching (`@Cacheable`) for frequently read data | Repeated DB queries for static/reference data |
| Async processing (`@Async`) for non-critical paths | Blocking HTTP thread on slow I/O |
| Connection pooling (HikariCP — default in Spring Boot) | Creating connections manually |
| Streaming response for large payloads | Loading entire result set into memory |
| DTO projection for partial data needs | Fetching full entity graphs when only 2 fields needed |

## Ecosystem Toolchain

| Tool | Purpose |
|------|---------|
| **Spotless** or **Google Java Format** | Code formatter |
| **Checkstyle** + **SpotBugs** | Static analysis |
| **Error Prone** (Google) | Compile-time bug detection |
| **JUnit 5** + **Mockito** + **AssertJ** | Testing |
| **Testcontainers** | Integration tests with real databases |
| **MapStruct** | Type-safe bean mapping |
| **Lombok** | Boilerplate reduction (use sparingly — prefer records where possible) |
| **Spring Boot Actuator** | Health checks, metrics, observability |
| **Micrometer** + **Prometheus/Grafana** | Metrics collection |
| **Flyway** or **Liquibase** | Database versioned migrations |
| **Gradle (Kotlin DSL)** or **Maven** | Build tool (Gradle preferred for new projects) |
