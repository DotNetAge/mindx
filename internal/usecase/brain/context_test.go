package brain

import (
	"mindx/internal/core"
	"mindx/pkg/logging"
	"strings"
)

// ContextConsistencySuite 场景 1: 上下文一致性测试
// 测试 Brain 在多轮对话中能否保持上下文一致性
type ContextConsistencySuite struct {
	BrainIntegrationSuite
}

// TestContext_UserName 测试：用户能否被记住
// 场景：第一轮告诉名字，第二轮询问名字
func (s *ContextConsistencySuite) TestContext_UserName() {
	s.brain = s.createTestBrain()

	// 第一轮：告诉名字
	q1 := "我叫张三"
	s.logger.Info("第一轮对话", logging.String("question", q1))

	resp1, err := s.postWithHistory(&core.ThinkingRequest{
		Question: q1,
		Timeout:  30,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp1.Answer, "回答不能为空")
	s.logger.Info("第一轮响应", logging.String("answer", resp1.Answer))

	// 第二轮：询问名字
	q2 := "我叫什么名字？"
	s.logger.Info("第二轮对话", logging.String("question", q2))

	resp2, err := s.postWithHistory(&core.ThinkingRequest{
		Question: q2,
		Timeout:  30,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp2.Answer, "回答不能为空")

	// 验证：回答应该包含"张三"
	s.True(strings.Contains(resp2.Answer, "张三"),
		"第二轮回答应该记住用户的名字，实际回答: %s", resp2.Answer)

	s.logger.Info("第二轮响应", logging.String("answer", resp2.Answer),
		logging.Bool("contains_name", strings.Contains(resp2.Answer, "张三")))
}

// TestContext_TopicContinuity 测试：话题连续性
// 场景：第一轮介绍主题，第二轮追问相关问题
func (s *ContextConsistencySuite) TestContext_TopicContinuity() {
	s.brain = s.createTestBrain()

	// 第一轮：介绍主题
	q1 := "我想学编程，推荐一门语言"
	s.logger.Info("第一轮对话", logging.String("question", q1))

	resp1, err := s.postWithHistory(&core.ThinkingRequest{
		Question: q1,
		Timeout:  30,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp1.Answer)
	s.logger.Info("第一轮响应", logging.String("answer", resp1.Answer))

	// 第二轮：追问
	q2 := "那它难学吗？"
	s.logger.Info("第二轮对话", logging.String("question", q2))

	resp2, err := s.postWithHistory(&core.ThinkingRequest{
		Question: q2,
		Timeout:  30,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp2.Answer)

	// 验证：回答应该与第一轮相关（避免说"不知道指什么"）
	s.NotContains(resp2.Answer, "不知道",
		"第二轮回答应该保持话题连续，不应该说不知道", resp2.Answer)

	s.logger.Info("第二轮响应", logging.String("answer", resp2.Answer))
}

// TestContext_MultiRoundConversation 测试：多轮对话
// 场景：连续5轮对话，测试上下文保持
func (s *ContextConsistencySuite) TestContext_MultiRoundConversation() {
	s.brain = s.createTestBrain()

	questions := []string{
		"我喜欢吃苹果",
		"那我最喜欢的水果是什么？",
		"香蕉是什么颜色？",
		"那苹果呢？",
		"总结一下我提到的所有水果",
	}

	for i, q := range questions {
		s.logger.Info("第%d轮对话", logging.Int("round", i+1), logging.String("question", q))

		resp, err := s.postWithHistory(&core.ThinkingRequest{
			Question: q,
			Timeout:  30,
		})
		s.Require().NoError(err)
		s.NotEmpty(resp.Answer)

		s.logger.Info("第%d轮响应", logging.Int("round", i+1), logging.String("answer", resp.Answer))

		// 最后一轮：应该总结出"苹果"
		if i == len(questions)-1 {
			s.True(strings.Contains(resp.Answer, "苹果") || strings.Contains(resp.Answer, "苹果"),
				"最后应该总结出苹果，实际回答: %s", resp.Answer)
		}
	}
}
