package modelselect

import (
	"regexp"
	"strings"
	"testing"

	"github.com/DotNetAge/mindx/internal/i18n"
	setupdata "github.com/DotNetAge/mindx/internal/setup/data"
)

func stripANSI(s string) string {
	re := regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")
	return re.ReplaceAllString(s, "")
}

func TestViewContainsGradientTitle(t *testing.T) {
	items := []setupdata.ModelItem{
		{Name: "GPT-4o", Desc: "OpenAI 最新多模态模型", BaseURL: "https://api.openai.com/v1", CredRef: "openai"},
	}
	m := New(items, true)
	m.width = 80
	m.height = 24

	view := m.View()
	t.Logf("\n=== View Output ===\n%s\n=== End ===", view)

	plain := stripANSI(view)
	if !strings.Contains(plain, i18n.T("setup.model.select.title")) {
		t.Error("View output does NOT contain '选择一个 AI 模型'")
	} else {
		t.Log("✅ View output contains '选择一个 AI 模型'")
	}

	if strings.Contains(view, "\x1b[") {
		t.Log("✅ View output contains ANSI escape codes")
	} else {
		t.Error("View output does NOT contain ANSI escape codes")
	}
}
