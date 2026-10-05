# Java 专项开发

> 本文件是 `dev-expert` 的 Java/Spring 专项补充参考，不作为独立子技能计数，也不提供 `@` 显式调用入口。命中 Java、Spring Boot、Spring、MyBatis、Hibernate、JPA、Maven、Gradle、JUnit、Mockito、JVM、GC、线程池、并发、Java 源码分析等信号时，按需与 `code-generation`、`code-review`、`test-generation`、`performance-benchmark`、`software-project` 协同加载。

## 适用边界

- 适用：Java 项目源码走读、Spring Boot 应用开发、接口设计、数据访问、单元测试、并发分析、JVM/GC 基础诊断。
- 不适用：未确认语言/框架时强行套用 Spring 结构；Android、纯前端、PHP/CMS 项目不使用本参考。
- Java 版本：以项目 `pom.xml`、`build.gradle`、CI 和线上 JDK 为准。Java 17/21 特性只能在确认运行版本支持后使用。

## 源码分析流程

1. **先读结构**：查看目录、包名、模块、构建文件，识别 Controller/Service/Repository/Domain 等分层。
2. **找入口**：定位 `main` 类、Controller、Message Listener、Scheduled Task、CommandLineRunner 或核心业务入口。
3. **逐层深入**：沿调用链追踪 Controller -> Service -> Repository/Mapper -> 外部依赖，标注关键分支和异常路径。
4. **识别依赖**：梳理类间依赖、接口实现、循环依赖、配置注入和第三方组件。
5. **总结输出**：给出项目概览、核心流程、关键类职责、风险点和验证建议。

### 源码分析输出模板

```markdown
## 项目概览
- 技术栈：
- 分层架构：
- 构建工具：

## 核心流程
1. 入口：
2. 业务逻辑：
3. 数据访问：
4. 外部依赖：

## 关键类说明
| 类名 | 职责 | 关键方法 |
| --- | --- | --- |
| ... | ... | ... |

## 质量与风险
- 正确性：
- 性能：
- 安全：
- 并发：
- 建议验证：
```

## Spring Boot 标准结构

```text
src/main/java/com/example/
├── config/          # 配置类
├── controller/      # REST API 层
├── service/         # 业务逻辑层
│   └── impl/
├── repository/      # JPA Repository 或 DAO
├── mapper/          # MyBatis Mapper
├── domain/entity/   # 实体类
├── dto/             # 请求/响应 DTO
├── exception/       # 自定义异常与统一异常处理
└── util/            # 无状态工具类
```

### 常见注解速查

| 场景 | 注解 |
| - | - |
| REST Controller | `@RestController`, `@RequestMapping`, `@GetMapping`, `@PostMapping` |
| 服务层 | `@Service`, `@Transactional` |
| 仓储层 | `@Repository`, `@Mapper` |
| 依赖注入 | 构造器注入、`@RequiredArgsConstructor` |
| 配置 | `@Configuration`, `@Bean`, `@ConfigurationProperties` |
| 参数校验 | `@Valid`, `@Validated`, `@NotNull`, `@NotBlank`, `@Size` |
| 缓存 | `@Cacheable`, `@CacheEvict`, `@CachePut` |
| 异常处理 | `@RestControllerAdvice`, `@ExceptionHandler` |

## 编码规范

- 类名使用 PascalCase，方法/变量使用 camelCase，常量使用 UPPER_SNAKE_CASE。
- 优先构造器注入，避免字段注入；依赖尽量声明为接口类型。
- 不吞异常；捕获后必须记录上下文、转换为业务异常或继续抛出。
- 使用 SLF4J 日志，禁止 `System.out.println()` 进入业务代码。
- Controller 不写业务细节，Service 不直接拼 HTTP 响应，Repository/Mapper 不承载业务规则。
- `@Transactional` 放在业务边界；注意自调用、异常类型和只读事务配置。
- DTO 与 Entity 分离，禁止把 JPA Entity 直接暴露为长期 API 响应契约。
- 魔法数字、状态码、业务类型优先使用 enum 或常量表达。

## 数据访问

- JPA：关注懒加载、N+1、事务边界、级联删除、脏检查和分页查询。
- MyBatis：关注 XML/注解 SQL 参数绑定，禁止 `${}` 拼接外部输入，优先 `#{}`。
- 批量写入：使用批处理或分批提交，明确事务大小与失败回滚策略。
- 查询性能：为高频 WHERE/JOIN/ORDER BY 字段提供索引依据，必要时协同 `mysql-database`。

## 并发与异步

- 线程池必须显式配置核心线程数、最大线程数、队列长度、拒绝策略和线程名。
- `CompletableFuture` 需指定 Executor，避免默认 ForkJoinPool 被阻塞任务占满。
- 共享状态必须说明线程安全策略：不可变对象、局部变量、锁、并发集合或原子类。
- 分布式场景关注幂等、重试、超时、熔断、降级和链路追踪。
- `@Async`、调度任务、消息消费必须有异常处理和监控日志。

## 测试

