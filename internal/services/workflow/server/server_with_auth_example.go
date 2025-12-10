// Example of how to integrate auth middleware into workflow service
// This file demonstrates the integration pattern for AUTH-010

package server

import (
	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/services/auth/jwt"
	"github.com/linkflow-go/internal/services/workflow/handlers"
	"github.com/linkflow-go/pkg/config"
	"github.com/linkflow-go/pkg/logger"
	"github.com/linkflow-go/pkg/middleware/auth"
	"github.com/redis/go-redis/v9"
)

// setupRouterWithAuth shows how to integrate auth middleware
func setupRouterWithAuth(h *handlers.WorkflowHandlers, cfg *config.Config, redisClient *redis.Client, log logger.Logger) (*gin.Engine, error) {
	router := gin.New()
	
	// Initialize JWT manager for this service
	jwtManager, err := jwt.NewManager(cfg.Auth)
	if err != nil {
		return nil, err
	}
	
	// Create JWT middleware
	jwtMiddleware := auth.NewJWTMiddleware(jwtManager, redisClient)
	
	// Create service-to-service auth (for inter-service communication)
	serviceAuth := auth.NewServiceToServiceAuth(map[string]string{
		"executor-service": "executor-secret-token",
		"schedule-service": "schedule-secret-token",
		"node-service": "node-secret-token",
	})
	
	// Global middleware
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(loggingMiddleware(log))
	
	// Apply service-to-service auth globally (before JWT)
	router.Use(serviceAuth.Validate())
	
	// Apply JWT middleware globally (will skip public endpoints)
	router.Use(jwtMiddleware.Handle())
	
	// Health checks (public, no auth required)
	router.GET("/health", h.Health)
	router.GET("/ready", h.Ready)
	
	// API routes
	v1 := router.Group("/api/v1")
	{
		workflows := v1.Group("/workflows")
		{
			// List workflows - requires authentication
			workflows.GET("", h.ListWorkflows)
			
			// Create workflow - requires specific permission
			workflows.POST("", 
				auth.RequireResourcePermission(auth.ResourceWorkflow, auth.ActionCreate),
				h.CreateWorkflow,
			)
			
			// Get workflow - requires read permission or ownership
			workflows.GET("/:id", 
				auth.RequireOwnership(func(c *gin.Context) (string, error) {
					// This function should fetch the workflow and return its owner ID
					// For example:
					workflowID := c.Param("id")
					// In real implementation, you would fetch from service/repository
					// workflow, err := workflowService.GetWorkflow(c.Request.Context(), workflowID)
					// return workflow.UserID, err
					return workflowID, nil // Placeholder
				}),
				h.GetWorkflow,
			)
			
			// Update workflow - requires update permission and ownership
			workflows.PUT("/:id",
				auth.RequireResourcePermission(auth.ResourceWorkflow, auth.ActionUpdate),
				auth.RequireOwnership(getWorkflowOwner),
				h.UpdateWorkflow,
			)
			
			// Delete workflow - requires delete permission and ownership
			workflows.DELETE("/:id",
				auth.RequireResourcePermission(auth.ResourceWorkflow, auth.ActionDelete),
				auth.RequireOwnership(getWorkflowOwner),
				h.DeleteWorkflow,
			)
			
			// Execute workflow - requires execute permission
			workflows.POST("/:id/execute",
				auth.RequireResourcePermission(auth.ResourceWorkflow, auth.ActionExecute),
				h.ExecuteWorkflow,
			)
			
			// Workflow versions - requires read permission
			workflows.GET("/:id/versions",
				auth.RequireResourcePermission(auth.ResourceWorkflow, auth.ActionRead),
				h.GetWorkflowVersions,
			)
			
			// Admin-only endpoints
			adminWorkflows := workflows.Group("")
			adminWorkflows.Use(auth.RequireRoles("admin", "super_admin"))
			{
				// Admin can view all workflows
				adminWorkflows.GET("/all", h.ListAllWorkflows)
				
				// Admin can force execute any workflow
				adminWorkflows.POST("/:id/force-execute", h.ForceExecuteWorkflow)
			}
		}
		
		// Templates endpoint - different permission model
		templates := v1.Group("/templates")
		{
			// Anyone authenticated can view templates
			templates.GET("", h.ListTemplates)
			
			// Only admins can create templates
			templates.POST("",
				auth.RequireRoles("admin", "template_manager"),
				h.CreateTemplate,
			)
		}
		
		// Analytics endpoints - require analytics permission
		analytics := v1.Group("/analytics")
		analytics.Use(auth.RequireResourcePermission(auth.ResourceAnalytics, auth.ActionRead))
		{
			analytics.GET("/workflows", h.GetWorkflowAnalytics)
			analytics.GET("/executions", h.GetExecutionAnalytics)
		}
	}
	
	return router, nil
}

// Helper function to get workflow owner
func getWorkflowOwner(c *gin.Context) (string, error) {
	workflowID := c.Param("id")
	// In real implementation:
	// workflow, err := workflowService.GetWorkflow(c.Request.Context(), workflowID)
	// if err != nil {
	//     return "", err
	// }
	// return workflow.UserID, nil
	
	// Placeholder for example
	return workflowID, nil
}

// Example of using auth context in handlers
func ExampleHandlerUsingAuth(c *gin.Context) {
	// Get user information from context
	userID, _ := auth.GetUserID(c)
	email, _ := auth.GetUserEmail(c)
	roles, _ := auth.GetUserRoles(c)
	permissions, _ := auth.GetUserPermissions(c)
	
	// Check if this is a service-to-service call
	isService, _ := c.Get("isService")
	if isService.(bool) {
		serviceName, _ := c.Get("serviceName")
		// Handle service-to-service logic
		c.JSON(200, gin.H{
			"message": "Service call from " + serviceName.(string),
		})
		return
	}
	
	// Regular user request
	c.JSON(200, gin.H{
		"userID":      userID,
		"email":       email,
		"roles":       roles,
		"permissions": permissions,
	})
}

// Update all services to use this pattern:
// 1. Import the middleware packages
// 2. Initialize JWT manager
// 3. Create JWT middleware instance
// 4. Apply middleware to routes
// 5. Use RequireRoles, RequirePermissions, RequireOwnership as needed
// 6. Access user context in handlers using helper functions
