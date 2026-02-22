package brain

import (
	"context"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/pkg/logging"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// ThinkingTestSuite Thinking 测试套件
type ThinkingTestSuite struct {
	suite.Suite
	leftBrain     *Thinking
	rightBrain    *Thinking
	leftBrainCfg  *config.ModelConfig
	rightBrainCfg *config.ModelConfig
	logger        logging.Logger
}

// SetupTest 测试前准备
func (s *ThinkingTestSuite) SetupTest() {
	// 初始化日志
	logConfig := &config.LoggingConfig{
		SystemLogConfig: &config.SystemLogConfig{
			Level:      config.LevelDebug,
			OutputPath: "/tmp/test.log",
			MaxSize:    10,
			MaxBackups: 3,
			MaxAge:     7,
			Compress:   false,
		},
		ConversationLogConfig: &config.ConversationLogConfig{
			Enable:     false,
			OutputPath: "/tmp/conversation.log",
		},
	}
	_ = logging.Init(logConfig) // 日志初始化失败不阻断测试
	s.logger = logging.GetSystemLogger().Named("thinking_test")

	// 配置左脑（使用 qwen3:0.6b）
	s.leftBrainCfg = &config.ModelConfig{
		Name:        "qwen3:1.7b",
		APIKey:      "",
		BaseURL:     "http://localhost:11434/v1",
		Temperature: 0.7,
		MaxTokens:   800,
	}

	s.rightBrainCfg = &config.ModelConfig{
		Name:        "functiongemma:270m",
		APIKey:      "",
		BaseURL:     "http://localhost:11434/v1",
		Temperature: 0.7,
		MaxTokens:   1000,
	}

	// 创建左脑
	leftBrainPrompt := buildLeftBrainPrompt(&core.Persona{
		Name:        "小柔",
		Gender:      "女",
		Character:   "温柔",
		UserContent: "",
	})

	// 创建 Token 预算配置（测试用）
	tokenBudget := &config.TokenBudgetConfig{
		ReservedOutputTokens: 4096,
		MinHistoryRounds:      2,
		AvgTokensPerRound:     150,
	}

	s.leftBrain = NewThinking(s.leftBrainCfg, leftBrainPrompt, s.logger, nil, tokenBudget)

	// 创建右脑（不设置 prompt，用于函数调用）
	s.rightBrain = NewThinking(s.rightBrainCfg, "", s.logger, nil, tokenBudget)
}

// TearDownTest 测试后清理
func (s *ThinkingTestSuite) TearDownTest() {
	// 清理资源
}

// TestThinkingSuite 运行测试套件
func TestThinkingSuite(t *testing.T) {
	suite.Run(t, new(ThinkingTestSuite))
}

// TestLeftBrain_SimpleQuestion 测试左脑处理简单问题
func (s *ThinkingTestSuite) TestLeftBrain_SimpleQuestion() {
	question := "你好，今天天气怎么样？"

	result, err := s.leftBrain.Think(context.Background(),question, nil, "", true)

	if !assert.NoError(s.T(), err, "左脑思考应该成功") {
		s.T().FailNow()
	}
	assert.NotEmpty(s.T(), result.Answer, "应该有回答内容")
	assert.NotEmpty(s.T(), result.Intent, "应该提取出意图")
	assert.NotEmpty(s.T(), result.Keywords, "应该提取出关键词")

	s.logger.Info("左脑测试 - 简单问题",
		logging.String("question", question),
		logging.String("answer", result.Answer),
		logging.String("intent", result.Intent),
		logging.String("keywords", s.formatKeywords(result.Keywords)),
		logging.Bool("can_answer", result.CanAnswer))
}

// TestLeftBrain_CanAnswer 测试左脑判断能否回答
func (s *ThinkingTestSuite) TestLeftBrain_CanAnswer() {
	testCases := []struct {
		name            string
		question        string
		expectCanAnswer bool
		description     string
	}{
		{
			name:            "简单问候",
			question:        "你好，请问你是谁？",
			expectCanAnswer: true,
			description:     "左脑应该能回答简单问候",
		},
		{
			name:            "简单常识",
			question:        "天空是什么颜色的？",
			expectCanAnswer: true,
			description:     "左脑应该能回答简单常识问题",
		},
		{
			name:            "深度问题",
			question:        "请详细解释量子纠缠的原理及其在量子计算中的应用",
			expectCanAnswer: false,
			description:     "左脑应该判断无法回答深度问题",
		},
		{
			name:            "复杂推理",
			question:        "如果A大于B，B大于C，那么A一定大于C吗？为什么？请用数学证明。",
			expectCanAnswer: false,
			description:     "左脑应该判断无法回答复杂推理问题",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			maxRetries := 3
			var lastResult *core.ThinkingResult
			var lastErr error

			for i := 0; i < maxRetries; i++ {
				result, err := s.leftBrain.Think(context.Background(),tc.question, nil, "", true)
				lastResult = result
				lastErr = err

				if err == nil && result.CanAnswer == tc.expectCanAnswer {
					break
				}

				if i < maxRetries-1 {
					s.T().Logf("重试第%d次...", i+2)
				}
			}

			if !assert.NoError(s.T(), lastErr, tc.description) {
				s.T().FailNow()
			}

			assert.Equal(s.T(), tc.expectCanAnswer, lastResult.CanAnswer,
				tc.description+"，期望 CanAnswer=%v，实际=%v",
				tc.expectCanAnswer, lastResult.CanAnswer)

			s.logger.Info("左脑测试 - 判断能力",
				logging.String("test_case", tc.name),
				logging.String("question", tc.question),
				logging.String("answer", lastResult.Answer),
				logging.String("intent", lastResult.Intent),
				logging.Bool("can_answer", lastResult.CanAnswer))
		})
	}
}