- 单元测试：JUnit 5 + Mockito，隔离纯业务逻辑、边界条件和异常分支。
- Spring 集成测试：`@SpringBootTest`、`@WebMvcTest`、`@DataJpaTest` 按范围选择，避免所有测试都启动完整上下文。
- 外部依赖：数据库、Redis、MQ 可使用 Testcontainers 或测试替身。
- Web 接口：使用 MockMvc 或 WebTestClient 断言状态码、响应体和副作用。
- 每个修复缺陷至少补一条回归测试。

## 安全

- **认证授权**：Spring Security 优先用方法级注解（`@PreAuthorize` / `@PostAuthorize`）或集中式 `SecurityFilterChain` 配置；禁止在 Controller 手写 `if (role.equals("ADMIN"))` 式判断。
- **越权（IDOR）**：按 id 取资源的接口必须校验归属（`...where user_id = 当前登录用户`），不能只依赖"前端不会传别人的 id"。
- **SQL 注入**：MyBatis 一律 `#{}`（预编译），禁用 `${}` 拼接；JPA 不用字符串拼接 JPQL；原生 SQL 必须参数化。
- **反序列化**：禁止把外部数据交给原生 `ObjectInputStream.readObject`；改 JSON（Jackson）+ 多态类型白名单，禁用 `enableDefaultTyping`。
- **XXE**：`DocumentBuilderFactory` 必须设 `disallow-doctype-decl` 与 `FEATURE_SECURE_PROCESSING`；`SAXReader`、`XMLInputFactory`、`TransformerFactory` 同理。
- **SSRF**：服务端发起 URL 请求前做目标白名单 / 内网地址与元数据地址拒绝。
- **敏感信息**：日志与响应禁止输出口令、token、身份证、手机号全量；DTO 层统一脱敏；业务代码禁用 `printStackTrace()`（改结构化日志）。
- **口令与加密**：口令用 bcrypt / argon2；摘要用 SHA-256+ 且加盐；对称加密用 AES-GCM；禁用 MD5、SHA-1、DES、ECB 模式。
- **依赖与供应链**：引入新依赖前查 CVE 并锁定版本（协同 `threat-modeling`）；构建期跑 OWASP dependency-check。
- **上传与路径**：文件名与落盘路径做白名单 + 规范化校验，禁止 `new File(basePath + userInput)`。

## API 设计约定

- **统一响应体**：按项目既有 envelope（如 `{code, message, data}`）返回，不混用多种结构；错误码集中定义并可检索。
- **状态码语义**：400 参数错误 / 401 未认证 / 403 无权限 / 404 不存在 / 409 冲突 / 422 校验失败 / 5xx 服务端；**禁止一律 200 包错误**。
- **版本与兼容**：按项目约定用路径版本（`/api/v1`）或 header 版本；已发布字段只增不改不删，废弃走 `@Deprecated` + 迁移公告。
- **参数校验**：`@Valid` + Bean Validation 落在 DTO 上；Controller 内不堆叠手写 if 校验。
- **幂等**：下单、支付、重试友好的写接口须支持幂等键（`Idempotency-Key` 或业务唯一键）。
- **分页与排序**：统一入参与统一响应字段；大结果集用游标分页，避免深分页。
- **超时与限流**：对外接口声明超时与限流策略；批量接口限制单次条数上限并分片。
- **文档**：接口变更同步 OpenAPI（springdoc）注释，示例请求/响应可直接用于联调。
- **分工**：通用 API 规范以 `api-design.md` 为准，本节只补 Java / Spring 落地约定。

## JVM 与性能

基础诊断参数示例，使用前需确认线上 JDK、容器内存和运维规范：

```bash
# 堆内存
-Xms512m -Xmx2g

# Java 11+ 常用 GC
-XX:+UseG1GC

# GC 日志
-Xlog:gc*:file=gc.log:time,uptime:filecount=5,filesize=10m
```

性能问题优先采集证据：接口耗时、线程 dump、堆 dump、GC 日志、慢 SQL、连接池指标。没有基线不得声称优化有效。

## 验证命令

按项目构建工具选择：

```bash
mvn test
mvn verify
mvn spring-boot:run
./gradlew test
./gradlew build
./gradlew bootRun
```

## 关联 reference

- `code-generation`：生成 Controller、Service、DTO、Repository、测试代码。
- `code-review`：按 Java 专项审查清单补充正确性、安全、性能、并发检查。
- `test-generation` / `java-testing`：生成 JUnit 5、Mockito、MockMvc、Testcontainers 测试（分层策略与模板见 `java-testing.md`）。
- `performance-benchmark`：JVM、GC、线程池、接口耗时和基准对比（JMH）。
- `api-design`：通用 API 设计规范；本文件「API 设计约定」只补 Spring 落地部分。
- `mysql-database`：索引依据、慢 SQL、分页与批量写入规范。
- `threat-modeling`：依赖引入前审查门禁、STRIDE 与供应链安全。
- `production-readiness`：六维 PRR、发布回滚、监控告警与巡检。
- `architecture-decision`：分层 / 模块边界等不可逆取舍的决策记录模板。
- `software-project`：需求到交付的全生命周期总控与阶段编排。
