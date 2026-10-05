package cmd

// splitComma 按英文逗号拆分字符串（空段忽略）。
// 原定义于 cmd/graph.go（RAG 产品线剥离时删除），agent/session 等命令仍在使用，
// 迁移至此公共位置。
func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			if i > start {
				result = append(result, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}