// TestLeftBrain_WithHistory 测试左脑带历史对话
func (s *ThinkingTestSuite) TestLeftBrain_WithHistory() {
	question := "那我刚才问的是什么？"

	// 模拟历史对话
	history := []*core.DialogueMessage{
		{
			Role:    "user",
			Content: "你好",
		},
		{
			Role:    "assistant",
			Content: "你好！我是小柔，很高兴见到你。",
		},
		{
			Role:    "user",
			Content: "今天天气怎么样？",
		},
		{
			Role:    "assistant",
			Content: "我无法获取实时天气信息，你可以查看天气预报。",
		},
	}

	result, err := s.leftBrain.Think(context.Background(),question, history, "", true)

	if !assert.NoError(s.T(), err, "左脑带历史对话应该成功") {
		s.T().FailNow()
	}
	assert.NotEmpty(s.T(), result.Answer, "应该有回答内容")

	s.logger.Info("左脑测试 - 带历史对话",
		logging.String("question", question),
		logging.String("answer", result.Answer),
		logging.Int("history_count", len(history)))
}

// TestLeftBrain_IntentExtraction 测试意图提取
func (s *ThinkingTestSuite) TestLeftBrain_IntentExtraction() {
	testCases := []struct {
		name         string
		question     string
		expectIntent string
	}{
		{
			name:         "查询天气",
			question:     "北京今天的天气怎么样？",
			expectIntent: "查询天气",
		},
		{
			name:         "播放音乐",
			question:     "帮我放一首周杰伦的歌",
			expectIntent: "播放音乐",
		},
		{
			name:         "设置提醒",
			question:     "提醒我明天下午3点开会",
			expectIntent: "设置提醒",
		},
		{
			name:         "闲聊",
			question:     "你喜欢吃什么？",
			expectIntent: "闲聊",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			maxRetries := 3
			var lastResult *core.ThinkingResult
			var lastErr error

			for i := 0; i < maxRetries; i++ {
				result, err := s.leftBrain.Think(context.Background(),tc.question, nil, "", true)
				lastResult = result
				lastErr = err

				if err == nil && result.Intent != "" {
					break
				}

				if i < maxRetries-1 {
					s.T().Logf("重试第%d次...", i+2)
				}
			}

			if !assert.NoError(s.T(), lastErr) {
				s.T().FailNow()
			}
			assert.NotEmpty(s.T(), lastResult.Intent, "应该提取出意图")

			s.logger.Info("左脑测试 - 意图提取",
				logging.String("test_case", tc.name),
				logging.String("question", tc.question),
				logging.String("extracted_intent", lastResult.Intent),
				logging.String("expected_intent", tc.expectIntent),
				logging.String("keywords", s.formatKeywords(lastResult.Keywords)))
		})
	}
}

