package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetInstallPath_WithEnvVar(t *testing.T) {
	os.Setenv("MINDX_PATH", "/custom/install/path")
	defer os.Unsetenv("MINDX_PATH")

	path, err := GetInstallPath()
	assert.NoError(t, err)
	assert.Equal(t, "/custom/install/path", path)
}

func TestGetInstallPath_WithoutEnvVar(t *testing.T) {
	os.Unsetenv("MINDX_PATH")

	path, err := GetInstallPath()
	assert.NoError(t, err)
	assert.NotEmpty(t, path)
}

func TestGetInstallPath_EnvVarTakesPrecedence(t *testing.T) {
	os.Setenv("MINDX_PATH", "/override/path")
	defer os.Unsetenv("MINDX_PATH")

	path, err := GetInstallPath()
	assert.NoError(t, err)
	assert.Equal(t, "/override/path", path)
}
