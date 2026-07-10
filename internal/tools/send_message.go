package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/DotNetAge/goharness/tools"
)

// SendMessage sends a notification to the macOS Notification Center.
// This tool is only supported on macOS — it returns an error on other platforms.
type SendMessage struct{}

// NewSendMessage creates a SendMessage tool.
func NewSendMessage() tools.FuncTool {
	return &SendMessage{}
}

func (t *SendMessage) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:        "SendMessage",
		Description: "Send a notification to the macOS Notification Center. Only registered on macOS.",
		Prompt: `Send a notification to the macOS Notification Center using osascript. This tool is only available on macOS.

Use this to alert the user about long-running task completions, important updates, or when input is needed.

The notification appears as a standard macOS notification banner/alert. The subtitle field is optional but helps organize related notifications.

Examples:
- Notify when a background task completes: title="Build Complete", message="The project build finished successfully"
- Remind about updates: title="Daily Report", message="Your daily summary is ready", subtitle="Scheduled Task"

Only supported on macOS. Returns an error on Linux or Windows.`,
		IsReadOnly: true,
		Parameters: []tools.Parameter{
			{
				Name:        "title",
				Type:        "string",
				Description: "Notification title (bold text at the top of the notification).",
				Required:    true,
			},
			{
				Name:        "message",
				Type:        "string",
				Description: "Notification body text (the main message content).",
				Required:    true,
			},
			{
				Name:        "subtitle",
				Type:        "string",
				Description: "Optional subtitle text (smaller text below the title).",
				Required:    false,
			},
		},
	}
}

func (t *SendMessage) Execute(ctx context.Context, params map[string]any) (any, error) {
	title, err := tools.ValidateRequiredString(params, "title")
	if err != nil {
		return nil, fmt.Errorf("SendMessage: title is required: %w", err)
	}

	message, err := tools.ValidateRequiredString(params, "message")
	if err != nil {
		return nil, fmt.Errorf("SendMessage: message is required: %w", err)
	}

	subtitle, _ := params["subtitle"].(string)

	// Build osascript command
	var script strings.Builder
	script.WriteString("display notification ")
	script.WriteString(escapeAppleScriptString(message))
	script.WriteString(" with title ")
	script.WriteString(escapeAppleScriptString(title))
	if subtitle != "" {
		script.WriteString(" subtitle ")
		script.WriteString(escapeAppleScriptString(subtitle))
	}

	cmd := exec.CommandContext(ctx, "osascript", "-e", script.String())
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("SendMessage: failed to send notification: %w", err)
	}

	return map[string]any{
		"title":   title,
		"message": message,
		"sent":    true,
	}, nil
}

// escapeAppleScriptString wraps s in double quotes and escapes special characters
// for use in AppleScript string literals.
func escapeAppleScriptString(s string) string {
	escaped := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"\n", "\\n",
		"\r", "\\r",
		"\t", "\\t",
	).Replace(s)
	return `"` + escaped + `"`
}
