package client

import (
	"testing"
)

func TestCommandFindCaseSensitive(t *testing.T) {
	registry := NewSlashCommandRegistry()

	registry.Register(Command{
		Name: "agent", Description: "Test agent command",
		Run: func(args string) *CommandResult {
			return &CommandResult{Message: "agent executed"}
		},
	})

	registry.Register(Command{
		Name: "help", Description: "Test help command",
		Run: func(args string) *CommandResult {
			return &CommandResult{Message: "help executed"}
		},
	})

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Exact match lowercase", "agent", true},
		{"Exact match help", "help", true},
		{"Uppercase A should NOT match", "Agent", false},
		{"All uppercase should NOT match", "AGENT", false},
		{"Mixed case should NOT match", "aGeNt", false},
		{"Unknown command", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := registry.Find(tt.input)
			if tt.expected && cmd == nil {
				t.Errorf("Find(%q) = nil, want non-nil", tt.input)
			}
			if !tt.expected && cmd != nil {
				t.Errorf("Find(%q) = %v, want nil", tt.input, cmd.Name)
			}
		})
	}
}

func TestCommandFilterCaseSensitive(t *testing.T) {
	registry := NewSlashCommandRegistry()

	registry.Register(Command{
		Name: "agent", Description: "Test agent command",
	})
	registry.Register(Command{
		Name: "help", Description: "Test help command",
	})

	tests := []struct {
		name        string
		prefix      string
		expectCount int
	}{
		{"Lowercase 'a' matches agent", "a", 1},
		{"Lowercase 'h' matches help", "h", 1},
		{"Uppercase 'A' should match nothing", "A", 0},
		{"Empty prefix returns all", "", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := registry.Filter(tt.prefix)
			if len(result) != tt.expectCount {
				t.Errorf("Filter(%q) = %d commands, want %d", tt.prefix, len(result), tt.expectCount)
			}
		})
	}
}
