package client

import (
	"strings"
	"testing"
)

func TestThinkingDoneReplacesRawContent(t *testing.T) {
	answer := NewAgentAnswer("test-session", "test-agent")

	rawJSON := `{ "decision": "clarify", "reasoning": "Need clarification" }`
	formattedMarkdown := "### 思考完成\n\n**决策**: `clarify`  **置信度**: 0%\n\n**推理**: Need clarification"

	// 1. 追加原始JSON（模拟流式 thinking_delta）
	answer.AppendThinking(rawJSON)

	// 2. 验证此时显示的是原始内容
	viewBefore := answer.View()
	if !strings.Contains(viewBefore, rawJSON) {
		t.Error("Before SetThinkingDone: view should contain raw JSON")
	}
	if strings.Contains(viewBefore, "思考完成") {
		t.Error("Before SetThinkingDone: view should NOT contain formatted content")
	}

	// 3. 设置格式化内容（模拟 thinking_done）
	answer.SetThinkingDone(formattedMarkdown)

	// 4. 验证现在只显示格式化内容
	viewAfter := answer.View()
	if strings.Contains(viewAfter, rawJSON) {
		t.Error("After SetThinkingDone: view should NOT contain raw JSON")
	}
	if !strings.Contains(viewAfter, "思考完成") {
		t.Error("After SetThinkingDone: view should contain formatted content")
	}
	if !strings.Contains(viewAfter, "clarify") {
		t.Error("After SetThinkingDone: view should contain decision info")
	}
}

func TestOnlyRawThinkingWhenNoDone(t *testing.T) {
	answer := NewAgentAnswer("test-session", "test-agent")

	rawContent := "Thinking about the problem..."

	answer.AppendThinking(rawContent)
	view := answer.View()

	if !strings.Contains(view, rawContent) {
		t.Error("View should contain raw thinking content when no thinking_done is set")
	}
}

func TestMultipleThinkingDeltaAccumulation(t *testing.T) {
	answer := NewAgentAnswer("test-session", "test-agent")

	// 模拟多个流式delta
	answer.AppendThinking(`{ "decision":`)
	answer.AppendThinking(` "clarify",`)
	answer.AppendThinking(` "reasoning": "test" }`)

	viewBefore := answer.View()
	if !strings.Contains(viewBefore, `"clarify"`) {
		t.Error("Should accumulate multiple thinking deltas")
	}

	// 设置完成内容
	answer.SetThinkingDone("### Formatted Result\n**Decision**: clarify")

	viewAfter := answer.View()
	if strings.Contains(viewAfter, `{`) {
		t.Error("After thinking_done, raw JSON should be hidden")
	}
	if !strings.Contains(viewAfter, "Formatted Result") {
		t.Error("Should show formatted content after thinking_done")
	}
}