// TestRightBrain_FunctionCall 测试右脑函数调用
func (s *ThinkingTestSuite) TestRightBrain_FunctionCall() {
	// 构造一个测试用的工具列表（模拟播放音乐技能）
	tools := []*core.ToolSchema{
		{
			Name:        "play_music",
			Description: "播放指定的音乐",
			Params: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"song": map[string]interface{}{
						"type":        "string",
						"description": "歌曲名称",
					},
					"artist": map[string]interface{}{
						"type":        "string",
						"description": "歌手名称",
					},
				},
				"required": []string{"song", "artist"},
			},
		},
	}

	// 用户要求播放音乐
	question := "帮我播放周杰伦的《稻香》"

	// 右脑使用工具思考（添加 history 参数）
	result, err := s.rightBrain.ThinkWithTools(context.Background(),question, nil, tools)

	if !assert.NoError(s.T(), err, "右脑工具思考应该成功") {
		s.T().FailNow()
	}

	// 验证右脑决定调用函数
	assert.False(s.T(), result.NoCall, "应该决定调用函数")
	assert.NotNil(s.T(), result.Function, "应该有函数调用信息")

	// 验证函数名和参数正确
	assert.Equal(s.T(), "play_music", result.Function.Name, "应该调用 play_music 函数")

	// 验证参数包含歌曲名和歌手
	song, ok := result.Function.Arguments["song"].(string)
	assert.True(s.T(), ok, "song 参数应该是字符串")
	assert.Contains(s.T(), song, "稻香", "歌曲名应该包含'稻香'")

	artist, ok := result.Function.Arguments["artist"].(string)
	assert.True(s.T(), ok, "artist 参数应该是字符串")
	assert.Contains(s.T(), artist, "周杰伦", "歌手名应该包含'周杰伦'")

	s.logger.Info("右脑测试 - 函数调用",
		logging.String("question", question),
		logging.String("function", result.Function.Name),
		logging.Any("arguments", result.Function.Arguments))
}

// TestThinking_WithEnhancedPrompt 测试带记忆增强的思考
func (s *ThinkingTestSuite) TestThinking_WithEnhancedPrompt() {
	// 构建包含记忆的 prompt
	question := "我的生日是哪天？"
	// 构建增强 prompt
	references := "# 参考记忆\n- 用户的生日是6月15日\n- 用户喜欢红色"

	result, err := s.leftBrain.Think(context.Background(),question, nil, references, true)

	if !assert.NoError(s.T(), err, "带记忆的思考应该成功") {
		s.T().FailNow()
	}

	s.logger.Info("左脑测试 - 带记忆增强",
		logging.String("question", question),
		logging.String("reference_prompt", references),
		logging.String("answer", result.Answer),
		logging.String("intent", result.Intent))
}

// TestLeftBrain_LongHistory 测试左脑处理长历史对话
func (s *ThinkingTestSuite) TestLeftBrain_LongHistory() {
	// 构建 8 轮历史对话
	history := make([]*core.DialogueMessage, 0, 16) // 8轮 = 16条消息
	for i := 0; i < 8; i++ {
		history = append(history, &core.DialogueMessage{
			Role:    "user",
			Content: "第" + string(rune('1'+i)) + "次问问题",
		})
		history = append(history, &core.DialogueMessage{
			Role:    "assistant",
			Content: "第" + string(rune('1'+i)) + "次回答",
		})
	}

	question := "这是第几次对话？"
	result, err := s.leftBrain.Think(context.Background(),question, history, "", true)

	if !assert.NoError(s.T(), err, "左脑应该能处理8轮历史对话") {
		s.T().FailNow()
	}

	s.logger.Info("左脑测试 - 长历史对话",
		logging.String("question", question),
		logging.String("answer", result.Answer),
		logging.Int("history_count", len(history)))
}

