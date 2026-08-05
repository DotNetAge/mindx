package tools

import (
	"strings"
	"testing"
)

// TestStripANSI 验证剥离 terminal formatter 的 ANSI 颜色码。
// MindStore /api/query 的 terminal 格式返回带 \x1b[1m/\x1b[33m/\x1b[0m 等码，
// 这些码对 LLM 是纯噪音，QuickSearch 返回前必须剥离。
func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "剥离用户实际返回的颜色码",
			input: "\x1b[1m\x1b[33mFound 3 results:\x1b[0m\n" +
				"\x1b[1m\x1b[33m1. bega_post (header)\x1b[0m\n" +
				"\x1b[2m\x1b[36m路径:/users/ray/bega_post.go 位置:[P0-P362]\x1b[0m\n",
			want: "Found 3 results:\n" +
				"1. bega_post (header)\n" +
				"路径:/users/ray/bega_post.go 位置:[P0-P362]\n",
		},
		{
			name:  "无 ANSI 码原样返回",
			input: "纯文本内容",
			want:  "纯文本内容",
		},
		{
			name:  "空串",
			input: "",
			want:  "",
		},
		{
			name:  "多段颜色码全剥离",
			input: "\x1b[31m红\x1b[0m\x1b[32m绿\x1b[0m\x1b[1m粗\x1b[0m",
			want:  "红绿粗",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSI(tt.input)
			if got != tt.want {
				t.Errorf("stripANSI() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestStripANSI_NoANSIRemaining 确保剥离后字符串不含任何 ESC 字符。
func TestStripANSI_NoANSIRemaining(t *testing.T) {
	input := "\x1b[1m\x1b[33m标题\x1b[0m\x1b[2m\x1b[36m路径\x1b[0m"
	got := stripANSI(input)
	if strings.Contains(got, "\x1b") {
		t.Errorf("剥离后仍含 ESC 字符: %q", got)
	}
}
