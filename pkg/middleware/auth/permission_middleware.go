package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// PermissionMiddleware handles permission checking for resources
type PermissionMiddleware struct {
	// Resource to action mappings
	permissions map[string][]string
}

// NewPermissionMiddleware creates a new permission middleware
func NewPermissionMiddleware() *PermissionMiddleware {
	return &PermissionMiddleware{
		permissions: make(map[string][]string),
	}
}

// ResourcePermission defines a resource and its required action
type ResourcePermission struct {
	Resource string
	Action   string
}

// RequireResourcePermission checks if user has permission for a specific resource and action
func RequireResourcePermission(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		permissions, exists := c.Get("permissions")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "no permissions found"})
			c.Abort()
			return
		}

		userPerms, ok := permissions.([]string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid permissions format"})
			c.Abort()
			return
		}

		// Check if user has the required permission
		requiredPerm := fmt.Sprintf("%s:%s", resource, action)
		hasPermission := false

		for _, perm := range userPerms {
			if perm == requiredPerm || perm == "*:*" { // Super admin has all permissions
				hasPermission = true
				break
			}

			// Check for wildcard permissions
			if strings.HasSuffix(perm, ":*") {
				permResource := strings.TrimSuffix(perm, ":*")
				if permResource == resource {
					hasPermission = true
					break
				}
			}

			if strings.HasPrefix(perm, "*:") {
				permAction := strings.TrimPrefix(perm, "*:")
				if permAction == action {
					hasPermission = true
					break
				}
			}
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("permission denied for %s:%s", resource, action),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireOwnership checks if the user owns the resource they're trying to access
func RequireOwnership(getResourceOwnerFunc func(c *gin.Context) (string, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("userId")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "user not authenticated"})
			c.Abort()
			return
		}

		userIDStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID format"})
			c.Abort()
			return
		}

		// Check if user is admin (admins can access everything)
		roles, _ := c.Get("roles")
		if rolesList, ok := roles.([]string); ok {
			for _, role := range rolesList {
				if role == "admin" || role == "super_admin" {
					c.Next()
					return
				}
			}
		}

		// Get the resource owner
		ownerID, err := getResourceOwnerFunc(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to determine resource ownership"})
			c.Abort()
			return
		}

		// Check ownership
		if ownerID != userIDStr {
			c.JSON(http.StatusForbidden, gin.H{"error": "you don't have permission to access this resource"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// ServiceToServiceAuth validates service-to-service authentication
type ServiceToServiceAuth struct {
	serviceTokens map[string]string // Map of service names to their tokens
}

// NewServiceToServiceAuth creates a new service-to-service authentication middleware
func NewServiceToServiceAuth(tokens map[string]string) *ServiceToServiceAuth {
	return &ServiceToServiceAuth{
		serviceTokens: tokens,
	}
}

// Validate checks if the request is from a valid service
func (s *ServiceToServiceAuth) Validate() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for service token header
		serviceToken := c.GetHeader("X-Service-Token")
		serviceName := c.GetHeader("X-Service-Name")

		if serviceToken == "" || serviceName == "" {
			// Not a service-to-service request, continue with normal auth
			c.Next()
			return
		}

		// Validate service token
		expectedToken, exists := s.serviceTokens[serviceName]
		if !exists || expectedToken != serviceToken {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid service credentials"})
			c.Abort()
			return
		}

		// Set service context
		c.Set("isService", true)
		c.Set("serviceName", serviceName)

		// Services have all permissions
		c.Set("permissions", []string{"*:*"})

		c.Next()
	}
}

// CombineMiddleware combines multiple middleware functions
func CombineMiddleware(middlewares ...gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, middleware := range middlewares {
			middleware(c)
			if c.IsAborted() {
				return
			}
		}
		c.Next()
	}
}

// Common permission constants
const (
	// Resources
	ResourceUser       = "user"
	ResourceWorkflow   = "workflow"
	ResourceNode       = "node"
	ResourceCredential = "credential"
	ResourceAnalytics  = "analytics"
	ResourceSchedule   = "schedule"

	// Actions
	ActionCreate  = "create"
	ActionRead    = "read"
	ActionUpdate  = "update"
	ActionDelete  = "delete"
	ActionExecute = "execute"
	ActionList    = "list"
)
