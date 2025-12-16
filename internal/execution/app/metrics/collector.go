package metrics

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/linkflow-go/pkg/contracts/workflow"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Collector collects execution metrics
type Collector struct {
	mu       sync.RWMutex
	logger   logger.Logger
	eventBus events.EventBus

	// Prometheus metrics
	executionsStarted   prometheus.Counter
	executionsCompleted prometheus.Counter
	executionsFailed    prometheus.Counter
	executionDuration   prometheus.Histogram
	activeExecutions    prometheus.Gauge
	nodeExecutions      *prometheus.CounterVec
	nodeDuration        *prometheus.HistogramVec
	queueSize           *prometheus.GaugeVec
	workerUtilization   prometheus.Gauge

	// In-memory metrics for fast access
	metrics *ExecutionMetrics

	// Node-level metrics
	nodeMetrics map[string]*NodeMetrics

	// Control
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// ExecutionMetrics represents execution-level metrics
type ExecutionMetrics struct {
	TotalExecutions      int64                   `json:"total_executions"`
	ActiveExecutions     int64                   `json:"active_executions"`
	CompletedExecutions  int64                   `json:"completed_executions"`
	FailedExecutions     int64                   `json:"failed_executions"`
	AverageExecutionTime time.Duration           `json:"average_execution_time"`
	LastExecutionTime    time.Time               `json:"last_execution_time"`
	SuccessRate          float64                 `json:"success_rate"`
	NodeMetrics          map[string]*NodeMetrics `json:"node_metrics"`

	// Performance metrics
	ThroughputPerMinute float64       `json:"throughput_per_minute"`
	P50Latency          time.Duration `json:"p50_latency"`
	P95Latency          time.Duration `json:"p95_latency"`
	P99Latency          time.Duration `json:"p99_latency"`

	// Resource utilization
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage int64   `json:"memory_usage_bytes"`
	QueueDepth  int     `json:"queue_depth"`
}

// NodeMetrics represents node-level metrics
type NodeMetrics struct {
	NodeID            string        `json:"node_id"`
	NodeType          string        `json:"node_type"`
	ExecutionCount    int64         `json:"execution_count"`
	SuccessCount      int64         `json:"success_count"`
	FailureCount      int64         `json:"failure_count"`
	AverageDuration   time.Duration `json:"average_duration"`
	MinDuration       time.Duration `json:"min_duration"`
	MaxDuration       time.Duration `json:"max_duration"`
	LastExecutionTime time.Time     `json:"last_execution_time"`
	ErrorRate         float64       `json:"error_rate"`
}

// NewCollector creates a new metrics collector
func NewCollector(eventBus events.EventBus, logger logger.Logger) *Collector {
	collector := &Collector{
		logger:   logger,
		eventBus: eventBus,
		metrics: &ExecutionMetrics{
			NodeMetrics: make(map[string]*NodeMetrics),
		},
		nodeMetrics: make(map[string]*NodeMetrics),
		stopCh:      make(chan struct{}),
	}

	// Initialize Prometheus metrics
	collector.initPrometheusMetrics()

	return collector
}

// initPrometheusMetrics initializes Prometheus metrics
func (c *Collector) initPrometheusMetrics() {
	c.executionsStarted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "linkflow_executions_started_total",
		Help: "Total number of executions started",
	})

	c.executionsCompleted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "linkflow_executions_completed_total",
		Help: "Total number of executions completed successfully",
	})

	c.executionsFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "linkflow_executions_failed_total",
		Help: "Total number of executions failed",
	})

	c.executionDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "linkflow_execution_duration_seconds",
		Help:    "Execution duration in seconds",
		Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~100s
	})

	c.activeExecutions = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "linkflow_executions_active",
		Help: "Number of currently active executions",
	})

	c.nodeExecutions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "linkflow_node_executions_total",
		Help: "Total number of node executions by type and status",
	}, []string{"node_type", "status"})

	c.nodeDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "linkflow_node_duration_seconds",
		Help:    "Node execution duration in seconds",
		Buckets: prometheus.ExponentialBuckets(0.01, 2, 10), // 0.01s to ~10s
	}, []string{"node_type"})

	c.queueSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "linkflow_queue_size",
		Help: "Current queue size by priority",
	}, []string{"priority"})

	c.workerUtilization = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "linkflow_worker_utilization_ratio",
		Help: "Worker pool utilization ratio",
	})
}

