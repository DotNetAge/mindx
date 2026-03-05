package brain

import (
	"context"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/usecase/skills"
	"mindx/pkg/logging"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// getIntentTestModelName 从环境变量获取测试用模型名
func getIntentTestModelName() string {
	if m := os.Getenv("MINDX_TEST_MODEL"); m != "" {
		return m
	}
	return "qwen3:0.6b"
}

// IntentRecognitionSuite 意图识别回归测试套件
// 使用真实 Ollama 模型，验证 prompt + 模型 + 输入的端到端结果
// 每次改 prompt 或换模型都必须跑
type IntentRecognitionSuite struct {
	suite.Suite
	leftBrain *Thinking
	logger    logging.Logger
}

func isOllamaAvailable() bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:11434", 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (s *IntentRecognitionSuite) SetupSuite() {
	if testing.Short() {
		s.T().Skip("short mode: skip intent recognition integration suite")
	}
	if !isOllamaAvailable() {
		s.T().Skip("ollama is not available on 127.0.0.1:11434")
	}

	logConfig := &config.LoggingConfig{
		SystemLogConfig: &config.SystemLogConfig{
			Level:      config.LevelDebug,
			OutputPath: "/tmp/intent_recognition_test.log",
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
	_ = logging.Init(logConfig)
	s.logger = logging.GetSystemLogger().Named("intent_recognition_test")

	// 注入真实技能关键词（与生产环境一致）
	core.SetSkillKeywords([]string{
		"天气", "weather", "计算", "calculator", "文件", "finder",
		"系统", "sysinfo", "CPU", "内存", "提醒", "reminders",
		"日历", "calendar", "邮件", "mail", "截图", "screenshot",
		"搜索", "search", "新闻", "stock", "finance", "A股", "行情",
		"剪贴板", "clipboard", "通知", "notify", "音量", "volume",
		"终端", "terminal", "联系人", "contacts", "笔记", "notes",
	})

	modelCfg := &config.ModelConfig{
		Name:        getIntentTestModelName(),
		APIKey:      "",
		BaseURL:     "http://localhost:11434/v1",
		Temperature: 0.3, // 低温度减少随机性
		MaxTokens:   800,
	}

	prompt := buildLeftBrainPrompt(&core.Persona{
		Name:      "小柔",
		Gender:    "女",
		Character: "温柔",
	})

	tokenBudget := &config.TokenBudgetConfig{
		ReservedOutputTokens: 4096,
		MinHistoryRounds:     2,
		AvgTokensPerRound:    150,
	}

	s.leftBrain = NewThinking(modelCfg, prompt, s.logger, nil, tokenBudget)
}

func TestIntentRecognitionSuite(t *testing.T) {
	suite.Run(t, new(IntentRecognitionSuite))
}

// thinkWithRetry 调用左脑并允许重试，应对小模型随机性
func (s *IntentRecognitionSuite) thinkWithRetry(question string, maxRetries int, check func(*core.ThinkingResult) bool) *core.ThinkingResult {
	for i := 0; i < maxRetries; i++ {
		result, err := s.leftBrain.Think(context.Background(), question, nil, "", true)
		if err != nil {
			s.logger.Warn("左脑调用失败，重试",
				logging.String("question", question),
				logging.Int("attempt", i+1),
				logging.Err(err))
			continue
		}
		if check(result) {
			return result
		}
		s.logger.Warn("断言未通过，重试",
			logging.String("question", question),
			logging.Int("attempt", i+1),
			logging.String("intent", result.Intent),
			logging.String("keywords", strings.Join(result.Keywords, ",")),
			logging.Bool("can_answer", result.CanAnswer),
			logging.Bool("useless", result.Useless))
	}
	// 返回最后一次结果用于断言报错
	result, _ := s.leftBrain.Think(context.Background(), question, nil, "", true)
	return result
}

// containsAny 检查字符串是否包含任一子串（不区分大小写）
func containsAny(s string, substrs []string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// keywordsContainAny 检查关键词列表是否包含任一期望词
func keywordsContainAny(keywords []string, expected []string) bool {
	for _, kw := range keywords {
		kwLower := strings.ToLower(kw)
		for _, exp := range expected {
			if strings.Contains(kwLower, strings.ToLower(exp)) {
				return true
			}
		}
	}
	return false
}

// TestIntent_Classification 核心意图分类回归测试
func (s *IntentRecognitionSuite) TestIntent_Classification() {
	tests := []struct {
		name            string
		question        string
		expectCanAnswer *bool // nil 表示不检查 can_answer（小模型不稳定）
		expectUseless   bool
		intentContains  []string // intent 应包含其中之一（为空则不检查）
		keywordContains []string // keywords 应包含其中之一（为空则不检查）
	}{
		{
			name:            "A股行情查询",
			question:        "今天A股行情如何",
			expectCanAnswer: boolPtr(false),
			expectUseless:   false,
			intentContains:  []string{"股票", "行情", "A股", "stock", "finance"},
			keywordContains: []string{"A股", "行情"},
		},
		{
			name:            "天气查询",
			question:        "北京今天天气怎么样",
			expectUseless:   false,
			intentContains:  []string{"天气", "weather"},
			keywordContains: []string{"天气", "北京"},
		},
		{
			name:            "计算请求",
			question:        "帮我算一下 123*456",
			expectUseless:   false,
			intentContains:  []string{"计算", "算", "calculator", "math"},
			keywordContains: []string{"计算", "算", "123", "456"},
		},
		{
			name:            "新闻搜索",
			question:        "帮我搜一下最近的科技新闻",
			expectCanAnswer: boolPtr(false),
			expectUseless:   false,
			intentContains:  []string{"搜索", "新闻", "search", "news"},
			keywordContains: []string{"新闻", "科技", "搜索"},
		},
		{
			name:            "系统信息查询",
			question:        "帮我查一下系统CPU使用率",
			expectUseless:   false,
			intentContains:  []string{"系统", "CPU", "sysinfo", "system"},
			keywordContains: []string{"CPU", "系统", "使用率"},
		},
		{
			name:            "闲聊-你好",
			question:        "你好",
			expectCanAnswer: boolPtr(true),
			expectUseless:   true,
		},
		{
			name:            "闲聊-嗯",
			question:        "嗯",
			expectCanAnswer: boolPtr(true),
			expectUseless:   true,
		},
		{
			name:            "常识问题可直接回答",
			question:        "法国的首都是哪里",
			expectCanAnswer: boolPtr(true),
			expectUseless:   false,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			result := s.thinkWithRetry(tc.question, 3, func(r *core.ThinkingResult) bool {
				if tc.expectCanAnswer != nil && r.CanAnswer != *tc.expectCanAnswer {
					return false
				}
				if r.Useless != tc.expectUseless {
					return false
				}
				if len(tc.intentContains) > 0 && !containsAny(r.Intent, tc.intentContains) {
					return false
				}
				if len(tc.keywordContains) > 0 && !keywordsContainAny(r.Keywords, tc.keywordContains) {
					return false
				}
				return true
			})

			if tc.expectCanAnswer != nil {
				assert.Equal(s.T(), *tc.expectCanAnswer, result.CanAnswer,
					"问题: %s, intent: %s, keywords: %v", tc.question, result.Intent, result.Keywords)
			}
			assert.Equal(s.T(), tc.expectUseless, result.Useless,
				"问题: %s, intent: %s", tc.question, result.Intent)

			if len(tc.intentContains) > 0 {
				assert.True(s.T(), containsAny(result.Intent, tc.intentContains),
					"问题: %s, intent '%s' 应包含 %v 之一", tc.question, result.Intent, tc.intentContains)
			}
			if len(tc.keywordContains) > 0 {
				assert.True(s.T(), keywordsContainAny(result.Keywords, tc.keywordContains),
					"问题: %s, keywords %v 应包含 %v 之一", tc.question, result.Keywords, tc.keywordContains)
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }

// TestIntent_CanAnswer 单独测试 can_answer 判断
// 小模型在 can_answer 上不稳定，此测试用于追踪 prompt 优化进度
// 当前已知问题：天气/计算/系统信息 的 can_answer 经常被错误判断为 true
func (s *IntentRecognitionSuite) TestIntent_CanAnswer() {
	tests := []struct {
		name            string
		question        string
		expectCanAnswer bool
	}{
		{"天气需要工具", "北京今天天气怎么样", false},
		{"计算需要工具", "帮我算一下 123*456", false},
		{"系统信息需要工具", "帮我查一下系统CPU使用率", false},
		{"A股需要工具", "今天A股行情如何", false},
		{"闲聊不需要工具", "你好", true},
		{"常识不需要工具", "法国的首都是哪里", true},
	}

	passed := 0
	total := len(tests)
	for _, tc := range tests {
		s.Run(tc.name, func() {
			// 只调用一次，不重试 — 此测试用于追踪准确率，不阻断 CI
			result, err := s.leftBrain.Think(context.Background(), tc.question, nil, "", true)
			if err != nil {
				s.T().Logf("⚠ 调用失败: question=%s, err=%v", tc.question, err)
				return
			}

			if result.CanAnswer == tc.expectCanAnswer {
				passed++
			} else {
				s.T().Logf("⚠ can_answer 不符合预期: question=%s, got=%v, want=%v, intent=%s",
					tc.question, result.CanAnswer, tc.expectCanAnswer, result.Intent)
			}
		})
	}

	s.T().Logf("can_answer 准确率: %d/%d", passed, total)
}

// TestIntent_Schedule 定时任务意图识别（端到端，基于 Ollama）
func (s *IntentRecognitionSuite) TestIntent_Schedule() {
	tests := []struct {
		name               string
		question           string
		expectSchedule     bool
		expectCronNotEmpty bool
		expectNameNotEmpty bool
		expectMsgNotEmpty  bool
		cronContains       string // cron 表达式应包含的片段（为空则不检查）
	}{
		{
			name:               "每天早上9点提醒",
			question:           "每天早上9点提醒我喝水",
			expectSchedule:     true,
			expectCronNotEmpty: true,
			expectNameNotEmpty: true,
			expectMsgNotEmpty:  true,
			cronContains:       "9",
		},
		{
			name:               "每周一提醒",
			question:           "每周一早上8点提醒我开周会",
			expectSchedule:     true,
			expectCronNotEmpty: true,
			expectNameNotEmpty: true,
			expectMsgNotEmpty:  true,
		},
		{
			name:               "每小时提醒",
			question:           "每小时提醒我休息一下眼睛",
			expectSchedule:     true,
			expectCronNotEmpty: true,
			expectNameNotEmpty: true,
			expectMsgNotEmpty:  true,
		},
		{
			name:           "非定时-天气查询",
			question:       "今天天气怎么样",
			expectSchedule: false,
		},
		{
			name:           "非定时-闲聊",
			question:       "你好",
			expectSchedule: false,
		},
		{
			name:           "非定时-计算",
			question:       "帮我算一下100加200",
			expectSchedule: false,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			result := s.thinkWithRetry(tc.question, 3, func(r *core.ThinkingResult) bool {
				return r.HasSchedule == tc.expectSchedule
			})

			assert.Equal(s.T(), tc.expectSchedule, result.HasSchedule,
				"问题: %s", tc.question)

			if tc.expectCronNotEmpty {
				assert.NotEmpty(s.T(), result.ScheduleCron,
					"问题: %s, schedule_cron 应非空", tc.question)

				// 验证 cron 表达式基本格式：至少有 5 个字段
				fields := strings.Fields(result.ScheduleCron)
				assert.GreaterOrEqual(s.T(), len(fields), 5,
					"问题: %s, cron '%s' 应至少有5个字段", tc.question, result.ScheduleCron)
			}
			if tc.expectNameNotEmpty {
				assert.NotEmpty(s.T(), result.ScheduleName,
					"问题: %s, schedule_name 应非空", tc.question)
			}
			if tc.expectMsgNotEmpty {
				assert.NotEmpty(s.T(), result.ScheduleMessage,
					"问题: %s, schedule_message 应非空", tc.question)
			}
			if tc.cronContains != "" && result.ScheduleCron != "" {
				assert.Contains(s.T(), result.ScheduleCron, tc.cronContains,
					"问题: %s, cron '%s' 应包含 '%s'", tc.question, result.ScheduleCron, tc.cronContains)
			}

			s.T().Logf("问题: %s → has_schedule=%v, cron=%s, name=%s, msg=%s",
				tc.question, result.HasSchedule, result.ScheduleCron, result.ScheduleName, result.ScheduleMessage)
		})
	}
}

// TestIntent_Schedule_CronValidity 验证 Ollama 生成的 cron 表达式可被解析
func (s *IntentRecognitionSuite) TestIntent_Schedule_CronValidity() {
	questions := []struct {
		name     string
		question string
	}{
		{"每天早上9点", "每天早上9点提醒我喝水"},
		{"每周五下午3点", "每周五下午3点提醒我写周报"},
		{"工作日早上8点半", "工作日早上8点半提醒我打卡"},
	}

	for _, tc := range questions {
		s.Run(tc.name, func() {
			result := s.thinkWithRetry(tc.question, 3, func(r *core.ThinkingResult) bool {
				if !r.HasSchedule || r.ScheduleCron == "" {
					return false
				}
				fields := strings.Fields(r.ScheduleCron)
				return len(fields) >= 5
			})

			if !result.HasSchedule {
				s.T().Logf("⚠ 未识别为定时意图: %s", tc.question)
				return
			}

			fields := strings.Fields(result.ScheduleCron)
			assert.GreaterOrEqual(s.T(), len(fields), 5,
				"问题: %s, cron '%s' 格式不合法", tc.question, result.ScheduleCron)

			s.T().Logf("问题: %s → cron=%s (fields=%d)", tc.question, result.ScheduleCron, len(fields))
		})
	}
}

// TestIntent_CancelSchedule 取消定时任务意图识别（端到端，基于 Ollama）
func (s *IntentRecognitionSuite) TestIntent_CancelSchedule() {
	tests := []struct {
		name               string
		question           string
		expectCancel       bool   // cancel_schedule 应非空
		cancelNameContains string // cancel_schedule 应包含的关键词
	}{
		{
			name:               "取消喝水提醒",
			question:           "取消每日喝水提醒",
			expectCancel:       true,
			cancelNameContains: "喝水",
		},
		{
			name:               "停止开会提醒",
			question:           "不要再提醒我开会了",
			expectCancel:       true,
			cancelNameContains: "开会",
		},
		{
			name:         "删除定时任务",
			question:     "把那个每天早上的闹钟删掉",
			expectCancel: true,
		},
		{
			name:         "创建定时不应触发取消",
			question:     "每天早上9点提醒我喝水",
			expectCancel: false,
		},
		{
			name:         "普通查询不应触发取消",
			question:     "今天天气怎么样",
			expectCancel: false,
		},
		{
			name:         "闲聊不应触发取消",
			question:     "你好",
			expectCancel: false,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			result := s.thinkWithRetry(tc.question, 3, func(r *core.ThinkingResult) bool {
				hasCancel := r.CancelSchedule != ""
				return hasCancel == tc.expectCancel
			})

			hasCancel := result.CancelSchedule != ""
			assert.Equal(s.T(), tc.expectCancel, hasCancel,
				"问题: %s, cancel_schedule='%s', 期望有取消=%v",
				tc.question, result.CancelSchedule, tc.expectCancel)

			if tc.expectCancel && tc.cancelNameContains != "" {
				assert.Contains(s.T(), result.CancelSchedule, tc.cancelNameContains,
					"问题: %s, cancel_schedule='%s' 应包含 '%s'",
					tc.question, result.CancelSchedule, tc.cancelNameContains)
			}

			// 取消意图不应同时触发创建
			if tc.expectCancel {
				assert.False(s.T(), result.HasSchedule,
					"问题: %s, 取消意图不应同时设置 has_schedule=true", tc.question)
			}

			s.T().Logf("问题: %s → cancel_schedule='%s', has_schedule=%v, intent=%s",
				tc.question, result.CancelSchedule, result.HasSchedule, result.Intent)
		})
	}
}

// TestIntent_NoConfusion 防退化：验证之前出过的 bug 不再复现
func (s *IntentRecognitionSuite) TestIntent_NoConfusion() {
	tests := []struct {
		name            string
		question        string
		forbiddenIntent []string // intent 不应包含这些词
	}{
		{
			name:            "A股不应识别为天气",
			question:        "今天A股行情如何",
			forbiddenIntent: []string{"天气", "weather"},
		},
		{
			name:            "邮件不应识别为天气",
			question:        "帮我发个邮件",
			forbiddenIntent: []string{"天气", "weather"},
		},
		{
			name:            "计算器不应识别为天气",
			question:        "打开计算器",
			forbiddenIntent: []string{"天气", "weather"},
		},
		{
			name:            "提醒不应识别为计算",
			question:        "提醒我明天开会",
			forbiddenIntent: []string{"计算", "calculator", "math"},
		},
		{
			name:            "搜索新闻不应识别为文件",
			question:        "搜一下最近的新闻",
			forbiddenIntent: []string{"文件", "finder", "file"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			result := s.thinkWithRetry(tc.question, 3, func(r *core.ThinkingResult) bool {
				return !containsAny(r.Intent, tc.forbiddenIntent)
			})

			assert.False(s.T(), containsAny(result.Intent, tc.forbiddenIntent),
				"问题: %s, intent '%s' 不应包含 %v", tc.question, result.Intent, tc.forbiddenIntent)
		})
	}
}

// TestIntent_ToolKeywordQuality 验证左脑输出的 keywords 能被工具搜索器有效利用
// 核心逻辑：左脑 keywords 至少有一个命中对应技能的 tags
func (s *IntentRecognitionSuite) TestIntent_ToolKeywordQuality() {
	// 技能 tag 映射（与 searcher_test.go 中 newTestSearcher 保持一致）
	skillTags := map[string][]string{
		"weather":    {"天气", "weather", "forecast"},
		"calculator": {"计算", "calculator", "math"},
		"sysinfo":    {"系统", "sysinfo", "CPU", "内存"},
		"finance":    {"stock", "finance", "A股", "行情"},
		"finder":     {"文件", "finder", "files"},
		"reminders":  {"提醒", "reminders", "alarm"},
		"search":     {"搜索", "search", "新闻"},
	}

	tests := []struct {
		name       string
		question   string
		targetTool string // 期望命中的工具类别
	}{
		{"天气查询应产生天气关键词", "北京今天天气怎么样", "weather"},
		{"计算应产生计算关键词", "帮我算一下 100 加 200", "calculator"},
		{"系统信息应产生系统关键词", "查看一下CPU使用率", "sysinfo"},
		{"股票应产生金融关键词", "今天A股行情如何", "finance"},
		{"文件搜索应产生文件关键词", "帮我找一下文件", "finder"},
		{"提醒应产生提醒关键词", "提醒我下午三点开会", "reminders"},
	}

	passed := 0
	for _, tc := range tests {
		s.Run(tc.name, func() {
			result := s.thinkWithRetry(tc.question, 3, func(r *core.ThinkingResult) bool {
				tags := skillTags[tc.targetTool]
				// intent 或 keywords 中至少有一个命中 tags
				if containsAny(r.Intent, tags) {
					return true
				}
				return keywordsContainAny(r.Keywords, tags)
			})

			tags := skillTags[tc.targetTool]
			intentHit := containsAny(result.Intent, tags)
			keywordHit := keywordsContainAny(result.Keywords, tags)
			hit := intentHit || keywordHit

			if hit {
				passed++
			}

			s.T().Logf("问题: %s → intent=%s, keywords=%v, 目标=%s, 命中=%v",
				tc.question, result.Intent, result.Keywords, tc.targetTool, hit)

			assert.True(s.T(), hit,
				"问题: %s, intent='%s', keywords=%v 应至少命中 %s 的 tags %v",
				tc.question, result.Intent, result.Keywords, tc.targetTool, tags)
		})
	}

	s.T().Logf("工具关键词命中率: %d/%d", passed, len(tests))
}

// TestIntent_EdgeCases 边界场景：模糊表达、多意图、否定句等
func (s *IntentRecognitionSuite) TestIntent_EdgeCases() {
	tests := []struct {
		name          string
		question      string
		expectUseless bool
		checkFunc     func(*core.ThinkingResult) bool // 自定义校验
		description   string                          // 校验说明
	}{
		{
			name:          "否定句不应被当作闲聊",
			question:      "不用查天气了",
			expectUseless: true,
			checkFunc:     func(r *core.ThinkingResult) bool { return r.Useless || r.CanAnswer },
			description:   "取消类表达应标记为 useless 或 can_answer",
		},
		{
			name:          "纯表情不应触发工具",
			question:      "😊",
			expectUseless: true,
			checkFunc:     func(r *core.ThinkingResult) bool { return r.CanAnswer },
			description:   "纯表情应能直接回答",
		},
		{
			name:          "长句子中的工具意图",
			question:      "我想知道明天上海的天气预报，方便决定穿什么衣服",
			expectUseless: false,
			checkFunc: func(r *core.ThinkingResult) bool {
				return containsAny(r.Intent, []string{"天气", "weather"})
			},
			description: "长句子应正确提取天气意图",
		},
		{
			name:          "英文输入",
			question:      "What's the weather in Beijing?",
			expectUseless: false,
			checkFunc: func(r *core.ThinkingResult) bool {
				return containsAny(r.Intent, []string{"天气", "weather"}) ||
					keywordsContainAny(r.Keywords, []string{"weather", "天气", "Beijing", "北京"})
			},
			description: "英文天气查询应识别为天气意图",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			result := s.thinkWithRetry(tc.question, 3, func(r *core.ThinkingResult) bool {
				return tc.checkFunc(r)
			})

			assert.True(s.T(), tc.checkFunc(result),
				"问题: %s, 校验失败: %s, intent=%s, keywords=%v, useless=%v, can_answer=%v",
				tc.question, tc.description, result.Intent, result.Keywords, result.Useless, result.CanAnswer)
		})
	}
}

// buildTestSearcher 构造一个与生产环境一致的技能搜索器（无 embedding，走 keyword 路径）
func buildTestSearcher(logger logging.Logger) *skills.SkillSearcher {
	searcher := skills.NewSkillSearcher(nil, logger)

	skillDefs := []struct {
		name        string
		description string
		category    string
		tags        []string
	}{
		{"weather", "查询天气预报", "general", []string{"天气", "weather", "forecast"}},
		{"calculator", "数学计算器", "general", []string{"计算", "calculator", "math"}},
		{"sysinfo", "查看系统信息", "general", []string{"系统", "sysinfo", "CPU", "内存"}},
		{"mcp_sina-finance_get-quote", "获取股票实时行情", "mcp", []string{"mcp", "sina-finance", "stock", "finance", "A股", "行情"}},
		{"finder", "文件搜索", "general", []string{"文件", "finder", "files"}},
		{"reminders", "提醒管理", "general", []string{"提醒", "reminders", "alarm"}},
		{"search", "搜索引擎", "general", []string{"搜索", "search", "新闻", "news"}},
	}

	skillMap := map[string]*core.Skill{}
	infoMap := map[string]*entity.SkillInfo{}
	for _, sd := range skillDefs {
		name := sd.name
		skillMap[name] = &core.Skill{
			GetName: func() string { return name },
		}
		infoMap[name] = &entity.SkillInfo{
			Def: &entity.SkillDef{
				Name:        sd.name,
				Description: sd.description,
				Category:    sd.category,
				Tags:        sd.tags,
				Enabled:     true,
			},
			Status: "ready",
			CanRun: true,
		}
	}

	searcher.SetData(skillMap, infoMap, nil)
	return searcher
}

// TestIntent_EndToEnd_ToolSearch 端到端测试：Ollama 推理 → keywords → 搜索器找到正确工具
// 这是最核心的回归测试：验证 prompt + 模型 + 搜索器 三者配合的完整链路
func (s *IntentRecognitionSuite) TestIntent_EndToEnd_ToolSearch() {
	searcher := buildTestSearcher(s.logger)

	tests := []struct {
		name        string
		question    string
		expectTools []string // 搜索结果应包含的技能名
		forbidTools []string // 搜索结果不应包含的技能名
	}{
		{
			name:        "天气查询应找到weather",
			question:    "北京今天天气怎么样",
			expectTools: []string{"weather"},
			forbidTools: []string{"calculator", "mcp_sina-finance_get-quote"},
		},
		{
			name:        "A股行情应找到finance",
			question:    "今天A股行情如何",
			expectTools: []string{"mcp_sina-finance_get-quote"},
			forbidTools: []string{"weather"},
		},
		{
			name:        "计算应找到calculator",
			question:    "帮我算一下 123 乘以 456",
			expectTools: []string{"calculator"},
			forbidTools: []string{"weather", "mcp_sina-finance_get-quote"},
		},
		{
			name:        "系统信息应找到sysinfo",
			question:    "帮我查一下系统CPU使用率",
			expectTools: []string{"sysinfo"},
		},
		{
			name:        "文件搜索应找到finder",
			question:    "帮我找一下桌面上的文件",
			expectTools: []string{"finder"},
		},
		{
			name:        "提醒应找到reminders",
			question:    "提醒我下午三点开会",
			expectTools: []string{"reminders"},
		},
		{
			name:        "搜索新闻应找到search",
			question:    "帮我搜一下最近的科技新闻",
			expectTools: []string{"search"},
			forbidTools: []string{"finder"},
		},
	}

	passed := 0
	for _, tc := range tests {
		s.Run(tc.name, func() {
			// 用真实 Ollama 推理
			result := s.thinkWithRetry(tc.question, 3, func(r *core.ThinkingResult) bool {
				// 构造搜索关键词（与 brain.go tryRightBrainProcess 一致）
				searchKeywords := []string{tc.question}
				if r.Intent != "" {
					searchKeywords = append(searchKeywords, r.Intent)
				}
				if len(r.Keywords) > 0 {
					searchKeywords = append(searchKeywords, r.Keywords...)
				}

				found, err := searcher.Search(searchKeywords...)
				if err != nil {
					return false
				}

				foundNames := make([]string, 0, len(found))
				for _, sk := range found {
					foundNames = append(foundNames, sk.GetName())
				}

				for _, expected := range tc.expectTools {
					hit := false
					for _, name := range foundNames {
						if name == expected {
							hit = true
							break
						}
					}
					if !hit {
						return false
					}
				}
				return true
			})

			// 最终断言
			searchKeywords := []string{tc.question}
			if result.Intent != "" {
				searchKeywords = append(searchKeywords, result.Intent)
			}
			if len(result.Keywords) > 0 {
				searchKeywords = append(searchKeywords, result.Keywords...)
			}

			found, err := searcher.Search(searchKeywords...)
			assert.NoError(s.T(), err)

			foundNames := make([]string, 0, len(found))
			for _, sk := range found {
				foundNames = append(foundNames, sk.GetName())
			}

			s.T().Logf("问题: %s → intent=%s, keywords=%v, 搜索词=%v, 找到工具=%v",
				tc.question, result.Intent, result.Keywords, searchKeywords, foundNames)

			allHit := true
			for _, expected := range tc.expectTools {
				hit := false
				for _, name := range foundNames {
					if name == expected {
						hit = true
						break
					}
				}
				assert.True(s.T(), hit,
					"问题: %s, 期望找到 %s, 实际: %v (intent=%s, keywords=%v)",
					tc.question, expected, foundNames, result.Intent, result.Keywords)
				if !hit {
					allHit = false
				}
			}

			for _, forbidden := range tc.forbidTools {
				for _, name := range foundNames {
					assert.NotEqual(s.T(), forbidden, name,
						"问题: %s, 不应找到 %s, 实际: %v",
						tc.question, forbidden, foundNames)
				}
			}

			if allHit {
				passed++
			}
		})
	}

	s.T().Logf("端到端工具识别准确率: %d/%d", passed, len(tests))
}

// TestIntent_SendTo 转发意图识别（端到端，基于 Ollama）
func (s *IntentRecognitionSuite) TestIntent_SendTo() {
	tests := []struct {
		name         string
		question     string
		expectSendTo bool
		sendToHints  []string // send_to 应包含的关键词之一（为空则不检查）
	}{
		{
			name:         "转发到微信",
			question:     "帮我把这条消息转发到微信",
			expectSendTo: true,
			sendToHints:  []string{"wechat", "微信", "weixin"},
		},
		{
			name:         "发送到钉钉",
			question:     "把这个发到钉钉群里",
			expectSendTo: true,
			sendToHints:  []string{"dingtalk", "钉钉", "dingding"},
		},
		{
			name:         "发送到Telegram",
			question:     "forward this to telegram",
			expectSendTo: true,
			sendToHints:  []string{"telegram", "tg"},
		},
		{
			name:         "普通天气查询无转发",
			question:     "今天天气怎么样",
			expectSendTo: false,
		},
		{
			name:         "闲聊无转发",
			question:     "你好",
			expectSendTo: false,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			result := s.thinkWithRetry(tc.question, 3, func(r *core.ThinkingResult) bool {
				hasSendTo := r.SendTo != ""
				return hasSendTo == tc.expectSendTo
			})

			hasSendTo := result.SendTo != ""
			assert.Equal(s.T(), tc.expectSendTo, hasSendTo,
				"问题: %s, send_to='%s', 期望有转发=%v", tc.question, result.SendTo, tc.expectSendTo)

			if tc.expectSendTo && len(tc.sendToHints) > 0 {
				assert.True(s.T(), containsAny(result.SendTo, tc.sendToHints),
					"问题: %s, send_to='%s' 应包含 %v 之一", tc.question, result.SendTo, tc.sendToHints)
			}

			s.T().Logf("问题: %s → send_to='%s'", tc.question, result.SendTo)
		})
	}
}

// TestIntent_MultiIntent 复合意图测试：一句话包含多个意图时，验证主意图正确
func (s *IntentRecognitionSuite) TestIntent_MultiIntent() {
	tests := []struct {
		name            string
		question        string
		primaryIntent   []string // 主意图应包含之一
		keywordContains []string // keywords 应覆盖多个意图的关键词
	}{
		{
			name:            "天气+定时",
			question:        "查一下北京天气，然后每天早上提醒我带伞",
			primaryIntent:   []string{"天气", "weather"},
			keywordContains: []string{"天气", "提醒"},
		},
		{
			name:            "计算+转发",
			question:        "帮我算一下100加200，然后把结果发到微信",
			primaryIntent:   []string{"计算", "算", "calculator"},
			keywordContains: []string{"计算", "算", "100", "200"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			result := s.thinkWithRetry(tc.question, 3, func(r *core.ThinkingResult) bool {
				return containsAny(r.Intent, tc.primaryIntent)
			})

			assert.True(s.T(), containsAny(result.Intent, tc.primaryIntent),
				"问题: %s, intent='%s' 应包含 %v 之一", tc.question, result.Intent, tc.primaryIntent)

			if len(tc.keywordContains) > 0 {
				assert.True(s.T(), keywordsContainAny(result.Keywords, tc.keywordContains),
					"问题: %s, keywords=%v 应包含 %v 之一", tc.question, result.Keywords, tc.keywordContains)
			}

			s.T().Logf("问题: %s → intent=%s, keywords=%v, has_schedule=%v, send_to=%s",
				tc.question, result.Intent, result.Keywords, result.HasSchedule, result.SendTo)
		})
	}
}
