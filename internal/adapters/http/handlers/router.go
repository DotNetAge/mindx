package handlers

import (
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/usecase/capability"
	"mindx/internal/usecase/cron"
	"mindx/internal/usecase/session"
	"mindx/internal/usecase/skills"
	httpMiddleware "mindx/internal/adapters/http/middleware"
	"mindx/pkg/logging"

	"github.com/gin-gonic/gin"
)

type Assistant interface {
	Ask(question string, sessionID string, eventChan chan<- entity.ThinkingEvent) (string, string, error)
}

// RegisterRoutes 注册所有路由
func RegisterRoutes(
	router *gin.Engine,
	tokenUsageRepo core.TokenUsageRepository,
	skillMgr *skills.SkillMgr,
	capMgr *capability.CapabilityManager,
	sessionMgr *session.SessionMgr,
	cronScheduler cron.Scheduler,
	assistant Assistant,
	authService core.AuthenticationService,
	logger logging.Logger,
) {
	// Apply CORS middleware
	router.Use(httpMiddleware.CORS())

	// Public routes (no authentication required)
	public := router.Group("/api")
	{
		// Health check
		ctrl := NewControlHandler()
		public.GET("/health", ctrl.handleHealth)

		// Authentication
		authHandler := NewAuthHandler(authService, logger)
		public.POST("/auth/login", authHandler.Login)
	}

	// Protected routes (authentication required)
	api := router.Group("/api")
	api.Use(httpMiddleware.AuthMiddleware(authService, logger))
	{
		// 服务控制
		service := NewServiceHandler()
		api.POST("/service/start", service.Start)
		api.POST("/service/stop", service.Stop)
		api.GET("/service/ollama-check", service.OllamaCheck)
		api.POST("/service/ollama-install", service.OllamaInstall)
		api.POST("/service/model-test", service.ModelTest)

		// 会话管理
		conversations := NewConversationsHandler(sessionMgr, assistant)
		conversationsGroup := api.Group("/conversations")
		{
			conversationsGroup.GET("", conversations.listConversations)
			conversationsGroup.POST("", conversations.createNewConversation)
			conversationsGroup.GET("/current", conversations.getCurrentSession)
			conversationsGroup.POST("/current/message", conversations.sendMessage)
			conversationsGroup.GET("/:id", conversations.getConversation)
			conversationsGroup.POST("/:id/switch", conversations.switchConversation)
			conversationsGroup.DELETE("/:id", conversations.deleteConversation)
		}

		// 渠道管理
		channels := NewChannelsHandler()
		channelsGroup := api.Group("/channels")
		{
			channelsGroup.GET("", channels.getChannels)
			channelsGroup.PUT("/:id", channels.updateChannelConfig)
			channelsGroup.POST("/:id/config", channels.updateChannelConfig)
			channelsGroup.POST("/:id/toggle", channels.toggleChannel)
			channelsGroup.POST("/:id/start", channels.startChannel)
			channelsGroup.POST("/:id/stop", channels.stopChannel)
		}

		// 技能管理
		skillsHandler := NewSkillsHandler(skillMgr)
		skillsGroup := api.Group("/skills")
		{
			skillsGroup.GET("", skillsHandler.listSkills)
			skillsGroup.GET("/reindex/status", skillsHandler.getReIndexStatus)
			skillsGroup.POST("/reindex", skillsHandler.triggerReIndex)
			skillsGroup.GET("/:name", skillsHandler.getSkill)
			skillsGroup.GET("/:name/dependencies", skillsHandler.getDependencies)
			skillsGroup.GET("/:name/env", skillsHandler.getEnv)
			skillsGroup.GET("/:name/stats", skillsHandler.getStats)
			skillsGroup.POST("/:name/convert", skillsHandler.convertSkill)
			skillsGroup.POST("/:name/install", skillsHandler.installDependencies)
			skillsGroup.POST("/:name/install/runtime", skillsHandler.installRuntime)
			skillsGroup.POST("/:name/env", skillsHandler.setEnv)
			skillsGroup.POST("/:name/validate", skillsHandler.validateSkill)
			skillsGroup.POST("/:name/enable", skillsHandler.enableSkill)
			skillsGroup.POST("/:name/disable", skillsHandler.disableSkill)
			skillsGroup.POST("/batch/convert", skillsHandler.batchConvert)
			skillsGroup.POST("/batch/install", skillsHandler.batchInstall)
		}

		// 能力管理
		capabilities := NewCapabilitiesHandler(capMgr)
		capabilitiesGroup := api.Group("/capabilities")
		{
			capabilitiesGroup.GET("", capabilities.list)
			capabilitiesGroup.GET("/reindex/status", capabilities.getReIndexStatus)
			capabilitiesGroup.POST("/reindex", capabilities.triggerReIndex)
			capabilitiesGroup.POST("", capabilities.add)
			capabilitiesGroup.PUT("", capabilities.update)
			capabilitiesGroup.DELETE("", capabilities.remove)
		}

		// Cron 任务管理
		if cronScheduler != nil {
			cronHandler := NewCronHandler(cronScheduler)
			cronHandler.RegisterRoutes(api)
		}

		// 设置管理
		settings := NewSettingsHandler()
		api.GET("/settings", settings.getSettings)
		api.POST("/settings", settings.saveSettings)

		// 高级配置管理
		advancedConfig := NewAdvancedConfigHandler()
		api.GET("/config/advanced", advancedConfig.GetAdvancedConfig)
		api.POST("/config/advanced", advancedConfig.SaveAdvancedConfig)

		// 配置管理
		configHandler := NewConfigHandler()
		api.GET("/config/general", configHandler.GetGeneralConfig)
		api.POST("/config/general", configHandler.SaveGeneralConfig)
		api.GET("/config/server", configHandler.GetServerConfig)
		api.POST("/config/server", configHandler.SaveServerConfig)
		api.GET("/config/models", configHandler.GetModelsConfig)
		api.POST("/config/models", configHandler.SaveModelsConfig)
		api.GET("/config/capabilities", configHandler.GetCapabilitiesConfig)
		api.POST("/config/capabilities", configHandler.SaveCapabilitiesConfig)

		// 监控日志
		monitor := NewMonitorHandler()
		api.GET("/monitor", monitor.getLogs)
		api.DELETE("/monitor", monitor.clearLogs)

		// Token 使用统计
		tokenUsage := NewTokenUsageHandler(tokenUsageRepo)
		tokenUsageGroup := api.Group("/token-usage")
		{
			tokenUsageGroup.GET("/by-model", tokenUsage.GetByModelSummary)
			tokenUsageGroup.GET("/summary", tokenUsage.GetSummary)
		}
	}
}
