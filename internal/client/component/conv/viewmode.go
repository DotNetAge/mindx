package conv

import (
	"strings"

	"github.com/DotNetAge/mindx/internal/client/data"
	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/internal/client/style"
)

func (p *ConversationPanel) renderResultEntry(res data.ResultEntry) string {
	if res.Role == "error" {
		return style.RedStyle.Render("⏺ " + res.Content)
	}
	var b strings.Builder
	b.WriteString(style.WhiteStyle.Render("⏺ "))
	b.WriteString(render.MarkdownWithWidth(res.Content, p.width-4))
	b.WriteByte('\n')
	return b.String()
}
