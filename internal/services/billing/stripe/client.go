package stripe

import (
	"context"
)

type Client struct {
	secretKey string
}

func NewClient(secretKey string) *Client {
	return &Client{
		secretKey: secretKey,
	}
}

func (c *Client) CreateCustomer(ctx context.Context, email string) (string, error) {
	// Stripe customer creation logic
	return "cus_example", nil
}

func (c *Client) CreateSubscription(ctx context.Context, customerID, priceID string) (string, error) {
	// Stripe subscription creation logic
	return "sub_example", nil
}
