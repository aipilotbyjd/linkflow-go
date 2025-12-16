package database

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SlowQueryThreshold defines the threshold for slow queries
const SlowQueryThreshold = 100 * time.Millisecond

// SlowQueryLogger logs and tracks slow queries
type SlowQueryLogger struct {
	logger     *zap.Logger
	queries    []SlowQueryInfo
	maxQueries int
	mu         sync.RWMutex
}

// SlowQueryInfo contains information about a slow query
type SlowQueryInfo struct {
	Query     string        `json:"query"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
	CallStack string        `json:"callStack,omitempty"`
}

// NewSlowQueryLogger creates a new slow query logger
func NewSlowQueryLogger(logger *zap.Logger) *SlowQueryLogger {
	return &SlowQueryLogger{
		logger:     logger,
		maxQueries: 100,
		queries:    make([]SlowQueryInfo, 0, 100),
	}
}

// Log logs a slow query
func (l *SlowQueryLogger) Log(query string, duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Log to logger
	l.logger.Warn("slow query detected",
		zap.String("query", query),
		zap.Duration("duration", duration),
		zap.String("threshold", SlowQueryThreshold.String()),
	)

	// Store in memory
	info := SlowQueryInfo{
		Query:     query,
		Duration:  duration,
		Timestamp: time.Now(),
	}

	l.queries = append(l.queries, info)

	// Keep only last N queries
	if len(l.queries) > l.maxQueries {
		l.queries = l.queries[len(l.queries)-l.maxQueries:]
	}
}

// GetRecent returns recent slow queries
func (l *SlowQueryLogger) GetRecent(limit int) []SlowQueryInfo {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if limit <= 0 || limit > len(l.queries) {
		limit = len(l.queries)
	}

	// Return most recent queries
	start := len(l.queries) - limit
	if start < 0 {
		start = 0
	}

	result := make([]SlowQueryInfo, limit)
	copy(result, l.queries[start:])

	return result
}

// GetStats returns statistics about slow queries
func (l *SlowQueryLogger) GetStats() SlowQueryStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := SlowQueryStats{
		TotalCount: len(l.queries),
	}

	if len(l.queries) == 0 {
		return stats
	}

	// Calculate statistics
	var totalDuration time.Duration
	stats.MaxDuration = l.queries[0].Duration
	stats.MinDuration = l.queries[0].Duration

	queryCount := make(map[string]int)

	for _, q := range l.queries {
		totalDuration += q.Duration

		if q.Duration > stats.MaxDuration {
			stats.MaxDuration = q.Duration
		}

		if q.Duration < stats.MinDuration {
			stats.MinDuration = q.Duration
		}

		// Track query patterns (simplified - just first 50 chars)
		pattern := q.Query
		if len(pattern) > 50 {
			pattern = pattern[:50] + "..."
		}
		queryCount[pattern]++
	}

	stats.AvgDuration = totalDuration / time.Duration(len(l.queries))

	// Find most frequent slow queries
	for pattern, count := range queryCount {
		stats.TopQueries = append(stats.TopQueries, QueryPattern{
			Pattern: pattern,
			Count:   count,
		})
	}

	// Sort top queries by count
	for i := 0; i < len(stats.TopQueries)-1; i++ {
		for j := i + 1; j < len(stats.TopQueries); j++ {
			if stats.TopQueries[i].Count < stats.TopQueries[j].Count {
				stats.TopQueries[i], stats.TopQueries[j] = stats.TopQueries[j], stats.TopQueries[i]
			}
		}
	}

	// Keep only top 10
	if len(stats.TopQueries) > 10 {
		stats.TopQueries = stats.TopQueries[:10]
	}

	return stats
}

// Clear clears all logged queries
func (l *SlowQueryLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.queries = l.queries[:0]
}

// SlowQueryStats contains statistics about slow queries
type SlowQueryStats struct {
	TotalCount  int            `json:"totalCount"`
	AvgDuration time.Duration  `json:"avgDuration"`
	MaxDuration time.Duration  `json:"maxDuration"`
	MinDuration time.Duration  `json:"minDuration"`
	TopQueries  []QueryPattern `json:"topQueries"`
}

// QueryPattern represents a query pattern and its frequency
type QueryPattern struct {
	Pattern string `json:"pattern"`
	Count   int    `json:"count"`
}

// QueryAnalyzer analyzes query patterns and provides optimization suggestions
type QueryAnalyzer struct {
	logger *zap.Logger
}

// NewQueryAnalyzer creates a new query analyzer
func NewQueryAnalyzer(logger *zap.Logger) *QueryAnalyzer {
	return &QueryAnalyzer{
		logger: logger,
	}
}

// Analyze analyzes a query and provides suggestions
func (a *QueryAnalyzer) Analyze(query string, duration time.Duration) *QueryAnalysis {
	analysis := &QueryAnalysis{
		Query:       query,
		Duration:    duration,
		IsSlow:      duration > SlowQueryThreshold,
		Suggestions: []string{},
	}

	// Check for common issues
	a.checkMissingIndex(query, analysis)
	a.checkFullTableScan(query, analysis)
	a.checkNPlusOne(query, analysis)
	a.checkMissingLimit(query, analysis)

	return analysis
}

func (a *QueryAnalyzer) checkMissingIndex(query string, analysis *QueryAnalysis) {
	// Simple heuristic - look for WHERE without index hint
	if contains(query, "WHERE") && !contains(query, "INDEX") {
		analysis.Suggestions = append(analysis.Suggestions,
			"Consider adding an index on the WHERE clause columns")
	}
}

func (a *QueryAnalyzer) checkFullTableScan(query string, analysis *QueryAnalysis) {
	// Look for SELECT * without WHERE
	if contains(query, "SELECT *") && !contains(query, "WHERE") && !contains(query, "LIMIT") {
		analysis.Issues = append(analysis.Issues, "Potential full table scan detected")
		analysis.Suggestions = append(analysis.Suggestions,
			"Add WHERE clause or LIMIT to avoid full table scan")
	}
}

func (a *QueryAnalyzer) checkNPlusOne(query string, analysis *QueryAnalysis) {
	// Simple detection - multiple similar queries
	// This would need more context in real implementation
	if contains(query, "SELECT") && contains(query, "WHERE") && contains(query, "id = ") {
		analysis.Suggestions = append(analysis.Suggestions,
			"Consider using batch loading or JOIN to avoid N+1 queries")
	}
}

func (a *QueryAnalyzer) checkMissingLimit(query string, analysis *QueryAnalysis) {
	// Check for SELECT without LIMIT
	if contains(query, "SELECT") && !contains(query, "LIMIT") && !contains(query, "COUNT(") {
		analysis.Suggestions = append(analysis.Suggestions,
			"Consider adding LIMIT clause to prevent loading too many records")
	}
}

// QueryAnalysis contains query analysis results
type QueryAnalysis struct {
	Query       string        `json:"query"`
	Duration    time.Duration `json:"duration"`
	IsSlow      bool          `json:"isSlow"`
	Issues      []string      `json:"issues,omitempty"`
	Suggestions []string      `json:"suggestions,omitempty"`
}

// Helper function for string contains (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			fmt.Sprintf("%s", s) != "" &&
				fmt.Sprintf("%s", substr) != "")
}
