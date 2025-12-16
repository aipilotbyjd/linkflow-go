package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/variable/app/service"
	variable "github.com/linkflow-go/internal/variable/domain"
	"github.com/linkflow-go/pkg/logger"
)

type VariableHandlers struct {
	service *service.VariableService
	logger  logger.Logger
}

func NewVariableHandlers(svc *service.VariableService, logger logger.Logger) *VariableHandlers {
	return &VariableHandlers{
		service: svc,
		logger:  logger,
	}
}

func (h *VariableHandlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func (h *VariableHandlers) Ready(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (h *VariableHandlers) List(c *gin.Context) {
	variables, err := h.service.List(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list variables", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list variables"})
		return
	}

	response := make([]variable.VariableResponse, len(variables))
	for i, v := range variables {
		response[i] = v.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{"data": response})
}

func (h *VariableHandlers) Get(c *gin.Context) {
	id := c.Param("id")

	v, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "variable not found"})
		return
	}

	c.JSON(http.StatusOK, v.ToResponse())
}

func (h *VariableHandlers) Create(c *gin.Context) {
	var req service.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	v, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to create variable", "error", err)
		status := http.StatusBadRequest
		if err == variable.ErrVariableExists {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, v.ToResponse())
}

func (h *VariableHandlers) Update(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	v, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		h.logger.Error("Failed to update variable", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, v.ToResponse())
}

func (h *VariableHandlers) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to delete variable", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