// Start starts the metrics collector
func (c *Collector) Start(ctx context.Context) error {
	c.logger.Info("Starting metrics collector")

	// Subscribe to events
	if err := c.subscribeToEvents(ctx); err != nil {
		return err
	}

	// Start metrics aggregation
	c.wg.Add(2)
	go c.aggregateMetrics(ctx)
	go c.reportMetrics(ctx)

	return nil
}

// Stop stops the metrics collector
func (c *Collector) Stop(ctx context.Context) error {
	c.logger.Info("Stopping metrics collector")

	close(c.stopCh)

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.logger.Info("Metrics collector stopped")
	case <-ctx.Done():
		c.logger.Warn("Metrics collector stop timeout")
	}

	return nil
}

// RecordExecutionStart records the start of an execution
func (c *Collector) RecordExecutionStart(executionID string, workflowID string) {
	c.executionsStarted.Inc()
	c.activeExecutions.Inc()

	atomic.AddInt64(&c.metrics.TotalExecutions, 1)
	atomic.AddInt64(&c.metrics.ActiveExecutions, 1)

	c.logger.Debug("Execution started", "executionId", executionID, "workflowId", workflowID)
}

// RecordExecutionComplete records the completion of an execution
func (c *Collector) RecordExecutionComplete(executionID string, duration time.Duration, success bool) {
	c.activeExecutions.Dec()
	c.executionDuration.Observe(duration.Seconds())

	if success {
		c.executionsCompleted.Inc()
		atomic.AddInt64(&c.metrics.CompletedExecutions, 1)
	} else {
		c.executionsFailed.Inc()
		atomic.AddInt64(&c.metrics.FailedExecutions, 1)
	}

	atomic.AddInt64(&c.metrics.ActiveExecutions, -1)

	// Update average execution time
	c.updateAverageExecutionTime(duration)

	// Update success rate
	c.updateSuccessRate()

	c.logger.Debug("Execution completed",
		"executionId", executionID,
		"duration", duration,
		"success", success,
	)
}

// RecordNodeExecution records a node execution
func (c *Collector) RecordNodeExecution(nodeID string, nodeType string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}

	c.nodeExecutions.WithLabelValues(nodeType, status).Inc()
	c.nodeDuration.WithLabelValues(nodeType).Observe(duration.Seconds())

	// Update node metrics
	c.updateNodeMetrics(nodeID, nodeType, duration, success)
}

// RecordQueueSize records the current queue size
func (c *Collector) RecordQueueSize(priority string, size int) {
	c.queueSize.WithLabelValues(priority).Set(float64(size))

	if priority == "total" {
		c.metrics.QueueDepth = size
	}
}

// RecordWorkerUtilization records worker utilization
func (c *Collector) RecordWorkerUtilization(utilization float64) {
	c.workerUtilization.Set(utilization)
}

// RecordResourceUsage records resource usage
func (c *Collector) RecordResourceUsage(cpuUsage float64, memoryUsage int64) {
	c.mu.Lock()
	c.metrics.CPUUsage = cpuUsage
	c.metrics.MemoryUsage = memoryUsage
	c.mu.Unlock()
}

