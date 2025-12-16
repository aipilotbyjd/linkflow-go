package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/pkg/auth/jwt"
	"github.com/redis/go-redis/v9"
)

// JWTMiddleware validates JWT tokens and extracts user information
type JWTMiddleware struct {
	jwtManager *jwt.Manager
	redis      *redis.Client
	skipPaths  []string
}

// NewJWTMiddleware creates a new JWT middleware
func NewJWTMiddleware(jwtManager *jwt.Manager, redis *redis.Client) *JWTMiddleware {
	return &JWTMiddleware{
		jwtManager: jwtManager,
		redis:      redis,
		skipPaths: []string{
			"/health",
			"/ready",
			"/metrics",
			"/api/v1/auth/login",
			"/api/v1/auth/register",
			"/api/v1/auth/refresh",
			"/api/v1/auth/verify-email",
			"/api/v1/auth/forgot-password",
			"/api/v1/auth/reset-password",
			"/api/v1/auth/oauth",
		},
	}
}

// Handle returns the middleware handler function
func (m *JWTMiddleware) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authentication for certain paths
		path := c.Request.URL.Path
		for _, skipPath := range m.skipPaths {
			if strings.HasPrefix(path, skipPath) {
				c.Next()
				return
			}
		}

		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		// Validate Bearer scheme
		const bearerScheme = "Bearer "
		if !strings.HasPrefix(authHeader, bearerScheme) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		token := authHeader[len(bearerScheme):]

		// Check if token is blacklisted (for logout functionality)
		if m.redis != nil {
			blacklisted, _ := m.redis.Exists(context.Background(), "blacklist:"+token).Result()
			if blacklisted > 0 {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
				c.Abort()
				return
			}
		}

		// Validate token
		claims, err := m.jwtManager.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// Set user context
		c.Set("userId", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("roles", claims.Roles)
		c.Set("permissions", claims.Permissions)
		c.Set("token", token)

		c.Next()
	}
}

// RequireRoles creates a middleware that checks if user has any of the required roles
func RequireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRoles, exists := c.Get("roles")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "no roles found in context"})
			c.Abort()
			return
		}

		userRolesList, ok := userRoles.([]string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid roles format"})
			c.Abort()
			return
		}

		// Check if user has any of the required roles
		hasRole := false
		for _, requiredRole := range roles {
			for _, userRole := range userRolesList {
				if userRole == requiredRole {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequirePermissions creates a middleware that checks if user has all required permissions
func RequirePermissions(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userPermissions, exists := c.Get("permissions")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "no permissions found in context"})
			c.Abort()
			return
		}

		userPermsList, ok := userPermissions.([]string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid permissions format"})
			c.Abort()
			return
		}

		// Check if user has all required permissions
		for _, requiredPerm := range permissions {
			hasPerm := false
			for _, userPerm := range userPermsList {
				if userPerm == requiredPerm {
					hasPerm = true
					break
				}
			}

			if !hasPerm {
				c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// GetUserID extracts user ID from context
func GetUserID(c *gin.Context) (string, bool) {
	userID, exists := c.Get("userId")
	if !exists {
		return "", false
	}

	id, ok := userID.(string)
	return id, ok
}

// GetUserEmail extracts user email from context
func GetUserEmail(c *gin.Context) (string, bool) {
	email, exists := c.Get("email")
	if !exists {
		return "", false
	}

	emailStr, ok := email.(string)
	return emailStr, ok
}

// GetUserRoles extracts user roles from context
func GetUserRoles(c *gin.Context) ([]string, bool) {
	roles, exists := c.Get("roles")
	if !exists {
		return nil, false
	}

	rolesList, ok := roles.([]string)
	return rolesList, ok
}

// GetUserPermissions extracts user permissions from context
func GetUserPermissions(c *gin.Context) ([]string, bool) {
	permissions, exists := c.Get("permissions")
	if !exists {
		return nil, false
	}

	permsList, ok := permissions.([]string)
	return permsList, ok
}
