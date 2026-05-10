package client

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	"os"
)

// ResultRenderer 是内容渲染器接口，用于支持可扩展的内容格式。
type ResultRenderer func(content string, width int) string

// WelcomeData 欢迎面板数据结构。
type WelcomeData struct {
	appTitle  string
	version   string
	agentName string
	workspace string
	sessionID string
}

// ContentPanel 管理可滚动内容区，包含 Welcome（一次性）和 AgentAnswer 列表。
type ContentPanel struct {
	width, height   int
	viewport        viewport.Model
	glamourRenderer *glamour.TermRenderer
	welcome         WelcomeData
	welcomeShown    bool
	answers         []*AgentAnswer
	renderers       map[string]ResultRenderer

	dirty bool // 脏标记，避免高频重渲染
}

func NewContentPanel() *ContentPanel {
	vp := viewport.New()
	vp.SoftWrap = true
	return &ContentPanel{
		renderers: make(map[string]ResultRenderer),
		viewport:  vp,
	}
}

// RegisterRenderer 注册一个内容渲染器，支持按 contentType 动态路由。
func (p *ContentPanel) RegisterRenderer(contentType string, renderer ResultRenderer) {
	p.renderers[contentType] = renderer
}

// GetRenderer 根据 contentType 获取注册的渲染器，若未注册返回 nil。
func (p *ContentPanel) GetRenderer(contentType string) ResultRenderer {
	return p.renderers[contentType]
}

// SetSize 更新尺寸，由 Root 在 WindowSizeMsg 时调用。
func (p *ContentPanel) SetSize(w, h int) {
	p.width = w
	p.height = h
	p.viewport.SetWidth(w)
	p.viewport.SetHeight(h)
	p.viewport.GotoBottom()
	if p.glamourRenderer == nil {
		p.initGlamour()
	}
}

func (p *ContentPanel) initGlamour() {
	if p.width == 0 {
		return
	}
	isDark := lipgloss.HasDarkBackground(os.Stdout, os.Stdin)
	style := styles.DarkStyleConfig
	if !isDark {
		style = styles.LightStyleConfig
	}
	width := p.width
	if width < 40 {
		width = 40
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return
	}
	p.glamourRenderer = r
}

// Update 处理 viewport 相关的消息（鼠标滚动、键盘滚动等）。
func (p *ContentPanel) Update(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		v, _ := p.viewport.Update(msg)
		p.viewport = v
	case tea.KeyPressMsg:
		// 传递键盘滚动事件给 viewport
		v, _ := p.viewport.Update(msg)
		p.viewport = v
	}
}

// RenderMarkdown 用 glamour 渲染 Markdown 内容。若渲染器未初始化则返回原文。
func (p *ContentPanel) RenderMarkdown(content string) string {
	if p.glamourRenderer == nil {
		return content
	}
	r, err := p.glamourRenderer.Render(content)
	if err != nil {
		return content
	}
	return r
}

// RenderTyped 根据内容类型查找注册的渲染器进行渲染，未注册则 fallback 到 Markdown。
func (p *ContentPanel) RenderTyped(contentType, content string) string {
	if renderer := p.GetRenderer(contentType); renderer != nil {
		return renderer(content, p.width)
	}
	return p.RenderMarkdown(content)
}

// CreateAnswer 创建一个新的 AgentAnswer，追加到列表尾部，返回指针供 session 路由。
func (p *ContentPanel) CreateAnswer(sessionID, agentName string) *AgentAnswer {
	a := NewAgentAnswer(sessionID, agentName)
	a.markdownFn = p.RenderMarkdown
	p.answers = append(p.answers, a)
	p.refreshOnUpdate()
	return a
}

// LatestAnswer 返回当前活跃的最后一个 AgentAnswer，用于路由无 sessionID 的旧事件。
func (p *ContentPanel) LatestAnswer() *AgentAnswer {
	if len(p.answers) == 0 {
		return nil
	}
	return p.answers[len(p.answers)-1]
}

