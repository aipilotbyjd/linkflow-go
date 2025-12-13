package billing

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errors
var (
	ErrSubscriptionNotFound   = errors.New("subscription not found")
	ErrPlanNotFound           = errors.New("plan not found")
	ErrInvoiceNotFound        = errors.New("invoice not found")
	ErrPaymentMethodNotFound  = errors.New("payment method not found")
	ErrCustomerNotFound       = errors.New("customer not found")
	ErrInvalidPlan            = errors.New("invalid plan")
	ErrSubscriptionCancelled  = errors.New("subscription is cancelled")
	ErrPaymentFailed          = errors.New("payment failed")
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
	InvoiceStatusDraft     = "draft"
	InvoiceStatusOpen      = "open"
	InvoiceStatusPaid      = "paid"
	InvoiceStatusVoid      = "void"
	InvoiceStatusUncollectible = "uncollectible"
)

// Customer represents a billing customer
type Customer struct {
	ID              string     `json:"id" gorm:"primaryKey"`
	UserID          string     `json:"userId" gorm:"uniqueIndex;not null"`
	StripeID        string     `json:"stripeId" gorm:"uniqueIndex"`
	Email           string     `json:"email" gorm:"not null"`
	Name            string     `json:"name"`
	DefaultPaymentID string    `json:"defaultPaymentId"`
	Balance         int64      `json:"balance"` // in cents
	Currency        string     `json:"currency" gorm:"default:'usd'"`
	Metadata        map[string]string `json:"metadata" gorm:"serializer:json"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func NewCustomer(userID, email string) *Customer {
	return &Customer{
		ID:        uuid.New().String(),
		UserID:    userID,
		Email:     email,
		Currency:  "usd",
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Plan represents a subscription plan
type Plan struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	StripeID     string    `json:"stripeId" gorm:"uniqueIndex"`
	Name         string    `json:"name" gorm:"not null"`
	Description  string    `json:"description"`
	Interval     string    `json:"interval" gorm:"not null"` // monthly, yearly
	Price        int64     `json:"price" gorm:"not null"`    // in cents
	Currency     string    `json:"currency" gorm:"default:'usd'"`
	Features     []string  `json:"features" gorm:"serializer:json"`
	Limits       PlanLimits `json:"limits" gorm:"serializer:json"`
	IsActive     bool      `json:"isActive" gorm:"default:true"`
	TrialDays    int       `json:"trialDays" gorm:"default:0"`
	Metadata     map[string]string `json:"metadata" gorm:"serializer:json"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type PlanLimits struct {
	Workflows      int   `json:"workflows"`      // -1 for unlimited
	Executions     int   `json:"executions"`     // per month
	TeamMembers    int   `json:"teamMembers"`
	StorageGB      int   `json:"storageGb"`
	APIRequests    int   `json:"apiRequests"`    // per month
	WebhooksPerDay int   `json:"webhooksPerDay"`
}

func NewPlan(name string, interval string, price int64) *Plan {
	return &Plan{
		ID:        uuid.New().String(),
		Name:      name,
		Interval:  interval,
		Price:     price,
		Currency:  "usd",
		Features:  []string{},
		IsActive:  true,
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Subscription represents a customer subscription
type Subscription struct {
	ID                   string     `json:"id" gorm:"primaryKey"`
	StripeID             string     `json:"stripeId" gorm:"uniqueIndex"`
	CustomerID           string     `json:"customerId" gorm:"not null;index"`
	PlanID               string     `json:"planId" gorm:"not null;index"`
	Status               string     `json:"status" gorm:"not null"`
	CurrentPeriodStart   time.Time  `json:"currentPeriodStart"`
	CurrentPeriodEnd     time.Time  `json:"currentPeriodEnd"`
	TrialStart           *time.Time `json:"trialStart"`
	TrialEnd             *time.Time `json:"trialEnd"`
	CancelledAt          *time.Time `json:"cancelledAt"`
	CancelAtPeriodEnd    bool       `json:"cancelAtPeriodEnd"`
	Quantity             int        `json:"quantity" gorm:"default:1"`
	Metadata             map[string]string `json:"metadata" gorm:"serializer:json"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

func NewSubscription(customerID, planID string) *Subscription {
	now := time.Now()
	return &Subscription{
		ID:                 uuid.New().String(),
		CustomerID:         customerID,
		PlanID:             planID,
		Status:             SubscriptionStatusActive,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		Quantity:           1,
		Metadata:           make(map[string]string),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func (s *Subscription) IsActive() bool {
	return s.Status == SubscriptionStatusActive || s.Status == SubscriptionStatusTrialing
}

func (s *Subscription) Cancel(atPeriodEnd bool) {
	now := time.Now()
	if atPeriodEnd {
		s.CancelAtPeriodEnd = true
	} else {
		s.Status = SubscriptionStatusCancelled
		s.CancelledAt = &now
	}
	s.UpdatedAt = now
}

// Invoice represents a billing invoice
type Invoice struct {
	ID             string       `json:"id" gorm:"primaryKey"`
	StripeID       string       `json:"stripeId" gorm:"uniqueIndex"`
	CustomerID     string       `json:"customerId" gorm:"not null;index"`
	SubscriptionID string       `json:"subscriptionId" gorm:"index"`
	Number         string       `json:"number"`
	Status         string       `json:"status" gorm:"not null"`
	Currency       string       `json:"currency" gorm:"default:'usd'"`
	Subtotal       int64        `json:"subtotal"`
	Tax            int64        `json:"tax"`
	Total          int64        `json:"total"`
	AmountPaid     int64        `json:"amountPaid"`
	AmountDue      int64        `json:"amountDue"`
	Lines          []InvoiceLine `json:"lines" gorm:"serializer:json"`
	PeriodStart    time.Time    `json:"periodStart"`
	PeriodEnd      time.Time    `json:"periodEnd"`
	DueDate        *time.Time   `json:"dueDate"`
	PaidAt         *time.Time   `json:"paidAt"`
	HostedURL      string       `json:"hostedUrl"`
	PDFURL         string       `json:"pdfUrl"`
	CreatedAt      time.Time    `json:"createdAt"`
	UpdatedAt      time.Time    `json:"updatedAt"`
}

type InvoiceLine struct {
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
	UnitAmount  int64  `json:"unitAmount"`
	Amount      int64  `json:"amount"`
}

func NewInvoice(customerID string, total int64) *Invoice {
	return &Invoice{
		ID:         uuid.New().String(),
		CustomerID: customerID,
		Status:     InvoiceStatusDraft,
		Currency:   "usd",
		Total:      total,
		AmountDue:  total,
		Lines:      []InvoiceLine{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// PaymentMethod represents a customer payment method
type PaymentMethod struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	StripeID     string    `json:"stripeId" gorm:"uniqueIndex"`
	CustomerID   string    `json:"customerId" gorm:"not null;index"`
	Type         string    `json:"type" gorm:"not null"` // card, bank_account
	IsDefault    bool      `json:"isDefault" gorm:"default:false"`
	Card         *CardInfo `json:"card" gorm:"serializer:json"`
	BillingDetails BillingDetails `json:"billingDetails" gorm:"serializer:json"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type CardInfo struct {
	Brand       string `json:"brand"`    // visa, mastercard, amex
	Last4       string `json:"last4"`
	ExpMonth    int    `json:"expMonth"`
	ExpYear     int    `json:"expYear"`
	Funding     string `json:"funding"`  // credit, debit
	Country     string `json:"country"`
}

type BillingDetails struct {
	Name    string  `json:"name"`
	Email   string  `json:"email"`
	Phone   string  `json:"phone"`
	Address Address `json:"address"`
}

type Address struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
}

// Usage represents usage tracking for metered billing
type Usage struct {
	ID             string    `json:"id" gorm:"primaryKey"`
	CustomerID     string    `json:"customerId" gorm:"not null;index"`
	SubscriptionID string    `json:"subscriptionId" gorm:"index"`
	Metric         string    `json:"metric" gorm:"not null;index"` // executions, api_calls, storage
	Quantity       int64     `json:"quantity"`
	Timestamp      time.Time `json:"timestamp" gorm:"index"`
	PeriodStart    time.Time `json:"periodStart"`
	PeriodEnd      time.Time `json:"periodEnd"`
	CreatedAt      time.Time `json:"createdAt"`
}

func NewUsage(customerID, metric string, quantity int64) *Usage {
	now := time.Now()
	return &Usage{
		ID:          uuid.New().String(),
		CustomerID:  customerID,
		Metric:      metric,
		Quantity:    quantity,
		Timestamp:   now,
		PeriodStart: time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()),
		PeriodEnd:   time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location()),
		CreatedAt:   now,
	}
}

// Credit represents account credits
type Credit struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	CustomerID  string    `json:"customerId" gorm:"not null;index"`
	Amount      int64     `json:"amount"` // in cents
	Description string    `json:"description"`
	ExpiresAt   *time.Time `json:"expiresAt"`
	UsedAmount  int64     `json:"usedAmount"`
	CreatedAt   time.Time `json:"createdAt"`
}

// PromoCode represents a promotional code
type PromoCode struct {
	ID              string    `json:"id" gorm:"primaryKey"`
	Code            string    `json:"code" gorm:"uniqueIndex;not null"`
	DiscountType    string    `json:"discountType"` // percentage, fixed
	DiscountAmount  int64     `json:"discountAmount"`
	MaxRedemptions  int       `json:"maxRedemptions"`
	TimesRedeemed   int       `json:"timesRedeemed"`
	ValidFrom       time.Time `json:"validFrom"`
	ValidUntil      *time.Time `json:"validUntil"`
	IsActive        bool      `json:"isActive" gorm:"default:true"`
	ApplicablePlans []string  `json:"applicablePlans" gorm:"serializer:json"`
	CreatedAt       time.Time `json:"createdAt"`
}
