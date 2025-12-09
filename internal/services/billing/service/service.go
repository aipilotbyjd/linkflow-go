package service

import (
	"context"

	"github.com/linkflow-go/internal/services/billing/repository"
	"github.com/linkflow-go/internal/services/billing/stripe"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
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

func (s *BillingService) HandleBillingEvent(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling billing event", "type", event.Type, "id", event.ID)
	return nil
}
