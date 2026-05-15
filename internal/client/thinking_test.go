package client

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestBlinkAnimationDuringThinking(t *testing.T) {
	answer := NewAgentAnswer("test-session", "test-agent")
	answer.StartThinking()

	view1 := answer.View()
	if !strings.Contains(view1, "深度思考中") {
		t.Error("Should show '深度思考中' when thinking starts")
	}

	cmd := answer.Tick()
	if cmd == nil {
		t.Error("Tick should return command when isThinking=true")
	}

	view2 := answer.View()
	if view1 == view2 {
		t.Error("View should change after Tick (blink state toggled)")
	}

	for i := 0; i < 10; i++ {
		cmd = answer.Tick()
		if cmd == nil {
			t.Errorf("Tick %d: should still return command while thinking", i)
		}
	}

	answer.SetThinkingDone("")
	cmd = answer.Tick()
	if cmd != nil {
		t.Error("Tick should return nil after thinking done (no action executing)")
	}
}

func TestBlinkAnimationDuringToolExecution(t *testing.T) {
	answer := NewAgentAnswer("test-session", "test-agent")
	answer.AppendAction("Bash", 10000, map[string]any{"command": "echo test"})

	cmd := answer.Tick()
	if cmd == nil {
		t.Error("Tick should return command when tool is executing")
	}

	viewWithWhite := answer.View()
	if !strings.Contains(viewWithWhite, "Bash") {
		t.Error("Should show tool name when executing")
	}

	answer.Tick()

	viewWithGreen := answer.View()
	if viewWithWhite == viewWithGreen {
		t.Error("View should change after Tick (blink color toggled)")
	}

	answer.MarkActionDone("output result")
	cmd = answer.Tick()
	if cmd != nil {
		t.Error("Tick should return nil after all actions completed and not thinking")
	}
}

func TestBlinkAnimationStopsWhenAllComplete(t *testing.T) {
	answer := NewAgentAnswer("test-session", "test-agent")
	answer.StartThinking()
	answer.AppendAction("Tool1", 5000, nil)
	answer.AppendAction("Tool2", 8000, nil)

	var cmd tea.Cmd
	for i := 0; i < 5; i++ {
		cmd = answer.Tick()
		if cmd == nil {
			t.Errorf("Tick %d: should return command while thinking + actions executing", i)
		}
	}

	answer.MarkActionDone("result1")

	for i := 0; i < 5; i++ {
		cmd = answer.Tick()
		if cmd == nil {
			t.Errorf("Tick %d (after action1 done): should return command while thinking + action2 executing", i)
		}
	}

	answer.SetThinkingDone("")

	for i := 0; i < 5; i++ {
		cmd = answer.Tick()
		if cmd == nil {
			t.Errorf("Tick %d (after thinking done): should return command while action2 still executing", i)
		}
	}

	answer.MarkActionDone("result2")

	cmd = answer.Tick()
	if cmd != nil {
		t.Error("Tick should return nil when everything is complete")
	}
}

func TestBlinkColorAlternation(t *testing.T) {
	answer := NewAgentAnswer("test-session", "test-agent")
	answer.StartThinking()

	var whiteCount, blueCount int
	for i := 0; i < 20; i++ {
		view := answer.View()
		if strings.Contains(view, "#E0E0E0") || (i%2 == 0 && !answer.blinkOn) {
			whiteCount++
		} else {
			blueCount++
		}
		answer.Tick()
	}

	if whiteCount == 0 || blueCount == 0 {
		t.Errorf("Should alternate between white (%d) and blue (%d) during thinking", whiteCount, blueCount)
	}
}

func TestTickIntegration(t *testing.T) {
	answer := NewAgentAnswer("test-session", "test-agent")
	answer.StartThinking()

	msg := time.Now()
	cmd := answer.Update(msg)
	if cmd == nil {
		t.Error("Update with time.Time should call Tick and return command when thinking")
	}
}

func TestStreamThroughThinkingDisplay(t *testing.T) {
	answer := NewAgentAnswer("test-session", "test-agent")
	answer.StartThinking()

	thoughtText := "用户让我总结之前的搜索结果，我要分析电商行业的影响"

	answer.AppendThinking(thoughtText)

	view := answer.View()
	if !strings.Contains(view, thoughtText) {
		t.Error("流式直通：View 应该包含完整的思想流内容，got:", view)
	}
	if !strings.Contains(view, "●") {
		t.Error("View should show ● icon for thinking content")
	}
}

func TestAllContentDisplayedWithoutFilter(t *testing.T) {
	answer := NewAgentAnswer("test-session", "test-agent")
	answer.StartThinking()

	testCases := []string{
		"LLM returned native tool calls",
		"Decision: act",
		"Thought: something",
		`{"tool_name":"Bash"}`,
		"normal text",
	}

	for _, content := range testCases {
		answer.AppendThinking(content)
	}

	view := answer.View()
	for _, content := range testCases {
		if !strings.Contains(view, content) {
			t.Errorf("应该显示所有内容（不过滤）: %q", content)
		}
	}
}

