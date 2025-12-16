package repository

import (
	"context"
	"time"

	billing "github.com/linkflow-go/internal/billing/domain"
	"github.com/linkflow-go/pkg/database"
)

type BillingRepository struct {
	db *database.DB
}

func NewBillingRepository(db *database.DB) *BillingRepository {
	return &BillingRepository{db: db}
}

// Plan operations

func (r *BillingRepository) CreatePlan(ctx context.Context, plan *billing.Plan) error {
	return r.db.WithContext(ctx).Create(plan).Error
}

func (r *BillingRepository) GetPlan(ctx context.Context, id string) (*billing.Plan, error) {
	var plan billing.Plan
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&plan).Error
	if err != nil {
		return nil, billing.ErrPlanNotFound
	}
	return &plan, nil
}

func (r *BillingRepository) GetPlanBySlug(ctx context.Context, slug string) (*billing.Plan, error) {
	var plan billing.Plan
	err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&plan).Error
	if err != nil {
		return nil, billing.ErrPlanNotFound
	}
	return &plan, nil
}

func (r *BillingRepository) ListPlans(ctx context.Context, activeOnly bool) ([]*billing.Plan, error) {
	var plans []*billing.Plan
	query := r.db.WithContext(ctx)
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}
	err := query.Order("sort_order ASC").Find(&plans).Error
	return plans, err
}

func (r *BillingRepository) UpdatePlan(ctx context.Context, plan *billing.Plan) error {
	plan.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(plan).Error
}

func (r *BillingRepository) DeletePlan(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&billing.Plan{}).Error
}

// Subscription operations

func (r *BillingRepository) CreateSubscription(ctx context.Context, subscription *billing.Subscription) error {
	return r.db.WithContext(ctx).Create(subscription).Error
}

func (r *BillingRepository) GetSubscription(ctx context.Context, id string) (*billing.Subscription, error) {
	var subscription billing.Subscription
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&subscription).Error
	if err != nil {
		return nil, billing.ErrSubscriptionNotFound
	}
	return &subscription, nil
}

func (r *BillingRepository) GetSubscriptionByProviderID(ctx context.Context, providerID string) (*billing.Subscription, error) {
	var subscription billing.Subscription
	err := r.db.WithContext(ctx).Where("provider_subscription_id = ?", providerID).First(&subscription).Error
	if err != nil {
		return nil, billing.ErrSubscriptionNotFound
	}
	return &subscription, nil
}

func (r *BillingRepository) GetActiveSubscription(ctx context.Context, userID string) (*billing.Subscription, error) {
	var subscription billing.Subscription
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND status IN ?", userID, []string{billing.SubscriptionStatusActive, billing.SubscriptionStatusTrialing}).
		First(&subscription).Error
	if err != nil {
		return nil, billing.ErrSubscriptionNotFound
	}
	return &subscription, nil
}

func (r *BillingRepository) ListSubscriptions(ctx context.Context, userID string) ([]*billing.Subscription, error) {
	var subscriptions []*billing.Subscription
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&subscriptions).Error
	return subscriptions, err
}

func (r *BillingRepository) UpdateSubscription(ctx context.Context, subscription *billing.Subscription) error {
	subscription.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(subscription).Error
}

// Invoice operations

func (r *BillingRepository) CreateInvoice(ctx context.Context, invoice *billing.Invoice) error {
	return r.db.WithContext(ctx).Create(invoice).Error
}

func (r *BillingRepository) GetInvoice(ctx context.Context, id string) (*billing.Invoice, error) {
	var invoice billing.Invoice
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&invoice).Error
	if err != nil {
		return nil, billing.ErrInvoiceNotFound
	}
	return &invoice, nil
}

func (r *BillingRepository) GetInvoiceByNumber(ctx context.Context, invoiceNumber string) (*billing.Invoice, error) {
	var invoice billing.Invoice
	err := r.db.WithContext(ctx).Where("invoice_number = ?", invoiceNumber).First(&invoice).Error
	if err != nil {
		return nil, billing.ErrInvoiceNotFound
	}
	return &invoice, nil
}

