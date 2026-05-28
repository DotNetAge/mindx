package conv

import (
	"errors"
	"strings"
	"testing"

	"github.com/DotNetAge/mindx/internal/client/msg"
)

func TestQuestionView(t *testing.T) {
	m := Question{Text: "What is Go?"}
	view := ViewQuestion(m, 80)

	if !strings.Contains(view, "What is Go?") {
		t.Error("expected question text in view")
	}
}

func TestQuestionViewEmpty(t *testing.T) {
	m := Question{}
	view := ViewQuestion(m, 80)

	if view != "" {
		t.Error("expected empty view for empty question")
	}
}

func TestThoughtView(t *testing.T) {
	view := ViewThought("thinking...", 0, 0, false, "")

	if !strings.Contains(view, "thinking...") {
		t.Error("expected thought content in view")
	}
}

func TestActionUpdateStartExecEnd(t *testing.T) {
	m := Action{}
	m, _ = UpdateAction(m, msg.ToolExecStartMsg{
		SessionID: "s1",
		ToolName:  "bash",
		Params:    map[string]any{"cmd": "ls"},
	})

	if len(m.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(m.Steps))
	}
	if m.Steps[0].ToolName != "bash" {
		t.Errorf("expected ToolName 'bash', got %q", m.Steps[0].ToolName)
	}
	if m.Steps[0].Status != ActionStepExecuting {
		t.Errorf("expected ActionStepExecuting, got %v", m.Steps[0].Status)
	}
	if !m.Steps[0].Collapsed {
		t.Error("expected Collapsed=true by default")
	}

	m, _ = UpdateAction(m, msg.ToolExecEndMsg{
		SessionID: "s1",
		ToolName:  "bash",
		Success:   true,
		Result:    "command output",
	})

	if m.Steps[0].Status != ActionStepDone {
		t.Errorf("expected ActionStepDone, got %v", m.Steps[0].Status)
	}
	if m.Steps[0].ResultText != "command output" {
		t.Errorf("expected ResultText %q, got %q", "command output", m.Steps[0].ResultText)
	}
}

func TestActionView(t *testing.T) {
	m := Action{
		Steps: []ActionStep{
			{ToolName: "bash", Status: ActionStepDone, ResultText: "done"},
		},
		Completed: true,
	}
	view := ViewAction(m, 80)

	if !strings.Contains(view, "bash") {
		t.Error("expected tool name in view")
	}
}

func TestOutputUpdateFinalAnswer(t *testing.T) {
	m := Output{}
	m, _ = UpdateOutput(m, msg.FinalAnswerMsg{SessionID: "s1", Content: "Go is a programming language"})

	if len(m.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(m.Entries))
	}
	if m.Entries[0].Role != "assistant" {
		t.Errorf("expected Role 'assistant', got %q", m.Entries[0].Role)
	}
	if m.Entries[0].Content != "Go is a programming language" {
		t.Errorf("expected content %q, got %q", "Go is a programming language", m.Entries[0].Content)
	}
}

func TestOutputUpdateError(t *testing.T) {
	m := Output{}
	m, _ = UpdateOutput(m, msg.AgentErrorMsg{SessionID: "s1", Error: errors.New("something went wrong")})

	if len(m.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(m.Entries))
	}
	if m.Entries[0].Role != "error" {
		t.Errorf("expected Role 'error', got %q", m.Entries[0].Role)
	}
}

func TestOutputUpdateDeduplicate(t *testing.T) {
	m := Output{}
	m, _ = UpdateOutput(m, msg.FinalAnswerMsg{SessionID: "s1", Content: "answer"})
	m, _ = UpdateOutput(m, msg.FinalAnswerMsg{SessionID: "s1", Content: "answer"})

	if len(m.Entries) != 1 {
		t.Errorf("expected 1 entry (dedup), got %d", len(m.Entries))
	}
}

