package brain

import (
	"mindx/internal/core"
	"mindx/pkg/logging"
	"fmt"
	"strings"
)

// LongInputSuite 场景 4: 超长文输入测试
// 测试 Brain 处理超长输入的能力
type LongInputSuite struct {
	BrainIntegrationSuite
}

// TestLongInput_SingleMessage 测试：单条超长消息
// 验证：模型能处理超长输入并返回合理的响应
func (s *LongInputSuite) TestLongInput_SingleMessage() {
	s.brain = s.createTestBrain()

	// 生成超长文本（约 10000 字符）
	longText := generateLongText(10000)

	q := "请总结以下内容：\n" + longText
	s.logger.Info("提问", logging.Int("input_length", len(q)))

	resp, err := s.brain.Post(&core.ThinkingRequest{
		Question: q,
		Timeout: 60, // 增加超时时间
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "回答不能为空")

	// 验证：回答应该是一个总结，而不是错误
	answer := resp.Answer
	isValidResponse := len(answer) > 10 && // 至少有些内容
		!strings.Contains(answer, "错误") &&
		!strings.Contains(answer, "失败") &&
		!strings.Contains(answer, "Error") &&
		!strings.Contains(answer, "Failed")

	s.True(isValidResponse,
		"超长输入应该返回有效的总结，实际回答长度: %d", len(answer))

	s.logger.Info("响应",
		logging.String("answer", resp.Answer[:min(100, len(resp.Answer))] + "..."),
		logging.Int("answer_length", len(resp.Answer)))
}

// TestLongInput_MultipleMessages 测试：多条长消息
// 验证：模型能处理连续的长消息
func (s *LongInputSuite) TestLongInput_MultipleMessages() {
	s.brain = s.createTestBrain()

	// 连续发送3条长消息
	longTexts := []string{
		generateLongText(5000),
		generateLongText(5000),
		generateLongText(5000),
	}

	for i, text := range longTexts {
		q := "这是第" + string(rune('一'+i)) + "部分内容，请记住：\n" + text
		s.logger.Info("第%d次提问", logging.Int("index", i+1),
			logging.Int("input_length", len(q)))

		resp, err := s.brain.Post(&core.ThinkingRequest{
			Question: q,
			Timeout: 45,
		})
		s.Require().NoError(err)
		s.NotEmpty(resp.Answer, "回答不能为空")

		// 验证：每条消息都应该有有效响应
		isValidResponse := len(resp.Answer) > 5 &&
			!strings.Contains(resp.Answer, "错误") &&
			!strings.Contains(resp.Answer, "失败")

		s.True(isValidResponse,
			"第%d条消息应该返回有效响应，实际回答长度: %d", i+1, len(resp.Answer))
	}
}

// TestLongInput_CodeBlock 测试：超长代码块
// 验证：模型能处理超长代码块
func (s *LongInputSuite) TestLongInput_CodeBlock() {
	s.brain = s.createTestBrain()

	// 生成超长代码块（约 5000 行）
	longCode := generateLongCode(5000)

	q := "请解释以下代码的作用：\n```go\n" + longCode + "\n```"
	s.logger.Info("提问", logging.Int("input_length", len(q)),
		logging.Int("code_lines", 5000))

	resp, err := s.brain.Post(&core.ThinkingRequest{
		Question: q,
		Timeout: 60,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "回答不能为空")

	// 验证：回答应该是对代码的解释
	answer := resp.Answer
	hasExplanation := strings.Contains(answer, "代码") ||
		strings.Contains(answer, "函数") ||
		strings.Contains(answer, "作用") ||
		strings.Contains(answer, "功能") ||
		len(answer) > 50 // 至少有一些解释

	s.True(hasExplanation,
		"超长代码块应该得到解释，实际回答长度: %d", len(answer))

	s.logger.Info("响应", logging.String("answer", resp.Answer[:min(150, len(resp.Answer))] + "..."))
}

// TestLongInput_ContextOverflow 测试：上下文溢出
// 验证：当输入超过模型上下文限制时，模型仍能处理
func (s *LongInputSuite) TestLongInput_ContextOverflow() {
	s.brain = s.createTestBrain()

	// 构造超过模型上下文限制的输入
	// 假设模型上下文约 8k tokens，每字符约 0.5 token
	// 20000 字符约 10000 tokens，应该会超过上下文
	extremeLongText := generateLongText(20000)

	q := "请处理以下超长文本：\n" + extremeLongText
	s.logger.Info("提问", logging.Int("input_length", len(q)),
		logging.String("estimated_tokens", "~40000"))

	resp, err := s.brain.Post(&core.ThinkingRequest{
		Question: q,
		Timeout: 90,
	})

	// 验证：即使超长，也不应该崩溃
	// 可能会返回错误或截断的响应，但不应该 panic
	if err != nil {
		// 错误也是可接受的（如上下文超限）
		s.logger.Warn("超长输入导致错误（这是可接受的）",
			logging.Err(err))
		return
	}

	if resp.Answer == "" {
		// 空响应在极端情况下是可接受的
		s.logger.Info("超长输入返回空响应（这是可接受的）")
		return
	}

	// 如果有响应，验证至少不是完全错误信息
	isError := strings.Contains(resp.Answer, "上下文") &&
		strings.Contains(resp.Answer, "超限") &&
		strings.Contains(resp.Answer, "超出")

	s.False(isError,
		"超长输入应该至少返回部分响应，而不是纯错误消息")

	s.logger.Info("响应", logging.String("answer", resp.Answer[:min(100, len(resp.Answer))] + "..."))
}

// generateLongText 生成长文本用于测试
func generateLongText(charCount int) string {
	var sb strings.Builder
	sentences := []string{
		"这是一个测试句子，用于生成长文本。",
		"人工智能技术的发展日新月异，改变了我们的生活方式。",
		"自然语言处理是人工智能的重要分支之一。",
		"深度学习模型在多个领域都取得了显著进展。",
		"机器学习算法需要大量的数据进行训练。",
		"云计算为大规模模型训练提供了基础设施支持。",
		"数据隐私和安全是人工智能应用中的关键问题。",
		"人机交互技术的发展让AI更易用。",
		"自动化流程可以提高工作效率和准确性。",
		"虚拟助手正在成为日常生活的一部分。",
	}

	for len(sb.String()) < charCount {
		for _, sentence := range sentences {
			if len(sb.String())+len(sentence) > charCount {
				break
			}
			sb.WriteString(sentence)
			sb.WriteString(" ")
		}
	}

	return sb.String()[:charCount]
}

// generateLongCode 生成长代码用于测试
func generateLongCode(lines int) string {
	var sb strings.Builder
	for i := 0; i < lines; i++ {
		sb.WriteString(fmt.Sprintf(`
// Function %d
func Func%d() {
	// 这是一个测试函数
	result := 0
	for j := 0; j < %d; j++ {
		result += j
	}
	return result
}

// Struct %d
type Struct%d struct {
	Field%d  int
	Field%d  string
	Field%d  bool
}

// Interface %d
type Interface%d interface {
	Method%d() error
	Method%d() string
}
`, i, i, 10, i, i, i, i, i, i, i, i, i))
	}

	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
