package cli

import (
	"fmt"
	"math"
	"strings"
	"time"

	"mindx/pkg/i18n"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: i18n.T("cli.tui.short"),
	Long:  i18n.T("cli.tui.long"),
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		sessionID, _ := cmd.Flags().GetString("session")

		model := initialModel(port, sessionID)
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Println(i18n.TWithData("cli.tui.start_failed", map[string]interface{}{"Error": err.Error()}))
		}
	},
}

func init() {
	tuiCmd.Flags().IntP("port", "p", 1314, i18n.T("cli.tui.flag.port"))
	tuiCmd.Flags().StringP("session", "s", "", i18n.T("cli.tui.flag.session"))
}

type (
	tickMsg      time.Time
	connectedMsg struct {
		conn *websocket.Conn
	}
	wsMsg struct {
		msgType string
		content string
	}
	errorMsg error
)

type chatMessage struct {
	sender      string
	content     string
	timestamp   time.Time
	isUser      bool
	isThinking  bool
	isThought   bool
	thoughtType string
}

type model struct {
	conn              *websocket.Conn
	messages          []chatMessage
	input             string
	width             int
	height            int
	connected         bool
	connecting        bool
	err               error
	port              int
	sessionID         string
	scrollOffset      int
	msgChan           chan tea.Msg
	thinking          bool
	thinkingStep      int
	animFrame         int
	reconnectAttempts int
	inputHistory      []string
	historyIndex      int
}

var (
	spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

func getThinkingSteps() []string {
	return []string{
		i18n.T("cli.tui.thinking.step1"),
		i18n.T("cli.tui.thinking.step2"),
		i18n.T("cli.tui.thinking.step3"),
		i18n.T("cli.tui.thinking.step4"),
	}
}

func initialModel(port int, sessionID string) model {
	return model{
		port:         port,
		sessionID:    sessionID,
		messages:     []chatMessage{},
		connecting:   true,
		scrollOffset: 0,
		msgChan:      make(chan tea.Msg, 10),
		thinking:     false,
		thinkingStep: 0,
		animFrame:    0,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		connectWebSocket(m.port, m.sessionID, m.msgChan),
		listenToMsgChan(m.msgChan),
		animateCmd(),
	)
}

func listenToMsgChan(msgChan chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-msgChan
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			if m.conn != nil {
				m.conn.Close()
			}
			return m, tea.Quit

		case tea.KeyEnter:
			if m.connected && m.input != "" {
				cmd = sendMessage(m.conn, m.input)
				m.messages = append(m.messages, chatMessage{
					sender:    i18n.T("cli.tui.sender.you"),
					content:   m.input,
					timestamp: time.Now(),
					isUser:    true,
				})
				m.inputHistory = append(m.inputHistory, m.input)
				m.historyIndex = len(m.inputHistory)
				m.input = ""
				m.thinking = true
				m.thinkingStep = 0
				m.scrollToBottom()
			}

		case tea.KeyUp:
			if m.historyIndex > 0 {
				m.historyIndex--
				m.input = m.inputHistory[m.historyIndex]
			}

		case tea.KeyDown:
			if m.historyIndex < len(m.inputHistory) {
				m.historyIndex++
				if m.historyIndex == len(m.inputHistory) {
					m.input = ""
				} else {
					m.input = m.inputHistory[m.historyIndex]
				}
			}

		case tea.KeyBackspace:
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}

		default:
			if msg.Type == tea.KeyRunes {
				m.input += string(msg.Runes)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.scrollToBottom()

	case wsMsg:
		if msg.msgType == "message" {
			m.thinking = false
			m.messages = append(m.messages, chatMessage{
				sender:    "MindX",
				content:   msg.content,
				timestamp: time.Now(),
				isUser:    false,
			})
			m.scrollToBottom()
		} else if msg.msgType == "thinking" {
			m.messages = append(m.messages, chatMessage{
				sender:      "MindX",
				content:     msg.content,
				timestamp:   time.Now(),
				isUser:      false,
				isThought:   true,
				thoughtType: "thinking",
			})
			m.scrollToBottom()
		}
		cmd = listenToMsgChan(m.msgChan)

	case connectedMsg:
		m.conn = msg.conn
		m.connected = true
		m.connecting = false
		m.reconnectAttempts = 0
		m.messages = append(m.messages, chatMessage{
			sender:    i18n.T("cli.tui.sender.system"),
			content:   i18n.T("cli.tui.server_connected"),
			timestamp: time.Now(),
			isUser:    false,
		})
		m.scrollToBottom()
		cmd = listenToMsgChan(m.msgChan)

	case errorMsg:
		m.connecting = false
		m.thinking = false
		m.err = msg
		m.messages = append(m.messages, chatMessage{
			sender:    i18n.T("cli.tui.sender.error"),
			content:   msg.Error(),
			timestamp: time.Now(),
			isUser:    false,
		})
		m.scrollToBottom()

	case tickMsg:
		if m.thinking {
			m.thinkingStep = (m.thinkingStep + 1) % len(getThinkingSteps())
		}
		if !m.connected && !m.connecting && m.err == nil {
			backoff := time.Duration(math.Min(float64(500*time.Millisecond)*math.Pow(2, float64(m.reconnectAttempts)), float64(30*time.Second)))
			m.reconnectAttempts++
			m.connecting = true
			return m, tea.Batch(
				tea.Tick(backoff, func(t time.Time) tea.Msg {
					return nil
				}),
				connectWebSocket(m.port, m.sessionID, m.msgChan),
			)
		}
		return m, tea.Batch(tickCmd(), animateCmd())

	case struct{}:
		m.animFrame = (m.animFrame + 1) % len(spinnerFrames)
		return m, animateCmd()
	}

	return m, cmd
}

func animateCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return struct{}{}
	})
}

func (m model) View() string {
	if m.width == 0 {
		return i18n.T("cli.tui.init")
	}

	styles := setupStyles()

	header := styles.header.Render(
		lipgloss.JoinHorizontal(lipgloss.Center,
			styles.logo.Render("🧠 MindX"),
			styles.divider.Render(" │ "),
			styles.headerInfo.Render(fmt.Sprintf("Port: %d", m.port)),
		),
	)

	var status string
	if m.connecting {
		spinner := spinnerFrames[m.animFrame]
		status = styles.statusConnecting.Render(fmt.Sprintf("%s %s", spinner, i18n.T("cli.tui.connecting")))
	} else if m.connected {
		status = styles.statusConnected.Render(fmt.Sprintf("✓ %s", i18n.T("cli.tui.connected")))
	} else if m.err != nil {
		status = styles.statusError.Render(fmt.Sprintf("✗ %s", i18n.T("cli.tui.connection_failed")))
	}

	contentArea := styles.contentBox.Width(m.width - 4).Height(m.height - 8)

	messages := m.getVisibleMessages()
	messagesView := ""
	for _, msg := range messages {
		timestamp := msg.timestamp.Format("15:04:05")

		if msg.isThought {
			messagesView += fmt.Sprintf("%s\n", styles.thoughtBox.Render(
				fmt.Sprintf("%s %s", styles.thoughtIcon.Render("💭"), msg.content),
			))
			continue
		}

		if msg.isUser {
			msgContent := wrapText(msg.content, m.width-30)
			userBubble := styles.userBubble.Padding(1, 2).Render(msgContent)
			senderLine := lipgloss.JoinHorizontal(lipgloss.Right,
				styles.timestamp.Render(timestamp),
				" ",
				styles.userSender.Render(msg.sender),
				" ",
				styles.userAvatar.Render("👤"),
			)
			messagesView += lipgloss.JoinVertical(lipgloss.Right, senderLine, userBubble) + "\n\n"
		} else {
			msgContent := wrapText(msg.content, m.width-30)
			botBubble := styles.botBubble.Padding(1, 2).Render(msgContent)
			senderLine := lipgloss.JoinHorizontal(lipgloss.Left,
				styles.botAvatar.Render("🤖"),
				" ",
				styles.botSender.Render(msg.sender),
				" ",
				styles.timestamp.Render(timestamp),
			)
			messagesView += lipgloss.JoinVertical(lipgloss.Left, senderLine, botBubble) + "\n\n"
		}
	}

	if m.thinking {
		spinner := spinnerFrames[m.animFrame]
		thinkingText := getThinkingSteps()[m.thinkingStep]
		thinkingBox := styles.thinkingBox.Render(
			fmt.Sprintf("%s %s %s",
				spinner,
				styles.thinkingText.Render(i18n.T("cli.tui.thinking.label")),
				styles.thinkingStep.Render(thinkingText),
			),
		)
		messagesView += thinkingBox + "\n"
	}

	content := contentArea.Render(messagesView)

	inputPrompt := styles.inputPrompt.Render("> ")
	inputLine := styles.inputBox.Render(inputPrompt + m.input + styles.cursor.Render("▊"))

	footer := styles.footer.Render(
		lipgloss.JoinHorizontal(lipgloss.Center,
			styles.footerKey.Render("ESC/Ctrl+C"),
			" ",
			styles.footerText.Render(i18n.T("cli.tui.footer.quit")),
			styles.footerKey.Render("Enter"),
			" ",
			styles.footerText.Render(i18n.T("cli.tui.footer.send")),
		),
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		status,
		content,
		inputLine,
		footer,
	)
}

