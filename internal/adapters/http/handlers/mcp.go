package handlers

import (
	"mindx/internal/config"
	"mindx/internal/usecase/skills"
	"mindx/pkg/logging"
	"net/http"

	"github.com/gin-gonic/gin"
)

type MCPHandler struct {
	skillMgr *skills.SkillMgr
	logger   logging.Logger
}

func NewMCPHandler(skillMgr *skills.SkillMgr) *MCPHandler {
	return &MCPHandler{
		skillMgr: skillMgr,
		logger:   logging.GetSystemLogger().Named("mcp_handler"),
	}
}

// TODO: Phase 5 - MCP 管理应该由独立的 MCPManager 负责，不应该在 SkillManager 中
// 临时禁用这些方法，等待 Phase 5 实现 MCPManager
func (h *MCPHandler) listServers(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "MCP management is being refactored",
		"message": "MCP server management will be available in Phase 5",
	})
}

func (h *MCPHandler) addServer(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "MCP management is being refactored",
		"message": "MCP server management will be available in Phase 5",
	})
}

func (h *MCPHandler) removeServer(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "MCP management is being refactored",
		"message": "MCP server management will be available in Phase 5",
	})
}

func (h *MCPHandler) restartServer(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "MCP management is being refactored",
		"message": "MCP server management will be available in Phase 5",
	})
}

func (h *MCPHandler) getServerTools(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "MCP management is being refactored",
		"message": "MCP server management will be available in Phase 5",
	})
}

func (h *MCPHandler) saveServerToConfig(name string, entry config.MCPServerEntry) {
	cfg, err := config.LoadMCPServersConfig()
	if err != nil {
		h.logger.Warn("加载 MCP 配置失败", logging.Err(err))
		return
	}
	cfg.MCPServers[name] = entry
	if err := config.SaveMCPServersConfig(cfg); err != nil {
		h.logger.Warn("保存 MCP 配置失败", logging.Err(err))
	}
}

func (h *MCPHandler) removeServerFromConfig(name string) {
	cfg, err := config.LoadMCPServersConfig()
	if err != nil {
		h.logger.Warn("加载 MCP 配置失败", logging.Err(err))
		return
	}
	delete(cfg.MCPServers, name)
	if err := config.SaveMCPServersConfig(cfg); err != nil {
		h.logger.Warn("保存 MCP 配置失败", logging.Err(err))
	}
}

// getCatalog 返回 MCP 目录列表 + 已安装状态
// TODO: Phase 5 - 使用 MCPManager 实现
func (h *MCPHandler) getCatalog(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "MCP management is being refactored",
		"message": "MCP catalog will be available in Phase 5",
	})
}

// installFromCatalog 从目录一键安装 MCP server
// TODO: Phase 5 - 使用 MCPManager 实现
func (h *MCPHandler) installFromCatalog(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "MCP management is being refactored",
		"message": "MCP installation will be available in Phase 5",
	})
}
