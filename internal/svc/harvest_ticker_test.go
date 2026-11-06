package svc

import (
	"strings"
	"testing"

	"github.com/DotNetAge/goharness/tools"
)

// TestFormatHarvestReportTruncatesResult 回灌报告中超长子任务结果被截断：
// 完整交付全文滞留主会话历史是持久性 token 开销，超限截断并引导按会话
// ID 查看完整内容。
func TestFormatHarvestReportTruncatesResult(t *testing.T) {
	long := strings.Repeat("长", harvestResultMaxChars+100)
	report := tools.HarvestReport{
		MainSessionID: "main-1",
		Targets: []tools.HarvestStatus{
			{TaskID: "t1", SessionID: "sub-1", AgentName: "researcher", Status: "completed", Result: long},
		},
	}
	out := formatHarvestReport(report)

	runes := []rune(out)
	if len(runes) > harvestResultMaxChars+200 {
		t.Fatalf("回灌正文应被截断至 ~%d 字符附近，实际 %d", harvestResultMaxChars, len(runes))
	}
	if !strings.Contains(out, "已截断") || !strings.Contains(out, "sub-1") {
		t.Fatalf("截断提示与会话锚点缺失:\n%s", out[:200])
	}
}

// TestFormatHarvestReportFailedGuidance 失败条目必须含下一步处理方案
// （异常回执军规：LLM 看不到处理方案会自行原样重派）。
func TestFormatHarvestReportFailedGuidance(t *testing.T) {
	report := tools.HarvestReport{
		MainSessionID: "main-1",
		Targets: []tools.HarvestStatus{
			{TaskID: "t1", SessionID: "sub-1", AgentName: "coder", Status: "failed", Error: "迭代超限"},
		},
	}
	out := formatHarvestReport(report)
	for _, want := range []string{"继续", "重试", "重新派发"} {
		if !strings.Contains(out, want) {
			t.Fatalf("失败条目缺少处理方案关键词 %q:\n%s", want, out)
		}
	}
}
