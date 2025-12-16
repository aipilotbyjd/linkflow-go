package repository

import (
	"context"

	"github.com/linkflow-go/pkg/database"
)

type AnalyticsRepository struct {
	db *database.DB
}

func NewAnalyticsRepository(db *database.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

func (r *AnalyticsRepository) SaveMetric(ctx context.Context, metric interface{}) error {
	return nil
}

func (r *AnalyticsRepository) GetMetrics(ctx context.Context) ([]interface{}, error) {
	return []interface{}{}, nil
}