func TestMultipleStreamChunksAccumulation(t *testing.T) {
	answer := NewAgentAnswer("test-session", "test-agent")
	answer.StartThinking()

	chunks := []string{
		"用户让我",
		"总结之前的",
		"搜索结果，",
		"我要分析",
		"电商行业的影响",
	}
	for _, chunk := range chunks {
		answer.AppendThinking(chunk)
	}

	expected := "用户让我总结之前的搜索结果，我要分析电商行业的影响"
	view := answer.View()
	if !strings.Contains(view, expected) {
		t.Error("多个流式chunk应该累积成完整内容，expected:", expected, "got:", view)
	}
}

func TestThinkingRoundsPreserved(t *testing.T) {
	answer := NewAgentAnswer("test-session", "test-agent")

	answer.StartThinking()
	answer.AppendThinking("第一轮思考内容")
	answer.SetThinkingDone("")

	answer.StartThinking()
	answer.AppendThinking("第二轮思考内容")
	answer.SetThinkingDone("")

	view := answer.View()
	if !strings.Contains(view, "第一轮思考内容") {
		t.Error("应该保留第一轮思考内容（来自 ThinkingDelta）")
	}
	if !strings.Contains(view, "第二轮思考内容") {
		t.Error("应该保留第二轮思考内容（来自 ThinkingDelta）")
	}
}

func TestRealTimeStreamDisplay(t *testing.T) {
	answer := NewAgentAnswer("test-session", "test-agent")
	answer.StartThinking()

	chunks := []string{
		"用户让我",
		"总结之前的",
		"搜索结果，",
		"我要分析",
		"电商行业的影响",
	}

	for i, chunk := range chunks {
		answer.AppendThinking(chunk)

		view := answer.View()
		expected := strings.Join(chunks[:i+1], "")
		if !strings.Contains(view, expected) {
			t.Errorf("实时显示：第 %d 个 chunk 后应该包含 '%s', got: %s", i+1, expected, view)
		}
	}
}

func TestThinkingDoneOnlySavesAccumulatedContent(t *testing.T) {
	answer := NewAgentAnswer("test-session", "test-agent")
	answer.StartThinking()

	answer.AppendThinking("这是来自 Delta 的内容")

	thoughtJSON := `{
		"reasoning": "这不应该被显示",
		"decision": "act",
		"tool_calls": {}
	}`

	answer.SetThinkingDone(thoughtJSON)

	if len(answer.thinkingRounds) != 1 {
		t.Errorf("应该只有 1 轮思考（来自 Delta），got: %d", len(answer.thinkingRounds))
	}

	if answer.thinkingRounds[0] != "这是来自 Delta 的内容" {
		t.Errorf("应该是 Delta 的内容，got: %s", answer.thinkingRounds[0])
	}

	view := answer.View()
	if strings.Contains(view, "这不应该被显示") {
		t.Error("不应该显示 ThoughtDone JSON 中的 reasoning 字段")
	}
	if strings.Contains(view, `"decision"`) {
		t.Error("不应该显示 ThoughtDone 的任何内部字段")
	}
}

func TestFullRealTimeDataChain(t *testing.T) {
	answer := NewAgentAnswer("test-session", "test-agent")

	answer.StartThinking()

	view1 := answer.View()
	if !strings.Contains(view1, "深度思考中") {
		t.Error("等待第一个 Delta 时应该显示'深度思考中'")
	}

	answer.AppendThinking("用户让我")

	view2 := answer.View()
	if strings.Contains(view2, "深度思考中") {
		t.Error("收到 Delta 后不应再显示'深度思考中'")
	}
	if !strings.Contains(view2, "用户让我") {
		t.Error("应该直接显示 Delta 内容")
	}

	answer.AppendThinking("总结搜索结果")

	view3 := answer.View()
	if !strings.Contains(view3, "用户让我总结搜索结果") {
		t.Error("应该累积显示所有 Delta 内容")
	}

	answer.SetThinkingDone(`{"reasoning":"...","decision":"act"}`)

	view4 := answer.View()
	if !strings.Contains(view4, "用户让我总结搜索结果") {
		t.Error("Done 后仍应显示 Delta 累积的内容")
	}
	if strings.Contains(view4, `"reasoning"`) {
		t.Error("Done 后不应显示 Thought 结构体的任何字段")
	}

	answer.AppendAction("Bash", 10000, nil)

	view5 := answer.View()
	if !strings.Contains(view5, "Bash") {
		t.Error("应该显示工具调用")
	}
}
