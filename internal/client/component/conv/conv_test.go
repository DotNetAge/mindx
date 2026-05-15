package conv

import (
	"errors"
	"strings"
	"testing"

	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
)

func TestWelcomeScreen(t *testing.T) {
	p := New()
	view := p.View()

	checks := []string{
		"MindX CLI",
		"Authenticated",
		"███",
	}
	for _, c := range checks {
		if !strings.Contains(view, c) {
			t.Errorf("expected welcome view to contain %q", c)
		}
	}
	if !p.WelcomeShown {
		t.Error("expected WelcomeShown to be true after View()")
	}
}

func TestWelcomeShownOnce(t *testing.T) {
	p := New()
	first := p.View()
	if first == "" {
		t.Fatal("expected non-empty welcome view")
	}
	if !p.WelcomeShown {
		t.Fatal("expected WelcomeShown after first View()")
	}

	second := p.View()
	if second != "" {
		t.Errorf("expected empty view on second call, got %q", second)
	}
}

func TestThinkingDelta(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.ThinkingDeltaMsg{SessionID: "s1", Content: "thinking step 1..."})

	if len(p.Answers) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(p.Answers))
	}
	a := p.Answers[0]
	if a.PendingThink != "thinking step 1..." {
		t.Errorf("expected PendingThink %q, got %q", "thinking step 1...", a.PendingThink)
	}
	if a.Status != data.StatusThinking {
		t.Errorf("expected StatusThinking, got %v", a.Status)
	}
	if !a.IsThinking {
		t.Error("expected IsThinking=true")
	}
}

func TestThinkingDone(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.ThinkingDeltaMsg{SessionID: "s1", Content: "thinking step 1..."})
	p.Update(clientmsg.ThinkingDoneMsg{SessionID: "s1"})

	a := p.Answers[0]
	if len(a.ThinkingLog) != 1 {
		t.Fatalf("expected 1 ThinkingLog entry, got %d", len(a.ThinkingLog))
	}
	if a.ThinkingLog[0].Content != "thinking step 1..." {
		t.Errorf("expected content %q, got %q", "thinking step 1...", a.ThinkingLog[0].Content)
	}
	if a.PendingThink != "" {
		t.Errorf("expected empty PendingThink, got %q", a.PendingThink)
	}
	if a.IsThinking {
		t.Error("expected IsThinking=false")
	}
}

func TestThinkingDeltaMultiRound(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.ThinkingDeltaMsg{SessionID: "s1", Content: "round 1"})
	p.Update(clientmsg.ThinkingDoneMsg{SessionID: "s1"})

	p.Update(clientmsg.ThinkingDeltaMsg{SessionID: "s1", Content: "round 2"})
	p.Update(clientmsg.ThinkingDoneMsg{SessionID: "s1"})

	if len(p.Answers[0].ThinkingLog) != 2 {
		t.Fatalf("expected 2 ThinkingLog entries, got %d", len(p.Answers[0].ThinkingLog))
	}
	if p.Answers[0].ThinkingLog[0].Content != "round 1" {
		t.Errorf("expected first entry %q, got %q", "round 1", p.Answers[0].ThinkingLog[0].Content)
	}
	if p.Answers[0].ThinkingLog[1].Content != "round 2" {
		t.Errorf("expected second entry %q, got %q", "round 2", p.Answers[0].ThinkingLog[1].Content)
	}
}

func TestActionStart(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.ActionStartMsg{
		SessionID:    "s1",
		ToolName:     "bash",
		EstimatedTok: 100,
		Params:       map[string]any{"cmd": "ls"},
	})

	if len(p.Answers) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(p.Answers))
	}
	if len(p.Answers[0].Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(p.Answers[0].Actions))
	}

	step := p.Answers[0].Actions[0]
	if step.ToolName != "bash" {
		t.Errorf("expected ToolName 'bash', got %q", step.ToolName)
	}
	if step.Status != data.ActionExecuting {
		t.Errorf("expected ActionExecuting, got %v", step.Status)
	}
	if step.EstimatedTok != 100 {
		t.Errorf("expected EstimatedTok 100, got %d", step.EstimatedTok)
	}
	if step.Params == nil || step.Params["cmd"] != "ls" {
		t.Errorf("expected params to contain cmd=ls")
	}
	if !step.Collapsed {
		t.Error("expected Collapsed=true by default")
	}
	if p.Answers[0].Status != data.StatusExecuting {
		t.Errorf("expected StatusExecuting, got %v", p.Answers[0].Status)
	}
}

