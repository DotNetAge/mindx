package handlers

import (
	"mindx/internal/core"
	"net/http"

	"github.com/gin-gonic/gin"
)

type TokenUsageHandler struct {
	tokenUsageRepo core.TokenUsageRepository
}

func NewTokenUsageHandler(tokenUsageRepo core.TokenUsageRepository) *TokenUsageHandler {
	return &TokenUsageHandler{
		tokenUsageRepo: tokenUsageRepo,
	}
}

// GetByModelSummary 按模型分组获取 Token 使用统计
func (h *TokenUsageHandler) GetByModelSummary(c *gin.Context) {
	summaries, err := h.tokenUsageRepo.GetSummaryByModel()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取 Token 使用统计失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": summaries,
	})
}

// GetSummary 获取总统计
func (h *TokenUsageHandler) GetSummary(c *gin.Context) {
	summary, err := h.tokenUsageRepo.GetSummary()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取统计失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": summary,
	})
}