func (r *BillingRepository) ListInvoices(ctx context.Context, userID string, limit int) ([]*billing.Invoice, error) {
	var invoices []*billing.Invoice
	query := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&invoices).Error
	return invoices, err
}

func (r *BillingRepository) UpdateInvoice(ctx context.Context, invoice *billing.Invoice) error {
	invoice.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(invoice).Error
}

// PaymentMethod operations

func (r *BillingRepository) CreatePaymentMethod(ctx context.Context, pm *billing.PaymentMethod) error {
	return r.db.WithContext(ctx).Create(pm).Error
}

func (r *BillingRepository) GetPaymentMethod(ctx context.Context, id string) (*billing.PaymentMethod, error) {
	var pm billing.PaymentMethod
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&pm).Error
	if err != nil {
		return nil, billing.ErrPaymentMethodNotFound
	}
	return &pm, nil
}

func (r *BillingRepository) ListPaymentMethods(ctx context.Context, userID string) ([]*billing.PaymentMethod, error) {
	var methods []*billing.PaymentMethod
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("is_default DESC, created_at DESC").
		Find(&methods).Error
	return methods, err
}

func (r *BillingRepository) SetDefaultPaymentMethod(ctx context.Context, userID, paymentMethodID string) error {
	// Unset all defaults
	if err := r.db.WithContext(ctx).
		Model(&billing.PaymentMethod{}).
		Where("user_id = ?", userID).
		Update("is_default", false).Error; err != nil {
		return err
	}
	// Set new default
	return r.db.WithContext(ctx).
		Model(&billing.PaymentMethod{}).
		Where("id = ? AND user_id = ?", paymentMethodID, userID).
		Update("is_default", true).Error
}

func (r *BillingRepository) DeletePaymentMethod(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&billing.PaymentMethod{}).Error
}

// Usage operations

func (r *BillingRepository) RecordUsage(ctx context.Context, usage *billing.Usage) error {
	return r.db.WithContext(ctx).Create(usage).Error
}

func (r *BillingRepository) GetUsageSummary(ctx context.Context, subscriptionID string, periodStart, periodEnd time.Time) (map[string]int64, error) {
	type result struct {
		Metric string
		Total  int64
	}
	var results []result

	err := r.db.WithContext(ctx).
		Model(&billing.Usage{}).
		Select("metric, SUM(quantity) as total").
		Where("subscription_id = ? AND period_start >= ? AND period_end <= ?", subscriptionID, periodStart, periodEnd).
		Group("metric").
		Find(&results).Error

	if err != nil {
		return nil, err
	}

	summary := make(map[string]int64)
	for _, r := range results {
		summary[r.Metric] = r.Total
	}
	return summary, nil
}

func (r *BillingRepository) GetUsageByMetric(ctx context.Context, subscriptionID, metric string, periodStart, periodEnd time.Time) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&billing.Usage{}).
		Select("COALESCE(SUM(quantity), 0)").
		Where("subscription_id = ? AND metric = ? AND period_start >= ? AND period_end <= ?", subscriptionID, metric, periodStart, periodEnd).
		Scan(&total).Error
	return total, err
}

// Coupon operations

func (r *BillingRepository) GetCoupon(ctx context.Context, code string) (*billing.Coupon, error) {
	var coupon billing.Coupon
	err := r.db.WithContext(ctx).
		Where("code = ? AND is_active = ?", code, true).
		First(&coupon).Error
	if err != nil {
		return nil, err
	}
	return &coupon, nil
}

func (r *BillingRepository) IncrementCouponRedemptions(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Model(&billing.Coupon{}).
		Where("id = ?", id).
		UpdateColumn("redemption_count", r.db.Raw("redemption_count + 1")).Error
}
