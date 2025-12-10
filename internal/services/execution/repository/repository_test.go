package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/pkg/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *database.DB {
	// Use in-memory SQLite for testing
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	
	// Auto-migrate test schemas
	err = gormDB.AutoMigrate(
		&workflow.WorkflowExecution{},
		&workflow.NodeExecution{},
		&StateTransition{},
		&ExecutionMetric{},
	)
	require.NoError(t, err)
	
	return &database.DB{DB: gormDB}
}

func TestExecutionRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewExecutionRepository(db)
	
	execution := &workflow.WorkflowExecution{
		ID:         uuid.New().String(),
		WorkflowID: uuid.New().String(),
		Status:     workflow.ExecutionPending,
		CreatedBy:  "test-user",
	}
	
	err := repo.Create(context.Background(), execution)
	assert.NoError(t, err)
	assert.NotEmpty(t, execution.ID)
	
	// Verify execution was created
	retrieved, err := repo.GetByID(context.Background(), execution.ID)
	assert.NoError(t, err)
	assert.Equal(t, execution.ID, retrieved.ID)
	assert.Equal(t, execution.WorkflowID, retrieved.WorkflowID)
}

func TestExecutionRepository_UpdateState(t *testing.T) {
	db := setupTestDB(t)
	repo := NewExecutionRepository(db)
	ctx := context.Background()
	
	// Create an execution
	execution := &workflow.WorkflowExecution{
		ID:         uuid.New().String(),
		WorkflowID: uuid.New().String(),
		Status:     workflow.ExecutionPending,
		CreatedBy:  "test-user",
	}
	
	err := repo.Create(ctx, execution)
	require.NoError(t, err)
	
	// Update state to running
	err = repo.UpdateState(ctx, execution.ID, workflow.ExecutionRunning, map[string]interface{}{
		"trigger": "manual",
	})
	assert.NoError(t, err)
	
	// Verify state was updated
	retrieved, err := repo.GetByID(ctx, execution.ID)
	assert.NoError(t, err)
	assert.Equal(t, workflow.ExecutionRunning, retrieved.Status)
	assert.False(t, retrieved.StartedAt.IsZero())
	
	// Update state to completed
	err = repo.UpdateState(ctx, execution.ID, workflow.ExecutionCompleted, map[string]interface{}{
		"result": "success",
	})
	assert.NoError(t, err)
	
	// Verify final state
	retrieved, err = repo.GetByID(ctx, execution.ID)
	assert.NoError(t, err)
	assert.Equal(t, workflow.ExecutionCompleted, retrieved.Status)
	assert.NotNil(t, retrieved.FinishedAt)
	assert.Greater(t, retrieved.ExecutionTime, int64(0))
	
	// Verify state transitions were recorded
	transitions, err := repo.GetStateTransitions(ctx, execution.ID)
	assert.NoError(t, err)
	assert.Len(t, transitions, 3) // Initial + Running + Completed
}

