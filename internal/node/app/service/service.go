package service

import (
	"context"

	"github.com/linkflow-go/internal/node/app/registry"
	"github.com/linkflow-go/internal/node/ports"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
)

type NodeService struct {
	repo     ports.NodeRepository
	registry *registry.NodeRegistry
	eventBus events.EventBus
	logger   logger.Logger
}

func NewNodeService(
	repo ports.NodeRepository,
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
	nodeTypes := s.registry.GetAllNodeTypes()
	result := make([]interface{}, len(nodeTypes))
	for i, nt := range nodeTypes {
		result[i] = nt
	}
	return result, nil
}

func (s *NodeService) ExecuteNode(ctx context.Context, nodeType string, input map[string]interface{}) (map[string]interface{}, error) {
	s.logger.Info("Executing node", "type", nodeType)
	// TODO: Implement node execution logic
	return input, nil
}
