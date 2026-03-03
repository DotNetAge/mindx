package builtins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ReadFile reads content from a file with security validation
func ReadFile(params map[string]any) (string, error) {
	path, ok := params["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("invalid param: path")
	}

	startTime := time.Now()

	// SECURITY: Reject path traversal
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path traversal detected: .. not allowed")
	}

	// Determine base directory
	workDir := os.Getenv("MINDX_WORKSPACE")
	if workDir == "" {
		return "", fmt.Errorf("MINDX_WORKSPACE environment variable is not set")
	}

	// If path is not absolute, resolve relative to workspace documents or data
	if !filepath.IsAbs(path) {
		baseDir := filepath.Join(workDir, "documents")
		if _, err := os.Stat(baseDir); os.IsNotExist(err) {
			baseDir = filepath.Join(workDir, "data")
		}
		path = filepath.Join(baseDir, path)
	}

	// Clean and validate final path
	cleanPath := filepath.Clean(path)

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
