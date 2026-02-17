# 日志系统使用说明

## 概述

本系统使用 Zap 作为统一的日志框架，支持两种日志类型：

1. **系统日志** - 用于模块运行跟踪、错误排查、性能监控
2. **对话日志** - 用于记录用户对话，便于记忆系统后续处理

## 初始化

```go
import (
    "mindx/internal/config"
    "mindx/pkg/logging"
)

// 使用默认配置初始化
logConfig := &config.LoggingConfig{
    SystemLogConfig: &config.SystemLogConfig{
        Level:      config.LevelInfo,
        OutputPath: "logs/system.log",
        MaxSize:    100,
        MaxBackups: 10,
        MaxAge:     30,
        Compress:   true,
    },
    ConversationLogConfig: &config.ConversationLogConfig{
        Enable:     false,
        OutputPath: "logs/conversation.log",
    },
}

if err := logging.Init(logConfig); err != nil {
    panic(err)
}

// 获取日志器
systemLogger := logging.GetSystemLogger()
conversationLogger := logging.GetConversationLogger()
```

## 系统日志使用

```go
import "mindx/pkg/logging"

// 获取系统日志器
logger := logging.GetSystemLogger().Named("module_name")

// 记录不同级别的日志
logger.Debug("调试信息",
    logging.String("key", "value"),
    logging.Int("count", 10))

logger.Info("普通信息",
    logging.String("user_id", "123"))

logger.Warn("警告信息",
    logging.String("reason", "something went wrong"))

logger.Error("错误信息",
    logging.Err(err),
    logging.String("operation", "save"))

logger.Fatal("致命错误",
    logging.Err(err))
```

## 对话日志使用

```go
// 获取对话日志器
convLogger := logging.GetConversationLogger()

// 记录收到的消息
convLogger.Info("收到消息",
    logging.String("session_id", "session_123"),
    logging.String("user_id", "user_456"),
    logging.String("content", "用户的问题"),
    logging.String("direction", "incoming"))

// 记录发送的回复
convLogger.Info("发送回复",
    logging.String("session_id", "session_123"),
    logging.String("content", "机器人的回答"),
    logging.String("direction", "outgoing"))
```

## 日志字段类型

```go
// 字符串字段
logging.String("key", "value")

// 整数字段
logging.Int("count", 10)
logging.Int64("timestamp", 1234567890)

// 浮点数字段
logging.Float64("score", 0.95)

// 布尔字段
logging.Bool("success", true)

// 错误字段
logging.Err(err)

// 任意类型字段
logging.Any("data", someStruct)

// 时长字段
logging.Duration("latency", time.Since(start))
```

## 在 Channel 中使用

Channel 模块同时使用两种日志：

```go
type FeishuChannel struct {
    logger   logging.Logger   // 系统日志 - 记录运行状态、错误
    // ...
}

// 记录系统日志
c.logger.Info("Channel 已启动",
    logging.Int("port", 8080),
    logging.String("path", "/webhook"))

// ChannelRouter 中同时使用两种日志
type ChannelRouter struct {
    logger     logging.Logger  // 系统日志
    convLogger logging.Logger  // 对话日志
}

// 记录消息处理
r.logger.Debug("处理消息",
    logging.String("session_id", msg.SessionID))

r.convLogger.Info("收到消息",
    logging.String("session_id", msg.SessionID),
    logging.String("content", msg.Content),
    logging.String("direction", "incoming"))
```

## 配置定义

配置类统一放在 `internal/config` 包中：

```go
// LogLevel 日志级别
type LogLevel string

const (
    LevelDebug LogLevel = "debug"
    LevelInfo  LogLevel = "info"
    LevelWarn  LogLevel = "warn"
    LevelError LogLevel = "error"
    LevelFatal LogLevel = "fatal"
)

// LoggingConfig 日志配置
type LoggingConfig struct {
    SystemLogConfig       *SystemLogConfig       `json:"system_log"`
    ConversationLogConfig *ConversationLogConfig `json:"conversation_log"`
}

// SystemLogConfig 系统日志配置
type SystemLogConfig struct {
    Level      LogLevel `json:"level"`
    OutputPath string   `json:"output_path"`
    MaxSize    int      `json:"max_size"`
    MaxBackups int      `json:"max_backups"`
    MaxAge     int      `json:"max_age"`
    Compress   bool     `json:"compress"`
}

// ConversationLogConfig 对话日志配置
type ConversationLogConfig struct {
    Enable     bool   `json:"enable"`
    OutputPath string `json:"output_path"`
}
```

## 日志输出

### 系统日志示例

```json
{
  "time": "2024-02-07T10:30:45.123+08:00",
  "level": "INFO",
  "logger": "channel_manager",
  "msg": "Channel 注册成功",
  "name": "feishu",
  "type": "feishu"
}
```

### 对话日志示例

```json
{
  "timestamp": "2024-02-07T10:30:45.123+08:00",
  "message": "收到消息",
  "session_id": "session_123",
  "user_id": "user_456",
  "channel_id": "feishu",
  "direction": "incoming",
  "content": "你好"
}
```

## 最佳实践

1. **合理使用日志级别**
   - DEBUG: 详细的调试信息
   - INFO: 重要的业务流程
   - WARN: 潜在的问题
   - ERROR: 处理失败的错误
   - FATAL: 导致程序退出的错误

2. **结构化字段**
   - 使用有意义的字段名
   - 保持字段命名一致
   - 包含足够的上下文信息

3. **性能考虑**
   - 避免在高频日志中使用复杂的序列化
   - 生产环境建议使用 INFO 级别
   - 对话日志会持久化，注意存储空间

4. **安全考虑**
   - 避免记录敏感信息（密码、token）
   - 使用脱敏字段记录用户数据
