package client

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// SearchModel 提供转录模式下的内联搜索功能。
type SearchModel struct {
	active      bool
	query       string
	current     int // 当前匹配位置（1-based）
	total       int // 总匹配数
	input       textinput.Model
	onSearch    func(query string) int          // 执行搜索，返回匹配数
	onNavigate  func(direction string)          // "next" 或 "prev"
	getPosition func() (current, total int)     // 获取当前匹配位置
}

// NewSearchModel 创建新的搜索模型。
func NewSearchModel() *SearchModel {
	ti := textinput.New()
	ti.Placeholder = "搜索... (Enter/N 下一个, Shift+Enter/P 上一个, Esc 退出)"
	ti.CharLimit = 100
	ti.Focus()

	return &SearchModel{
		input: ti,
	}
}

// Activate 激活搜索模式。
func (s *SearchModel) Activate() tea.Cmd {
	s.active = true
	cmd := s.input.Focus()
	s.input.SetValue(s.query)
	return cmd
}

// Deactivate 关闭搜索模式。
func (s *SearchModel) Deactivate() {
	s.active = false
	s.input.Blur()
}

// IsActive 返回搜索是否激活。
func (s *SearchModel) IsActive() bool {
	return s.active
}

// SetCallbacks 设置搜索回调函数。
func (s *SearchModel) SetCallbacks(
	onSearch func(query string) int,
	onNavigate func(direction string),
	getPosition func() (current, total int),
) {
	s.onSearch = onSearch
	s.onNavigate = onNavigate
	s.getPosition = getPosition
}

// SetWidth 设置搜索框宽度。
func (s *SearchModel) SetWidth(w int) {
	s.input.SetWidth(w - 10) // 预留边框和匹配计数的空间
}

// Update 处理键盘消息。
func (s *SearchModel) Update(msg tea.Msg) (*SearchModel, tea.Cmd) {
	if !s.active {
		return s, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "ctrl+f":
			s.Deactivate()
			return s, nil
		case "enter", "n":
			if s.onNavigate != nil {
				s.onNavigate("next")
			}
			if s.getPosition != nil {
				s.current, s.total = s.getPosition()
			}
			return s, nil
		case "shift+enter", "p":
			if s.onNavigate != nil {
				s.onNavigate("prev")
			}
			if s.getPosition != nil {
				s.current, s.total = s.getPosition()
			}
			return s, nil
		}
	}

	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)

	// 文本变化时触发搜索
	newQuery := s.input.Value()
	if newQuery != s.query {
		s.query = newQuery
		if s.onSearch != nil {
			s.total = s.onSearch(s.query)
			s.current = 0
			if s.total > 0 {
				s.current = 1
			}
		}
	}

	return s, cmd
}

// View 渲染搜索栏。
func (s *SearchModel) View() string {
	if !s.active {
		return ""
	}

	var b strings.Builder

	// 搜索输入框（由 lipgloss 样式加边框）
	b.WriteString(SearchInputStyle.Render(s.input.View()))

	// 匹配计数
	if s.query != "" {
		posStr := fmt.Sprintf(" %d/%d ", s.current, s.total)
		b.WriteString(" ")
		b.WriteString(SearchMatchStyle.Render(posStr))
	}

	return b.String()
}
