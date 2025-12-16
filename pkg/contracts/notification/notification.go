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
	UserID     string            `json:"userId" gorm:"column:user_id;not null;index"`
	Type       string            `json:"type" gorm:"not null"`
	Name       string            `json:"name" gorm:"not null"`
	Config     map[string]string `json:"config" gorm:"serializer:json"`
	IsActive   bool              `json:"isActive" gorm:"column:is_active;default:true"`
	IsVerified bool              `json:"isVerified" gorm:"column:is_verified;default:false"`
	VerifiedAt *time.Time        `json:"verifiedAt" gorm:"column:verified_at"`
	CreatedAt  time.Time         `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt  time.Time         `json:"updatedAt" gorm:"column:updated_at"`
}

// TableName specifies the table name for GORM
func (Channel) TableName() string {
	return "notification.channels"
}

// Preferences represents user notification preferences
type Preferences struct {
	ID               string    `json:"id" gorm:"primaryKey"`
	UserID           string    `json:"userId" gorm:"column:user_id;uniqueIndex;not null"`
	EmailEnabled     bool      `json:"emailEnabled" gorm:"column:email_enabled;default:true"`
	PushEnabled      bool      `json:"pushEnabled" gorm:"column:push_enabled;default:true"`
	SlackEnabled     bool      `json:"slackEnabled" gorm:"column:slack_enabled;default:false"`
	WebhookEnabled   bool      `json:"webhookEnabled" gorm:"column:webhook_enabled;default:false"`
	ExecutionSuccess bool      `json:"executionSuccess" gorm:"column:execution_success;default:false"`
	ExecutionFailure bool      `json:"executionFailure" gorm:"column:execution_failure;default:true"`
	WorkflowShared   bool      `json:"workflowShared" gorm:"column:workflow_shared;default:true"`
	TeamInvite       bool      `json:"teamInvite" gorm:"column:team_invite;default:true"`
	BillingAlerts    bool      `json:"billingAlerts" gorm:"column:billing_alerts;default:true"`
	WeeklyDigest     bool      `json:"weeklyDigest" gorm:"column:weekly_digest;default:true"`
	CreatedAt        time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt        time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

// TableName specifies the table name for GORM
func (Preferences) TableName() string {
	return "notification.preferences"
}

// Notification represents a notification message
type Notification struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	UserID      string                 `json:"userId" gorm:"column:user_id;not null;index"`
	ChannelID   string                 `json:"channelId" gorm:"column:channel_id;index"`
	Type        string                 `json:"type" gorm:"not null"`
	Priority    string                 `json:"priority" gorm:"default:'normal'"`
	Subject     string                 `json:"subject"`
	Body        string                 `json:"body" gorm:"not null"`
	Data        map[string]interface{} `json:"data" gorm:"serializer:json"`
	Status      string                 `json:"status" gorm:"default:'pending'"`
	Attempts    int                    `json:"attempts" gorm:"column:retry_count;default:0"`
	MaxAttempts int                    `json:"maxAttempts" gorm:"column:max_retries;default:3"`
	ScheduledAt *time.Time             `json:"scheduledAt" gorm:"column:scheduled_at"`
	SentAt      *time.Time             `json:"sentAt" gorm:"column:sent_at"`
	ReadAt      *time.Time             `json:"readAt" gorm:"column:read_at"`
	Error       string                 `json:"error" gorm:"column:error_message"`
	CreatedAt   time.Time              `json:"createdAt" gorm:"column:created_at"`
}

// TableName specifies the table name for GORM
func (Notification) TableName() string {
	return "notification.notifications"
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
