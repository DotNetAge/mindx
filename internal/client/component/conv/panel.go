package conv

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
)

type ConversationPanel struct {
	Answers     []data.AnswerData
	SearchState data.SearchState
	WelcomeData data.WelcomeData
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
	case clientmsg.ActionProgressMsg:
		return p.handleActionProgress(m)
	case clientmsg.ActionResultMsg:
		return p.handleActionResult(m)
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
	a.PendingThink += m.Content
	a.Status = data.StatusThinking
	a.IsThinking = true
	return p, p.tickCmd()
}

func (p *ConversationPanel) handleThinkingDone(m clientmsg.ThinkingDoneMsg) (*ConversationPanel, tea.Cmd) {
	idx := p.findAnswer(m.SessionID)
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
	if a.PendingThink != "" {
		round := data.ThinkingRound{
			Content:   a.PendingThink,
			Timestamp: time.Now(),
		}
		a.ThinkingLog = append(a.ThinkingLog, round)
		a.PendingThink = ""
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

func (p *ConversationPanel) handleActionProgress(m clientmsg.ActionProgressMsg) (*ConversationPanel, tea.Cmd) {
	idx := p.findAnswer(m.SessionID)
	if idx < 0 || len(p.Answers[idx].Actions) == 0 {
		return p, nil
	}
	last := &p.Answers[idx].Actions[len(p.Answers[idx].Actions)-1]
	if last.ToolName == m.ToolName {
		last.ProgressText = m.Progress
	}
	return p, nil
}

func (p *ConversationPanel) handleActionResult(m clientmsg.ActionResultMsg) (*ConversationPanel, tea.Cmd) {
	idx := p.findAnswer(m.SessionID)
	if idx < 0 || len(p.Answers[idx].Actions) == 0 {
		return p, nil
	}
	last := &p.Answers[idx].Actions[len(p.Answers[idx].Actions)-1]
	if last.ToolName == m.ToolName {
		if m.Success {
			last.Status = data.ActionDone
			last.ResultText = m.Result
		} else {
			last.Status = data.ActionFailed
			last.ResultText = m.Error
		}
	}
	return p, nil
}

func (p *ConversationPanel) handleFinalAnswer(m clientmsg.FinalAnswerMsg) (*ConversationPanel, tea.Cmd) {
	idx := p.findOrCreateAnswer(m.SessionID, "")
	if idx < 0 {
		return p, nil
	}
	a := &p.Answers[idx]
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
	a.Results = append(a.Results, data.ResultEntry{Role: "error", Content: m.Error.Error()})
	a.Status = data.StatusError
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

func (p *ConversationPanel) View() string {
	if p.width == 0 {
		p.width = 80
	}

	if len(p.Answers) == 0 {
		return p.renderWelcome()
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
	for i, round := range ans.ThinkingLog {
		b.WriteString(p.renderThinkingRound(round, i))
	}
	if ans.PendingThink != "" {
		b.WriteString(p.renderPendingThink(ans.PendingThink))
	} else if ans.IsThinking {
		icon := style.CyanStyle.Render("● ")
		if p.BlinkOn {
			icon = style.WhiteStyle.Render("● ")
		}
		b.WriteString(icon)
		b.WriteString(style.DarkStyle.Render("深度思考中..."))
		b.WriteByte('\n')
	}
	return b.String()
}

func (p *ConversationPanel) renderActionSection(ans data.AnswerData) string {
	var b strings.Builder
	for _, act := range ans.Actions {
		b.WriteString(p.renderActionStep(act))
	}
	return b.String()
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

	hasThinking := len(ans.ThinkingLog) > 0 || ans.PendingThink != ""
	hasActions := len(ans.Actions) > 0
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

func (p *ConversationPanel) renderThinkingRound(round data.ThinkingRound, idx int) string {
	var b strings.Builder
	icon := style.CyanStyle.Render("● ")
	lines := strings.Split(round.Content, "\n")
	displayLines := lines
	collapsed := idx > 0

	if collapsed && len(lines) > 3 {
		displayLines = lines[:3]
	}
	for _, line := range displayLines {
		b.WriteString("  ")
		b.WriteString(style.DarkStyle.Render(line))
		b.WriteByte('\n')
	}
	if collapsed && len(lines) > 3 {
		b.WriteString(fmt.Sprintf("  %s\n", style.GrayStyle.Render(fmt.Sprintf("… +%d lines (ctrl+o to expand)", len(lines)-3))))
	}
	if round.TokensIn > 0 || round.TokensOut > 0 {
		b.WriteString(fmt.Sprintf("  %s\n", style.DimStyle.Render(fmt.Sprintf("[Tokens: %d in / %d out]", round.TokensIn, round.TokensOut))))
	}

	if icon != "" {
		return icon + strings.TrimLeft(b.String(), " ")
	}
	return b.String()
}

func (p *ConversationPanel) renderPendingThink(content string) string {
	var b strings.Builder
	icon := style.CyanStyle.Render("● ")
	if p.BlinkOn {
		icon = style.WhiteStyle.Render("● ")
	}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		b.WriteString("  ")
		b.WriteString(style.DarkStyle.Render(line))
		b.WriteByte('\n')
	}
	if icon != "" {
		return icon + strings.TrimLeft(b.String(), " ")
	}
	return b.String()
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

	b.WriteString(icon)
	b.WriteString(style.WhiteStyle.Render(step.ToolName))
	if step.EstimatedTok > 0 {
		b.WriteString(fmt.Sprintf(" | %s", style.GrayStyle.Render(fmt.Sprintf("预计消耗 %d Tokens", step.EstimatedTok))))
	}
	if step.ProgressText != "" {
		b.WriteString(fmt.Sprintf(" | %s", style.GrayStyle.Render(step.ProgressText)))
	}
	b.WriteByte('\n')

	if step.ResultText != "" {
		if step.Collapsed {
			lines := strings.Split(step.ResultText, "\n")
			if len(lines) > 3 {
				b.WriteString(fmt.Sprintf("  %s\n", style.GrayStyle.Render(fmt.Sprintf("完成 (%d lines)", len(lines)))))
			} else {
				for _, line := range lines {
					b.WriteString(fmt.Sprintf("  %s\n", style.GrayStyle.Render(line)))
				}
			}
		} else {
			lines := strings.Split(step.ResultText, "\n")
			for i, line := range lines {
				if i >= 10 {
					b.WriteString(fmt.Sprintf("  %s\n", style.GrayStyle.Render(fmt.Sprintf("… +%d lines (ctrl+o to expand)", len(lines)-i))))
					break
				}
				b.WriteString(fmt.Sprintf("  %s\n", style.DimStyle.Render(line)))
			}
		}
	}
	return b.String()
}
