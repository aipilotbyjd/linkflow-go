package handlers

import (
"net/http"

"github.com/gin-gonic/gin"
"github.com/linkflow-go/internal/services/billing/service"
"github.com/linkflow-go/pkg/logger"
)

type BillingHandlers struct {
	service *service.BillingService
	logger  logger.Logger
}

func NewBillingHandlers(service *service.BillingService, logger logger.Logger) *BillingHandlers {
	return &BillingHandlers{
		service: service,
		logger:  logger,
	}
}

func (h *BillingHandlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func (h *BillingHandlers) Ready(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

// Plan handlers

func (h *BillingHandlers) ListPlans(c *gin.Context) {
	activeOnly := c.Query("active") != "false"

	plans, err := h.service.ListPlans(c.Request.Context(), activeOnly)
	if err != nil {
		h.logger.Error("Failed to list plans", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list plans"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"plans": plans})
}

func (h *BillingHandlers) GetPlan(c *gin.Context) {
	id := c.Param("id")

	plan, err := h.service.GetPlan(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plan not found"})
		return
	}

	c.JSON(http.StatusOK, plan)
}

// Subscription handlers

func (h *BillingHandlers) GetSubscriptions(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	subscriptions, err := h.service.ListSubscriptions(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to list subscriptions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list subscriptions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"subscriptions": subscriptions})
}

func (h *BillingHandlers) GetSubscription(c *gin.Context) {
	id := c.Param("id")

	subscription, err := h.service.GetSubscription(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}

	c.JSON(http.StatusOK, subscription)
}

// Invoice handlers

func (h *BillingHandlers) ListInvoices(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	invoices, err := h.service.ListInvoices(c.Request.Context(), userID, 50)
	if err != nil {
		h.logger.Error("Failed to list invoices", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list invoices"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"invoices": invoices})
}

func (h *BillingHandlers) GetInvoice(c *gin.Context) {
	id := c.Param("id")

	invoice, err := h.service.GetInvoice(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invoice not found"})
		return
	}

	c.JSON(http.StatusOK, invoice)
}

// Payment Method handlers

func (h *BillingHandlers) ListPaymentMethods(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	methods, err := h.service.ListPaymentMethods(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to list payment methods", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list payment methods"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"paymentMethods": methods})
}

// Coupon handlers

func (h *BillingHandlers) GetCoupon(c *gin.Context) {
	code := c.Param("code")

	coupon, err := h.service.GetCoupon(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "coupon not found"})
		return
	}

	c.JSON(http.StatusOK, coupon)
}
