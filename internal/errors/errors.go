package errors

import (
	"errors"
	"fmt"
	"runtime"
)

// ErrorType 错误类型
type ErrorType int

const (
	// ErrTypeUnknown 未知错误
	ErrTypeUnknown ErrorType = iota
	// ErrTypeConfig 配置错误
	ErrTypeConfig
	// ErrTypeNetwork 网络错误
	ErrTypeNetwork
	// ErrTypeStorage 存储错误
	ErrTypeStorage
	// ErrTypeModel 模型错误
	ErrTypeModel
	// ErrTypeSkill 技能错误
	ErrTypeSkill
	// ErrTypeMemory 记忆错误
	ErrTypeMemory
	// ErrTypeSession 会话错误
	ErrTypeSession
	// ErrTypeWebSocket WebSocket错误
	ErrTypeWebSocket
	// ErrTypeChannel 渠道错误
	ErrTypeChannel
)

// AppError 应用错误
type AppError struct {
	Type       ErrorType
	Message    string
	Err        error
	Caller     string
	StackTrace string
}

// Error 实现error接口
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type.String(), e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Type.String(), e.Message)
}

// Unwrap 返回原始错误
func (e *AppError) Unwrap() error {
	return e.Err
}

// String 返回类型的字符串表示
func (t ErrorType) String() string {
	switch t {
	case ErrTypeConfig:
		return "CONFIG"
	case ErrTypeNetwork:
		return "NETWORK"
	case ErrTypeStorage:
		return "STORAGE"
	case ErrTypeModel:
		return "MODEL"
	case ErrTypeSkill:
		return "SKILL"
	case ErrTypeMemory:
		return "MEMORY"
	case ErrTypeSession:
		return "SESSION"
	case ErrTypeWebSocket:
		return "WEBSOCKET"
	case ErrTypeChannel:
		return "CHANNEL"
	default:
		return "UNKNOWN"
	}
}

// New 创建新的应用错误
func New(errorType ErrorType, message string) *AppError {
	return &AppError{
		Type:    errorType,
		Message: message,
		Err:     nil,
		Caller:  getCaller(2),
	}
}

// Wrap 包装已有错误
func Wrap(err error, errorType ErrorType, message string) *AppError {
	if err == nil {
		return New(errorType, message)
	}

	return &AppError{
		Type:    errorType,
		Message: message,
		Err:     err,
		Caller:  getCaller(2),
	}
}

// Wrapf 包装已有错误并格式化消息
func Wrapf(err error, errorType ErrorType, format string, args ...interface{}) *AppError {
	if err == nil {
		return New(errorType, fmt.Sprintf(format, args...))
	}

	return &AppError{
		Type:    errorType,
		Message: fmt.Sprintf(format, args...),
		Err:     err,
		Caller:  getCaller(2),
	}
}

// getCaller 获取调用者信息
func getCaller(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown"
	}
	return fmt.Sprintf("%s:%d", file, line)
}

// IsAppError 检查是否是AppError
func IsAppError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr)
}

// GetAppError 从error中提取AppError
func GetAppError(err error) (*AppError, bool) {
	var appErr *AppError
	ok := errors.As(err, &appErr)
	return appErr, ok
}

// 预定义错误构造函数

// ConfigError 创建配置错误
func ConfigError(message string) *AppError {
	return New(ErrTypeConfig, message)
}

// WrapConfigError 包装配置错误
func WrapConfigError(err error, message string) *AppError {
	return Wrap(err, ErrTypeConfig, message)
}

// NetworkError 创建网络错误
func NetworkError(message string) *AppError {
	return New(ErrTypeNetwork, message)
}

// WrapNetworkError 包装网络错误
func WrapNetworkError(err error, message string) *AppError {
	return Wrap(err, ErrTypeNetwork, message)
}

// StorageError 创建存储错误
func StorageError(message string) *AppError {
	return New(ErrTypeStorage, message)
}

// WrapStorageError 包装存储错误
func WrapStorageError(err error, message string) *AppError {
	return Wrap(err, ErrTypeStorage, message)
}

// ModelError 创建模型错误
func ModelError(message string) *AppError {
	return New(ErrTypeModel, message)
}

// WrapModelError 包装模型错误
func WrapModelError(err error, message string) *AppError {
	return Wrap(err, ErrTypeModel, message)
}

// SkillError 创建技能错误
func SkillError(message string) *AppError {
	return New(ErrTypeSkill, message)
}

// WrapSkillError 包装技能错误
func WrapSkillError(err error, message string) *AppError {
	return Wrap(err, ErrTypeSkill, message)
}

// MemoryError 创建记忆错误
func MemoryError(message string) *AppError {
	return New(ErrTypeMemory, message)
}

// WrapMemoryError 包装记忆错误
func WrapMemoryError(err error, message string) *AppError {
	return Wrap(err, ErrTypeMemory, message)
}

// SessionError 创建会话错误
func SessionError(message string) *AppError {
	return New(ErrTypeSession, message)
}

// WrapSessionError 包装会话错误
func WrapSessionError(err error, message string) *AppError {
	return Wrap(err, ErrTypeSession, message)
}

// WebSocketError 创建WebSocket错误
func WebSocketError(message string) *AppError {
	return New(ErrTypeWebSocket, message)
}

// WrapWebSocketError 包装WebSocket错误
func WrapWebSocketError(err error, message string) *AppError {
	return Wrap(err, ErrTypeWebSocket, message)
}

// ChannelError 创建渠道错误
func ChannelError(message string) *AppError {
	return New(ErrTypeChannel, message)
}

// WrapChannelError 包装渠道错误
func WrapChannelError(err error, message string) *AppError {
	return Wrap(err, ErrTypeChannel, message)
}
