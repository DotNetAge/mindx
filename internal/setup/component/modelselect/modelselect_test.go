package modelselect

import (
	"strings"
	"testing"
	
	setupdata "github.com/DotNetAge/mindx/internal/setup/data"
)

func TestViewContainsGradientTitle(t *testing.T) {
	items := []setupdata.ModelItem{
		{Name: "GPT-4o", Desc: "OpenAI 最新多模态模型", BaseURL: "https://api.openai.com/v1", CredRef: "openai"},
	}
	m := New(items, true)
	m.width = 80
	m.height = 24
	
	view := m.View()
	t.Logf("\n=== View Output ===\n%s\n=== End ===", view)
	
	if !strings.Contains(view, "选择 AI 模型") {
		t.Error("View output does NOT contain '选择 AI 模型'")
	} else {
		t.Log("✅ View output contains '选择 AI 模型'")
	}
	
	if strings.Contains(view, "\x1b[") {
		t.Log("✅ View output contains ANSI escape codes")
	} else {
		t.Error("View output does NOT contain ANSI escape codes")
	}
}
