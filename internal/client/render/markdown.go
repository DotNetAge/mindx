package render

import (
	"charm.land/glamour/v2"
)

var defaultRenderer *glamour.TermRenderer

func init() {
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
	)
	if err != nil {
		return
	}
	defaultRenderer = r
}

func Markdown(src string) string {
	if defaultRenderer == nil {
		return src
	}
	out, err := defaultRenderer.Render(src)
	if err != nil {
		return src
	}
	return out
}

func MarkdownWithWidth(src string, width int) string {
	if width < 40 {
		width = 40
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return src
	}
	out, err := r.Render(src)
	if err != nil {
		return src
	}
	return out
}
