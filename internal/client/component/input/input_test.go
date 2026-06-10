package input

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/i18n"
)

func TestNewInputArea(t *testing.T) {
	i := New()
	if i == nil {
		t.Fatal("New() returned nil")
	}
	if i.Hidden {
		t.Error("New(): expected Hidden=false")
	}
	if i.Width != 0 {
		t.Errorf("New(): expected Width=0, got %d", i.Width)
	}
}

func TestInputAreaViewNotHidden(t *testing.T) {
	i := New()
	v := i.View()
	if v == "" {
		t.Error("View() returned empty when not hidden")
	}
}

func TestInputAreaViewHidden(t *testing.T) {
	i := New()
	i.Hidden = true
	v := i.View()
	if v != "" {
		t.Error("View() should return empty when Hidden=true")
	}
}

func TestInputAreaViewEmpty(t *testing.T) {
	i := New()
	v := i.View()
	if !strings.Contains(v, "❯") {
		t.Error("View() should contain prompt '❯'")
	}
	if !strings.Contains(v, i18n.T("client.ui.input.placeholder")) {
		t.Error("View() should contain placeholder '发送消息或...'")
	}
}

func TestWindowResize(t *testing.T) {
	i := New()
	i.Update(clientmsg.WindowResizeMsg{Width: 100})
	if i.Width != 100 {
		t.Errorf("Expected Width=100, got %d", i.Width)
	}
}

func TestTypeCharacter(t *testing.T) {
	i := New()
	for _, ch := range []string{"h", "e", "l", "l", "o"} {
		i.Update(tea.KeyPressMsg(tea.Key{Text: ch, Code: rune(ch[0])}))
	}
	if i.TextBuffer.String() != "hello" {
		t.Errorf("Expected 'hello', got '%s'", i.TextBuffer.String())
	}
	if i.CursorPos != 5 {
		t.Errorf("Expected CursorPos=5, got %d", i.CursorPos)
	}
}

func TestBackspace(t *testing.T) {
	i := New()
	for _, ch := range []string{"h", "e", "l", "l", "o"} {
		i.Update(tea.KeyPressMsg(tea.Key{Text: ch, Code: rune(ch[0])}))
	}
	i.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	if i.TextBuffer.String() != "hell" {
		t.Errorf("Expected 'hell', got '%s'", i.TextBuffer.String())
	}
	if i.CursorPos != 4 {
		t.Errorf("Expected CursorPos=4, got %d", i.CursorPos)
	}
}

func TestBackspaceEmpty(t *testing.T) {
	i := New()
	i.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	if i.TextBuffer.String() != "" {
		t.Errorf("Expected empty, got '%s'", i.TextBuffer.String())
	}
}

func TestEnterSendMessage(t *testing.T) {
	i := New()
	for _, ch := range []string{"h", "e", "l", "l", "o"} {
		i.Update(tea.KeyPressMsg(tea.Key{Text: ch, Code: rune(ch[0])}))
	}
	_, cmd := i.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("Expected non-nil cmd on enter")
	}
	msg := cmd()
	userMsg, ok := msg.(clientmsg.UserSendMsg)
	if !ok {
		t.Fatalf("Expected UserSendMsg, got %T", msg)
	}
	if userMsg.Text != "hello" {
		t.Errorf("Expected Text='hello', got '%s'", userMsg.Text)
	}
	if i.TextBuffer.String() != "" {
		t.Errorf("Expected empty buffer after enter, got '%s'", i.TextBuffer.String())
	}
}

func TestEnterSlashCommand(t *testing.T) {
	i := New()
	for _, ch := range []string{"/", "c", "l", "e", "a", "r"} {
		i.Update(tea.KeyPressMsg(tea.Key{Text: ch, Code: rune(ch[0])}))
	}
	_, cmd := i.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("Expected non-nil cmd on enter for slash command")
	}
	msg := cmd()
	slashMsg, ok := msg.(clientmsg.SlashCommandMsg)
	if !ok {
		t.Fatalf("Expected SlashCommandMsg, got %T", msg)
	}
	if slashMsg.Name != "clear" {
		t.Errorf("Expected Name='clear', got '%s'", slashMsg.Name)
	}
	if len(slashMsg.Args) != 0 {
		t.Errorf("Expected empty Args, got %v", slashMsg.Args)
	}
	if i.TextBuffer.String() != "" {
		t.Errorf("Expected empty buffer after enter, got '%s'", i.TextBuffer.String())
	}
}

func TestEnterEmptyText(t *testing.T) {
	i := New()
	_, cmd := i.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd != nil {
		t.Error("Expected nil cmd on empty enter")
	}
}

