package cli

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"mindx/pkg/i18n"
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
	wsMsg    string
	errorMsg error
)

type chatMessage struct {
	sender    string
	content   string
	timestamp time.Time
	isUser    bool
}

type model struct {
	conn         *websocket.Conn
	messages     []chatMessage
	input        string
	width        int
	height       int
	connected    bool
	connecting   bool
	err          error
	port         int
	sessionID    string
	scrollOffset int
	msgChan      chan tea.Msg
}

func initialModel(port int, sessionID string) model {
	return model{
		port:         port,
		sessionID:    sessionID,
		messages:     []chatMessage{},
		connecting:   true,
		scrollOffset: 0,
		msgChan:      make(chan tea.Msg, 10),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		connectWebSocket(m.port, m.sessionID, m.msgChan),
		listenToMsgChan(m.msgChan),
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
				m.input = ""
				m.scrollToBottom()
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
		m.messages = append(m.messages, chatMessage{
			sender:    "MindX",
			content:   string(msg),
			timestamp: time.Now(),
			isUser:    false,
		})
		m.scrollToBottom()
		cmd = listenToMsgChan(m.msgChan)

	case connectedMsg:
		m.conn = msg.conn
		m.connected = true
		m.connecting = false
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
		m.err = msg
		m.messages = append(m.messages, chatMessage{
			sender:    i18n.T("cli.tui.sender.error"),
			content:   msg.Error(),
			timestamp: time.Now(),
			isUser:    false,
		})
		m.scrollToBottom()

	case tickMsg:
		if !m.connected && !m.connecting && m.err == nil {
			m.connecting = true
			return m, connectWebSocket(m.port, m.sessionID, m.msgChan)
		}
		return m, tickCmd()
	}

	return m, cmd
}

func (m model) View() string {
	if m.width == 0 {
		return i18n.T("cli.tui.init")
	}

	styles := setupStyles()

	header := styles.header.Render(i18n.TWithData("cli.tui.header", map[string]interface{}{
		"Port":    m.port,
		"Session": m.sessionID,
	}))

	var status string
	if m.connecting {
		status = styles.statusConnecting.Render(i18n.T("cli.tui.connecting"))
	} else if m.connected {
		status = styles.statusConnected.Render(i18n.T("cli.tui.connected"))
	} else if m.err != nil {
		status = styles.statusError.Render(i18n.T("cli.tui.connection_failed"))
	}

	contentArea := styles.contentBox.Width(m.width - 4).Height(m.height - 6)

	messages := m.getVisibleMessages()
	messagesView := ""
	for _, msg := range messages {
		timestamp := msg.timestamp.Format("15:04:05")
		senderStyle := styles.userSender
		if !msg.isUser {
			senderStyle = styles.botSender
		}
		messagesView += fmt.Sprintf("%s %s: %s\n",
			styles.timestamp.Render(timestamp),
			senderStyle.Render(msg.sender),
			styles.message.Render(msg.content))
	}

	content := contentArea.Render(messagesView)

	inputPrompt := styles.inputPrompt.Render(i18n.T("cli.tui.input_prompt"))
	inputLine := styles.inputBox.Render(inputPrompt + m.input)

	footer := styles.footer.Render(i18n.T("cli.tui.footer"))

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		status,
		content,
		inputLine,
		footer,
	)
}

func (m model) getVisibleMessages() []chatMessage {
	maxLines := m.height - 8
	if maxLines <= 0 {
		return m.messages
	}

	visibleCount := 0
	startIdx := len(m.messages)

	for i := len(m.messages) - 1; i >= 0; i-- {
		msg := m.messages[i]
		lines := (len(msg.content) + len(msg.sender) + 20) / (m.width - 10)
		if lines < 1 {
			lines = 1
		}
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
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
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
			return errorMsg(fmt.Errorf("%s: %w", i18n.T("cli.tui.connection_failed"), err))
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
						msgChan <- wsMsg(content)
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
			return errorMsg(fmt.Errorf(i18n.T("cli.tui.not_connected")))
		}

		msg := map[string]any{
			"type":    "message",
			"content": content,
		}

		if err := conn.WriteJSON(msg); err != nil {
			return errorMsg(fmt.Errorf("%s: %w", i18n.T("cli.tui.send_failed"), err))
		}

		return nil
	}
}

type styles struct {
	header           lipgloss.Style
	statusConnected  lipgloss.Style
	statusConnecting lipgloss.Style
	statusError      lipgloss.Style
	contentBox       lipgloss.Style
	timestamp        lipgloss.Style
	userSender       lipgloss.Style
	botSender        lipgloss.Style
	message          lipgloss.Style
	inputPrompt      lipgloss.Style
	inputBox         lipgloss.Style
	footer           lipgloss.Style
}

func setupStyles() styles {
	return styles{
		header: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#86BBD8")).
			Background(lipgloss.Color("#2D3748")).
			Padding(0, 1).
			Bold(true),

		statusConnected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#98FB98")).
			Background(lipgloss.Color("#1D1F21")).
			Padding(0, 1),

		statusConnecting: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Background(lipgloss.Color("#1D1F21")).
			Padding(0, 1),

		statusError: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Background(lipgloss.Color("#1D1F21")).
			Padding(0, 1),

		contentBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4A5568")).
			Padding(0, 1),

		timestamp: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#718096")).
			Faint(true),

		userSender: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#86BBD8")).
			Bold(true),

		botSender: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F6AD55")).
			Bold(true),

		message: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0")),

		inputPrompt: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#86BBD8")).
			Bold(true),

		inputBox: lipgloss.NewStyle().
			Background(lipgloss.Color("#2D3748")).
			Padding(0, 1).
			Width(80),

		footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#718096")).
			Faint(true).
			Padding(0, 1),
	}
}
