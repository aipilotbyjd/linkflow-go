package repository

import (
	"context"
	
	"github.com/linkflow-go/internal/domain/node"
	"github.com/linkflow-go/pkg/database"
)

type NodeRepository struct {
	db *database.DB
}

func NewNodeRepository(db *database.DB) *NodeRepository {
	return &NodeRepository{db: db}
}

func (r *NodeRepository) CreateNodeType(ctx context.Context, nodeType *node.NodeType) error {
	return r.db.WithContext(ctx).Create(nodeType).Error
}

func (r *NodeRepository) GetNodeType(ctx context.Context, nodeType string) (*node.NodeType, error) {
	var n node.NodeType
	err := r.db.WithContext(ctx).Where("type = ?", nodeType).First(&n).Error
	return &n, err
}

func (r *NodeRepository) ListNodeTypes(ctx context.Context) ([]*node.NodeType, error) {
	var nodes []*node.NodeType
	err := r.db.WithContext(ctx).Find(&nodes).Error
	return nodes, err
}

func (r *NodeRepository) UpdateNodeType(ctx context.Context, nodeType *node.NodeType) error {
	return r.db.WithContext(ctx).Save(nodeType).Error
}

func (r *NodeRepository) DeleteNodeType(ctx context.Context, nodeType string) error {
	return r.db.WithContext(ctx).Where("type = ?", nodeType).Delete(&node.NodeType{}).Error
}
