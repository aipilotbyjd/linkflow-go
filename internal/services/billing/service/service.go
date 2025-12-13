package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/linkflow-go/internal/domain/billing"
	"github.com/linkflow-go/internal/services/billing/repository"
	"github.com/linkflow-go/internal/services/billing/stripe"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
	stripelib "github.com/stripe/stripe-go/v76"
)

type BillingService struct {
	repo         *repository.BillingRepository
	stripeClient *stripe.Client
	eventBus     events.EventBus
	redis        *redis.Client
	logger       logger.Logger
}

func NewBillingService(
	repo *repository.BillingRepository,
	stripeClient *stripe.Client,
	eventBus events.EventBus,
	redis *redis.Client,
	logger logger.Logger,
) *BillingService {
	return &BillingService{
		repo:         repo,
		stripeClient: stripeClient,
		eventBus:     eventBus,
		redis:        redis,
		logger:       logger,
	}
}

// Customer operations

func (s *BillingService) CreateCustomer(ctx context.Context, req CreateCustomerRequest) (*billing.Customer, error) {
	// Create in Stripe first
	stripeCustomer, err := s.stripeClient.CreateCustomer(ctx, req.Email, req.Name, map[string]string{"user_id": req.UserID})
	if err != nil {
		return nil, fmt.Errorf("failed to create Stripe customer: %w", err)
	}

	// Create in database
	customer := billing.NewCustomer(req.UserID, req.Email)
	customer.Name = req.Name
	customer.StripeID = stripeCustomer.ID

	if err := s.repo.CreateCustomer(ctx, customer); err != nil {
		return nil, fmt.Errorf("failed to create customer: %w", err)
	}

	// Publish event
	event := events.NewEventBuilder("billing.customer.created").
		WithAggregateID(customer.ID).
		WithUserID(req.UserID).
		Build()
	s.eventBus.Publish(ctx, event)

	s.logger.Info("Customer created", "id", customer.ID, "stripeId", customer.StripeID)
	return customer, nil
}

func (s *BillingService) GetCustomer(ctx context.Context, id string) (*billing.Customer, error) {
	return s.repo.GetCustomer(ctx, id)
}

func (s *BillingService) GetCustomerByUserID(ctx context.Context, userID string) (*billing.Customer, error) {
	return s.repo.GetCustomerByUserID(ctx, userID)
}

func (s *BillingService) GetOrCreateCustomer(ctx context.Context, userID, email, name string) (*billing.Customer, error) {
	customer, err := s.repo.GetCustomerByUserID(ctx, userID)
	if err == nil {
		return customer, nil
	}

	return s.CreateCustomer(ctx, CreateCustomerRequest{
		UserID: userID,
		Email:  email,
		Name:   name,
	})
}

// Plan operations

func (s *BillingService) CreatePlan(ctx context.Context, req CreatePlanRequest) (*billing.Plan, error) {
	// Create product in Stripe
	stripeProduct, err := s.stripeClient.CreateProduct(ctx, req.Name, req.Description)
	if err != nil {
		return nil, fmt.Errorf("failed to create Stripe product: %w", err)
	}

	// Create price in Stripe
	stripePrice, err := s.stripeClient.CreatePrice(ctx, stripeProduct.ID, req.Price, "usd", req.Interval)
	if err != nil {
		return nil, fmt.Errorf("failed to create Stripe price: %w", err)
	}

	// Create in database
	plan := billing.NewPlan(req.Name, req.Interval, req.Price)
	plan.Description = req.Description
	plan.StripeID = stripePrice.ID
	plan.Features = req.Features
	plan.Limits = req.Limits
	plan.TrialDays = req.TrialDays

	if err := s.repo.CreatePlan(ctx, plan); err != nil {
		return nil, fmt.Errorf("failed to create plan: %w", err)
	}

	s.logger.Info("Plan created", "id", plan.ID, "name", plan.Name)
	return plan, nil
}

func (s *BillingService) GetPlan(ctx context.Context, id string) (*billing.Plan, error) {
	return s.repo.GetPlan(ctx, id)
}