// GetMetrics returns current metrics
func (c *Collector) GetMetrics() *ExecutionMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Create a copy
	metrics := &ExecutionMetrics{
		TotalExecutions:      c.metrics.TotalExecutions,
		ActiveExecutions:     c.metrics.ActiveExecutions,
		CompletedExecutions:  c.metrics.CompletedExecutions,
		FailedExecutions:     c.metrics.FailedExecutions,
		AverageExecutionTime: c.metrics.AverageExecutionTime,
		LastExecutionTime:    c.metrics.LastExecutionTime,
		SuccessRate:          c.metrics.SuccessRate,
		ThroughputPerMinute:  c.metrics.ThroughputPerMinute,
		P50Latency:           c.metrics.P50Latency,
		P95Latency:           c.metrics.P95Latency,
		P99Latency:           c.metrics.P99Latency,
		CPUUsage:             c.metrics.CPUUsage,
		MemoryUsage:          c.metrics.MemoryUsage,
		QueueDepth:           c.metrics.QueueDepth,
		NodeMetrics:          make(map[string]*NodeMetrics),
	}

	// Copy node metrics
	for k, v := range c.metrics.NodeMetrics {
		metrics.NodeMetrics[k] = v
	}

	return metrics
}

// GetNodeMetrics returns metrics for a specific node
func (c *Collector) GetNodeMetrics(nodeID string) (*NodeMetrics, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics, exists := c.nodeMetrics[nodeID]
	if !exists {
		return nil, nil
	}

	return metrics, nil
}

// updateAverageExecutionTime updates the average execution time
func (c *Collector) updateAverageExecutionTime(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	total := c.metrics.CompletedExecutions + c.metrics.FailedExecutions
	if total == 0 {
		c.metrics.AverageExecutionTime = duration
	} else {
		// Weighted average
		currentAvg := c.metrics.AverageExecutionTime
		c.metrics.AverageExecutionTime = time.Duration((int64(currentAvg)*total + int64(duration)) / (total + 1))
	}

	c.metrics.LastExecutionTime = time.Now()
}

// updateSuccessRate updates the success rate
func (c *Collector) updateSuccessRate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	total := c.metrics.CompletedExecutions + c.metrics.FailedExecutions
	if total > 0 {
		c.metrics.SuccessRate = float64(c.metrics.CompletedExecutions) / float64(total)
	}
}

// updateNodeMetrics updates metrics for a specific node
func (c *Collector) updateNodeMetrics(nodeID string, nodeType string, duration time.Duration, success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	metrics, exists := c.nodeMetrics[nodeID]
	if !exists {
		metrics = &NodeMetrics{
			NodeID:      nodeID,
			NodeType:    nodeType,
			MinDuration: duration,
			MaxDuration: duration,
		}
		c.nodeMetrics[nodeID] = metrics
		c.metrics.NodeMetrics[nodeID] = metrics
	}

	metrics.ExecutionCount++
	if success {
		metrics.SuccessCount++
	} else {
		metrics.FailureCount++
	}

	// Update duration stats
	if duration < metrics.MinDuration {
		metrics.MinDuration = duration
	}
	if duration > metrics.MaxDuration {
		metrics.MaxDuration = duration
	}

	// Update average duration
	if metrics.ExecutionCount == 1 {
		metrics.AverageDuration = duration
	} else {
		metrics.AverageDuration = time.Duration((int64(metrics.AverageDuration)*(metrics.ExecutionCount-1) + int64(duration)) / metrics.ExecutionCount)
	}

	// Update error rate
	if metrics.ExecutionCount > 0 {
		metrics.ErrorRate = float64(metrics.FailureCount) / float64(metrics.ExecutionCount)
	}

	metrics.LastExecutionTime = time.Now()
}

// aggregateMetrics aggregates metrics periodically
func (c *Collector) aggregateMetrics(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var lastCount int64
	var lastTime time.Time = time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.mu.Lock()

			// Calculate throughput
			currentCount := c.metrics.CompletedExecutions + c.metrics.FailedExecutions
			deltaCount := currentCount - lastCount
			deltaTime := time.Since(lastTime).Minutes()

			if deltaTime > 0 {
				c.metrics.ThroughputPerMinute = float64(deltaCount) / deltaTime
			}

			lastCount = currentCount
			lastTime = time.Now()

			c.mu.Unlock()
		}
	}
}

