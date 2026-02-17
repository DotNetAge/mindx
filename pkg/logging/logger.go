package logging

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"mindx/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogType 日志类型
type LogType string

const (
	LogTypeSystem       LogType = "system"       // 系统日志
	LogTypeConversation LogType = "conversation" // 对话日志
)

// Logger 日志接口
type Logger interface {
	// Debug 记录 DEBUG 级别日志
	Debug(msg string, fields ...Field)
	// Info 记录 INFO 级别日志
	Info(msg string, fields ...Field)
	// Warn 记录 WARN 级别日志
	Warn(msg string, fields ...Field)
	// Error 记录 ERROR 级别日志
	Error(msg string, fields ...Field)
	// Fatal 记录 FATAL 级别日志
	Fatal(msg string, fields ...Field)

	// WithContext 添加上下文
	WithContext(ctx context.Context) Logger
	// With 添加字段
	With(fields ...Field) Logger
	// Named 创建命名子日志器
	Named(name string) Logger
}

// Field 日志字段
type Field struct {
	Key   string
	Value interface{}
}

// zapLogger Zap 日志实现
type zapLogger struct {
	logger  *zap.Logger
	logType LogType
	fields  []Field
}

// LogManager 日志管理器
type LogManager struct {
	systemLogger       Logger
	conversationLogger Logger
	config             *config.LoggingConfig
}

var (
	instance *LogManager
	once     sync.Once
)

// Init 初始化日志管理器
func Init(cfg *config.LoggingConfig) error {
	var initErr error
	once.Do(func() {
		systemLogger, err := createSystemLogger(cfg.SystemLogConfig)
		if err != nil {
			initErr = fmt.Errorf("创建系统日志失败: %w", err)
			return
		}

		conversationLogger, err := createConversationLogger(cfg.ConversationLogConfig)
		if err != nil {
			initErr = fmt.Errorf("创建对话日志失败: %w", err)
			return
		}

		instance = &LogManager{
			systemLogger:       systemLogger,
			conversationLogger: conversationLogger,
			config:             cfg,
		}
	})

	return initErr
}

// GetSystemLogger 获取系统日志
func GetSystemLogger() Logger {
	if instance == nil {
		return defaultLogger()
	}
	return instance.systemLogger
}

// GetConversationLogger 获取对话日志
func GetConversationLogger() Logger {
	if instance == nil {
		return defaultLogger()
	}
	return instance.conversationLogger
}

// createSystemLogger 创建系统日志器
func createSystemLogger(cfg *config.SystemLogConfig) (Logger, error) {
	if cfg == nil {
		cfg = defaultSystemLogConfig()
	}

	// 设置日志级别
	level := parseLevel(string(cfg.Level))

	// 配置文件轮转
	outputPath := cfg.OutputPath
	if outputPath == "" {
		outputPath = "logs/system.log"
	}

	// 确保目录存在
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	fileWriter := &lumberjack.Logger{
		Filename:   outputPath,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}

	// 编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 文件输出 Core
	fileCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(fileWriter),
		level,
	)

	// 控制台输出 Core（使用更友好的格式）
	consoleEncoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	consoleCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(consoleEncoderConfig),
		zapcore.AddSync(os.Stdout),
		level,
	)

	// 组合文件和控制台输出
	core := zapcore.NewTee(fileCore, consoleCore)

	loggerImpl := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &zapLogger{
		logger:  loggerImpl,
		logType: LogTypeSystem,
	}, nil
}

// createConversationLogger 创建对话日志器
func createConversationLogger(cfg *config.ConversationLogConfig) (Logger, error) {
	if cfg == nil {
		cfg = defaultConversationLogConfig()
	}

	// 配置编码器
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:    "timestamp",
		MessageKey: "message",
		LevelKey:   zapcore.OmitKey,
	}

	var writer zapcore.WriteSyncer

	if cfg.Enable {
		// TODO: 实现数据库 Writer
		// 这里暂时使用文件 Writer
		outputPath := cfg.OutputPath
		if outputPath == "" {
			outputPath = "logs/conversation.log"
		}

		dir := filepath.Dir(outputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建日志目录失败: %w", err)
		}

		file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("打开对话日志文件失败: %w", err)
		}
		writer = zapcore.AddSync(file)
	} else {
		writer = zapcore.AddSync(os.Stdout)
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		writer,
		zapcore.InfoLevel,
	)

	loggerImpl := zap.New(core, zap.AddCallerSkip(1))

	return &zapLogger{
		logger:  loggerImpl,
		logType: LogTypeConversation,
	}, nil
}