func TestConversationFullFlow(t *testing.T) {
	conv := NewConversation("s1", "agent1", "What is Go?")

	if conv.Question.Text != "What is Go?" {
		t.Errorf("expected question %q, got %q", "What is Go?", conv.Question.Text)
	}
	if conv.Status != StatusThinking {
		t.Errorf("expected StatusThinking, got %v", conv.Status)
	}

	conv, _ = UpdateConversation(conv, msg.ThinkingDeltaMsg{SessionID: "s1", Content: "I need to think..."})
	conv, _ = UpdateConversation(conv, msg.ThinkingDoneMsg{SessionID: "s1", Content: "I need to think..."})

	if len(conv.Rounds) != 1 {
		t.Fatalf("expected 1 round after thinking, got %d", len(conv.Rounds))
	}
	if conv.Rounds[0].ThoughtContent != "I need to think..." {
		t.Errorf("expected thought content %q, got %q", "I need to think...", conv.Rounds[0].ThoughtContent)
	}

	conv, _ = UpdateConversation(conv, msg.ToolExecStartMsg{SessionID: "s1", ToolName: "bash"})
	conv, _ = UpdateConversation(conv, msg.ToolExecEndMsg{SessionID: "s1", ToolName: "bash", Success: true, Result: "done"})

	if len(conv.Rounds) != 1 {
		t.Fatalf("expected 1 round after tool exec, got %d", len(conv.Rounds))
	}

	conv, _ = UpdateConversation(conv, msg.FinalAnswerMsg{SessionID: "s1", Content: "Go is a programming language"})
	conv, _ = UpdateConversation(conv, msg.SessionDoneMsg{SessionID: "s1"})

	if conv.Status != StatusDone {
		t.Errorf("expected StatusDone, got %v", conv.Status)
	}
}

func TestConversationMultiRoundFlow(t *testing.T) {
	conv := NewConversation("s1", "agent1", "Complex task?")

	conv, _ = UpdateConversation(conv, msg.ThinkingDeltaMsg{SessionID: "s1", Content: "First thought..."})
	conv, _ = UpdateConversation(conv, msg.ThinkingDoneMsg{SessionID: "s1", Content: "First thought..."})
	conv, _ = UpdateConversation(conv, msg.ToolExecStartMsg{SessionID: "s1", ToolName: "tool1"})
	conv, _ = UpdateConversation(conv, msg.ToolExecEndMsg{SessionID: "s1", ToolName: "tool1", Success: true, Result: "ok"})

	conv, _ = UpdateConversation(conv, msg.ThinkingDeltaMsg{SessionID: "s1", Content: "Second thought..."})
	conv, _ = UpdateConversation(conv, msg.ThinkingDoneMsg{SessionID: "s1", Content: "Second thought..."})
	conv, _ = UpdateConversation(conv, msg.ToolExecStartMsg{SessionID: "s1", ToolName: "tool2"})
	conv, _ = UpdateConversation(conv, msg.ToolExecEndMsg{SessionID: "s1", ToolName: "tool2", Success: true, Result: "done"})

	if len(conv.Rounds) != 2 {
		t.Fatalf("expected 2 rounds, got %d", len(conv.Rounds))
	}
	if conv.Rounds[0].ThoughtContent != "First thought..." {
		t.Errorf("expected round 0 thought %q, got %q", "First thought...", conv.Rounds[0].ThoughtContent)
	}
	if conv.Rounds[1].ThoughtContent != "Second thought..." {
		t.Errorf("expected round 1 thought %q, got %q", "Second thought...", conv.Rounds[1].ThoughtContent)
	}
}

func TestConversationViewFullTAO(t *testing.T) {
	conv := NewConversation("s1", "agent1", "What is Go?")
	conv.Rounds = append(conv.Rounds, ConversationRound{
		ThoughtContent: "thinking step 1...",
		Action: Action{
			Steps:     []ActionStep{{ToolName: "bash", Status: ActionStepDone, ResultText: "done"}},
			Completed: true,
		},
	})
	conv.Output = Output{
		Entries: []OutputEntry{{Role: "assistant", Content: "Go is a programming language"}},
	}

	view := ViewConversation(conv, 80)
	if view == "" {
		t.Fatal("expected non-empty view for full T-A-O conversation")
	}
	if !strings.Contains(view, "What is Go?") {
		t.Error("expected question in view")
	}
	if !strings.Contains(view, "thinking step 1...") {
		t.Error("expected thinking content in view")
	}
	if !strings.Contains(view, "bash") {
		t.Error("expected tool name in view")
	}
}

