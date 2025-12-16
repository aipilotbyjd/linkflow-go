package aggregator

import (
	"context"
	"time"

	"github.com/linkflow-go/internal/analytics/ports"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type MetricsAggregator struct {
	repo   ports.AnalyticsRepository
	redis  *redis.Client
	logger logger.Logger
	stopCh chan struct{}
}

func NewMetricsAggregator(
	repo ports.AnalyticsRepository,
	redis *redis.Client,
	logger logger.Logger,
) *MetricsAggregator {
	return &MetricsAggregator{
		repo:   repo,
		redis:  redis,
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

func (a *MetricsAggregator) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.aggregate(ctx)
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (a *MetricsAggregator) Stop() {
	close(a.stopCh)
}

func (a *MetricsAggregator) aggregate(ctx context.Context) {
	a.logger.Info("Aggregating metrics")
	// Aggregation logic here
}
