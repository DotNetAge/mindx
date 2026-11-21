package scheduler

import "strings"

// TitleMaxRunes 任务名长度上限（侧栏会话标题同口径，超出省略）。
const TitleMaxRunes = 20

// DeriveTitle 从提示词里取任务名：首个非空行，压缩空白后截断。
// 仅在没有显式 title 时兜底，避免会话出现「未命名任务」。
func DeriveTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.Join(strings.Fields(line), " "))
		if line == "" {
			continue
		}
		r := []rune(line)
		if len(r) > TitleMaxRunes {
			return string(r[:TitleMaxRunes])
		}
		return line
	}
	return ""
}
