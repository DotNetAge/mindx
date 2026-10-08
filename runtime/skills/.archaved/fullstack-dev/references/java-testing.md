# Java 测试专项参考

> 本文件是 `test-generation` 的 Java/JUnit 补充参考，**不作为独立子技能计数，也不提供 `@` 显式调用入口**。命中 Java、JUnit 5、Mockito、Spring Boot Test、MockMvc、切片测试、Testcontainers 等信号时按需加载，并与 `java-development.md`、`api-design.md` 协同。

## 测试分层

- **单元测试（JUnit 5 + Mockito）**：覆盖纯业务逻辑、边界条件、异常分支；**不启动 Spring 容器**，被测对象直接 `new`，依赖用构造器注入替身。
- **切片测试（`@WebMvcTest` / `@DataJpaTest` / `@JsonTest`）**：只加载目标层与必要配置，比全上下文快一个量级；Web 层与持久层验证的**默认优先级**。
- **集成测试（`@SpringBootTest`）**：覆盖跨层链路、事务、真实外部依赖替身（Testcontainers）；仅在切片无法覆盖时使用。
- **回归测试**：每个已修复缺陷至少补一条可复现用例（失败先行或明确复现步骤）。

## 基础规则

1. 一个测试只验证一个主要行为；命名采用 `方法名_场景_期望`（如 `transfer_insufficientBalance_throws`），不用 `test1`。
2. 同时断言**返回值与副作用**：响应状态/JSON、数据库状态、事件发布、MQ 发送、缓存写入。
3. 断言用 AssertJ（`assertThat`）或 JUnit 5 原生断言；**禁止只断言"不抛异常"**当作通过。
4. 时间、随机、时区、ID 生成必须可注入（`Clock`、`IdGenerator`）；测试内禁止 `System.currentTimeMillis()`、`LocalDate.now()` 直取。
5. 测试必须可重复执行：不依赖执行顺序，不共享可变静态状态；`@BeforeEach` 复位。
6. 涉及权限与校验时，**允许路径与拒绝路径成对覆盖**，不得只测 happy path。
7. 测试数据构造集中在 `@TestConfiguration` / Builder / Fixture，不在测试主体里散落大段 setter 链。

## Spring 容器策略

- **默认不启动容器**：单元测试优先纯 Mockito；只有需要容器能力时才升到切片或集成。
- 需要容器时**先切片后集成**，避免所有测试都 `@SpringBootTest`（上下文启动成本会拖垮整个测试套件）。
- `@SpringBootTest` 的 `webEnvironment` 按需选择：`MOCK`（默认，不占端口）/ `RANDOM_PORT`（需真实 HTTP 栈时）。
- **上下文缓存**：`@DirtiesContext` 慎用（销毁缓存会导致后续测试重新启动上下文）；`@MockBean` / `@TestConfiguration` 尽量集中声明以减少缓存分裂。
- 测试配置放 `src/test/resources/application-test.yml`（`@ActiveProfiles("test")`），不污染主配置；测试专用 `Bean` 用 `@Primary` 或 `@Profile("test")` 隔离。

## 数据与外部依赖

| 依赖类型 | 推荐策略 | 说明 |
| - | - | - |
| 关系型数据库 | `@DataJpaTest` + H2（快）或 **Testcontainers**（行为一致性优先） | MySQL/PostgreSQL 特有语法、函数、锁行为必须用 Testcontainers 验证 |
| 事务 | 测试方法加 `@Transactional` 自动回滚 | ⚠️ 它会让延迟约束、异步副作用、`afterCommit` **不生效**；需真实提交时用 `@Commit` 或 Testcontainers |
| 缓存 / Redis | Testcontainers 或 `@MockBean` 替身 | 断言"命中/未命中"与过期行为，不只断言返回值 |
| 消息队列 | Testcontainers 或注入的 mock 生产者 | 断言消息内容、发送次数、重试次数 |
| 外部 HTTP | WireMock / MockWebServer | 必须显式模拟**超时**与 **5xx**，验证降级路径 |
| 时间 | 注入 `Clock`，`Clock.fixed(...)` | 固定"现在"后断言过期/超时逻辑 |
| 文件系统 | JUnit 5 `@TempDir` | 不写真实业务目录 |

- 测试数据隔离：数据库用回滚或每类独立 schema；**禁止依赖其他测试的残留数据**。

## Web 层测试模板