func (s *BillingService) ListPlans(ctx context.Context, activeOnly bool) ([]*billing.Plan, error) {
	return s.repo.ListPlans(ctx, activeOnly)
}

func (s *BillingService) UpdatePlan(ctx context.Context, id string, req UpdatePlanRequest) (*billing.Plan, error) {
	plan, err := s.repo.GetPlan(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		plan.Name = req.Name
	}
	if req.Description != "" {
		plan.Description = req.Description
	}
	if req.Features != nil {
		plan.Features = req.Features
	}
	if req.IsActive != nil {
		plan.IsActive = *req.IsActive
	}

	if err := s.repo.UpdatePlan(ctx, plan); err != nil {
		return nil, fmt.Errorf("failed to update plan: %w", err)
	}

	return plan, nil
}

func (s *BillingService) DeletePlan(ctx context.Context, id string) error {
	// Soft delete - just mark as inactive
	plan, err := s.repo.GetPlan(ctx, id)
	if err != nil {
		return err
	}
	plan.IsActive = false
	return s.repo.UpdatePlan(ctx, plan)
}

// Subscription operations

func (s *BillingService) CreateSubscription(ctx context.Context, req CreateSubscriptionRequest) (*billing.Subscription, error) {
	// Get customer
	customer, err := s.repo.GetCustomerByUserID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	// Get plan
	plan, err := s.repo.GetPlan(ctx, req.PlanID)
	if err != nil {
		return nil, err
	}

	// Check for existing active subscription
	existing, _ := s.repo.GetActiveSubscription(ctx, customer.ID)
	if existing != nil {
		return nil, fmt.Errorf("customer already has an active subscription")
	}

	// Create in Stripe
	stripeSub, err := s.stripeClient.CreateSubscription(ctx, customer.StripeID, plan.StripeID, int64(plan.TrialDays))
	if err != nil {
		return nil, fmt.Errorf("failed to create Stripe subscription: %w", err)
	}

	// Create in database
	subscription := billing.NewSubscription(customer.ID, plan.ID)
	subscription.StripeID = stripeSub.ID
	subscription.Status = s.stripeClient.MapSubscriptionStatus(stripeSub.Status)
	subscription.CurrentPeriodStart = time.Unix(stripeSub.CurrentPeriodStart, 0)
	subscription.CurrentPeriodEnd = time.Unix(stripeSub.CurrentPeriodEnd, 0)

	if stripeSub.TrialStart > 0 {
		trialStart := time.Unix(stripeSub.TrialStart, 0)
		subscription.TrialStart = &trialStart
	}
	if stripeSub.TrialEnd > 0 {
		trialEnd := time.Unix(stripeSub.TrialEnd, 0)
		subscription.TrialEnd = &trialEnd
	}

	if err := s.repo.CreateSubscription(ctx, subscription); err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	// Publish event
	event := events.NewEventBuilder("billing.subscription.created").
		WithAggregateID(subscription.ID).
		WithUserID(req.UserID).
		WithPayload("planId", plan.ID).
		WithPayload("status", subscription.Status).
		Build()
	s.eventBus.Publish(ctx, event)

	s.logger.Info("Subscription created", "id", subscription.ID, "planId", plan.ID)
	return subscription, nil
}

func (s *BillingService) GetSubscription(ctx context.Context, id string) (*billing.Subscription, error) {
	return s.repo.GetSubscription(ctx, id)
}

func (s *BillingService) GetActiveSubscription(ctx context.Context, userID string) (*billing.Subscription, error) {
	customer, err := s.repo.GetCustomerByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetActiveSubscription(ctx, customer.ID)
}

func (s *BillingService) ListSubscriptions(ctx context.Context, userID string) ([]*billing.Subscription, error) {
	customer, err := s.repo.GetCustomerByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListSubscriptions(ctx, customer.ID)
}

