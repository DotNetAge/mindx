package security

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidatePath validates that a user-provided path stays within base directory
func ValidatePath(baseDir, userPath string) (string, error) {
	// Clean and normalize paths
	cleanBase := filepath.Clean(baseDir)
	cleanUser := filepath.Clean(userPath)

	// Reject absolute paths
	if filepath.IsAbs(cleanUser) {
		return "", errors.New("absolute paths not allowed")
	}

	// Reject paths containing ..
	if strings.Contains(cleanUser, "..") {
		return "", errors.New("path traversal detected: '..' not allowed")
	}

	// Join paths
	fullPath := filepath.Join(cleanBase, cleanUser)
	cleanFull := filepath.Clean(fullPath)

	// Ensure the result is still within base directory
	rel, err := filepath.Rel(cleanBase, cleanFull)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if strings.HasPrefix(rel, "..") {
		return "", errors.New("path traversal detected: result outside base directory")
	}

	// Check if path resolves to a symlink that might escape
	finalPath, err := filepath.EvalSymlinks(cleanFull)
	if err == nil {
		rel, err := filepath.Rel(cleanBase, finalPath)
		if err == nil && strings.HasPrefix(rel, "..") {
			return "", errors.New("path traversal detected: symlink escapes base directory")
		}
	}

	return cleanFull, nil
}

// ValidateFilePath validates a file path against allowed base directories
func ValidateFilePath(baseDir, userPath string, allowedDirs []string) (string, error) {
	// First, do basic path validation
	validatedPath, err := ValidatePath(baseDir, userPath)
	if err != nil {
		return "", err
	}

	// If no allowed directories specified, return validated path
	if len(allowedDirs) == 0 {
		return validatedPath, nil
	}

	// Check if the validated path is within any of the allowed directories
	allowed := false
	for _, allowedDir := range allowedDirs {
		cleanAllowed := filepath.Clean(allowedDir)
		if strings.HasPrefix(validatedPath, cleanAllowed) {
			allowed = true
			break
		}
	}

	if !allowed {
		return "", errors.New("access denied: path not in allowed directories")
	}

	return validatedPath, nil
}

// IsAllowedDirectory checks if a directory is in the allowed list
func IsAllowedDirectory(path string, allowedDirs []string) bool {
	cleanPath := filepath.Clean(path)

	for _, allowedDir := range allowedDirs {
		cleanAllowed := filepath.Clean(allowedDir)
		if strings.HasPrefix(cleanPath, cleanAllowed) {
			return true
		}
	}

	return false
}

// EnsureDirectoryExists ensures a directory exists, creating it if necessary
func EnsureDirectoryExists(dir string, perm os.FileMode) error {
	if err := os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	return nil
}

// SanitizeFilename sanitizes a filename by removing dangerous characters
func SanitizeFilename(filename string) string {
	// Remove directory separators
	filename = filepath.Base(filename)

	// Replace dangerous characters with underscore
	dangerousChars := []string{"..", "~", "\x00"}
	for _, char := range dangerousChars {
		filename = strings.ReplaceAll(filename, char, "_")
	}

	return filename
}