func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	var lines []string
	words := strings.Fields(text)
	var currentLine string
	for _, word := range words {
		if len(currentLine)+len(word)+1 > width {
			if currentLine != "" {
				lines = append(lines, currentLine)
			}
			currentLine = word
		} else {
			if currentLine != "" {
				currentLine += " "
			}
			currentLine += word
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	return strings.Join(lines, "\n")
}

func (m model) getVisibleMessages() []chatMessage {
	maxLines := m.height - 10
	if maxLines <= 0 {
		return m.messages
	}

	visibleCount := 0
	startIdx := len(m.messages)

	for i := len(m.messages) - 1; i >= 0; i-- {
		msg := m.messages[i]
		lines := strings.Count(msg.content, "\n") + 3
		visibleCount += lines
		if visibleCount >= maxLines {
			startIdx = i
			break
		}
	}

	if startIdx < 0 {
		startIdx = 0
	}

	return m.messages[startIdx:]
}

func (m *model) scrollToBottom() {
	m.scrollOffset = len(m.messages)
}

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func connectWebSocket(port int, sessionID string, msgChan chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		url := fmt.Sprintf("ws://localhost:%d/ws", port)
		if sessionID != "" {
			url += fmt.Sprintf("?session_id=%s", sessionID)
		}

		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			return errorMsg(fmt.Errorf("connection failed: %w", err))
		}

		go func() {
			defer conn.Close()
			for {
				var msg map[string]any
				if err := conn.ReadJSON(&msg); err != nil {
					return
				}

				msgType, ok := msg["type"].(string)
				if !ok {
					continue
				}

				switch msgType {
				case "message":
					if content, ok := msg["content"].(string); ok {
						msgChan <- wsMsg{msgType: "message", content: content}
					}
				case "thinking":
					if event, ok := msg["event"].(map[string]any); ok {
						content, _ := event["content"].(string)
						eventType, _ := event["type"].(string)
						if eventType == "chunk" {
							msgChan <- wsMsg{msgType: "thinking", content: content}
						} else if content != "" {
							displayContent := content
							if eventType != "" {
								displayContent = fmt.Sprintf("[%s] %s", eventType, content)
							}
							msgChan <- wsMsg{msgType: "thinking", content: displayContent}
						}
					}
				case "connected":
					msgChan <- connectedMsg{conn: conn}
				}
			}
		}()

		return nil
	}
}

