package ports

import (
	"context"
)

type MetricsAggregator interface {
	Start(ctx context.Context)
	Stop()
}
