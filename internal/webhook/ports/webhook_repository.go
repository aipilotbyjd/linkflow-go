package ports

import (
	"context"

	"github.com/linkflow-go/pkg/contracts/webhook"
)

type WebhookRepository interface {
	ListActive(ctx context.Context) ([]*webhook.Webhook, error)
	GetByPath(ctx context.Context, path string) (*webhook.Webhook, error)
	Create(ctx context.Context, wh *webhook.Webhook) error
	GetByID(ctx context.Context, id string) (*webhook.Webhook, error)
	ListByUser(ctx context.Context, userID string) ([]*webhook.Webhook, error)
	ListByWorkflow(ctx context.Context, workflowID string) ([]*webhook.Webhook, error)
	Update(ctx context.Context, wh *webhook.Webhook) error
	Delete(ctx context.Context, id string) error

	RecordExecution(ctx context.Context, exec *webhook.WebhookExecution) error
	UpdateExecution(ctx context.Context, exec *webhook.WebhookExecution) error
}
