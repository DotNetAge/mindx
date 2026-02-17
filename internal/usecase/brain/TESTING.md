# Brain 集成测试

本目录包含 Brain 的集成测试，使用真实组件（Memory、SkillMgr），不使用 Mock。

## 测试文件

| 文件 | 描述 |
|------|------|
| `suite_test.go` | 测试套件基础设置 |
| `scenario1_test.go` | 场景 1: 普通对话 |
| `scenario2_test.go` | 场景 2: 多轮对话 |
| `scenario3_test.go` | 场景 3: 记忆参考 |
| `scenario4_test.go` | 场景 4: 发送意图识别 |
| `scenario5_test.go` | 场景 5: 工具执行 |

## 测试场景

### 场景 1: 普通对话

测试 Brain 处理简单的日常对话能力：

- **简单问候**: 测试基本的打招呼功能
- **简单问题**: 测试回答天气、时间、常识类问题
- **闲聊**: 测试开放式对话
- **人设反映**: 测试回答是否反映设定的人设

### 场景 2: 多轮对话

测试 Brain 是否能保持上下文的一致性：

- **上下文一致性**: 测试能否记住之前对话的内容
- **多轮对话**: 模拟连续的多轮对话
- **话题连续性**: 测试话题的连贯性

### 场景 3: 记忆参考

测试 Brain 是否能参考记忆的内容进行流畅对话：

- **记忆检索**: 测试从长时记忆中获取相关信息
- **编程兴趣记忆**: 测试特定领域的记忆
- **地点记忆**: 测试位置相关记忆
- **记忆整合**: 测试多条记忆的整合
- **关键词搜索**: 测试基于关键词的记忆搜索

### 场景 4: 发送意图识别

测试 Brain 识别用户的转发意图：

- **发送到飞书**: 识别发送到飞书的意图
- **发送到微信**: 识别发送到微信的意图
- **发送到 QQ**: 识别发送到 QQ 的意图
- **无发送意图**: 识别不需要发送的普通对话

### 场景 5: 工具执行

测试 Brain 执行工具的对话：

- **天气查询工具**: 测试天气工具调用
- **时间查询工具**: 测试时间工具调用
- **发送消息工具**: 测试消息发送工具调用
- **工具发现**: 测试工具搜索功能
- **工具执行**: 测试直接执行工具

## 运行测试

### 运行所有测试

```bash
go test ./internal/usecase/brain/ -v
```

### 运行特定场景

```bash
# 场景 1
go test ./internal/usecase/brain/ -v -run TestScenario1Suite

# 场景 2
go test ./internal/usecase/brain/ -v -run TestScenario2Suite

# 场景 3
go test ./internal/usecase/brain/ -v -run TestScenario3Suite

# 场景 4
go test ./internal/usecase/brain/ -v -run TestScenario4Suite

# 场景 5
go test ./internal/usecase/brain/ -v -run TestScenario5Suite
```

### 运行特定测试用例

```bash
go test ./internal/usecase/brain/ -v -run TestScenario1SimpleGreeting
```

## 环境要求

- OpenAI API Key（通过环境变量 `OPENAI_API_KEY` 设置）
- 如果未设置 API Key，测试会使用 "test-key"，但实际 API 调用会失败

## 注意事项

1. 集成测试使用真实的 Memory、SkillMgr 组件
2. 测试数据存储在临时目录中，测试结束后自动清理
3. 部分测试可能需要真实的 LLM API 调用才能完全验证
4. 测试中的人设反映功能需要重构 `createTestBrain` 以支持动态设置人设
