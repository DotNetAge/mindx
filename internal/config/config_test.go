package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadServerConfig_Success(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	configDir := filepath.Join(tmpDir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configContent := `server:
  port: 8080
  host: "localhost"
`
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "server.yml"), []byte(configContent), 0644))

	cfg, err := LoadServerConfig()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestLoadServerConfig_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	configDir := filepath.Join(tmpDir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	_, err := LoadServerConfig()
	// Should fail because no config file and no template
	assert.Error(t, err)
}

func TestInitVippers_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MINDX_WORKSPACE", tmpDir)
	defer os.Unsetenv("MINDX_WORKSPACE")

	configDir := filepath.Join(tmpDir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	// No config files exist, should return error
	_, _, _, _, err := InitVippers()
	assert.Error(t, err)
}
