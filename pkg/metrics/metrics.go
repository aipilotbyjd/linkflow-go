package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Common metrics for all services
var (
	// HTTP metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"service", "method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method", "path"},
	)

	// Workflow metrics
	WorkflowsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "workflows_total",
			Help: "Total number of workflows",
		},
		[]string{"status"},
	)

	WorkflowExecutionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "workflow_executions_total",
			Help: "Total number of workflow executions",
		},
		[]string{"workflow_id", "status", "trigger"},
	)

	WorkflowExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "workflow_execution_duration_seconds",
			Help:    "Workflow execution duration in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
		},
		[]string{"workflow_id"},
	)

	// Node metrics
	NodeExecutionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_executions_total",
			Help: "Total number of node executions",
		},
		[]string{"node_type", "status"},
	)

	NodeExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "node_execution_duration_seconds",
			Help:    "Node execution duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10},
		},
		[]string{"node_type"},
	)

	// Database metrics
	DatabaseConnectionsActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "database_connections_active",
			Help: "Number of active database connections",
		},
		[]string{"service"},
	)

	DatabaseQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "database_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
		},
		[]string{"service", "operation"},
	)

	// Event bus metrics
	EventsPublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "events_published_total",
			Help: "Total number of events published",
		},
		[]string{"event_type"},
	)

	EventsConsumed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "events_consumed_total",
			Help: "Total number of events consumed",
		},
		[]string{"event_type", "consumer"},
	)

	// Cache metrics
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache"},
	)

	CacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache"},
	)
)

// RecordHTTPRequest records an HTTP request metric
func RecordHTTPRequest(service, method, path, status string) {
	HTTPRequestsTotal.WithLabelValues(service, method, path, status).Inc()
}

// RecordHTTPDuration records HTTP request duration
func RecordHTTPDuration(service, method, path string, duration float64) {
	HTTPRequestDuration.WithLabelValues(service, method, path).Observe(duration)
}

// RecordWorkflowExecution records a workflow execution
func RecordWorkflowExecution(workflowID, status, trigger string) {
	WorkflowExecutionsTotal.WithLabelValues(workflowID, status, trigger).Inc()
}

// RecordWorkflowDuration records workflow execution duration
func RecordWorkflowDuration(workflowID string, duration float64) {
	WorkflowExecutionDuration.WithLabelValues(workflowID).Observe(duration)
}

// RecordNodeExecution records a node execution
func RecordNodeExecution(nodeType, status string) {
	NodeExecutionsTotal.WithLabelValues(nodeType, status).Inc()
}

// RecordNodeDuration records node execution duration
func RecordNodeDuration(nodeType string, duration float64) {
	NodeExecutionDuration.WithLabelValues(nodeType).Observe(duration)
}
