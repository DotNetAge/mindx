package builtins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	_, err := WriteFile(params)
	assert.NoError(t, err)

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

	_, err := WriteFile(params)
	assert.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(targetDir, "result.txt"))
	assert.NoError(t, err)
	assert.Equal(t, "abs path write", string(content))
}

func TestWriteFile_AbsolutePathWithDangerousFlag(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	targetDir := filepath.Join(tmpDir, "abs_dir")

	params := map[string]any{
		"filename":  "result.txt",
		"content":   "abs path write",
		"path":      targetDir,
		"dangerous": true,
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

func TestWriteFile_PathTraversalBlockedByFilename(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	writeFileAccessConfigForTest(t, tmpDir, true, nil)

	params := map[string]any{
		"filename": "../escape.txt",
		"content":  "blocked",
	}

	_, err := WriteFile(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outside allowed scope")
}

func TestWriteFile_PathTraversalBlockedByPath(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	writeFileAccessConfigForTest(t, tmpDir, true, nil)

	params := map[string]any{
		"filename": "ok.txt",
		"content":  "blocked",
		"path":     "../escape",
	}

	_, err := WriteFile(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outside allowed scope")
}

func TestWriteFile_FileAccessEnabled_DefaultWorkspaceOnly(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")
	writeFileAccessConfigForTest(t, tmpDir, true, nil)

	outsideFile := filepath.Join(t.TempDir(), "blocked.txt")
	_, err := WriteFile(map[string]any{
		"filename": outsideFile,
		"content":  "blocked",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outside allowed scope")
}

func TestWriteFile_FileAccessEnabled_AllowsConfiguredExternalDir(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	allowedDir := t.TempDir()
	writeFileAccessConfigForTest(t, tmpDir, true, []string{allowedDir})

	targetFile := filepath.Join(allowedDir, "allowed.txt")
	_, err := WriteFile(map[string]any{
		"filename": targetFile,
		"content":  "allowed",
	})
	assert.NoError(t, err)

	content, err := os.ReadFile(targetFile)
	assert.NoError(t, err)
	assert.Equal(t, "allowed", string(content))
}

func TestWriteFile_DefaultPolicyDeniesAbsoluteOutsideWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	outsideFile := filepath.Join(t.TempDir(), "blocked.txt")
	_, err := WriteFile(map[string]any{
		"filename": outsideFile,
		"content":  "blocked",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outside allowed scope")
}

func TestWriteFile_FileAccessEnabled_AllowsNonExistentExternalDirPattern(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	parentDir := t.TempDir()
	allowedDir := filepath.Join(parentDir, "new-allowed-dir")
	writeFileAccessConfigForTest(t, tmpDir, true, []string{allowedDir + "/**"})

	targetFile := filepath.Join(allowedDir, "nested", "allowed.txt")
	_, err := WriteFile(map[string]any{
		"filename": targetFile,
		"content":  "allowed",
	})
	require.NoError(t, err)

	content, readErr := os.ReadFile(targetFile)
	require.NoError(t, readErr)
	assert.Equal(t, "allowed", string(content))
}
