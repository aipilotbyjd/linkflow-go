package service

import (
	"context"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/linkflow-go/internal/services/search/indexer"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type SearchService struct {
	esClient *elasticsearch.Client
	indexer  *indexer.Indexer
	eventBus events.EventBus
	redis    *redis.Client
	logger   logger.Logger
}

func NewSearchService(
	esClient *elasticsearch.Client,
	indexer *indexer.Indexer,
	eventBus events.EventBus,
	redis *redis.Client,
	logger logger.Logger,
) *SearchService {
	return &SearchService{
		esClient: esClient,
		indexer:  indexer,
		eventBus: eventBus,
		redis:    redis,
		logger:   logger,
	}
}

func (s *SearchService) HandleIndexEvent(ctx context.Context, event interface{}) error {
	s.logger.Info("Handling index event")
	// Index document in Elasticsearch
	return s.indexer.IndexDocument(ctx, event)
}

func (s *SearchService) Search(ctx context.Context, query string) ([]interface{}, error) {
	// Perform search in Elasticsearch
	return []interface{}{}, nil
}