func TestCtrlC(t *testing.T) {
	i := New()
	var key tea.Key
	if isDarwin {
		key = tea.Key{Mod: tea.ModSuper, Code: 'c'}
	} else {
		key = tea.Key{Mod: tea.ModCtrl, Code: 'c'}
	}
	_, cmd := i.Update(tea.KeyPressMsg(key))
	if cmd == nil {
		t.Fatal("Expected non-nil cmd on copy shortcut")
	}
	msg := cmd()
	if _, ok := msg.(clientmsg.ExitMsg); !ok {
		t.Fatalf("Expected ExitMsg, got %T", msg)
	}

	i.TextBuffer.WriteString("some text")
	i.CursorPos = 9
	_, cmd2 := i.Update(tea.KeyPressMsg(key))
	if cmd2 == nil {
		t.Fatal("Expected non-nil cmd on copy shortcut with text")
	}
	msg2 := cmd2()
	if _, ok := msg2.(clientmsg.ExitMsg); ok {
		t.Fatal("Expected no ExitMsg when there's text (should clear instead)")
	}
}

func TestCtrlL(t *testing.T) {
	i := New()
	var key tea.Key
	if isDarwin {
		key = tea.Key{Mod: tea.ModSuper, Code: 'l'}
	} else {
		key = tea.Key{Mod: tea.ModCtrl, Code: 'l'}
	}
	_, cmd := i.Update(tea.KeyPressMsg(key))
	if cmd == nil {
		t.Fatal("Expected non-nil cmd on clear screen shortcut")
	}
	msg := cmd()
	if _, ok := msg.(clientmsg.ClearScreenMsg); !ok {
		t.Fatalf("Expected ClearScreenMsg, got %T", msg)
	}
}

func TestCommandSuggestions(t *testing.T) {
	i := New()
	i.Commands = []SlashCommand{
		{Name: "help", Description: "help info"},
		{Name: "clear", Description: "clear screen"},
		{Name: "exit", Description: "exit app"},
	}
	i.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	if i.cmdSuggest.Filter != "" {
		t.Errorf("Expected empty filter after '/', got '%s'", i.cmdSuggest.Filter)
	}
	if len(i.cmdSuggest.Items) != 3 {
		t.Errorf("Expected 3 commands, got %d", len(i.cmdSuggest.Items))
	}
	i.Update(tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}))
	i.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	if i.cmdSuggest.Filter != "he" {
		t.Errorf("Expected filter 'he', got '%s'", i.cmdSuggest.Filter)
	}
	filtered := i.cmdSuggest.filtered()
	if len(filtered) != 1 || filtered[0].Name != "help" {
		t.Errorf("Expected filtered to return only 'help', got %+v", filtered)
	}
}

func TestTabAutocompleteCommand(t *testing.T) {
	i := New()
	i.Commands = []SlashCommand{
		{Name: "help", Description: "help info"},
		{Name: "clear", Description: "clear screen"},
	}
	i.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	i.Update(tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}))
	i.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	expected := "/help "
	if i.TextBuffer.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, i.TextBuffer.String())
	}
	if i.CursorPos != len(expected) {
		t.Errorf("Expected CursorPos=%d, got %d", len(expected), i.CursorPos)
	}
}

func TestCommandSuggestionDownUp(t *testing.T) {
	i := New()
	i.Commands = []SlashCommand{
		{Name: "help", Description: "help info"},
		{Name: "history", Description: "show history"},
	}
	i.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	i.Update(tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}))
	if i.cmdSuggest.Filter != "h" {
		t.Errorf("Expected filter 'h', got '%s'", i.cmdSuggest.Filter)
	}
	if i.cmdSuggest.SelIdx != 0 {
		t.Errorf("Expected SelIdx=0, got %d", i.cmdSuggest.SelIdx)
	}
	i.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if i.cmdSuggest.SelIdx != 1 {
		t.Errorf("Expected SelIdx=1 after down, got %d", i.cmdSuggest.SelIdx)
	}
	i.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if i.cmdSuggest.SelIdx != 0 {
		t.Errorf("Expected SelIdx=0 after up, got %d", i.cmdSuggest.SelIdx)
	}
}

func TestAgentSuggestions(t *testing.T) {
	i := New()
	i.Agents = []data.AgentInfo{
		{Name: "architect", Description: "sys design"},
		{Name: "developer", Description: "coding"},
	}
	i.Update(tea.KeyPressMsg(tea.Key{Text: "@", Code: '@'}))
	if i.agentSuggest.Filter != "" {
		t.Errorf("Expected empty filter after '@', got '%s'", i.agentSuggest.Filter)
	}
	if len(i.agentSuggest.Items) != 2 {
		t.Errorf("Expected 2 agents, got %d", len(i.agentSuggest.Items))
	}
	i.Update(tea.KeyPressMsg(tea.Key{Text: "d", Code: 'd'}))
	i.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	if i.agentSuggest.Filter != "de" {
		t.Errorf("Expected filter 'de', got '%s'", i.agentSuggest.Filter)
	}
	filtered := i.agentSuggest.filtered()
	if len(filtered) != 1 || filtered[0].Name != "developer" {
		t.Errorf("Expected filtered to return 'developer', got %+v", filtered)
	}
}

