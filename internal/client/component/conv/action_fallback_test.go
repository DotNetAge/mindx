package conv

import (
	"strings"
	"testing"
)

// formatter 返回空时必须兜底展示原始结果，不允许静默吞掉。
func TestViewStepResultFallbackOnEmptyFormatter(t *testing.T) {
	RegisterResultFormatter("FakeTool", func(_ ActionStep, _ string, _ int) string { return "" })
	defer delete(resultFormatters, "FakeTool")

	step := ActionStep{
		ToolName: "FakeTool",
		Status:   ActionStepDone,
		Result:   `{"success": true, "items": [{"name": "a"}]}`,
	}
	out := viewStepResult(step, 80)
	if !strings.Contains(out, `"items"`) {
		t.Errorf("raw JSON must survive fallback rendering, got:\n%s", out)
	}
	if !strings.Contains(out, "⎿") {
		t.Errorf("fallback output should carry result prefix, got:\n%s", out)
	}
}

// 未注册 formatter 的工具同样走原始文本兜底。
func TestViewStepResultFallbackWithoutFormatter(t *testing.T) {
	step := ActionStep{
		ToolName: "UnregisteredTool",
		Status:   ActionStepDone,
		Result:   "plain text result\nsecond line",
	}
	out := viewStepResult(step, 80)
	for _, want := range []string{"plain text result", "second line"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in fallback output:\n%s", want, out)
		}
	}
}

// 超长原始结果截断，防止兜底路径把界面冲爆。
func TestFallbackResultPreviewTruncates(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "line"
	}
	out := fallbackResultPreview(strings.Join(lines, "\n"))
	// "… +80 lines" 提示行本身含 "lines" 一词，排除后统计正文行。
	if got := strings.Count(strings.ReplaceAll(out, "+80 lines", ""), "line"); got != 20 {
		t.Errorf("expected 20 shown lines, got %d", got)
	}
	if !strings.Contains(out, "+80 lines") {
		t.Errorf("missing truncation notice, got:\n%s", out)
	}
}
