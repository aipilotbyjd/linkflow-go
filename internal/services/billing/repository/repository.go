package repository

import (
	"context"
	"time"

	"github.com/linkflow-go/internal/domain/billing"
	"github.com/linkflow-go/pkg/database"
)

type BillingRepository struct {
	db *database.DB
}

func NewBillingRepository(db *database.DB) *BillingRepository {
	return &BillingRepository{db: db}
}

// Customer operations

func (r *BillingRepository) CreateCustomer(ctx context.Context, customer *billing.Customer) error {
	return r.db.WithContext(ctx).Create(customer).Error
}

func (r *BillingRepository) GetCustomer(ctx context.Context, id string) (*billing.Customer, error) {
	var customer billing.Customer
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&customer).Error
	if err != nil {
		return nil, billing.ErrCustomerNotFound
	}
	return &customer, nil
}

func (r *BillingRepository) GetCustomerByUserID(ctx context.Context, userID string) (*billing.Customer, error) {
	var customer billing.Customer
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&customer).Error
	if err != nil {
		return nil, billing.ErrCustomerNotFound
	}
	return &customer, nil
}

func (r *BillingRepository) GetCustomerByStripeID(ctx context.Context, stripeID string) (*billing.Customer, error) {
	var customer billing.Customer
	err := r.db.WithContext(ctx).Where("stripe_id = ?", stripeID).First(&customer).Error
	if err != nil {
		return nil, billing.ErrCustomerNotFound
	}
	return &customer, nil
}

func (r *BillingRepository) UpdateCustomer(ctx context.Context, customer *billing.Customer) error {
	customer.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(customer).Error
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

func (r *BillingRepository) GetPlanByStripeID(ctx context.Context, stripeID string) (*billing.Plan, error) {
	var plan billing.Plan
	err := r.db.WithContext(ctx).Where("stripe_id = ?", stripeID).First(&plan).Error
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
	err := query.Order("price ASC").Find(&plans).Error
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

func (r *BillingRepository) GetSubscriptionByStripeID(ctx context.Context, stripeID string) (*billing.Subscription, error) {
	var subscription billing.Subscription
	err := r.db.WithContext(ctx).Where("stripe_id = ?", stripeID).First(&subscription).Error
	if err != nil {
		return nil, billing.ErrSubscriptionNotFound
	}
	return &subscription, nil
}

func (r *BillingRepository) GetActiveSubscription(ctx context.Context, customerID string) (*billing.Subscription, error) {
	var subscription billing.Subscription
	err := r.db.WithContext(ctx).
		Where("customer_id = ? AND status IN ?", customerID, []string{billing.SubscriptionStatusActive, billing.SubscriptionStatusTrialing}).
		First(&subscription).Error
	if err != nil {
		return nil, billing.ErrSubscriptionNotFound
	}
	return &subscription, nil
}

func (r *BillingRepository) ListSubscriptions(ctx context.Context, customerID string) ([]*billing.Subscription, error) {
	var subscriptions []*billing.Subscription
	err := r.db.WithContext(ctx).
		Where("customer_id = ?", customerID).
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

func (r *BillingRepository) GetInvoiceByStripeID(ctx context.Context, stripeID string) (*billing.Invoice, error) {
	var invoice billing.Invoice
	err := r.db.WithContext(ctx).Where("stripe_id = ?", stripeID).First(&invoice).Error
	if err != nil {
		return nil, billing.ErrInvoiceNotFound
	}
	return &invoice, nil
}

func (r *BillingRepository) ListInvoices(ctx context.Context, customerID string, limit int) ([]*billing.Invoice, error) {
	var invoices []*billing.Invoice
	query := r.db.WithContext(ctx).
		Where("customer_id = ?", customerID).
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

func (r *BillingRepository) ListPaymentMethods(ctx context.Context, customerID string) ([]*billing.PaymentMethod, error) {
	var methods []*billing.PaymentMethod
	err := r.db.WithContext(ctx).
		Where("customer_id = ?", customerID).
		Order("is_default DESC, created_at DESC").
		Find(&methods).Error
	return methods, err
}

func (r *BillingRepository) SetDefaultPaymentMethod(ctx context.Context, customerID, paymentMethodID string) error {
	// Unset all defaults
	if err := r.db.WithContext(ctx).
		Model(&billing.PaymentMethod{}).
		Where("customer_id = ?", customerID).
		Update("is_default", false).Error; err != nil {
		return err
	}
	// Set new default
	return r.db.WithContext(ctx).
		Model(&billing.PaymentMethod{}).
		Where("id = ? AND customer_id = ?", paymentMethodID, customerID).
		Update("is_default", true).Error
}

func (r *BillingRepository) DeletePaymentMethod(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&billing.PaymentMethod{}).Error
}

// Usage operations

func (r *BillingRepository) RecordUsage(ctx context.Context, usage *billing.Usage) error {
	return r.db.WithContext(ctx).Create(usage).Error
}

func (r *BillingRepository) GetUsageSummary(ctx context.Context, customerID string, periodStart, periodEnd time.Time) (map[string]int64, error) {
	type result struct {
		Metric   string
		Total    int64
	}
	var results []result
	
	err := r.db.WithContext(ctx).
		Model(&billing.Usage{}).
		Select("metric, SUM(quantity) as total").
		Where("customer_id = ? AND timestamp >= ? AND timestamp < ?", customerID, periodStart, periodEnd).
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

func (r *BillingRepository) GetUsageByMetric(ctx context.Context, customerID, metric string, periodStart, periodEnd time.Time) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&billing.Usage{}).
		Select("COALESCE(SUM(quantity), 0)").
		Where("customer_id = ? AND metric = ? AND timestamp >= ? AND timestamp < ?", customerID, metric, periodStart, periodEnd).
		Scan(&total).Error
	return total, err
}

// Credit operations

func (r *BillingRepository) CreateCredit(ctx context.Context, credit *billing.Credit) error {
	return r.db.WithContext(ctx).Create(credit).Error
}

func (r *BillingRepository) GetAvailableCredits(ctx context.Context, customerID string) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&billing.Credit{}).
		Select("COALESCE(SUM(amount - used_amount), 0)").
		Where("customer_id = ? AND (expires_at IS NULL OR expires_at > ?)", customerID, time.Now()).
		Scan(&total).Error
	return total, err
}

// PromoCode operations

func (r *BillingRepository) GetPromoCode(ctx context.Context, code string) (*billing.PromoCode, error) {
	var promo billing.PromoCode
	err := r.db.WithContext(ctx).
		Where("code = ? AND is_active = ?", code, true).
		First(&promo).Error
	if err != nil {
		return nil, err
	}
	return &promo, nil
}

func (r *BillingRepository) IncrementPromoRedemptions(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Model(&billing.PromoCode{}).
		Where("id = ?", id).
		UpdateColumn("times_redeemed", r.db.Raw("times_redeemed + 1")).Error
}
