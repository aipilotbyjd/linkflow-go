package service

import (
	"context"

	"github.com/linkflow-go/internal/services/webhook/repository"
	"github.com/linkflow-go/internal/services/webhook/router"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type WebhookService struct {
	repo     *repository.WebhookRepository
	router   *router.WebhookRouter
	eventBus events.EventBus
	redis    *redis.Client
	logger   logger.Logger
}

func NewWebhookService(
	repo *repository.WebhookRepository,
	router *router.WebhookRouter,
	eventBus events.EventBus,
	redis *redis.Client,
	logger logger.Logger,
) *WebhookService {
	return &WebhookService{
		repo:     repo,
		router:   router,
		eventBus: eventBus,
		redis:    redis,
		logger:   logger,
	}
}

func (s *WebhookService) HandleWorkflowExecuted(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling workflow executed event for webhook")
	return nil
}

func (s *WebhookService) HandleWorkflowFailed(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling workflow failed event for webhook")
	return nil
}

func (s *WebhookService) HandleExecutionCompleted(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling execution completed event for webhook")
	return nil
}
