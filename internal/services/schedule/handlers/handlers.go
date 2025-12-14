package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/domain/schedule"
	"github.com/linkflow-go/internal/services/schedule/service"
	"github.com/linkflow-go/pkg/logger"
)

type ScheduleHandlers struct {
	service *service.ScheduleService
	logger  logger.Logger
}

func NewScheduleHandlers(svc *service.ScheduleService, log logger.Logger) *ScheduleHandlers {
	return &ScheduleHandlers{
		service: svc,
		logger:  log,
	}
}

func (h *ScheduleHandlers) RegisterRoutes(r *gin.RouterGroup) {
	schedules := r.Group("/schedules")
	{
		schedules.GET("", h.ListSchedules)
		schedules.POST("", h.CreateSchedule)
		schedules.GET("/:id", h.GetSchedule)
		schedules.PUT("/:id", h.UpdateSchedule)
		schedules.DELETE("/:id", h.DeleteSchedule)
		schedules.POST("/:id/pause", h.PauseSchedule)
		schedules.POST("/:id/resume", h.ResumeSchedule)
		schedules.POST("/:id/trigger", h.TriggerSchedule)
	}
}

// ListSchedules returns all schedules for the user
func (h *ScheduleHandlers) ListSchedules(c *gin.Context) {
	userID := c.GetString("userId")
	workflowID := c.Query("workflowId")

	schedules, err := h.service.ListSchedules(c.Request.Context(), userID, workflowID)
	if err != nil {
		h.logger.Error("Failed to list schedules", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list schedules"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"schedules": schedules})
}

// CreateSchedule creates a new schedule
func (h *ScheduleHandlers) CreateSchedule(c *gin.Context) {
	userID := c.GetString("userId")

	var req CreateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sched := schedule.NewSchedule(req.Name, req.WorkflowID, userID, req.CronExpression)
	sched.Description = req.Description
	sched.Timezone = req.Timezone
	sched.Data = req.Data

	if err := h.service.CreateSchedule(c.Request.Context(), sched); err != nil {
		h.logger.Error("Failed to create schedule", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create schedule"})
		return
	}

	c.JSON(http.StatusCreated, sched)
}

// GetSchedule returns a schedule by ID
func (h *ScheduleHandlers) GetSchedule(c *gin.Context) {
	id := c.Param("id")

	sched, err := h.service.GetSchedule(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get schedule", "error", err, "id", id)
		c.JSON(http.StatusNotFound, gin.H{"error": "Schedule not found"})
		return
	}

	c.JSON(http.StatusOK, sched)
}

// UpdateSchedule updates a schedule
func (h *ScheduleHandlers) UpdateSchedule(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sched, err := h.service.UpdateSchedule(c.Request.Context(), id, &req)
	if err != nil {
		h.logger.Error("Failed to update schedule", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update schedule"})
		return
	}

	c.JSON(http.StatusOK, sched)
}

// DeleteSchedule deletes a schedule
func (h *ScheduleHandlers) DeleteSchedule(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteSchedule(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to delete schedule", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete schedule"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Schedule deleted"})
}

// PauseSchedule pauses a schedule
func (h *ScheduleHandlers) PauseSchedule(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.PauseSchedule(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to pause schedule", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to pause schedule"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Schedule paused"})
}

// ResumeSchedule resumes a schedule
func (h *ScheduleHandlers) ResumeSchedule(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.ResumeSchedule(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to resume schedule", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resume schedule"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Schedule resumed"})
}

// TriggerSchedule manually triggers a schedule
func (h *ScheduleHandlers) TriggerSchedule(c *gin.Context) {
	id := c.Param("id")

	executionID, err := h.service.TriggerSchedule(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Failed to trigger schedule", "error", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to trigger schedule"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"executionId": executionID})
}

// Request types
type CreateScheduleRequest struct {
	Name           string                 `json:"name" binding:"required"`
	Description    string                 `json:"description"`
	WorkflowID     string                 `json:"workflowId" binding:"required"`
	CronExpression string                 `json:"cronExpression" binding:"required"`
	Timezone       string                 `json:"timezone"`
	Data           map[string]interface{} `json:"data"`
}
