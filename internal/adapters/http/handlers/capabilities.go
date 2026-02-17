package handlers

import (
	"net/http"

	"mindx/internal/entity"
	"mindx/internal/usecase/capability"

	"github.com/gin-gonic/gin"
)

type CapabilitiesHandler struct {
	capMgr *capability.CapabilityManager
}

func NewCapabilitiesHandler(capMgr *capability.CapabilityManager) *CapabilitiesHandler {
	return &CapabilitiesHandler{
		capMgr: capMgr,
	}
}

func (h *CapabilitiesHandler) list(c *gin.Context) {
	if h.capMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "能力管理器不可用"})
		return
	}

	caps := h.capMgr.ListCapabilities()

	c.JSON(http.StatusOK, gin.H{
		"capabilities": caps,
		"count":        len(caps),
	})
}

func (h *CapabilitiesHandler) add(c *gin.Context) {
	if h.capMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "能力管理器不可用"})
		return
	}

	var newCap entity.Capability
	if err := c.ShouldBindJSON(&newCap); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.capMgr.AddCapability(newCap); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "能力添加成功"})
}

func (h *CapabilitiesHandler) update(c *gin.Context) {
	if h.capMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "能力管理器不可用"})
		return
	}

	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少名称参数"})
		return
	}

	var updateData entity.Capability
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.capMgr.UpdateCapability(name, updateData); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "能力更新成功"})
}

func (h *CapabilitiesHandler) remove(c *gin.Context) {
	if h.capMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "能力管理器不可用"})
		return
	}

	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少名称参数"})
		return
	}

	if err := h.capMgr.RemoveCapability(name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "能力删除成功"})
}

func (h *CapabilitiesHandler) getReIndexStatus(c *gin.Context) {
	if h.capMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "能力管理器不可用"})
		return
	}

	isReIndexing := h.capMgr.IsReIndexing()
	reIndexError := h.capMgr.GetReIndexError()

	response := gin.H{
		"isReIndexing": isReIndexing,
	}

	if reIndexError != nil {
		response["error"] = reIndexError.Error()
	}

	c.JSON(http.StatusOK, response)
}

func (h *CapabilitiesHandler) triggerReIndex(c *gin.Context) {
	if h.capMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "能力管理器不可用"})
		return
	}

	if h.capMgr.IsReIndexing() {
		c.JSON(http.StatusConflict, gin.H{"error": "重新索引正在进行中"})
		return
	}

	go func() {
		if err := h.capMgr.ReIndex(); err != nil {
			// 错误已在 ReIndex 中记录
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{"message": "重新索引已启动"})
}
