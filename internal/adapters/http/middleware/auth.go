package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"mindx/internal/core"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
)

const (
	userIDKey   = "user_id"
	usernameKey = "username"
)

// AuthMiddleware creates an authentication middleware
func AuthMiddleware(authService core.AuthenticationService, logger logging.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try JWT token first
		token := c.GetHeader("Authorization")
		if strings.HasPrefix(token, "Bearer ") {
			user, err := authService.ValidateJWT(token[7:])
			if err == nil && user != nil {
				// Set user info in context
				c.Set(userIDKey, user.ID)
				c.Set(usernameKey, user.Username)
				c.Next()
				return
			}
		}

		// Try API key
		apiKey := c.GetHeader("X-API-Key")
		if apiKey != "" {
			user, err := authService.ValidateAPIKey(apiKey)
			if err == nil && user != nil {
				// Set user info in context
				c.Set(userIDKey, user.ID)
				c.Set(usernameKey, user.Username)
				c.Next()
				return
			}
		}

		// Check if this is a public endpoint
		if isPublicEndpoint(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Authentication failed
		logger.Warn("Authentication failed",
			logging.String("path", c.Request.URL.Path),
			logging.String("ip", c.ClientIP()))

		c.JSON(http.StatusUnauthorized, gin.H{
			"error": i18n.T("auth.unauthorized"),
		})
		c.Abort()
	}
}

// GetUserID retrieves the user ID from context
func GetUserID(c *gin.Context) string {
	if userID, exists := c.Get(userIDKey); exists {
		if id, ok := userID.(string); ok {
			return id
		}
	}
	return ""
}

// GetUsername retrieves the username from context
func GetUsername(c *gin.Context) string {
	if username, exists := c.Get(usernameKey); exists {
		if name, ok := username.(string); ok {
			return name
		}
	}
	return ""
}

// isPublicEndpoint checks if the endpoint is public (doesn't require authentication)
func isPublicEndpoint(path string) bool {
	publicPaths := []string{
		"/api/health",
		"/api/auth/login",
		"/api/auth/register",
	}

	for _, publicPath := range publicPaths {
		if strings.HasPrefix(path, publicPath) {
			return true
		}
	}

	return false
}
