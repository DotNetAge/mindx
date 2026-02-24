package bootstrap

import (
	"context"
	"fmt"
	"time"
	"mindx/internal/adapters/channels"
	"mindx/internal/adapters/http/handlers"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	infra_cron "mindx/internal/infrastructure/cron"
	infraEmbedding "mindx/internal/infrastructure/embedding"
	infraLlama "mindx/internal/infrastructure/llama"
	"mindx/internal/infrastructure/persistence"
	"mindx/internal/usecase/auth"
	"mindx/internal/usecase/capability"
	"mindx/internal/usecase/cron"
	"mindx/internal/usecase/embedding"
	"mindx/internal/usecase/memory"
	"mindx/internal/usecase/session"
	"mindx/internal/usecase/skills"
	"mindx/internal/usecase/skills/builtins"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"os"
	"path/filepath"
	"runtime"

	"github.com/joho/godotenv"
	"github.com/sashabaranov/go-openai"
)

type Channel = core.Channel

type bootstrapSkillInfoProvider struct {
	skillMgr *skills.SkillMgr
}

func (p *bootstrapSkillInfoProvider) GetSkillInfo(name string) (isInternal bool, command string, dir string, err error) {
	info, exists := p.skillMgr.GetSkillInfo(name)
	if !exists {
		return false, "", "", fmt.Errorf("skill not found: %s", name)
	}

	if info.Def != nil {
		isInternal = info.Def.IsInternal
		command = info.Def.Command
	}
	dir = info.Directory
	return isInternal, command, dir, nil
}

type App struct {
	Server         *Server
	Assistant      *Assistant
	ChannelRouter  *channels.Gateway
	SessionMgr     core.SessionMgr
	Embedding      *embedding.EmbeddingService
	Skills         *skills.SkillMgr
	Capabilities   *capability.CapabilityManager
	CronScheduler  cron.Scheduler
	TokenUsageRepo core.TokenUsageRepository
	AuthService    core.AuthenticationService
}

var a *App

