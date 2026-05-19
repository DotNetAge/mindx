package conv

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/internal/client/style"
)

type ConversationPanel struct {
	Answers     []data.AnswerData
	SearchState data.SearchState
	BlinkOn     bool

	width    int
	height   int
	viewport viewport.Model
}

func New() *ConversationPanel {
	return &ConversationPanel{
		viewport: viewport.New(),
	}
}

func (p *ConversationPanel) Update(msg any) (*ConversationPanel, tea.Cmd) {
	switch m := msg.(type) {
	case clientmsg.ThinkingDeltaMsg:
		return p.handleThinkingDelta(m)
	case clientmsg.ThinkingDoneMsg:
		return p.handleThinkingDone(m)
	case clientmsg.ActionStartMsg:
		return p.handleActionStart(m)
	case clientmsg.ToolExecStartMsg:
		return p.handleToolExecStart(m)
	case clientmsg.ToolExecEndMsg:
		return p.handleToolExecEnd(m)
	case clientmsg.ActionProgressMsg:
		return p.handleActionProgress(m)
	case clientmsg.ActionEndMsg:
		return p.handleActionEnd(m)
	case clientmsg.ExecutionSummaryMsg:
		return p.handleExecutionSummary(m)
	case clientmsg.FinalAnswerMsg:
		return p.handleFinalAnswer(m)
	case clientmsg.AgentErrorMsg:
		return p.handleAgentError(m)
	case clientmsg.SessionDoneMsg:
		return p.handleSessionDone(m)
	case clientmsg.WindowResizeMsg:
		p.width = m.Width
		p.height = m.Height
		p.viewport.SetWidth(m.Width)
		vh := m.Height
		if vh > 2 {
			vh -= 2
		}
		p.viewport.SetHeight(vh)
	case clientmsg.TickMsg:
		p.BlinkOn = !p.BlinkOn
		if p.needsTick() {
			return p, p.tickCmd()
		}
	case clientmsg.CollapseToggleMsg:
		if m.AnswerIndex >= 0 && m.AnswerIndex < len(p.Answers) {
			if m.ActionIndex >= 0 && m.ActionIndex < len(p.Answers[m.AnswerIndex].Actions) {
				p.Answers[m.AnswerIndex].Actions[m.ActionIndex].Collapsed = !p.Answers[m.AnswerIndex].Actions[m.ActionIndex].Collapsed
			}
		}
	case clientmsg.ThinkCollapseMsg:
		if m.AnswerIndex >= 0 && m.AnswerIndex < len(p.Answers) {
			p.Answers[m.AnswerIndex].ThinkingCollapsed = !p.Answers[m.AnswerIndex].ThinkingCollapsed
		}
	case clientmsg.ClearScreenMsg:
		p.Answers = nil
	}
	return p, nil
}

func (p *ConversationPanel) handleThinkingDelta(m clientmsg.ThinkingDeltaMsg) (*ConversationPanel, tea.Cmd) {
	idx := p.findOrCreateAnswer(m.SessionID, "")
	if idx < 0 {
		return p, nil
	}
	a := &p.Answers[idx]
	if a.Status == data.StatusDone || a.Status == data.StatusError {
		return p, nil
	}
	a.PendingThink += m.Content
	a.Status = data.StatusThinking
	a.IsThinking = true
	return p, p.tickCmd()
}

func (p *ConversationPanel) handleThinkingDone(m clientmsg.ThinkingDoneMsg) (*ConversationPanel, tea.Cmd) {
	idx := p.canModify(m.SessionID)
	if idx < 0 {
		return p, nil
	}
	a := &p.Answers[idx]
	if a.PendingThink != "" {
		round := data.ThinkingRound{
			Content:   a.PendingThink,
			Timestamp: time.Now(),
		}
		a.ThinkingLog = append(a.ThinkingLog, round)
		a.PendingThink = ""
	}
	a.IsThinking = false
	return p, nil
}

