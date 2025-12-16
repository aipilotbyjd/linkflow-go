package ports

import (
	"context"

	node "github.com/linkflow-go/internal/node/domain"
)

type NodeRepository interface {
	CreateNodeType(ctx context.Context, nodeType *node.NodeType) error
	GetNodeType(ctx context.Context, nodeType string) (*node.NodeType, error)
	GetAllNodeTypes(ctx context.Context) ([]*node.NodeType, error)
	UpdateNodeType(ctx context.Context, nodeType *node.NodeType) error
	DeleteNodeType(ctx context.Context, nodeType string) error
}
