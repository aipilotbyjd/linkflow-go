package service

import (
	"context"

	"github.com/linkflow-go/internal/search/ports"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type SearchService struct {
	indexer  ports.Indexer
	eventBus events.EventBus
	redis    *redis.Client
	logger   logger.Logger
}

func NewSearchService(
	indexer ports.Indexer,
	eventBus events.EventBus,
	redis *redis.Client,
	logger logger.Logger,
) *SearchService {
	return &SearchService{
		indexer:  indexer,
		eventBus: eventBus,
		redis:    redis,
		logger:   logger,
	}
}

func (s *SearchService) HandleIndexEvent(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling index event", "type", event.Type, "id", event.ID)
	// Index document in Elasticsearch
	return s.indexer.IndexDocument(ctx, event)
}

func (s *SearchService) Search(ctx context.Context, query string) ([]interface{}, error) {
	// Perform search in Elasticsearch
	return []interface{}{}, nil
}
