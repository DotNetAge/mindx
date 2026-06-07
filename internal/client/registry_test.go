package client_test

import (
	"strings"
	"testing"

	"github.com/DotNetAge/mindx/internal/client"
)

func TestRegisterAndGet(t *testing.T) {
	r := client.NewSlashCommandRegistry()
	r.Register(client.CommandDef{
		Name:        "test",
		Description: "a test command",
		Run:         func(args []string) client.CommandResult { return client.CommandResult{} },
	})
	cmd := r.Get("test")
	if cmd == nil {
		t.Fatal("Get returned nil")
	}
	if cmd.Name != "test" {
		t.Errorf("Name = %q, want %q", cmd.Name, "test")
	}
	if cmd.Description != "a test command" {
		t.Errorf("Description = %q, want %q", cmd.Description, "a test command")
	}
}

func TestList(t *testing.T) {
	r := client.NewSlashCommandRegistry()
	r.Register(client.CommandDef{
		Name:        "cmd1",
		Description: "first",
		Run:         func(args []string) client.CommandResult { return client.CommandResult{} },
	})
	r.Register(client.CommandDef{
		Name:        "cmd2",
		Description: "second",
		Run:         func(args []string) client.CommandResult { return client.CommandResult{} },
	})
	list := r.List()
	if len(list) != 2 {
		t.Fatalf("List() returned %d items, want 2", len(list))
	}
	names := map[string]bool{}
	for _, c := range list {
		names[c.Name] = true
	}
	if !names["cmd1"] {
		t.Error("List missing cmd1")
	}
	if !names["cmd2"] {
		t.Error("List missing cmd2")
	}
}

func TestGetUnknown(t *testing.T) {
	r := client.NewSlashCommandRegistry()
	cmd := r.Get("nonexistent")
	if cmd != nil {
		t.Errorf("Get returned %v, want nil", cmd)
	}
}

func TestBuiltinCommands(t *testing.T) {
	deps := client.CommandDeps{
		OnClear:  func() {},
		OnExit:   func() {},
		OnDoctor: func() {},
	}
	r := client.BuiltinCommands(deps)
	expected := []string{"help", "clear", "exit", "doctor", "model", "chat", "agents"}
	for _, name := range expected {
		cmd := r.Get(name)
		if cmd == nil {
			t.Errorf("BuiltinCommands missing %q", name)
		}
	}
}

func TestHelpCommand(t *testing.T) {
	deps := client.CommandDeps{
		OnClear:  func() {},
		OnExit:   func() {},
		OnDoctor: func() {},
	}
	r := client.BuiltinCommands(deps)
	helpCmd := r.Get("help")
	if helpCmd == nil {
		t.Fatal("help command not found")
	}
	result := helpCmd.Run(nil)
	if !strings.Contains(result.Message, "clear") {
		t.Errorf("help output should contain 'clear', got: %s", result.Message)
	}
}

func TestClearCommand(t *testing.T) {
	cleared := false
	deps := client.CommandDeps{
		OnClear:  func() { cleared = true },
		OnExit:   func() {},
		OnDoctor: func() {},
	}
	r := client.BuiltinCommands(deps)
	clearCmd := r.Get("clear")
	if clearCmd == nil {
		t.Fatal("clear command not found")
	}
	result := clearCmd.Run(nil)
	if result.Message != "" {
		t.Errorf("clear command returned message %q, want empty", result.Message)
	}
	if !cleared {
		t.Error("clear command should call OnClear callback")
	}
}