func (p *ConversationPanel) handleActionStart(m clientmsg.ActionStartMsg) (*ConversationPanel, tea.Cmd) {
	idx := p.findOrCreateAnswer(m.SessionID, "")
	if idx < 0 {
		return p, nil
	}
	a := &p.Answers[idx]
	if a.Status == data.StatusDone || a.Status == data.StatusError {
		return p, nil
	}
	if a.PendingThink != "" {
		round := data.ThinkingRound{
			Content:   a.PendingThink,
			Timestamp: time.Now(),
		}
		a.ThinkingLog = append(a.ThinkingLog, round)
		a.PendingThink = ""
	}
	// Store action-level metadata
	a.CurrentAction = &data.ActionInfo{
		ToolCount:            m.ToolCount,
		ToolNames:            m.ToolNames,
		TotalPredictedTokens: m.EstimatedTok,
	}
	a.Status = data.StatusExecuting
	return p, p.tickCmd()
}

func (p *ConversationPanel) handleToolExecStart(m clientmsg.ToolExecStartMsg) (*ConversationPanel, tea.Cmd) {
	idx := p.canModify(m.SessionID)
	if idx < 0 {
		return p, nil
	}
	a := &p.Answers[idx]
	if a.Status == data.StatusDone || a.Status == data.StatusError {
		return p, nil
	}
	step := data.ActionStep{
		ToolName:     m.ToolName,
		Status:       data.ActionExecuting,
		EstimatedTok: m.EstimatedTok,
		Params:       m.Params,
		Collapsed:    true,
	}
	a.Actions = append(a.Actions, step)
	a.Status = data.StatusExecuting
	return p, p.tickCmd()
}

func (p *ConversationPanel) handleToolExecEnd(m clientmsg.ToolExecEndMsg) (*ConversationPanel, tea.Cmd) {
	idx := p.canModify(m.SessionID)
	if idx < 0 || len(p.Answers[idx].Actions) == 0 {
		return p, nil
	}
	// Match by tool name (last occurrence for multi-tool)
	for i := len(p.Answers[idx].Actions) - 1; i >= 0; i-- {
		step := &p.Answers[idx].Actions[i]
		if step.ToolName == m.ToolName && step.Status == data.ActionExecuting {
			if m.Success {
				step.Status = data.ActionDone
				step.ResultText = m.Result
			} else {
				step.Status = data.ActionFailed
				step.ResultText = m.Error
				step.Duration = m.Duration
			}
			break
		}
	}
	return p, nil
}

func (p *ConversationPanel) handleActionProgress(m clientmsg.ActionProgressMsg) (*ConversationPanel, tea.Cmd) {
	return p, nil
}

func (p *ConversationPanel) handleActionEnd(m clientmsg.ActionEndMsg) (*ConversationPanel, tea.Cmd) {
	idx := p.canModify(m.SessionID)
	if idx < 0 {
		return p, nil
	}
	a := &p.Answers[idx]
	a.ActionCompleted = true
	a.ActionSuccessCount = m.SuccessCount
	a.ActionFailedCount = m.FailedCount
	return p, p.tickCmd()
}

func (p *ConversationPanel) handleFinalAnswer(m clientmsg.FinalAnswerMsg) (*ConversationPanel, tea.Cmd) {
	idx := p.findOrCreateAnswer(m.SessionID, "")
	if idx < 0 {
		return p, nil
	}
	a := &p.Answers[idx]
	if a.Status == data.StatusResponding || a.Status == data.StatusError {
		return p, nil
	}
	if len(a.Results) > 0 {
		last := a.Results[len(a.Results)-1]
		if last.Role == "assistant" && last.Content == m.Content {
			return p, nil
		}
	}
	a.Results = append(a.Results, data.ResultEntry{Role: "assistant", Content: m.Content})
	a.Status = data.StatusResponding
	return p, nil
}

func (p *ConversationPanel) handleAgentError(m clientmsg.AgentErrorMsg) (*ConversationPanel, tea.Cmd) {
	idx := p.findOrCreateAnswer(m.SessionID, "")
	if idx < 0 {
		return p, nil
	}
	a := &p.Answers[idx]
	if a.Status == data.StatusDone || a.Status == data.StatusError {
		return p, nil
	}
	errMsg := m.Error.Error()
	if len(a.Results) > 0 {
		last := a.Results[len(a.Results)-1]
		if last.Role == "error" && last.Content == errMsg {
			return p, nil
		}
	}
	a.Results = append(a.Results, data.ResultEntry{Role: "error", Content: errMsg})
	a.Status = data.StatusError
	return p, nil
}

