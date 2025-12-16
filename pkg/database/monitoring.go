package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// DBMonitor monitors database performance and health
type DBMonitor struct {
	db              *gorm.DB
	sqlDB           *sql.DB
	logger          *zap.Logger
	metrics         *DBMetrics
	slowQueryLogger *SlowQueryLogger
	mu              sync.RWMutex
	running         bool
	stopChan        chan struct{}
}

// DBMetrics contains Prometheus metrics for database monitoring
type DBMetrics struct {
	ConnectionsActive   prometheus.Gauge
	ConnectionsIdle     prometheus.Gauge
	ConnectionsMax      prometheus.Gauge
	ConnectionsWait     prometheus.Gauge
	QueriesTotal        prometheus.Counter
	QueryDuration       prometheus.Histogram
	SlowQueries         prometheus.Counter
	TransactionsTotal   prometheus.Counter
	TransactionDuration prometheus.Histogram
	ErrorsTotal         prometheus.Counter
	TableSizeBytes      *prometheus.GaugeVec
	IndexUsage          *prometheus.GaugeVec
}

// NewDBMonitor creates a new database monitor
func NewDBMonitor(db *gorm.DB, logger *zap.Logger) (*DBMonitor, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	metrics := &DBMetrics{
		ConnectionsActive: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "database_connections_active",
			Help: "Number of active database connections",
		}),
		ConnectionsIdle: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "database_connections_idle",
			Help: "Number of idle database connections",
		}),
		ConnectionsMax: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "database_connections_max",
			Help: "Maximum number of database connections",
		}),
		ConnectionsWait: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "database_connections_wait",
			Help: "Number of connections waiting",
		}),
		QueriesTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "database_queries_total",
			Help: "Total number of database queries",
		}),
		QueryDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "database_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		}),
		SlowQueries: promauto.NewCounter(prometheus.CounterOpts{
			Name: "database_slow_queries_total",
			Help: "Total number of slow queries",
		}),
		TransactionsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "database_transactions_total",
			Help: "Total number of database transactions",
		}),
		TransactionDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "database_transaction_duration_seconds",
			Help:    "Database transaction duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 12),
		}),
		ErrorsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "database_errors_total",
			Help: "Total number of database errors",
		}),
		TableSizeBytes: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "database_table_size_bytes",
				Help: "Size of database tables in bytes",
			},
			[]string{"table", "schema"},
		),
		IndexUsage: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "database_index_usage_ratio",
				Help: "Index usage ratio (0-1)",
			},
			[]string{"table", "index"},
		),
	}

	monitor := &DBMonitor{
		db:              db,
		sqlDB:           sqlDB,
		logger:          logger,
		metrics:         metrics,
		slowQueryLogger: NewSlowQueryLogger(logger),
		stopChan:        make(chan struct{}),
	}

	// Register callbacks
	monitor.registerCallbacks()

	return monitor, nil
}

// registerCallbacks registers GORM callbacks for monitoring
func (m *DBMonitor) registerCallbacks() {
	// Before query callback
	m.db.Callback().Query().Before("gorm:query").Register("monitor:before_query", func(db *gorm.DB) {
		db.InstanceSet("query_start", time.Now())
	})

	// After query callback
	m.db.Callback().Query().After("gorm:query").Register("monitor:after_query", func(db *gorm.DB) {
		m.recordQuery(db)
	})

	// Error callback
	m.db.Callback().Query().After("gorm:query").Register("monitor:error", func(db *gorm.DB) {
		if db.Error != nil && db.Error != gorm.ErrRecordNotFound {
			m.metrics.ErrorsTotal.Inc()
			m.logger.Error("database error",
				zap.Error(db.Error),
				zap.String("sql", db.Statement.SQL.String()),
			)
		}
	})
}

// recordQuery records query metrics
func (m *DBMonitor) recordQuery(db *gorm.DB) {
	if startTime, ok := db.InstanceGet("query_start"); ok {
		duration := time.Since(startTime.(time.Time))

		m.metrics.QueriesTotal.Inc()
		m.metrics.QueryDuration.Observe(duration.Seconds())

		// Check for slow query
		if duration > SlowQueryThreshold {
			m.metrics.SlowQueries.Inc()
			m.slowQueryLogger.Log(db.Statement.SQL.String(), duration)
		}
	}
}

