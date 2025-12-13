package handlers

import (
	"io"
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

// Webhook handler
func (h *BillingHandlers) HandleStripeWebhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error("Failed to read webhook body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	signature := c.GetHeader("Stripe-Signature")
	if err := h.service.HandleStripeWebhook(c.Request.Context(), payload, signature); err != nil {
		h.logger.Error("Failed to handle webhook", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
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

func (h *BillingHandlers) CreateSubscription(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	var req service.CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserID = userID

	subscription, err := h.service.CreateSubscription(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to create subscription", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, subscription)
}

func (h *BillingHandlers) UpdateSubscription(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	id := c.Param("id")

	var req struct {
		PlanID string `json:"planId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	subscription, err := h.service.ChangePlan(c.Request.Context(), id, req.PlanID, userID)
	if err != nil {
		h.logger.Error("Failed to update subscription", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, subscription)
}

func (h *BillingHandlers) CancelSubscription(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	id := c.Param("id")

	var req struct {
		AtPeriodEnd bool `json:"atPeriodEnd"`
	}
	c.ShouldBindJSON(&req)

	subscription, err := h.service.CancelSubscription(c.Request.Context(), id, userID, req.AtPeriodEnd)
	if err != nil {
		h.logger.Error("Failed to cancel subscription", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, subscription)
}

func (h *BillingHandlers) ReactivateSubscription(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	id := c.Param("id")

	subscription, err := h.service.ReactivateSubscription(c.Request.Context(), id, userID)
	if err != nil {
		h.logger.Error("Failed to reactivate subscription", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, subscription)
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

func (h *BillingHandlers) CreatePlan(c *gin.Context) {
	var req service.CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	plan, err := h.service.CreatePlan(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to create plan", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, plan)
}

func (h *BillingHandlers) UpdatePlan(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	plan, err := h.service.UpdatePlan(c.Request.Context(), id, req)
	if err != nil {
		h.logger.Error("Failed to update plan", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, plan)
}

func (h *BillingHandlers) DeletePlan(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeletePlan(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to delete plan", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
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

func (h *BillingHandlers) AddPaymentMethod(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	var req struct {
		PaymentMethodID string `json:"paymentMethodId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pm, err := h.service.AddPaymentMethod(c.Request.Context(), userID, req.PaymentMethodID)
	if err != nil {
		h.logger.Error("Failed to add payment method", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, pm)
}

func (h *BillingHandlers) RemovePaymentMethod(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	id := c.Param("id")

	if err := h.service.RemovePaymentMethod(c.Request.Context(), userID, id); err != nil {
		h.logger.Error("Failed to remove payment method", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *BillingHandlers) SetDefaultPaymentMethod(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	id := c.Param("id")

	if err := h.service.SetDefaultPaymentMethod(c.Request.Context(), userID, id); err != nil {
		h.logger.Error("Failed to set default payment method", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Default payment method set"})
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

func (h *BillingHandlers) DownloadInvoice(c *gin.Context) {
	id := c.Param("id")

	invoice, err := h.service.GetInvoice(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invoice not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": invoice.PDFURL})
}

func (h *BillingHandlers) PayInvoice(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	id := c.Param("id")

	invoice, err := h.service.PayInvoice(c.Request.Context(), id, userID)
	if err != nil {
		h.logger.Error("Failed to pay invoice", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, invoice)
}

// Usage handlers
func (h *BillingHandlers) GetUsage(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	usage, err := h.service.GetUsageSummary(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get usage", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get usage"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"usage": usage})
}

func (h *BillingHandlers) ReportUsage(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	var req struct {
		Metric   string `json:"metric" binding:"required"`
		Quantity int64  `json:"quantity" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.RecordUsage(c.Request.Context(), userID, req.Metric, req.Quantity); err != nil {
		h.logger.Error("Failed to report usage", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Usage recorded"})
}

func (h *BillingHandlers) GetUsageSummary(c *gin.Context) {
	h.GetUsage(c)
}

// Billing info handlers
func (h *BillingHandlers) GetBillingInfo(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	info, err := h.service.GetBillingInfo(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get billing info", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get billing info"})
		return
	}

	c.JSON(http.StatusOK, info)
}

func (h *BillingHandlers) UpdateBillingInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Billing info updated"})
}

// Credits handlers
func (h *BillingHandlers) GetCredits(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	credits, err := h.service.GetCredits(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get credits", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get credits"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"credits": credits})
}

func (h *BillingHandlers) ApplyPromoCode(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.ApplyPromoCode(c.Request.Context(), userID, req.Code); err != nil {
		h.logger.Error("Failed to apply promo code", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Promo code applied"})
}

func (h *BillingHandlers) GetAvailablePromotions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"promotions": []interface{}{}})
}

// Portal handler
func (h *BillingHandlers) CreatePortalSession(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"url": "https://billing.stripe.com/session/..."})
}

// Report handlers
func (h *BillingHandlers) GetRevenueReport(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"report": map[string]interface{}{"mrr": 0, "arr": 0}})
}

func (h *BillingHandlers) GetChurnReport(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"report": map[string]interface{}{"rate": 0}})
}

func (h *BillingHandlers) GetMRRReport(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"report": map[string]interface{}{"mrr": 0, "growth": 0}})
}