func sendMessage(conn *websocket.Conn, content string) tea.Cmd {
	return func() tea.Msg {
		if conn == nil {
			return errorMsg(fmt.Errorf("not connected"))
		}

		msg := map[string]any{
			"type":    "message",
			"content": content,
		}

		if err := conn.WriteJSON(msg); err != nil {
			return errorMsg(fmt.Errorf("send failed: %w", err))
		}

		return nil
	}
}

type styles struct {
	header           lipgloss.Style
	logo             lipgloss.Style
	headerInfo       lipgloss.Style
	divider          lipgloss.Style
	statusConnected  lipgloss.Style
	statusConnecting lipgloss.Style
	statusError      lipgloss.Style
	contentBox       lipgloss.Style
	timestamp        lipgloss.Style
	userSender       lipgloss.Style
	botSender        lipgloss.Style
	userAvatar       lipgloss.Style
	botAvatar        lipgloss.Style
	userBubble       lipgloss.Style
	botBubble        lipgloss.Style
	thinkingBox      lipgloss.Style
	thinkingText     lipgloss.Style
	thinkingStep     lipgloss.Style
	thoughtBox       lipgloss.Style
	thoughtIcon      lipgloss.Style
	message          lipgloss.Style
	inputPrompt      lipgloss.Style
	inputBox         lipgloss.Style
	cursor           lipgloss.Style
	footer           lipgloss.Style
	footerKey        lipgloss.Style
	footerText       lipgloss.Style
}

func setupStyles() styles {
	return styles{
		header: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E0E7FF")).
			Background(lipgloss.Color("#1E1B4B")).
			Padding(0, 2).
			Bold(true).
			Height(1),

		logo: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A78BFA")).
			Bold(true),

		headerInfo: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C4B5FD")),

		divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6D28D9")),

		statusConnected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#34D399")).
			Background(lipgloss.Color("#064E3B")).
			Padding(0, 2).
			Bold(true),

		statusConnecting: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FBBF24")).
			Background(lipgloss.Color("#451A03")).
			Padding(0, 2),

		statusError: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F87171")).
			Background(lipgloss.Color("#450A0A")).
			Padding(0, 2).
			Bold(true),

		contentBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4C1D95")).
			BorderBackground(lipgloss.Color("#0F0A1E")).
			Background(lipgloss.Color("#0F0A1E")).
			Padding(1, 1),

		timestamp: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6D28D9")).
			Faint(true),

		userSender: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C4B5FD")).
			Bold(true),

		botSender: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A78BFA")).
			Bold(true),

		userAvatar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#34D399")),

		botAvatar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8B5CF6")),

		userBubble: lipgloss.NewStyle().
			Background(lipgloss.Color("#1E1B4B")).
			Foreground(lipgloss.Color("#E0E7FF")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4C1D95")),

		botBubble: lipgloss.NewStyle().
			Background(lipgloss.Color("#1F2937")).
			Foreground(lipgloss.Color("#E5E7EB")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4B5563")),

		thinkingBox: lipgloss.NewStyle().
			Background(lipgloss.Color("#1E1B4B")).
			Foreground(lipgloss.Color("#C4B5FD")).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#6D28D9")),

		thinkingText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A78BFA")).
			Bold(true),

		thinkingStep: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E0E7FF")),

		thoughtBox: lipgloss.NewStyle().
			Background(lipgloss.Color("#1F2937")).
			Foreground(lipgloss.Color("#9CA3AF")).
			Padding(0, 2).
			Italic(true),

		thoughtIcon: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")),

		message: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0")),

		inputPrompt: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A78BFA")).
			Bold(true),

		inputBox: lipgloss.NewStyle().
			Background(lipgloss.Color("#1E1B4B")).
			Foreground(lipgloss.Color("#E0E7FF")).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4C1D95")),

		cursor: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A78BFA")).
			Blink(true),

		footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6D28D9")).
			Background(lipgloss.Color("#0F0A1E")).
			Padding(0, 2),

		footerKey: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A78BFA")).
			Bold(true),

		footerText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6D28D9")),
	}
}
