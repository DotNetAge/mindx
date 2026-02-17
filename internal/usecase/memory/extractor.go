package memory

import (
	"mindx/internal/core"
	"mindx/internal/entity"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// LLMExtractor 基于LLM的记忆提取器
type LLMExtractor struct {
	brain  core.Thinking // 用于调用LLM
	memory core.Memory   // 用于存储记忆点
}

// MemoryPointResponse LLM返回的JSON结构
type MemoryPointResponse struct {
	Memories []MemoryPoint `json:"memories"`
}

// MemoryPoint 记忆点
type MemoryPoint struct {
	Topic    string   `json:"topic"`    // 话题名称
	Keywords []string `json:"keywords"` // 关键词（3-5个）
	Summary  string   `json:"summary"`  // 摘要（50-100字）
	Content  string   `json:"content"`  // 主要内容（200-300字）
}

// NewLLMExtractor 创建基于LLM的记忆提取器
func NewLLMExtractor(brain core.Thinking, memory core.Memory) *LLMExtractor {
	return &LLMExtractor{
		brain:  brain,
		memory: memory,
	}
}

// Extract 从会话中提取记忆点并存储
func (e *LLMExtractor) Extract(session entity.Session) bool {
	if len(session.Messages) == 0 {
		return true
	}

	// 1. 格式化会话内容
	conversation := e.formatConversation(session.Messages)

	// 2. 构造LLM Prompt
	prompt := e.buildPrompt(conversation)

	// 3. 调用LLM进行记忆提取
	thinkResult, err := e.brain.Think(prompt, nil, "", false)
	if err != nil || thinkResult == nil {
		return false
	}

	// 4. 解析JSON响应
	var result MemoryPointResponse
	if err := json.Unmarshal([]byte(thinkResult.Answer), &result); err != nil {
		// 如果解析JSON失败，尝试直接创建一个记忆点
		return e.createFallbackMemory(session)
	}

	// 5. 存储记忆点到Memory
	for _, mem := range result.Memories {
		memoryPoint := core.MemoryPoint{
			Keywords:  mem.Keywords,
			Content:   mem.Content,
			Summary:   mem.Summary,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := e.memory.Record(memoryPoint); err != nil {
			// 记录失败但继续处理其他记忆点
			fmt.Printf("存储记忆点失败: %v\n", err)
		}
	}

	return len(result.Memories) > 0
}

// formatConversation 格式化会话内容
func (e *LLMExtractor) formatConversation(msgs []entity.Message) string {
	var sb strings.Builder
	for _, msg := range msgs {
		sb.WriteString(fmt.Sprintf("[%s] %s\n", msg.Role, msg.Content))
	}
	return sb.String()
}

// buildPrompt 构造记忆提取Prompt
func (e *LLMExtractor) buildPrompt(conversation string) string {
	return `你是一个智能记忆提取助手。请分析以下对话内容，按照话题分类并生成记忆点。

任务：
1. 将对话按话题分类（相同话题的对话合并）
2. 总结每个话题的关键词（3-5个）
3. 总结每个话题的摘要（50-100字）
4. 总结每个话题的主要内容（200-300字）
5. 格式化输出JSON

输出格式：
{
  "memories": [
    {
      "topic": "话题名称",
      "keywords": ["关键词1", "关键词2", "关键词3"],
      "summary": "摘要",
      "content": "主要内容"
    }
  ]
}

注意事项：
- 如果对话内容较短或话题单一，可以只生成一个记忆点
- 关键词应该是最能代表话题的核心词汇
- 摘要应该简洁明了，概括话题的核心内容
- 主要内容应该包含对话中的关键信息和上下文
- 避免生成过于冗长的内容

对话内容：
` + conversation
}

// createFallbackMemory 创建备用记忆点（当LLM返回非JSON时）
func (e *LLMExtractor) createFallbackMemory(session entity.Session) bool {
	// 简单地将整个会话摘要作为记忆点
	var content strings.Builder
	for i, msg := range session.Messages {
		if i > 0 {
			content.WriteString("\n")
		}
		content.WriteString(fmt.Sprintf("%s: %s", msg.Role, msg.Content))
	}

	memoryPoint := core.MemoryPoint{
		Keywords:  []string{"对话"},
		Content:   content.String(),
		Summary:   fmt.Sprintf("包含%d条消息的对话", len(session.Messages)),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return e.memory.Record(memoryPoint) == nil
}