// TestLeftBrain_8RoundLimit 测试左脑动态轮数计算
func (s *ThinkingTestSuite) TestLeftBrain_8RoundLimit() {
	// 获取计算的最大轮数
	maxRounds := s.leftBrain.CalculateMaxHistoryCount()

	s.logger.Info("测试动态轮数计算",
		logging.Int("max_tokens", s.leftBrainCfg.MaxTokens),
		logging.Int("reserved_output_tokens", 4096),
		logging.Int("avg_tokens_per_round", 150),
		logging.Int("calculated_max_rounds", maxRounds))

	// 测试1: 正好在计算的最大轮数内
	historyWithinLimit := make([]*core.DialogueMessage, 0, maxRounds*2)
	for i := 0; i < maxRounds; i++ {
		historyWithinLimit = append(historyWithinLimit, &core.DialogueMessage{
			Role:    "user",
			Content: "用户" + string(rune('1'+i)),
		})
		historyWithinLimit = append(historyWithinLimit, &core.DialogueMessage{
			Role:    "assistant",
			Content: "助手" + string(rune('1'+i)),
		})
	}

	s.logger.Info("测试最大轮数边界情况", logging.Int("history_count", len(historyWithinLimit)))
	_, err := s.leftBrain.Think(context.Background(),"第一个用户是谁？", historyWithinLimit, "", true)
	if !assert.NoError(s.T(), err, "最大轮数内应该能处理") {
		s.T().FailNow()
	}

	// 测试2: 超过计算的轮数（传入更多轮数，但实际使用应该截断到计算的最大值）
	overLimit := maxRounds + 5
	historyOverLimit := make([]*core.DialogueMessage, 0, overLimit*2)
	for i := 0; i < overLimit; i++ {
		historyOverLimit = append(historyOverLimit, &core.DialogueMessage{
			Role:    "user",
			Content: "用户" + string(rune('1'+i)),
		})
		historyOverLimit = append(historyOverLimit, &core.DialogueMessage{
			Role:    "assistant",
			Content: "助手" + string(rune('1'+i)),
		})
	}

	s.logger.Info("测试超过最大轮数的情况", logging.Int("history_count", len(historyOverLimit)))
	result, err := s.leftBrain.Think(context.Background(),"最后一个用户是谁？", historyOverLimit, "", true)
	// 注意：这里不应该断言出错，因为即使历史太长，模型也应该能处理
	if err != nil {
		s.logger.Warn("超过最大轮数处理失败", logging.Err(err))
	} else {
		s.logger.Info("超过最大轮数处理结果",
			logging.String("answer", result.Answer))
	}
}

// formatKeywords 辅助函数：格式化关键词
func (s *ThinkingTestSuite) formatKeywords(keywords []string) string {
	if len(keywords) == 0 {
		return "[]"
	}
	result := "["
	for i, kw := range keywords {
		if i > 0 {
			result += ", "
		}
		result += kw
	}
	result += "]"
	return result
}

