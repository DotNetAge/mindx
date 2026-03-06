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

	// Create test file at workspace root
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello world"), 0644))

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

	// Create file inside workspace
	testFile := filepath.Join(tmpDir, "absolute_test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("absolute content"), 0644))

	// Absolute path should work
	params := map[string]any{
		"path": testFile,
	}

	result, err := ReadFile(params)
	assert.NoError(t, err)
	assert.Contains(t, result, "absolute content")
}

func TestReadFile_AbsolutePathOutsideWorkspaceDeniedByDefault(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	// Create a file outside workspace
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "outside.txt")
	require.NoError(t, os.WriteFile(outsideFile, []byte("outside content"), 0644))

	// Absolute path outside workspace should be denied by default
	params := map[string]any{
		"path": outsideFile,
	}

	result, err := ReadFile(params)
	// ReadFile reports business failures inside the JSON payload, while Go errors are reserved for invalid inputs/system failures.
	assert.NoError(t, err)
	assert.Contains(t, result, `"success": false`)
	assert.Contains(t, result, "读取路径超出允许范围")
}

func TestReadFile_RelativePathResolvesToWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	// Create subdirectory and file
	subDir := filepath.Join(tmpDir, "documents")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "note.txt"), []byte("note content"), 0644))

	params := map[string]any{
		"path": "documents/note.txt",
	}

	result, err := ReadFile(params)
	assert.NoError(t, err)
	assert.Contains(t, result, "note content")
	assert.Contains(t, result, `"success": true`)
}

func TestReadFile_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

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

func TestReadFile_FileAccessEnabled_DefaultWorkspaceOnly(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")
	writeFileAccessConfigForTest(t, tmpDir, true, nil)

	outsideFile := filepath.Join(t.TempDir(), "outside.txt")
	require.NoError(t, os.WriteFile(outsideFile, []byte("outside content"), 0644))

	result, err := ReadFile(map[string]any{"path": outsideFile})
	assert.NoError(t, err)
	assert.Contains(t, result, `"success": false`)
	assert.Contains(t, result, "读取路径超出允许范围")
}

func TestReadFile_FileAccessEnabled_AllowsConfiguredExternalPath(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	allowedDir := t.TempDir()
	allowedFile := filepath.Join(allowedDir, "allowed.txt")
	require.NoError(t, os.WriteFile(allowedFile, []byte("allowed content"), 0644))
	writeFileAccessConfigForTest(t, tmpDir, true, []string{allowedDir})

	result, err := ReadFile(map[string]any{"path": allowedFile})
	assert.NoError(t, err)
	assert.Contains(t, result, `"success": true`)
	assert.Contains(t, result, "allowed content")
}
