# Gort 重构验收清单 (Checklist)

> 每完成一个任务，勾选对应项。所有 P0/P1 项必须全部勾选才能进入下一阶段。

## 阶段一: P0 编译修复验收

### Task 1.1: Message 结构体增强
- [ ] `Message` 结构体包含 `ChannelID string` 字段
- [ ] `UserInfo` 结构体包含 `Platform string` 字段
- [ ] 定义了 `MessageTypeMarkdown` 常量
- [ ] 定义了 `MessageTypeNews` 常量
- [ ] 定义了 `MessageTypeVoice` 常量
- [ ] 定义了 `MessageTypeTemplateCard` 常量
- [ ] 实现了 `GetMetadata(key) (interface{}, bool)` 方法
- [ ] 实现了 `SetMetadata(key, value interface{})` 方法
- [ ] 实现了 `NewMessage()` 构造函数（签名含 channelID 参数）
- [ ] `go build ./pkg/channel` 通过

### Task 1.2: wechat channel
- [ ] 无编译错误
- [ ] `msg.ChannelID` 赋值正确
- [ ] `UserInfo.Platform = "wechat"` 已设置
- [ ] 所有 `SetMetadata()` / `GetMetadata()` 调用可解析
- [ ] Timestamp 字段赋值类型匹配

### Task 1.3: dingtalk channel
- [ ] 无编译错误
- [ ] ChannelID、Platform 字段已设置
- [ ] GetMetadata/SetMetadata 调用正确

### Task 1.4: telegram channel
- [ ] 无编译错误
- [ ] Timestamp 从 time.Time 正确转换为 string
- [ ] ChannelID、Platform 字段已设置

### Task 1.5: feishu channel
- [ ] 无编译错误
- [ ] Timestamp 类型转换正确
- [ ] GetMetadata/SetMetadata 调用正确

### Task 1.6: slack channel
- [ ] 无编译错误
- [ ] `NewMessage()` 调用使用新签名
- [ ] `MessageTypeMarkdown` 引用正确
- [ ] Platform 字段已设置

### Task 1.7: discord channel
- [ ] 无编译错误
- [ ] 同 slack 的所有检查项

### Task 1.8: wecom channel
- [ ] 无编译错误
- [ ] MessageTypeMarkdown/News/Voice/TemplateCard 引用正确
- [ ] GetMetadata/SetMetadata 调用正确

### Task 1.9: whatsapp / messenger / imessage
- [ ] whatsapp.go 编译通过
- [ ] messenger.go 编译通过
- [ ] imessage.go 编译通过（如有修改需要）

### Task 1.10: 测试文件
- [ ] channel_test.go 编译通过
- [ ] channel_bench_test.go 编译通过
- [ ] wechat_test.go 编译通过
- [ ] dingtalk_test.go 编译通过
- [ ] telegram_test.go 编译通过
- [ ] feishu_test.go 编译通过
- [ ] slack_test.go 编译通过
- [ ] discord_test.go 编译通过
- [ ] wecom_test.go 编译通过
- [ ] whatsapp_test.go 编译通过
- [ ] messenger_test.go 编译通过

### Task 1.11: 全量验证
- [ ] `go build ./...` 零错误零警告
- [ ] `go vet ./...` 通过
- [ ] `go test ./... -count=1` 全部测试通过
- [ ] **阶段一签收**: ___/___

---

## 阶段二: P1 安全加固验收

### Task 2.1: WebSocket 安全
- [ ] `WSConfig` 结构体定义完整
- [ ] `matchOrigin()` 支持通配符匹配
- [ ] 开发环境可通过环境变量放行
- [ ] 生产环境拒绝未知 Origin
- [ ] `WithWSConfig()` Option 可用
- [ ] 日志记录被拒绝的连接尝试
- [ ] 测试覆盖: 合法 Origin 通过、非法 Origin 拒绝

### Task 2.2: wechat 凭证安全
- [ ] `refreshToken()` 中 secret 不再出现在 URL 中
- [ ] 使用 POST Body 传递 appid + secret
- [ ] Content-Type 为 application/json
- [ ] 功能测试: token 刷新正常工作

### Task 2.3: 凭证全面审查
- [ ] dingtalk: URL 中无敏感信息
- [ ] telegram: Token 仅在 Authorization header 中
- [ ] feishu: Secret 在 POST Body 中
- [ ] slack: BotToken 仅在 Authorization header 中
- [ ] discord: BotToken 仅在 Authorization header 中
- [ ] wecom: Key 不在日志中明文输出
- [ ] grep 确认: 无 `secret=` 或 `token=` 出现在 URL 构建中（排除 header 设置）

