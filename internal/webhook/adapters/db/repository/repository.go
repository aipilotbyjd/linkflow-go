package repository

import (
	"context"
	"time"

	"github.com/linkflow-go/pkg/contracts/webhook"
	"github.com/linkflow-go/pkg/database"
)

type WebhookRepository struct {
	db *database.DB
}

func NewWebhookRepository(db *database.DB) *WebhookRepository {
	return &WebhookRepository{db: db}
}

// Webhook operations

func (r *WebhookRepository) Create(ctx context.Context, wh *webhook.Webhook) error {
	return r.db.WithContext(ctx).Create(wh).Error
}

func (r *WebhookRepository) GetByID(ctx context.Context, id string) (*webhook.Webhook, error) {
	var wh webhook.Webhook
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&wh).Error
	if err != nil {
		return nil, webhook.ErrWebhookNotFound
	}
	return &wh, nil
}

func (r *WebhookRepository) GetByPath(ctx context.Context, path string) (*webhook.Webhook, error) {
	var wh webhook.Webhook
	err := r.db.WithContext(ctx).Where("path = ?", path).First(&wh).Error
	if err != nil {
		return nil, webhook.ErrWebhookNotFound
	}
	return &wh, nil
}

func (r *WebhookRepository) ListActive(ctx context.Context) ([]*webhook.Webhook, error) {
	var webhooks []*webhook.Webhook
	err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&webhooks).Error
	return webhooks, err
}

func (r *WebhookRepository) ListByUser(ctx context.Context, userID string) ([]*webhook.Webhook, error) {
	var webhooks []*webhook.Webhook
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&webhooks).Error
	return webhooks, err
}

func (r *WebhookRepository) ListByWorkflow(ctx context.Context, workflowID string) ([]*webhook.Webhook, error) {
	var webhooks []*webhook.Webhook
	err := r.db.WithContext(ctx).Where("workflow_id = ?", workflowID).Find(&webhooks).Error
	return webhooks, err
}

func (r *WebhookRepository) Update(ctx context.Context, wh *webhook.Webhook) error {
	wh.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(wh).Error
}

func (r *WebhookRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&webhook.Webhook{}).Error
}

// Execution operations

func (r *WebhookRepository) RecordExecution(ctx context.Context, exec *webhook.WebhookExecution) error {
	return r.db.WithContext(ctx).Create(exec).Error
}

func (r *WebhookRepository) UpdateExecution(ctx context.Context, exec *webhook.WebhookExecution) error {
	return r.db.WithContext(ctx).Save(exec).Error
}

func (r *WebhookRepository) GetExecution(ctx context.Context, id string) (*webhook.WebhookExecution, error) {
	var exec webhook.WebhookExecution
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&exec).Error
	return &exec, err
}

func (r *WebhookRepository) ListExecutions(ctx context.Context, webhookID string, limit int) ([]*webhook.WebhookExecution, error) {
	var executions []*webhook.WebhookExecution
	query := r.db.WithContext(ctx).Where("webhook_id = ?", webhookID).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&executions).Error
	return executions, err
}

func (r *WebhookRepository) GetExecutionStats(ctx context.Context, webhookID string) (map[string]int64, error) {
	stats := make(map[string]int64)

	type result struct {
		Status string
		Count  int64
	}
	var results []result

	err := r.db.WithContext(ctx).
		Model(&webhook.WebhookExecution{}).
		Select("status, COUNT(*) as count").
		Where("webhook_id = ?", webhookID).
		Group("status").
		Find(&results).Error

	if err != nil {
		return nil, err
	}

	for _, r := range results {
		stats[r.Status] = r.Count
	}

	return stats, nil
}

// Backward compatibility

func (r *WebhookRepository) CreateWebhook(ctx context.Context, wh interface{}) error {
	return r.db.WithContext(ctx).Create(wh).Error
}

func (r *WebhookRepository) GetWebhook(ctx context.Context, id string) (interface{}, error) {
	return r.GetByID(ctx, id)
}

func (r *WebhookRepository) ListWebhooks(ctx context.Context) ([]interface{}, error) {
	webhooks, err := r.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]interface{}, len(webhooks))
	for i, wh := range webhooks {
		result[i] = wh
	}
	return result, nil
}

func (r *WebhookRepository) LoadAllWebhooks(ctx context.Context) ([]interface{}, error) {
	return r.ListWebhooks(ctx)
}
