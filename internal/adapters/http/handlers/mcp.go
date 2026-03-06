package handlers

import (
	"mindx/internal/config"
	"mindx/internal/usecase/mcp"
	"mindx/pkg/logging"
	"net/http"

	"github.com/gin-gonic/gin"
)

type MCPHandler struct {
	mcpManager *mcp.MCPManager
	logger     logging.Logger
}

func NewMCPHandler(mcpManager *mcp.MCPManager) *MCPHandler {
	return &MCPHandler{
		mcpManager: mcpManager,
		logger:     logging.GetSystemLogger().Named("mcp_handler"),
	}
}

func (h *MCPHandler) listServers(c *gin.Context) {
	servers := h.mcpManager.GetServers()
	c.JSON(http.StatusOK, gin.H{
		"servers": servers,
		"count":   len(servers),
	})
}

func (h *MCPHandler) addServer(c *gin.Context) {
	var req struct {
		Name    string            `json:"name" binding:"required"`
		Command string            `json:"command" binding:"required"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	server := &mcp.MCPServer{
		Name:    req.Name,
		Command: req.Command,
		Args:    req.Args,
		Env:     req.Env,
	}

	if err := h.mcpManager.AddServer(c.Request.Context(), req.Name, server); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 持久化到配置文件
	h.saveServerToConfig(req.Name, config.MCPServerEntry{
		Type:    "stdio",
		Command: req.Command,
		Args:    req.Args,
		Env:     req.Env,
		Enabled: true,
	})

	c.JSON(http.StatusOK, gin.H{"message": "MCP server added", "name": req.Name})
}

func (h *MCPHandler) removeServer(c *gin.Context) {
	name := c.Param("name")
	if err := h.mcpManager.RemoveServer(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 从配置文件中移除
	h.removeServerFromConfig(name)

	c.JSON(http.StatusOK, gin.H{"message": "MCP server removed", "name": name})
}

func (h *MCPHandler) restartServer(c *gin.Context) {
	name := c.Param("name")
	if err := h.mcpManager.RestartServer(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "MCP server restarted", "name": name})
}

func (h *MCPHandler) getServerTools(c *gin.Context) {
	name := c.Param("name")
	tools, err := h.mcpManager.GetServerTools(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	toolList := make([]gin.H, 0, len(tools))
	for _, t := range tools {
		toolList = append(toolList, gin.H{
			"name":        t.Name,
			"description": t.Description,
			"schema":      t.Schema,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"server": name,
		"tools":  toolList,
		"count":  len(toolList),
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
