package audit

import (
	"time"

	"github.com/google/uuid"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	ID           string                 `json:"id" gorm:"primaryKey"`
	UserID       string                 `json:"userId" gorm:"index"`
	Action       string                 `json:"action" gorm:"not null;index"`
	ResourceType string                 `json:"resourceType" gorm:"not null;index"`
	ResourceID   string                 `json:"resourceId" gorm:"index"`
	Changes      map[string]interface{} `json:"changes" gorm:"serializer:json"`
	OldValues    map[string]interface{} `json:"oldValues" gorm:"serializer:json"`
	NewValues    map[string]interface{} `json:"newValues" gorm:"serializer:json"`
	IPAddress    string                 `json:"ipAddress"`
	UserAgent    string                 `json:"userAgent"`
	SessionID    string                 `json:"sessionId"`
	Metadata     map[string]interface{} `json:"metadata" gorm:"serializer:json"`
	CreatedAt    time.Time              `json:"createdAt" gorm:"index"`
}

// Action types
const (
	ActionCreate   = "create"
	ActionRead     = "read"
	ActionUpdate   = "update"
	ActionDelete   = "delete"
	ActionLogin    = "login"
	ActionLogout   = "logout"
	ActionExecute  = "execute"
	ActionActivate = "activate"
	ActionShare    = "share"
	ActionExport   = "export"
	ActionImport   = "import"
)

// Resource types
const (
	ResourceUser       = "user"
	ResourceWorkflow   = "workflow"
	ResourceExecution  = "execution"
	ResourceCredential = "credential"
	ResourceWebhook    = "webhook"
	ResourceSchedule   = "schedule"
	ResourceTeam       = "team"
	ResourceAPIKey     = "api_key"
	ResourceVariable   = "variable"
	ResourceTemplate   = "template"
)

// NewAuditLog creates a new audit log entry
func NewAuditLog(userID, action, resourceType, resourceID string) *AuditLog {
	return &AuditLog{
		ID:           uuid.New().String(),
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Changes:      make(map[string]interface{}),
		Metadata:     make(map[string]interface{}),
		CreatedAt:    time.Now(),
	}
}

// WithChanges adds change details to the audit log
func (a *AuditLog) WithChanges(old, new map[string]interface{}) *AuditLog {
	a.OldValues = old
	a.NewValues = new

	// Calculate diff
	changes := make(map[string]interface{})
	for key, newVal := range new {
		if oldVal, exists := old[key]; exists {
			if oldVal != newVal {
				changes[key] = map[string]interface{}{
					"old": oldVal,
					"new": newVal,
				}
			}
		} else {
			changes[key] = map[string]interface{}{
				"new": newVal,
			}
		}
	}
	a.Changes = changes

	return a
}

// WithRequest adds request context to the audit log
func (a *AuditLog) WithRequest(ipAddress, userAgent, sessionID string) *AuditLog {
	a.IPAddress = ipAddress
	a.UserAgent = userAgent
	a.SessionID = sessionID
	return a
}

// WithMetadata adds additional metadata
func (a *AuditLog) WithMetadata(key string, value interface{}) *AuditLog {
	if a.Metadata == nil {
		a.Metadata = make(map[string]interface{})
	}
	a.Metadata[key] = value
	return a
}

// AuditQuery represents query parameters for audit logs
type AuditQuery struct {
	UserID       string
	Action       string
	ResourceType string
	ResourceID   string
	StartDate    *time.Time
	EndDate      *time.Time
	Page         int
	Limit        int
}

// AuditSummary represents aggregated audit data
type AuditSummary struct {
	TotalActions     int            `json:"totalActions"`
	ActionCounts     map[string]int `json:"actionCounts"`
	ResourceCounts   map[string]int `json:"resourceCounts"`
	TopUsers         []UserActivity `json:"topUsers"`
	RecentActivities []AuditLog     `json:"recentActivities"`
}

// UserActivity represents user activity summary
type UserActivity struct {
	UserID      string    `json:"userId"`
	ActionCount int       `json:"actionCount"`
	LastActive  time.Time `json:"lastActive"`
}
