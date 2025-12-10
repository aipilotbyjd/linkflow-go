package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/internal/services/workflow/service"
	"github.com/linkflow-go/pkg/logger"
)

type WorkflowHandlers struct {
	service *service.WorkflowService
	logger  logger.Logger
}

func NewWorkflowHandlers(service *service.WorkflowService, logger logger.Logger) *WorkflowHandlers {
	return &WorkflowHandlers{
		service: service,
		logger:  logger,
	}
}

// Health check handlers
func (h *WorkflowHandlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func (h *WorkflowHandlers) Ready(c *gin.Context) {
	if err := h.service.CheckReady(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

// Workflow CRUD
func (h *WorkflowHandlers) ListWorkflows(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.Query("status")
	
	workflows, total, err := h.service.ListWorkflows(c.Request.Context(), userID, page, limit, status)
	if err != nil {
		h.logger.Error("Failed to list workflows", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list workflows"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"workflows": workflows,
		"total":     total,
		"page":      page,
		"limit":     limit,
	})
}

func (h *WorkflowHandlers) GetWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	workflow, err := h.service.GetWorkflow(c.Request.Context(), workflowID, userID)
	if err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to get workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get workflow"})
		return
	}
	
	c.JSON(http.StatusOK, workflow)
}

func (h *WorkflowHandlers) CreateWorkflow(c *gin.Context) {
	var req workflow.CreateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	req.UserID = c.GetString("user_id")
	
	workflow, err := h.service.CreateWorkflow(c.Request.Context(), &req)
	if err != nil {
		if err == service.ErrInvalidWorkflow {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("Failed to create workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create workflow"})
		return
	}
	
	c.JSON(http.StatusCreated, workflow)
}

func (h *WorkflowHandlers) UpdateWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	var req workflow.UpdateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	req.WorkflowID = workflowID
	req.UserID = userID
	
	workflow, err := h.service.UpdateWorkflow(c.Request.Context(), &req)
	if err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		if err == service.ErrUnauthorized {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}
		h.logger.Error("Failed to update workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update workflow"})
		return
	}
	
	c.JSON(http.StatusOK, workflow)
}

func (h *WorkflowHandlers) DeleteWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	if err := h.service.DeleteWorkflow(c.Request.Context(), workflowID, userID); err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		if err == service.ErrUnauthorized {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}
		h.logger.Error("Failed to delete workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workflow"})
		return
	}
	
	c.Status(http.StatusNoContent)
}

// Workflow versions
func (h *WorkflowHandlers) GetWorkflowVersions(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	versions, err := h.service.GetWorkflowVersions(c.Request.Context(), workflowID, userID)
	if err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to get workflow versions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get workflow versions"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"versions": versions})
}

func (h *WorkflowHandlers) GetWorkflowVersion(c *gin.Context) {
	workflowID := c.Param("id")
	version, _ := strconv.Atoi(c.Param("version"))
	userID := c.GetString("user_id")
	
	workflow, err := h.service.GetWorkflowVersion(c.Request.Context(), workflowID, version, userID)
	if err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow version not found"})
			return
		}
		h.logger.Error("Failed to get workflow version", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get workflow version"})
		return
	}
	
	c.JSON(http.StatusOK, workflow)
}

func (h *WorkflowHandlers) CreateWorkflowVersion(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	var req workflow.CreateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	version, err := h.service.CreateWorkflowVersion(c.Request.Context(), workflowID, userID, &req)
	if err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to create workflow version", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create workflow version"})
		return
	}
	
	c.JSON(http.StatusCreated, gin.H{"version": version})
}

func (h *WorkflowHandlers) RollbackWorkflowVersion(c *gin.Context) {
	workflowID := c.Param("id")
	version, _ := strconv.Atoi(c.Param("version"))
	userID := c.GetString("user_id")
	
	if err := h.service.RollbackWorkflowVersion(c.Request.Context(), workflowID, version, userID); err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow version not found"})
			return
		}
		h.logger.Error("Failed to rollback workflow version", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to rollback workflow version"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Workflow rolled back successfully"})
}