func TestActionStartFlushesPendingThink(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.ThinkingDeltaMsg{SessionID: "s1", Content: "thinking before action..."})
	p.Update(clientmsg.ActionStartMsg{SessionID: "s1", ToolName: "bash"})

	if len(p.Answers[0].ThinkingLog) != 1 {
		t.Fatalf("expected 1 ThinkingLog entry, got %d", len(p.Answers[0].ThinkingLog))
	}
	if p.Answers[0].ThinkingLog[0].Content != "thinking before action..." {
		t.Errorf("expected flushed content %q, got %q", "thinking before action...", p.Answers[0].ThinkingLog[0].Content)
	}
	if p.Answers[0].PendingThink != "" {
		t.Errorf("expected empty PendingThink after flush, got %q", p.Answers[0].PendingThink)
	}
}

func TestActionProgress(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.ActionStartMsg{SessionID: "s1", ToolName: "bash"})
	p.Update(clientmsg.ActionProgressMsg{SessionID: "s1", ToolName: "bash", Progress: "running command..."})

	if p.Answers[0].Actions[0].ProgressText != "running command..." {
		t.Errorf("expected ProgressText %q, got %q", "running command...", p.Answers[0].Actions[0].ProgressText)
	}
}

func TestActionResultSuccess(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.ActionStartMsg{SessionID: "s1", ToolName: "bash"})
	p.Update(clientmsg.ActionResultMsg{
		SessionID: "s1",
		ToolName:  "bash",
		Success:   true,
		Result:    "command output",
	})

	step := p.Answers[0].Actions[0]
	if step.Status != data.ActionDone {
		t.Errorf("expected ActionDone, got %v", step.Status)
	}
	if step.ResultText != "command output" {
		t.Errorf("expected ResultText %q, got %q", "command output", step.ResultText)
	}
}

func TestActionResultFailed(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.ActionStartMsg{SessionID: "s1", ToolName: "bash"})
	p.Update(clientmsg.ActionResultMsg{
		SessionID: "s1",
		ToolName:  "bash",
		Success:   false,
		Error:     "permission denied",
	})

	step := p.Answers[0].Actions[0]
	if step.Status != data.ActionFailed {
		t.Errorf("expected ActionFailed, got %v", step.Status)
	}
	if step.ResultText != "permission denied" {
		t.Errorf("expected ResultText %q, got %q", "permission denied", step.ResultText)
	}
}

func TestActionProgressWrongTool(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.ActionStartMsg{SessionID: "s1", ToolName: "bash"})
	p.Update(clientmsg.ActionProgressMsg{SessionID: "s1", ToolName: "wrong-tool", Progress: "should not update"})

	if p.Answers[0].Actions[0].ProgressText != "" {
		t.Errorf("expected empty ProgressText for wrong tool, got %q", p.Answers[0].Actions[0].ProgressText)
	}
}

func TestSessionDone(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.ThinkingDeltaMsg{SessionID: "s1", Content: "thinking..."})
	p.Update(clientmsg.ThinkingDoneMsg{SessionID: "s1"})
	p.Update(clientmsg.SessionDoneMsg{SessionID: "s1"})

	a := p.Answers[0]
	if a.Status != data.StatusDone {
		t.Errorf("expected StatusDone, got %v", a.Status)
	}
	if a.IsThinking {
		t.Error("expected IsThinking=false")
	}
}

func TestMultipleAnswers(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.ThinkingDeltaMsg{SessionID: "s1", Content: "s1 thinking..."})
	p.Update(clientmsg.ThinkingDoneMsg{SessionID: "s1"})
	p.Update(clientmsg.FinalAnswerMsg{SessionID: "s1", Content: "s1 answer"})

	p.Update(clientmsg.ThinkingDeltaMsg{SessionID: "s2", Content: "s2 thinking..."})
	p.Update(clientmsg.ThinkingDoneMsg{SessionID: "s2"})
	p.Update(clientmsg.FinalAnswerMsg{SessionID: "s2", Content: "s2 answer"})

	if len(p.Answers) != 2 {
		t.Fatalf("expected 2 answers, got %d", len(p.Answers))
	}
	if p.Answers[0].SessionID != "s1" {
		t.Errorf("expected s1, got %s", p.Answers[0].SessionID)
	}
	if p.Answers[1].SessionID != "s2" {
		t.Errorf("expected s2, got %s", p.Answers[1].SessionID)
	}
}

