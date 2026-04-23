package logging

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// zapLogger is an implementation of Logger that uses uber-go/zap for high-performance logging.
type zapLogger struct {
	logger *zap.Logger
}

// ZapConfig defines the options for the Zap rolling logger
type ZapConfig struct {
	// Filename is the file to write logs to.
	Filename string
	// MaxSize is the maximum size in megabytes of the log file before it gets rotated.
	MaxSize int
	// MaxBackups is the maximum number of old log files to retain.
	MaxBackups int
	// MaxAge is the maximum number of days to retain old log files.
	MaxAge int
	// Compress determines if the rotated log files should be compressed using gzip.
	Compress bool
	// Console specifies if logs should also be printed to standard output.
	Console bool
}

// DefaultZapLogger creates a high-performance logger using uber-go/zap with lumberjack for log rotation.
func DefaultZapLogger(cfg ZapConfig) Logger {
	if cfg.Filename == "" {
		cfg.Filename = "logs/gorag.log"
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 100
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = 30
	}
	if cfg.MaxBackups == 0 {
		cfg.MaxBackups = 7
	}

	lumberJackLogger := &lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	fileWriter := zapcore.AddSync(lumberJackLogger)
	fileCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		fileWriter,
		zap.DebugLevel,
	)

	cores := []zapcore.Core{fileCore}

	if cfg.Console {
		consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
		consoleCore := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zap.DebugLevel)
		cores = append(cores, consoleCore)
	}

	core := zapcore.NewTee(cores...)

	return &zapLogger{
		logger: zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)),
	}
}

func (l *zapLogger) Info(msg string, keyvals ...any) {
	l.logger.Info(msg, toZapFields(keyvals)...)
}

func (l *zapLogger) Error(msg string, err error, keyvals ...any) {
	zFields := toZapFields(keyvals)
	if err != nil {
		zFields = append(zFields, zap.Error(err))
	}
	l.logger.Error(msg, zFields...)
}

func (l *zapLogger) Debug(msg string, keyvals ...any) {
	l.logger.Debug(msg, toZapFields(keyvals)...)
}

func (l *zapLogger) Warn(msg string, keyvals ...any) {
	l.logger.Warn(msg, toZapFields(keyvals)...)
}

// toZapFields converts alternating key-value pairs to zap.Field slice.
// keyvals: "key1", val1, "key2", val2, ...
func toZapFields(keyvals []any) []zap.Field {
	if len(keyvals) == 0 {
		return nil
	}

	fields := make([]zap.Field, 0, len(keyvals)/2)
	for i := 0; i+1 < len(keyvals); i += 2 {
		key, ok := keyvals[i].(string)
		if !ok {
			key = fmt.Sprintf("%v", keyvals[i])
		}
		fields = append(fields, zap.Any(key, keyvals[i+1]))
	}
	return fields
}
