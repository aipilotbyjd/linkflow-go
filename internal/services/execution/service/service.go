package service

import (
	"context"

	"github.com/linkflow-go/internal/services/execution/orchestrator"
	"github.com/linkflow-go/internal/services/execution/repository"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type ExecutionService struct {
	repo        *repository.ExecutionRepository
	orchestrator *orchestrator.Orchestrator
	eventBus    events.EventBus
	redis       *redis.Client
	logger      logger.Logger
}

func NewExecutionService(
	repo *repository.ExecutionRepository,
	orchestrator *orchestrator.Orchestrator,
	eventBus events.EventBus,
	redis *redis.Client,
	logger logger.Logger,
) *ExecutionService {
	return &ExecutionService{
		repo:        repo,
		orchestrator: orchestrator,
		eventBus:    eventBus,
		redis:       redis,
		logger:      logger,
	}
}

func (s *ExecutionService) StartExecution(ctx context.Context, workflowID string, data map[string]interface{}) (string, error) {
	s.logger.Info("Starting execution", "workflowId", workflowID)
	return s.orchestrator.Execute(ctx, workflowID, data)
}

func (s *ExecutionService) StopExecution(ctx context.Context, executionID string) error {
	s.logger.Info("Stopping execution", "executionId", executionID)
	return s.orchestrator.Stop(ctx, executionID)
}
