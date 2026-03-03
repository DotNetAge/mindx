package builtins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFile_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	// Create documents dir and test file
	docsDir := filepath.Join(tmpDir, "documents")
	require.NoError(t, os.MkdirAll(docsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "test.txt"), []byte("hello world"), 0644))

	params := map[string]any{
		"path": "test.txt",
	}

	result, err := ReadFile(params)
	assert.NoError(t, err)
	assert.Contains(t, result, "hello world")
	assert.Contains(t, result, `"success": true`)
}

func TestReadFile_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	// Create a file with absolute path
	testFile := filepath.Join(tmpDir, "absolute_test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("absolute content"), 0644))

	params := map[string]any{
		"path": testFile,
	}

	result, err := ReadFile(params)
	assert.NoError(t, err)
	assert.Contains(t, result, "absolute content")
}

func TestReadFile_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	params := map[string]any{
		"path": "../../etc/passwd",
	}

	_, err := ReadFile(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal detected")
}

func TestReadFile_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "documents"), 0755))

	params := map[string]any{
		"path": "nonexistent.txt",
	}

	result, err := ReadFile(params)
	assert.NoError(t, err) // Returns JSON with error info, not a Go error
	assert.Contains(t, result, `"success": false`)
	assert.Contains(t, result, "文件不存在")
}

func TestReadFile_MissingParam(t *testing.T) {
	params := map[string]any{}

	_, err := ReadFile(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid param")
}
