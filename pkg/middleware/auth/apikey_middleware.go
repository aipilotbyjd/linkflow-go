package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIKeyMiddleware creates middleware that authenticates requests using API keys
func APIKeyMiddleware(apiKeyValidator APIKeyValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for API key in header
		apiKeyHeader := c.GetHeader("X-API-Key")
		if apiKeyHeader == "" {
			// Also check Authorization header with ApiKey scheme
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "ApiKey ") {
				apiKeyHeader = strings.TrimPrefix(authHeader, "ApiKey ")
			}
		}

		if apiKeyHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
			c.Abort()
			return
		}

		if apiKeyValidator == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "API key validation not configured"})
			c.Abort()
			return
		}

		// Validate the API key
		key, err := apiKeyValidator.Validate(c.Request.Context(), apiKeyHeader)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		// Set user context from API key
		c.Set("userId", key.UserID)
		c.Set("apiKeyId", key.ID)
		c.Set("apiKeyPermissions", key.Permissions)
		c.Set("authMethod", "apikey")

		c.Next()
	}
}

// CombinedAuthMiddleware supports both JWT and API key authentication
func CombinedAuthMiddleware(jwtMiddleware, apiKeyMiddleware gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for API key first
		apiKeyHeader := c.GetHeader("X-API-Key")
		authHeader := c.GetHeader("Authorization")

		// If X-API-Key header is present or Authorization uses ApiKey scheme
		if apiKeyHeader != "" || strings.HasPrefix(authHeader, "ApiKey ") {
			apiKeyMiddleware(c)
			return
		}

		// Otherwise, try JWT authentication
		jwtMiddleware(c)
	}
}

// APIKeyValidatorFunc adapts a function to APIKeyValidator.
type APIKeyValidatorFunc func(ctx context.Context, rawKey string) (*APIKeyInfo, error)

func (f APIKeyValidatorFunc) Validate(ctx context.Context, rawKey string) (*APIKeyInfo, error) {
	return f(ctx, rawKey)
}

// RequireAPIKeyPermission creates middleware that checks for specific API key permission
func RequireAPIKeyPermission(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only check for API key authenticated requests
		authMethod, exists := c.Get("authMethod")
		if !exists || authMethod != "apikey" {
			// Not using API key, proceed (JWT has its own permission system)
			c.Next()
			return
		}

		permissions, exists := c.Get("apiKeyPermissions")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "no permissions found"})
			c.Abort()
			return
		}

		permList, ok := permissions.([]string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid permissions format"})
			c.Abort()
			return
		}

		// Check permission
		hasPermission := false
		requiredPerm := resource + ":" + action
		wildcardPerm := resource + ":*"

		for _, p := range permList {
			if p == requiredPerm || p == wildcardPerm || p == "*" {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{
				"error":    "insufficient permissions",
				"required": requiredPerm,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
