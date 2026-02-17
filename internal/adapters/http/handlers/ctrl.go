package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ControlHandler struct{}

func NewControlHandler() *ControlHandler {
	return &ControlHandler{}
}

func (h *ControlHandler) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
