package ports

import "context"

type AnalyticsRepository interface {
	SaveMetric(ctx context.Context, metric interface{}) error
	GetMetrics(ctx context.Context) ([]interface{}, error)
}
