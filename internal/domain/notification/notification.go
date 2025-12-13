package notification

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrChannelNotFound = errors.New("notification channel not found")
	ErrInvalidChannel  = errors.New("invalid notification channel")
)

// Channel represents a notification channel
type Channel struct {
	ID         string            `json:"id" gorm:"primaryKey"`
	UserID     string            `json:"userId" gorm:"not null;index"`
	Type       string            `json:"type" gorm:"not null"`
	Name       string            `json:"name" gorm:"not null"`
	Config     map[string]string `json:"config" gorm:"serializer:json"`
	IsActive   bool              `json:"isActive" gorm:"default:true"`
	IsVerified bool              `json:"isVerified" gorm:"default:false"`
	VerifiedAt *time.Time        `json:"verifiedAt"`
	CreatedAt  time.Time         `json:"createdAt"`
	UpdatedAt  time.Time         `json:"updatedAt"`
}

// Preferences represents user notification preferences
type Preferences struct {
	ID               string    `json:"id" gorm:"primaryKey"`
	UserID           string    `json:"userId" gorm:"uniqueIndex;not null"`
	EmailEnabled     bool      `json:"emailEnabled" gorm:"default:true"`
	PushEnabled      bool      `json:"pushEnabled" gorm:"default:true"`
	SlackEnabled     bool      `json:"slackEnabled" gorm:"default:false"`
	WebhookEnabled   bool      `json:"webhookEnabled" gorm:"default:false"`
	ExecutionSuccess bool      `json:"executionSuccess" gorm:"default:false"`
	ExecutionFailure bool      `json:"executionFailure" gorm:"default:true"`
	WorkflowShared   bool      `json:"workflowShared" gorm:"default:true"`
	TeamInvite       bool      `json:"teamInvite" gorm:"default:true"`
	BillingAlerts    bool      `json:"billingAlerts" gorm:"default:true"`
	WeeklyDigest     bool      `json:"weeklyDigest" gorm:"default:true"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// Notification represents a notification message
type Notification struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	UserID      string                 `json:"userId" gorm:"not null;index"`
	ChannelID   string                 `json:"channelId" gorm:"index"`
	Type        string                 `json:"type" gorm:"not null"`
	Priority    string                 `json:"priority" gorm:"default:'normal'"`
	Subject     string                 `json:"subject"`
	Body        string                 `json:"body" gorm:"not null"`
	Data        map[string]interface{} `json:"data" gorm:"serializer:json"`
	Status      string                 `json:"status" gorm:"default:'pending'"`
	Attempts    int                    `json:"attempts" gorm:"default:0"`
	MaxAttempts int                    `json:"maxAttempts" gorm:"default:3"`
	ScheduledAt *time.Time             `json:"scheduledAt"`
	SentAt      *time.Time             `json:"sentAt"`
	ReadAt      *time.Time             `json:"readAt"`
	Error       string                 `json:"error"`
	CreatedAt   time.Time              `json:"createdAt"`
}

// Channel types
const (
	ChannelTypeEmail   = "email"
	ChannelTypePush    = "push"
	ChannelTypeSlack   = "slack"
	ChannelTypeWebhook = "webhook"
	ChannelTypeSMS     = "sms"
)

// Notification types
const (
	TypeExecutionSuccess = "execution_success"
	TypeExecutionFailure = "execution_failure"
	TypeWorkflowShared   = "workflow_shared"
	TypeTeamInvite       = "team_invite"
	TypeBillingAlert     = "billing_alert"
	TypeWeeklyDigest     = "weekly_digest"
	TypeSystemAlert      = "system_alert"
	TypeCustom           = "custom"
)

// Priority levels
const (
	PriorityLow    = "low"
	PriorityNormal = "normal"
	PriorityHigh   = "high"
	PriorityUrgent = "urgent"
)

// Status values
const (
	StatusPending   = "pending"
	StatusQueued    = "queued"
	StatusSent      = "sent"
	StatusDelivered = "delivered"
	StatusFailed    = "failed"
	StatusRead      = "read"
)

// NewChannel creates a new notification channel
func NewChannel(userID, channelType, name string) *Channel {
	return &Channel{
		ID:        uuid.New().String(),
		UserID:    userID,
		Type:      channelType,
		Name:      name,
		Config:    make(map[string]string),
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// NewNotification creates a new notification
func NewNotification(userID, notifType, subject, body string) *Notification {
	return &Notification{
		ID:          uuid.New().String(),
		UserID:      userID,
		Type:        notifType,
		Priority:    PriorityNormal,
		Subject:     subject,
		Body:        body,
		Data:        make(map[string]interface{}),
		Status:      StatusPending,
		MaxAttempts: 3,
		CreatedAt:   time.Now(),
	}
}

// MarkAsSent marks the notification as sent
func (n *Notification) MarkAsSent() {
	now := time.Now()
	n.Status = StatusSent
	n.SentAt = &now
}

// MarkAsRead marks the notification as read
func (n *Notification) MarkAsRead() {
	now := time.Now()
	n.Status = StatusRead
	n.ReadAt = &now
}

// MarkAsFailed marks the notification as failed
func (n *Notification) MarkAsFailed(err string) {
	n.Status = StatusFailed
	n.Error = err
	n.Attempts++
}

// CanRetry checks if the notification can be retried
func (n *Notification) CanRetry() bool {
	return n.Status == StatusFailed && n.Attempts < n.MaxAttempts
}

// NewPreferences creates default notification preferences
func NewPreferences(userID string) *Preferences {
	return &Preferences{
		ID:               uuid.New().String(),
		UserID:           userID,
		EmailEnabled:     true,
		PushEnabled:      true,
		ExecutionFailure: true,
		WorkflowShared:   true,
		TeamInvite:       true,
		BillingAlerts:    true,
		WeeklyDigest:     true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}