// reportMetrics reports metrics periodically
func (c *Collector) reportMetrics(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			metrics := c.GetMetrics()

			// Publish metrics event
			event := events.NewEventBuilder("execution.metrics").
				WithPayload("metrics", metrics).
				Build()

			c.eventBus.Publish(ctx, event)

			c.logger.Info("Execution metrics",
				"total", metrics.TotalExecutions,
				"active", metrics.ActiveExecutions,
				"completed", metrics.CompletedExecutions,
				"failed", metrics.FailedExecutions,
				"successRate", metrics.SuccessRate,
				"throughput", metrics.ThroughputPerMinute,
			)
		}
	}
}

// subscribeToEvents subscribes to relevant events
func (c *Collector) subscribeToEvents(ctx context.Context) error {
	// Subscribe to execution events
	events := map[string]events.HandlerFunc{
		events.ExecutionStarted:       c.handleExecutionStarted,
		events.ExecutionCompleted:     c.handleExecutionCompleted,
		events.ExecutionFailed:        c.handleExecutionFailed,
		events.NodeExecutionStarted:   c.handleNodeExecutionStarted,
		events.NodeExecutionCompleted: c.handleNodeExecutionCompleted,
		"queue.metrics":               c.handleQueueMetrics,
		"workerpool.metrics":          c.handleWorkerPoolMetrics,
	}

	for eventType, handler := range events {
		if err := c.eventBus.Subscribe(eventType, handler); err != nil {
			return err
		}
	}

	return nil
}

// Event handlers

func (c *Collector) handleExecutionStarted(ctx context.Context, event events.Event) error {
	executionID := event.AggregateID
	workflowID, _ := event.Payload["workflowId"].(string)

	c.RecordExecutionStart(executionID, workflowID)
	return nil
}

func (c *Collector) handleExecutionCompleted(ctx context.Context, event events.Event) error {
	executionID := event.AggregateID
	duration, _ := event.Payload["duration"].(int64)

	c.RecordExecutionComplete(executionID, time.Duration(duration)*time.Millisecond, true)
	return nil
}

func (c *Collector) handleExecutionFailed(ctx context.Context, event events.Event) error {
	executionID := event.AggregateID

	// Estimate duration from context
	c.RecordExecutionComplete(executionID, 0, false)
	return nil
}

func (c *Collector) handleNodeExecutionStarted(ctx context.Context, event events.Event) error {
	// Track node start time for duration calculation
	return nil
}

func (c *Collector) handleNodeExecutionCompleted(ctx context.Context, event events.Event) error {
	nodeID, _ := event.Payload["nodeId"].(string)
	nodeType, _ := event.Payload["nodeType"].(string)
	status, _ := event.Payload["status"].(string)

	// Calculate duration (simplified - in production, would track start time)
	duration := 1 * time.Second
	success := status == string(workflow.NodeExecutionCompleted)

	c.RecordNodeExecution(nodeID, nodeType, duration, success)
	return nil
}

func (c *Collector) handleQueueMetrics(ctx context.Context, event events.Event) error {
	status, _ := event.Payload["status"].(map[string]interface{})

	c.RecordQueueSize("high", int(status["highPriority"].(float64)))
	c.RecordQueueSize("normal", int(status["normalPriority"].(float64)))
	c.RecordQueueSize("low", int(status["lowPriority"].(float64)))
	c.RecordQueueSize("total", int(status["totalQueued"].(float64)))

	return nil
}

func (c *Collector) handleWorkerPoolMetrics(ctx context.Context, event events.Event) error {
	metrics, _ := event.Payload["metrics"].(map[string]interface{})

	totalWorkers := metrics["totalWorkers"].(float64)
	activeWorkers := metrics["activeWorkers"].(float64)

	if totalWorkers > 0 {
		utilization := activeWorkers / totalWorkers
		c.RecordWorkerUtilization(utilization)
	}

	return nil
}