// Workflow operations
func (h *WorkflowHandlers) ActivateWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	if err := h.service.ActivateWorkflow(c.Request.Context(), workflowID, userID); err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to activate workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to activate workflow"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Workflow activated"})
}

func (h *WorkflowHandlers) DeactivateWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	if err := h.service.DeactivateWorkflow(c.Request.Context(), workflowID, userID); err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to deactivate workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deactivate workflow"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Workflow deactivated"})
}

func (h *WorkflowHandlers) DuplicateWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	workflow, err := h.service.DuplicateWorkflow(c.Request.Context(), workflowID, userID, req.Name)
	if err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to duplicate workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to duplicate workflow"})
		return
	}
	
	c.JSON(http.StatusCreated, workflow)
}

func (h *WorkflowHandlers) ValidateWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	errors, warnings, err := h.service.ValidateWorkflow(c.Request.Context(), workflowID, userID)
	if err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to validate workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate workflow"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"valid":    len(errors) == 0,
		"errors":   errors,
		"warnings": warnings,
	})
}

func (h *WorkflowHandlers) ExecuteWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	var req struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	executionID, err := h.service.ExecuteWorkflow(c.Request.Context(), workflowID, userID, req.Data)
	if err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		if err == service.ErrWorkflowInactive {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Workflow is inactive"})
			return
		}
		h.logger.Error("Failed to execute workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute workflow"})
		return
	}
	
	c.JSON(http.StatusAccepted, gin.H{
		"execution_id": executionID,
		"status":       "started",
	})
}

func (h *WorkflowHandlers) TestWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	var req struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	result, err := h.service.TestWorkflow(c.Request.Context(), workflowID, userID, req.Data)
	if err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to test workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to test workflow"})
		return
	}
	
	c.JSON(http.StatusOK, result)
}

// Workflow sharing
func (h *WorkflowHandlers) GetWorkflowPermissions(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	permissions, err := h.service.GetWorkflowPermissions(c.Request.Context(), workflowID, userID)
	if err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to get workflow permissions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get workflow permissions"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"permissions": permissions})
}

func (h *WorkflowHandlers) ShareWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	var req struct {
		UserID     string `json:"user_id"`
		Permission string `json:"permission" binding:"required,oneof=view edit admin"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if err := h.service.ShareWorkflow(c.Request.Context(), workflowID, userID, req.UserID, req.Permission); err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to share workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to share workflow"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Workflow shared successfully"})
}

func (h *WorkflowHandlers) UnshareWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	targetUserID := c.Param("userId")
	userID := c.GetString("user_id")
	
	if err := h.service.UnshareWorkflow(c.Request.Context(), workflowID, userID, targetUserID); err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to unshare workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unshare workflow"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Workflow unshared successfully"})
}

func (h *WorkflowHandlers) PublishWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	var req struct {
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if err := h.service.PublishWorkflow(c.Request.Context(), workflowID, userID, req.Description, req.Tags); err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to publish workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to publish workflow"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Workflow published successfully"})
}

// Workflow templates
func (h *WorkflowHandlers) ListTemplates(c *gin.Context) {
	category := c.Query("category")
	
	templates, err := h.service.ListTemplates(c.Request.Context(), category)
	if err != nil {
		h.logger.Error("Failed to list templates", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list templates"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

func (h *WorkflowHandlers) GetTemplate(c *gin.Context) {
	templateID := c.Param("id")
	
	template, err := h.service.GetTemplate(c.Request.Context(), templateID)
	if err != nil {
		if err == service.ErrTemplateNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
			return
		}
		h.logger.Error("Failed to get template", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get template"})
		return
	}
	
	c.JSON(http.StatusOK, template)
}

func (h *WorkflowHandlers) CreateTemplate(c *gin.Context) {
	userID := c.GetString("user_id")
	
	var req workflow.CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	req.CreatorID = userID
	
	template, err := h.service.CreateTemplate(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to create template", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create template"})
		return
	}
	
	c.JSON(http.StatusCreated, template)
}

func (h *WorkflowHandlers) CreateFromTemplate(c *gin.Context) {
	templateID := c.Param("templateId")
	userID := c.GetString("user_id")
	
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	workflow, err := h.service.CreateFromTemplate(c.Request.Context(), templateID, userID, req.Name)
	if err != nil {
		if err == service.ErrTemplateNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
			return
		}
		h.logger.Error("Failed to create from template", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create from template"})
		return
	}
	
	c.JSON(http.StatusCreated, workflow)
}

// Workflow import/export
func (h *WorkflowHandlers) ImportWorkflow(c *gin.Context) {
	userID := c.GetString("user_id")
	
	var req struct {
		Data   interface{} `json:"data" binding:"required"`
		Format string      `json:"format" binding:"required,oneof=json yaml n8n"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	workflow, err := h.service.ImportWorkflow(c.Request.Context(), userID, req.Data, req.Format)
	if err != nil {
		h.logger.Error("Failed to import workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to import workflow"})
		return
	}
	
	c.JSON(http.StatusCreated, workflow)
}

