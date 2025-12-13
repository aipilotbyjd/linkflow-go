package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrWebhookNotFound   = errors.New("webhook not found")
	ErrWebhookDisabled   = errors.New("webhook is disabled")
	ErrInvalidSignature  = errors.New("invalid webhook signature")
	ErrWebhookExpired    = errors.New("webhook has expired")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

// Webhook represents a registered webhook endpoint
type Webhook struct {
	ID           string            `json:"id" gorm:"primaryKey"`
	WorkflowID   string            `json:"workflowId" gorm:"not null;index"`
	NodeID       string            `json:"nodeId" gorm:"not null"`
	UserID       string            `json:"userId" gorm:"not null;index"`
	Name         string            `json:"name"`
	Path         string            `json:"path" gorm:"uniqueIndex;not null"`
	Method       string            `json:"method" gorm:"default:'POST'"`
	Secret       string            `json:"secret"` // For HMAC signature verification
	IsActive     bool              `json:"isActive" gorm:"default:true"`
	RequireAuth  bool              `json:"requireAuth" gorm:"default:false"`
	AuthType     string            `json:"authType"` // none, header, basic, bearer
	AuthConfig   map[string]string `json:"authConfig" gorm:"serializer:json"`
	Headers      map[string]string `json:"headers" gorm:"serializer:json"` // Required headers
	RateLimit    int               `json:"rateLimit" gorm:"default:100"`   // requests per minute
	ExpiresAt    *time.Time        `json:"expiresAt"`
	LastCalledAt *time.Time        `json:"lastCalledAt"`
	CallCount    int64             `json:"callCount" gorm:"default:0"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
}

// TableName specifies the table name for GORM
func (Webhook) TableName() string {
	return "workflow.webhooks"
}

// WebhookExecution records each webhook call
type WebhookExecution struct {
	ID           string                 `json:"id" gorm:"primaryKey"`
	WebhookID    string                 `json:"webhookId" gorm:"not null;index"`
	WorkflowID   string                 `json:"workflowId" gorm:"not null;index"`
	ExecutionID  string                 `json:"executionId"`
	Method       string                 `json:"method"`
	Path         string                 `json:"path"`
	Headers      map[string]string      `json:"headers" gorm:"serializer:json"`
	QueryParams  map[string]string      `json:"queryParams" gorm:"serializer:json"`
	Body         string                 `json:"body"`
	ContentType  string                 `json:"contentType"`
	IPAddress    string                 `json:"ipAddress"`
	UserAgent    string                 `json:"userAgent"`
	Status       string                 `json:"status"` // received, processed, failed
	ResponseCode int                    `json:"responseCode"`
	ResponseBody string                 `json:"responseBody"`
	Error        string                 `json:"error"`
	ProcessedAt  *time.Time             `json:"processedAt"`
	Duration     int64                  `json:"duration"` // in ms
	Metadata     map[string]interface{} `json:"metadata" gorm:"serializer:json"`
	CreatedAt    time.Time              `json:"createdAt"`
}

// TableName specifies the table name for GORM
func (WebhookExecution) TableName() string {
	return "workflow.webhook_logs"
}

// NewWebhook creates a new webhook
func NewWebhook(workflowID, nodeID, userID, path string) *Webhook {
	return &Webhook{
		ID:         uuid.New().String(),
		WorkflowID: workflowID,
		NodeID:     nodeID,
		UserID:     userID,
		Path:       path,
		Method:     "POST",
		Secret:     generateSecret(),
		IsActive:   true,
		RateLimit:  100,
		Headers:    make(map[string]string),
		AuthConfig: make(map[string]string),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// generateSecret generates a random secret for HMAC
func generateSecret() string {
	return uuid.New().String()
}

// Validate validates the webhook
func (w *Webhook) Validate() error {
	if w.WorkflowID == "" {
		return errors.New("workflow ID is required")
	}
	if w.Path == "" {
		return errors.New("webhook path is required")
	}
	if w.Method == "" {
		w.Method = "POST"
	}

	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true,
	}
	if !validMethods[w.Method] {
		return errors.New("invalid HTTP method")
	}

	return nil
}

// IsExpired checks if the webhook has expired
func (w *Webhook) IsExpired() bool {
	if w.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*w.ExpiresAt)
}

// CanAccept checks if the webhook can accept requests
func (w *Webhook) CanAccept() error {
	if !w.IsActive {
		return ErrWebhookDisabled
	}
	if w.IsExpired() {
		return ErrWebhookExpired
	}
	return nil
}

// VerifySignature verifies HMAC signature
func (w *Webhook) VerifySignature(payload []byte, signature string) bool {
	if w.Secret == "" {
		return true // No signature required
	}

	mac := hmac.New(sha256.New, []byte(w.Secret))
	mac.Write(payload)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// RecordCall records a webhook call
func (w *Webhook) RecordCall() {
	now := time.Now()
	w.LastCalledAt = &now
	w.CallCount++
	w.UpdatedAt = now
}

// GetURL returns the full webhook URL
func (w *Webhook) GetURL(baseURL string) string {
	return baseURL + "/webhooks/" + w.Path
}

// WebhookResponse represents the response to return to webhook caller
type WebhookResponse struct {
	Success     bool        `json:"success"`
	ExecutionID string      `json:"executionId,omitempty"`
	Message     string      `json:"message,omitempty"`
	Data        interface{} `json:"data,omitempty"`
}