func (s *BillingService) CancelSubscription(ctx context.Context, id, userID string, atPeriodEnd bool) (*billing.Subscription, error) {
	subscription, err := s.repo.GetSubscription(ctx, id)
	if err != nil {
		return nil, err
	}

	// Cancel in Stripe
	_, err = s.stripeClient.CancelSubscription(ctx, subscription.StripeID, atPeriodEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel Stripe subscription: %w", err)
	}

	// Update in database
	subscription.Cancel(atPeriodEnd)
	if err := s.repo.UpdateSubscription(ctx, subscription); err != nil {
		return nil, fmt.Errorf("failed to update subscription: %w", err)
	}

	// Publish event
	event := events.NewEventBuilder("billing.subscription.cancelled").
		WithAggregateID(subscription.ID).
		WithUserID(userID).
		WithPayload("atPeriodEnd", atPeriodEnd).
		Build()
	s.eventBus.Publish(ctx, event)

	s.logger.Info("Subscription cancelled", "id", subscription.ID, "atPeriodEnd", atPeriodEnd)
	return subscription, nil
}

func (s *BillingService) ReactivateSubscription(ctx context.Context, id, userID string) (*billing.Subscription, error) {
	subscription, err := s.repo.GetSubscription(ctx, id)
	if err != nil {
		return nil, err
	}

	if subscription.Status == billing.SubscriptionStatusCancelled {
		return nil, billing.ErrSubscriptionCancelled
	}

	// Reactivate in Stripe
	_, err = s.stripeClient.ReactivateSubscription(ctx, subscription.StripeID)
	if err != nil {
		return nil, fmt.Errorf("failed to reactivate Stripe subscription: %w", err)
	}

	// Update in database
	subscription.CancelAtPeriodEnd = false
	subscription.Status = billing.SubscriptionStatusActive
	if err := s.repo.UpdateSubscription(ctx, subscription); err != nil {
		return nil, fmt.Errorf("failed to update subscription: %w", err)
	}

	s.logger.Info("Subscription reactivated", "id", subscription.ID)
	return subscription, nil
}