func (h *WorkflowHandlers) ExportWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	format := c.DefaultQuery("format", "json")
	
	data, err := h.service.ExportWorkflow(c.Request.Context(), workflowID, userID, format)
	if err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to export workflow", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export workflow"})
		return
	}
	
	c.JSON(http.StatusOK, data)
}

// Workflow statistics
func (h *WorkflowHandlers) GetWorkflowStats(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	stats, err := h.service.GetWorkflowStats(c.Request.Context(), workflowID, userID)
	if err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to get workflow stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get workflow stats"})
		return
	}
	
	c.JSON(http.StatusOK, stats)
}

func (h *WorkflowHandlers) GetWorkflowExecutions(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	
	executions, total, err := h.service.GetWorkflowExecutions(c.Request.Context(), workflowID, userID, page, limit)
	if err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to get workflow executions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get workflow executions"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"executions": executions,
		"total":      total,
		"page":       page,
		"limit":      limit,
	})
}

func (h *WorkflowHandlers) GetLatestRun(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	execution, err := h.service.GetLatestRun(c.Request.Context(), workflowID, userID)
	if err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to get latest run", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get latest run"})
		return
	}
	
	c.JSON(http.StatusOK, execution)
}

// Categories and tags
func (h *WorkflowHandlers) ListCategories(c *gin.Context) {
	categories, err := h.service.ListCategories(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list categories", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list categories"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

func (h *WorkflowHandlers) CreateCategory(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	category, err := h.service.CreateCategory(c.Request.Context(), req.Name, req.Description, req.Icon)
	if err != nil {
		h.logger.Error("Failed to create category", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create category"})
		return
	}
	
	c.JSON(http.StatusCreated, category)
}

func (h *WorkflowHandlers) SearchWorkflows(c *gin.Context) {
	userID := c.GetString("user_id")
	query := c.Query("q")
	category := c.Query("category")
	tags := c.QueryArray("tags")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	
	workflows, total, err := h.service.SearchWorkflows(c.Request.Context(), userID, query, category, tags, page, limit)
	if err != nil {
		h.logger.Error("Failed to search workflows", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search workflows"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"workflows": workflows,
		"total":     total,
		"page":      page,
		"limit":     limit,
	})
}

func (h *WorkflowHandlers) GetPopularTags(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	
	tags, err := h.service.GetPopularTags(c.Request.Context(), limit)
	if err != nil {
		h.logger.Error("Failed to get popular tags", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get popular tags"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

// Trigger handlers

// CreateTrigger creates a new trigger for a workflow
func (h *WorkflowHandlers) CreateTrigger(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	var config map[string]interface{}
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	trigger, err := h.service.CreateTrigger(c.Request.Context(), workflowID, userID, config)
	if err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to create trigger", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create trigger"})
		return
	}
	
	c.JSON(http.StatusCreated, trigger)
}

// ListTriggers lists all triggers for a workflow
func (h *WorkflowHandlers) ListTriggers(c *gin.Context) {
	workflowID := c.Param("id")
	userID := c.GetString("user_id")
	
	triggers, err := h.service.ListTriggers(c.Request.Context(), workflowID, userID)
	if err != nil {
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
			return
		}
		h.logger.Error("Failed to list triggers", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list triggers"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"triggers": triggers})
}

// GetTrigger gets a specific trigger
func (h *WorkflowHandlers) GetTrigger(c *gin.Context) {
	triggerID := c.Param("triggerId")
	userID := c.GetString("user_id")
	
	trigger, err := h.service.GetTrigger(c.Request.Context(), triggerID, userID)
	if err != nil {
		if err == service.ErrUnauthorized {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}
		h.logger.Error("Failed to get trigger", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trigger"})
		return
	}
	
	c.JSON(http.StatusOK, trigger)
}

// UpdateTrigger updates a trigger
func (h *WorkflowHandlers) UpdateTrigger(c *gin.Context) {
	triggerID := c.Param("triggerId")
	userID := c.GetString("user_id")
	
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	trigger, err := h.service.UpdateTrigger(c.Request.Context(), triggerID, userID, updates)
	if err != nil {
		if err == service.ErrUnauthorized {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}
		h.logger.Error("Failed to update trigger", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update trigger"})
		return
	}
	
	c.JSON(http.StatusOK, trigger)
}

// DeleteTrigger deletes a trigger
func (h *WorkflowHandlers) DeleteTrigger(c *gin.Context) {
	triggerID := c.Param("triggerId")
	userID := c.GetString("user_id")
	
	if err := h.service.DeleteTrigger(c.Request.Context(), triggerID, userID); err != nil {
		if err == service.ErrUnauthorized {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}
		h.logger.Error("Failed to delete trigger", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete trigger"})
		return
	}
	
	c.Status(http.StatusNoContent)
}

// ActivateTrigger activates a trigger
func (h *WorkflowHandlers) ActivateTrigger(c *gin.Context) {
	triggerID := c.Param("triggerId")
	userID := c.GetString("user_id")
	
	if err := h.service.ActivateTrigger(c.Request.Context(), triggerID, userID); err != nil {
		if err == service.ErrUnauthorized {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}
		if err == service.ErrWorkflowInactive {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Workflow is not active"})
			return
		}
		h.logger.Error("Failed to activate trigger", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to activate trigger"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Trigger activated"})
}

// DeactivateTrigger deactivates a trigger
func (h *WorkflowHandlers) DeactivateTrigger(c *gin.Context) {
	triggerID := c.Param("triggerId")
	userID := c.GetString("user_id")
	
	if err := h.service.DeactivateTrigger(c.Request.Context(), triggerID, userID); err != nil {
		if err == service.ErrUnauthorized {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}
		h.logger.Error("Failed to deactivate trigger", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deactivate trigger"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Trigger deactivated"})
}

// TestTrigger tests a trigger with sample data
func (h *WorkflowHandlers) TestTrigger(c *gin.Context) {
	triggerID := c.Param("triggerId")
	userID := c.GetString("user_id")
	
	var testData map[string]interface{}
	if err := c.ShouldBindJSON(&testData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	result, err := h.service.TestTrigger(c.Request.Context(), triggerID, userID, testData)
	if err != nil {
		if err == service.ErrUnauthorized {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}
		h.logger.Error("Failed to test trigger", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to test trigger"})
		return
	}
	
	c.JSON(http.StatusOK, result)
}