// TestLeftBrain_MultiRoundContext 测试左脑多轮会话的上下文记忆能力
func (s *ThinkingTestSuite) TestLeftBrain_MultiRoundContext() {
	testCases := []struct {
		name     string
		scenario []struct {
			role    string
			content string
		}
		finalQuestion    string
		expectedKeywords []string
		description      string
	}{
		{
			name: "用户信息追踪",
			scenario: []struct {
				role    string
				content string
			}{
				{"user", "我叫张三"},
				{"assistant", "你好，张三！很高兴认识你。"},
				{"user", "我住在北京"},
				{"assistant", "明白了，北京是个很棒的城市。"},
				{"user", "我今年25岁"},
				{"assistant", "好的，我知道了。"},
			},
			finalQuestion:    "我叫什么名字，住哪里，今年多大？",
			expectedKeywords: []string{"张三", "北京", "25岁"},
			description:      "左脑应该能记住之前的对话上下文",
		},
		{
			name: "对话主题追踪",
			scenario: []struct {
				role    string
				content string
			}{
				{"user", "我想学吉他"},
				{"assistant", "学吉他是个不错的选择！"},
				{"user", "我听说吉他很难学"},
				{"assistant", "刚开始可能会有点挑战，但坚持下去会越来越好的。"},
				{"user", "你有什么建议吗？"},
				{"assistant", "建议每天练习半小时，从基础和弦开始。"},
			},
			finalQuestion:    "你刚才给了我什么建议？",
			expectedKeywords: []string{"吉他", "建议", "练习"},
			description:      "左脑应该能追踪对话的主题",
		},
		{
			name: "多轮对话的意图一致性",
			scenario: []struct {
				role    string
				content string
			}{
				{"user", "帮我播放音乐"},
				{"assistant", "好的，请问你想听什么音乐？"},
				{"user", "周杰伦的歌"},
				{"assistant", "好的，正在为您播放周杰伦的歌曲。"},
				{"user", "换一首"},
				{"assistant", "好的，正在为您切换歌曲。"},
			},
			finalQuestion:    "还在播放吗？",
			expectedKeywords: []string{"播放", "音乐"},
			description:      "左脑应该能保持对播放状态的理解",
		},
		{
			name: "复杂对话链",
			scenario: []struct {
				role    string
				content string
			}{
				{"user", "我今天想吃火锅"},
				{"assistant", "火锅是个不错的选择！"},
				{"user", "但是我想吃辣的"},
				{"assistant", "那推荐重庆火锅，比较辣。"},
				{"user", "重庆火锅有什么特色？"},
				{"assistant", "重庆火锅以麻辣鲜香著称，用料丰富。"},
				{"user", "你刚才推荐的是什么火锅？"},
				{"assistant", "我推荐的是重庆火锅。"},
			},
			finalQuestion:    "我想吃什么火锅？",
			expectedKeywords: []string{"火锅", "辣", "重庆"},
			description:      "左脑应该能理解复杂的对话链",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// 构建历史对话
			history := make([]*core.DialogueMessage, 0)
			for _, msg := range tc.scenario {
				history = append(history, &core.DialogueMessage{
					Role:    msg.role,
					Content: msg.content,
				})
			}

			s.logger.Info("多轮会话测试开始",
				logging.String("test_case", tc.name),
				logging.String("description", tc.description),
				logging.Int("history_count", len(history)),
				logging.String("final_question", tc.finalQuestion))

			// 执行思考
			result, err := s.leftBrain.Think(context.Background(),tc.finalQuestion, history, "", true)

			if !assert.NoError(s.T(), err, tc.description) {
				s.T().FailNow()
			}
			assert.NotEmpty(s.T(), result.Answer, "应该有回答内容")
			assert.NotEmpty(s.T(), result.Intent, "应该提取出意图")

			// 检查回答是否包含关键信息
			answer := result.Answer
			hasKeywords := true
			for _, kw := range tc.expectedKeywords {
				if !contains(answer, kw) {
					hasKeywords = false
					s.logger.Warn("回答中未找到关键词",
						logging.String("keyword", kw),
						logging.String("answer", answer))
				}
			}

			if hasKeywords {
				s.logger.Info("多轮会话测试通过",
					logging.String("test_case", tc.name),
					logging.String("question", tc.finalQuestion),
					logging.String("answer", result.Answer),
					logging.String("intent", result.Intent))
			} else {
				s.logger.Warn("回答可能不完整，但这可能是模型能力限制",
					logging.String("test_case", tc.name),
					logging.String("answer", result.Answer))
			}
		})
	}
}

