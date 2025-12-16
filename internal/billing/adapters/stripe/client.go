package stripe

import (
	"context"
	"fmt"

	billing "github.com/linkflow-go/internal/billing/domain"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/invoice"
	"github.com/stripe/stripe-go/v76/paymentmethod"
	"github.com/stripe/stripe-go/v76/price"
	"github.com/stripe/stripe-go/v76/product"
	"github.com/stripe/stripe-go/v76/subscription"
	"github.com/stripe/stripe-go/v76/webhook"
)

type Client struct {
	secretKey     string
	webhookSecret string
}

func NewClient(secretKey, webhookSecret string) *Client {
	stripe.Key = secretKey
	return &Client{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
	}
}

// Customer operations

func (c *Client) CreateCustomer(ctx context.Context, email, name string, metadata map[string]string) (*stripe.Customer, error) {
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
	}
	for k, v := range metadata {
		params.AddMetadata(k, v)
	}
	return customer.New(params)
}

func (c *Client) GetCustomer(ctx context.Context, customerID string) (*stripe.Customer, error) {
	return customer.Get(customerID, nil)
}

func (c *Client) UpdateCustomer(ctx context.Context, customerID string, params *stripe.CustomerParams) (*stripe.Customer, error) {
	return customer.Update(customerID, params)
}

func (c *Client) DeleteCustomer(ctx context.Context, customerID string) (*stripe.Customer, error) {
	return customer.Del(customerID, nil)
}

// Product/Price operations

func (c *Client) CreateProduct(ctx context.Context, name, description string) (*stripe.Product, error) {
	params := &stripe.ProductParams{
		Name:        stripe.String(name),
		Description: stripe.String(description),
	}
	return product.New(params)
}

func (c *Client) CreatePrice(ctx context.Context, productID string, unitAmount int64, currency, interval string) (*stripe.Price, error) {
	params := &stripe.PriceParams{
		Product:    stripe.String(productID),
		UnitAmount: stripe.Int64(unitAmount),
		Currency:   stripe.String(currency),
		Recurring: &stripe.PriceRecurringParams{
			Interval: stripe.String(interval),
		},
	}
	return price.New(params)
}

// Subscription operations

func (c *Client) CreateSubscription(ctx context.Context, customerID, priceID string, trialDays int64) (*stripe.Subscription, error) {
	params := &stripe.SubscriptionParams{
		Customer: stripe.String(customerID),
		Items: []*stripe.SubscriptionItemsParams{
			{Price: stripe.String(priceID)},
		},
	}
	if trialDays > 0 {
		params.TrialPeriodDays = stripe.Int64(trialDays)
	}
	return subscription.New(params)
}

func (c *Client) GetSubscription(ctx context.Context, subscriptionID string) (*stripe.Subscription, error) {
	return subscription.Get(subscriptionID, nil)
}

func (c *Client) UpdateSubscription(ctx context.Context, subscriptionID string, params *stripe.SubscriptionParams) (*stripe.Subscription, error) {
	return subscription.Update(subscriptionID, params)
}

func (c *Client) CancelSubscription(ctx context.Context, subscriptionID string, cancelAtPeriodEnd bool) (*stripe.Subscription, error) {
	if cancelAtPeriodEnd {
		params := &stripe.SubscriptionParams{
			CancelAtPeriodEnd: stripe.Bool(true),
		}
		return subscription.Update(subscriptionID, params)
	}
	return subscription.Cancel(subscriptionID, nil)
}

func (c *Client) ReactivateSubscription(ctx context.Context, subscriptionID string) (*stripe.Subscription, error) {
	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(false),
	}
	return subscription.Update(subscriptionID, params)
}

func (c *Client) ChangeSubscriptionPlan(ctx context.Context, subscriptionID, newPriceID string) (*stripe.Subscription, error) {
	sub, err := subscription.Get(subscriptionID, nil)
	if err != nil {
		return nil, err
	}

	if len(sub.Items.Data) == 0 {
		return nil, fmt.Errorf("subscription has no items")
	}

	params := &stripe.SubscriptionParams{
		Items: []*stripe.SubscriptionItemsParams{
			{
				ID:    stripe.String(sub.Items.Data[0].ID),
				Price: stripe.String(newPriceID),
			},
		},
		ProrationBehavior: stripe.String("create_prorations"),
	}
	return subscription.Update(subscriptionID, params)
}

