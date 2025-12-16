package billing

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errors
var (
	ErrSubscriptionNotFound  = errors.New("subscription not found")
	ErrPlanNotFound          = errors.New("plan not found")
	ErrInvoiceNotFound       = errors.New("invoice not found")
	ErrPaymentMethodNotFound = errors.New("payment method not found")
	ErrInvalidPlan           = errors.New("invalid plan")
	ErrSubscriptionCancelled = errors.New("subscription is cancelled")
	ErrPaymentFailed         = errors.New("payment failed")
)

// Subscription statuses
const (
	SubscriptionStatusActive    = "active"
	SubscriptionStatusTrialing  = "trialing"
	SubscriptionStatusPastDue   = "past_due"
	SubscriptionStatusCancelled = "cancelled"
	SubscriptionStatusPaused    = "paused"
)

// Plan intervals
const (
	IntervalMonthly = "monthly"
	IntervalYearly  = "yearly"
)

// Invoice statuses
const (
	InvoiceStatusDraft         = "draft"
	InvoiceStatusOpen          = "open"
	InvoiceStatusPaid          = "paid"
	InvoiceStatusVoid          = "void"
	InvoiceStatusUncollectible = "uncollectible"
)

// Plan represents a subscription plan
type Plan struct {
	ID              string                 `json:"id" gorm:"primaryKey"`
	Name            string                 `json:"name" gorm:"not null"`
	Slug            string                 `json:"slug" gorm:"uniqueIndex;not null"`
	Description     string                 `json:"description"`
	PriceMonthly    float64                `json:"priceMonthly" gorm:"column:price_monthly"`
	PriceYearly     float64                `json:"priceYearly" gorm:"column:price_yearly"`
	Currency        string                 `json:"currency" gorm:"default:'USD'"`
	Features        map[string]interface{} `json:"features" gorm:"serializer:json"`
	IsActive        bool                   `json:"isActive" gorm:"column:is_active;default:true"`
	IsPublic        bool                   `json:"isPublic" gorm:"column:is_public;default:true"`
	SortOrder       int                    `json:"sortOrder" gorm:"column:sort_order;default:0"`
	MaxWorkflows    int                    `json:"maxWorkflows" gorm:"column:max_workflows"`
	MaxExecutions   int                    `json:"maxExecutions" gorm:"column:max_executions_month"`
	MaxTeamMembers  int                    `json:"maxTeamMembers" gorm:"column:max_team_members"`
	MaxStorageBytes int64                  `json:"maxStorageBytes" gorm:"column:max_storage_bytes"`
	CreatedAt       time.Time              `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt       time.Time              `json:"updatedAt" gorm:"column:updated_at"`
}

// TableName specifies the table name for GORM
func (Plan) TableName() string {
	return "billing.plans"
}

type PlanLimits struct {
	Workflows      int `json:"workflows"`  // -1 for unlimited
	Executions     int `json:"executions"` // per month
	TeamMembers    int `json:"teamMembers"`
	StorageGB      int `json:"storageGb"`
	APIRequests    int `json:"apiRequests"` // per month
	WebhooksPerDay int `json:"webhooksPerDay"`
}

func NewPlan(name, slug string, priceMonthly float64) *Plan {
	return &Plan{
		ID:           uuid.New().String(),
		Name:         name,
		Slug:         slug,
		PriceMonthly: priceMonthly,
		Currency:     "USD",
		Features:     make(map[string]interface{}),
		IsActive:     true,
		IsPublic:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// Subscription represents a customer subscription
type Subscription struct {
	ID                     string                 `json:"id" gorm:"primaryKey"`
	UserID                 string                 `json:"userId" gorm:"column:user_id;not null;index"`
	TeamID                 *string                `json:"teamId" gorm:"column:team_id;index"`
	PlanID                 string                 `json:"planId" gorm:"column:plan_id;not null;index"`
	Status                 string                 `json:"status" gorm:"not null"`
	BillingCycle           string                 `json:"billingCycle" gorm:"column:billing_cycle;default:'monthly'"`
	CurrentPeriodStart     time.Time              `json:"currentPeriodStart" gorm:"column:current_period_start"`
	CurrentPeriodEnd       time.Time              `json:"currentPeriodEnd" gorm:"column:current_period_end"`
	TrialEndsAt            *time.Time             `json:"trialEndsAt" gorm:"column:trial_ends_at"`
	CanceledAt             *time.Time             `json:"canceledAt" gorm:"column:canceled_at"`
	Provider               string                 `json:"provider"`
	ProviderSubscriptionID string                 `json:"providerSubscriptionId" gorm:"column:provider_subscription_id"`
	ProviderCustomerID     string                 `json:"providerCustomerId" gorm:"column:provider_customer_id"`
	Metadata               map[string]interface{} `json:"metadata" gorm:"serializer:json"`
	CreatedAt              time.Time              `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt              time.Time              `json:"updatedAt" gorm:"column:updated_at"`
}

// TableName specifies the table name for GORM
func (Subscription) TableName() string {
	return "billing.subscriptions"
}

