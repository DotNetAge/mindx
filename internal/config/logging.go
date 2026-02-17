package config

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
	// SystemLogConfig 系统日志配置
	SystemLogConfig *SystemLogConfig `json:"system_log" yaml:"system_log"`
	// ConversationLogConfig 对话日志配置
	ConversationLogConfig *ConversationLogConfig `json:"conversation_log" yaml:"conversation_log"`
}

// SystemLogConfig 系统日志配置
type SystemLogConfig struct {
	// Level 日志级别
	Level LogLevel `json:"level" yaml:"level"`
	// OutputPath 输出路径
	OutputPath string `json:"output_path" yaml:"output_path"`
	// MaxSize 单个文件最大大小 (MB)
	MaxSize int `json:"max_size" yaml:"max_size"`
	// MaxBackups 保留的最大历史文件数
	MaxBackups int `json:"max_backups" yaml:"max_backups"`
	// MaxAge 保留的最大天数
	MaxAge int `json:"max_age" yaml:"max_age"`
	// Compress 是否压缩
	Compress bool `json:"compress" yaml:"compress"`
}

// ConversationLogConfig 对话日志配置
type ConversationLogConfig struct {
	// Enable 是否启用数据库持久化
	Enable bool `json:"enable" yaml:"enable"`
	// OutputPath 输出路径 (备选文件存储)
	OutputPath string `json:"output_path" yaml:"output_path"`
}