func (s *BillingService) ChangePlan(ctx context.Context, subscriptionID, newPlanID, userID string) (*billing.Subscription, error) {
	subscription, err := s.repo.GetSubscription(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	newPlan, err := s.repo.GetPlan(ctx, newPlanID)
	if err != nil {
		return nil, err
	}

	// Change plan in Stripe
	_, err = s.stripeClient.ChangeSubscriptionPlan(ctx, subscription.StripeID, newPlan.StripeID)
	if err != nil {
		return nil, fmt.Errorf("failed to change plan in Stripe: %w", err)
	}

	// Update in database
	oldPlanID := subscription.PlanID
	subscription.PlanID = newPlanID
	if err := s.repo.UpdateSubscription(ctx, subscription); err != nil {
		return nil, fmt.Errorf("failed to update subscription: %w", err)
	}

	// Publish event
	event := events.NewEventBuilder("billing.subscription.plan_changed").
		WithAggregateID(subscription.ID).
		WithUserID(userID).
		WithPayload("oldPlanId", oldPlanID).
		WithPayload("newPlanId", newPlanID).
		Build()
	s.eventBus.Publish(ctx, event)

	s.logger.Info("Subscription plan changed", "id", subscription.ID, "newPlanId", newPlanID)
	return subscription, nil
}

// Payment Method operations

func (s *BillingService) AddPaymentMethod(ctx context.Context, userID, paymentMethodID string) (*billing.PaymentMethod, error) {
	customer, err := s.repo.GetCustomerByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Attach to Stripe customer
	stripePM, err := s.stripeClient.AttachPaymentMethod(ctx, paymentMethodID, customer.StripeID)
	if err != nil {
		return nil, fmt.Errorf("failed to attach payment method: %w", err)
	}

	// Create in database
	pm := &billing.PaymentMethod{
		ID:         stripePM.ID,
		StripeID:   stripePM.ID,
		CustomerID: customer.ID,
		Type:       string(stripePM.Type),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if stripePM.Card != nil {
		pm.Card = &billing.CardInfo{
			Brand:    string(stripePM.Card.Brand),
			Last4:    stripePM.Card.Last4,
			ExpMonth: int(stripePM.Card.ExpMonth),
			ExpYear:  int(stripePM.Card.ExpYear),
			Funding:  string(stripePM.Card.Funding),
			Country:  stripePM.Card.Country,
		}
	}

	// Check if this is the first payment method
	existing, _ := s.repo.ListPaymentMethods(ctx, customer.ID)
	if len(existing) == 0 {
		pm.IsDefault = true
		// Set as default in Stripe too
		s.stripeClient.SetDefaultPaymentMethod(ctx, customer.StripeID, pm.StripeID)
	}

	if err := s.repo.CreatePaymentMethod(ctx, pm); err != nil {
		return nil, fmt.Errorf("failed to create payment method: %w", err)
	}

	s.logger.Info("Payment method added", "customerId", customer.ID, "type", pm.Type)
	return pm, nil
}

func (s *BillingService) ListPaymentMethods(ctx context.Context, userID string) ([]*billing.PaymentMethod, error) {
	customer, err := s.repo.GetCustomerByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListPaymentMethods(ctx, customer.ID)
}

func (s *BillingService) SetDefaultPaymentMethod(ctx context.Context, userID, paymentMethodID string) error {
	customer, err := s.repo.GetCustomerByUserID(ctx, userID)
	if err != nil {
		return err
	}

	// Update in Stripe
	_, err = s.stripeClient.SetDefaultPaymentMethod(ctx, customer.StripeID, paymentMethodID)
	if err != nil {
		return fmt.Errorf("failed to set default in Stripe: %w", err)
	}

	// Update in database
	return s.repo.SetDefaultPaymentMethod(ctx, customer.ID, paymentMethodID)
}

func (s *BillingService) RemovePaymentMethod(ctx context.Context, userID, paymentMethodID string) error {
	// Detach from Stripe
	_, err := s.stripeClient.DetachPaymentMethod(ctx, paymentMethodID)
	if err != nil {
		return fmt.Errorf("failed to detach payment method: %w", err)
	}

	// Delete from database
	return s.repo.DeletePaymentMethod(ctx, paymentMethodID)
}

// Invoice operations

func (s *BillingService) GetInvoice(ctx context.Context, id string) (*billing.Invoice, error) {
	return s.repo.GetInvoice(ctx, id)
}

func (s *BillingService) ListInvoices(ctx context.Context, userID string, limit int) ([]*billing.Invoice, error) {
	customer, err := s.repo.GetCustomerByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListInvoices(ctx, customer.ID, limit)
}

func (s *BillingService) PayInvoice(ctx context.Context, invoiceID, userID string) (*billing.Invoice, error) {
	invoice, err := s.repo.GetInvoice(ctx, invoiceID)
	if err != nil {
		return nil, err
	}

	// Pay in Stripe
	stripeInvoice, err := s.stripeClient.PayInvoice(ctx, invoice.StripeID)
	if err != nil {
		return nil, fmt.Errorf("failed to pay invoice: %w", err)
	}

	// Update in database
	invoice.Status = s.stripeClient.MapInvoiceStatus(stripeInvoice.Status)
	invoice.AmountPaid = stripeInvoice.AmountPaid
	invoice.AmountDue = stripeInvoice.AmountDue
	if stripeInvoice.Status == stripelib.InvoiceStatusPaid {
		now := time.Now()
		invoice.PaidAt = &now
	}

	if err := s.repo.UpdateInvoice(ctx, invoice); err != nil {
		return nil, fmt.Errorf("failed to update invoice: %w", err)
	}

	return invoice, nil
}

// Usage operations

func (s *BillingService) RecordUsage(ctx context.Context, userID, metric string, quantity int64) error {
	customer, err := s.repo.GetCustomerByUserID(ctx, userID)
	if err != nil {
		return err
	}

	usage := billing.NewUsage(customer.ID, metric, quantity)
	return s.repo.RecordUsage(ctx, usage)
}

func (s *BillingService) GetUsageSummary(ctx context.Context, userID string) (map[string]int64, error) {
	customer, err := s.repo.GetCustomerByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	periodEnd := periodStart.AddDate(0, 1, 0)

	return s.repo.GetUsageSummary(ctx, customer.ID, periodStart, periodEnd)
}

// Credits and Promo

func (s *BillingService) GetCredits(ctx context.Context, userID string) (int64, error) {
	customer, err := s.repo.GetCustomerByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}
	return s.repo.GetAvailableCredits(ctx, customer.ID)
}

func (s *BillingService) ApplyPromoCode(ctx context.Context, userID, code string) error {
	promo, err := s.repo.GetPromoCode(ctx, code)
	if err != nil {
		return fmt.Errorf("invalid promo code")
	}

	// Check if valid
	now := time.Now()
	if now.Before(promo.ValidFrom) || (promo.ValidUntil != nil && now.After(*promo.ValidUntil)) {
		return fmt.Errorf("promo code is expired")
	}
	if promo.MaxRedemptions > 0 && promo.TimesRedeemed >= promo.MaxRedemptions {
		return fmt.Errorf("promo code has reached maximum redemptions")
	}

	customer, err := s.repo.GetCustomerByUserID(ctx, userID)
	if err != nil {
		return err
	}

	// Apply credit
	credit := &billing.Credit{
		CustomerID:  customer.ID,
		Amount:      promo.DiscountAmount,
		Description: fmt.Sprintf("Promo code: %s", code),
		CreatedAt:   time.Now(),
	}
	if err := s.repo.CreateCredit(ctx, credit); err != nil {
		return err
	}

	// Increment redemptions
	return s.repo.IncrementPromoRedemptions(ctx, promo.ID)
}

// Billing info

func (s *BillingService) GetBillingInfo(ctx context.Context, userID string) (*BillingInfo, error) {
	customer, err := s.repo.GetCustomerByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	subscription, _ := s.repo.GetActiveSubscription(ctx, customer.ID)
	paymentMethods, _ := s.repo.ListPaymentMethods(ctx, customer.ID)
	credits, _ := s.repo.GetAvailableCredits(ctx, customer.ID)
	usage, _ := s.GetUsageSummary(ctx, userID)

	var plan *billing.Plan
	if subscription != nil {
		plan, _ = s.repo.GetPlan(ctx, subscription.PlanID)
	}

	return &BillingInfo{
		Customer:       customer,
		Subscription:   subscription,
		Plan:           plan,
		PaymentMethods: paymentMethods,
		Credits:        credits,
		Usage:          usage,
	}, nil
}

// Webhook handling

func (s *BillingService) HandleStripeWebhook(ctx context.Context, payload []byte, signature string) error {
	event, err := s.stripeClient.ConstructWebhookEvent(payload, signature)
	if err != nil {
		return fmt.Errorf("invalid webhook signature: %w", err)
	}

	s.logger.Info("Received Stripe webhook", "type", event.Type)

	switch event.Type {
	case "customer.subscription.updated":
		var sub stripelib.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return err
		}
		return s.handleSubscriptionUpdated(ctx, &sub)

	case "customer.subscription.deleted":
		var sub stripelib.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return err
		}
		return s.handleSubscriptionDeleted(ctx, &sub)

	case "invoice.paid":
		var inv stripelib.Invoice
		if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
			return err
		}
		return s.handleInvoicePaid(ctx, &inv)

	case "invoice.payment_failed":
		var inv stripelib.Invoice
		if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
			return err
		}
		return s.handleInvoicePaymentFailed(ctx, &inv)
	}

	return nil
}