```java
@WebMvcTest(OrderController.class)
class OrderControllerTest {

    @Autowired MockMvc mockMvc;
    @MockBean OrderService orderService;   // 只替身依赖，不启动真实服务

    @Test
    void create_validRequest_returns201() throws Exception {
        when(orderService.create(any())).thenReturn(new OrderVO(1L, "PAID"));

        mockMvc.perform(post("/api/orders")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"sku\":\"A1\",\"qty\":2}"))
                .andExpect(status().isCreated())
                .andExpect(jsonPath("$.data.id").value(1))
                .andExpect(jsonPath("$.data.internalCost").doesNotExist());  // 敏感字段不外泄

        verify(orderService).create(any());    // 断言副作用：确实调用了服务
    }
}
```

- 统一响应 envelope 项目按 `api-design` 约定断言（如 `$.code`、`$.message`、`$.data`）。
- 大结果集断言分页字段（`total`、`page`、`hasNext`）而不只断言数组长度。

## 校验与授权测试

- **参数校验**：断言 400（或项目统一校验错误码）+ 字段错误路径，并确认**无数据库副作用**。
- **授权拒绝**：断言 403，且响应不泄露内部实现细节（异常类名、SQL、堆栈）。
- **认证缺失/过期**：断言 401；token 过期与签名非法分别覆盖。
- **越权（IDOR）**：用 A 用户身份访问 B 用户资源，断言 403 或 404（按项目口径统一），**必须存在此用例**。
- **响应契约**：断言 `password`、`idCard`、`internalId` 等敏感字段**不存在**（`jsonPath("$.password").doesNotExist()`）。
- 权限矩阵：角色 × 接口的关键组合应成对覆盖允许/拒绝，而非只测最高权限。

## Mock 与替身

| 场景 | 工具 | 断言方式 |
| - | - | - |
| 依赖接口行为 | Mockito `when / thenReturn / thenThrow` | `verify(...)`、`ArgumentCaptor` 捕获入参 |
| 静态方法 / 构造器 | Mockito `mockStatic` / `mockConstruction`（**谨慎使用**，优先重构为可注入依赖） | `verify` |
| 外部 HTTP 服务 | WireMock / MockWebServer | 请求匹配 + 响应桩（含超时、5xx） |
| 数据库真实行为 | Testcontainers | 真实 SQL 与约束断言 |
| 消息队列 | Testcontainers / mock 生产者 | 消息内容与发送次数 |
| 时间 | 注入 `Clock` | 固定时间下的行为断言 |

- Mock 只用于隔离**外部边界或昂贵依赖**；核心业务规则必须真实执行。
- **禁止空测试**：只验证"mock 被调用"而不验证业务结果的测试视为无效。
- `@MockBean` 会分裂 Spring 上下文缓存；能用构造器注入纯 Mockito 的优先用纯 Mockito。

## N+1 与性能回归

- JPA 列表查询记录 SQL 数量基线（`hibernate.generate_statistics` 或 `datasource-proxy`）；新增关联字段时必须复测。
- 断言"关联字段存在" **+** "查询数不超基线"，双重约束防 N+1 回归。
- 批量写入断言批次数与事务边界，不只看最终行数。
- 性能断言**不替代功能断言**；需量化对比时协同 `performance-benchmark`（JMH）。

## 并发与异步测试

- `@Async` / 线程池：用 `Awaitility` 或 `CountDownLatch` 等待，**禁止 `Thread.sleep` 猜时间**。
- 超时行为：`assertTimeoutPreemptively` 验证真实超时与中断后的资源释放。
- 并发竞争：`ExecutorService` 起 N 个线程压同一资源，断言"只成功一次"（幂等键 / 乐观锁 / 分布式锁）。
- 消息消费：Testcontainers 或注入同步 `Executor`，断言处理结果、重试次数与死信去向。
- 定时任务：手动触发（`TaskScheduler` 替身）后断言副作用，不依赖真实时钟等待。

## 运行命令

```bash
# Maven：单测（Surefire，*Test）与集成测试（Failsafe，*IT）分离
mvn test
mvn test -Dtest=OrderControllerTest
mvn verify                                  # 含集成测试与质量门禁

# Gradle
./gradlew test
./gradlew test --tests '*OrderControllerTest'
./gradlew build
```

- 覆盖率用 JaCoCo 报告核对**新增代码**覆盖；**禁止为凑覆盖率写无断言测试**。
- 首次运行的失败用例必须给出阻塞原因（环境 / 依赖 / 数据），不得静默跳过（`@Disabled` 需注明原因与恢复条件）。

## 交付证据

交付 Java 测试相关任务时，至少提供：

- 新增/修改的测试文件路径。
- 运行命令（含 `-Dtest=` / `--tests` 单类过滤命令）。
- 通过/失败结果（`Tests run: X, Failures: 0, Errors: 0, Skipped: 0`）。
- 如失败：阻塞原因与下一步所需输入（缺失依赖、环境变量、测试数据等）。