func TestExecutionRepository_RecordMetrics(t *testing.T) {
	db := setupTestDB(t)
	repo := NewExecutionRepository(db)
	ctx := context.Background()
	
	executionID := uuid.New().String()
	
	// Record a single metric
	metric := &ExecutionMetric{
		ExecutionID: executionID,
		NodeID:      "node-1",
		MetricType:  "execution_time",
		Value:       1234.5,
		Unit:        "ms",
	}
	
	err := repo.RecordMetric(ctx, metric)
	assert.NoError(t, err)
	assert.NotEmpty(t, metric.ID)
	
	// Record multiple metrics for a node
	nodeMetrics := map[string]float64{
		"memory_usage": 256.5,
		"cpu_usage":    45.2,
		"throughput":   1000,
	}
	
	err = repo.RecordNodeMetrics(ctx, executionID, nodeMetrics)
	assert.NoError(t, err)
	
	// Retrieve metrics
	metrics, err := repo.GetExecutionMetrics(ctx, executionID, "", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	assert.NoError(t, err)
	assert.Len(t, metrics, 4) // 1 single + 3 node metrics
}

func TestExecutionRepository_GetExecutionStats(t *testing.T) {
	db := setupTestDB(t)
	repo := NewExecutionRepository(db)
	ctx := context.Background()
	
	workflowID := uuid.New().String()
	
	// Create multiple executions with different statuses
	executions := []struct {
		status        string
		executionTime int64
	}{
		{workflow.ExecutionCompleted, 1000},
		{workflow.ExecutionCompleted, 1500},
		{workflow.ExecutionFailed, 500},
		{workflow.ExecutionRunning, 0},
		{workflow.ExecutionCompleted, 2000},
	}
	
	for _, exec := range executions {
		e := &workflow.WorkflowExecution{
			ID:            uuid.New().String(),
			WorkflowID:    workflowID,
			Status:        exec.status,
			ExecutionTime: exec.executionTime,
			StartedAt:     time.Now(),
		}
		
		if exec.status != workflow.ExecutionRunning {
			finishedAt := time.Now()
			e.FinishedAt = &finishedAt
		}
		
		err := db.Create(e).Error
		require.NoError(t, err)
	}
	
	// Get statistics
	stats, err := repo.GetExecutionStats(ctx, workflowID)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), stats.Total)
	assert.Equal(t, int64(3), stats.Successful)
	assert.Equal(t, int64(1), stats.Failed)
	assert.Equal(t, int64(1), stats.Running)
	assert.Greater(t, stats.AverageExecutionTime, float64(0))
	assert.NotNil(t, stats.LastExecutionAt)
}

func BenchmarkExecutionRepository_UpdateState(b *testing.B) {
	db := setupTestDB(&testing.T{})
	repo := NewExecutionRepository(db)
	ctx := context.Background()
	
	// Create executions for benchmarking
	executionIDs := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		execution := &workflow.WorkflowExecution{
			ID:         uuid.New().String(),
			WorkflowID: uuid.New().String(),
			Status:     workflow.ExecutionPending,
			CreatedBy:  "bench-user",
		}
		
		err := db.Create(execution).Error
		if err != nil {
			b.Fatal(err)
		}
		executionIDs[i] = execution.ID
	}
	
	b.ResetTimer()
	
	// Run parallel updates
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			execID := executionIDs[i%len(executionIDs)]
			_ = repo.UpdateState(ctx, execID, workflow.ExecutionRunning, nil)
			i++
		}
	})
}

func BenchmarkExecutionRepository_RecordMetrics(b *testing.B) {
	db := setupTestDB(&testing.T{})
	repo := NewExecutionRepository(db)
	ctx := context.Background()
	
	executionID := uuid.New().String()
	
	b.ResetTimer()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			metric := &ExecutionMetric{
				ExecutionID: executionID,
				NodeID:      "node-" + uuid.New().String(),
				MetricType:  "execution_time",
				Value:       float64(time.Now().UnixNano() % 10000),
				Unit:        "ms",
			}
			
			_ = repo.RecordMetric(ctx, metric)
		}
	})
}

func BenchmarkExecutionRepository_GetExecutionMetrics(b *testing.B) {
	db := setupTestDB(&testing.T{})
	repo := NewExecutionRepository(db)
	ctx := context.Background()
	
	executionID := uuid.New().String()
	
	// Pre-populate metrics
	for i := 0; i < 1000; i++ {
		metric := &ExecutionMetric{
			ID:          uuid.New().String(),
			ExecutionID: executionID,
			NodeID:      "node-" + uuid.New().String(),
			MetricType:  "execution_time",
			Value:       float64(i),
			Unit:        "ms",
			Timestamp:   time.Now().Add(-time.Duration(i) * time.Second),
		}
		_ = db.Create(metric).Error
	}
	
	start := time.Now().Add(-time.Hour)
	end := time.Now()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, _ = repo.GetExecutionMetrics(ctx, executionID, "execution_time", start, end)
	}
}
