package handlers

import (
	"context"
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

func (h *MCPHandler) listServers(c *gin.Context) {
	servers := h.skillMgr.GetMCPServers()
	c.JSON(http.StatusOK, gin.H{
		"servers": servers,
		"count":   len(servers),
	})
}

func (h *MCPHandler) addServer(c *gin.Context) {
	var req struct {
		Name    string            `json:"name" binding:"required"`
		Type    string            `json:"type"`
		// stdio fields
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
		// sse fields
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
		// common
		Enabled bool              `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	serverType := req.Type
	if serverType == "" {
		serverType = "stdio"
	}

	// 校验必填字段
	switch serverType {
	case "sse":
		if req.URL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "url is required for SSE type"})
			return
		}
	default:
		if req.Command == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "command is required for stdio type"})
			return
		}
	}

	entry := config.MCPServerEntry{
		Type:    serverType,
		Command: req.Command,
		Args:    req.Args,
		Env:     req.Env,
		URL:     req.URL,
		Headers: req.Headers,
		Enabled: req.Enabled,
	}

	if err := h.skillMgr.AddMCPServer(c.Request.Context(), req.Name, entry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 持久化到配置文件
	h.saveServerToConfig(req.Name, entry)

	c.JSON(http.StatusOK, gin.H{"message": "MCP server added", "name": req.Name})
}

func (h *MCPHandler) removeServer(c *gin.Context) {
	name := c.Param("name")
	if err := h.skillMgr.RemoveMCPServer(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 从配置文件中移除
	h.removeServerFromConfig(name)

	c.JSON(http.StatusOK, gin.H{"message": "MCP server removed", "name": name})
}

func (h *MCPHandler) restartServer(c *gin.Context) {
	name := c.Param("name")
	if err := h.skillMgr.RestartMCPServer(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "MCP server restarted", "name": name})
}

func (h *MCPHandler) getServerTools(c *gin.Context) {
	name := c.Param("name")
	tools, err := h.skillMgr.GetMCPServerTools(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	toolList := make([]gin.H, 0, len(tools))
	for _, t := range tools {
		toolList = append(toolList, gin.H{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
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
func (h *MCPHandler) getCatalog(c *gin.Context) {
	catalog, err := config.LoadBuiltinCatalog()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load catalog"})
		return
	}

	// 获取已安装的 server 名称列表
	servers := h.skillMgr.GetMCPServers()
	installed := make([]string, 0, len(servers))
	for _, s := range servers {
		installed = append(installed, s.Name)
	}

	c.JSON(http.StatusOK, gin.H{
		"servers":   catalog.Servers,
		"installed": installed,
	})
}

// installFromCatalog 从目录一键安装 MCP server
func (h *MCPHandler) installFromCatalog(c *gin.Context) {
	var req struct {
		ID        string            `json:"id" binding:"required"`
		Variables map[string]string `json:"variables"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	catalog, err := config.LoadBuiltinCatalog()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load catalog"})
		return
	}

	// 查找目录条目
	var found *config.CatalogEntry
	for i := range catalog.Servers {
		if catalog.Servers[i].ID == req.ID {
			found = &catalog.Servers[i]
			break
		}
	}
	if found == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "catalog entry not found: " + req.ID})
		return
	}

	// 校验必填变量
	for _, v := range found.Variables {
		if v.Required {
			val := req.Variables[v.Key]
			if val == "" && v.Default == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "missing required variable: " + v.Key})
				return
			}
			// 使用默认值
			if val == "" {
				if req.Variables == nil {
					req.Variables = make(map[string]string)
				}
				req.Variables[v.Key] = v.Default
			}
		}
	}

	// 解析目录条目为 MCPServerEntry
	entry := config.ResolveCatalogEntry(found, req.Variables)

	// 先持久化配置（立即生效）
	h.saveServerToConfig(req.ID, entry)

	// 异步连接 MCP server（不阻塞 HTTP 响应）
	go func() {
		if err := h.skillMgr.AddMCPServer(context.Background(), req.ID, entry); err != nil {
			h.logger.Warn("MCP server 异步连接失败",
				logging.String("server", req.ID),
				logging.Err(err))
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "installed", "name": req.ID})
}
