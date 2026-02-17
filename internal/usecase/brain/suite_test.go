package brain

import (
	"fmt"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/infrastructure/llama"
	"mindx/internal/infrastructure/persistence"
	"mindx/internal/usecase/memory"
	"mindx/internal/usecase/session"
	"mindx/internal/usecase/skills"
	"mindx/pkg/logging"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// BrainIntegrationSuite Brain 集成测试套件
// 使用真实组件（Memory、SkillMgr），不使用 Mock
// 注意：这些测试必须串行执行，因为 Ollama 无法并行处理大量请求
type BrainIntegrationSuite struct {
	suite.Suite
	brain       *core.Brain
	bionicBrain *BionicBrain
	memory      *memory.Memory
	skillMgr    *skills.SkillMgr
	sessionMgr  *session.SessionMgr
	srvCfg      *config.GlobalConfig
	testData    string
	testLogs    string
	logger      logging.Logger
}

// SetupSuite 初始化测试套件
func (s *BrainIntegrationSuite) SetupSuite() {
	s.logger = logging.GetSystemLogger().Named("brain_integration_test")
	s.testData = filepath.Join(os.TempDir(), fmt.Sprintf("bot_brain_test_%d_%d", time.Now().Unix(), os.Getpid()))
	s.testLogs = filepath.Join(s.testData, "logs")

	// 创建测试目录
	err := os.MkdirAll(s.testData, 0755)
	s.Require().NoError(err)
	err = os.MkdirAll(s.testLogs, 0755)
	s.Require().NoError(err)

	s.logger.Info("测试数据目录", logging.String("path", s.testData))

	// 加载配置
	if err := config.EnsureWorkspace(); err != nil {
		s.Require().NoError(err)
	}
	srvCfg, _, _ := config.InitVippers()
	s.srvCfg = srvCfg

	// 初始化会话管理器
	sessionStorage := session.NewFileSessionStorage(filepath.Join(s.testData, "sessions"))
	s.sessionMgr = session.NewSessionMgr(srvCfg.Brain.LeftbrainModel.MaxTokens, sessionStorage, s.logger)
	err = s.sessionMgr.RestoreSession()
	if err != nil {
		s.logger.Warn("恢复会话失败", logging.Err(err))
	}

	// 初始化记忆系统（需要先创建 store）
	store, err := persistence.NewStore(srvCfg.VectorStore.Type, filepath.Join(s.testData, "memory"), nil)
	s.Require().NoError(err)

	s.srvCfg.VectorStore.DataPath = filepath.Join(s.testData, "memory")
	s.memory, err = memory.NewMemory(s.srvCfg, nil, s.logger, store, nil)
	s.Require().NoError(err)
	s.logger.Info("Memory 初始化成功")

	// 初始化 llama 服务
	ollamaSvc := llama.NewOllamaService(s.srvCfg.Brain.LeftbrainModel.Name)
	if s.srvCfg.OllamaURL != "" {
		// OllamaURL 是 OpenAI 兼容接口地址，需要去掉 /v1 后缀用于原生 API
		baseURL := s.srvCfg.OllamaURL
		if len(baseURL) > 3 && baseURL[len(baseURL)-3:] == "/v1" {
			baseURL = baseURL[:len(baseURL)-3]
		}
		ollamaSvc = ollamaSvc.WithBaseUrl(baseURL)
	}

	// 初始化技能管理器
	installSkillsPath, err := config.GetInstallSkillsPath()
	s.Require().NoError(err)
	workspacePath, err := config.GetWorkspacePath()
	s.Require().NoError(err)
	s.skillMgr, err = skills.NewSkillMgr(installSkillsPath, workspacePath, nil, ollamaSvc, s.logger)
	s.Require().NoError(err)
	s.logger.Info("SkillMgr 初始化成功")

	// 初始化 Token 使用记录仓库
	tokenUsageRepo, err := persistence.NewSQLiteTokenUsageRepository(filepath.Join(s.testData, "token_usage.db"))
	s.Require().NoError(err)

	// 创建 BionicBrain（内部实现）
	persona := &core.Persona{Name: "小柔", Gender: "女", Character: "温柔"}

	historyRequest := func(maxCount int) ([]*core.DialogueMessage, error) {
		messages := s.sessionMgr.GetHistory()

		// 转换为 []*core.DialogueMessage
		dialogueMessages := make([]*core.DialogueMessage, len(messages))
		for i, msg := range messages {
			dialogueMessages[i] = &core.DialogueMessage{
				Role:    msg.Role,
				Content: msg.Content,
			}
		}

		// 限制数量
		if maxCount > 0 && len(dialogueMessages) > maxCount {
			dialogueMessages = dialogueMessages[len(dialogueMessages)-maxCount:]
		}

		return dialogueMessages, nil
	}

	toolsRequest := func(keywords ...string) ([]*core.ToolSchema, error) {
		skillList, err := s.skillMgr.SearchSkills(keywords...)
		if err != nil {
			return nil, err
		}
		tools := make([]*core.ToolSchema, 0, len(skillList))
		for _, skill := range skillList {
			name := skill.GetName()
			info, exists := s.skillMgr.GetSkillInfo(name)
			if !exists {
				s.logger.Warn("技能信息不存在", logging.String("skill", name))
				tools = append(tools, &core.ToolSchema{
					Name:        name,
					Description: "",
					Params:      make(map[string]interface{}),
				})
				continue
			}

			params := make(map[string]interface{})
			if info.Def != nil && info.Def.Parameters != nil {
				for paramName, paramDef := range info.Def.Parameters {
					params[paramName] = map[string]interface{}{
						"type":        paramDef.Type,
						"description": paramDef.Description,
						"required":    paramDef.Required,
					}
				}
			}

			tools = append(tools, &core.ToolSchema{
				Name:        info.Def.Name,
				Description: info.Def.Description,
				Params:      params,
			})
		}
		return tools, nil
	}

	capRequest := func(keywords ...string) (*entity.Capability, error) {
		return nil, fmt.Errorf("capability not found")
	}

	s.bionicBrain = &BionicBrain{
		leftBrain:      nil,
		rightBrain:     nil,
		memory:         s.memory,
		persona:        persona,
		logger:         s.logger,
		historyRequest: historyRequest,
		toolsRequest:   toolsRequest,
		capRequest:     capRequest,
		tokenUsageRepo: tokenUsageRepo,
	}

	// 预先记录一些测试记忆
	s.recordTestMemories()
}

// TearDownSuite 清理测试套件
func (s *BrainIntegrationSuite) TearDownSuite() {
	if s.memory != nil {
		s.memory.Close()
	}
	os.RemoveAll(s.testData)
	s.logger.Info("清理测试数据完成", logging.String("path", s.testData))
}

// recordTestMemories 记录测试记忆
func (s *BrainIntegrationSuite) recordTestMemories() {
	// 记忆 1: 用户喜欢编程
	mem1 := core.MemoryPoint{
		Keywords:       []string{"编程", "代码", "开发"},
		Content:        "用户是一名程序员，喜欢使用 Go 语言进行开发",
		Summary:        "用户喜欢编程",
		TimeWeight:     1.0,
		RepeatWeight:   1.0,
		EmphasisWeight: 0.2,
		TotalWeight:    1.0,
		CreatedAt:      time.Now().Add(-24 * time.Hour),
	}
	err := s.memory.Record(mem1)
	s.Require().NoError(err)

	s.logger.Info("记录测试记忆成功")
}

// createTestBrain 创建测试用的 Brain（使用真实的配置）
func (s *BrainIntegrationSuite) createTestBrain() *core.Brain {
	// 注意：不需要手动创建会话，第一次调用 RecordMessage 会自动创建

	// 从配置文件读取（与 bootstrap/assistant.go 一致）
	leftBrainModel := &config.ModelConfig{
		Name:        s.srvCfg.Brain.LeftbrainModel.Name,
		BaseURL:     s.srvCfg.Brain.LeftbrainModel.BaseURL,
		APIKey:      s.srvCfg.Brain.LeftbrainModel.APIKey,
		Domain:      s.srvCfg.Brain.LeftbrainModel.Domain,
		Temperature: s.srvCfg.Brain.LeftbrainModel.Temperature,
		MaxTokens:   s.srvCfg.Brain.LeftbrainModel.MaxTokens,
	}
	rightBrainModel := &config.ModelConfig{
		Name:        s.srvCfg.Brain.RightbrainModel.Name,
		BaseURL:     s.srvCfg.Brain.RightbrainModel.BaseURL,
		APIKey:      s.srvCfg.Brain.RightbrainModel.APIKey,
		Domain:      s.srvCfg.Brain.RightbrainModel.Domain,
		Temperature: s.srvCfg.Brain.RightbrainModel.Temperature,
		MaxTokens:   s.srvCfg.Brain.RightbrainModel.MaxTokens,
	}

	s.logger.Info("加载配置完成",
		logging.String("leftbrain_name", leftBrainModel.Name),
		logging.String("leftbrain_base_url", leftBrainModel.BaseURL),
		logging.String("rightbrain_name", rightBrainModel.Name),
		logging.String("rightbrain_base_url", rightBrainModel.BaseURL))

	// 构建左脑的 systemPrompt（包含人设信息）
	leftBrainPrompt := buildLeftBrainPrompt(s.bionicBrain.persona)

	// 创建 Token 预算配置（测试用）
	tokenBudget := &config.TokenBudgetConfig{
		ReservedOutputTokens: 4096,
		MinHistoryRounds:     2,
		AvgTokensPerRound:    150,
	}

	leftBrain := NewThinking(leftBrainModel, leftBrainPrompt, s.logger, s.bionicBrain.tokenUsageRepo, tokenBudget)

	rightBrain := NewThinking(rightBrainModel, "", s.logger, s.bionicBrain.tokenUsageRepo, tokenBudget)

	s.bionicBrain.leftBrain = leftBrain
	s.bionicBrain.rightBrain = rightBrain
	s.bionicBrain.contextPreparer = NewContextPreparer(s.memory, s.bionicBrain.historyRequest, s.logger)
	s.bionicBrain.toolCaller = NewToolCaller(s.skillMgr, s.logger)
	s.bionicBrain.consciousnessMgr = NewConsciousnessManager(s.srvCfg, s.bionicBrain.persona, s.bionicBrain.tokenUsageRepo, s.logger)
	s.bionicBrain.responseBuilder = NewResponseBuilder()
	s.bionicBrain.fallbackHandler = NewFallbackHandler(rightBrain, s.bionicBrain.toolCaller, s.bionicBrain.responseBuilder, s.logger)

	// 创建符合 core.Brain 接口的结构体
	brain := &core.Brain{
		LeftBrain:     leftBrain,
		RightBrain:    rightBrain,
		Consciousness: nil,
		GetMemory: func() (core.Memory, error) {
			return s.memory, nil
		},
		Post: s.bionicBrain.post,
	}

	// 设置记忆提取器（参考 bootstrap/app.go）
	if leftBrain != nil {
		_ = memory.NewLLMExtractor(leftBrain, s.memory)
		// 设置会话结束回调（这里不能直接设置，因为 sessionMgr 在测试套件外部）
		// 但可以在具体的测试中设置
		s.logger.Info("记忆提取器创建完成")
	}

	return brain
}

// postWithHistory 包装 Post 方法，自动记录对话到会话
func (s *BrainIntegrationSuite) postWithHistory(req *core.ThinkingRequest) (*core.ThinkingResponse, error) {
	// 记录用户消息
	err := s.sessionMgr.RecordMessage(entity.Message{
		Role:    "user",
		Content: req.Question,
		Time:    time.Now(),
	})
	if err != nil {
		s.logger.Warn("记录用户消息失败", logging.Err(err))
	}

	// 调用 Brain 的 Post 方法（内部会通过 historyRequest 获取历史）
	resp, err := s.brain.Post(req)
	if err != nil {
		return nil, err
	}

	// 记录助手回复
	err = s.sessionMgr.RecordMessage(entity.Message{
		Role:    "assistant",
		Content: resp.Answer,
		Time:    time.Now(),
	})
	if err != nil {
		s.logger.Warn("记录助手消息失败", logging.Err(err))
	}

	return resp, nil
}

// TestBrainIntegrationSuite 运行集成测试套件
func TestBrainIntegrationSuite(t *testing.T) {
	suite.Run(t, new(BrainIntegrationSuite))
}

// TestContextConsistencySuite 运行上下文一致性测试
func TestContextConsistencySuite(t *testing.T) {
	suite.Run(t, new(ContextConsistencySuite))
}

// TestMemoryReferenceSuite 运行记忆参考测试
func TestMemoryReferenceSuite(t *testing.T) {
	suite.Run(t, new(MemoryReferenceSuite))
}

// TestSkillExecutionSuite 运行技能执行测试
func TestSkillExecutionSuite(t *testing.T) {
	suite.Run(t, new(SkillExecutionSuite))
}

// TestLongInputSuite 运行超长文测试
func TestLongInputSuite(t *testing.T) {
	suite.Run(t, new(LongInputSuite))
}
