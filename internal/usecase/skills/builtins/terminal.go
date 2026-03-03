package builtins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// dangerousCommands lists commands that require explicit dangerous=true
var dangerousCommands = map[string]bool{
	// Unix destructive commands
	"rm": true, "dd": true, "mkfs": true, "format": true,
	"shutdown": true, "reboot": true, "init": true,
	"kill": true, "killall": true, "pkill": true,
	"fdisk": true, "parted": true, "chmod": true, "chown": true,
	"sudo": true, "su": true, "systemctl": true,
	// Windows destructive commands
	"del": true, "rd": true, "rmdir": true,
	"powershell": true,
}

// dangerousChars lists shell metacharacters that indicate injection
var dangerousChars = []string{";", "&", "|", "`", "$(", "${", ")", ">", ">>", "<", "\n", "\r"}

// Terminal executes a terminal command with security validation
func Terminal(params map[string]any) (string, error) {
	command, ok := params["command"].(string)
	if !ok || command == "" {
		return "", fmt.Errorf("invalid param: command")
	}

	timeoutSec := 30
	if t, ok := params["timeout"].(float64); ok && t > 0 {
		timeoutSec = int(t)
	}

	dangerous := false
	if d, ok := params["dangerous"].(bool); ok {
		dangerous = d
	}
	if d, ok := params["dangerous"].(string); ok && d == "true" {
		dangerous = true
	}

	// SECURITY: Check for dangerous characters (injection patterns)
	for _, ch := range dangerousChars {
		if strings.Contains(command, ch) {
			return "", fmt.Errorf("command contains dangerous characters: %s", ch)
		}
	}

	// SECURITY: Check for dangerous commands
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	baseCmd := parts[0]
	if dangerousCommands[baseCmd] && !dangerous {
		return "", fmt.Errorf("dangerous command '%s' requires dangerous=true parameter", baseCmd)
	}

	// Execute command with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return getJSONTerminalResult("", "Command timed out", 124, time.Duration(timeoutSec)*time.Second)
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", fmt.Errorf("failed to execute command: %w", err)
		}
	}

	output := stdout.String()
	if output == "" {
		output = stderr.String()
	}

	if exitCode != 0 {
		return getJSONTerminalResult(output, fmt.Sprintf("Command failed with exit code %d", exitCode), exitCode, time.Duration(0))
	}

	return getJSONTerminalResult(output, "", exitCode, time.Duration(0))
}

func getJSONTerminalResult(output, errMsg string, exitCode int, elapsed time.Duration) (string, error) {
	result := map[string]interface{}{
		"exit_code": exitCode,
	}

	if errMsg != "" {
		result["error"] = errMsg
		if output != "" {
			result["output"] = output
		}
	} else {
		result["result"] = output
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("json serialize failed: %w", err)
	}
	return string(data), nil
}
