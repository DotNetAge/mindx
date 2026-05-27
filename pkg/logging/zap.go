package logging

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// ResolveLogDir returns the absolute path to the MindX log directory.
//   - macOS/Linux: ~/.mindx/logs
//   - Windows:     %APPDATA%\mindx\logs
func ResolveLogDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "logs"
	}

	var base string
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			base = filepath.Join(appData, "mindx")
		} else {
			base = filepath.Join(home, ".mindx")
		}
	} else {
		base = filepath.Join(home, ".mindx")
	}

	logDir := filepath.Join(base, "logs")

	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: failed to create log directory %s: %v\n", logDir, err)
	}

	return logDir
}

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
func DefaultZapLogger(cfg *ZapConfig) Logger {
	if cfg.Filename == "" {
		cfg.Filename = "logs/mindx.log"
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

	logDir := filepath.Dir(cfg.Filename)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: failed to create log directory %s: %v\n", logDir, err)
	}

	fixLogFilePermissions(cfg.Filename)

	fileWriter := createSafeFileWriter(cfg)

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

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

// fixLogFilePermissions ensures the log file has correct permissions and no macOS
// quarantine attributes that would prevent lumberjack from rotating (renaming) it.
// This must be called before creating the lumberjack logger.
func fixLogFilePermissions(filename string) {
	dir := filepath.Dir(filename)

	info, err := os.Stat(dir)
	if err != nil {
		return
	}

	if info.Mode().Perm() != 0755 {
		os.Chmod(dir, 0755)
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		f, createErr := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if createErr == nil {
			f.Close()
		}
		return
	}

	os.Chmod(filename, 0644)

	removeMacOSQuarantine(filename)

	cleanOldRotatedFiles(dir, filepath.Base(filename))
}

func removeMacOSQuarantine(path string) {
	if runtime.GOOS != "darwin" {
		return
	}

	buildInfo, ok := debug.ReadBuildInfo()
	if ok && buildInfo.Main.Path != "" {
		if strings.Contains(buildInfo.Main.Path, "/tmp/go-build") {
			return
		}
	}

	out, err := exec.Command("xattr", "-l", path).CombinedOutput()
	if err != nil {
		return
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) < 2 {
			continue
		}
		attrName := parts[0]
		switch attrName {
		case "com.apple.quarantine", "com.apple.provenance":
			exec.Command("xattr", "-d", attrName, path).Run()
		}
	}
}

func cleanOldRotatedFiles(logDir, baseName string) {
	if runtime.GOOS == "darwin" {
		exec.Command("xattr", "-dr", "com.apple.quarantine", logDir).Run()
		exec.Command("xattr", "-dr", "com.apple.provenance", logDir).Run()
	}

	matches, _ := filepath.Glob(filepath.Join(logDir, baseName+"-*"))
	for _, m := range matches {
		if info, err := os.Stat(m); err == nil && info.Mode().Perm() != 0644 {
			os.Chmod(m, 0644)
		}
		removeMacOSQuarantine(m)
	}
}

// createSafeFileWriter creates a zapcore.WriteSyncer that handles macOS permission issues.
// It tries lumberjack first; if rotate fails (macOS sandbox/TCC), falls back to simple append.
func createSafeFileWriter(cfg *ZapConfig) zapcore.WriteSyncer {
	lj := &lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
		LocalTime:  true,
	}

	testData := []byte("log rotation test\n")
	if _, err := lj.Write(testData); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: lumberjack write failed (%v), using simple file writer for %s\n", err, cfg.Filename)
		lj.Close()
		return newSimpleFileWriter(cfg.Filename)
	}

	if err := lj.Rotate(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: lumberjack rotate failed (%v), using simple file writer for %s\n", err, cfg.Filename)
		lj.Close()
		return newSimpleFileWriter(cfg.Filename)
	}

	return zapcore.AddSync(lj)
}

type simpleFileWriter struct {
	mu     sync.Mutex
	file   *os.File
	path   string
	closed bool
}

func newSimpleFileWriter(path string) *simpleFileWriter {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: cannot open log file %s: %v\n", path, err)
		return &simpleFileWriter{path: path, file: nil}
	}
	return &simpleFileWriter{path: path, file: f}
}

func (w *simpleFileWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.file == nil {
		return len(p), nil
	}
	return w.file.Write(p)
}

func (w *simpleFileWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	return w.file.Sync()
}

func (w *simpleFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	if w.file == nil {
		return nil
	}
	return w.file.Close()
}
