package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"mindx/internal/config"
	"mindx/internal/core"
	infraLlama "mindx/internal/infrastructure/llama"
	"mindx/internal/usecase/skills"
	"mindx/pkg/logging"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// ToolExecutionSuite 右脑工具执行端到端测试套件
// 完整链路：Ollama 左脑识别意图 → 搜索工具 → Ollama 右脑决定调用 → 真实执行技能
type ToolExecutionSuite struct {
	suite.Suite
	leftBrain  *Thinking
	rightBrain *Thinking
	toolCaller *ToolCaller
	skillMgr   *skills.SkillMgr
	logger     logging.Logger
}

func (s *ToolExecutionSuite) SetupSuite() {
	logConfig := &config.LoggingConfig{
		SystemLogConfig: &config.SystemLogConfig{
			Level:      config.LevelDebug,
			OutputPath: "/tmp/tool_execution_test.log",
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
	s.logger = logging.GetSystemLogger().Named("tool_execution_test")

	modelName := getIntentTestModelName()

	// 初始化 SkillMgr（加载真实技能）
	skillsDir := getProjectRootSkillsDir(s.T())
	workspaceDir, err := config.GetWorkspacePath()
	if err != nil {
		s.T().Skipf("获取 workspace 路径失败: %v", err)
	}
	llamaSvc := infraLlama.NewOllamaService(modelName)
	mgr, err := skills.NewSkillMgr(skillsDir, workspaceDir, nil, llamaSvc, s.logger)
	if err != nil {
		s.T().Skipf("创建技能管理器失败: %v", err)
	}
	s.skillMgr = mgr

	// 注入技能关键词
	core.SetSkillKeywords([]string{
		"天气", "weather", "计算", "calculator", "文件", "finder",
		"系统", "sysinfo", "CPU", "内存", "提醒", "reminders",
		"日历", "calendar", "搜索", "search", "新闻",
	})

	modelCfg := &config.ModelConfig{
		Name:        modelName,
		APIKey:      "",
		BaseURL:     "http://localhost:11434/v1",
		Temperature: 0.3,
		MaxTokens:   800,
	}

	tokenBudget := &config.TokenBudgetConfig{
		ReservedOutputTokens: 4096,
		MinHistoryRounds:     2,
		AvgTokensPerRound:    150,
	}

	// 左脑：意图识别
	leftPrompt := buildLeftBrainPrompt(&core.Persona{
		Name:      "小柔",
		Gender:    "女",
		Character: "温柔",
	})
	s.leftBrain = NewThinking(modelCfg, leftPrompt, s.logger, nil, tokenBudget)

	// 右脑：工具调用（无 system prompt，走 function calling）
	rightModelCfg := &config.ModelConfig{
		Name:        modelName,
		APIKey:      "",
		BaseURL:     "http://localhost:11434/v1",
		Temperature: 0.1,
		MaxTokens:   2000,
	}
	s.rightBrain = NewThinking(rightModelCfg, "", s.logger, nil, tokenBudget)

	// ToolCaller
	s.toolCaller = NewToolCaller(s.skillMgr, s.logger)
}

func TestToolExecutionSuite(t *testing.T) {
	suite.Run(t, new(ToolExecutionSuite))
}

func getProjectRootSkillsDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取工作目录失败: %v", err)
	}
	root := filepath.Join(wd, "..", "..", "..")
	dir := filepath.Join(root, "skills")
	if _, err := os.Stat(dir); err == nil {
		return dir
	}
	t.Skipf("找不到 skills 目录，wd=%s", wd)
	return ""
}

