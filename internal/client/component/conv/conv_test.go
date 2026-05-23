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

func TestThoughtUpdateDelta(t *testing.T) {
	m := Thought{IsActive: true}
	m, _ = UpdateThought(m, msg.ThinkingDeltaMsg{SessionID: "s1", Content: "thinking step 1..."})

	if m.Pending != "thinking step 1..." {
		t.Errorf("expected Pending %q, got %q", "thinking step 1...", m.Pending)
	}
}

func TestThoughtUpdateDone(t *testing.T) {
	m := Thought{IsActive: true, Content: "thinking step 1..."}
	m, _ = UpdateThought(m, msg.ThinkingDoneMsg{SessionID: "s1"})

	if m.Content != "thinking step 1..." {
		t.Errorf("expected content %q, got %q", "thinking step 1...", m.Content)
	}
	if m.IsActive {
		t.Error("expected IsActive=false")
	}
}

func TestThoughtView(t *testing.T) {
	m := Thought{
		IsActive: true,
		Content:  "thinking...",
	}
	view := ViewThought(m)

	if !strings.Contains(view, "thinking...") {
		t.Error("expected thought content in view")
	}
}

func TestActionUpdateStartExecEnd(t *testing.T) {
	m := Action{}
	m, _ = UpdateAction(m, msg.ActionStartMsg{
		SessionID:    "s1",
		ToolCount:    1,
		ToolNames:    []string{"bash"},
		EstimatedTok: 100,
	})

	if m.CurrentInfo == nil {
		t.Fatal("expected CurrentInfo after ActionStart")
	}
	if m.CurrentInfo.ToolCount != 1 {
		t.Errorf("expected ToolCount 1, got %d", m.CurrentInfo.ToolCount)
	}

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

func TestActionUpdateEnd(t *testing.T) {
	m := Action{
		Steps: []ActionStep{
			{ToolName: "bash", Status: ActionStepDone},
		},
		CurrentInfo: &ActionInfo{ToolCount: 1, ToolNames: []string{"bash"}},
	}
	m, _ = UpdateAction(m, msg.ActionEndMsg{
		SessionID:    "s1",
		TotalTools:   1,
		SuccessCount: 1,
		FailedCount:  0,
	})

	if !m.Completed {
		t.Error("expected Completed=true")
	}
	if m.SuccessCount != 1 {
		t.Errorf("expected SuccessCount 1, got %d", m.SuccessCount)
	}
}

func TestActionView(t *testing.T) {
	m := Action{
		CurrentInfo: &ActionInfo{ToolCount: 1, ToolNames: []string{"bash"}},
		Steps: []ActionStep{
			{ToolName: "bash", Status: ActionStepDone, ResultText: "done"},
		},
		Completed: true,
	}
	view := ViewAction(m, 80)

	if !strings.Contains(view, "执行操作") {
		t.Error("expected action header in view")
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
	conv, _ = UpdateConversation(conv, msg.ThinkingDoneMsg{SessionID: "s1", Reasoning: "I need to think..."})

	// Rounds are created by thinking events to capture Thought content
	if len(conv.Rounds) != 1 {
		t.Fatalf("expected 1 round after thinking (to capture thought content), got %d", len(conv.Rounds))
	}
	if conv.Rounds[0].Thought.Content != "I need to think..." {
		t.Errorf("expected thought content %q, got %q", "I need to think...", conv.Rounds[0].Thought.Content)
	}

	conv, _ = UpdateConversation(conv, msg.ActionStartMsg{SessionID: "s1", ToolCount: 1, ToolNames: []string{"bash"}})
	conv, _ = UpdateConversation(conv, msg.ToolExecStartMsg{SessionID: "s1", ToolName: "bash"})
	conv, _ = UpdateConversation(conv, msg.ToolExecEndMsg{SessionID: "s1", ToolName: "bash", Success: true, Result: "done"})
	conv, _ = UpdateConversation(conv, msg.ActionEndMsg{SessionID: "s1", SuccessCount: 1, FailedCount: 0})

	if len(conv.Rounds) != 1 {
		t.Fatalf("expected 1 round after action, got %d", len(conv.Rounds))
	}
	if !conv.Rounds[0].Action.Completed {
		t.Error("expected action completed in current round")
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
	conv, _ = UpdateConversation(conv, msg.ThinkingDoneMsg{SessionID: "s1", Reasoning: "First thought..."})
	conv, _ = UpdateConversation(conv, msg.ActionStartMsg{SessionID: "s1", ToolCount: 1, ToolNames: []string{"tool1"}})
	conv, _ = UpdateConversation(conv, msg.ToolExecStartMsg{SessionID: "s1", ToolName: "tool1"})
	conv, _ = UpdateConversation(conv, msg.ToolExecEndMsg{SessionID: "s1", ToolName: "tool1", Success: true, Result: "ok"})
	conv, _ = UpdateConversation(conv, msg.ActionEndMsg{SessionID: "s1", SuccessCount: 1, FailedCount: 0})

	conv, _ = UpdateConversation(conv, msg.ThinkingDeltaMsg{SessionID: "s1", Content: "Second thought..."})
	conv, _ = UpdateConversation(conv, msg.ThinkingDoneMsg{SessionID: "s1", Reasoning: "Second thought..."})
	conv, _ = UpdateConversation(conv, msg.ActionStartMsg{SessionID: "s1", ToolCount: 1, ToolNames: []string{"tool2"}})
	conv, _ = UpdateConversation(conv, msg.ToolExecStartMsg{SessionID: "s1", ToolName: "tool2"})
	conv, _ = UpdateConversation(conv, msg.ToolExecEndMsg{SessionID: "s1", ToolName: "tool2", Success: true, Result: "done"})
	conv, _ = UpdateConversation(conv, msg.ActionEndMsg{SessionID: "s1", SuccessCount: 1, FailedCount: 0})

	// Rounds are created per ActionStartMsg
	if len(conv.Rounds) != 2 {
		t.Fatalf("expected 2 rounds, got %d", len(conv.Rounds))
	}
	if !conv.Rounds[0].Action.Completed {
		t.Error("expected round 0 action completed")
	}
	if !conv.Rounds[1].Action.Completed {
		t.Error("expected round 1 action completed")
	}
}

func TestConversationViewFullTAO(t *testing.T) {
	conv := NewConversation("s1", "agent1", "What is Go?")
	conv.Rounds = append(conv.Rounds, ThoughtActionRound{
		Action: Action{
			CurrentInfo: &ActionInfo{ToolCount: 1, ToolNames: []string{"bash"}},
			Steps:       []ActionStep{{ToolName: "bash", Status: ActionStepDone, ResultText: "done"}},
			Completed:   true,
		},
	})
	conv.Rounds[0].Thought.Content = "thinking step 1..."
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
	if !strings.Contains(view, "执行操作") {
		t.Error("expected action header in view")
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
