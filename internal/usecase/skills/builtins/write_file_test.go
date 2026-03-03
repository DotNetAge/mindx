package builtins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteFile_ValidWrite(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	params := map[string]any{
		"filename": "test.txt",
		"content":  "hello world",
	}

	result, err := WriteFile(params)
	assert.NoError(t, err)
	assert.Contains(t, result, "test.txt")

	// Verify file was written at workspace root
	content, err := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
	assert.NoError(t, err)
	assert.Equal(t, "hello world", string(content))
}

func TestWriteFile_ValidWriteWithRelativePath(t *testing.T) {
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

	// Verify file was written in the subdirectory
	content, err := os.ReadFile(filepath.Join(tmpDir, "subdir", "test.txt"))
	assert.NoError(t, err)
	assert.Equal(t, "hello world", string(content))
}

func TestWriteFile_AbsolutePathInFilename(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	targetFile := filepath.Join(tmpDir, "outside", "abs_test.txt")

	params := map[string]any{
		"filename": targetFile,
		"content":  "absolute write",
	}

	result, err := WriteFile(params)
	assert.NoError(t, err)
	assert.Contains(t, result, "abs_test.txt")

	content, err := os.ReadFile(targetFile)
	assert.NoError(t, err)
	assert.Equal(t, "absolute write", string(content))
}

func TestWriteFile_AbsolutePathParam(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	targetDir := filepath.Join(tmpDir, "abs_dir")

	params := map[string]any{
		"filename": "result.txt",
		"content":  "abs path write",
		"path":     targetDir,
	}

	result, err := WriteFile(params)
	assert.NoError(t, err)
	assert.Contains(t, result, "result.txt")

	content, err := os.ReadFile(filepath.Join(targetDir, "result.txt"))
	assert.NoError(t, err)
	assert.Equal(t, "abs path write", string(content))
}

func TestWriteFile_MissingFilename(t *testing.T) {
	params := map[string]any{
		"content": "hello",
	}

	_, err := WriteFile(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid param: filename")
}

func TestWriteFile_MissingContent(t *testing.T) {
	params := map[string]any{
		"filename": "test.txt",
	}

	_, err := WriteFile(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid param: content")
}

func TestWriteFile_DocumentsSubdir(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	params := map[string]any{
		"filename": "note.txt",
		"content":  "document content",
		"path":     "documents/notes",
	}

	result, err := WriteFile(params)
	assert.NoError(t, err)
	assert.Contains(t, result, "note.txt")

	content, err := os.ReadFile(filepath.Join(tmpDir, "documents", "notes", "note.txt"))
	assert.NoError(t, err)
	assert.Equal(t, "document content", string(content))
}
