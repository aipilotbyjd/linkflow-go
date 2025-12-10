package resolver

import (
	"github.com/linkflow-go/pkg/config"
	"github.com/linkflow-go/pkg/logger"
)

// Resolver is the GraphQL resolver root
type Resolver struct {
	config *config.Config
	logger logger.Logger
}

// NewResolver creates a new GraphQL resolver
func NewResolver(cfg *config.Config, log logger.Logger) *Resolver {
	return &Resolver{
		config: cfg,
		logger: log,
	}
}
