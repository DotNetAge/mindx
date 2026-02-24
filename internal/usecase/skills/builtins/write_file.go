package builtins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WriteFile writes content to a file with security validation
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

	baseDir := filepath.Join(workDir, "documents")

	var filePath string
	if path, ok := params["path"].(string); ok && path != "" {
		// SECURITY: Validate path to prevent traversal
		validatedPath, err := validateAndSanitizePath(baseDir, path, filename)
		if err != nil {
			return "", err
		}
		filePath = validatedPath
	} else {
		// No path specified, use base directory
		filePath = filepath.Join(baseDir, filename)
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

// validateAndSanitizePath validates and sanitizes a file path to prevent path traversal attacks
func validateAndSanitizePath(baseDir, userPath, filename string) (string, error) {
	// Clean all paths
	cleanBase := filepath.Clean(baseDir)
	cleanUserPath := filepath.Clean(userPath)
	cleanFilename := filepath.Clean(filename)

	// Reject absolute paths in user input
	if filepath.IsAbs(cleanUserPath) {
		return "", fmt.Errorf("absolute paths not allowed in user path")
	}

	// Reject paths containing ..
	if filepath.HasPrefix(cleanFilename, "..") {
		return "", fmt.Errorf("path traversal detected: .. not allowed in filename")
	}

	if filepath.HasPrefix(cleanUserPath, "..") {
		return "", fmt.Errorf("path traversal detected: .. not allowed in path")
	}

	// Join paths
	fullPath := filepath.Join(cleanBase, cleanUserPath, cleanFilename)
	cleanFull := filepath.Clean(fullPath)

	// Ensure the result is still within base directory
	rel, err := filepath.Rel(cleanBase, cleanFull)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if filepath.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path traversal detected: result outside base directory")
	}

	return cleanFull, nil
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
