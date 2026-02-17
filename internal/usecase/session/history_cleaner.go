package session

import (
	"mindx/internal/entity"
	"crypto/md5"
	"encoding/hex"
	"strings"
	"unicode"
)

// HistoryCleaner 历史对话清理器
type HistoryCleaner struct {
	similarityThreshold float64 // 相似度阈值（0-1）
}

// NewHistoryCleaner 创建历史对话清理器
func NewHistoryCleaner() *HistoryCleaner {
	return &HistoryCleaner{
		similarityThreshold: 0.8, // 默认相似度阈值80%
	}
}

// Clean 清理历史对话，去除重复和相似的消息
func (hc *HistoryCleaner) Clean(messages []entity.Message) []entity.Message {
	if len(messages) == 0 {
		return messages
	}

	// 1. 完全相同去重（基于内容哈希）
	uniqueMessages := hc.removeExactDuplicates(messages)

	// 2. 相似消息去重
	cleanedMessages := hc.removeSimilarMessages(uniqueMessages)

	return cleanedMessages
}

// removeExactDuplicates 去除完全相同的消息
func (hc *HistoryCleaner) removeExactDuplicates(messages []entity.Message) []entity.Message {
	seen := make(map[string]bool)
	result := make([]entity.Message, 0, len(messages))

	for _, msg := range messages {
		hash := hc.hashMessage(msg)
		if !seen[hash] {
			seen[hash] = true
			result = append(result, msg)
		}
	}

	return result
}

// removeSimilarMessages 去除相似的消息（保留最新的）
func (hc *HistoryCleaner) removeSimilarMessages(messages []entity.Message) []entity.Message {
	if len(messages) <= 1 {
		return messages
	}

	result := make([]entity.Message, 0, len(messages))
	keep := make([]bool, len(messages))

	for i := 0; i < len(messages); i++ {
		if keep[i] {
			continue
		}

		result = append(result, messages[i])
		keep[i] = true

		// 检查后面的消息是否与当前消息相似
		for j := i + 1; j < len(messages); j++ {
			if keep[j] {
				continue
			}

			// 只比较相同角色的消息
			if messages[i].Role == messages[j].Role {
				similarity := hc.calculateSimilarity(messages[i].Content, messages[j].Content)
				if similarity >= hc.similarityThreshold {
					// 标记为重复，不保留
					keep[j] = true
				}
			}
		}
	}

	return result
}

// hashMessage 计算消息的哈希值
func (hc *HistoryCleaner) hashMessage(msg entity.Message) string {
	content := strings.TrimSpace(msg.Content)
	content = strings.ToLower(content)
	content = normalizeWhitespace(content)

	hash := md5.Sum([]byte(msg.Role + ":" + content))
	return hex.EncodeToString(hash[:])
}

// calculateSimilarity 计算两个字符串的相似度（基于编辑距离）
func (hc *HistoryCleaner) calculateSimilarity(s1, s2 string) float64 {
	// 预处理：转小写、去除多余空格
	s1 = normalizeWhitespace(strings.ToLower(s1))
	s2 = normalizeWhitespace(strings.ToLower(s2))

	// 如果完全相同
	if s1 == s2 {
		return 1.0
	}

	// 如果其中一个为空
	if s1 == "" || s2 == "" {
		return 0.0
	}

	// 计算编辑距离
	distance := levenshteinDistance(s1, s2)
	maxLen := max(len(s1), len(s2))

	// 相似度 = 1 - (编辑距离 / 最大长度)
	similarity := 1.0 - float64(distance)/float64(maxLen)

	return similarity
}

// normalizeWhitespace 标准化空白字符
func normalizeWhitespace(s string) string {
	// 去除首尾空格
	s = strings.TrimSpace(s)
	// 将连续的空白字符替换为单个空格
	var builder strings.Builder
	var prevSpace bool
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				builder.WriteRune(' ')
				prevSpace = true
			}
		} else {
			builder.WriteRune(r)
			prevSpace = false
		}
	}
	return builder.String()
}

// levenshteinDistance 计算编辑距离
func levenshteinDistance(s1, s2 string) int {
	m, n := len(s1), len(s2)

	// 优化：如果其中一个字符串为空
	if m == 0 {
		return n
	}
	if n == 0 {
		return m
	}

	// 使用滚动数组优化空间复杂度
	prev := make([]int, n+1)
	curr := make([]int, n+1)

	// 初始化第一行
	for j := 0; j <= n; j++ {
		prev[j] = j
	}

	for i := 1; i <= m; i++ {
		curr[0] = i
		for j := 1; j <= n; j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			curr[j] = minInt(
				prev[j]+1,      // 删除
				curr[j-1]+1,    // 插入
				prev[j-1]+cost, // 替换
			)
		}
		prev, curr = curr, prev
	}

	return prev[n]
}

func minInt(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
