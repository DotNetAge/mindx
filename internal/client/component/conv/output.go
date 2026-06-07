package conv

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/internal/client/style"
)

type OutputEntry struct {
	Role      string
	Content   string
	Streaming bool
}

type Output struct {
	Entries []OutputEntry
}

func UpdateOutput(m Output, e tea.Msg) (Output, tea.Cmd) {
	switch e := e.(type) {
	case msg.ContentDeltaMsg:
		if len(m.Entries) == 0 {
			m.Entries = append(m.Entries, OutputEntry{Role: "assistant", Content: e.Content, Streaming: true})
		} else {
			last := &m.Entries[len(m.Entries)-1]
			if last.Role == "assistant" && last.Streaming {
				last.Content += e.Content
			} else if last.Role == "assistant" && !last.Streaming {
				m.Entries = append(m.Entries, OutputEntry{Role: "assistant", Content: e.Content, Streaming: true})
			} else {
				m.Entries = append(m.Entries, OutputEntry{Role: "assistant", Content: e.Content, Streaming: true})
			}
		}
		return m, nil

	case msg.FinalAnswerMsg:
		if len(m.Entries) > 0 {
			last := m.Entries[len(m.Entries)-1]
			if last.Role == "assistant" && last.Streaming {
				last.Content = e.Content
				last.Streaming = false
				return m, nil
			}
			if last.Role == "assistant" && last.Content == e.Content {
				return m, nil
			}
		}
		m.Entries = append(m.Entries, OutputEntry{Role: "assistant", Content: e.Content})
		return m, nil

	case msg.AgentErrorMsg:
		errMsg := e.Error.Error()
		if len(m.Entries) > 0 {
			last := m.Entries[len(m.Entries)-1]
			if last.Role == "error" && last.Content == errMsg {
				return m, nil
			}
		}
		m.Entries = append(m.Entries, OutputEntry{Role: "error", Content: errMsg})
		return m, nil

	case msg.LLMTimeoutMsg:
		timeoutMsg := fmt.Sprintf("⚠️ LLM 响应超时 (限制: %v, 已用: %v)", e.Timeout.Round(time.Second), e.Elapsed.Round(time.Second))
		if e.Error != "" {
			timeoutMsg += "\n原因: " + e.Error
		}
		m.Entries = append(m.Entries, OutputEntry{Role: "timeout", Content: timeoutMsg})
		return m, nil
	}

	return m, nil
}

func ViewOutput(m Output, width int) string {
	if len(m.Entries) == 0 {
		return ""
	}

	var b strings.Builder
	for i, entry := range m.Entries {
		if strings.TrimSpace(entry.Content) == "" {
			continue
		}
		if i > 0 || b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(viewOutputEntry(entry, width))
	}
	return b.String()
}

func viewOutputEntry(entry OutputEntry, width int) string {
	sep := style.Divider(strings.Repeat("─", width))

	if entry.Role == "error" {
		return sep + "\n" + style.RedStyle.Render(entry.Content) + "\n" + sep
	}

	if entry.Role == "timeout" {
		return sep + "\n" + style.YellowStyle.Render(entry.Content) + "\n" + sep
	}

	content := render.MarkdownWithWidth(entry.Content, width-4)
	if entry.Streaming {
		content += style.DimStyle.Render("▌")
	}
	return sep + "\n" + content + sep
}