// FindAnswer 按 sessionID 查找 AgentAnswer。
func (p *ContentPanel) FindAnswer(sessionID string) *AgentAnswer {
	for _, a := range p.answers {
		if a.SessionID == sessionID {
			return a
		}
	}
	return nil
}

// ClearAll 清空所有内容（Ctrl+L），Welcome 不再恢复。
func (p *ContentPanel) ClearAll() {
	p.answers = nil
	p.welcomeShown = false
	p.doRefresh()
}

// ShowWelcome 显示 Welcome 信息，仅在首次启动时调用一次。
func (p *ContentPanel) ShowWelcome(appTitle, version, workspace, sessionID, agentName string) {
	p.welcome = WelcomeData{
		appTitle:  appTitle,
		version:   version,
		agentName: agentName,
		workspace: workspace,
		sessionID: sessionID,
	}

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#BB86FC")).Render(appTitle))
	if version != "" {
		b.WriteString("  " + version)
	}
	b.WriteString("\n")

	lines := []string{
		fmt.Sprintf("Agent: %s", agentName),
		fmt.Sprintf("Session: %s", sessionID),
	}
	if workspace != "" {
		lines = append(lines, fmt.Sprintf("Workspace: %s", workspace))
	}
	b.WriteString(strings.Join(lines, "\n"))
	b.WriteString("\n")

	p.viewport.SetContent(b.String())
	p.welcomeShown = true
}

// UpdateWelcomeAgent 更新 Welcome 中显示的 AgentName，用于 agents 数据获取后更新。
func (p *ContentPanel) UpdateWelcomeAgent(agentName string) {
	if !p.welcomeShown || agentName == "" {
		return
	}
	if p.welcome.agentName == agentName {
		return
	}
	p.welcome.agentName = agentName

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#BB86FC")).Render(p.welcome.appTitle))
	if p.welcome.version != "" {
		b.WriteString("  " + p.welcome.version)
	}
	b.WriteString("\n")

	lines := []string{
		fmt.Sprintf("Agent: %s", agentName),
		fmt.Sprintf("Session: %s", p.welcome.sessionID),
	}
	if p.welcome.workspace != "" {
		lines = append(lines, fmt.Sprintf("Workspace: %s", p.welcome.workspace))
	}
	b.WriteString(strings.Join(lines, "\n"))
	b.WriteString("\n")

	p.viewport.SetContent(b.String())
}

// refreshView 根据当前 answers 重新渲染 viewport 内容。
func (p *ContentPanel) refreshView() {
	var b strings.Builder
	if p.welcomeShown && len(p.answers) == 0 {
		return
	}

	for _, a := range p.answers {
		b.WriteString(a.View())
		b.WriteString("\n")
	}
	content := b.String()
	if content == "" {
		return
	}
	p.viewport.SetContent(content)
	p.viewport.GotoBottom()
}

// View 返回渲染后的字符串。仅在脏标记为 true 时执行实际重渲染。
func (p *ContentPanel) View() string {
	if p.dirty {
		p.doRefresh()
		p.dirty = false
	}
	return p.viewport.View()
}

// refreshOnUpdate 标记内容需要刷新，由外部路由逻辑调用。
// 不立即执行重渲染，延迟到下次 View() 调用时批量处理。
func (p *ContentPanel) refreshOnUpdate() {
	p.dirty = true
}

// doRefresh 执行实际的 viewport 内容重渲染。
func (p *ContentPanel) doRefresh() {
	var b strings.Builder
	if p.welcomeShown && len(p.answers) == 0 {
		return
	}

	for _, a := range p.answers {
		b.WriteString(a.View())
		b.WriteString("\n")
	}
	content := b.String()
	if content == "" {
		return
	}
	p.viewport.SetContent(content)
	p.viewport.GotoBottom()
}