func (s *BillingService) handleSubscriptionUpdated(ctx context.Context, stripeSub *stripelib.Subscription) error {
	subscription, err := s.repo.GetSubscriptionByStripeID(ctx, stripeSub.ID)
	if err != nil {
		return err
	}

	subscription.Status = s.stripeClient.MapSubscriptionStatus(stripeSub.Status)
	subscription.CurrentPeriodStart = time.Unix(stripeSub.CurrentPeriodStart, 0)
	subscription.CurrentPeriodEnd = time.Unix(stripeSub.CurrentPeriodEnd, 0)
	subscription.CancelAtPeriodEnd = stripeSub.CancelAtPeriodEnd

	return s.repo.UpdateSubscription(ctx, subscription)
}

func (s *BillingService) handleSubscriptionDeleted(ctx context.Context, stripeSub *stripelib.Subscription) error {
	subscription, err := s.repo.GetSubscriptionByStripeID(ctx, stripeSub.ID)
	if err != nil {
		return err
	}

	subscription.Status = billing.SubscriptionStatusCancelled
	now := time.Now()
	subscription.CancelledAt = &now

	return s.repo.UpdateSubscription(ctx, subscription)
}

func (s *BillingService) handleInvoicePaid(ctx context.Context, stripeInv *stripelib.Invoice) error {
	invoice, err := s.repo.GetInvoiceByStripeID(ctx, stripeInv.ID)
	if err != nil {
		// Create new invoice record
		customer, err := s.repo.GetCustomerByStripeID(ctx, stripeInv.Customer.ID)
		if err != nil {
			return err
		}

		invoice = billing.NewInvoice(customer.ID, stripeInv.Total)
		invoice.StripeID = stripeInv.ID
		invoice.Number = stripeInv.Number
		invoice.Status = billing.InvoiceStatusPaid
		invoice.Subtotal = stripeInv.Subtotal
		invoice.Tax = stripeInv.Tax
		invoice.Total = stripeInv.Total
		invoice.AmountPaid = stripeInv.AmountPaid
		invoice.AmountDue = stripeInv.AmountDue
		invoice.HostedURL = stripeInv.HostedInvoiceURL
		invoice.PDFURL = stripeInv.InvoicePDF
		now := time.Now()
		invoice.PaidAt = &now

		return s.repo.CreateInvoice(ctx, invoice)
	}

	invoice.Status = billing.InvoiceStatusPaid
	invoice.AmountPaid = stripeInv.AmountPaid
	now := time.Now()
	invoice.PaidAt = &now

	return s.repo.UpdateInvoice(ctx, invoice)
}