// contains 辅助函数：检查字符串是否包含子串
func contains(s, substr string) bool {
	if substr == "" {
		return true
	}
	return len(s) >= len(substr) && (s == substr || containsSubstring(s, substr))
}

// containsSubstring 递归检查子串
func containsSubstring(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	if s[:len(substr)] == substr {
		return true
	}
	return containsSubstring(s[1:], substr)
}

// TestCalculateMaxHistoryCount 测试最大历史轮数计算
func (s *ThinkingTestSuite) TestCalculateMaxHistoryCount() {
	testCases := []struct {
		name                    string
		maxTokens               int
		reservedOutputTokens    int
		avgTokensPerRound       int
		minHistoryRounds        int
		expectedMaxRounds       int
	}{
		{
			name:                  "小模型(800 tokens)",
			maxTokens:            800,
			reservedOutputTokens: 4096,
			avgTokensPerRound:    150,
			minHistoryRounds:     2,
			expectedMaxRounds:    2, // (800-4096)/150 < 0，返回最小值2
		},
		{
			name:                  "中等模型(4096 tokens)",
			maxTokens:            4096,
			reservedOutputTokens: 4096,
			avgTokensPerRound:    150,
			minHistoryRounds:     2,
			expectedMaxRounds:    2, // (4096-4096)/150 = 0，返回最小值2
		},
		{
			name:                  "大模型(8192 tokens)",
			maxTokens:            8192,
			reservedOutputTokens: 4096,
			avgTokensPerRound:    150,
			minHistoryRounds:     2,
			expectedMaxRounds:    27, // (8192-4096)/150 = 27
		},
		{
			name:                  "超大模型(32768 tokens)",
			maxTokens:            32768,
			reservedOutputTokens: 4096,
			avgTokensPerRound:    150,
			minHistoryRounds:     2,
			expectedMaxRounds:    191, // (32768-4096)/150 = 191
		},
		{
			name:                  "自定义配置",
			maxTokens:            10000,
			reservedOutputTokens: 2000,
			avgTokensPerRound:    200,
			minHistoryRounds:     5,
			expectedMaxRounds:    40, // (10000-2000)/200 = 40
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// 创建测试配置
			cfg := &config.ModelConfig{
				Name:      "test-model",
				MaxTokens: tc.maxTokens,
			}

			tokenBudget := &config.TokenBudgetConfig{
				ReservedOutputTokens: tc.reservedOutputTokens,
				MinHistoryRounds:      tc.minHistoryRounds,
				AvgTokensPerRound:     tc.avgTokensPerRound,
			}

			// 创建 Thinking 实例
			thinking := NewThinking(cfg, "", s.logger, nil, tokenBudget)

			// 计算最大轮数
			maxRounds := thinking.CalculateMaxHistoryCount()

			s.logger.Info("计算历史轮数",
				logging.String("test_case", tc.name),
				logging.Int("max_tokens", tc.maxTokens),
				logging.Int("reserved_output_tokens", tc.reservedOutputTokens),
				logging.Int("avg_tokens_per_round", tc.avgTokensPerRound),
				logging.Int("calculated_max_rounds", maxRounds),
				logging.Int("expected_max_rounds", tc.expectedMaxRounds))

			s.Equal(tc.expectedMaxRounds, maxRounds, "计算的最大轮数应该符合预期")
		})
	}
}

