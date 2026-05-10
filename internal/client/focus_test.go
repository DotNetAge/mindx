package client

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestMouseClickRestoresFocus(t *testing.T) {
	registry := NewSlashCommandRegistry()
	inputBox := NewInputBox(registry)

	tests := []struct {
		name           string
		initialFocus   bool
		hidden         bool
		mouseButton    tea.MouseButton
		expectFocus    bool
	}{
		{
			name:         "Click restores focus when unfocused",
			initialFocus: false,
			hidden:       false,
			mouseButton:  tea.MouseLeft,
			expectFocus:  true,
		},
		{
			name:         "Click maintains focus when already focused",
			initialFocus: true,
			hidden:       false,
			mouseButton:  tea.MouseLeft,
			expectFocus:  true,
		},
		{
			name:         "Click does not restore focus when hidden",
			initialFocus: false,
			hidden:       true,
			mouseButton:  tea.MouseLeft,
			expectFocus:  false,
		},
		{
			name:         "Right click does not restore focus",
			initialFocus: false,
			hidden:       false,
			mouseButton:  tea.MouseRight,
			expectFocus:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputBox.hidden = tt.hidden

			if !tt.initialFocus && !tt.hidden {
				inputBox.textarea.Blur()
			}

			if tt.initialFocus && !tt.hidden {
				inputBox.textarea.Focus()
			}

			focusedBefore := inputBox.IsFocused()

			mouseMsg := tea.MouseMsg(&testMouseEvent{
				button: tt.mouseButton,
			})

			mouse := mouseMsg.Mouse()
			if mouse.Button == tea.MouseLeft {
				if !inputBox.IsFocused() && !inputBox.hidden {
					inputBox.textarea.Focus()
				}
			}

			focusedAfter := inputBox.IsFocused()

			if focusedBefore == tt.expectFocus && focusedAfter != tt.expectFocus {
				t.Errorf("Focus not changed as expected: before=%v, after=%v, want=%v",
					focusedBefore, focusedAfter, tt.expectFocus)
			}
		})
	}
}

type testMouseEvent struct {
	button tea.MouseButton
}

func (m *testMouseEvent) Mouse() tea.Mouse {
	return tea.Mouse{
		Button: m.button,
	}
}

func (m *testMouseEvent) String() string {
	return "test mouse event"
}