func TestFindAnswer(t *testing.T) {
	p := New()
	p.Answers = []data.AnswerData{
		{SessionID: "s1"},
		{SessionID: "s2"},
	}

	if idx := p.findAnswer("s1"); idx != 0 {
		t.Errorf("expected 0 for s1, got %d", idx)
	}
	if idx := p.findAnswer("s2"); idx != 1 {
		t.Errorf("expected 1 for s2, got %d", idx)
	}
}

func TestFindAnswerNotFound(t *testing.T) {
	p := New()
	if idx := p.findAnswer("nonexistent"); idx != -1 {
		t.Errorf("expected -1, got %d", idx)
	}
}

func TestFindOrCreateAnswer(t *testing.T) {
	p := New()
	idx1 := p.findOrCreateAnswer("s1", "")
	if idx1 != 0 {
		t.Errorf("expected 0 for first create, got %d", idx1)
	}
	if len(p.Answers) != 1 {
		t.Errorf("expected 1 answer, got %d", len(p.Answers))
	}

	idx2 := p.findOrCreateAnswer("s1", "")
	if idx2 != 0 {
		t.Errorf("expected 0 for existing, got %d", idx2)
	}
	if len(p.Answers) != 1 {
		t.Errorf("expected still 1 answer, got %d", len(p.Answers))
	}
}

func TestEmptyView(t *testing.T) {
	p := New()

	first := p.View()
	if first == "" {
		t.Error("expected welcome text on first View() call")
	}

	second := p.View()
	if second != "" {
		t.Errorf("expected empty string for second View() call, got %q", second)
	}
}

func TestTranscriptView(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.TranscriptToggleMsg{})
	if p.ViewMode != ViewTranscript {
		t.Error("expected ViewTranscript after toggle")
	}

	view := p.View()
	if view != "" {
		t.Errorf("expected empty transcript view with no answers, got %q", view)
	}
}

func TestTranscriptToggle(t *testing.T) {
	p := New()

	if p.ViewMode != ViewNormal {
		t.Error("expected ViewNormal initially")
	}

	p.Update(clientmsg.TranscriptToggleMsg{})
	if p.ViewMode != ViewTranscript {
		t.Error("expected ViewTranscript after first toggle")
	}

	p.Update(clientmsg.TranscriptToggleMsg{})
	if p.ViewMode != ViewNormal {
		t.Error("expected ViewNormal after second toggle")
	}
}

func TestCollapseToggle(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.ActionStartMsg{SessionID: "s1", ToolName: "bash"})
	step := &p.Answers[0].Actions[0]

	if !step.Collapsed {
		t.Error("expected Collapsed=true initially")
	}

	p.Update(clientmsg.CollapseToggleMsg{AnswerIndex: 0, ActionIndex: 0})
	if step.Collapsed {
		t.Error("expected Collapsed=false after toggle")
	}

	p.Update(clientmsg.CollapseToggleMsg{AnswerIndex: 0, ActionIndex: 0})
	if !step.Collapsed {
		t.Error("expected Collapsed=true after second toggle")
	}
}

func TestThinkCollapse(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.ThinkingDeltaMsg{SessionID: "s1", Content: "thinking..."})
	p.Update(clientmsg.ThinkingDoneMsg{SessionID: "s1"})

	if !p.Answers[0].ThinkingCollapsed {
		t.Error("expected ThinkingCollapsed=true initially")
	}

	p.Update(clientmsg.ThinkCollapseMsg{AnswerIndex: 0})
	if p.Answers[0].ThinkingCollapsed {
		t.Error("expected ThinkingCollapsed=false after toggle")
	}

	p.Update(clientmsg.ThinkCollapseMsg{AnswerIndex: 0})
	if !p.Answers[0].ThinkingCollapsed {
		t.Error("expected ThinkingCollapsed=true after second toggle")
	}
}

func TestClearScreen(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.ThinkingDeltaMsg{SessionID: "s1", Content: "thinking..."})
	p.Update(clientmsg.ThinkingDoneMsg{SessionID: "s1"})
	if len(p.Answers) == 0 {
		t.Fatal("expected answers before clear")
	}

	p.Update(clientmsg.ClearScreenMsg{})
	if len(p.Answers) != 0 {
		t.Errorf("expected 0 answers after clear, got %d", len(p.Answers))
	}
	if p.WelcomeShown {
		t.Error("expected WelcomeShown=false after clear")
	}
}