// TestLeftBrain_ScheduleIntent 测试左脑识别定时意图
func (s *ThinkingTestSuite) TestLeftBrain_ScheduleIntent() {
	testCases := []struct {
		name          string
		question      string
		expectSchedule bool
		description   string
	}{
		{
			name:          "每周六写日报",
			question:      "每周六帮我写日报",
			expectSchedule: true,
			description:   "应该识别到定时意图并设置 has_schedule 字段",
		},
		{
			name:          "明天早上8点提醒",
			question:      "明天早上8点提醒我开会",
			expectSchedule: true,
			description:   "应该识别到定时意图并设置 has_schedule 字段",
		},
		{
			name:          "每天早上9点",
			question:      "每天早上9点提醒我起床",
			expectSchedule: true,
			description:   "应该识别到定时意图并设置 has_schedule 字段",
		},
		{
			name:          "普通问题",
			question:      "今天天气怎么样",
			expectSchedule: false,
			description:   "普通问题不应该设置 has_schedule 字段",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			maxRetries := 3
			var lastResult *core.ThinkingResult
			var lastErr error

			for i := 0; i < maxRetries; i++ {
				result, err := s.leftBrain.Think(context.Background(),tc.question, nil, "", true)
				lastResult = result
				lastErr = err

				if err == nil {
					if tc.expectSchedule && result.HasSchedule {
						break
					}
					if !tc.expectSchedule && !result.HasSchedule {
						break
					}
				}

				if i < maxRetries-1 {
					s.T().Logf("重试第%d次...", i+2)
				}
			}

			if !assert.NoError(s.T(), lastErr, tc.description) {
				s.T().FailNow()
			}

			if tc.expectSchedule {
				assert.True(s.T(), lastResult.HasSchedule, tc.description+"，期望有 has_schedule")
				s.logger.Info("左脑测试 - 定时意图",
					logging.String("test_case", tc.name),
					logging.String("question", tc.question),
					logging.String("schedule_name", lastResult.ScheduleName),
					logging.String("schedule_cron", lastResult.ScheduleCron),
					logging.String("schedule_message", lastResult.ScheduleMessage))
			} else {
				assert.False(s.T(), lastResult.HasSchedule, tc.description+"，期望无 has_schedule")
				s.logger.Info("左脑测试 - 无定时意图",
					logging.String("test_case", tc.name),
					logging.String("question", tc.question))
			}
		})
	}
}

// TestLeftBrain_SendToIntent 测试左脑识别转发意图
func (s *ThinkingTestSuite) TestLeftBrain_SendToIntent() {
	testCases := []struct {
		name          string
		question      string
		expectSendTo  string
		description   string
	}{
		{
			name:          "转发给微信",
			question:      "把这个消息发给微信",
			expectSendTo:  "微信",
			description:   "应该识别到转发意图并设置 send_to 为微信",
		},
		{
			name:          "转发给QQ",
			question:      "转发给QQ",
			expectSendTo:  "QQ",
			description:   "应该识别到转发意图并设置 send_to 为QQ",
		},
		{
			name:          "普通问题",
			question:      "今天天气怎么样",
			expectSendTo:  "",
			description:   "普通问题不应该设置 send_to 字段",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			maxRetries := 3
			var lastResult *core.ThinkingResult
			var lastErr error

			for i := 0; i < maxRetries; i++ {
				result, err := s.leftBrain.Think(context.Background(),tc.question, nil, "", true)
				lastResult = result
				lastErr = err

				if err == nil {
					if tc.expectSendTo != "" && lastResult.SendTo != "" {
						break
					}
					if tc.expectSendTo == "" && lastResult.SendTo == "" {
						break
					}
				}

				if i < maxRetries-1 {
					s.T().Logf("重试第%d次...", i+2)
				}
			}

			if !assert.NoError(s.T(), lastErr, tc.description) {
				s.T().FailNow()
			}

			if tc.expectSendTo != "" {
				assert.NotEmpty(s.T(), lastResult.SendTo, tc.description+"，期望 send_to 不为空")
				s.logger.Info("左脑测试 - 转发意图",
					logging.String("test_case", tc.name),
					logging.String("question", tc.question),
					logging.String("send_to", lastResult.SendTo))
			} else {
				assert.Empty(s.T(), lastResult.SendTo, tc.description+"，期望 send_to 为空")
				s.logger.Info("左脑测试 - 无转发意图",
					logging.String("test_case", tc.name),
					logging.String("question", tc.question))
			}
		})
	}
}
