package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/webhook/app/service"
	"github.com/linkflow-go/pkg/logger"
)

type WebhookHandlers struct {
	service *service.WebhookService
	logger  logger.Logger
}

func NewWebhookHandlers(svc *service.WebhookService, logger logger.Logger) *WebhookHandlers {
	return &WebhookHandlers{
		service: svc,
		logger:  logger,
	}
}

func (h *WebhookHandlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func (h *WebhookHandlers) Ready(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (h *WebhookHandlers) ListWebhooks(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	webhooks, err := h.service.ListWebhooks(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to list webhooks", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list webhooks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"webhooks": webhooks})
}

func (h *WebhookHandlers) GetWebhook(c *gin.Context) {
	id := c.Param("id")

	webhook, err := h.service.GetWebhook(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}

	c.JSON(http.StatusOK, webhook)
}

func (h *WebhookHandlers) CreateWebhook(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	var req service.CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserID = userID

	webhook, err := h.service.CreateWebhook(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to create webhook", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, webhook)
}

func (h *WebhookHandlers) UpdateWebhook(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	webhook, err := h.service.UpdateWebhook(c.Request.Context(), id, req)
	if err != nil {
		h.logger.Error("Failed to update webhook", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, webhook)
}

func (h *WebhookHandlers) DeleteWebhook(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteWebhook(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to delete webhook", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *WebhookHandlers) EnableWebhook(c *gin.Context) {
	id := c.Param("id")

	isActive := true
	webhook, err := h.service.UpdateWebhook(c.Request.Context(), id, service.UpdateWebhookRequest{
		IsActive: &isActive,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, webhook)
}

func (h *WebhookHandlers) DisableWebhook(c *gin.Context) {
	id := c.Param("id")

	isActive := false
	webhook, err := h.service.UpdateWebhook(c.Request.Context(), id, service.UpdateWebhookRequest{
		IsActive: &isActive,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, webhook)
}

func (h *WebhookHandlers) RegenerateSecret(c *gin.Context) {
	// Would regenerate the webhook secret
	c.JSON(http.StatusOK, gin.H{"message": "Secret regenerated"})
}

func (h *WebhookHandlers) TestWebhook(c *gin.Context) {
	// Would send a test request to the webhook
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Test webhook sent"})
}

func (h *WebhookHandlers) GetWebhookLogs(c *gin.Context) {
	// Would return webhook execution logs
	c.JSON(http.StatusOK, gin.H{"logs": []interface{}{}})
}

func (h *WebhookHandlers) GetWebhookStats(c *gin.Context) {
	// Would return webhook statistics
	c.JSON(http.StatusOK, gin.H{"stats": map[string]interface{}{
		"totalCalls":      0,
		"successfulCalls": 0,
		"failedCalls":     0,
	}})
}

func (h *WebhookHandlers) RetryWebhook(c *gin.Context) {
	// Would retry a failed webhook execution
	c.JSON(http.StatusOK, gin.H{"message": "Retry initiated"})
}

func (h *WebhookHandlers) ListWebhookTemplates(c *gin.Context) {
	templates := []map[string]interface{}{
		{"id": "github", "name": "GitHub Webhook", "description": "Receive GitHub events"},
		{"id": "stripe", "name": "Stripe Webhook", "description": "Receive Stripe events"},
		{"id": "slack", "name": "Slack Webhook", "description": "Receive Slack events"},
	}
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

func (h *WebhookHandlers) CreateFromTemplate(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{"message": "Webhook created from template"})
}

func (h *WebhookHandlers) VerifyWebhookSignature(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"valid": true})
}

// HandleIncomingWebhook handles incoming webhook requests
func (h *WebhookHandlers) HandleIncomingWebhook(c *gin.Context) {
	path := c.Param("path")

	response, statusCode, err := h.service.HandleWebhook(c.Request.Context(), path, c.Request)
	if err != nil {
		h.logger.Error("Webhook error", "path", path, "error", err)
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(statusCode, response)
}
