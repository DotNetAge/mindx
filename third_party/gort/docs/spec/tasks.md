# Gort 重构任务分解 (Tasks)

> 基于 spec.md 的详细规格，将重构工作拆分为可执行的任务单元。

## 阶段一: P0 紧急修复（预计 4-6 小时）

### Task 1.1: 增强 channel.Message 结构体
- **优先级**: P0-Critical
- **文件**: `pkg/channel/channel.go`
- **预估**: 30 分钟
- **依赖**: 无
- **步骤**:
  1. 在 `Message` struct 中添加 `ChannelID string` 字段
  2. 在 `UserInfo` struct 中添加 `Platform string` 字段
  3. 添加 `MessageTypeMarkdown`, `MessageTypeNews`, `MessageTypeVoice`, `MessageTypeTemplateCard` 常量
  4. 实现 `GetMetadata(key string) (interface{}, bool)` 方法
  5. 实现 `SetMetadata(key string, value interface{})` 方法
  6. 实现 `NewMessage()` 构造函数
- **验证**: `go build ./pkg/channel` 通过

### Task 1.2: 修复 wechat channel 编译错误
- **优先级**: P0-Critical
- **文件**: `pkg/channel/wechat/wechat.go`
- **预估**: 20 分钟
- **依赖**: Task 1.1
- **步骤**:
  1. HandleWebhook 中 `msg.ChannelID = "wechat"` 改为使用字段赋值
  2. UserInfo 赋值添加 `Platform: "wechat"`
  3. 所有 `msg.SetMetadata(...)` 确认方法存在
  4. Timestamp 保持为字符串（检查 time.Unix 输出格式）
- **验证**: `go build ./pkg/channel/wechat` 通过

### Task 1.3: 修复 dingtalk channel 编译错误
- **优先级**: P0-Critical
- **文件**: `pkg/channel/dingtalk/dingtalk.go`
- **预估**: 15 分钟
- **依赖**: Task 1.1
- **步骤**: 同 1.2 模式，适配 dingtalk 特定字段
- **验证**: `go build ./pkg/channel/dingtalk` 通过

### Task 1.4: 修复 telegram channel 编译错误
- **优先级**: P0-Critical
- **文件**: `pkg/channel/telegram/telegram.go`
- **预估**: 25 分钟
- **依赖**: Task 1.1
- **步骤**:
  1. `time.Now()` 赋值给 `Timestamp string` 需要格式化
  2. 所有 SetMetadata/GetMetadata 调用确认
  3. ChannelID 和 Platform 字段补充
- **验证**: `go build ./pkg/channel/telegram` 通过

### Task 1.5: 修复 feishu channel 编译错误
- **优先级**: P0-Critical
- **文件**: `pkg/channel/feishu/feishu.go`
- **预估**: 20 分钟
- **依赖**: Task 1.1
- **步骤**: 同上模式
- **验证**: `go build ./pkg/channel/feishu` 通过

### Task 1.6: 修复 slack channel 编译错误
- **优先级**: P0-Critical
- **文件**: `pkg/channel/slack/slack.go`
- **预估**: 20 分钟
- **依赖**: Task 1.1
- **步骤**:
  1. 替换 `NewMessage(...)` 为新的构造函数调用
  2. 添加 `Platform` 字段到 UserInfo
  3. 确认 `MessageTypeMarkdown` 常量存在
  4. GetMetadata/SetMetadata 调用确认
- **验证**: `go build ./pkg/channel/slack` 通过

### Task 1.7: 修复 discord channel 编译错误
- **优先级**: P0-Critical
- **文件**: `pkg/channel/discord/discord.go`
- **预估**: 20 分钟
- **依赖**: Task 1.1
- **步骤**: 同 slack 模式
- **验证**: `go build ./pkg/channel/discord` 通过

### Task 1.8: 修复 wecom channel 编译错误
- **优先级**: P0-Critical
- **文件**: `pkg/channel/wecom/wecom.go`
- **预估**: 15 分钟
- **依赖**: Task 1.1
- **步骤**:
  1. 添加缺失的 MessageType 常量引用
  2. GetMetadata/SetMetadata 调用修复
- **验证**: `go build ./pkg/channel/wecom` 通过