func (s *BillingService) handleInvoicePaymentFailed(ctx context.Context, stripeInv *stripelib.Invoice) error {
	// Publish event for notification
	event := events.NewEventBuilder("billing.invoice.payment_failed").
		WithAggregateID(stripeInv.ID).
		WithPayload("amount", stripeInv.AmountDue).
		Build()
	s.eventBus.Publish(ctx, event)

	return nil
}

// Event handlers
func (s *BillingService) HandleBillingEvent(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling billing event", "type", event.Type, "id", event.ID)

	switch event.Type {
	case "execution.completed":
		// Record execution usage
		if userID, ok := event.Payload["userId"].(string); ok {
			return s.RecordUsage(ctx, userID, "executions", 1)
		}
	case "storage.used":
		// Record storage usage
		if userID, ok := event.Payload["userId"].(string); ok {
			if bytes, ok := event.Payload["bytes"].(float64); ok {
				return s.RecordUsage(ctx, userID, "storage_bytes", int64(bytes))
			}
		}
	}

	return nil
}

// Request/Response types

type CreateCustomerRequest struct {
	UserID string `json:"userId" binding:"required"`
	Email  string `json:"email" binding:"required"`
	Name   string `json:"name"`
}

type CreatePlanRequest struct {
	Name        string             `json:"name" binding:"required"`
	Description string             `json:"description"`
	Interval    string             `json:"interval" binding:"required"`
	Price       int64              `json:"price" binding:"required"`
	Features    []string           `json:"features"`
	Limits      billing.PlanLimits `json:"limits"`
	TrialDays   int                `json:"trialDays"`
}

type UpdatePlanRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Features    []string `json:"features"`
	IsActive    *bool    `json:"isActive"`
}

type CreateSubscriptionRequest struct {
	UserID string `json:"-"`
	PlanID string `json:"planId" binding:"required"`
}

type BillingInfo struct {
	Customer       *billing.Customer        `json:"customer"`
	Subscription   *billing.Subscription    `json:"subscription"`
	Plan           *billing.Plan            `json:"plan"`
	PaymentMethods []*billing.PaymentMethod `json:"paymentMethods"`
	Credits        int64                    `json:"credits"`
	Usage          map[string]int64         `json:"usage"`
}
