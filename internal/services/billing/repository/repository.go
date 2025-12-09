package repository

import (
	"context"
	
	"github.com/linkflow-go/pkg/database"
)

type BillingRepository struct {
	db *database.DB
}

func NewBillingRepository(db *database.DB) *BillingRepository {
	return &BillingRepository{db: db}
}

func (r *BillingRepository) CreateSubscription(ctx context.Context, subscription interface{}) error {
	return r.db.WithContext(ctx).Create(&subscription).Error
}

func (r *BillingRepository) GetSubscriptions(ctx context.Context, userID string) ([]interface{}, error) {
	var subscriptions []interface{}
	return subscriptions, nil
}