### Task 2.4: 输入验证
- [ ] `MaxMessageSize` 常量定义 (1MB)
- [ ] `MaxSessionParts` 常量定义 (1000)
- [ ] `MaxSessionTotal` 常量定义 (1000)
- [ ] `validCommands` 白名单 map 已定义
- [ ] `dispatchCommand()`: 超长消息被拒绝并记录日志
- [ ] `dispatchCommand()`: 未知命令被拒绝
- [ ] `handleSessionStart()`: 格式化错误有处理
- [ ] `handleSessionStart()`: total 上限为 MaxSessionTotal
- [ ] `handleData()`: 数据长度有检查
- [ ] 测试覆盖: 边界值测试通过
- [ ] **阶段二签收**: ___/___

---

## 阶段三: P2 质量提升验收

### Task 3.1: httpclient 扩展
- [ ] `PostJSON(ctx, endpoint, payload, result)` 方法实现
- [ ] `WithAuth(tokenType, token)` 方法实现
- [ ] `SetDefaultHeaders(headers)` 方法实现
- [ ] PostJSON 测试: 成功响应解析正确
- [ ] PostJSON 测试: 错误响应返回 error
- [ ] PostJSON 测试: JSON marshal 失败返回 error
- [ ] WithAuth 测试: Authorization header 正确设置

### Task 3.2: wechat HTTP 迁移
- [ ] 使用 `*httpclient.Client` 替代 `*http.Client`
- [ ] `sendAPIRequest()` 使用 `PostJSON()` 简化
- [ ] 原有功能测试全部通过
- [ ] 代码行数减少（删除重复的 HTTP 逻辑）

### Task 3.3: 其余 channel HTTP 迁移
- [ ] dingtalk: 迁移完成，测试通过
- [ ] telegram: 迁移完成，测试通过
- [ ] feishu: 迁移完成，测试通过
- [ ] slack: 迁移完成，测试通过
- [ ] discord: 迁移完成，测试通过
- [ ] wecom: 迁移完成，测试通过

### Task 3.4: Session 管理
- [ ] `SessionManager.maxSessions` 字段存在
- [ ] `SessionManager.currentCount` 原子计数器存在
- [ ] `create()` 超过限制时返回明确错误
- [ ] `remove()` 正确递减计数器
- [ ] 清理间隔改为 10 秒
- [ ] 测试: 达到上限后新 session 被拒绝
- [ ] 测试: 删除 session 后可以创建新的

### Task 3.5: 错误处理统一
- [ ] sentinel errors 定义在 channel.go 中
- [ ] 包含通用错误集 (InvalidArgument, NotFound 等)
- [ ] 包含 Channel 专用错误集
- [ ] 包含验证专用错误集
- [ ] wechat 错误返回统一使用 sentinel errors
- [ ] dingtalk 错误返回统一
- [ ] telegram 错误返回统一
- [ ] feishu 错误返回统一
- [ ] slack 错误返回统一
- [ ] discord 错误返回统一
- [ ] wecom 错误返回统一
- [ ] gateway 错误返回统一
- [ ] `errors.Is()` 可用于错误类型判断
- [ ] **阶段三签收**: ___/___

---

## 阶段四: P3 文档与收尾验收

### Task 4.1: 文档更新
- [ ] README.md 中无引用不存在的包 (`pkg/message`, `pkg/session`)
- [ ] 项目结构图反映实际目录结构
- [ ] 测试覆盖率表格数据准确
- [ ] 代码示例可编译运行
- [ ] README_zh-CN.md 同步更新

### Task 4.2: 配置系统扩展
- [ ] Telegram 配置结构体存在
- [ ] Slack 配置结构体存在
- [ ] Discord 配置结构体存在
- [ ] WhatsApp 配置结构体存在
- [ ] Messenger 配置结构体存在
- [ ] WeCom 配置结构体存在
- [ ] iMessage 配置结构体存在（如适用）
- [ ] 新配置 Validate() 实现
- [ ] setDefaults() 包含新配置默认值

### Task 4.3: 最终验证
- [ ] `go build ./...` ✅
- [ ] `go vet ./...` ✅
- [ ] `go test ./... -count=1 -race` 全部通过 ✅
- [ ] 核心模块覆盖率 >= 85%:
  - [ ] pkg/channel >= 85%
  - [ ] pkg/config >= 85%
  - [ ] pkg/gateway >= 85%
  - [ ] pkg/channel/httpclient >= 80%
  - [ ] pkg/channel/tokenmanager >= 80%
- [ ] **最终签收**: ___/___

---

## 总体验收签字

| 角色 | 姓名 | 日期 | 签字 |
|------|------|------|------|
| 审计执行 | | | |
| 代码审查 | | | |
| 安全审查 | | | |
| 测试验证 | | | |
| 项目负责人 | | | |
