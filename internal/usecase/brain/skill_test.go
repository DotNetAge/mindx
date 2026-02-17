package brain

import (
	"mindx/internal/core"
	"mindx/pkg/logging"
)

// SkillExecutionSuite 场景 3: 技能执行测试
// 测试 Brain 能否正确识别并执行技能
type SkillExecutionSuite struct {
	BrainIntegrationSuite
}

// TestSkill_WeatherQuery 测试：天气查询技能
// 验证：模型能识别到天气查询意图，并返回工具 Schema
func (s *SkillExecutionSuite) TestSkill_WeatherQuery() {
	s.brain = s.createTestBrain()

	// 验证技能已加载
	skills, err := s.skillMgr.SearchSkills("天气", "查询")
	s.Require().NoError(err)
	s.GreaterOrEqual(len(skills), 1, "应该有天气相关的技能")

	// 提问天气
	q := "查询一下北京的天气"
	s.logger.Info("提问", logging.String("question", q))

	resp, err := s.brain.Post(&core.ThinkingRequest{
		Question: q,
		Timeout:  30,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "回答不能为空")

	// 验证：应该有工具返回
	s.Greater(len(resp.Tools), 0,
		"应该识别到天气查询工具，实际工具数量: %d", len(resp.Tools))

	// 验证：工具名称包含 weather
	foundWeatherSkill := false
	for _, tool := range resp.Tools {
		s.logger.Info("返回的工具",
			logging.String("name", tool.Name),
			logging.Any("params", tool.Params))

		// 检查工具名称
		if tool.Name == "get_weather" || tool.Name == "weather" {
			foundWeatherSkill = true
		}
	}
	s.True(foundWeatherSkill,
		"应该返回天气相关的工具，实际工具: %v", resp.Tools)

	s.logger.Info("响应", logging.String("answer", resp.Answer),
		logging.Int("tools_count", len(resp.Tools)))
}

// TestSkill_TimeQuery 测试：时间查询
// 验证：模型能处理时间查询（即使没有专门的技能）
func (s *SkillExecutionSuite) TestSkill_TimeQuery() {
	s.brain = s.createTestBrain()

	q := "现在几点了？"
	s.logger.Info("提问", logging.String("question", q))

	resp, err := s.brain.Post(&core.ThinkingRequest{
		Question: q,
		Timeout:  30,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "回答不能为空")

	// 验证：时间查询不需要工具，模型可以直接回答
	s.Equal(len(resp.Tools), 0,
		"时间查询不需要工具，实际工具数量: %d", len(resp.Tools))

	s.logger.Info("响应", logging.String("answer", resp.Answer),
		logging.Int("tools_count", len(resp.Tools)))
}

// TestSkill_MultipleSkills 测试：多个技能识别
// 验证：在一次对话中识别多个技能意图
func (s *SkillExecutionSuite) TestSkill_MultipleSkills() {
	s.brain = s.createTestBrain()

	// 提问涉及多个技能的问题
	q := "查一下北京天气，还有现在几点了"
	s.logger.Info("提问", logging.String("question", q))

	resp, err := s.brain.Post(&core.ThinkingRequest{
		Question: q,
		Timeout:  30,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "回答不能为空")

	// 验证：应该识别到至少一个工具
	s.Greater(len(resp.Tools), 0,
		"应该识别到工具，实际工具数量: %d", len(resp.Tools))

	s.logger.Info("响应", logging.String("answer", resp.Answer),
		logging.Int("tools_count", len(resp.Tools)))

	for i, tool := range resp.Tools {
		s.logger.Info("工具%d", logging.Int("index", i),
			logging.String("name", tool.Name))
	}
}

// TestSkill_NoIntent 测试：无技能意图
// 验证：当问题不需要技能时，不返回工具
func (s *SkillExecutionSuite) TestSkill_NoIntent() {
	s.brain = s.createTestBrain()

	// 纯闲聊问题
	q := "你今天心情怎么样？"
	s.logger.Info("提问", logging.String("question", q))

	resp, err := s.brain.Post(&core.ThinkingRequest{
		Question: q,
		Timeout:  30,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "回答不能为空")

	// 验证：不应该有工具
	s.Equal(len(resp.Tools), 0,
		"闲聊问题不应该返回工具，实际工具数量: %d", len(resp.Tools))

	s.logger.Info("响应", logging.String("answer", resp.Answer))
}
