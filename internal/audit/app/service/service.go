package service

import (
	"context"

	"github.com/linkflow-go/internal/audit/ports"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
)

type AuditService struct {
	repo     ports.AuditRepository
	eventBus events.EventBus
	logger   logger.Logger
}

func NewAuditService(repo ports.AuditRepository, eventBus events.EventBus, logger logger.Logger) *AuditService {
	return &AuditService{
		repo:     repo,
		eventBus: eventBus,
		logger:   logger,
	}
}

func (s *AuditService) LogEvent(ctx context.Context, event events.Event) error {
	s.logger.Info("Logging audit event", "type", event.Type, "id", event.ID)
	// Audit logging logic
	return nil
}

func (s *AuditService) GetAuditLogs(ctx context.Context, filters map[string]interface{}) ([]interface{}, error) {
	return s.repo.GetAuditLogs(ctx, filters)
}
