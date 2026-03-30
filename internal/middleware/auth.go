package middleware

import (
	"dblocker_control/internal/models"
	"dblocker_control/internal/service"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func AuthRequired(authSvc *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Check X-API-Key header (for service-to-service / automated instances)
		if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
			if authSvc.ValidateAPIKey(apiKey) {
				// API key is valid — set a synthetic service user
				c.Set("user", &models.User{
					ID:       0,
					Username: "_service",
					IsAdmin:  false,
				})
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			return
		}

		// 2. Check Bearer token (for human users)
		var tokenStr string

		header := c.GetHeader("Authorization")
		if header != "" && strings.HasPrefix(header, "Bearer ") {
			tokenStr = strings.TrimPrefix(header, "Bearer ")
		}

		// Fallback to query parameter (needed for EventSource/SSE)
		if tokenStr == "" {
			tokenStr = c.Query("token")
		}

		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
			return
		}

		user, err := authSvc.ValidateToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set("user", user)
		c.Next()
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userVal, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
			return
		}

		user, ok := userVal.(*models.User)
		if !ok || !user.IsAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			return
		}

		c.Next()
	}
}
