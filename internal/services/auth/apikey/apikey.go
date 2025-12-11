package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// APIKey represents an API key for programmatic access
type APIKey struct {
	ID          string     `json:"id" gorm:"primaryKey;size:36"`
	UserID      string     `json:"userId" gorm:"size:36;index;not null"`
	Name        string     `json:"name" gorm:"size:255;not null"`
	KeyPrefix   string     `json:"keyPrefix" gorm:"size:12;not null"` // First 12 chars for identification
	KeyHash     string     `json:"-" gorm:"size:64;not null;uniqueIndex"`
	Permissions []string   `json:"permissions" gorm:"-"`
	PermJSON    string     `json:"-" gorm:"column:permissions;type:text"`
	LastUsedAt  *time.Time `json:"lastUsedAt"`
	ExpiresAt   *time.Time `json:"expiresAt"`
	CreatedAt   time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	RevokedAt   *time.Time `json:"revokedAt,omitempty"`
}

// TableName returns the table name for GORM
func (APIKey) TableName() string {
	return "api_keys"
}

// IsExpired checks if the API key has expired
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// IsRevoked checks if the API key has been revoked
func (k *APIKey) IsRevoked() bool {
	return k.RevokedAt != nil
}

// APIKeyService handles API key operations
type APIKeyService struct {
	repository APIKeyRepository
}

// APIKeyRepository defines the interface for API key storage
type APIKeyRepository interface {
	Create(ctx context.Context, key *APIKey) error
	GetByKeyHash(ctx context.Context, hash string) (*APIKey, error)
	GetByID(ctx context.Context, id string) (*APIKey, error)
	GetByUserID(ctx context.Context, userID string) ([]*APIKey, error)
	UpdateLastUsed(ctx context.Context, id string, t time.Time) error
	Revoke(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}

// NewAPIKeyService creates a new API key service
func NewAPIKeyService(repo APIKeyRepository) *APIKeyService {
	return &APIKeyService{repository: repo}
}

// CreateAPIKeyRequest represents a request to create an API key
type CreateAPIKeyRequest struct {
	UserID      string
	Name        string
	Permissions []string
	ExpiresIn   *time.Duration // Optional expiry duration
}

// CreateAPIKeyResponse contains the created key and raw key value
type CreateAPIKeyResponse struct {
	APIKey *APIKey `json:"apiKey"`
	RawKey string  `json:"key"` // Only returned once at creation
}

// Create generates a new API key for a user
func (s *APIKeyService) Create(ctx context.Context, req CreateAPIKeyRequest) (*CreateAPIKeyResponse, error) {
	// Generate random key bytes
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Create the raw key string (prefix_base64key format for easy identification)
	rawKey := fmt.Sprintf("lf_%s", base64.RawURLEncoding.EncodeToString(keyBytes))

	// Hash the key for storage
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	// Create key prefix for display (first 12 chars of raw key)
	keyPrefix := rawKey[:12]

	// Set expiry if specified
	var expiresAt *time.Time
	if req.ExpiresIn != nil {
		t := time.Now().Add(*req.ExpiresIn)
		expiresAt = &t
	}

	// Serialize permissions
	permJSON := strings.Join(req.Permissions, ",")

	apiKey := &APIKey{
		ID:          uuid.New().String(),
		UserID:      req.UserID,
		Name:        req.Name,
		KeyPrefix:   keyPrefix,
		KeyHash:     keyHash,
		Permissions: req.Permissions,
		PermJSON:    permJSON,
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now(),
	}

	if err := s.repository.Create(ctx, apiKey); err != nil {
		return nil, fmt.Errorf("failed to save API key: %w", err)
	}

	return &CreateAPIKeyResponse{
		APIKey: apiKey,
		RawKey: rawKey, // Only returned once
	}, nil
}

// Validate validates an API key and returns the key details if valid
func (s *APIKeyService) Validate(ctx context.Context, rawKey string) (*APIKey, error) {
	// Hash the provided key
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	// Look up by hash
	apiKey, err := s.repository.GetByKeyHash(ctx, keyHash)
	if err != nil {
		return nil, errors.New("invalid API key")
	}

	// Check if revoked
	if apiKey.IsRevoked() {
		return nil, errors.New("API key has been revoked")
	}

	// Check if expired
	if apiKey.IsExpired() {
		return nil, errors.New("API key has expired")
	}

	// Parse permissions from JSON
	if apiKey.PermJSON != "" {
		apiKey.Permissions = strings.Split(apiKey.PermJSON, ",")
	}

	// Update last used timestamp
	now := time.Now()
	go s.repository.UpdateLastUsed(ctx, apiKey.ID, now)

	return apiKey, nil
}

// List returns all API keys for a user (without the actual key values)
func (s *APIKeyService) List(ctx context.Context, userID string) ([]*APIKey, error) {
	keys, err := s.repository.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	// Parse permissions for each key
	for _, key := range keys {
		if key.PermJSON != "" {
			key.Permissions = strings.Split(key.PermJSON, ",")
		}
	}

	return keys, nil
}

// Revoke revokes an API key
func (s *APIKeyService) Revoke(ctx context.Context, userID, keyID string) error {
	// Get key to verify ownership
	key, err := s.repository.GetByID(ctx, keyID)
	if err != nil {
		return errors.New("API key not found")
	}

	if key.UserID != userID {
		return errors.New("unauthorized: API key does not belong to user")
	}

	return s.repository.Revoke(ctx, keyID)
}

// Delete permanently deletes an API key
func (s *APIKeyService) Delete(ctx context.Context, userID, keyID string) error {
	// Get key to verify ownership
	key, err := s.repository.GetByID(ctx, keyID)
	if err != nil {
		return errors.New("API key not found")
	}

	if key.UserID != userID {
		return errors.New("unauthorized: API key does not belong to user")
	}

	return s.repository.Delete(ctx, keyID)
}

// HasPermission checks if an API key has a specific permission
func (k *APIKey) HasPermission(resource, action string) bool {
	perm := fmt.Sprintf("%s:%s", resource, action)
	wildcard := fmt.Sprintf("%s:*", resource)

	for _, p := range k.Permissions {
		if p == perm || p == wildcard || p == "*" {
			return true
		}
	}
	return false
}