func (p *ConversationPanel) handleExecutionSummary(m clientmsg.ExecutionSummaryMsg) (*ConversationPanel, tea.Cmd) {
	idx := p.findAnswer(m.SessionID)
	if idx < 0 {
		return p, nil
	}
	a := &p.Answers[idx]
	a.TotalTokens = m.TokensUsed
	a.TotalDuration = m.Duration
	return p, nil
}

func (p *ConversationPanel) handleSessionDone(m clientmsg.SessionDoneMsg) (*ConversationPanel, tea.Cmd) {
	idx := p.findAnswer(m.SessionID)
	if idx < 0 {
		return p, nil
	}
	a := &p.Answers[idx]
	a.Status = data.StatusDone
	a.IsThinking = false
	return p, nil
}

func (p *ConversationPanel) findAnswer(sessionID string) int {
	for i, a := range p.Answers {
		if a.SessionID == sessionID {
			return i
		}
	}
	return -1
}

func (p *ConversationPanel) findOrCreateAnswer(sessionID, agentName string) int {
	if idx := p.findAnswer(sessionID); idx >= 0 {
		return idx
	}
	ans := data.AnswerData{
		SessionID:         sessionID,
		AgentName:         agentName,
		CreatedAt:         time.Now(),
		ThinkingCollapsed: true,
	}
	p.Answers = append(p.Answers, ans)
	return len(p.Answers) - 1
}

func (p *ConversationPanel) canModify(sessionID string) int {
	idx := p.findAnswer(sessionID)
	if idx < 0 {
		return -1
	}
	a := p.Answers[idx]
	if a.Status == data.StatusDone || a.Status == data.StatusError {
		return -1
	}
	return idx
}

func (p *ConversationPanel) needsTick() bool {
	for _, a := range p.Answers {
		if a.IsThinking {
			return true
		}
		for _, act := range a.Actions {
			if act.Status == data.ActionExecuting {
				return true
			}
		}
	}
	return false
}

func (p *ConversationPanel) tickCmd() tea.Cmd {
	return func() tea.Msg { return clientmsg.TickMsg{} }
}

func (p *ConversationPanel) ViewportUpdate(msg tea.Msg) {
	p.viewport, _ = p.viewport.Update(msg)
}

func (p *ConversationPanel) Clear() {
	p.Answers = nil
}

func (p *ConversationPanel) View() string {
	if p.width == 0 {
		p.width = 80
	}

	content := p.renderNormalView()
	if content == "" {
		return ""
	}
	p.viewport.SetWidth(p.width)
	vh := p.height
	if vh > 2 {
		vh -= 2
	}
	p.viewport.SetHeight(vh)
	p.viewport.SetContent(content)
	p.viewport.GotoBottom()
	return p.viewport.View()
}

func (p *ConversationPanel) renderThinkingSection(ans data.AnswerData) string {
	var b strings.Builder

	if ans.IsThinking {
		icon := style.CyanStyle.Render("● ")
		if p.BlinkOn {
			icon = style.WhiteStyle.Render("● ")
		}
		b.WriteString(icon)
		b.WriteString(style.DarkStyle.Render("深度思考"))
		b.WriteByte('\n')
	}

	for _, round := range ans.ThinkingLog {
		b.WriteString(p.renderThinkingRound(round, ans.ThinkingCollapsed))
	}
	if ans.PendingThink != "" {
		b.WriteString(p.renderPendingThink(ans.PendingThink))
	}
	return b.String()
}

func (p *ConversationPanel) renderActionSection(ans data.AnswerData) string {
	var b strings.Builder

	// Action-level header
	if ans.CurrentAction != nil {
		b.WriteString(p.renderActionHeader(*ans.CurrentAction))
	}

	// Tool-level steps (indented)
	for _, act := range ans.Actions {
		b.WriteString(indentText(p.renderActionStep(act), "  "))
	}

	// Action completion summary
	if ans.ActionCompleted {
		b.WriteString(p.renderActionSummary(ans))
	}

	return b.String()
}

