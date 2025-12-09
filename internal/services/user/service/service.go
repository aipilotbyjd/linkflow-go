package service

import (
	"context"

	"github.com/linkflow-go/internal/services/user/repository"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type UserService struct {
	repo     *repository.UserRepository
	eventBus events.EventBus
	redis    *redis.Client
	logger   logger.Logger
}

func NewUserService(
	repo *repository.UserRepository,
	eventBus events.EventBus,
	redis *redis.Client,
	logger logger.Logger,
) *UserService {
	return &UserService{
		repo:     repo,
		eventBus: eventBus,
		redis:    redis,
		logger:   logger,
	}
}

func (s *UserService) CheckReady() error {
	// Check service readiness
	return nil
}

func (s *UserService) HandleUserRegistered(ctx context.Context, event interface{}) error {
	s.logger.Info("Handling user registered event")
	return nil
}

func (s *UserService) HandleUserDeleted(ctx context.Context, event interface{}) error {
	s.logger.Info("Handling user deleted event")
	return nil
}

func (s *UserService) HandleWorkflowCreated(ctx context.Context, event interface{}) error {
	s.logger.Info("Handling workflow created event for ownership tracking")
	return nil
}
