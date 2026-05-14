package client

import "strings"

// Header 是顶栏组件，显示 MindX² 品牌标识、连接状态和 agent/model 信息。
// 首次启动时显示完整的 ASCII logo，之后自动切换为紧凑状态栏。
type Header struct {
	collapsed bool
	width     int

	connected  bool
	agentName  string
	modelName  string
	modeText   string
	sessionName string

	showLogo bool // 仅在首次初始化时显示完整 ASCII logo
}

// NewHeader 创建一个新的 Header 组件。
func NewHeader() *Header {
	return &Header{
		collapsed:    false,
		showLogo:     true,
		connected:    false,
		modeText:     "Normal",
	}
}

// SetCollapsed 设置折叠状态。
func (h *Header) SetCollapsed(v bool) { h.collapsed = v }

// SetConnected 设置连接状态。
func (h *Header) SetConnected(v bool) { h.connected = v }

// SetAgent 设置当前 agent 名称和模型。
func (h *Header) SetAgent(name, model string) { h.agentName = name; h.modelName = model }

// SetMode 设置当前模式文本（如 Prompt/Fast/Normal）。
func (h *Header) SetMode(text string) { h.modeText = text }

// SetSessionName 设置会话名称。
func (h *Header) SetSessionName(name string) { h.sessionName = name }

// SetWidth 设置宽度。
func (h *Header) SetWidth(w int) { h.width = w }

// Height 返回 Header 当前占用的行数。
func (h *Header) Height() int {
	if h.collapsed {
		return 0
	}
	if h.showLogo {
		return 10 // 6 行 logo + 3 行状态 + 1 行分隔线
	}
	return 2 // 1 行紧凑状态 + 1 行分隔线
}

// DismissLogo 在首次渲染后调用，将 showLogo 设为 false 以切换到紧凑模式。
func (h *Header) DismissLogo() {
	h.showLogo = false
}

// View 返回 Header 的渲染字符串。
func (h *Header) View() string {
	if h.collapsed || h.width == 0 {
		return ""
	}

	var b strings.Builder

	if h.showLogo {
		// 完整 ASCII logo 模式
		logo := MindxLogo()
		b.WriteString(HeaderLogoStyle.Render(logo))
		b.WriteString("\n")

		// 版本行
		versionLine := "MindX² v" + MindxVersion
		b.WriteString(HeaderLogoStyle.Render(versionLine))
		b.WriteString("\n")

		// 状态行
		statusParts := []string{}
		if h.connected {
			statusParts = append(statusParts, ConnectedDot.Render()+" Connected")
		} else {
			statusParts = append(statusParts, DisconnectedDot.Render()+" Disconnected")
		}
		if h.agentName != "" {
			statusParts = append(statusParts, "Agent: "+h.agentName)
		}
		if h.modelName != "" {
			statusParts = append(statusParts, "Model: "+h.modelName)
		}
		b.WriteString(HeaderStatusStyle.Render(strings.Join(statusParts, "  |  ")))
		b.WriteString("\n")
	} else {
		// 紧凑模式：单行状态
		parts := []string{}
		if h.connected {
			parts = append(parts, ConnectedDot.Render())
		} else {
			parts = append(parts, DisconnectedDot.Render())
		}
		if h.agentName != "" {
			parts = append(parts, h.agentName)
		}
		if h.modelName != "" {
			parts = append(parts, h.modelName)
		}
		if h.modeText != "" {
			parts = append(parts, h.modeText)
		}
		if h.sessionName != "" {
			parts = append(parts, h.sessionName)
		}
		line := strings.Join(parts, "  ")
		b.WriteString(HeaderLogoStyle.Render(line))
		b.WriteString("\n")
	}

	// 分隔线
	if h.width > 0 {
		b.WriteString(DividerStyle.Render(strings.Repeat("─", h.width)))
	} else {
		b.WriteString(DividerStyle.Render(strings.Repeat("─", 40)))
	}

	return b.String()
}
