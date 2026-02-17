package session

import (
	"mindx/internal/entity"
	"strings"
	"unicode"
)

// MessageSplitter 消息拆分器
type MessageSplitter struct {
	maxTokensPerMessage int // 每条消息的最大Token数
}

// NewMessageSplitter 创建消息拆分器
func NewMessageSplitter(maxTokensPerMessage int) *MessageSplitter {
	return &MessageSplitter{
		maxTokensPerMessage: maxTokensPerMessage,
	}
}

// Split 拆分超长消息
func (ms *MessageSplitter) Split(msg entity.Message) []entity.Message {
	// 计算消息的Token数
	tokens := calculateTokens(msg.Content)

	// 如果不超过限制，直接返回原消息
	if tokens <= ms.maxTokensPerMessage {
		return []entity.Message{msg}
	}

	// 超过限制，进行拆分
	return ms.splitByTokens(msg)
}

// splitByTokens 基于Token数拆分消息
func (ms *MessageSplitter) splitByTokens(msg entity.Message) []entity.Message {
	content := msg.Content
	var parts []string

	// 优先在段落边界拆分（双换行符）
	paragraphs := strings.Split(content, "\n\n")
	var currentPart strings.Builder
	currentTokens := 0

	for _, paragraph := range paragraphs {
		paragraphTokens := calculateTokens(paragraph)

		// 如果当前段落加上当前部分会超过限制
		if currentTokens+paragraphTokens > ms.maxTokensPerMessage && currentPart.Len() > 0 {
			parts = append(parts, strings.TrimSpace(currentPart.String()))
			currentPart.Reset()
			currentTokens = 0
		}

		// 如果单个段落就超过限制，需要在句子级别拆分
		if paragraphTokens > ms.maxTokensPerMessage {
			if currentPart.Len() > 0 {
				parts = append(parts, strings.TrimSpace(currentPart.String()))
				currentPart.Reset()
				currentTokens = 0
			}
			sentences := ms.splitBySentences(paragraph)
			parts = append(parts, sentences...)
		} else {
			// 添加到当前部分
			if currentPart.Len() > 0 {
				currentPart.WriteString("\n\n")
			}
			currentPart.WriteString(paragraph)
			currentTokens += paragraphTokens
		}
	}

	// 添加最后一部分
	if currentPart.Len() > 0 {
		parts = append(parts, strings.TrimSpace(currentPart.String()))
	}

	// 构建拆分后的消息列表
	messages := make([]entity.Message, len(parts))
	for i, part := range parts {
		messages[i] = entity.Message{
			Role:    msg.Role,
			Content: part,
		}
	}

	return messages
}

// splitBySentences 在句子级别拆分
func (ms *MessageSplitter) splitBySentences(content string) []string {
	var sentences []string
	var currentSentence strings.Builder
	currentTokens := 0

	runes := []rune(content)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		currentSentence.WriteRune(r)

		// 检查是否是句子结束符
		if r == '。' || r == '！' || r == '？' || r == '.' || r == '!' || r == '?' {
			// 检查下一个字符是否是引号或括号
			if i+1 < len(runes) && (runes[i+1] == '"' || runes[i+1] == '\'' || runes[i+1] == ')' || runes[i+1] == '）') {
				currentSentence.WriteRune(runes[i+1])
				i++
			}

			sentence := strings.TrimSpace(currentSentence.String())
			sentenceTokens := calculateTokens(sentence)

			// 如果当前句子加上累积的句子会超过限制
			if currentTokens+sentenceTokens > ms.maxTokensPerMessage && currentTokens > 0 {
				sentences = append(sentences, strings.TrimSpace(currentSentence.String()[:currentSentence.Len()-len(sentence)]))
				currentSentence.Reset()
				currentSentence.WriteString(sentence)
				currentTokens = sentenceTokens
			} else {
				currentTokens += sentenceTokens
			}
		}
	}

	// 添加最后的部分
	if currentSentence.Len() > 0 {
		sentences = append(sentences, strings.TrimSpace(currentSentence.String()))
	}

	// 如果拆分后的句子仍然太长，在空格处强制拆分
	result := make([]string, 0, len(sentences))
	for _, sentence := range sentences {
		if calculateTokens(sentence) > ms.maxTokensPerMessage {
			chunks := ms.forceSplitBySpace(sentence)
			result = append(result, chunks...)
		} else {
			result = append(result, sentence)
		}
	}

	return result
}

// forceSplitBySpace 在空格处强制拆分
func (ms *MessageSplitter) forceSplitBySpace(content string) []string {
	words := strings.Fields(content)
	var chunks []string
	var currentChunk strings.Builder
	currentTokens := 0

	for _, word := range words {
		wordTokens := calculateTokens(word)

		if currentTokens+wordTokens > ms.maxTokensPerMessage && currentChunk.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
			currentChunk.Reset()
			currentTokens = 0
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString(" ")
		}
		currentChunk.WriteString(word)
		currentTokens += wordTokens
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
	}

	return chunks
}

// calculateTokens 计算Token数（简化版，包级别函数）
func calculateTokens(content string) int {
	// 简化估算：1个中文字符约等于1个token，1个英文单词约等于0.75个token
	count := 0
	inWord := false

	for _, r := range content {
		if unicode.Is(unicode.Han, r) {
			// 中文字符
			count++
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			// 英文字母或数字
			if !inWord {
				count++
				inWord = true
			}
		} else if unicode.IsSpace(r) {
			inWord = false
		} else {
			// 其他字符（标点符号等）
			count++
		}
	}

	// 如果计算结果为0，至少返回1
	if count == 0 {
		return 1
	}

	return count
}
