package conv

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
)

type Reasoning struct {
	Label    string
	Result   string
	IsActive bool
	Spinner  spinner.Model
}

func NewReasoning() Reasoning {
	s := spinner.New()
	s.Style = style.DimStyle
	s.Spinner = spinner.MiniDot
	return Reasoning{
		Label:    "深度思考",
		IsActive: true,
		Spinner:  s,
	}
}

func (r Reasoning) WithLabel(label string) Reasoning {
	r.Label = label
	return r
}

func UpdateReasoning(m Reasoning, e tea.Msg) (Reasoning, tea.Cmd) {
	var cmd tea.Cmd
	switch e := e.(type) {
	case msg.ThinkingDoneMsg:
		if e.Reasoning != "" {
			m.Result = e.Reasoning
		}
		m.IsActive = false
		return m, nil
	default:
		if !m.IsActive {
			return m, nil
		}
		m.Spinner, cmd = m.Spinner.Update(e)
	}

	return m, cmd
}

func ViewReasoning(m Reasoning) string {
	if m.Result == "" && !m.IsActive {
		return ""
	}

	if m.IsActive {
		return style.GrayStyle.Render(m.Spinner.View() + " " + m.Label)
	}

	if m.Result == "LLM returned native tool calls" {
		return ""
	}

	return style.GrayStyle.Render("● " + m.Result)
}
