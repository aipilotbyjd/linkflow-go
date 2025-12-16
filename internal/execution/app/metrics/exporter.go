package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/linkflow-go/pkg/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Exporter exports metrics to external systems
type Exporter struct {
	collector *Collector
	server    *http.Server
	logger    logger.Logger

	// Export targets
	prometheusEnabled bool
	grafanaEnabled    bool
	customExporters   []CustomExporter
}

// CustomExporter interface for custom metric exporters
type CustomExporter interface {
	Export(ctx context.Context, metrics *ExecutionMetrics) error
	Name() string
}

// ExporterConfig contains configuration for the exporter
type ExporterConfig struct {
	PrometheusPort    int
	PrometheusEnabled bool
	GrafanaEnabled    bool
}

// NewExporter creates a new metrics exporter
func NewExporter(collector *Collector, config ExporterConfig, logger logger.Logger) *Exporter {
	if config.PrometheusPort == 0 {
		config.PrometheusPort = 2112
	}

	exporter := &Exporter{
		collector:         collector,
		logger:            logger,
		prometheusEnabled: config.PrometheusEnabled,
		grafanaEnabled:    config.GrafanaEnabled,
		customExporters:   []CustomExporter{},
	}

	if config.PrometheusEnabled {
		// Setup Prometheus HTTP server
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.HandleFunc("/health", exporter.healthHandler)
		mux.HandleFunc("/ready", exporter.readyHandler)

		exporter.server = &http.Server{
			Addr:    fmt.Sprintf(":%d", config.PrometheusPort),
			Handler: mux,
		}
	}

	return exporter
}

// Start starts the metrics exporter
func (e *Exporter) Start(ctx context.Context) error {
	if e.prometheusEnabled && e.server != nil {
		go func() {
			e.logger.Info("Starting Prometheus metrics server", "addr", e.server.Addr)
			if err := e.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				e.logger.Error("Prometheus server error", "error", err)
			}
		}()
	}

	// Start periodic export to custom exporters
	go e.exportLoop(ctx)

	return nil
}

// Stop stops the metrics exporter
func (e *Exporter) Stop(ctx context.Context) error {
	if e.server != nil {
		e.logger.Info("Stopping Prometheus metrics server")
		return e.server.Shutdown(ctx)
	}
	return nil
}

// AddCustomExporter adds a custom exporter
func (e *Exporter) AddCustomExporter(exporter CustomExporter) {
	e.customExporters = append(e.customExporters, exporter)
	e.logger.Info("Added custom exporter", "name", exporter.Name())
}

// exportLoop periodically exports metrics to custom exporters
func (e *Exporter) exportLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metrics := e.collector.GetMetrics()

			for _, exporter := range e.customExporters {
				if err := exporter.Export(ctx, metrics); err != nil {
					e.logger.Error("Failed to export metrics",
						"exporter", exporter.Name(),
						"error", err,
					)
				}
			}
		}
	}
}

// healthHandler handles health check requests
func (e *Exporter) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// readyHandler handles readiness check requests
func (e *Exporter) readyHandler(w http.ResponseWriter, r *http.Request) {
	// Check if collector is ready
	metrics := e.collector.GetMetrics()
	if metrics != nil {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ready"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Not Ready"))
	}
}

// CloudWatchExporter exports metrics to AWS CloudWatch
type CloudWatchExporter struct {
	namespace string
	region    string
	logger    logger.Logger
}

// NewCloudWatchExporter creates a new CloudWatch exporter
func NewCloudWatchExporter(namespace, region string, logger logger.Logger) *CloudWatchExporter {
	return &CloudWatchExporter{
		namespace: namespace,
		region:    region,
		logger:    logger,
	}
}

// Export exports metrics to CloudWatch
func (cw *CloudWatchExporter) Export(ctx context.Context, metrics *ExecutionMetrics) error {
	// In production, would use AWS SDK to push metrics to CloudWatch
	cw.logger.Debug("Exporting metrics to CloudWatch",
		"namespace", cw.namespace,
		"activeExecutions", metrics.ActiveExecutions,
		"successRate", metrics.SuccessRate,
	)
	return nil
}

// Name returns the exporter name
func (cw *CloudWatchExporter) Name() string {
	return "CloudWatch"
}

// DatadogExporter exports metrics to Datadog
type DatadogExporter struct {
	apiKey string
	appKey string
	logger logger.Logger
}

// NewDatadogExporter creates a new Datadog exporter
func NewDatadogExporter(apiKey, appKey string, logger logger.Logger) *DatadogExporter {
	return &DatadogExporter{
		apiKey: apiKey,
		appKey: appKey,
		logger: logger,
	}
}

// Export exports metrics to Datadog
func (dd *DatadogExporter) Export(ctx context.Context, metrics *ExecutionMetrics) error {
	// In production, would use Datadog API to push metrics
	dd.logger.Debug("Exporting metrics to Datadog",
		"activeExecutions", metrics.ActiveExecutions,
		"throughput", metrics.ThroughputPerMinute,
	)
	return nil
}

// Name returns the exporter name
func (dd *DatadogExporter) Name() string {
	return "Datadog"
}
