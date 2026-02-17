package brain

import (
	"mindx/internal/core"
	"mindx/pkg/logging"
	"strings"
	"time"
)

// MemoryReferenceSuite 场景 2: 记忆参考测试
// 测试 Brain 是否能正确使用记忆回答用户的问题
type MemoryReferenceSuite struct {
	BrainIntegrationSuite
}

// TestMemory_ProgrammingPreference 测试：编程偏好记忆
// 预置记忆：用户喜欢用 Go 编程
// 验证：问"我喜欢用什么语言编程"时能回答正确
func (s *MemoryReferenceSuite) TestMemory_ProgrammingPreference() {
	s.brain = s.createTestBrain()

	// 验证记忆已经记录（在 SetupSuite 中）
	memories, err := s.memory.Search("编程")
	s.Require().NoError(err)
	s.GreaterOrEqual(len(memories), 1, "应该有编程相关的记忆")

	// 提问关于编程偏好
	q := "我喜欢用什么语言编程？"
	s.logger.Info("提问", logging.String("question", q))

	resp, err := s.postWithHistory(&core.ThinkingRequest{
		Question: q,
		Timeout:  30,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "回答不能为空")

	// 验证：回答应该包含"Go"或"go"
	answer := resp.Answer
	hasGo := strings.Contains(answer, "Go") || strings.Contains(answer, "go")
	s.True(hasGo,
		"回答应该参考记忆并提到 Go 语言，实际回答: %s", resp.Answer)

	s.logger.Info("响应", logging.String("answer", resp.Answer),
		logging.Bool("contains_go", hasGo))
}

// TestMemory_LocationMemory 测试：地点记忆
// 预置记忆：用户住在上海浦东
// 验证：问"我住在哪里"时能回答正确
func (s *MemoryReferenceSuite) TestMemory_LocationMemory() {
	s.brain = s.createTestBrain()

	// 提问关于居住地点
	q := "我住在哪里？"
	s.logger.Info("提问", logging.String("question", q))

	resp, err := s.postWithHistory(&core.ThinkingRequest{
		Question: q,
		Timeout:  30,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "回答不能为空")

	// 验证：回答应该包含"上海"
	answer := resp.Answer
	hasLocation := strings.Contains(answer, "上海")
	s.True(hasLocation,
		"回答应该参考记忆并提到上海，实际回答: %s", resp.Answer)

	s.logger.Info("响应", logging.String("answer", resp.Answer),
		logging.Bool("contains_location", hasLocation))
}

// TestMemory_CombinedMemory 测试：组合记忆
// 预置记忆：用户住在上海，喜欢编程
// 验证：问"作为一个住在上海的程序员，我可以做什么"时能结合两个记忆
func (s *MemoryReferenceSuite) TestMemory_CombinedMemory() {
	s.brain = s.createTestBrain()

	// 先添加地点记忆
	locationMem := core.MemoryPoint{
		Keywords:  []string{"上海", "居住", "城市"},
		Content:   "用户住在上海市浦东新区",
		Summary:   "用户住在上海",
		CreatedAt: time.Now(),
	}
	err := s.memory.Record(locationMem)
	s.Require().NoError(err)

	// 提问需要组合记忆的问题
	q := "作为一个住在上海的程序员，我可以做什么？"
	s.logger.Info("提问", logging.String("question", q))

	resp, err := s.postWithHistory(&core.ThinkingRequest{
		Question: q,
		Timeout:  30,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "回答不能为空")

	// 验证：回答应该同时提到"编程"和"上海"
	answer := resp.Answer
	hasProgramming := strings.Contains(answer, "编程") || strings.Contains(answer, "Go")
	hasLocation := strings.Contains(answer, "上海")

	s.True(hasProgramming && hasLocation,
		"回答应该组合两个记忆：提到编程和上海，实际回答: %s", resp.Answer)

	s.logger.Info("响应", logging.String("answer", resp.Answer),
		logging.Bool("contains_programming", hasProgramming),
		logging.Bool("contains_location", hasLocation))
}

// TestMemory_UnknownQuestion 测试：未知问题
// 验证：当问题没有相关记忆时，模型能合理回答
func (s *MemoryReferenceSuite) TestMemory_UnknownQuestion() {
	s.brain = s.createTestBrain()

	// 提问预置记忆中没有的问题
	q := "今天天气怎么样？"
	s.logger.Info("提问", logging.String("question", q))

	resp, err := s.brain.Post(&core.ThinkingRequest{
		Question: q,
		Timeout:  30,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "回答不能为空")

	// 验证：回答不应该说"我不知道用户喜欢什么"之类的话
	answer := resp.Answer
	notReferring := !strings.Contains(answer, "我不知道") &&
		!strings.Contains(answer, "没有记录") &&
		!strings.Contains(answer, "不记得")

	s.True(notReferring,
		"未知问题应该合理回答，不应该说没有记录，实际回答: %s", resp.Answer)

	s.logger.Info("响应", logging.String("answer", resp.Answer))
}
