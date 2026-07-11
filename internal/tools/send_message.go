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
		Description: "向 macOS 通知中心发送通知。仅在 macOS 上可用。",
		Prompt: `使用 osascript 向 macOS 通知中心发送通知。此工具仅在 macOS 上可用。

通知显示为标准的 macOS 通知横幅/提醒。副标题字段可选，有助于组织相关通知。`,
		IsReadOnly: true,
		Parameters: []tools.Parameter{
			{
				Name:        "title",
				Type:        "string",
				Description: "通知标题（通知顶部的粗体文本）。",
				Required:    true,
			},
			{
				Name:        "message",
				Type:        "string",
				Description: "通知正文（主要消息内容）。",
				Required:    true,
			},
			{
				Name:        "subtitle",
				Type:        "string",
				Description: "可选副标题（标题下方较小的文本）。",
				Required:    false,
			},
		},
	}
}

func (t *SendMessage) Execute(ctx context.Context, params map[string]any) (any, error) {
	title, err := tools.ValidateRequiredString(params, "title")
	if err != nil {
		return nil, fmt.Errorf("SendMessage：title 为必填项：%w", err)
	}

	message, err := tools.ValidateRequiredString(params, "message")
	if err != nil {
		return nil, fmt.Errorf("SendMessage：message 为必填项：%w", err)
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
		return nil, fmt.Errorf("SendMessage：发送通知失败：%w", err)
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