### Task 1.9: 修复 whatsapp/messenger/imessage channel 编译错误
- **优先级**: P0-Critical
- **文件**: `pkg/channel/whatsapp/whatsapp.go`, `pkg/channel/messenger/messenger.go`, `pkg/channel/imessage/imessage.go`
- **预估**: 20 分钟
- **依赖**: Task 1.1
- **步骤**: 各文件按相同模式修复
- **验证**: 对应包编译通过

### Task 1.10: 修复测试文件编译错误
- **优先级**: P0-Critical
- **文件**: `pkg/channel/channel_test.go`, `pkg/channel/channel_bench_test.go`, 以及各 channel 的 `_test.go`
- **预估**: 30 分钟
- **依赖**: Task 1.1 - 1.9
- **步骤**:
  1. 更新所有测试文件中的类型引用
  2. 更 NewMessage 调用签名
  3. 更新字段访问方式
- **验证**: `go test ./pkg/channel/...` 编译通过

### Task 1.11: 全量编译验证
- **优先级**: P0-Critical
- **预估**: 10 分钟
- **依赖**: Task 1.1 - 1.10
- **步骤**:
  1. `go build ./...` 全量编译
  2. `go vet ./...` 静态分析
  3. `go test ./... -count=1` 运行全部测试
- **验收标准**: 零错误零警告

---

## 阶段二: P1 安全加固（预计 5-7 小时）

### Task 2.1: WebSocket Origin 白名单
- **优先级**: P1-High
- **文件**: `pkg/gateway/gateway.go`, `pkg/gateway/constants.go`
- **预估**: 1 小时
- **依赖**: Task 1.11
- **步骤**:
  1. 定义 `WSConfig` 结构体和默认值
  2. 实现 `matchOrigin()` 通配符匹配函数
  3. 将全局 upgrader 改为可配置
  4. 添加 `WithWSConfig()` Option
  5. 在 `Start()` 中初始化 upgrader
- **验收**: 测试验证 Origin 过滤生效

### Task 2.2: wechat 凭证安全传递
- **优先级**: P1-High
- **文件**: `pkg/channel/wechat/wechat.go`
- **预估**: 45 分钟
- **依赖**: 无（可与 P0 并行）
- **步骤**:
  1. 修改 `refreshToken()`: 将 appid+secret 从 URL 移至 POST Body
  2. 修改 Content-Type 为 application/json
  3. 更新 API 响应解析逻辑
- **验收**: URL 中不再包含 secret 参数

### Task 2.3: 其他 channel 凭证审查与修复
- **优先级**: P1-High
- **文件**: 所有 channel 实现文件
- **预估**: 1 小时
- **依赖**: 无
- **步骤**:
  1. 审查所有 HTTP 请求中是否有凭证出现在 URL
  2. 逐一修复发现的问题
- **验收**: grep 确认无 secret/token/key 出现在 URL 构建中

### Task 2.4: 输入验证增强
- **优先级**: P1-High
- **文件**: `pkg/gateway/constants.go`, `pkg/gateway/gateway.go`
- **预估**: 1.5 小时
- **依赖**: Task 1.11
- **步骤**:
  1. 在 constants.go 定义限制常量
  2. 定义 validCommands 白名单 map
  3. 重构 dispatchCommand(): 添加长度检查 + 命令白名单
  4. 重构 handleSessionStart(): 格式化错误处理 + 上限降低
  5. 重构 handleData(): 添加数据长度检查
- **验收**: 测试超长消息、非法命令被正确拒绝

---

## 阶段三: P2 质量提升（预计 12-16 小时）

### Task 3.1: 扩展 httpclient 包
- **优先级**: P2-Medium
- **文件**: `pkg/channel/httpclient/httpclient.go`, `httpclient_test.go`
- **预估**: 2 小时
- **依赖**: Task 1.11
- **步骤**:
  1. 添加 `PostJSON()` 方法
  2. 添加 `WithAuth()` 方法
  3. 添加 `SetDefaultHeaders()` 方法
  4. 编写对应测试
- **验收**: httpclient 测试全通过

