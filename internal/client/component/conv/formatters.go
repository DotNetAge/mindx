package conv

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DotNetAge/mindx/internal/client/style"
	"github.com/DotNetAge/mindx/internal/i18n"
)

// 工具结果摘要格式化。
//
// 到达 TUI 的 tool_exec_end.result 中：结构化工具（Ls/Bash/Write/Edit/Read/Skill）
// 是 executor 序列化的 JSON 字符串，文本工具（Grep/WebSearch/WebFetch）是纯文本或
// markdown。此处按工具名注册 ResultFormatter，把结果压缩成单行摘要或简短列表，
// 避免在对话流中原样输出 JSON 长串。执行失败的结果不走 formatter（由错误组件呈现）。

func init() {
	RegisterResultFormatter("Ls", formatLSResult)
	RegisterResultFormatter("Grep", formatGrepResult)
	RegisterResultFormatter("Bash", formatBashResult)
	RegisterResultFormatter("Write", formatWriteResult)
	RegisterResultFormatter("Edit", formatEditResult)
	RegisterResultFormatter("Read", formatReadResult)
	RegisterResultFormatter("Skill", formatSkillResult)
	RegisterResultFormatter("WebSearch", formatWebSearchResult)
	RegisterResultFormatter("WebFetch", formatWebFetchResult)
}

// ── 解析辅助 ────────────────────────────────────────────────

// resultJSON 把 JSON 形态的 result 解析为 map；非 JSON 返回 nil。
func resultJSON(result string) map[string]any {
	var m map[string]any
	if strings.HasPrefix(strings.TrimSpace(result), "{") {
		_ = json.Unmarshal([]byte(result), &m)
	}
	return m
}

func jsonBool(m map[string]any, key string) bool {
	b, _ := m[key].(bool)
	return b
}

func jsonInt(m map[string]any, key string) int {
	f, _ := m[key].(float64)
	return int(f)
}

