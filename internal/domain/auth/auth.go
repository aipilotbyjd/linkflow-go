package auth

import (
	"time"

	"github.com/google/uuid"
)

// Token represents a JWT token pair
type Token struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
	TokenType    string    `json:"tokenType"`
}

// Claims represents JWT claims
type Claims struct {
	UserID      string   `json:"userId"`
	Email       string   `json:"email"`
	Username    string   `json:"username"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	SessionID   string   `json:"sessionId"`
	IssuedAt    int64    `json:"iat"`
	ExpiresAt   int64    `json:"exp"`
}

// APIKey represents an API key for programmatic access
type APIKey struct {
	ID         string     `json:"id" gorm:"primaryKey"`
	UserID     string     `json:"userId" gorm:"not null;index"`
	Name       string     `json:"name" gorm:"not null"`
	KeyHash    string     `json:"-" gorm:"uniqueIndex;not null"`
	KeyPrefix  string     `json:"keyPrefix"` // First 8 chars for identification
	Scopes     []string   `json:"scopes" gorm:"serializer:json"`
	RateLimit  int        `json:"rateLimit" gorm:"default:1000"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	LastUsedIP string     `json:"lastUsedIp"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	IsActive   bool       `json:"isActive" gorm:"default:true"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

// TableName specifies the table name for GORM
func (APIKey) TableName() string {
	return "auth.api_keys"
}

// LoginAttempt tracks login attempts for security
type LoginAttempt struct {
	ID         string    `json:"id" gorm:"primaryKey"`
	Email      string    `json:"email" gorm:"index"`
	IPAddress  string    `json:"ipAddress" gorm:"index"`
	UserAgent  string    `json:"userAgent"`
	Success    bool      `json:"success"`
	FailReason string    `json:"failReason"`
	CreatedAt  time.Time `json:"createdAt"`
}

// PasswordReset represents a password reset request
type PasswordReset struct {
	ID        string     `json:"id" gorm:"primaryKey"`
	UserID    string     `json:"userId" gorm:"not null;index"`
	Token     string     `json:"token" gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time  `json:"expiresAt"`
	UsedAt    *time.Time `json:"usedAt"`
	CreatedAt time.Time  `json:"createdAt"`
}

// TwoFactorSetup represents 2FA setup data
type TwoFactorSetup struct {
	Secret      string   `json:"secret"`
	QRCode      string   `json:"qrCode"`
	BackupCodes []string `json:"backupCodes"`
}

// AuthProvider represents OAuth provider configuration
type AuthProvider struct {
	Name         string   `json:"name"`
	ClientID     string   `json:"clientId"`
	ClientSecret string   `json:"-"`
	AuthURL      string   `json:"authUrl"`
	TokenURL     string   `json:"tokenUrl"`
	Scopes       []string `json:"scopes"`
	RedirectURL  string   `json:"redirectUrl"`
}

// Scope constants for API keys
const (
	ScopeWorkflowRead    = "workflow:read"
	ScopeWorkflowWrite   = "workflow:write"
	ScopeWorkflowExecute = "workflow:execute"
	ScopeExecutionRead   = "execution:read"
	ScopeExecutionWrite  = "execution:write"
	ScopeCredentialRead  = "credential:read"
	ScopeCredentialWrite = "credential:write"
	ScopeUserRead        = "user:read"
	ScopeUserWrite       = "user:write"
	ScopeAdmin           = "admin"
)

// NewAPIKey creates a new API key
func NewAPIKey(userID, name string, scopes []string) *APIKey {
	return &APIKey{
		ID:        uuid.New().String(),
		UserID:    userID,
		Name:      name,
		Scopes:    scopes,
		RateLimit: 1000,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// HasScope checks if the API key has a specific scope
func (k *APIKey) HasScope(scope string) bool {
	for _, s := range k.Scopes {
		if s == scope || s == ScopeAdmin {
			return true
		}
	}
	return false
}

// IsExpired checks if the API key has expired
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// IsValid checks if the API key is valid for use
func (k *APIKey) IsValid() bool {
	return k.IsActive && !k.IsExpired()
}

// RecordUsage records API key usage
func (k *APIKey) RecordUsage(ip string) {
	now := time.Now()
	k.LastUsedAt = &now
	k.LastUsedIP = ip
}

// NewPasswordReset creates a new password reset request
func NewPasswordReset(userID string) *PasswordReset {
	return &PasswordReset{
		ID:        uuid.New().String(),
		UserID:    userID,
		Token:     uuid.New().String(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
		CreatedAt: time.Now(),
	}
}

// IsExpired checks if the password reset has expired
func (p *PasswordReset) IsExpired() bool {
	return time.Now().After(p.ExpiresAt)
}

// IsUsed checks if the password reset has been used
func (p *PasswordReset) IsUsed() bool {
	return p.UsedAt != nil
}
