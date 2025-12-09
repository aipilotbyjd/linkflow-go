package service

import (
	"context"

	"github.com/linkflow-go/internal/services/node/registry"
	"github.com/linkflow-go/internal/services/node/repository"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
)

type NodeService struct {
	repo     *repository.NodeRepository
	registry *registry.NodeRegistry
	eventBus events.EventBus
	logger   logger.Logger
}

func NewNodeService(
	repo *repository.NodeRepository,
	registry *registry.NodeRegistry,
	eventBus events.EventBus,
	logger logger.Logger,
) *NodeService {
	return &NodeService{
		repo:     repo,
		registry: registry,
		eventBus: eventBus,
		logger:   logger,
	}
}

func (s *NodeService) GetNodeTypes(ctx context.Context) ([]interface{}, error) {
	return s.registry.GetAllNodeTypes(), nil
}

func (s *NodeService) ExecuteNode(ctx context.Context, nodeType string, input map[string]interface{}) (map[string]interface{}, error) {
	s.logger.Info("Executing node", "type", nodeType)
	return s.registry.ExecuteNode(ctx, nodeType, input)
}
