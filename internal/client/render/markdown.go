package render

import (
	"sync"

	"charm.land/glamour/v2"
)

type cachedRenderer struct {
	mu   sync.RWMutex
	def  *glamour.TermRenderer
	pool map[int]*glamour.TermRenderer
}

var cache = &cachedRenderer{
	pool: make(map[int]*glamour.TermRenderer),
}

func init() {
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
	)
	if err != nil {
		return
	}
	cache.def = r
}

func getRenderer(width int) *glamour.TermRenderer {
	if width <= 0 {
		return cache.def
	}

	cache.mu.RLock()
	r, ok := cache.pool[width]
	cache.mu.RUnlock()
	if ok {
		return r
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	// Double-check after acquiring write lock
	if r, ok := cache.pool[width]; ok {
		return r
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return cache.def
	}
	cache.pool[width] = r
	return r
}

func Markdown(src string) string {
	r := getRenderer(0)
	if r == nil {
		return src
	}
	out, err := r.Render(src)
	if err != nil {
		return src
	}
	return out
}

func MarkdownWithWidth(src string, width int) string {
	if width < 40 {
		width = 40
	}
	r := getRenderer(width)
	if r == nil {
		return src
	}
	out, err := r.Render(src)
	if err != nil {
		return src
	}
	return out
}