func Startup() (*App, error) {
	if err := godotenv.Load(); err != nil {
		fmt.Printf("加载.env文件失败(非必选,可忽略):%v\n", err)
	}

	if err := config.EnsureWorkspace(); err != nil {
		return nil, fmt.Errorf("初始化工作区失败: %w", err)
	}

	workspace, err := config.GetWorkspacePath()
	if err != nil {
		return nil, err
	}

	installPath, err := config.GetInstallPath()
	if err != nil {
		return nil, err
	}

	srvCfg, channelsCfg, capabilitiesCfg, _ := config.InitVippers()

	ctx := context.Background()

	logConfigPath, err := config.GetSystemLogPath()
	if err != nil {
		return nil, err
	}
	logConfig := &config.LoggingConfig{
		SystemLogConfig: &config.SystemLogConfig{
			Level:      config.LevelInfo,
			OutputPath: logConfigPath,
			MaxSize:    100,
			MaxBackups: 10,
			MaxAge:     30,
			Compress:   true,
		},
		ConversationLogConfig: nil,
	}
	if err := logging.Init(logConfig); err != nil {
		return nil, fmt.Errorf("初始化日志系统失败: %w", err)
	}

	systemLogger := logging.GetSystemLogger().Named("app")

	systemLogger.Info("Assistant 启动中...",
		logging.String("version", srvCfg.Version),
	)

	modelsMgr := config.GetModelsManager()

	systemLogger.Info("初始化向量化服务")
	embeddingModel := modelsMgr.GetEmbeddingModel()
	if embeddingModel == "" {
		embeddingModel = "qllama/bge-small-zh-v1.5:latest"
	}
	ollamaProvider, err := infraEmbedding.NewOllamaEmbedding("http://localhost:11434", embeddingModel)
	if err != nil {
		return nil, fmt.Errorf("构建向量化提供器失败: %w", err)
	}
	embeddingSvc := embedding.NewEmbeddingService(ollamaProvider)
	systemLogger.Info("向量化服务初始化完成", logging.String("provider", "ollama"), logging.String("model", embeddingModel))

	vectorsPath, err := config.GetWorkspaceVectorsPath()
	if err != nil {
		return nil, err
	}
	store, err := persistence.NewStore("badger", vectorsPath, nil)
	if err != nil {
		return nil, fmt.Errorf("创建向量存储失败: %w", err)
	}
	systemLogger.Info("向量存储初始化完成", logging.String("type", "badger"))

	systemLogger.Info("初始化会话管理器")
	sessionsPath, err := config.GetWorkspaceSessionsPath()
	if err != nil {
		return nil, err
	}
	sessionStorage := session.NewFileSessionStorage(sessionsPath)

	brainModels := modelsMgr.GetBrainModels()
	defaultModelName := modelsMgr.GetDefaultModel()
	if defaultModelName == "" {
		defaultModelName = brainModels.SubconsciousModel
	}
	defaultModel, err := modelsMgr.GetModel(defaultModelName)
	if err != nil {
		return nil, fmt.Errorf("获取默认模型失败: %w", err)
	}
	maxTokens := defaultModel.MaxTokens
	if maxTokens == 0 {
		maxTokens = 40960
	}

	sessionMgr := session.NewSessionMgr(
		maxTokens,
		sessionStorage,
		systemLogger,
	)
	if err := sessionMgr.RestoreSession(); err != nil {
		systemLogger.Warn("恢复会话失败", logging.Err(err))
	}
	systemLogger.Info("会话管理器初始化完成", logging.String("path", sessionsPath), logging.Int("max_tokens", maxTokens))

	systemLogger.Info("初始化记忆系统")

	memModelName := brainModels.MemoryModel
	if memModelName == "" {
		memModelName = defaultModelName
	}
	memModel, err := modelsMgr.GetModel(memModelName)
	if err != nil {
		return nil, fmt.Errorf("获取记忆模型失败: %w", err)
	}
	openaiCfg := openai.DefaultConfig(memModel.APIKey)
	openaiCfg.BaseURL = memModel.BaseURL
	memLLMClient := openai.NewClientWithConfig(openaiCfg)

	mem, err := memory.NewMemory(srvCfg, memLLMClient, systemLogger, store, embeddingSvc)
	if err != nil {
		return nil, fmt.Errorf("初始化记忆系统失败: %w", err)
	}
	systemLogger.Info("记忆系统初始化完成", logging.String("type", srvCfg.VectorStore.Type), logging.String("model", memModelName))

	systemLogger.Info("初始化 Token 使用记录仓库")
	tokenUsageDBPath, err := config.GetTokenUsageDBPath()
	if err != nil {
		return nil, err
	}
	tokenUsageRepo, err := persistence.NewSQLiteTokenUsageRepository(tokenUsageDBPath)
	if err != nil {
		return nil, fmt.Errorf("初始化 Token 使用记录仓库失败: %w", err)
	}
	systemLogger.Info("Token 使用记录仓库初始化完成")

	systemLogger.Info("初始化能力管理器")
	capMgr, err := capability.NewManager(capabilitiesCfg, store, embeddingSvc, workspace)
	if err != nil {
		return nil, fmt.Errorf("初始化能力管理器失败: %w", err)
	}

	if store != nil {
		if err := capMgr.LoadVectorsFromStore(); err != nil {
			systemLogger.Warn("加载能力向量失败", logging.Err(err))
		}
	}

	if embeddingSvc != nil && len(capMgr.ListCapabilities()) > 0 && !capMgr.HasVectors() {
		systemLogger.Info("开始建立能力索引（后台运行）")
		capMgr.StartReIndexInBackground()
	}

	systemLogger.Info("能力管理器初始化完成", logging.Int("capabilities_count", len(capMgr.ListCapabilities())))

	// Initialize authentication service
	systemLogger.Info("初始化认证服务")
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		// Generate a temporary secret for development
		jwtSecret = "development-secret-key-please-change-in-production-32chars"
		systemLogger.Warn("使用开发模式JWT密钥，生产环境请设置JWT_SECRET环境变量")
	}

	authConfig := auth.Config{
		JWTSecret:        jwtSecret,
		TokenExpiration:  24 * time.Hour,
		RefreshExpiration: 7 * 24 * time.Hour,
		PasswordPolicy: auth.PasswordPolicy{
			MinLength:       8,
			RequireUppercase: false,
			RequireLowercase: false,
			RequireNumbers:   false,
			RequireSpecial:   false,
		},
	}

	authService, err := auth.NewService(authConfig, systemLogger)
	if err != nil {
		return nil, fmt.Errorf("初始化认证服务失败: %w", err)
	}
	systemLogger.Info("认证服务初始化完成")

	systemLogger.Info("初始化技能管理器")

	indexModelName := brainModels.IndexModel
	if indexModelName == "" {
		indexModelName = defaultModelName
	}
	indexModel, err := modelsMgr.GetModel(indexModelName)
	if err != nil {
		return nil, fmt.Errorf("获取索引模型失败: %w", err)
	}
	llamaSvc := infraLlama.NewOllamaService(indexModelName)
	if indexModel.BaseURL != "" {
		baseURL := indexModel.BaseURL
		if len(baseURL) > 3 && baseURL[len(baseURL)-3:] == "/v1" {
			baseURL = baseURL[:len(baseURL)-3]
		}
		llamaSvc = llamaSvc.WithBaseUrl(baseURL)
	}

	installSkillsPath, err := config.GetInstallSkillsPath()
	if err != nil {
		return nil, err
	}

	skillMgr, err := skills.NewSkillMgrWithStore(installSkillsPath, workspace, embeddingSvc, llamaSvc, store, systemLogger)
	if err != nil {
		return nil, fmt.Errorf("初始化技能管理器失败: %w", err)
	}

	var cronScheduler cron.Scheduler
	switch runtime.GOOS {
	case "linux", "darwin":
		cronScheduler, err = infra_cron.NewCrontabScheduler()
	case "windows":
		cronScheduler, err = infra_cron.NewWindowsTaskScheduler()
	default:
		systemLogger.Warn("Unsupported platform for cron scheduler")
	}
	if err != nil {
		systemLogger.Warn("Failed to init cron scheduler", logging.Err(err))
		cronScheduler = nil
	}

	currentLang := i18n.GetLanguage()
	langName := "Chinese"
	if currentLang == "en-US" {
		langName = "English"
	}

	defaultCapName := capabilitiesCfg.DefaultCapability
	defaultCap, _ := capMgr.GetCapability(defaultCapName)

	var builtinCfg *builtins.BuiltinConfig
	if defaultCap != nil {
		capModel, err := modelsMgr.GetModel(defaultCap.Model)
		if err == nil {
			builtinCfg = &builtins.BuiltinConfig{
				BaseURL:  capModel.BaseURL,
				Model:    defaultCap.Model,
				APIKey:   capModel.APIKey,
				LangName: langName,
			}
		}
	}
	if builtinCfg == nil {
		builtinCfg = &builtins.BuiltinConfig{
			BaseURL:  defaultModel.BaseURL,
			Model:    defaultModelName,
			APIKey:   defaultModel.APIKey,
			LangName: langName,
		}
	}

	builtins.RegisterBuiltins(skillMgr, builtinCfg, cronScheduler)

	if embeddingSvc != nil && llamaSvc != nil && skillMgr.IsVectorTableEmpty() {
		systemLogger.Info("开始建立技能索引（后台运行）")
		skillMgr.StartReIndexInBackground()
	}

	systemLogger.Info("技能管理器初始化完成", logging.Int("skills_count", len(skillMgr.GetSkillInfos())))

	var memoryExtractor *memory.LLMExtractor

	systemLogger.Info("初始化 Assistant")
	assistant := NewAssistant(
		srvCfg,
		capMgr,
		sessionMgr,
		skillMgr,
		mem,
		systemLogger,
		tokenUsageRepo,
		cronScheduler,
	)
	systemLogger.Info("Assistant 初始化完成",
		logging.String("name", assistant.GetName()),
		logging.String("gender", assistant.GetGender()),
		logging.String("character", assistant.GetCharacter()),
	)

	leftBrain := assistant.GetBrain().LeftBrain
	if leftBrain != nil {
		memoryExtractor = memory.NewLLMExtractor(leftBrain, mem)
		systemLogger.Info("记忆提取器创建完成")
	} else {
		systemLogger.Warn("Brain.LeftBrain为nil，记忆提取器未创建")
	}

	if memoryExtractor != nil {
		session.SetOnSessionEnd(sessionMgr, func(sess entity.Session) bool {
			return memoryExtractor.Extract(sess)
		})
		systemLogger.Info("会话结束回调设置完成")
	}

	systemLogger.Info("初始化消息网关")
	channelRouter := channels.NewGateway("realtime", embeddingSvc)

	realtimeChannel := channels.NewRealTimeChannel(srvCfg.WsPort)

	assistant.SetOnThinkingEvent(func(sessionID string, event map[string]any) {
		if err := realtimeChannel.SendThinkingEvent(sessionID, event); err != nil {
			systemLogger.Debug("发送思考事件失败",
				logging.String("session_id", sessionID),
				logging.Err(err),
			)
		}
	})

	channelRouter.SetOnMessage(func(ctx context.Context, msg *entity.IncomingMessage, eventChan chan<- entity.ThinkingEvent) (string, string, error) {
		answer, sendTo, err := assistant.Ask(msg.Content, msg.SessionID, eventChan)
		if err != nil {
			systemLogger.Error("处理消息失败",
				logging.String("session_id", msg.SessionID),
				logging.Err(err),
			)
			return "", "", err
		}

		return answer, sendTo, nil
	})

	manager := channelRouter.Manager()

	_ = manager.CreateAndStartChannel(realtimeChannel, channelRouter.HandleMessage, ctx)

	if err := manager.CreateChannelsFromConfig(channelsCfg, channelRouter.HandleMessage, ctx); err != nil {
		systemLogger.Error("创建 Channels 失败", logging.Err(err))
	}

	systemLogger.Info("创建 HTTP API 服务器")
	staticDir := filepath.Join(installPath, "static")

	// 开发模式：如果 installPath 下没有 static，尝试从源代码目录找
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		devStaticDirs := []string{
			filepath.Join(installPath, "dashboard", "dist"),
			filepath.Join(installPath, "dashboard", "build"),
			filepath.Join(installPath, "static"),
		}
		for _, dir := range devStaticDirs {
			if _, err := os.Stat(dir); err == nil {
				staticDir = dir
				systemLogger.Info("开发模式：使用静态文件目录", logging.String("path", staticDir))
				break
			}
		}
	}

	srv, err := NewServer(srvCfg.Port, staticDir)
	if err != nil {
		return nil, fmt.Errorf("创建服务器失败: %w", err)
	}
	systemLogger.Info("HTTP API 服务器创建完成", logging.Int("port", srvCfg.Port))

	handlers.RegisterRoutes(srv.GetEngine(), tokenUsageRepo, skillMgr, capMgr, sessionMgr, cronScheduler, assistant, authService, systemLogger)

	a = &App{
		Server:         srv,
		Assistant:      assistant,
		ChannelRouter:  channelRouter,
		SessionMgr:     sessionMgr,
		Embedding:      embeddingSvc,
		Skills:         skillMgr,
		Capabilities:   capMgr,
		CronScheduler:  cronScheduler,
		TokenUsageRepo: tokenUsageRepo,
		AuthService:    authService,
	}

	if err := srv.Start(); err != nil {
		return nil, fmt.Errorf("启动 Web 服务器失败: %w", err)
	}

	systemLogger.Info("========================================")
	systemLogger.Info("Assistant 已启动!")
	systemLogger.Info("========================================",
		logging.String("web_server", fmt.Sprintf("http://localhost:%d", srvCfg.Port)),
		logging.Int("channel_count", channelRouter.Manager().Count()),
	)
	systemLogger.Info("========================================")

	return a, nil
}

func GetApp() *App {
	return a
}

func Shutdown() error {
	if a == nil {
		return nil
	}

	logger := logging.GetSystemLogger().Named("app")
	logger.Info(i18n.T("infra.shutting_down"))

	if a.ChannelRouter != nil {
		logger.Info(i18n.T("infra.stop_channels"))
		_ = a.ChannelRouter.Manager().StopAll()
	}

	if a.Server != nil {
		logger.Info(i18n.T("infra.close_web_server"))
		a.Server.GracefulShutdown()
	}

	logger.Info(i18n.T("infra.close_session_mgr"))

	if a.TokenUsageRepo != nil {
		logger.Info(i18n.T("infra.close_token_repo"))
		_ = a.TokenUsageRepo.Close()
	}

	logger.Info(i18n.T("infra.shutdown_complete"))
	return nil
}
