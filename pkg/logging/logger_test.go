package logging

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{Level(100), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.String())
		})
	}
}

func TestWithLevel(t *testing.T) {
	logger := &defaultLogger{level: INFO}
	opt := WithLevel(DEBUG)
	opt(logger)
	assert.Equal(t, DEBUG, logger.level)
}

func TestDefaultConsoleLogger(t *testing.T) {
	logger := DefaultConsoleLogger()
	assert.NotNil(t, logger)
	assert.IsType(t, &defaultLogger{}, logger)
}

func TestDefaultConsoleLogger_ImplementsInterface(t *testing.T) {
	logger := DefaultConsoleLogger()
	_, ok := logger.(Logger)
	assert.True(t, ok)
}

func TestDefaultFileLogger_Success(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, err := DefaultFileLogger(filePath)

	assert.NoError(t, err)
	assert.NotNil(t, fileLogger)

	if dl, ok := fileLogger.(*defaultLogger); ok {
		err = dl.Close()
		assert.NoError(t, err)
	}
}

func TestDefaultFileLogger_InvalidPath(t *testing.T) {
	logger, err := DefaultFileLogger("/nonexistent/directory/test.log")
	assert.Error(t, err)
	assert.Nil(t, logger)
}

func TestDefaultNoopLogger(t *testing.T) {
	logger := DefaultNoopLogger()
	assert.NotNil(t, logger)
	_, ok := logger.(Logger)
	assert.True(t, ok)
}

func TestNoopLogger_AllMethodsNoOp(t *testing.T) {
	logger := &noopLogger{}
	logger.Info("info message", "key", "value")
	logger.Debug("debug message", "key", "value")
	logger.Warn("warn message", "key", "value")
	logger.Error("error message", nil, "key", "value")
}

func TestDefaultLogger_Info(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	fileLogger.Info("test info", "key", "value")
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "[INFO]")
	assert.Contains(t, string(content), "test info")
	assert.Contains(t, string(content), "key=value")
}

func TestDefaultLogger_Error(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	fileLogger.Error("test error", nil, "key", "value")
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "[ERROR]")
	assert.Contains(t, string(content), "test error")
}

func TestDefaultLogger_Debug(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	fileLogger.Debug("test debug", "key", "value")
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "[DEBUG]")
	assert.Contains(t, string(content), "test debug")
	assert.Contains(t, string(content), "key=value")
}

func TestDefaultLogger_Warn(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	fileLogger.Warn("test warn", "key", "value")
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "[WARN]")
	assert.Contains(t, string(content), "test warn")
	assert.Contains(t, string(content), "key=value")
}

func TestDefaultLogger_LevelFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(WARN))
	fileLogger.Debug("debug should not appear")
	fileLogger.Info("info should not appear")
	fileLogger.Error("error should appear", nil)
	fileLogger.Warn("warn should appear")
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.NotContains(t, string(content), "debug should not appear")
	assert.NotContains(t, string(content), "info should not appear")
	assert.Contains(t, string(content), "error should appear")
	assert.Contains(t, string(content), "warn should appear")
}

func TestDefaultLogger_ErrorWithNilError(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	fileLogger.Error("test error", nil, "key", "value")
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "[ERROR]")
	assert.Contains(t, string(content), "test error")
}

func TestDefaultLogger_ErrorWithRealError(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	testErr := os.ErrPermission
	fileLogger.Error("test error", testErr, "key", "value")
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "[ERROR]")
	assert.Contains(t, string(content), "test error")
	assert.Contains(t, string(content), testErr.Error())
}

func TestDefaultLogger_KeyvalsFormat(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	fileLogger.Info("test message", "string", "value", "int", 42, "bool", true)
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "string=value")
	assert.Contains(t, string(content), "int=42")
	assert.Contains(t, string(content), "bool=true")
}

func TestDefaultLogger_NoKeyvals(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	fileLogger.Info("test message")
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "test message")
}

func TestDefaultLogger_OddKeyvals(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	fileLogger.Info("test message", "orphan_key")
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "test message")
	// Odd keyval: last key without a value is silently dropped (same as zap)
	assert.NotContains(t, string(content), "orphan_key=")
}

func TestDefaultLogger_Close(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, err := DefaultFileLogger(filePath)
	assert.NoError(t, err)

	if dl, ok := fileLogger.(*defaultLogger); ok {
		err = dl.Close()
		assert.NoError(t, err)
	}
}

func TestDefaultLogger_ErrorWithErrorAndKeyvals(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")
	fileLogger, _ := DefaultFileLogger(filePath, WithLevel(DEBUG))
	testErr := errors.New("disk full")
	fileLogger.Error("write failed", testErr, "file", "data.bin", "size", 1024)
	if dl, ok := fileLogger.(*defaultLogger); ok {
		dl.Close()
	}

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "[ERROR]")
	assert.Contains(t, string(content), "write failed")
	assert.Contains(t, string(content), "error=disk full")
	assert.Contains(t, string(content), "file=data.bin")
	assert.Contains(t, string(content), "size=1024")
}