### Task 3.2: 迁移 wechat 到统一 HTTP 客户端
- **优先级**: P2-Medium
- **文件**: `pkg/channel/wechat/wechat.go`
- **预估**: 1 小时
- **依赖**: Task 3.1
- **步骤**:
  1. 替换 `*http.Client` 为 `*httpclient.Client`
  2. 删除自定义 sendAPIRequest 方法或简化为调用 httpclient
  3. 运行测试确保功能不变
- **验收**: wechat 测试通过 + 代码行数减少

### Task 3.3: 迁移其余 channel 到统一 HTTP 客户端
- **优先级**: P2-Medium
- **文件**: dingtalk, telegram, feishu, slack, discord, wecom
- **预估**: 4 小时
- **依赖**: Task 3.1
- **步骤**: 逐个迁移，每个约 40 分钟
- **验收**: 所有 channel 测试通过

### Task 3.4: Session 管理优化
- **优先级**: P2-Medium
- **文件**: `pkg/gateway/session.go`
- **预估**: 1.5 小时
- **依赖**: Task 1.11
- **步骤**:
  1. 添加 maxSessions 字段和 currentCount 原子计数器
  2. create() 中添加数量限制检查
  3. remove() 中递减计数器
  4. 清理间隔从 30s 改为 10s
  5. 添加相关测试
- **验收**: Session 数量超过上限时返回错误

### Task 3.5: 统一错误处理
- **优先级**: P2-Medium
- **文件**: `pkg/channel/channel.go`（扩展 errors）, 各 channel 文件
- **预估**: 3 小时
- **依赖**: Task 1.11
- **步骤**:
  1. 在 channel.go 中定义完整的 sentinel error 变量
  2. 逐个审查各 channel 的错误返回
  3. 统一替换为 sentinel error + fmt.Errorf %w 包装
  4. 更新测试中的错误断言
- **验收**: go vet 通过 + 错误模式一致

---

## 阶段四: P3 文档与收尾（预计 4-6 小时）

### Task 4.1: 更新 README.md
- **优先级**: P3-Low
- **文件**: `README.md`, `README_zh-CN.md`
- **预估**: 1 小时
- **步骤**:
  1. 修正不存在的包引用
  2. 更新项目结构图
  3. 更新测试覆盖率表格
  4. 更新代码示例

### Task 4.2: 扩展配置系统
- **优先级**: P3-Low
- **文件**: `pkg/config/config.go`
- **预估**: 2 小时
- **步骤**:
  1. 添加所有 channel 的配置结构体
  2. 添加 Validate() 方法覆盖新配置
  3. 添加 setDefaults() 补充

### Task 4.3: 最终全量验证
- **优先级**: P3-Low
- **预估**: 1 小时
- **依赖**: 所有前置任务
- **步骤**:
  1. `go build ./...`
  2. `go vet ./...`
  3. `golangci-lint run...`（如已配置）
  4. `go test ./... -count=1 -race -coverprofile=coverage.out`
  5. 检查覆盖率报告
- **验收标准**: 编译通过、测试全绿、核心模块 >= 85% 覆盖率

---

## 任务依赖关系图

```
Task 1.1 (Message 增强)
    ├── Task 1.2 (wechat)
    ├── Task 1.3 (dingtalk)
    ├── Task 1.4 (telegram)
    ├── Task 1.5 (feishu)
    ├── Task 1.6 (slack)
    ├── Task 1.7 (discord)
    ├── Task 1.8 (wecom)
    ├── Task 1.9 (whatsapp/messager/imessage)
    └── Task 1.10 (测试文件)
            └── Task 1.11 (全量编译验证)
                    ├── Task 2.1 (WS 安全)
                    ├── Task 2.4 (输入验证)
                    ├── Task 3.1 (httpclient 扩展) ──┬── Task 3.2 (wechat 迁移)
                    │                               ├── Task 3.3 (其余迁移)
                    ├── Task 3.4 (Session 优化)      │
                    └── Task 3.5 (统一错误)           │
                                                    │
Task 2.2 (wechat 凭证) ──────────────────────────────┤
Task 2.3 (凭证审查)   ──────────────────────────────┤
                                                    │
                                                    └── Task 4.3 (最终验证)
                                                          └── 完成 ✅
```
