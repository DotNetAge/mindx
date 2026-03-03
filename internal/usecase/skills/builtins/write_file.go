package builtins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WriteFile writes content to a file
// Supports absolute paths directly; relative paths resolve against MINDX_WORKSPACE
func WriteFile(params map[string]any) (string, error) {
	filename, ok := params["filename"].(string)
	if !ok || filename == "" {
		return "", fmt.Errorf("invalid param: filename")
	}

	content, ok := params["content"].(string)
	if !ok {
		return "", fmt.Errorf("invalid param: content")
	}

	startTime := time.Now()

	// Determine the target file path
	var filePath string
	cleanFilename := filepath.Clean(filename)

	if path, ok := params["path"].(string); ok && path != "" {
		// "path" param provided: treat as directory, append filename
		cleanPath := filepath.Clean(path)
		if filepath.IsAbs(cleanPath) {
			filePath = filepath.Join(cleanPath, cleanFilename)
		} else {
			workDir := os.Getenv("MINDX_WORKSPACE")
			if workDir == "" {
				return "", fmt.Errorf("MINDX_WORKSPACE environment variable is not set")
			}
			filePath = filepath.Join(workDir, cleanPath, cleanFilename)
		}
	} else if filepath.IsAbs(cleanFilename) {
		// filename itself is an absolute path
		filePath = cleanFilename
	} else {
		// Relative filename: resolve against workspace
		workDir := os.Getenv("MINDX_WORKSPACE")
		if workDir == "" {
			return "", fmt.Errorf("MINDX_WORKSPACE environment variable is not set")
		}
		filePath = filepath.Join(workDir, cleanFilename)
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