// parseLevel 解析日志级别
func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// defaultSystemLogConfig 默认系统日志配置
func defaultSystemLogConfig() *config.SystemLogConfig {
	return &config.SystemLogConfig{
		Level:      config.LevelInfo,
		OutputPath: "logs/system.log",
		MaxSize:    100,
		MaxBackups: 10,
		MaxAge:     30,
		Compress:   true,
	}
}

// defaultConversationLogConfig 默认对话日志配置
func defaultConversationLogConfig() *config.ConversationLogConfig {
	return &config.ConversationLogConfig{
		Enable:     false,
		OutputPath: "logs/conversation.log",
	}
}

// defaultLogger 默认日志器 (未初始化时使用)
func defaultLogger() Logger {
	loggerImpl := zap.NewNop()
	return &zapLogger{
		logger:  loggerImpl,
		logType: LogTypeSystem,
	}
}

// String 返回字段字符串
func (f Field) String() string {
	return fmt.Sprintf("%s=%v", f.Key, f.Value)
}

// Debug 记录 DEBUG 级别日志
func (l *zapLogger) Debug(msg string, fields ...Field) {
	zapFields := l.toZapFields(fields)
	l.logger.Debug(msg, zapFields...)
}

// Info 记录 INFO 级别日志
func (l *zapLogger) Info(msg string, fields ...Field) {
	zapFields := l.toZapFields(fields)
	l.logger.Info(msg, zapFields...)
}

// Warn 记录 WARN 级别日志
func (l *zapLogger) Warn(msg string, fields ...Field) {
	zapFields := l.toZapFields(fields)
	l.logger.Warn(msg, zapFields...)
}

// Error 记录 ERROR 级别日志
func (l *zapLogger) Error(msg string, fields ...Field) {
	zapFields := l.toZapFields(fields)
	l.logger.Error(msg, zapFields...)
}

// Fatal 记录 FATAL 级别日志
func (l *zapLogger) Fatal(msg string, fields ...Field) {
	zapFields := l.toZapFields(fields)
	l.logger.Fatal(msg, zapFields...)
}

// WithContext 添加上下文
func (l *zapLogger) WithContext(ctx context.Context) Logger {
	return l.With(
		Field{Key: "trace_id", Value: ctx.Value("trace_id")},
	)
}

// With 添加字段
func (l *zapLogger) With(fields ...Field) Logger {
	allFields := make([]Field, 0, len(l.fields)+len(fields))
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)
	return &zapLogger{
		logger:  l.logger.With(l.toZapFields(fields)...),
		logType: l.logType,
		fields:  allFields,
	}
}

// Named 创建命名子日志器
func (l *zapLogger) Named(name string) Logger {
	return &zapLogger{
		logger:  l.logger.Named(name),
		logType: l.logType,
		fields:  l.fields,
	}
}

// toZapFields 转换为 Zap 字段
func (l *zapLogger) toZapFields(fields []Field) []zap.Field {
	zapFields := make([]zap.Field, 0, len(fields))
	for _, f := range fields {
		zapFields = append(zapFields, zap.Any(f.Key, f.Value))
	}
	return zapFields
}

// String 创建字符串字段
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int 创建整数字段
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 创建 int64 字段
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Float64 创建 float64 字段
func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

// Bool 创建布尔字段
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Any 创建任意类型字段
func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// Err 创建错误字段
func Err(err error) Field {
	return Field{Key: "error", Value: err}
}

// Duration 创建时长字段
func Duration(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}
