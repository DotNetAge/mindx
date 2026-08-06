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

var (
	globalLogger Logger
	loggerOnce   sync.Once
)

// DefaultZapLogger returns the global singleton zap logger.
// The first call initializes it with the provided config; subsequent calls
// return the same instance and ignore the config parameter.
func DefaultZapLogger(cfg *ZapConfig) Logger {
	loggerOnce.Do(func() {
		globalLogger = newZapLogger(cfg)
	})
	return globalLogger
}

// newZapLogger creates a new zap logger instance (unshared).
func newZapLogger(cfg *ZapConfig) Logger {
	if cfg.Filename == "" {
		cfg.Filename = "logs/mindx.log"
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 20
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = 30
	}
	if cfg.MaxBackups == 0 {
		cfg.MaxBackups = 7
	}

	// Derive error log filename from the main log filename.
	// e.g. /path/to/mindx.log → /path/to/error.log
	errorFilename := deriveErrorFilename(cfg.Filename)

	logDir := filepath.Dir(cfg.Filename)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: failed to create log directory %s: %v\n", logDir, err)
	}

	fixLogFilePermissions(cfg.Filename)
	fixLogFilePermissions(errorFilename)

	// Writer for all logs (mindx.log)
	fileWriter := createSafeFileWriter(cfg)

	// Writer for error-only logs (error.log)
	errorCfg := *cfg
	errorCfg.Filename = errorFilename
	errorWriter := createSafeFileWriter(&errorCfg)

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	fileCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		fileWriter,
		zap.DebugLevel,
	)

	errorCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		errorWriter,
		zap.ErrorLevel,
	)

	cores := []zapcore.Core{fileCore, errorCore}

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

// deriveErrorFilename returns the error log path derived from the main log path.
//   - /path/to/mindx.log    → /path/to/error.log
//   - /path/to/foo.log      → /path/to/error.log
//   - /path/to/mindx        → /path/to/error.log (no extension)
func deriveErrorFilename(mainPath string) string {
	dir := filepath.Dir(mainPath)
	return filepath.Join(dir, "error.log")
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
		_ = os.Chmod(dir, 0755)
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		f, createErr := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if createErr == nil {
			_ = f.Close()
		}
		return
	}

	_ = os.Chmod(filename, 0644)

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
			_ = exec.Command("xattr", "-d", attrName, path).Run()
		}
	}
}

func cleanOldRotatedFiles(logDir, baseName string) {
	if runtime.GOOS == "darwin" {
		_ = exec.Command("xattr", "-dr", "com.apple.quarantine", logDir).Run()
		_ = exec.Command("xattr", "-dr", "com.apple.provenance", logDir).Run()
	}

	matches, _ := filepath.Glob(filepath.Join(logDir, baseName+"-*"))
	for _, m := range matches {
		if info, err := os.Stat(m); err == nil && info.Mode().Perm() != 0644 {
			_ = os.Chmod(m, 0644)
		}
		removeMacOSQuarantine(m)
	}
}

// createSafeFileWriter 创建日志写入器：
//  1. 优先使用 lumberjack（按大小滚动）；通过同目录临时文件探测旋转能力，
//     避免在真实日志上做「写入+旋转」测试污染日志内容。
//  2. 目录不支持重命名（macOS 沙箱/TCC 等）时降级为追加写入（不滚动）。
//  3. 两种路径都会检测日志文件被外部删除并自动重建，避免日志写入已删除的 inode
//     造成「文件不重建 + 日志黑洞」。
func createSafeFileWriter(cfg *ZapConfig) zapcore.WriteSyncer {
	if !rotationCapable(filepath.Dir(cfg.Filename)) {
		fmt.Fprintf(os.Stderr, "WARNING: 日志目录不支持重命名，%s 将使用追加写入（不滚动）\n", cfg.Filename)
		return newSimpleFileWriter(cfg.Filename)
	}
	return zapcore.AddSync(newReopenOnMissingWriter(cfg))
}

// rotationCapable 探测目录是否支持重命名（滚动依赖 os.Rename）。
// 用同目录下的临时文件做验证，成功即认为 lumberjack 可正常滚动。
func rotationCapable(dir string) bool {
	if dir == "" {
		dir = "."
	}
	probe, err := os.CreateTemp(dir, ".rotate-probe-*")
	if err != nil {
		return false
	}
	probePath := probe.Name()
	defer os.Remove(probePath)

	if _, err := probe.WriteString("probe"); err != nil {
		_ = probe.Close()
		return false
	}
	if err := probe.Close(); err != nil {
		return false
	}

	renamed := probePath + ".renamed"
	defer os.Remove(renamed)
	return os.Rename(probePath, renamed) == nil
}

// newLumberjack 以给定配置创建 lumberjack.Logger。
func newLumberjack(cfg *ZapConfig) *lumberjack.Logger {
	return &lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
		LocalTime:  true,
	}
}

// reopenOnMissingWriter 包装 lumberjack.Logger，在日志文件被外部删除时自动重建：
// lumberjack 一旦打开文件就持有句柄，外部删除后写入会落到已删除的 inode 上，
// 文件不会重建、日志直接丢失；此处每次写入前检查路径是否存在，缺失即重建。
type reopenOnMissingWriter struct {
	mu    sync.Mutex
	cfg   *ZapConfig
	inner *lumberjack.Logger
}

func newReopenOnMissingWriter(cfg *ZapConfig) *reopenOnMissingWriter {
	return &reopenOnMissingWriter{cfg: cfg, inner: newLumberjack(cfg)}
}

func (w *reopenOnMissingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := os.Lstat(w.cfg.Filename); os.IsNotExist(err) {
		// 日志文件被外部删除：重建文件并重新打开，避免写入已删除的 inode
		_ = w.inner.Close()
		w.inner = newLumberjack(w.cfg)
	}
	return w.inner.Write(p)
}

func (w *reopenOnMissingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.inner.Close()
}

// simpleFileWriter 是旋转不可用（目录禁止重命名）时的追加写入兜底实现。
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
	if w.closed {
		return len(p), nil
	}
	if _, statErr := os.Lstat(w.path); os.IsNotExist(statErr) {
		// 日志文件被外部删除：重新打开并重建文件
		if w.file != nil {
			_ = w.file.Close()
		}
		f, openErr := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if openErr != nil {
			fmt.Fprintf(os.Stderr, "WARNING: 重建日志文件失败 %s: %v\n", w.path, openErr)
			w.file = nil
			return len(p), nil
		}
		w.file = f
	}
	if w.file == nil {
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