func TestTickAnimation(t *testing.T) {
	p := New()
	p.View()

	if p.BlinkOn {
		t.Error("expected BlinkOn=false initially")
	}

	p.Update(clientmsg.ThinkingDeltaMsg{SessionID: "s1", Content: "..."})
	p.Update(clientmsg.TickMsg{})
	if !p.BlinkOn {
		t.Error("expected BlinkOn=true after TickMsg while thinking")
	}

	p.Update(clientmsg.TickMsg{})
	if p.BlinkOn {
		t.Error("expected BlinkOn=false after second TickMsg")
	}
}

func TestTickCmdReturned(t *testing.T) {
	p := New()
	p.View()

	_, cmd := p.Update(clientmsg.ThinkingDeltaMsg{SessionID: "s1", Content: "thinking..."})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from ThinkingDelta")
	}

	msg := cmd()
	if _, ok := msg.(clientmsg.TickMsg); !ok {
		t.Errorf("expected TickMsg from cmd, got %T", msg)
	}
}

func TestNoTickWhenIdle(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.TickMsg{})
	_, cmd := p.Update(clientmsg.TickMsg{})
	if cmd != nil {
		t.Error("expected nil cmd when no active thinking or executing")
	}
}

func TestViewContainsThinkingContent(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.ThinkingDeltaMsg{SessionID: "s1", Content: "deep thinking content..."})
	p.Update(clientmsg.ThinkingDoneMsg{SessionID: "s1"})

	view := p.View()
	if !strings.Contains(view, "deep thinking content...") {
		t.Errorf("expected view to contain thinking content, got:\n%s", view)
	}
}

func TestViewContainsActionContent(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.ActionStartMsg{SessionID: "s1", ToolName: "bash", EstimatedTok: 100})
	p.Update(clientmsg.ActionResultMsg{
		SessionID: "s1",
		ToolName:  "bash",
		Success:   true,
		Result:    "command executed successfully",
	})

	view := p.View()
	if !strings.Contains(view, "bash") {
		t.Errorf("expected view to contain tool name 'bash', got:\n%s", view)
	}
	if !strings.Contains(view, "command executed successfully") {
		t.Errorf("expected view to contain action result, got:\n%s", view)
	}
}

func TestViewContainsAnswer(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.FinalAnswerMsg{SessionID: "s1", Content: "Here is your final answer"})

	view := p.View()
	if !strings.Contains(view, "Here is your final answer") {
		t.Errorf("expected view to contain answer content, got:\n%s", view)
	}
}

func TestNormalViewWithAnswer(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.ThinkingDeltaMsg{SessionID: "s1", Content: "thinking step 1..."})
	p.Update(clientmsg.ThinkingDoneMsg{SessionID: "s1"})
	p.Update(clientmsg.ActionStartMsg{SessionID: "s1", ToolName: "bash", EstimatedTok: 100})
	p.Update(clientmsg.ActionResultMsg{
		SessionID: "s1",
		ToolName:  "bash",
		Success:   true,
		Result:    "done",
	})
	p.Answers[0].UserQuestion = "What is Go?"
	p.Update(clientmsg.FinalAnswerMsg{SessionID: "s1", Content: "Go is a programming language"})

	view := p.View()

	checks := []string{
		"What is Go?",
		"thinking step 1...",
		"bash",
		"Go is a programming language",
	}
	for _, c := range checks {
		if !strings.Contains(view, c) {
			t.Errorf("expected view to contain %q", c)
		}
	}
}

func TestFinalAnswer(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.FinalAnswerMsg{SessionID: "s1", Content: "Here is the answer"})

	if len(p.Answers[0].Results) != 1 {
		t.Fatalf("expected 1 ResultEntry, got %d", len(p.Answers[0].Results))
	}
	r := p.Answers[0].Results[0]
	if r.Role != "assistant" {
		t.Errorf("expected Role 'assistant', got %q", r.Role)
	}
	if r.Content != "Here is the answer" {
		t.Errorf("expected Content %q, got %q", "Here is the answer", r.Content)
	}
}

func TestAgentError(t *testing.T) {
	p := New()
	p.View()

	p.Update(clientmsg.AgentErrorMsg{
		SessionID: "s1",
		Error:     errors.New("something went wrong"),
	})

	if len(p.Answers[0].Results) != 1 {
		t.Fatalf("expected 1 ResultEntry, got %d", len(p.Answers[0].Results))
	}
	r := p.Answers[0].Results[0]
	if r.Role != "error" {
		t.Errorf("expected Role 'error', got %q", r.Role)
	}
	if !strings.Contains(r.Content, "something went wrong") {
		t.Errorf("expected Content to contain error text, got %q", r.Content)
	}
}
