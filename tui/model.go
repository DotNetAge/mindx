package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type chatMsg struct {
	role    string
	content string
	time    time.Time
}

type errMsg error

type serverMsg string

type connectedMsg struct{}

// suggestionType distinguishes "/" commands from "@" file listings.
type suggestionType int

const (
	suggestNone suggestionType = iota
	suggestSlash
	suggestAt
)

type model struct {
	client       *gateway.Client
	messages     []chatMsg
	input        textinput.Model
	spinner      spinner.Model
	viewport     viewport.Model // scrollable message area
	loading      bool
	connected    bool
	err          error
	width        int
	height       int
	windowHeight int
	respCh       chan string

	// slash / @ command system
	registry             *CommandRegistry
	showSuggestions      bool
	suggestionType       suggestionType
	suggestions          []string    // either Command.Name or FileEntry.Name
	suggestionData       []FileEntry // only populated for @ listing
	selectedIdx          int

	// hierarchical navigation state
	commandPath           []string  // e.g. ["agents"] when browsing /agents subcommands
	commandQuery          string    // current query being typed after command path
	subCommands           []Command // current level subcommand list
	atBrowsePath          string    // current directory for @ browsing, e.g. "~/docs"
	navigating            bool      // true when user just navigated into subcommand/dir; blocks refreshSuggestions from resetting state
	suggestionsDismissed  bool      // true after closeSuggestions; won't re-trigger until user freshly types / or @
	prevValue            string    // previous input value before latest keystroke; used to detect fresh / @ triggers

	// token tracking
	tokensIn  int
	tokensOut int
}

