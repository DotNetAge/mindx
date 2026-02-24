package security

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// DangerousCommandError is returned when a dangerous command is detected
type DangerousCommandError struct {
	Command string
	Message string
}

func (e *DangerousCommandError) Error() string {
	return fmt.Sprintf("dangerous command '%s': %s", e.Command, e.Message)
}

// DangerousCommands is a list of commands that require explicit approval
var DangerousCommands = []string{
	"rm", "dd", "mkfs", "format", "shutdown",
	"reboot", "init", "kill", "killall",
	"pkill", "killall5", "halt", "poweroff",
	"fdisk", "parted", "mkfs.", "format.",
	"curl", "wget", "nc", "netcat", "telnet",
	"chmod", "chown", "chgrp",
	"passwd", "usermod", "userdel",
	"iptables", "ufw", "firewall-cmd",
}

// InjectionPatterns are patterns that indicate command injection attempts
var InjectionPatterns = []string{
	";", "|", "&", "`", "$(", "&&", "||",
	"$(ssh", "$(curl", "$(wget", "$(nc",
	"`ssh", "`curl", "`wget", "`nc",
	">/", "< /dev/",
}

// ValidateCommand validates a command string for security issues
func ValidateCommand(cmdStr string) error {
	// Check for empty command
	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return errors.New("empty command")
	}

	// Check for command injection patterns
	if containsInjectionPatterns(cmdStr) {
		return &DangerousCommandError{
			Command: cmdStr,
			Message: "command contains injection patterns",
		}
	}

	// Parse command to extract base command
	parts := parseCommand(cmdStr)
	if len(parts) == 0 {
		return errors.New("invalid command format")
	}

	baseCmd := filepath.Base(parts[0])

	// Check if it's a dangerous command
	for _, dangerous := range DangerousCommands {
		if baseCmd == dangerous || strings.HasPrefix(baseCmd, dangerous) {
			return &DangerousCommandError{
				Command: baseCmd,
				Message: "requires explicit 'dangerous: true' parameter",
			}
		}
	}

	return nil
}

// containsInjectionPatterns checks if command contains injection patterns
func containsInjectionPatterns(cmd string) bool {
	// First, check for basic shell metacharacters
	for _, pattern := range InjectionPatterns {
		if strings.Contains(cmd, pattern) {
			// Some patterns like "&&" and "||" might be legitimate in quotes
			// Do additional checking
			if pattern == "&&" || pattern == "||" {
				// Check if it's inside quotes
				if isInQuotes(cmd, strings.Index(cmd, pattern)) {
					continue
				}
			}
			return true
		}
	}

	// Check for variable substitution that might be dangerous
	if matched, _ := regexp.MatchString(`\$[a-zA-Z_][a-zA-Z0-9_]*`, cmd); matched {
		// Variable substitution could be safe, but let's be cautious
		// Only allow specific safe variables
		safeVars := []string{"$HOME", "$PATH", "$USER", "$MINDX_WORKSPACE"}
		isSafe := false
		for _, safeVar := range safeVars {
			if strings.Contains(cmd, safeVar) {
				isSafe = true
				break
			}
		}
		if !isSafe && !strings.Contains(cmd, "$(") {
			// Allow simple variables but warn
			return true
		}
	}

	return false
}

// isInQuotes checks if a position in the string is within quotes
func isInQuotes(s string, pos int) bool {
	inSingleQuote := false
	inDoubleQuote := false

	for i := 0; i < pos && i < len(s); i++ {
		switch s[i] {
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			}
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			}
		case '\\':
			// Skip escaped character
			if i+1 < len(s) {
				i++
			}
		}
	}

	return inSingleQuote || inDoubleQuote
}

// parseCommand parses a command string into parts
// This is a simplified version - the actual executor should use its own parser
func parseCommand(cmdStr string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	escapeNext := false

	for i := 0; i < len(cmdStr); i++ {
		c := cmdStr[i]

		if escapeNext {
			current.WriteByte(c)
			escapeNext = false
			continue
		}

		switch c {
		case '\\':
			escapeNext = true
		case '"':
			inQuote = !inQuote
		case ' ':
			if !inQuote {
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}
			} else {
				current.WriteByte(c)
			}
		default:
			current.WriteByte(c)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
