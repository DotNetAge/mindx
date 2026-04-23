// Package logging provides structured logging capabilities for the goRAG framework.
// It offers a simple, flexible logging interface with support for multiple log levels,
// file and console output, and structured field logging.
//
// The API is designed to mirror uber-go/zap's calling convention:
//
//	logger.Info("server started", "port", 8080, "host", "localhost")
//	logger.Warn("slow request", "duration", 2.5*time.Second)
//	logger.Error("connection failed", err, "addr", "127.0.0.1:3306")
//	logger.Debug("cache hit", "key", userID)
//
// The package provides three main implementations:
//   - Console logger: Outputs to stdout with minimal formatting
//   - File logger: Writes to a file with configurable log level
//   - No-op logger: Discards all log output (useful for testing)
//   - Zap logger: High-performance logger with log rotation (requires zap dependency)
package logging

import (
	"fmt"
	"log"
	"os"
)

// Level represents the severity level of a log message.
// Log levels are ordered from least to most severe: DEBUG < INFO < WARN < ERROR.
type Level int

// Log level constants define the severity of log messages.
// Messages with a level below the configured threshold will not be logged.
const (
	// DEBUG level is for detailed debugging information.
	DEBUG Level = iota

	// INFO level is for general operational information.
	INFO

	// WARN level is for warning messages that indicate potential issues.
	WARN

	// ERROR level is for error messages indicating failures.
	ERROR
)

// String returns the string representation of the log level.
func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger defines the interface for structured logging.
// All methods accept optional key-value pairs (alternating string keys and any values),
// following the same convention as uber-go/zap.
//
// Example:
//
//	logger.Info("user logged in", "user_id", 123, "ip", "192.168.1.1")
//	logger.Error("database error", err, "query", sql)
//	logger.Warn("rate limit approaching", "remaining", 5)
//	logger.Debug("processing chunk", "chunkID", chunk.ID)
type Logger interface {
	// Info logs an informational message with optional key-value pairs.
	Info(msg string, keyvals ...any)

	// Error logs an error message. The error is automatically included in the output.
	// Additional key-value pairs can be provided after the error.
	Error(msg string, err error, keyvals ...any)

	// Debug logs a debug message with optional key-value pairs.
	Debug(msg string, keyvals ...any)

	// Warn logs a warning message with optional key-value pairs.
	Warn(msg string, keyvals ...any)
}

// defaultLogger is the standard implementation of Logger.
// It supports both console and file output with configurable log levels.
type defaultLogger struct {
	filePath string
	file     *os.File
	logger   *log.Logger
	level    Level
}

// Option is a function that configures a defaultLogger.
type Option func(*defaultLogger)

// WithLevel returns an Option that sets the minimum log level.
func WithLevel(level Level) Option {
	return func(l *defaultLogger) {
		l.level = level
	}
}

// DefaultConsoleLogger creates a logger that writes to stdout with INFO level.
func DefaultConsoleLogger() Logger {
	return &defaultLogger{
		file:   os.Stdout,
		logger: log.New(os.Stdout, "", 0),
		level:  INFO,
	}
}

// DefaultFileLogger creates a logger that writes to a file.
// The file is created if it doesn't exist, and appended to if it does.
func DefaultFileLogger(filePath string, opts ...Option) (Logger, error) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	l := &defaultLogger{
		filePath: filePath,
		file:     file,
		logger:   log.New(file, "", 0),
		level:    INFO,
	}

	for _, opt := range opts {
		opt(l)
	}

	return l, nil
}

// log writes a formatted log message if the level meets the threshold.
// keyvals are alternating key-value pairs: "key1", val1, "key2", val2, ...
func (l *defaultLogger) log(level Level, msg string, keyvals []any) {
	if level < l.level {
		return
	}

	var fieldStr string
	for i := 0; i+1 < len(keyvals); i += 2 {
		fieldStr += fmt.Sprintf(" %s=%v", keyvals[i], keyvals[i+1])
	}

	l.logger.Printf("[%s] %s%s", level.String(), msg, fieldStr)
}

func (l *defaultLogger) Info(msg string, keyvals ...any) {
	l.log(INFO, msg, keyvals)
}

func (l *defaultLogger) Error(msg string, err error, keyvals ...any) {
	kvs := make([]any, 0, 2+len(keyvals))
	if err != nil {
		kvs = append(kvs, "error", err.Error())
	}
	kvs = append(kvs, keyvals...)
	l.log(ERROR, msg, kvs)
}

func (l *defaultLogger) Debug(msg string, keyvals ...any) {
	l.log(DEBUG, msg, keyvals)
}

func (l *defaultLogger) Warn(msg string, keyvals ...any) {
	l.log(WARN, msg, keyvals)
}

// Close closes the underlying file if this is a file logger.
func (l *defaultLogger) Close() error {
	return l.file.Close()
}

// noopLogger is a no-op implementation that discards all log messages.
type noopLogger struct{}

// DefaultNoopLogger creates a logger that discards all output.
func DefaultNoopLogger() Logger {
	return &noopLogger{}
}

func (l *noopLogger) Info(string, ...any)         {}
func (l *noopLogger) Error(string, error, ...any) {}
func (l *noopLogger) Debug(string, ...any)        {}
func (l *noopLogger) Warn(string, ...any)         {}
