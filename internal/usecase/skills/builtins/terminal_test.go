package builtins

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTerminal_SafeCommand(t *testing.T) {
	var expectedCmd string
	if runtime.GOOS == "windows" {
		expectedCmd = "echo hello"
	} else {
		expectedCmd = "echo hello"
	}

	params := map[string]any{
		"command": expectedCmd,
	}

	result, err := Terminal(params)
	assert.NoError(t, err)
	assert.Contains(t, result, "hello")
	assert.Contains(t, result, `"exit_code": 0`)
}

func TestTerminal_DangerousCharacters(t *testing.T) {
	params := map[string]any{
		"command": "echo hello; cat /etc/passwd",
	}

	_, err := Terminal(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dangerous characters")
}

func TestTerminal_DangerousCommand(t *testing.T) {
	params := map[string]any{
		"command": "rm -rf /tmp/test",
	}

	_, err := Terminal(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dangerous command")
}

func TestTerminal_DangerousCommandWithFlag(t *testing.T) {
	// With dangerous=true, the command is allowed (but we test a safe one)
	params := map[string]any{
		"command":   "echo dangerous-test",
		"dangerous": true,
	}

	result, err := Terminal(params)
	assert.NoError(t, err)
	assert.Contains(t, result, "dangerous-test")
}

func TestTerminal_MissingParam(t *testing.T) {
	params := map[string]any{}

	_, err := Terminal(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid param")
}

func TestTerminal_PipeCharBlocked(t *testing.T) {
	params := map[string]any{
		"command": "ls | grep test",
	}

	_, err := Terminal(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dangerous characters")
}