func (p *ConversationPanel) renderActionHeader(info data.ActionInfo) string {
	var b strings.Builder
	icon := style.GreenStyle.Render("⏺ ")
	b.WriteString(icon)
	b.WriteString(style.WhiteStyle.Render(fmt.Sprintf("执行操作: %d 个工具", info.ToolCount)))
	if info.TotalPredictedTokens > 0 {
		b.WriteString(fmt.Sprintf(" | %s", style.GrayStyle.Render(fmt.Sprintf("预计消耗 %s Tokens", formatNumber(info.TotalPredictedTokens)))))
	}
	b.WriteByte('\n')
	return b.String()
}

func (p *ConversationPanel) renderActionSummary(ans data.AnswerData) string {
	var b strings.Builder
	total := ans.ActionSuccessCount + ans.ActionFailedCount
	icon := style.GreenStyle.Render("⏺ ")
	b.WriteString(icon)
	summary := fmt.Sprintf("操作完成: %d / %d 成功", ans.ActionSuccessCount, total)
	if ans.ActionFailedCount > 0 {
		summary += fmt.Sprintf(", %s 失败", style.RedStyle.Render(fmt.Sprintf("%d", ans.ActionFailedCount)))
	}
	b.WriteString(style.WhiteStyle.Render(summary))
	b.WriteByte('\n')
	return b.String()
}

