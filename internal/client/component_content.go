package client

import (
	"fmt"
	"strings"
	"time"

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
	appTitle   string
	version     string
	agentName   string
	workspace   string
	sessionID   string
	projectDir  string
}

// ViewMode 表示内容面板的显示模式。
type ViewMode int

const (
	ViewModeNormal     ViewMode = iota // 默认对话模式
	ViewModeTranscript                  // 对话历史模式（编号、时间戳、搜索）
	ViewModeFullscreen                  // 全屏沉浸模式
)

// ContentPanel 管理可滚动内容区，包含 Welcome（一次性）和 AgentAnswer 列表。
type ContentPanel struct {
	width, height   int
	viewport        viewport.Model
	glamourRenderer *glamour.TermRenderer
	welcome         WelcomeData
	welcomeShown    bool
	answers         []*AgentAnswer
	renderers       map[string]ResultRenderer

	// 视图模式
	viewMode ViewMode

	// Transcript 模式相关
	truncateAbove int // 超过此数量时隐藏较早消息
	hiddenCount   int // 被隐藏的消息数

	// Search related
	searchQuery     string
	searchMatches   []matchLocation
	currentMatchIdx int

	dirty bool // 脏标记，避免高频重渲染
}

func NewContentPanel() *ContentPanel {
	vp := viewport.New()
	vp.SoftWrap = true
	vp.FillHeight = true
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
	// 移除默认边距（defaultMargin=2），让内容使用终端全宽度
	zero := uint(0)
	style.Document.Margin = &zero
	style.CodeBlock.Margin = &zero

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

// SetViewMode 设置视图模式。
func (p *ContentPanel) SetViewMode(mode ViewMode) {
	p.viewMode = mode
	p.refreshOnUpdate()
}

// ToggleTranscript 切换普通模式和 Transcript 模式。
func (p *ContentPanel) ToggleTranscript() {
	if p.viewMode == ViewModeTranscript {
		p.viewMode = ViewModeNormal
	} else {
		p.viewMode = ViewModeTranscript
	}
	p.refreshOnUpdate()
}

// ToggleFullscreen 切换普通模式和全屏模式。
func (p *ContentPanel) ToggleFullscreen() {
	if p.viewMode == ViewModeFullscreen {
		p.viewMode = ViewModeNormal
	} else {
		p.viewMode = ViewModeFullscreen
	}
	p.refreshOnUpdate()
}

// matchLocation 表示搜索结果中的一个匹配位置。
type matchLocation struct {
	answerIdx int
	resultIdx int
	startPos  int
	endPos    int
}

// SetSearchQuery 设置搜索查询并执行搜索。
func (p *ContentPanel) SetSearchQuery(query string) int {
	p.searchQuery = query
	if query == "" {
		p.searchMatches = nil
		p.currentMatchIdx = 0
		return 0
	}
	return p.performSearch(query)
}

// performSearch 在 answers 中执行搜索，返回匹配数量。
func (p *ContentPanel) performSearch(query string) int {
	p.searchMatches = nil
	p.currentMatchIdx = 0
	lowerQuery := strings.ToLower(query)

	for ai, ans := range p.answers {
		// Search in user question
		content := strings.ToLower(ans.userQuestion)
		if idx := strings.Index(content, lowerQuery); idx >= 0 {
			p.searchMatches = append(p.searchMatches, matchLocation{
				answerIdx: ai,
				resultIdx: -1,
				startPos:  idx,
				endPos:    idx + len(query),
			})
		}
		// Search in results
		for ri, res := range ans.results {
			content := strings.ToLower(res.Content)
			offset := 0
			for {
				idx := strings.Index(content[offset:], lowerQuery)
				if idx < 0 {
					break
				}
				p.searchMatches = append(p.searchMatches, matchLocation{
					answerIdx: ai,
					resultIdx: ri,
					startPos:  offset + idx,
					endPos:    offset + idx + len(query),
				})
				offset += idx + 1
			}
		}
	}

	return len(p.searchMatches)
}

// SearchResult 返回当前匹配的信息（用于 SearchModel 显示）。
func (p *ContentPanel) SearchResult() (current, total int) {
	return p.currentMatchIdx + 1, len(p.searchMatches)
}

// SearchNext 移动到下一个匹配。
func (p *ContentPanel) SearchNext() {
	if len(p.searchMatches) == 0 {
		return
	}
	p.currentMatchIdx = (p.currentMatchIdx + 1) % len(p.searchMatches)
	p.refreshOnUpdate()
}

// SearchPrev 移动到上一个匹配。
func (p *ContentPanel) SearchPrev() {
	if len(p.searchMatches) == 0 {
		return
	}
	p.currentMatchIdx--
	if p.currentMatchIdx < 0 {
		p.currentMatchIdx = len(p.searchMatches) - 1
	}
	p.refreshOnUpdate()
}

// ToggleActionCollapse 切换指定答案中指定动作的折叠状态。
func (p *ContentPanel) ToggleActionCollapse(answerIdx, actionIdx int) {
	if answerIdx < 0 || answerIdx >= len(p.answers) {
		return
	}
	p.answers[answerIdx].ToggleActionCollapse(actionIdx)
	p.refreshOnUpdate()
}

// ClearAll 清空所有内容（Ctrl+L），Welcome 不再恢复。
func (p *ContentPanel) ClearAll() {
	p.answers = nil
	p.welcomeShown = false
	p.doRefresh()
}

// ShowWelcome 显示 Welcome 信息，仅在首次启动时调用一次。
func (p *ContentPanel) ShowWelcome(appTitle, version, workspace, sessionID, agentName string, projectDir ...string) {
	pd := ""
	if len(projectDir) > 0 {
		pd = projectDir[0]
	}
	p.welcome = WelcomeData{
		appTitle:   appTitle,
		version:     version,
		agentName:   agentName,
		workspace:   workspace,
		sessionID:   sessionID,
		projectDir:  pd,
	}

	var b strings.Builder
	b.WriteString(ThemeTitleStyle.Render(appTitle))
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
	if pd != "" {
		lines = append(lines, fmt.Sprintf("Project Dir: %s", pd))
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
	b.WriteString(ThemeTitleStyle.Render(p.welcome.appTitle))
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
	if p.welcome.projectDir != "" {
		lines = append(lines, fmt.Sprintf("Project Dir: %s", p.welcome.projectDir))
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
	var content string
	switch p.viewMode {
	case ViewModeTranscript:
		content = p.renderTranscriptView()
	default:
		content = p.renderNormalView()
	}

	if content == "" {
		return
	}

	p.viewport.SetContent(content)
	if p.viewMode == ViewModeNormal {
		p.viewport.GotoBottom()
	}
}

// renderNormalView 渲染普通模式的视图。
func (p *ContentPanel) renderNormalView() string {
	if p.welcomeShown && len(p.answers) == 0 {
		return ""
	}

	var b strings.Builder
	for _, a := range p.answers {
		b.WriteString(a.View())
		b.WriteString("\n")
	}
	return b.String()
}

// renderTranscriptView 渲染 Transcript 模式视图。
// 显示消息编号、时间戳、时间分组分隔线，以及消息截断。
func (p *ContentPanel) renderTranscriptView() string {
	if len(p.answers) == 0 {
		return p.renderNormalView()
	}

	// 截断策略：保留最近 truncateAbove 条，显示隐藏计数
	startIdx := 0
	total := len(p.answers)
	p.hiddenCount = 0
	if p.truncateAbove > 0 && total > p.truncateAbove {
		p.hiddenCount = total - p.truncateAbove
		startIdx = p.hiddenCount
	}

	var b strings.Builder

	// 隐藏消息提示
	if p.hiddenCount > 0 {
		b.WriteString(DividerStyle.Render(fmt.Sprintf("── %d messages hidden ──", p.hiddenCount)))
		b.WriteString("\n")
	}

	for i := startIdx; i < total; i++ {
		a := p.answers[i]

		// 时间分组分隔线（相邻消息间隔 > 5 分钟时）
		if i > startIdx && !a.CreatedAt.IsZero() {
			prev := p.answers[i-1]
			if !prev.CreatedAt.IsZero() {
				diff := a.CreatedAt.Sub(prev.CreatedAt)
				if diff > 5*time.Minute {
					timeStr := a.CreatedAt.Format("15:04")
					b.WriteString(DividerStyle.Render(fmt.Sprintf("───── %s ─────", timeStr)))
					b.WriteString("\n")
				}
			}
		}

		// 消息编号和时间戳
		if !a.CreatedAt.IsZero() {
			timeStr := a.CreatedAt.Format("15:04:05")
			b.WriteString(TimestampStyle.Render(fmt.Sprintf("[%d/%d] %s", i+1, total, timeStr)))
			b.WriteString("\n")
		} else {
			b.WriteString(TimestampStyle.Render(fmt.Sprintf("[%d/%d]", i+1, total)))
			b.WriteString("\n")
		}

		// 渲染消息内容
		b.WriteString(a.View())
		b.WriteString("\n")
	}

	return b.String()
}