func TestConversationViewEmpty(t *testing.T) {
	conv := Conversation{
		SessionID: "s1",
		Status:    StatusThinking,
	}
	view := ViewConversation(conv, 80)

	if view != "" {
		t.Error("expected empty view for empty conversation")
	}
}

func TestConversationDoneBlocksUpdates(t *testing.T) {
	conv := NewConversation("s1", "agent1", "hello")
	conv, _ = UpdateConversation(conv, msg.SessionDoneMsg{SessionID: "s1"})

	if conv.Status != StatusDone {
		t.Fatalf("expected StatusDone, got %v", conv.Status)
	}

	conv, _ = UpdateConversation(conv, msg.FinalAnswerMsg{SessionID: "s1", Content: "should be ignored"})

	if len(conv.Output.Entries) != 0 {
		t.Error("expected no output after done")
	}
}

func TestClearScreen(t *testing.T) {
	list := NewConversationList()
	list.Conversations = append(list.Conversations, NewConversation("s1", "agent1", "hello"))

	if len(list.Conversations) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(list.Conversations))
	}

	list, _ = list.Update(msg.ClearScreenMsg{})

	if len(list.Conversations) != 0 {
		t.Errorf("expected 0 conversations after clear, got %d", len(list.Conversations))
	}
}

func TestConversationMaxTurnsReached(t *testing.T) {
	conv := NewConversation("s1", "agent1", "复杂任务")

	conv, _ = UpdateConversation(conv, msg.ThinkingDeltaMsg{SessionID: "s1", Content: "分析中..."})
	conv, _ = UpdateConversation(conv, msg.ThinkingDoneMsg{SessionID: "s1", Content: "分析中..."})

	if conv.Status != StatusThinking {
		t.Fatalf("expected StatusThinking before MaxTurnsReached, got %v", conv.Status)
	}

	conv, _ = UpdateConversation(conv, msg.MaxTurnsReachedMsg{
		SessionID:      "s1",
		TurnsCompleted: 20,
		MaxTurns:       20,
		Suggestion:     "已达到最大思考轮次 (20/20)。你可以发送\"继续\"让 AI 继续。",
	})

	if conv.Status != StatusDone {
		t.Errorf("expected StatusDone after MaxTurnsReached, got %v", conv.Status)
	}
	if conv.MaxTurnsNotice == "" {
		t.Error("expected MaxTurnsNotice to be set")
	}
	if !strings.Contains(conv.MaxTurnsNotice, "20/20") {
		t.Errorf("expected notice to contain turn info, got %q", conv.MaxTurnsNotice)
	}
	if conv.Error.Error != "" {
		t.Errorf("expected no error for MaxTurnsReached, got %q", conv.Error.Error)
	}
}

func TestConversationMaxTurnsReachedView(t *testing.T) {
	conv := NewConversation("s1", "agent1", "为什么天空是蓝色的？")

	conv, _ = UpdateConversation(conv, msg.MaxTurnsReachedMsg{
		SessionID:      "s1",
		TurnsCompleted: 20,
		MaxTurns:       20,
		Suggestion:     "已达到最大思考轮次 (20/20)，任务可能需要更详细的指令。",
	})

	view := ViewConversation(conv, 80)
	if view == "" {
		t.Fatal("expected non-empty view for MaxTurnsReached conversation")
	}
	if !strings.Contains(view, "为什么天空是蓝色的？") {
		t.Error("expected question in view")
	}
	if !strings.Contains(view, "20/20") {
		t.Errorf("expected turn info in view, got:\n%s", view)
	}
	if !strings.Contains(view, "💡") {
		t.Error("expected lightbulb icon in MaxTurnsNotice view")
	}
}