// Start starts monitoring
func (m *DBMonitor) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("monitor already running")
	}
	m.running = true
	m.mu.Unlock()

	// Start monitoring goroutine
	go m.monitor(ctx)

	return nil
}

// Stop stops monitoring
func (m *DBMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		close(m.stopChan)
		m.running = false
	}
}

// monitor runs the monitoring loop
func (m *DBMonitor) monitor(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.collectMetrics()
		}
	}
}

// collectMetrics collects database metrics
func (m *DBMonitor) collectMetrics() {
	// Connection pool stats
	stats := m.sqlDB.Stats()
	m.metrics.ConnectionsActive.Set(float64(stats.InUse))
	m.metrics.ConnectionsIdle.Set(float64(stats.Idle))
	m.metrics.ConnectionsMax.Set(float64(stats.MaxOpenConnections))
	m.metrics.ConnectionsWait.Set(float64(stats.WaitCount))

	// Table sizes
	m.collectTableSizes()

	// Index usage
	m.collectIndexUsage()
}

// collectTableSizes collects table size metrics
func (m *DBMonitor) collectTableSizes() {
	query := `
		SELECT 
			schemaname,
			tablename,
			pg_total_relation_size(schemaname||'.'||tablename) as size_bytes
		FROM pg_tables
		WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
	`

	type tableSize struct {
		Schema string
		Table  string
		Size   int64
	}

	var sizes []tableSize
	if err := m.db.Raw(query).Scan(&sizes).Error; err != nil {
		m.logger.Error("failed to collect table sizes", zap.Error(err))
		return
	}

	for _, size := range sizes {
		m.metrics.TableSizeBytes.WithLabelValues(size.Table, size.Schema).Set(float64(size.Size))
	}
}

// collectIndexUsage collects index usage metrics
func (m *DBMonitor) collectIndexUsage() {
	query := `
		SELECT
			schemaname || '.' || tablename as table_name,
			indexrelname as index_name,
			CASE 
				WHEN idx_scan = 0 THEN 0
				ELSE ROUND(100.0 * idx_scan / (seq_scan + idx_scan), 2)
			END as usage_ratio
		FROM pg_stat_user_indexes ui
		JOIN pg_stat_user_tables ut ON ui.schemaname = ut.schemaname AND ui.tablename = ut.tablename
		WHERE ut.schemaname NOT IN ('pg_catalog', 'information_schema')
			AND (ut.seq_scan + ui.idx_scan) > 0
	`

	type indexUsage struct {
		TableName  string
		IndexName  string
		UsageRatio float64
	}

	var usages []indexUsage
	if err := m.db.Raw(query).Scan(&usages).Error; err != nil {
		m.logger.Error("failed to collect index usage", zap.Error(err))
		return
	}

	for _, usage := range usages {
		m.metrics.IndexUsage.WithLabelValues(usage.TableName, usage.IndexName).Set(usage.UsageRatio / 100)
	}
}

// GetSlowQueries returns recent slow queries
func (m *DBMonitor) GetSlowQueries(limit int) []SlowQueryInfo {
	return m.slowQueryLogger.GetRecent(limit)
}

// GetConnectionPoolStats returns connection pool statistics
func (m *DBMonitor) GetConnectionPoolStats() sql.DBStats {
	return m.sqlDB.Stats()
}

// GetQueryStats returns query statistics
func (m *DBMonitor) GetQueryStats(ctx context.Context) (*QueryStats, error) {
	stats := &QueryStats{}

	// Get total queries
	query := `
		SELECT 
			COUNT(*) as total_queries,
			AVG(mean_exec_time) as avg_exec_time,
			MAX(mean_exec_time) as max_exec_time,
			SUM(calls) as total_calls
		FROM pg_stat_statements
	`

	err := m.db.Raw(query).Scan(stats).Error
	if err != nil {
		// pg_stat_statements might not be enabled
		m.logger.Warn("failed to get query stats", zap.Error(err))
	}

	// Get cache hit ratio
	cacheQuery := `
		SELECT 
			sum(heap_blks_hit) / nullif(sum(heap_blks_hit) + sum(heap_blks_read), 0) as cache_hit_ratio
		FROM pg_statio_user_tables
	`

	var cacheHitRatio *float64
	if err := m.db.Raw(cacheQuery).Scan(&cacheHitRatio).Error; err == nil && cacheHitRatio != nil {
		stats.CacheHitRatio = *cacheHitRatio
	}

	return stats, nil
}

