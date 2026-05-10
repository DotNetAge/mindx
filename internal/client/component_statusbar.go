package client

import (
	"fmt"
	"strings"
)

// StatusBar 单行状态栏。
// 格式: [● Connected] [5.7K Tokens]           [gpt-4o | ↓1.2K ↑4.5K]
type StatusBar struct {
	width        int
	connected    bool
	tokensTotal  int
	tokensIn     int
	tokensOut    int
	currentAgent string
	currentModel string
}

func NewStatusBar() StatusBar {
	return StatusBar{}
}

func (s *StatusBar) SetConnected(v bool)      { s.connected = v }
func (s *StatusBar) SetTokens(in, out int)     { s.tokensIn = in; s.tokensOut = out }
func (s *StatusBar) AddTokens(in, out int)     { s.tokensIn += in; s.tokensOut += out; s.tokensTotal += in + out }
func (s *StatusBar) SetAgent(name, model string) { s.currentAgent = name; s.currentModel = model }
func (s *StatusBar) SetWidth(w int)            { s.width = w }

func (s *StatusBar) View() string {
	if s.width == 0 {
		return ""
	}

	// Left
	var leftParts []string
	if s.connected {
		leftParts = append(leftParts, connectedDot.Render()+" Connected")
	} else {
		leftParts = append(leftParts, disconnectedDot.Render()+" Connecting...")
	}
	if s.tokensTotal > 0 {
		leftParts = append(leftParts, formatTokens(s.tokensTotal)+" Tokens")
	}
	left := strings.Join(leftParts, "  ")

	// Right: agent | model [| tokens]
	right := s.currentAgent
	if s.currentModel != "" {
		right = fmt.Sprintf("%s | %s", s.currentAgent, s.currentModel)
	}
	if s.tokensIn > 0 || s.tokensOut > 0 {
		right = fmt.Sprintf("%s | ↓%s ↑%s", right, formatTokens(s.tokensIn), formatTokens(s.tokensOut))
	}

	// Padding
	leftW := approximateWidth(left)
	rightW := approximateWidth(right)
	pad := s.width - leftW - rightW
	if pad < 4 {
		pad = 4
	}
	return left + strings.Repeat(" ", pad) + right
}
