package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"mindx/internal/core"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
)

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	authService core.AuthenticationService
	logger      logging.Logger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService core.AuthenticationService, logger logging.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		logger:      logger.Named("AuthHandler"),
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login handles user login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": i18n.T("auth.invalid_request"),
			"details": err.Error(),
		})
		return
	}

	// Authenticate
	token, apiKey, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		h.logger.Warn("Login failed",
			logging.String(i18n.T("auth.username"), req.Username),
			logging.Err(err))

		c.JSON(http.StatusUnauthorized, gin.H{
			"error": i18n.T("auth.invalid_credentials"),
		})
		return
	}

	h.logger.Info("User logged in successfully",
		logging.String(i18n.T("auth.username"), req.Username))

	c.JSON(http.StatusOK, gin.H{
		"token":    token,
		"api_key":  apiKey,
		"username": req.Username,
	})
}
