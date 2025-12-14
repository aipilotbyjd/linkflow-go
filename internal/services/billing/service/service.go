// Package service provides billing service functionality.
package service

import (
"context"

"github.com/linkflow-go/internal/domain/billing"
"github.com/linkflow-go/internal/services/billing/repository"
"github.com/linkflow-go/pkg/events"
"github.com/linkflow-go/pkg/logger"
"github.com/redis/go-redis/v9"
)

// BillingService handles billing operations
type BillingService struct {
	repo     *repository.BillingRepository
	eventBus events.EventBus
	redis    *redis.Client
	logger   logger.Logger
}

// NewBillingService creates a new billing service
func NewBillingService(
repo *repository.BillingRepository,
eventBus events.EventBus,
redis *redis.Client,
logger logger.Logger,
) *BillingService {
	return &BillingService{
		repo:     repo,
		eventBus: eventBus,
		redis:    redis,
		logger:   logger,
	}
}

// GetPlan returns a plan by ID
func (s *BillingService) GetPlan(ctx context.Context, id string) (*billing.Plan, error) {
	return s.repo.GetPlan(ctx, id)
}

// GetPlanBySlug returns a plan by slug
func (s *BillingService) GetPlanBySlug(ctx context.Context, slug string) (*billing.Plan, error) {
	return s.repo.GetPlanBySlug(ctx, slug)
}

// ListPlans returns all plans
func (s *BillingService) ListPlans(ctx context.Context, activeOnly bool) ([]*billing.Plan, error) {
	return s.repo.ListPlans(ctx, activeOnly)
}

// GetSubscription returns a subscription by ID
func (s *BillingService) GetSubscription(ctx context.Context, id string) (*billing.Subscription, error) {
	return s.repo.GetSubscription(ctx, id)
}

// GetActiveSubscription returns the active subscription for a user
func (s *BillingService) GetActiveSubscription(ctx context.Context, userID string) (*billing.Subscription, error) {
	return s.repo.GetActiveSubscription(ctx, userID)
}

// ListSubscriptions returns all subscriptions for a user
func (s *BillingService) ListSubscriptions(ctx context.Context, userID string) ([]*billing.Subscription, error) {
	return s.repo.ListSubscriptions(ctx, userID)
}

// GetInvoice returns an invoice by ID
func (s *BillingService) GetInvoice(ctx context.Context, id string) (*billing.Invoice, error) {
	return s.repo.GetInvoice(ctx, id)
}

// ListInvoices returns invoices for a user
func (s *BillingService) ListInvoices(ctx context.Context, userID string, limit int) ([]*billing.Invoice, error) {
	return s.repo.ListInvoices(ctx, userID, limit)
}

// ListPaymentMethods returns payment methods for a user
func (s *BillingService) ListPaymentMethods(ctx context.Context, userID string) ([]*billing.PaymentMethod, error) {
	return s.repo.ListPaymentMethods(ctx, userID)
}

// RecordUsage records usage for a subscription
func (s *BillingService) RecordUsage(ctx context.Context, subscriptionID, metric string, quantity int64) error {
	usage := billing.NewUsage(subscriptionID, metric, quantity)
	return s.repo.RecordUsage(ctx, usage)
}

// GetCoupon returns a coupon by code
func (s *BillingService) GetCoupon(ctx context.Context, code string) (*billing.Coupon, error) {
	return s.repo.GetCoupon(ctx, code)
}