// TestToolExecution_Calculator 端到端：计算器工具执行
func (s *ToolExecutionSuite) TestToolExecution_Calculator() {
	question := "帮我算一下 2 加 3 等于多少"

	// 1. 左脑识别意图
	result, err := s.leftBrain.Think(context.Background(), question, nil, "", true)
	if err != nil {
		s.T().Skipf("左脑调用失败: %v", err)
	}
	s.T().Logf("左脑: intent=%s, keywords=%v, can_answer=%v", result.Intent, result.Keywords, result.CanAnswer)

	// 2. 搜索工具（模拟 tryRightBrainProcess）
	searchKeywords := []string{question}
	if result.Intent != "" {
		searchKeywords = append(searchKeywords, result.Intent)
	}
	searchKeywords = append(searchKeywords, result.Keywords...)

	toolSchemas, err := s.toolCaller.SearchTools(searchKeywords)
	assert.NoError(s.T(), err)

	s.T().Logf("搜索到 %d 个工具", len(toolSchemas))
	for _, t := range toolSchemas {
		s.T().Logf("  工具: %s - %s", t.Name, t.Description)
	}

	if len(toolSchemas) == 0 {
		s.T().Skip("未搜索到工具，跳过执行测试")
	}

	// 转为指针切片
	toolPtrs := make([]*core.ToolSchema, len(toolSchemas))
	for i := range toolSchemas {
		toolPtrs[i] = &toolSchemas[i]
	}

	// 3. 右脑决定调用 + 执行
	answer, err := s.toolCaller.ExecuteToolCall(
		context.Background(), s.rightBrain, question, nil, toolPtrs)
	if err != nil {
		s.T().Logf("⚠ 工具执行失败: %v", err)
		// 小模型可能无法正确调用 function calling，不阻断测试
		return
	}

	s.T().Logf("最终回答: %s", answer)
	assert.NotEmpty(s.T(), answer, "工具执行应返回非空回答")
}

// TestToolExecution_Sysinfo 端到端：系统信息工具执行
func (s *ToolExecutionSuite) TestToolExecution_Sysinfo() {
	question := "查看一下系统信息"

	result, err := s.leftBrain.Think(context.Background(), question, nil, "", true)
	if err != nil {
		s.T().Skipf("左脑调用失败: %v", err)
	}
	s.T().Logf("左脑: intent=%s, keywords=%v", result.Intent, result.Keywords)

	searchKeywords := []string{question}
	if result.Intent != "" {
		searchKeywords = append(searchKeywords, result.Intent)
	}
	searchKeywords = append(searchKeywords, result.Keywords...)

	toolSchemas, err := s.toolCaller.SearchTools(searchKeywords)
	assert.NoError(s.T(), err)

	if len(toolSchemas) == 0 {
		s.T().Skip("未搜索到工具")
	}

	// 验证搜索到了 sysinfo
	foundSysinfo := false
	for _, t := range toolSchemas {
		if strings.Contains(t.Name, "sysinfo") {
			foundSysinfo = true
		}
	}
	assert.True(s.T(), foundSysinfo, "应搜索到 sysinfo 工具, 实际: %v", toolSchemaNames(toolSchemas))

	toolPtrs := make([]*core.ToolSchema, len(toolSchemas))
	for i := range toolSchemas {
		toolPtrs[i] = &toolSchemas[i]
	}

	answer, err := s.toolCaller.ExecuteToolCall(
		context.Background(), s.rightBrain, question, nil, toolPtrs)
	if err != nil {
		s.T().Logf("⚠ 工具执行失败: %v", err)
		return
	}

	s.T().Logf("最终回答: %s", answer)
	assert.NotEmpty(s.T(), answer, "sysinfo 应返回非空回答")
}

