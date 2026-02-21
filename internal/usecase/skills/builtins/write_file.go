package builtins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func WriteFile(params map[string]any) (string, error) {
	filename, ok := params["filename"].(string)
	if !ok {
		return "", fmt.Errorf("invalid param: filename")
	}

	content, ok := params["content"].(string)
	if !ok {
		return "", fmt.Errorf("invalid param: content")
	}

	startTime := time.Now()

	workDir := os.Getenv("MINDX_WORKSPACE")
	if workDir == "" {
		return "", fmt.Errorf("MINDX_WORKSPACE environment variable is not set")
	}

	var filePath string
	if path, ok := params["path"].(string); ok && path != "" {
		filePath = filepath.Join(workDir, "documents", path, filename)
	} else {
		filePath = filepath.Join(workDir, "documents", filename)
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create dir %s: %w", dir, err)
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	elapsed := time.Since(startTime)

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	return getJSONWriteResult(absPath, len(content), elapsed)
}

func getJSONWriteResult(filePath string, contentLength int, elapsed time.Duration) (string, error) {
	output := map[string]interface{}{
		"file_path":      filePath,
		"content_length": contentLength,
		"elapsed_ms":     elapsed.Milliseconds(),
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("json serialize failed: %w", err)
	}
	return string(data), nil
}