func jsonStr(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// paramLines 统计字符串参数的行数（Write/Edit 的内容来自调用参数）。
func paramLines(params map[string]any, key string) int {
	s, _ := params[key].(string)
	return countLines(s)
}

func countLines(s string) int {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// statusPrefix 结果状态前缀。
func statusPrefix(ok bool) string {
	if ok {
		return i18n.T("conv.tool.success")
	}
	return i18n.T("conv.tool.failed")
}

// parseHeaderInt 从形如 "…行数：123…" 的文本头中提取整数（兼容全/半角冒号）。
func parseHeaderInt(text, key string) int {
	i := strings.Index(text, key)
	if i < 0 {
		return 0
	}
	rest := strings.TrimLeft(text[i+len(key):], "：: ")
	n := 0
	for _, r := range rest {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// ── 各工具 formatter ────────────────────────────────────────

// formatLSResult 目录条目以列表形式展示（目录追加 "/"），超出上限折叠。
func formatLSResult(_ ActionStep, result string, _ int) string {
	m := resultJSON(result)
	if m == nil {
		return ""
	}
	items, _ := m["items"].([]any)
	const maxItems = 50
	var b strings.Builder
	for i, it := range items {
		if i >= maxItems {
			fmt.Fprintf(&b, "… +%d", len(items)-maxItems)
			break
		}
		im, _ := it.(map[string]any)
		name := jsonStr(im, "name")
		if name == "" {
			continue
		}
		if jsonStr(im, "type") == "directory" {
			name += "/"
		}
		fmt.Fprintln(&b, name)
	}
	if b.Len() == 0 {
		if msg := jsonStr(m, "message"); msg != "" {
			return msg
		}
		return ""
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// formatGrepResult 匹配结果本身就是逐行文本，按列表截断展示。
func formatGrepResult(_ ActionStep, result string, _ int) string {
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	const maxLines = 30
	var b strings.Builder
	shown := 0
	for _, line := range lines {
		if shown >= maxLines {
			fmt.Fprintf(&b, "… +%d", len(lines)-maxLines)
			break
		}
		fmt.Fprintln(&b, line)
		shown++
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// formatBashResult 只展示 stdout；非零退出码（含 1）时结果整体标红，
// stdout 为空时回退展示 stderr。
func formatBashResult(_ ActionStep, result string, _ int) string {
	m := resultJSON(result)
	if m == nil {
		return ""
	}
	exitCode := jsonInt(m, "exit_code")
	text := jsonStr(m, "stdout")
	if strings.TrimSpace(text) == "" {
		text = jsonStr(m, "stderr")
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	const maxLines = 20

	lineStyle := style.DimStyle
	if exitCode != 0 {
		lineStyle = style.RedStyle
	}

	var b strings.Builder
	shown := 0
	for _, line := range lines {
		if line == "" && len(lines) > 0 && shown < len(lines)-1 {
			continue // 折叠连续空行
		}
		if shown >= maxLines {
			fmt.Fprintf(&b, "… +%d", len(lines)-shown)
			break
		}
		fmt.Fprintln(&b, lineStyle.Render(line))
		shown++
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// formatWriteResult 「成功 | + <写入行数>」（写入内容取自调用参数 content）。
func formatWriteResult(step ActionStep, result string, _ int) string {
	m := resultJSON(result)
	if m == nil {
		return ""
	}
	written := paramLines(step.Params, "content")
	return fmt.Sprintf("%s | +%d", statusPrefix(jsonBool(m, "success")), written)
}

// formatEditResult 「成功 | + <编辑行数> - <删除行数>」。
func formatEditResult(step ActionStep, result string, _ int) string {
	m := resultJSON(result)
	if m == nil {
		return ""
	}
	added := paramLines(step.Params, "new_string")
	removed := paramLines(step.Params, "old_string")
	prefix := statusPrefix(jsonBool(m, "success"))
	if removed == 0 {
		return fmt.Sprintf("%s | +%d", prefix, added)
	}
	return fmt.Sprintf("%s | +%d -%d", prefix, added, removed)
}

// formatReadResult 「成功 | <读取行数>」。
func formatReadResult(_ ActionStep, result string, _ int) string {
	m := resultJSON(result)
	if m == nil {
		return ""
	}
	lines := jsonInt(m, "lines_read")
	if lines == 0 {
		lines = countLines(jsonStr(m, "content"))
	}
	return fmt.Sprintf("%s | %d", statusPrefix(jsonBool(m, "success")), lines)
}

// formatSkillResult 「成功 | <指令文本行数>」。
func formatSkillResult(_ ActionStep, result string, _ int) string {
	m := resultJSON(result)
	if m == nil {
		return ""
	}
	content := jsonStr(m, "content")
	loaded := jsonBool(m, "loaded") || content != ""
	if !loaded && !jsonBool(m, "success") && content == "" {
		loaded = false
	}
	return fmt.Sprintf("%s | %d", statusPrefix(loaded), countLines(content))
}

// formatWebSearchResult 「共找到 N 条符合条件的结果」。
// 结果条数即 markdown 中 "### N." 标题的数量。
func formatWebSearchResult(_ ActionStep, result string, _ int) string {
	count := strings.Count(result, "### ")
	if count == 0 {
		return ""
	}
	return fmt.Sprintf(i18n.T("conv.tool.websearch.found"), count)
}

// formatWebFetchResult 「成功 | <获取行数>」。
// 大页面落盘形态的头部携带「行数：N」直接采用；否则统计正文行数。
func formatWebFetchResult(_ ActionStep, result string, _ int) string {
	if n := parseHeaderInt(result, "行数："); n > 0 {
		return fmt.Sprintf("%s | %d", statusPrefix(true), n)
	}
	if n := parseHeaderInt(result, "行数:"); n > 0 {
		return fmt.Sprintf("%s | %d", statusPrefix(true), n)
	}

	// 小页面：剥离头两行（--- 网页获取 … / 状态：…）后的正文行数。
	body := result
	if i := strings.Index(body, "\n状态："); i >= 0 {
		body = body[i+1:]
	}
	if j := strings.Index(body, "\n"); j >= 0 {
		body = body[j+1:]
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	return fmt.Sprintf("%s | %d", statusPrefix(true), countLines(body))
}
