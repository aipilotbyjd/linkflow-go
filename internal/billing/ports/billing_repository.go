package ports

import (
	"context"

	billing "github.com/linkflow-go/internal/billing/domain"
)

type BillingRepository interface {
	GetPlan(ctx context.Context, id string) (*billing.Plan, error)
	GetPlanBySlug(ctx context.Context, slug string) (*billing.Plan, error)
	ListPlans(ctx context.Context, activeOnly bool) ([]*billing.Plan, error)

	GetSubscription(ctx context.Context, id string) (*billing.Subscription, error)
	GetActiveSubscription(ctx context.Context, userID string) (*billing.Subscription, error)
	ListSubscriptions(ctx context.Context, userID string) ([]*billing.Subscription, error)

	GetInvoice(ctx context.Context, id string) (*billing.Invoice, error)
	ListInvoices(ctx context.Context, userID string, limit int) ([]*billing.Invoice, error)

	ListPaymentMethods(ctx context.Context, userID string) ([]*billing.PaymentMethod, error)
	RecordUsage(ctx context.Context, usage *billing.Usage) error
	GetCoupon(ctx context.Context, code string) (*billing.Coupon, error)
}