func NewProgram() *tea.Program {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.Focus()
	ti.CharLimit = 512
	ti.Width = 100 // will be resized on WindowSizeMsg
	ti.Prompt = "> "
	// Match cursor background to input container background (#282A36)
	ti.Cursor.Style = lipgloss.NewStyle().Background(lipgloss.Color("#282A36"))

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	vp := viewport.New(100, 20)
	vp.KeyMap = viewport.KeyMap{
		Up:     key.NewBinding(key.WithKeys("k")),
		Down:   key.NewBinding(key.WithKeys("j")),
		PageUp: key.NewBinding(key.WithKeys("b")),
		PageDown: key.NewBinding(key.WithKeys("f")),
	}

	m := &model{
		input:      ti,
		spinner:    sp,
		viewport:   vp,
		respCh:     make(chan string, 16),
		registry:   BuiltinCommands(),
		prevValue:  "",
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	return p
}

func (m *model) Init() tea.Cmd {
	m.client = gateway.NewClient(
		gateway.WithClientAddr("localhost:1314"),
		gateway.WithClientPath("/ws"),
	)

	m.client.OnReceived(func(msg *gateway.Message) {
		display := formatMessage(msg)
		select {
		case m.respCh <- display:
		default:
		}
	})

	return tea.Batch(
		m.input.Focus(),
		m.spinner.Tick,
		connectCmd(m.client),
	)
}

func formatMessage(msg *gateway.Message) string {
	if msg == nil {
		return "(空消息)"
	}
	data := msg.Text()
	if isJSON(data) {
		var pretty bytes.Buffer
		json.Indent(&pretty, []byte(data), "", "  ")
		return pretty.String()
	}
	return data
}

func isJSON(s string) bool {
	s = strings.TrimSpace(s)
	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}

func connectCmd(client *gateway.Client) tea.Cmd {
	return func() tea.Msg {
		if err := client.Connect(); err != nil {
			return errMsg(fmt.Errorf("连接失败: %w", err))
		}
		return connectedMsg{}
	}
}

func waitServerMsg(ch chan string) tea.Cmd {
	return func() tea.Msg {
		msg := <-ch
		return serverMsg(msg)
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Layout: viewport + status + input(with padding top/bottom)
		// viewport.Height + 1(status) + 1(newline) + 3(input with Padding(1,1)) = m.height
		// => viewport.Height = m.height - 5
		m.windowHeight = m.height - 5
		// Input container: use MaxWidth so padding doesn't overflow
		inputContainerStyle.Width(m.width - 2) // account for left+right padding(1,1)
		m.input.Width = m.width - 10 // account for padding(2) + prompt "> "(2) + margin(6)
		// Resize viewport to fill available space
		m.viewport.Width = m.width
		m.viewport.Height = m.windowHeight
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		// Mouse wheel scrolling for viewport (when suggestions not shown)
		if !m.showSuggestions {
			switch msg.Type {
			case tea.MouseWheelUp, tea.MouseWheelDown:
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		}
		return m, nil

	case connectedMsg:
		m.connected = true
		return m, nil

	case serverMsg:
		m.loading = false
		display := string(msg)
		m.messages = append(m.messages, chatMsg{
			role:    "assistant",
			content: display,
			time:    time.Now(),
		})
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, waitServerMsg(m.respCh)

	case errMsg:
		m.loading = false
		m.err = msg
		m.messages = append(m.messages, chatMsg{
			role:    "error",
			content: msg.Error(),
			time:    time.Now(),
		})
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.refreshSuggestions()
	return m, cmd
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keys - always handled regardless of suggestions
	if key == "ctrl+c" {
		if m.client != nil && m.client.IsConnected() {
			m.client.Close()
		}
		return m, tea.Quit
	}

	// Suggestion navigation
	if m.showSuggestions {
		switch key {
		case "up":
			if m.selectedIdx > 0 {
				m.selectedIdx--
			}
			return m, nil
		case "down":
			if m.selectedIdx < len(m.suggestions)-1 {
				m.selectedIdx++
			}
			return m, nil
		case "right":
			if m.selectedIdx >= 0 && m.selectedIdx < len(m.suggestions) {
				m.navigateInto()
			}
			return m, nil
		case "left":
			m.navigateBack()
			return m, nil
		case "tab": // Tab always accepts immediately
			if m.selectedIdx >= 0 && m.selectedIdx < len(m.suggestions) {
				m.acceptSuggestion()
			} else {
				m.showSuggestions = false
			}
			return m, nil
		case "enter": // Enter behavior differs by type
			// Guard: ignore if no selection or empty list
			if m.selectedIdx < 0 || m.selectedIdx >= len(m.suggestions) {
				return m, nil
			}
			switch m.suggestionType {
			case suggestSlash:
				cmdName := m.suggestions[m.selectedIdx]
				var parentCmd *Command
				if len(m.commandPath) == 0 {
					parentCmd = m.registry.Find(cmdName)
				} else {
					for i := range m.subCommands {
						if m.subCommands[i].Name == cmdName {
							parentCmd = &m.subCommands[i]
							break
						}
					}
				}
				// If has subcommands -> navigate into; if not -> accept
				if parentCmd != nil && len(parentCmd.SubCommands) > 0 {
					m.navigateInto()
					return m, nil
				}
				m.acceptSuggestion()
				return m, nil
			case suggestAt:
				if m.selectedIdx >= 0 && m.selectedIdx < len(m.suggestionData) {
					entry := m.suggestionData[m.selectedIdx]
					if entry.IsDir {
						m.navigateInto()
						return m, nil
					}
				}
				m.acceptSuggestion()
				return m, nil
			}
			return m, nil
		case " ":
			if m.suggestionType == suggestSlash && len(m.commandPath) > 0 {
				if m.selectedIdx >= 0 && m.selectedIdx < len(m.suggestions) {
					m.acceptSuggestion()
				} else {
					var cmd tea.Cmd
					m.input, cmd = m.input.Update(msg)
					m.refreshSuggestions()
					return m, cmd
				}
			} else {
				// Propagate space to textinput normally
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				m.refreshSuggestions()
				return m, cmd
			}
			return m, nil
		case "esc":
			m.closeSuggestions()
			return m, nil
		case "backspace":
			if !m.navigateBack() {
				// Propagate backspace to textinput for normal deletion
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				m.refreshSuggestions()
				return m, cmd
			}
			return m, nil
		}
	}

	// Handle enter key for sending messages
	if key == "enter" {
		if m.loading || !m.connected {
			return m, nil
		}
		text := strings.TrimSpace(m.input.Value())
		if text == "" {
			return m, nil
		}

		// Local slash commands
		if strings.HasPrefix(text, "/") {
			name, args := parseCommandInput(text)
			if cmd := m.registry.Find(name); cmd != nil {
				m.closeSuggestions()
				result := cmd.Run(args)
				if result.ClearChat {
					m.messages = nil
				}
				m.messages = append(m.messages, chatMsg{
					role:    "user",
					content: text,
					time:    time.Now(),
				})
				m.input.Reset()
				m.messages = append(m.messages, chatMsg{
					role:    "command",
					content: result.Message,
					time:    time.Now(),
				})
				m.viewport.SetContent(m.renderMessages())
				m.viewport.GotoBottom()
				return m, nil
			}
		}

		// Send to agent
		m.messages = append(m.messages, chatMsg{
			role:    "user",
			content: text,
			time:    time.Now(),
		})
		m.input.Reset()
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		m.loading = true
		return m, tea.Batch(
			m.spinner.Tick,
			sendCmd(m.client, text),
			waitServerMsg(m.respCh),
		)
	}

	// Viewport scrolling keys (only when suggestions are not shown)
	if !m.showSuggestions {
		switch key {
		case "k", "up":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		case "j", "down":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		case "b":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		case "f":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	// Pass to textinput - unlock navigation mode on actual typing
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	// Unlock navigation lock on any keypress that isn't pure navigation
	// This prevents the UI from getting stuck in navigation mode
	if key != "" && key != "up" && key != "down" && key != "left" && key != "right" &&
		key != "enter" && key != "tab" && key != "esc" && key != "ctrl+c" {
		m.navigating = false
	}
	m.refreshSuggestions()
	return m, cmd
}

func (m *model) closeSuggestions() {
	m.showSuggestions = false
	m.commandPath = nil
	m.subCommands = nil
	m.commandQuery = ""
	m.atBrowsePath = ""
	m.navigating = false
	// Mark as dismissed: won't reappear until user freshly types / or @ as a NEW character
	m.suggestionsDismissed = true
}

// acceptSuggestion fills the input with the selected suggestion and closes the list.
func (m *model) acceptSuggestion() {
	if m.selectedIdx < 0 || m.selectedIdx >= len(m.suggestions) {
		return
	}
	selected := m.suggestions[m.selectedIdx]

	switch m.suggestionType {
	case suggestSlash:
		if len(m.commandPath) > 0 {
			// Full path: /parent subcommand
			m.input.SetValue("/" + strings.Join(m.commandPath, " ") + " " + selected + " ")
		} else {
			m.input.SetValue("/" + selected + " ")
		}
		m.input.CursorEnd()
	case suggestAt:
		// Use full path for files/directories
		if m.selectedIdx < len(m.suggestionData) {
			path := m.suggestionData[m.selectedIdx].Path
			m.input.SetValue("@" + path + " ")
		} else {
			m.input.SetValue("@" + selected + " ")
		}
		m.input.CursorEnd()
	}
	m.closeSuggestions()
}

// navigateInto moves into the selected subcommand or directory.
func (m *model) navigateInto() {
	// Guard: ignore if no valid selection
	if m.selectedIdx < 0 || m.selectedIdx >= len(m.suggestions) {
		return
	}
	switch m.suggestionType {
	case suggestSlash:
		// Check if this command has subcommands
		cmdName := m.suggestions[m.selectedIdx]
		var parentCmd *Command
		if len(m.commandPath) == 0 {
			parentCmd = m.registry.Find(cmdName)
		} else {
			// Find subcommand in current level
			for i := range m.subCommands {
				if m.subCommands[i].Name == cmdName {
					parentCmd = &m.subCommands[i]
					break
				}
			}
		}
		if parentCmd != nil && len(parentCmd.SubCommands) > 0 {
			// Navigate into subcommand level - lock to prevent refresh from resetting
			m.navigating = true
			m.commandPath = append(m.commandPath, cmdName)
			m.subCommands = parentCmd.SubCommands
			m.commandQuery = ""
			m.showSlashSuggestionsAtLevel("")
		} else {
			// No subcommands, just accept
			m.acceptSuggestion()
		}
	case suggestAt:
		if m.selectedIdx >= 0 && m.selectedIdx < len(m.suggestionData) {
			entry := m.suggestionData[m.selectedIdx]
			if entry.IsDir {
				// Navigate into directory - lock to prevent refresh from resetting
				m.navigating = true
				m.atBrowsePath = entry.Path
				m.input.SetValue("@" + entry.Name + "/")
				m.input.CursorEnd()
				// Manually show suggestions for the new directory (bypass navigating lock)
				m.showAtSuggestions("")
			} else {
				m.acceptSuggestion()
			}
		}
	}
}

// navigateBack moves back up one level in the hierarchy.
func (m *model) navigateBack() bool {
	switch m.suggestionType {
	case suggestSlash:
		if len(m.commandPath) > 0 {
			// Go back one level
			m.commandPath = m.commandPath[:len(m.commandPath)-1]
			m.commandQuery = "" // reset query when navigating back
			if len(m.commandPath) == 0 {
				// Back to root level - show top-level commands
				m.subCommands = nil
				m.showSlashSuggestions("")
			} else {
				// Show subcommands at this level
				m.showSlashSuggestionsAtLevel("")
			}
			return true
		}
		m.showSuggestions = false
		m.commandPath = nil
		m.commandQuery = ""
		m.subCommands = nil
	case suggestAt:
		if m.atBrowsePath != "" && m.atBrowsePath != expandHomeDir("~/") {
			// Go up one directory
			parent := filepath.Dir(m.atBrowsePath)
			usr, _ := user.Current()
			home := usr.HomeDir
			if !strings.HasPrefix(parent, home) {
				parent = home
			}
			m.atBrowsePath = parent
			// Update input to reflect parent dir name
			name := filepath.Base(parent)
			switch name {
			case usr.HomeDir, "~":
				name = "~"
			case "/":
				name = "~"
			}
			m.input.SetValue("@" + name + "/")
			m.input.CursorEnd()
			m.refreshSuggestions()
			return true
		}
		m.showSuggestions = false
		m.atBrowsePath = ""
	}
	return false
}

// refreshSuggestions updates the dropdown based on the current input.
func (m *model) refreshSuggestions() {
	// If we're in navigation mode (user navigated into subcommand/dir),
	// skip auto-refresh from input value - stay at current level
	if m.navigating {
		return
	}

	text := m.input.Value()

	// Once suggestions are dismissed (by accept, Esc, etc.), do NOT re-trigger
	// just because the current text happens to contain "/" or "@".
	// Only re-trigger when the user FRESHLY types a "/" or "@" as a new character.
	if m.suggestionsDismissed {
		// Check if this keystroke ADDED a fresh / or @ that wasn't there before
		freshSlash := strings.HasSuffix(text, "/") && !strings.HasSuffix(m.prevValue, "/")
		freshAt := strings.HasSuffix(text, "@") && !strings.HasSuffix(m.prevValue, "@")
		if !freshSlash && !freshAt {
			return // No fresh trigger — keep dismissed
		}
		// Fresh / or @ detected — clear dismissal and proceed normally
		m.suggestionsDismissed = false
	}

	m.prevValue = text

	// Detect @ file listing trigger
	if strings.Contains(text, "@") {
		// Find the last @ and get the partial path after it
		idx := strings.LastIndex(text, "@")
		partial := text[idx+1:] // everything after the last @

		// Determine browse path from the @ prefix
		// e.g., "@docs/" -> browse "~/docs"
		slashIdx := strings.Index(partial, "/")
		if slashIdx >= 0 {
			m.atBrowsePath = expandHomeDir("~/" + partial[:slashIdx])
			partial = partial[slashIdx+1:]
		} else {
			// Reset to home if just "@"
			if partial == "" {
				m.atBrowsePath = expandHomeDir("~/")
			}
		}

		m.showAtSuggestions(partial)
		return
	}

	// Detect / command trigger
	if strings.HasPrefix(text, "/") {
		// Parse command path from input
		// e.g., "/agents list" -> commandPath = ["agents"], partial = "list"
		m.parseCommandPath(text[1:])
		m.showSlashSuggestions(m.commandQuery)
		return
	}

	m.showSuggestions = false
	m.suggestionType = suggestNone
	m.commandPath = nil
	m.subCommands = nil
	m.atBrowsePath = ""
}

// parseCommandPath extracts the command path and remaining query from input.
// e.g., "/agents list" -> commandPath = ["agents"], commandQuery = "list"
func (m *model) parseCommandPath(input string) {
	input = strings.TrimSpace(input)
	parts := strings.Fields(input)
	if len(parts) == 0 {
		m.commandPath = nil
		m.commandQuery = ""
		return
	}
	if len(parts) == 1 {
		m.commandPath = nil
		m.commandQuery = ""
		return
	}
	m.commandPath = parts[:len(parts)-1]
	m.commandQuery = parts[len(parts)-1]
}

func (m *model) showSlashSuggestions(query string) {
	query = strings.ToLower(strings.TrimSpace(query))
	idx := strings.IndexAny(query, " \t")
	if idx >= 0 {
		query = query[:idx]
	}

	// If commandPath is set, we're browsing subcommands
	if len(m.commandPath) > 0 {
		m.showSlashSuggestionsAtLevel(m.commandQuery)
		return
	}

	cmds := m.registry.Filter(query)
	if len(cmds) == 0 {
		m.showSuggestions = false
		m.suggestionType = suggestNone
		return
	}

	m.showSuggestions = true
	m.suggestionType = suggestSlash
	m.suggestionData = nil

	names := make([]string, len(cmds))
	for i, c := range cmds {
		names[i] = c.Name
	}
	m.suggestions = names

	if m.selectedIdx < 0 || m.selectedIdx >= len(m.suggestions) {
		m.selectedIdx = 0
	}
}

// showSlashSuggestionsAtLevel displays subcommands at the current commandPath level.
func (m *model) showSlashSuggestionsAtLevel(query string) {
	var parentCmd *Command
	if len(m.commandPath) == 1 {
		parentCmd = m.registry.Find(m.commandPath[0])
	} else if len(m.commandPath) > 1 {
		// Find the parent in the grandparent's subcommands
		var grandparent *Command
		if len(m.commandPath) == 2 {
			grandparent = m.registry.Find(m.commandPath[0])
		}
		if grandparent != nil {
			for i := range grandparent.SubCommands {
				if grandparent.SubCommands[i].Name == m.commandPath[1] {
					parentCmd = &grandparent.SubCommands[i]
					break
				}
			}
		}
	}

	if parentCmd == nil || len(parentCmd.SubCommands) == 0 {
		m.showSuggestions = false
		return
	}

	// Filter subcommands by query if provided
	filtered := parentCmd.SubCommands
	if query != "" {
		var filteredCmds []Command
		for _, c := range parentCmd.SubCommands {
			if strings.HasPrefix(strings.ToLower(c.Name), strings.ToLower(query)) {
				filteredCmds = append(filteredCmds, c)
			}
		}
		if len(filteredCmds) > 0 {
			filtered = filteredCmds
		}
	}

	m.subCommands = filtered
	m.showSuggestions = true
	m.suggestionType = suggestSlash
	m.suggestionData = nil

	names := make([]string, len(m.subCommands))
	for i, c := range m.subCommands {
		names[i] = c.Name
	}
	m.suggestions = names
	m.selectedIdx = 0
}

func (m *model) showAtSuggestions(query string) {
	// Use m.atBrowsePath if set, otherwise default to ~/
	dir := m.atBrowsePath
	if dir == "" {
		dir = expandHomeDir("~/")
	}
	entries, err := lsFiles(dir)
	if err != nil {
		m.showSuggestions = false
		m.suggestionType = suggestNone
		return
	}

	// Filter by partial query (filename without path)
	if query != "" {
		var filtered []FileEntry
		for _, e := range entries {
			if strings.HasPrefix(strings.ToLower(e.Name), strings.ToLower(query)) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	if len(entries) == 0 {
		m.showSuggestions = false
		m.suggestionType = suggestNone
		return
	}

	m.showSuggestions = true
	m.suggestionType = suggestAt
	m.suggestionData = entries

	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	m.suggestions = names

	if m.selectedIdx < 0 || m.selectedIdx >= len(m.suggestions) {
		m.selectedIdx = 0
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m *model) View() string {
	if m.width == 0 {
		return "正在加载..."
	}

	var b strings.Builder

	// Message area - use viewport for scrolling
	m.viewport.SetContent(m.renderMessages())
	b.WriteString(m.viewport.View())

	// Status line: hints (left) + connection status + tokens (right)
	statusLine := m.renderStatusLine()

	// Input area - render inline with status
	inputArea := m.renderInputArea()

	// Combine into single block
	b.WriteString(statusLine)
	b.WriteString("\n")
	b.WriteString(inputArea)

	// Suggestion dropdown
	if m.showSuggestions && len(m.suggestions) > 0 {
		b.WriteString("\n")
		b.WriteString(m.renderSuggestions())
	}

	return b.String()
}

// renderMessages builds the full message content string for the viewport.
func (m *model) renderMessages() string {
	if len(m.messages) == 0 {
		return ""
	}

	var b strings.Builder
	for _, msg := range m.messages {
		var rendered string
		switch msg.role {
		case "user":
			rendered = userStyle.Render(fmt.Sprintf(" 你: %s", msg.content))
		case "assistant":
			rendered = assistantStyle.Render(fmt.Sprintf(" MindX:\n%s", indentString(msg.content, "   ")))
		case "command":
			rendered = systemStyle.Render(fmt.Sprintf(" %s", indentString(msg.content, " ")))
		case "system":
			rendered = systemStyle.Render(fmt.Sprintf(" ● %s", msg.content))
		case "error":
			rendered = errorStyle.Render(fmt.Sprintf(" ✗ %s", msg.content))
		}
		b.WriteString(rendered)
		b.WriteString("\n")
	}

	return b.String()
}

// renderStatusLine returns the single status line with hints (left) and status/tokens (right)
func (m *model) renderStatusLine() string {
	// Build hint on left
	hint := ""
	if m.showSuggestions {
		switch m.suggestionType {
		case suggestSlash:
			hint = "↑↓ Navigate  → Enter subcommand  Tab/Enter Accept  Esc Close"
		case suggestAt:
			hint = "↑↓ Navigate  → Enter directory  Tab/Enter Select  Esc Close"
		}
	} else if m.loading {
		hint = "Waiting for response..."
	}

	// Build status on right
	status := "🔌 Disconnected"
	if m.connected {
		status = "● Connected"
	}
	tokenStr := fmt.Sprintf("⬇ %d  ⬆ %d", m.tokensIn, m.tokensOut)

	// Calculate padding
	totalWidth := m.width - 4
	leftStr := inputHintStyle.Render(hint)
	rightStr := tokenCounterStyle.Render(status + "  " + tokenStr)
	leftWidth := lipgloss.Width(leftStr)
	rightWidth := lipgloss.Width(rightStr)
	padding := totalWidth - leftWidth - rightWidth
	if padding < 0 {
		padding = 0
	}

	return leftStr + strings.Repeat(" ", padding) + rightStr
}

func (m *model) renderInputArea() string {
	// Use textinput's native View() to preserve the cursor rendering.
	// The textinput already has Prompt="> " configured.
	return inputContainerStyle.Render(m.input.View())
}

// colorizeInput applies purple color to /command and @path patterns.
func (m *model) colorizeInput(text string) string {
	var result strings.Builder
	i := 0
	for i < len(text) {
		// Check for /command pattern
		if text[i] == '/' && (i == 0 || text[i-1] == ' ' || text[i-1] == '\t') {
			end := len(text)
			for j := i + 1; j < len(text); j++ {
				if text[j] == ' ' || text[j] == '\t' {
					end = j
					break
				}
			}
			result.WriteString(commandHighlightStyle.Render(text[i:end]))
			i = end
			continue
		}
		// Check for @path pattern
		if text[i] == '@' && (i == 0 || text[i-1] == ' ' || text[i-1] == '\t') {
			end := len(text)
			for j := i + 1; j < len(text); j++ {
				if text[j] == ' ' || text[j] == '\t' {
					end = j
					break
				}
			}
			result.WriteString(commandHighlightStyle.Render(text[i:end]))
			i = end
			continue
		}
		result.WriteByte(text[i])
		i++
	}
	return result.String()
}

func (m *model) renderSuggestions() string {
	n := len(m.suggestions)
	visible := n
	if visible > maxSuggestionRows {
		visible = maxSuggestionRows
	}

	start := m.selectedIdx - visible/2
	if start < 0 {
		start = 0
	}
	if start > n-visible {
		start = n - visible
	}

	var b strings.Builder
	for i := start; i < start+visible; i++ {
		var line string
		if m.suggestionType == suggestAt && i < len(m.suggestionData) {
			e := m.suggestionData[i]
			line = formatFileEntryForDisplay(e)
		} else {
			cmd := m.suggestions[i]
			if m.suggestionType == suggestSlash {
				// Build full command path for display
				fullPath := "/" + cmd
				if len(m.commandPath) > 0 {
					fullPath = "/" + strings.Join(m.commandPath, " ") + " " + cmd
				}

				// Find description from command or subcommand
				desc := ""
				if len(m.commandPath) == 0 {
					if c := m.registry.Find(cmd); c != nil {
						desc = c.Description
					}
				} else {
					for j := range m.subCommands {
						if m.subCommands[j].Name == cmd {
							desc = m.subCommands[j].Description
							break
						}
					}
				}

				// Show arrow indicator if command has subcommands
				hasSubcommands := false
				if len(m.commandPath) == 0 {
					if c := m.registry.Find(cmd); c != nil && len(c.SubCommands) > 0 {
						hasSubcommands = true
					}
				} else {
					for j := range m.subCommands {
						if m.subCommands[j].Name == cmd && len(m.subCommands[j].SubCommands) > 0 {
							hasSubcommands = true
							break
						}
					}
				}

				arrow := "  "
				if hasSubcommands {
					arrow = "▶ "
				}
				line = fmt.Sprintf("%s%-20s %s", arrow, fullPath, desc)
			} else {
				line = "  " + cmd
			}
		}

		if i == m.selectedIdx {
			b.WriteString(suggestionActiveStyle.Render(line))
		} else {
			b.WriteString(suggestionStyle.Render(line))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func sendCmd(client *gateway.Client, text string) tea.Cmd {
	return func() tea.Msg {
		if err := client.Send(&gateway.Message{
			Data: []byte(text),
		}); err != nil {
			return errMsg(fmt.Errorf("发送失败: %w", err))
		}
		return nil
	}
}

func parseCommandInput(input string) (name string, args string) {
	input = strings.TrimSpace(input)
	if len(input) < 2 || input[0] != '/' {
		return "", ""
	}
	input = input[1:]
	input = strings.TrimSpace(input)

	idx := strings.IndexAny(input, " \t")
	if idx == -1 {
		return strings.ToLower(input), ""
	}
	return strings.ToLower(input[:idx]), strings.TrimSpace(input[idx+1:])
}

func expandHomeDir(path string) string {
	if strings.HasPrefix(path, "~/") {
		usr, _ := user.Current()
		if usr != nil {
			return filepath.Join(usr.HomeDir, path[2:])
		}
	}
	return path
}

func formatFileEntryForDisplay(e FileEntry) string {
	if e.IsDir {
		return fmt.Sprintf("  📁 %s/", e.Name)
	}
	// Show size for files
	size := formatSize(e.Size)
	return fmt.Sprintf("  %-20s %s", e.Name, size)
}

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return strconv.FormatInt(bytes, 10) + " B"
	}
	if bytes < 1024*1024 {
		return strconv.FormatInt(bytes/1024, 10) + " KB"
	}
	return strconv.FormatInt(bytes/(1024*1024), 10) + " MB"
}

func indentString(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if i > 0 || len(lines) == 1 {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}