func TestTabAutocompleteAgent(t *testing.T) {
	i := New()
	i.Agents = []data.AgentInfo{
		{Name: "architect", Description: "sys design"},
		{Name: "developer", Description: "coding"},
	}
	i.Update(tea.KeyPressMsg(tea.Key{Text: "@", Code: '@'}))
	i.Update(tea.KeyPressMsg(tea.Key{Text: "d", Code: 'd'}))
	i.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	i.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	expected := "@developer "
	if i.TextBuffer.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, i.TextBuffer.String())
	}
	if i.CursorPos != len(expected) {
		t.Errorf("Expected CursorPos=%d, got %d", len(expected), i.CursorPos)
	}
}

func TestAgentSuggestionDownUp(t *testing.T) {
	i := New()
	i.Agents = []data.AgentInfo{
		{Name: "developer", Description: "coding"},
		{Name: "designer", Description: "design"},
	}
	i.Update(tea.KeyPressMsg(tea.Key{Text: "@", Code: '@'}))
	i.Update(tea.KeyPressMsg(tea.Key{Text: "d", Code: 'd'}))
	if i.agentSuggest.Filter != "d" {
		t.Errorf("Expected filter 'd', got '%s'", i.agentSuggest.Filter)
	}
	if i.agentSuggest.SelIdx != 0 {
		t.Errorf("Expected SelIdx=0, got %d", i.agentSuggest.SelIdx)
	}
	i.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if i.agentSuggest.SelIdx != 1 {
		t.Errorf("Expected SelIdx=1 after down, got %d", i.agentSuggest.SelIdx)
	}
	i.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if i.agentSuggest.SelIdx != 0 {
		t.Errorf("Expected SelIdx=0 after up, got %d", i.agentSuggest.SelIdx)
	}
}

func TestViewWithText(t *testing.T) {
	i := New()
	for _, ch := range []string{"t", "e", "s", "t"} {
		i.Update(tea.KeyPressMsg(tea.Key{Text: ch, Code: rune(ch[0])}))
	}
	v := i.View()
	if !strings.Contains(v, "❯") {
		t.Error("View() should contain prompt '❯'")
	}
	if !strings.Contains(v, "test") {
		t.Error("View() should contain 'test' text")
	}
}

func TestViewWithCommandSuggestions(t *testing.T) {
	i := New()
	i.Commands = []SlashCommand{
		{Name: "help", Description: "help info"},
	}
	i.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	i.Update(tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}))
	i.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	v := i.View()
	if !strings.Contains(v, "/help") {
		t.Error("View() should contain '/help' in suggestions")
	}
}

func TestViewWithAgentSuggestions(t *testing.T) {
	i := New()
	i.Agents = []data.AgentInfo{
		{Name: "developer", Description: "coding"},
	}
	i.Update(tea.KeyPressMsg(tea.Key{Text: "@", Code: '@'}))
	i.Update(tea.KeyPressMsg(tea.Key{Text: "d", Code: 'd'}))
	i.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	v := i.View()
	if !strings.Contains(v, "@developer") {
		t.Error("View() should contain '@developer' in suggestions")
	}
}

func TestOtherMsgIgnored(t *testing.T) {
	i := New()
	i.Update("some random string message")
}

func TestKeyPressWhenHidden(t *testing.T) {
	i := New()
	i.Hidden = true
	i.Update(tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}))
	if i.TextBuffer.String() != "" {
		t.Error("Keypress should be ignored when Hidden=true")
	}
}

func TestSlashCommandWithArgs(t *testing.T) {
	i := New()
	for _, ch := range []string{"/", "r", "u", "n"} {
		i.Update(tea.KeyPressMsg(tea.Key{Text: ch, Code: rune(ch[0])}))
	}
	_, cmd := i.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("Expected non-nil cmd on enter")
	}
	msg := cmd()
	slashMsg, ok := msg.(clientmsg.SlashCommandMsg)
	if !ok {
		t.Fatalf("Expected SlashCommandMsg, got %T", msg)
	}
	if slashMsg.Name != "run" {
		t.Errorf("Expected Name='run', got '%s'", slashMsg.Name)
	}
}
