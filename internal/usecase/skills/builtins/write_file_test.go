package builtins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAndSanitizePath_ValidPath(t *testing.T) {
	baseDir := "/tmp/test/documents"
	result, err := validateAndSanitizePath(baseDir, "subdir", "test.txt")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(baseDir, "subdir", "test.txt"), result)
}

func TestValidateAndSanitizePath_PathTraversalInPath(t *testing.T) {
	baseDir := "/tmp/test/documents"
	_, err := validateAndSanitizePath(baseDir, "../../../etc", "passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal detected")
}

func TestValidateAndSanitizePath_PathTraversalInFilename(t *testing.T) {
	baseDir := "/tmp/test/documents"
	_, err := validateAndSanitizePath(baseDir, "subdir", "../../etc/passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal detected")
}

func TestValidateAndSanitizePath_AbsolutePath(t *testing.T) {
	baseDir := "/tmp/test/documents"
	_, err := validateAndSanitizePath(baseDir, "/etc", "passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "absolute paths not allowed")
}

func TestValidateAndSanitizePath_NestedValidPath(t *testing.T) {
	baseDir := "/tmp/test/documents"
	result, err := validateAndSanitizePath(baseDir, "a/b/c", "file.txt")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(baseDir, "a/b/c", "file.txt"), result)
}

func TestWriteFile_PathTraversalPrevented(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	// Create documents dir
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "documents"), 0755))

	params := map[string]any{
		"filename": "test.txt",
		"content":  "hello",
		"path":     "../../etc",
	}

	_, err := WriteFile(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal detected")
}

func TestWriteFile_ValidWrite(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	// Create documents dir
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "documents"), 0755))

	params := map[string]any{
		"filename": "test.txt",
		"content":  "hello world",
	}

	result, err := WriteFile(params)
	assert.NoError(t, err)
	assert.Contains(t, result, "test.txt")

	// Verify file was actually written
	content, err := os.ReadFile(filepath.Join(tmpDir, "documents", "test.txt"))
	assert.NoError(t, err)
	assert.Equal(t, "hello world", string(content))
}

func TestWriteFile_ValidWriteWithPath(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	params := map[string]any{
		"filename": "test.txt",
		"content":  "hello world",
		"path":     "subdir",
	}

	result, err := WriteFile(params)
	assert.NoError(t, err)
	assert.Contains(t, result, "test.txt")

	// Verify file was actually written in the subdirectory
	content, err := os.ReadFile(filepath.Join(tmpDir, "documents", "subdir", "test.txt"))
	assert.NoError(t, err)
	assert.Equal(t, "hello world", string(content))
}
