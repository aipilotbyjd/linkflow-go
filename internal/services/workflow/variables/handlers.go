package variables

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/pkg/logger"
)

type Handlers struct {
	service *Service
	logger  logger.Logger
}

func NewHandlers(service *Service, logger logger.Logger) *Handlers {
	return &Handlers{
		service: service,
		logger:  logger,
	}
}

// Variable handlers

func (h *Handlers) ListVariables(c *gin.Context) {
	workflowID := c.Param("workflowId")
	scope := c.Query("scope")

	var vars interface{}
	var err error

	if scope != "" {
		vars, err = h.service.ListVariablesByScope(c.Request.Context(), workflowID, scope)
	} else {
		vars, err = h.service.ListVariables(c.Request.Context(), workflowID)
	}

	if err != nil {
		h.logger.Error("Failed to list variables", "error", err, "workflowId", workflowID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list variables"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"variables": vars})
}

func (h *Handlers) GetVariable(c *gin.Context) {
	workflowID := c.Param("workflowId")
	key := c.Param("key")

	variable, err := h.service.GetVariable(c.Request.Context(), workflowID, key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "variable not found"})
		return
	}

	c.JSON(http.StatusOK, variable)
}

func (h *Handlers) CreateVariable(c *gin.Context) {
	workflowID := c.Param("workflowId")

	var req CreateVariableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.WorkflowID = workflowID

	variable, err := h.service.CreateVariable(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to create variable", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, variable)
}

func (h *Handlers) UpdateVariable(c *gin.Context) {
	workflowID := c.Param("workflowId")
	key := c.Param("key")

	var req UpdateVariableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	variable, err := h.service.UpdateVariable(c.Request.Context(), workflowID, key, req)
	if err != nil {
		h.logger.Error("Failed to update variable", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, variable)
}

func (h *Handlers) DeleteVariable(c *gin.Context) {
	workflowID := c.Param("workflowId")
	key := c.Param("key")

	if err := h.service.DeleteVariable(c.Request.Context(), workflowID, key); err != nil {
		h.logger.Error("Failed to delete variable", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// Environment handlers

func (h *Handlers) ListEnvironments(c *gin.Context) {
	workflowID := c.Param("workflowId")

	environments, err := h.service.ListEnvironments(c.Request.Context(), workflowID)
	if err != nil {
		h.logger.Error("Failed to list environments", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list environments"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"environments": environments})
}

func (h *Handlers) GetEnvironment(c *gin.Context) {
	workflowID := c.Param("workflowId")
	envID := c.Param("envId")

	env, err := h.service.GetEnvironment(c.Request.Context(), workflowID, envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}

	c.JSON(http.StatusOK, env)
}

func (h *Handlers) CreateEnvironment(c *gin.Context) {
	workflowID := c.Param("workflowId")

	var req CreateEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.WorkflowID = workflowID

	env, err := h.service.CreateEnvironment(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to create environment", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, env)
}

func (h *Handlers) UpdateEnvironment(c *gin.Context) {
	workflowID := c.Param("workflowId")
	envID := c.Param("envId")

	var req UpdateEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	env, err := h.service.UpdateEnvironment(c.Request.Context(), workflowID, envID, req)
	if err != nil {
		h.logger.Error("Failed to update environment", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, env)
}

func (h *Handlers) SetDefaultEnvironment(c *gin.Context) {
	workflowID := c.Param("workflowId")
	envID := c.Param("envId")

	if err := h.service.SetDefaultEnvironment(c.Request.Context(), workflowID, envID); err != nil {
		h.logger.Error("Failed to set default environment", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Default environment set"})
}

func (h *Handlers) DeleteEnvironment(c *gin.Context) {
	workflowID := c.Param("workflowId")
	envID := c.Param("envId")

	if err := h.service.DeleteEnvironment(c.Request.Context(), workflowID, envID); err != nil {
		h.logger.Error("Failed to delete environment", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// Environment variable handlers

func (h *Handlers) SetEnvironmentVariable(c *gin.Context) {
	workflowID := c.Param("workflowId")
	envID := c.Param("envId")
	key := c.Param("key")

	var req struct {
		Value interface{} `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.SetEnvironmentVariable(c.Request.Context(), workflowID, envID, key, req.Value); err != nil {
		h.logger.Error("Failed to set environment variable", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Variable set"})
}

func (h *Handlers) DeleteEnvironmentVariable(c *gin.Context) {
	workflowID := c.Param("workflowId")
	envID := c.Param("envId")
	key := c.Param("key")

	if err := h.service.DeleteEnvironmentVariable(c.Request.Context(), workflowID, envID, key); err != nil {
		h.logger.Error("Failed to delete environment variable", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// Global variables handlers

func (h *Handlers) ListGlobalVariables(c *gin.Context) {
	vars, err := h.service.repo.ListGlobalVariables(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list global variables", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list global variables"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"variables": vars})
}

func (h *Handlers) CreateGlobalVariable(c *gin.Context) {
	var req CreateVariableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.WorkflowID = "global"
	req.Scope = "global"

	variable, err := h.service.CreateVariable(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to create global variable", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, variable)
}

// RegisterRoutes registers variable routes on a router group
func RegisterRoutes(router *gin.RouterGroup, handlers *Handlers) {
	// Workflow variables
	vars := router.Group("/workflows/:workflowId/variables")
	{
		vars.GET("", handlers.ListVariables)
		vars.POST("", handlers.CreateVariable)
		vars.GET("/:key", handlers.GetVariable)
		vars.PUT("/:key", handlers.UpdateVariable)
		vars.DELETE("/:key", handlers.DeleteVariable)
	}

	// Workflow environments
	envs := router.Group("/workflows/:workflowId/environments")
	{
		envs.GET("", handlers.ListEnvironments)
		envs.POST("", handlers.CreateEnvironment)
		envs.GET("/:envId", handlers.GetEnvironment)
		envs.PUT("/:envId", handlers.UpdateEnvironment)
		envs.DELETE("/:envId", handlers.DeleteEnvironment)
		envs.POST("/:envId/default", handlers.SetDefaultEnvironment)
		envs.PUT("/:envId/variables/:key", handlers.SetEnvironmentVariable)
		envs.DELETE("/:envId/variables/:key", handlers.DeleteEnvironmentVariable)
	}

	// Global variables
	global := router.Group("/variables/global")
	{
		global.GET("", handlers.ListGlobalVariables)
		global.POST("", handlers.CreateGlobalVariable)
	}
}