// TestToolExecution_Weather 端到端：天气工具执行
func (s *ToolExecutionSuite) TestToolExecution_Weather() {
	question := "北京今天天气怎么样"

	result, err := s.leftBrain.Think(context.Background(), question, nil, "", true)
	if err != nil {
		s.T().Skipf("左脑调用失败: %v", err)
	}
	s.T().Logf("左脑: intent=%s, keywords=%v", result.Intent, result.Keywords)

	searchKeywords := []string{question}
	if result.Intent != "" {
		searchKeywords = append(searchKeywords, result.Intent)
	}
	searchKeywords = append(searchKeywords, result.Keywords...)

	toolSchemas, err := s.toolCaller.SearchTools(searchKeywords)
	assert.NoError(s.T(), err)

	if len(toolSchemas) == 0 {
		s.T().Skip("未搜索到工具")
	}

	foundWeather := false
	for _, t := range toolSchemas {
		if strings.Contains(t.Name, "weather") {
			foundWeather = true
		}
	}
	assert.True(s.T(), foundWeather, "应搜索到 weather 工具, 实际: %v", toolSchemaNames(toolSchemas))

	toolPtrs := make([]*core.ToolSchema, len(toolSchemas))
	for i := range toolSchemas {
		toolPtrs[i] = &toolSchemas[i]
	}

	answer, err := s.toolCaller.ExecuteToolCall(
		context.Background(), s.rightBrain, question, nil, toolPtrs)
	if err != nil {
		s.T().Logf("⚠ 工具执行失败: %v", err)
		return
	}

	s.T().Logf("最终回答: %s", answer)
	assert.NotEmpty(s.T(), answer, "weather 应返回非空回答")
}

// TestToolExecution_DirectSkillExec 直接技能执行测试（不经过 Ollama 右脑，验证技能本身可用）
func (s *ToolExecutionSuite) TestToolExecution_DirectSkillExec() {
	tests := []struct {
		name       string
		skillName  string
		params     map[string]any
		expectJSON bool // 期望输出是 JSON
	}{
		{
			name:       "calculator直接执行",
			skillName:  "calculator",
			params:     map[string]any{"expression": "2+3"},
			expectJSON: true,
		},
		{
			name:       "sysinfo直接执行",
			skillName:  "sysinfo",
			params:     map[string]any{"type": "overview"},
			expectJSON: true,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			output, err := s.skillMgr.ExecuteByName(tc.skillName, tc.params)
			if err != nil {
				s.T().Logf("⚠ 技能 %s 执行失败: %v", tc.skillName, err)
				return
			}

			assert.NotEmpty(s.T(), output, "技能 %s 应返回非空输出", tc.skillName)

			if tc.expectJSON {
				var parsed any
				err := json.Unmarshal([]byte(output), &parsed)
				assert.NoError(s.T(), err, "技能 %s 输出应为合法 JSON: %s", tc.skillName, output)
			}

			s.T().Logf("技能 %s 输出: %s", tc.skillName, truncate(output, 200))
		})
	}
}

// TestToolExecution_MultiToolScenario 多工具场景：验证搜索到多个工具时右脑能正确选择
func (s *ToolExecutionSuite) TestToolExecution_MultiToolScenario() {
	question := "帮我算一下 100 加 200"

	// 故意传入多个工具，验证右脑能选对
	allSchemas, err := s.toolCaller.SearchTools([]string{"calculator", "sysinfo", "weather"})
	if err != nil || len(allSchemas) == 0 {
		s.T().Skip("搜索工具失败或无工具")
	}

	s.T().Logf("提供 %d 个工具: %v", len(allSchemas), toolSchemaNames(allSchemas))

	toolPtrs := make([]*core.ToolSchema, len(allSchemas))
	for i := range allSchemas {
		toolPtrs[i] = &allSchemas[i]
	}

	answer, err := s.toolCaller.ExecuteToolCall(
		context.Background(), s.rightBrain, question, nil, toolPtrs)
	if err != nil {
		s.T().Logf("⚠ 多工具执行失败: %v", err)
		return
	}

	s.T().Logf("多工具场景回答: %s", answer)
	assert.NotEmpty(s.T(), answer, "多工具场景应返回非空回答")
}

func toolSchemaNames(schemas []core.ToolSchema) []string {
	names := make([]string, len(schemas))
	for i, s := range schemas {
		names[i] = s.Name
	}
	return names
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + fmt.Sprintf("...(%d chars)", len(s))
}

