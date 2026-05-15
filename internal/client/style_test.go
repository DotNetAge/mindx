package client

import (
	"strings"
	"testing"
)

func TestThinkingStyleHasItalicAndColor(t *testing.T) {
	testContent := "This is a thinking message"

	rendered := ThinkingStyle.Render(testContent)

	if rendered == "" {
		t.Error("ThinkingStyle.Render() returned empty string")
	}

	if !strings.Contains(rendered, testContent) {
		t.Errorf("Rendered output missing expected content: %q\nGot: %q", testContent, rendered)
	}

	if !strings.Contains(rendered, "[3;") {
		t.Errorf("Rendered output missing italic style code\nGot: %q", rendered)
	}
	if !strings.Contains(rendered, "38;2;136;136;136") {
		t.Errorf("Rendered output missing foreground color (#888888)\nGot: %q", rendered)
	}
}

func TestThinkingStyleConfiguration(t *testing.T) {
	testContent := "test"
	rendered := ThinkingStyle.Render(testContent)

	if rendered == "" {
		t.Error("ThinkingStyle.Render() returned empty string, style may not be configured")
	}
}
