package builtins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ReadFile reads content from a file
// Absolute paths are used directly; relative paths resolve against MINDX_WORKSPACE
func ReadFile(params map[string]any) (string, error) {
	path, ok := params["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("invalid param: path")
	}

	startTime := time.Now()

	cleanPath := filepath.Clean(path)

	// Resolve relative paths against workspace root
	workDir := os.Getenv("MINDX_WORKSPACE")
	if workDir == "" {
		return "", fmt.Errorf("MINDX_WORKSPACE environment variable is not set")
	}
	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve workspace path: %w", err)
	}
	resolvedWorkDir, err := filepath.EvalSymlinks(absWorkDir)
	if err != nil {
		return "", fmt.Errorf("workspace path contains unresolvable symlinks %s: %w", absWorkDir, err)
	}

	if !filepath.IsAbs(cleanPath) {
		cleanPath = filepath.Clean(filepath.Join(absWorkDir, cleanPath))
	}
	filePolicy, err := loadFileAccessPolicy(resolvedWorkDir)
	if err != nil {
		return "", fmt.Errorf("failed to load file access policy: %w", err)
	}
	if !filePolicy.isAllowed(cleanPath) {
		return getJSONReadResult(cleanPath, "", 0, false, "读取路径超出允许范围", time.Since(startTime))
	}

	// Check file exists
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return getJSONReadResult(cleanPath, "", 0, false, fmt.Sprintf("文件不存在: %s", cleanPath), time.Since(startTime))
		}
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return getJSONReadResult(cleanPath, "", 0, false, fmt.Sprintf("路径是目录而非文件: %s", cleanPath), time.Since(startTime))
	}

	resolvedPath, err := filepath.EvalSymlinks(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path %s: %w", cleanPath, err)
	}
	if !filePolicy.isAllowed(resolvedPath) {
		return getJSONReadResult(cleanPath, "", 0, false, "读取路径超出允许范围", time.Since(startTime))
	}

	// Read file content
	content, err := os.ReadFile(cleanPath)
	if err != nil {
		return getJSONReadResult(cleanPath, "", 0, false, fmt.Sprintf("读取文件失败: %v", err), time.Since(startTime))
	}

	elapsed := time.Since(startTime)
	return getJSONReadResult(cleanPath, string(content), len(content), true, "", elapsed)
}

func getJSONReadResult(filePath, content string, bytesRead int, success bool, errMsg string, elapsed time.Duration) (string, error) {
	output := map[string]interface{}{
		"success":    success,
		"path":       filePath,
		"elapsed_ms": elapsed.Milliseconds(),
	}

	if success {
		output["content"] = content
		output["bytes_read"] = bytesRead
	} else {
		output["error"] = errMsg
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("json serialize failed: %w", err)
	}
	return string(data), nil
}