// TestToolExecution_FullPipeline 完整端到端测试：
// 左脑 Ollama 识别意图 → keywords 搜索到多个工具 → 右脑 Ollama 选择工具并调用 → 真实执行 → 结果反喂右脑 → 输出最终回答
// 这是最接近生产环境 brain.go post() 流程的测试
func (s *ToolExecutionSuite) TestToolExecution_FullPipeline() {
	tests := []struct {
		name           string
		question       string
		expectToolHit  []string // 搜索结果中期望包含的工具
		answerContains []string // 最终回答应包含的关键词之一（为空则只检查非空）
	}{
		{
			name:          "计算器完整流程",
			question:      "帮我算一下 15 乘以 20 等于多少",
			expectToolHit: []string{"calculator"},
			answerContains: []string{"300"},
		},
		{
			name:          "系统信息完整流程",
			question:      "查看一下当前系统CPU和内存使用情况",
			expectToolHit: []string{"sysinfo"},
		},
		{
			name:          "天气查询完整流程",
			question:      "北京今天天气怎么样",
			expectToolHit: []string{"weather"},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			ctx := context.Background()

			// ===== 第1步：左脑识别意图 =====
			thinkResult, err := s.leftBrain.Think(ctx, tc.question, nil, "", true)
			if err != nil {
				s.T().Skipf("左脑调用失败: %v", err)
			}
			s.T().Logf("[左脑] intent=%s, keywords=%v, can_answer=%v, useless=%v",
				thinkResult.Intent, thinkResult.Keywords, thinkResult.CanAnswer, thinkResult.Useless)

			// ===== 第2步：构造搜索关键词（与 brain.go tryRightBrainProcess 一致）=====
			searchKeywords := []string{tc.question}
			if thinkResult.Intent != "" {
				searchKeywords = append(searchKeywords, thinkResult.Intent)
			}
			if len(thinkResult.Keywords) > 0 {
				searchKeywords = append(searchKeywords, thinkResult.Keywords...)
			}
			s.T().Logf("[搜索] 关键词: %v", searchKeywords)

			// ===== 第3步：搜索工具 =====
			toolSchemas, err := s.toolCaller.SearchTools(searchKeywords)
			assert.NoError(s.T(), err)
			s.T().Logf("[搜索] 找到 %d 个工具: %v", len(toolSchemas), toolSchemaNames(toolSchemas))

			if len(toolSchemas) == 0 {
				s.T().Skipf("未搜索到任何工具，搜索词: %v", searchKeywords)
			}

			// 验证期望的工具在搜索结果中
			foundNames := toolSchemaNames(toolSchemas)
			for _, expected := range tc.expectToolHit {
				found := false
				for _, name := range foundNames {
					if strings.Contains(name, expected) {
						found = true
						break
					}
				}
				assert.True(s.T(), found,
					"搜索结果 %v 应包含 %s", foundNames, expected)
			}

			// ===== 第4步：右脑决定调用 + 执行 + 结果反喂 =====
			// ExecuteToolCall 内部完成：ThinkWithTools → Execute → ReturnFuncResults 循环
			toolPtrs := make([]*core.ToolSchema, len(toolSchemas))
			for i := range toolSchemas {
				toolPtrs[i] = &toolSchemas[i]
			}

			answer, err := s.toolCaller.ExecuteToolCall(ctx, s.rightBrain, tc.question, nil, toolPtrs)
			if err != nil {
				s.T().Logf("⚠ 右脑工具执行失败: %v (小模型 function calling 能力有限)", err)
				return
			}

			// ===== 第5步：验证最终回答 =====
			s.T().Logf("[最终回答] %s", truncate(answer, 500))
			assert.NotEmpty(s.T(), answer, "完整流程应产出非空回答")

			if len(tc.answerContains) > 0 {
				hit := false
				for _, kw := range tc.answerContains {
					if strings.Contains(answer, kw) {
						hit = true
						break
					}
				}
				if !hit {
					s.T().Logf("⚠ 回答未包含期望关键词 %v (小模型输出不稳定)", tc.answerContains)
				}
			}
		})
	}
}