// Payment Method operations

func (c *Client) AttachPaymentMethod(ctx context.Context, paymentMethodID, customerID string) (*stripe.PaymentMethod, error) {
	params := &stripe.PaymentMethodAttachParams{
		Customer: stripe.String(customerID),
	}
	return paymentmethod.Attach(paymentMethodID, params)
}

func (c *Client) DetachPaymentMethod(ctx context.Context, paymentMethodID string) (*stripe.PaymentMethod, error) {
	return paymentmethod.Detach(paymentMethodID, nil)
}

func (c *Client) ListPaymentMethods(ctx context.Context, customerID string) ([]*stripe.PaymentMethod, error) {
	params := &stripe.PaymentMethodListParams{
		Customer: stripe.String(customerID),
		Type:     stripe.String("card"),
	}

	var methods []*stripe.PaymentMethod
	iter := paymentmethod.List(params)
	for iter.Next() {
		methods = append(methods, iter.PaymentMethod())
	}
	return methods, iter.Err()
}

func (c *Client) SetDefaultPaymentMethod(ctx context.Context, customerID, paymentMethodID string) (*stripe.Customer, error) {
	params := &stripe.CustomerParams{
		InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
			DefaultPaymentMethod: stripe.String(paymentMethodID),
		},
	}
	return customer.Update(customerID, params)
}

// Invoice operations

func (c *Client) GetInvoice(ctx context.Context, invoiceID string) (*stripe.Invoice, error) {
	return invoice.Get(invoiceID, nil)
}

func (c *Client) ListInvoices(ctx context.Context, customerID string, limit int64) ([]*stripe.Invoice, error) {
	params := &stripe.InvoiceListParams{
		Customer: stripe.String(customerID),
	}
	params.Limit = stripe.Int64(limit)

	var invoices []*stripe.Invoice
	iter := invoice.List(params)
	for iter.Next() {
		invoices = append(invoices, iter.Invoice())
	}
	return invoices, iter.Err()
}

func (c *Client) PayInvoice(ctx context.Context, invoiceID string) (*stripe.Invoice, error) {
	return invoice.Pay(invoiceID, nil)
}

func (c *Client) VoidInvoice(ctx context.Context, invoiceID string) (*stripe.Invoice, error) {
	return invoice.VoidInvoice(invoiceID, nil)
}

// Webhook handling

func (c *Client) ConstructWebhookEvent(payload []byte, signature string) (stripe.Event, error) {
	return webhook.ConstructEvent(payload, signature, c.webhookSecret)
}

// Utility functions

func (c *Client) MapSubscriptionStatus(status stripe.SubscriptionStatus) string {
	switch status {
	case stripe.SubscriptionStatusActive:
		return billing.SubscriptionStatusActive
	case stripe.SubscriptionStatusTrialing:
		return billing.SubscriptionStatusTrialing
	case stripe.SubscriptionStatusPastDue:
		return billing.SubscriptionStatusPastDue
	case stripe.SubscriptionStatusCanceled:
		return billing.SubscriptionStatusCancelled
	case stripe.SubscriptionStatusPaused:
		return billing.SubscriptionStatusPaused
	default:
		return string(status)
	}
}

func (c *Client) MapInvoiceStatus(status stripe.InvoiceStatus) string {
	switch status {
	case stripe.InvoiceStatusDraft:
		return billing.InvoiceStatusDraft
	case stripe.InvoiceStatusOpen:
		return billing.InvoiceStatusOpen
	case stripe.InvoiceStatusPaid:
		return billing.InvoiceStatusPaid
	case stripe.InvoiceStatusVoid:
		return billing.InvoiceStatusVoid
	case stripe.InvoiceStatusUncollectible:
		return billing.InvoiceStatusUncollectible
	default:
		return string(status)
	}
}
