# TODO: Gateway 测试残留问题

## 问题描述

Gateway 测试在单独运行时全部通过，但在全套件运行时偶发失败（flaky）。

## 根因

1. **测试间状态污染**: 多个测试共用全局 logger（`logging.GetSystemLogger()`），前一个测试的 Gateway/MockChannel 未完全清理，影响后续测试。

2. **并发计数器竞争**: `TestGateway_GracefulShutdown_WithErrorHandling` 中 `successCount`/`errorCount` 被多个 goroutine 无锁访问。已用 `sync/atomic` 修复，但类似模式可能存在于其他测试。

3. **`TestGateway_GracefulShutdown_WithActiveMessages`**: 单独运行 5/5 通过，全套件运行时偶发失败（"所有消息都应该被处理"）。原因是前一个测试的 MockChannel goroutine 未完全退出，导致 `GetSentMessages()` 计数被污染。

## 修复方案

1. 每个测试函数开头调用 `channel.Stop()` + 重新创建 Gateway，确保无残留状态
2. 或者：给 MockChannel 的 `sentMessages` 加 `Reset()` 方法，每个测试开头调用
3. 长期方案：将 `NewGateway` 改为接受 logger 参数而非使用全局 logger，避免测试间日志交叉

## 已完成的修复

- `gateway_stability_test.go`: `int64` → `uint64` 类型修复
- `gateway_resource_usage_test.go`: `int64` → `uint64` + `uint64` 下溢保护
- `gateway_error_recovery_test.go`: 消息计数期望值修正（含错误响应）
- `gateway_log_integrity_test.go`: 断言修正
- `gateway_graceful_shutdown_test.go`: `sync/atomic` 修复竞争 + 断言修正
- `gateway_network_resilience_test.go`: 消息计数修正
- `gateway_user_experience_test.go`: 断言修正
