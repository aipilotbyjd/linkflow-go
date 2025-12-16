package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CasbinMiddleware provides RBAC authorization using Casbin
type CasbinMiddleware struct {
	enforcer PermissionChecker
}

// NewCasbinMiddleware creates a new Casbin middleware
func NewCasbinMiddleware(enforcer PermissionChecker) *CasbinMiddleware {
	return &CasbinMiddleware{
		enforcer: enforcer,
	}
}

// Authorize returns a middleware that checks permissions using Casbin
func (m *CasbinMiddleware) Authorize() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authorization for health checks and public endpoints
		path := c.Request.URL.Path
		if isPublicEndpoint(path) {
			c.Next()
			return
		}

		// Check if this is a service-to-service call
		isService, _ := c.Get("isService")
		if isServiceBool, ok := isService.(bool); ok && isServiceBool {
			// Services have all permissions
			c.Next()
			return
		}

		// Get user ID from context (set by JWT middleware)
		userID, exists := c.Get("userId")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
			c.Abort()
			return
		}

		userIDStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID format"})
			c.Abort()
			return
		}

		// Get request path and method
		obj := c.Request.URL.Path
		act := methodToAction(c.Request.Method)

		// Special handling for "owned" resources
		if strings.Contains(obj, "/owned/") {
			// Replace "owned" with actual user ID for permission check
			obj = strings.Replace(obj, "/owned/", fmt.Sprintf("/%s/", userIDStr), 1)
		}

		// Check permission
		allowed, err := m.enforcer.CheckPermission(userIDStr, obj, act)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check permissions"})
			c.Abort()
			return
		}

		if !allowed {
			// Try checking with user's roles
			roles, _ := c.Get("roles")
			if rolesList, ok := roles.([]string); ok {
				for _, role := range rolesList {
					allowed, _ = m.enforcer.CheckPermission(role, obj, act)
					if allowed {
						break
					}
				}
			}
		}

		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "permission denied",
				"details": fmt.Sprintf("user %s cannot %s %s", userIDStr, act, obj),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequirePermission creates a middleware that requires specific permission
func (m *CasbinMiddleware) RequirePermission(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context
		userID, exists := c.Get("userId")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
			c.Abort()
			return
		}

		userIDStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID format"})
			c.Abort()
			return
		}

		// Check permission
		allowed, err := m.enforcer.CheckPermission(userIDStr, resource, action)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check permissions"})
			c.Abort()
			return
		}

		// If not allowed directly, check through roles
		if !allowed {
			roles, _ := c.Get("roles")
			if rolesList, ok := roles.([]string); ok {
				for _, role := range rolesList {
					allowed, _ = m.enforcer.CheckPermission(role, resource, action)
					if allowed {
						break
					}
				}
			}
		}

		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "permission denied",
				"details": fmt.Sprintf("requires permission: %s:%s", resource, action),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireRole creates a middleware that requires specific role
func (m *CasbinMiddleware) RequireRole(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("userId")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
			c.Abort()
			return
		}

		userIDStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID format"})
			c.Abort()
			return
		}

		// Get user's roles from Casbin
		userRoles, err := m.enforcer.GetRoles(userIDStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user roles"})
			c.Abort()
			return
		}

		// Check if user has any of the required roles
		hasRole := false
		for _, requiredRole := range requiredRoles {
			for _, userRole := range userRoles {
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
			c.JSON(http.StatusForbidden, gin.H{
				"error":    "insufficient role",
				"required": requiredRoles,
			})
			c.Abort()
			return
		}

		// Update roles in context for other middleware
		c.Set("casbinRoles", userRoles)

		c.Next()
	}
}

// Helper function to map HTTP methods to actions
func methodToAction(method string) string {
	switch method {
	case http.MethodGet:
		return "read"
	case http.MethodPost:
		return "create"
	case http.MethodPut, http.MethodPatch:
		return "update"
	case http.MethodDelete:
		return "delete"
	default:
		return "read"
	}
}

// Helper function to check if endpoint is public
func isPublicEndpoint(path string) bool {
	publicEndpoints := []string{
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
	}

	for _, endpoint := range publicEndpoints {
		if strings.HasPrefix(path, endpoint) {
			return true
		}
	}

	return false
}
