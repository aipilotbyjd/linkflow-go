package repository

import (
	"context"
	
	"github.com/linkflow-go/pkg/database"
)

type WebhookRepository struct {
	db *database.DB
}

func NewWebhookRepository(db *database.DB) *WebhookRepository {
	return &WebhookRepository{db: db}
}

func (r *WebhookRepository) CreateWebhook(ctx context.Context, webhook interface{}) error {
	return r.db.WithContext(ctx).Create(&webhook).Error
}

func (r *WebhookRepository) GetWebhook(ctx context.Context, id string) (interface{}, error) {
	var webhook interface{}
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&webhook).Error
	return webhook, err
}

func (r *WebhookRepository) ListWebhooks(ctx context.Context) ([]interface{}, error) {
	var webhooks []interface{}
	return webhooks, nil
}

func (r *WebhookRepository) LoadAllWebhooks(ctx context.Context) ([]interface{}, error) {
	var webhooks []interface{}
	return webhooks, nil
}
