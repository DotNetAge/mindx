package builtins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	dangerous := false
	if d, ok := params["dangerous"].(bool); ok {
		dangerous = d
	}
	if d, ok := params["dangerous"].(string); ok && d == "true" {
		dangerous = true
	}
	workDir := os.Getenv("MINDX_WORKSPACE")
	if workDir == "" {
		return "", fmt.Errorf("MINDX_WORKSPACE environment variable is not set")
	}
	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve workspace path: %w", err)
	}
	workDir = absWorkDir
	resolvedWorkDir, err := filepath.EvalSymlinks(workDir)
	if err != nil {
		return "", fmt.Errorf("workspace path contains unresolvable symlinks %s: %w", workDir, err)
	}

	// Determine the target file path
	var filePath string
	needsWorkspaceBoundaryCheck := false
	cleanFilename := filepath.Clean(filename)

	if path, ok := params["path"].(string); ok && path != "" {
		// "path" param provided: treat as directory, append filename
		cleanPath := filepath.Clean(path)
		if filepath.IsAbs(cleanPath) {
			if !dangerous {
				return "", fmt.Errorf("absolute path requires dangerous=true parameter")
			}
			filePath = filepath.Join(cleanPath, cleanFilename)
		} else {
			filePath = filepath.Clean(filepath.Join(workDir, cleanPath, cleanFilename))
			needsWorkspaceBoundaryCheck = true
			if !isPathWithinWorkspace(workDir, filePath) {
				return "", fmt.Errorf("path outside workspace is not allowed")
			}
		}
	} else if filepath.IsAbs(cleanFilename) {
		// filename itself is an absolute path
		if !dangerous {
			return "", fmt.Errorf("absolute path requires dangerous=true parameter")
		}
		filePath = cleanFilename
	} else {
		// Relative filename: resolve against workspace
		filePath = filepath.Clean(filepath.Join(workDir, cleanFilename))
		needsWorkspaceBoundaryCheck = true
		if !isPathWithinWorkspace(workDir, filePath) {
			return "", fmt.Errorf("path outside workspace is not allowed")
		}
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create dir %s: %w", dir, err)
	}
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path %s: %w", dir, err)
	}
	if needsWorkspaceBoundaryCheck {
		if !isPathWithinWorkspace(resolvedWorkDir, filepath.Join(resolvedDir, filepath.Base(filePath))) {
			return "", fmt.Errorf("path outside workspace is not allowed")
		}
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

func isPathWithinWorkspace(workDir, targetPath string) bool {
	relPath, relErr := filepath.Rel(workDir, filepath.Clean(targetPath))
	if relErr != nil {
		return false
	}
	// Writing directly into workspace root is allowed.
	if relPath == "." {
		return true
	}
	return relPath != ".." && !strings.HasPrefix(relPath, ".."+string(filepath.Separator))
}
