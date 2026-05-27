// Package logging provides structured logging capabilities for the MindX framework.
// It uses GoReact's core.Logger interface to ensure compatibility across the entire stack.
//
// The API follows uber-go/zap's calling convention:
//
//	logger.Info("server started", "port", 8080, "host", "localhost")
//	logger.Warn("slow request", "duration", 2.5*time.Second)
//	logger.Error("connection failed", err, "addr", "127.0.0.1:3306")
//	logger.Debug("cache hit", "key", userID)
//
// Implementations:
//   - Console logger: Outputs to stdout with ANSI color support
//   - File logger: Writes to a file with configurable log level
//   - No-op logger: Discards all log output (for TUI/testing)
//   - Zap logger: High-performance logger with rotation (requires zap)
package logging

import (
	"fmt"
	"log"
	"os"

	goreactlogging "github.com/DotNetAge/goreact/logging"
)

// Logger is an alias for GoReact's core.Logger interface.
// This ensures type compatibility across MindX and GoReact.
type Logger = goreactlogging.Logger

// Level represents the severity level of a log message.
// Log levels are ordered from least to most severe: DEBUG < INFO < WARN < ERROR.
type Level int

// Log level constants define the severity of log messages.
const (
	DEBUG Level = iota
	INFO
	WARN
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

// ANSI color codes for terminal output.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
)

// levelColor returns the ANSI color code for the given log level.
func levelColor(level Level) string {
	switch level {
	case DEBUG:
		return colorCyan
	case INFO:
		return colorGreen
	case WARN:
		return colorYellow
	case ERROR:
		return colorRed
	default:
		return colorReset
	}
}

// defaultLogger is the standard implementation of Logger.
// It supports both console and file output with configurable log levels.
type defaultLogger struct {
	file     *os.File
	logger   *log.Logger
	level    Level
	colorize bool
}

// Option is a function that configures a defaultLogger.
type Option func(*defaultLogger)

// WithLevel returns an Option that sets the minimum log level.
func WithLevel(level Level) Option {
	return func(l *defaultLogger) {
		l.level = level
	}
}

// WithColor returns an Option that enables or disables ANSI color output.
func WithColor(enabled bool) Option {
	return func(l *defaultLogger) {
		l.colorize = enabled
	}
}

// DefaultConsoleLogger creates a logger that writes to stdout with INFO level and colored output.
func DefaultConsoleLogger() Logger {
	return &defaultLogger{
		file:     os.Stdout,
		logger:   log.New(os.Stdout, "", log.LstdFlags),
		level:    INFO,
		colorize: true,
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
		file:   file,
		logger: log.New(file, "", log.LstdFlags),
		level:  DEBUG,
	}

	for _, opt := range opts {
		opt(l)
	}

	return l, nil
}

// log writes a formatted log message if the level meets the threshold.
func (l *defaultLogger) log(level Level, msg string, keyvals []any) {
	if level < l.level {
		return
	}

	var fieldStr string
	for i := 0; i+1 < len(keyvals); i += 2 {
		fieldStr += fmt.Sprintf(" %s=%v", keyvals[i], keyvals[i+1])
	}

	if l.colorize {
		l.logger.Printf("%s[%s]%s %s%s", levelColor(level), level.String(), colorReset, msg, fieldStr)
	} else {
		l.logger.Printf("[%s] %s%s", level.String(), msg, fieldStr)
	}
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
