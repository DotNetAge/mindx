package client

import (
	"strings"
	"testing"
)

func TestThinkingStyleHasDarkBackground(t *testing.T) {
	testContent := "This is a thinking message"

	rendered := thinkingStyle.Render(testContent)

	if rendered == "" {
		t.Error("thinkingStyle.Render() returned empty string")
	}

	if !strings.Contains(rendered, testContent) {
		t.Errorf("Rendered output missing expected content: %q\nGot: %q", testContent, rendered)
	}

	if !strings.Contains(rendered, "48;2;45;45;45") {
		t.Errorf("Rendered output missing dark background color code (#2d2d2d)\nGot: %q", rendered)
	}
}

func TestThinkingStyleConfiguration(t *testing.T) {
	testContent := "test"
	rendered := thinkingStyle.Render(testContent)

	if rendered == "" {
		t.Error("thinkingStyle.Render() returned empty string, style may not be configured")
	}
}