func indentText(text, prefix string) string {
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

func (p *ConversationPanel) renderResultSection(ans data.AnswerData) string {
	var b strings.Builder
	for _, res := range ans.Results {
		b.WriteString(p.renderResultEntry(res))
	}
	return b.String()
}

func (p *ConversationPanel) renderNormalView() string {
	if len(p.Answers) == 0 {
		return ""
	}

	var blocks []string
	for _, ans := range p.Answers {
		blocks = append(blocks, p.renderAnswer(ans))
	}
	return strings.Join(blocks, "\n\n")
}

func (p *ConversationPanel) renderAnswer(ans data.AnswerData) string {
	var b strings.Builder

	if ans.UserQuestion != "" {
		b.WriteString(style.CyanStyle.Render("● "))
		b.WriteString(style.WhiteStyle.Render(ans.UserQuestion))
		b.WriteByte('\n')
	}

	hasThinking := len(ans.ThinkingLog) > 0 || ans.PendingThink != "" || ans.IsThinking
	hasActions := len(ans.Actions) > 0 || ans.CurrentAction != nil
	hasResults := len(ans.Results) > 0

	if hasThinking {
		b.WriteString(p.renderThinkingSection(ans))
	}
	if hasActions {
		b.WriteString(p.renderActionSection(ans))
	}
	if hasResults {
		b.WriteString(p.renderResultSection(ans))
	}
	return b.String()
}

func (p *ConversationPanel) renderThinkingRound(round data.ThinkingRound, collapsed bool) string {
	var b strings.Builder
	icon := style.CyanStyle.Render("● ")
	lines := strings.Split(round.Content, "\n")
	shouldCollapse := collapsed && len(lines) > 3
	displayLines := lines
	if shouldCollapse {
		displayLines = lines[len(lines)-3:]
	}
	for _, line := range displayLines {
		b.WriteString("  ")
		b.WriteString(style.DarkStyle.Render(line))
		b.WriteByte('\n')
	}
	if shouldCollapse {
		b.WriteString(fmt.Sprintf("  %s\n", style.GrayStyle.Render(fmt.Sprintf("… +%d lines (ctrl+o to expand)", len(lines)-3))))
	}
	if round.TokensIn > 0 || round.TokensOut > 0 {
		b.WriteString(fmt.Sprintf("  %s\n", style.DimStyle.Render(fmt.Sprintf("[Tokens: %s in / %s out]",
			formatNumber(round.TokensIn), formatNumber(round.TokensOut)))))
	}

	return icon + b.String()
}

func (p *ConversationPanel) renderPendingThink(content string) string {
	var b strings.Builder
	icon := style.CyanStyle.Render("● ")
	if p.BlinkOn {
		icon = style.WhiteStyle.Render("● ")
	}
	b.WriteString("  ")
	b.WriteString(style.DarkStyle.Render(content))
	b.WriteByte('\n')
	return icon + b.String()
}

func (p *ConversationPanel) renderActionStep(step data.ActionStep) string {
	var b strings.Builder
	var icon string
	switch step.Status {
	case data.ActionExecuting:
		if p.BlinkOn {
			icon = style.WhiteStyle.Render("⏺ ")
		} else {
			icon = style.GreenStyle.Render("⏺ ")
		}
	case data.ActionDone:
		icon = style.GreenStyle.Render("⏺ ")
	case data.ActionFailed:
		icon = style.RedStyle.Render("⏺ ")
	}

	paramStr := formatParams(step.Params)
	b.WriteString(icon)
	b.WriteString(style.WhiteStyle.Render(step.ToolName))
	if paramStr != "" {
		b.WriteString(fmt.Sprintf("(%s)", paramStr))
	}
	if step.EstimatedTok > 0 && step.Status == data.ActionExecuting {
		b.WriteString(fmt.Sprintf(" | %s", style.GrayStyle.Render(fmt.Sprintf("预计消耗 %s Tokens", formatNumber(step.EstimatedTok)))))
	} else if step.EstimatedTok > 0 {
		b.WriteString(fmt.Sprintf(" | %s", style.GrayStyle.Render(fmt.Sprintf("预计消耗 %s Tokens", formatNumber(step.EstimatedTok)))))
	}
	if step.ProgressText != "" {
		b.WriteString(fmt.Sprintf(" | %s", style.GrayStyle.Render(step.ProgressText)))
	}

	if step.Status == data.ActionFailed {
		if step.ResultText != "" {
			b.WriteString(fmt.Sprintf(" | failed: %s\n", style.RedStyle.Render(step.ResultText)))
		} else {
			b.WriteByte('\n')
		}
		return b.String()
	}

	b.WriteByte('\n')

	if step.ResultText != "" {
		if step.Collapsed {
			lines := strings.Split(step.ResultText, "\n")
			b.WriteString(fmt.Sprintf("  ⎿ %s\n", style.GrayStyle.Render(fmt.Sprintf("完成 (%d lines)", len(lines)))))
		} else {
			lines := strings.Split(step.ResultText, "\n")
			for i, line := range lines {
				if i >= 3 {
					b.WriteString(fmt.Sprintf("    … +%d lines (ctrl+o to expand)\n", len(lines)-i))
					break
				}
				b.WriteString(fmt.Sprintf("  ⎿ %s\n", style.DimStyle.Render(line)))
			}
		}
	}
	return b.String()
}

func (p *ConversationPanel) renderResultEntry(res data.ResultEntry) string {
	if res.Role == "error" {
		return fmt.Sprintf("%s%s\n", style.RedStyle.Render("⏺ "), res.Content)
	}
	var b strings.Builder
	b.WriteString(style.WhiteStyle.Render("⏺ "))
	b.WriteString(render.MarkdownWithWidth(res.Content, p.width-4))
	b.WriteByte('\n')
	return b.String()
}

func formatParams(params map[string]any) string {
	if params == nil || len(params) == 0 {
		return ""
	}
	var parts []string
	for k, v := range params {
		parts = append(parts, fmt.Sprintf("%v=%v", k, v))
	}
	result := strings.Join(parts, ", ")
	if len(result) > 60 {
		return result[:57] + "..."
	}
	return result
}

func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	runes := []rune(s)
	var result []rune
	for i, r := range runes {
		pos := len(runes) - i
		if pos%3 == 0 && i != 0 {
			result = append(result, ',')
		}
		result = append(result, r)
	}
	return string(result)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	if mins > 0 {
		return fmt.Sprintf("%dm %ds", mins, secs)
	}
	return fmt.Sprintf("%ds", secs)
}

func formatTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