func NewSubscription(userID, planID string) *Subscription {
	now := time.Now()
	return &Subscription{
		ID:                 uuid.New().String(),
		UserID:             userID,
		PlanID:             planID,
		Status:             SubscriptionStatusActive,
		BillingCycle:       IntervalMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		Metadata:           make(map[string]interface{}),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func (s *Subscription) IsActive() bool {
	return s.Status == SubscriptionStatusActive || s.Status == SubscriptionStatusTrialing
}

func (s *Subscription) Cancel(atPeriodEnd bool) {
	now := time.Now()
	if !atPeriodEnd {
		s.Status = SubscriptionStatusCancelled
		s.CanceledAt = &now
	}
	s.UpdatedAt = now
}

// Invoice represents a billing invoice
type Invoice struct {
	ID                string        `json:"id" gorm:"primaryKey"`
	SubscriptionID    string        `json:"subscriptionId" gorm:"column:subscription_id;not null;index"`
	UserID            string        `json:"userId" gorm:"column:user_id;not null;index"`
	InvoiceNumber     string        `json:"invoiceNumber" gorm:"column:invoice_number;uniqueIndex;not null"`
	Status            string        `json:"status" gorm:"not null"`
	Currency          string        `json:"currency" gorm:"default:'USD'"`
	Subtotal          float64       `json:"subtotal"`
	Tax               float64       `json:"tax"`
	Discount          float64       `json:"discount"`
	Total             float64       `json:"total"`
	LineItems         []InvoiceLine `json:"lineItems" gorm:"column:line_items;serializer:json"`
	DueDate           *time.Time    `json:"dueDate" gorm:"column:due_date"`
	PaidAt            *time.Time    `json:"paidAt" gorm:"column:paid_at"`
	ProviderInvoiceID string        `json:"providerInvoiceId" gorm:"column:provider_invoice_id"`
	PDFURL            string        `json:"pdfUrl" gorm:"column:pdf_url"`
	CreatedAt         time.Time     `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt         time.Time     `json:"updatedAt" gorm:"column:updated_at"`
}

// TableName specifies the table name for GORM
func (Invoice) TableName() string {
	return "billing.invoices"
}

type InvoiceLine struct {
	Description string  `json:"description"`
	Quantity    int     `json:"quantity"`
	UnitAmount  float64 `json:"unitAmount"`
	Amount      float64 `json:"amount"`
}

func NewInvoice(subscriptionID, userID string, total float64) *Invoice {
	return &Invoice{
		ID:             uuid.New().String(),
		SubscriptionID: subscriptionID,
		UserID:         userID,
		Status:         InvoiceStatusDraft,
		Currency:       "USD",
		Total:          total,
		LineItems:      []InvoiceLine{},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// PaymentMethod represents a customer payment method
type PaymentMethod struct {
	ID               string    `json:"id" gorm:"primaryKey"`
	UserID           string    `json:"userId" gorm:"column:user_id;not null;index"`
	Type             string    `json:"type" gorm:"not null"` // card, bank_account, paypal
	CardBrand        string    `json:"cardBrand" gorm:"column:card_brand"`
	CardLast4        string    `json:"cardLast4" gorm:"column:card_last4"`
	CardExpMonth     int       `json:"cardExpMonth" gorm:"column:card_exp_month"`
	CardExpYear      int       `json:"cardExpYear" gorm:"column:card_exp_year"`
	IsDefault        bool      `json:"isDefault" gorm:"column:is_default;default:false"`
	Provider         string    `json:"provider"`
	ProviderMethodID string    `json:"providerMethodId" gorm:"column:provider_method_id"`
	CreatedAt        time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt        time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

// TableName specifies the table name for GORM
func (PaymentMethod) TableName() string {
	return "billing.payment_methods"
}

// Usage represents usage tracking for metered billing
type Usage struct {
	ID             string                 `json:"id" gorm:"primaryKey"`
	SubscriptionID string                 `json:"subscriptionId" gorm:"column:subscription_id;not null;index"`
	Metric         string                 `json:"metric" gorm:"not null;index"` // executions, api_calls, storage
	Quantity       int64                  `json:"quantity"`
	PeriodStart    time.Time              `json:"periodStart" gorm:"column:period_start"`
	PeriodEnd      time.Time              `json:"periodEnd" gorm:"column:period_end"`
	Metadata       map[string]interface{} `json:"metadata" gorm:"serializer:json"`
	CreatedAt      time.Time              `json:"createdAt" gorm:"column:created_at"`
}

// TableName specifies the table name for GORM
func (Usage) TableName() string {
	return "billing.usage_records"
}

func NewUsage(subscriptionID, metric string, quantity int64) *Usage {
	now := time.Now()
	return &Usage{
		ID:             uuid.New().String(),
		SubscriptionID: subscriptionID,
		Metric:         metric,
		Quantity:       quantity,
		PeriodStart:    time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()),
		PeriodEnd:      time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location()),
		CreatedAt:      now,
	}
}

// Coupon represents a promotional coupon
type Coupon struct {
	ID              string     `json:"id" gorm:"primaryKey"`
	Code            string     `json:"code" gorm:"uniqueIndex;not null"`
	Name            string     `json:"name"`
	DiscountType    string     `json:"discountType" gorm:"column:discount_type"` // percent, fixed
	DiscountValue   float64    `json:"discountValue" gorm:"column:discount_value"`
	Currency        string     `json:"currency" gorm:"default:'USD'"`
	MaxRedemptions  *int       `json:"maxRedemptions" gorm:"column:max_redemptions"`
	RedemptionCount int        `json:"redemptionCount" gorm:"column:redemption_count;default:0"`
	ValidFrom       time.Time  `json:"validFrom" gorm:"column:valid_from"`
	ValidUntil      *time.Time `json:"validUntil" gorm:"column:valid_until"`
	IsActive        bool       `json:"isActive" gorm:"column:is_active;default:true"`
	CreatedAt       time.Time  `json:"createdAt" gorm:"column:created_at"`
}

// TableName specifies the table name for GORM
func (Coupon) TableName() string {
	return "billing.coupons"
}