// GetTableStats returns statistics for a specific table
func (m *DBMonitor) GetTableStats(ctx context.Context, tableName string) (*TableStats, error) {
	stats := &TableStats{
		TableName: tableName,
	}

	// Get row count and size
	sizeQuery := `
		SELECT 
			n_live_tup as row_count,
			pg_size_pretty(pg_total_relation_size($1)) as total_size,
			pg_size_pretty(pg_relation_size($1)) as table_size,
			pg_size_pretty(pg_indexes_size($1)) as indexes_size
		FROM pg_stat_user_tables
		WHERE tablename = $1
	`

	if err := m.db.Raw(sizeQuery, tableName).Scan(stats).Error; err != nil {
		return nil, err
	}

	// Get index information
	indexQuery := `
		SELECT 
			indexname,
			idx_scan as scans,
			idx_tup_read as tuples_read,
			idx_tup_fetch as tuples_fetched
		FROM pg_stat_user_indexes
		WHERE tablename = $1
	`

	if err := m.db.Raw(indexQuery, tableName).Scan(&stats.Indexes).Error; err != nil {
		m.logger.Warn("failed to get index stats", zap.Error(err))
	}

	return stats, nil
}

// QueryStats contains query statistics
type QueryStats struct {
	TotalQueries  int64   `json:"totalQueries"`
	AvgExecTime   float64 `json:"avgExecTime"`
	MaxExecTime   float64 `json:"maxExecTime"`
	TotalCalls    int64   `json:"totalCalls"`
	CacheHitRatio float64 `json:"cacheHitRatio"`
}

// TableStats contains table statistics
type TableStats struct {
	TableName   string       `json:"tableName"`
	RowCount    int64        `json:"rowCount"`
	TotalSize   string       `json:"totalSize"`
	TableSize   string       `json:"tableSize"`
	IndexesSize string       `json:"indexesSize"`
	Indexes     []IndexStats `json:"indexes"`
}

// IndexStats contains index statistics
type IndexStats struct {
	IndexName     string `json:"indexName"`
	Scans         int64  `json:"scans"`
	TuplesRead    int64  `json:"tuplesRead"`
	TuplesFetched int64  `json:"tuplesFetched"`
}

// HealthCheck performs a database health check
func (m *DBMonitor) HealthCheck(ctx context.Context) (*HealthStatus, error) {
	status := &HealthStatus{
		Timestamp: time.Now(),
		Healthy:   true,
	}

	// Check connection
	if err := m.sqlDB.PingContext(ctx); err != nil {
		status.Healthy = false
		status.Issues = append(status.Issues, fmt.Sprintf("connection failed: %v", err))
	}

	// Check connection pool
	stats := m.sqlDB.Stats()
	if stats.OpenConnections > 0 && float64(stats.InUse)/float64(stats.OpenConnections) > 0.9 {
		status.Warnings = append(status.Warnings, "connection pool utilization > 90%")
	}

	// Check for long-running queries
	var longQueries int64
	longQueryCheck := `
		SELECT COUNT(*) 
		FROM pg_stat_activity 
		WHERE state != 'idle' 
		AND query_start < NOW() - INTERVAL '1 minute'
	`

	if err := m.db.Raw(longQueryCheck).Scan(&longQueries).Error; err == nil && longQueries > 0 {
		status.Warnings = append(status.Warnings,
			fmt.Sprintf("%d long-running queries (> 1 minute)", longQueries))
	}

	// Check replication lag (if applicable)
	var replicationLag *int64
	lagQuery := `
		SELECT EXTRACT(EPOCH FROM (NOW() - pg_last_xact_replay_timestamp()))::INT as lag_seconds
		FROM pg_stat_replication
	`

	if err := m.db.Raw(lagQuery).Scan(&replicationLag).Error; err == nil && replicationLag != nil && *replicationLag > 10 {
		status.Warnings = append(status.Warnings,
			fmt.Sprintf("replication lag: %d seconds", *replicationLag))
	}

	status.ConnectionPool = stats

	return status, nil
}

// HealthStatus represents database health status
type HealthStatus struct {
	Timestamp      time.Time   `json:"timestamp"`
	Healthy        bool        `json:"healthy"`
	Issues         []string    `json:"issues,omitempty"`
	Warnings       []string    `json:"warnings,omitempty"`
	ConnectionPool sql.DBStats `json:"connectionPool"`
}
