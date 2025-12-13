package analytics

import (
	"time"

	"github.com/google/uuid"
)

// WorkflowStats represents daily workflow statistics
type WorkflowStats struct {
	ID                   string    `json:"id" gorm:"primaryKey"`
	WorkflowID           string    `json:"workflowId" gorm:"not null;index"`
	Date                 time.Time `json:"date" gorm:"not null;index"`
	TotalExecutions      int       `json:"totalExecutions" gorm:"default:0"`
	SuccessfulExecutions int       `json:"successfulExecutions" gorm:"default:0"`
	FailedExecutions     int       `json:"failedExecutions" gorm:"default:0"`
	AvgExecutionTime     int64     `json:"avgExecutionTime" gorm:"default:0"`
	MinExecutionTime     int64     `json:"minExecutionTime"`
	MaxExecutionTime     int64     `json:"maxExecutionTime"`
	TotalNodesExecuted   int       `json:"totalNodesExecuted" gorm:"default:0"`
	CreatedAt            time.Time `json:"createdAt"`
}

// UserActivity represents daily user activity
type UserActivity struct {
	ID                string    `json:"id" gorm:"primaryKey"`
	UserID            string    `json:"userId" gorm:"not null;index"`
	Date              time.Time `json:"date" gorm:"not null;index"`
	WorkflowsCreated  int       `json:"workflowsCreated" gorm:"default:0"`
	WorkflowsExecuted int       `json:"workflowsExecuted" gorm:"default:0"`
	APICalls          int       `json:"apiCalls" gorm:"default:0"`
	LoginCount        int       `json:"loginCount" gorm:"default:0"`
	SessionDuration   int64     `json:"sessionDuration" gorm:"default:0"`
	CreatedAt         time.Time `json:"createdAt"`
}

// SystemMetrics represents system-level metrics
type SystemMetrics struct {
	ID          string            `json:"id" gorm:"primaryKey"`
	Timestamp   time.Time         `json:"timestamp" gorm:"not null;index"`
	MetricName  string            `json:"metricName" gorm:"not null;index"`
	MetricValue float64           `json:"metricValue" gorm:"not null"`
	Labels      map[string]string `json:"labels" gorm:"serializer:json"`
	CreatedAt   time.Time         `json:"createdAt"`
}

// NodeUsage represents node type usage statistics
type NodeUsage struct {
	ID               string    `json:"id" gorm:"primaryKey"`
	NodeType         string    `json:"nodeType" gorm:"not null;index"`
	Date             time.Time `json:"date" gorm:"not null;index"`
	UsageCount       int       `json:"usageCount" gorm:"default:0"`
	SuccessCount     int       `json:"successCount" gorm:"default:0"`
	FailureCount     int       `json:"failureCount" gorm:"default:0"`
	AvgExecutionTime int64     `json:"avgExecutionTime" gorm:"default:0"`
	CreatedAt        time.Time `json:"createdAt"`
}

// Dashboard represents analytics dashboard data
type Dashboard struct {
	TotalWorkflows   int                `json:"totalWorkflows"`
	ActiveWorkflows  int                `json:"activeWorkflows"`
	TotalExecutions  int                `json:"totalExecutions"`
	SuccessRate      float64            `json:"successRate"`
	AvgExecutionTime float64            `json:"avgExecutionTime"`
	ExecutionsByDay  []DailyCount       `json:"executionsByDay"`
	TopWorkflows     []WorkflowSummary  `json:"topWorkflows"`
	TopNodes         []NodeSummary      `json:"topNodes"`
	RecentExecutions []ExecutionSummary `json:"recentExecutions"`
	ErrorsByType     map[string]int     `json:"errorsByType"`
}

// DailyCount represents a count for a specific day
type DailyCount struct {
	Date    string `json:"date"`
	Count   int    `json:"count"`
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
}

// WorkflowSummary represents workflow summary for analytics
type WorkflowSummary struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	ExecutionCount int     `json:"executionCount"`
	SuccessRate    float64 `json:"successRate"`
	AvgDuration    float64 `json:"avgDuration"`
}

// NodeSummary represents node usage summary
type NodeSummary struct {
	NodeType    string  `json:"nodeType"`
	UsageCount  int     `json:"usageCount"`
	SuccessRate float64 `json:"successRate"`
}

// ExecutionSummary represents execution summary
type ExecutionSummary struct {
	ID         string    `json:"id"`
	WorkflowID string    `json:"workflowId"`
	Status     string    `json:"status"`
	Duration   int64     `json:"duration"`
	StartedAt  time.Time `json:"startedAt"`
}

// NewWorkflowStats creates new workflow stats
func NewWorkflowStats(workflowID string, date time.Time) *WorkflowStats {
	return &WorkflowStats{
		ID:         uuid.New().String(),
		WorkflowID: workflowID,
		Date:       date,
		CreatedAt:  time.Now(),
	}
}

// NewUserActivity creates new user activity record
func NewUserActivity(userID string, date time.Time) *UserActivity {
	return &UserActivity{
		ID:        uuid.New().String(),
		UserID:    userID,
		Date:      date,
		CreatedAt: time.Now(),
	}
}

// NewSystemMetrics creates new system metrics
func NewSystemMetrics(name string, value float64, labels map[string]string) *SystemMetrics {
	return &SystemMetrics{
		ID:          uuid.New().String(),
		Timestamp:   time.Now(),
		MetricName:  name,
		MetricValue: value,
		Labels:      labels,
		CreatedAt:   time.Now(),
	}
}

// AnalyticsQuery represents query parameters for analytics
type AnalyticsQuery struct {
	StartDate   time.Time
	EndDate     time.Time
	WorkflowID  string
	UserID      string
	Granularity string // hour, day, week, month
}
