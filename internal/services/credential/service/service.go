package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/linkflow-go/internal/domain/credential"
	"github.com/linkflow-go/internal/services/credential/repository"
	"github.com/linkflow-go/internal/services/credential/vault"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type CredentialService struct {
	repo     *repository.CredentialRepository
	vault    *vault.VaultManager
	eventBus events.EventBus
	redis    *redis.Client
	logger   logger.Logger
}

func NewCredentialService(
	repo *repository.CredentialRepository,
	vault *vault.VaultManager,
	eventBus events.EventBus,
	redis *redis.Client,
	logger logger.Logger,
) *CredentialService {
	return &CredentialService{
		repo:     repo,
		vault:    vault,
		eventBus: eventBus,
		redis:    redis,
		logger:   logger,
	}
}

// CreateCredential creates a new credential with encrypted data
func (s *CredentialService) CreateCredential(ctx context.Context, req CreateCredentialRequest) (*credential.Credential, error) {
	cred := credential.NewCredential(req.Name, req.Type, req.UserID)
	cred.Description = req.Description
	cred.TeamID = req.TeamID
	cred.Data = req.Data
	cred.Tags = req.Tags
	cred.ExpiresAt = req.ExpiresAt

	// Validate credential
	if err := cred.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Encrypt sensitive data
	if err := s.vault.EncryptCredential(ctx, cred); err != nil {
		return nil, fmt.Errorf("failed to encrypt credential: %w", err)
	}

	// Save to database
	if err := s.repo.CreateCredential(ctx, cred); err != nil {
		return nil, fmt.Errorf("failed to save credential: %w", err)
	}

	// Publish event
	event := events.NewEventBuilder("credential.created").
		WithAggregateID(cred.ID).
		WithUserID(req.UserID).
		WithPayload("name", cred.Name).
		WithPayload("type", cred.Type).
		Build()
	s.eventBus.Publish(ctx, event)

	s.logger.Info("Credential created", "id", cred.ID, "type", cred.Type)
	return cred, nil
}

// GetCredential retrieves a credential by ID
func (s *CredentialService) GetCredential(ctx context.Context, id, userID string) (*credential.Credential, error) {
	cred, err := s.repo.GetCredential(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("credential not found: %w", err)
	}

	// Check ownership or sharing
	if cred.UserID != userID && !cred.IsShared {
		return nil, fmt.Errorf("access denied")
	}

	return cred, nil
}

// GetDecryptedCredential retrieves and decrypts a credential
func (s *CredentialService) GetDecryptedCredential(ctx context.Context, id, userID string) (*credential.Credential, error) {
	cred, err := s.GetCredential(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	// Decrypt sensitive data
	if err := s.vault.DecryptCredential(ctx, cred); err != nil {
		return nil, fmt.Errorf("failed to decrypt credential: %w", err)
	}

	// Record usage
	cred.RecordUsage()
	s.repo.UpdateCredential(ctx, cred)

	return cred, nil
}

// ListCredentials lists all credentials for a user
func (s *CredentialService) ListCredentials(ctx context.Context, userID string) ([]*credential.Credential, error) {
	return s.repo.ListCredentials(ctx, userID)
}

// UpdateCredential updates an existing credential
func (s *CredentialService) UpdateCredential(ctx context.Context, id string, req UpdateCredentialRequest) (*credential.Credential, error) {
	cred, err := s.repo.GetCredential(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("credential not found: %w", err)
	}

	// Check ownership
	if cred.UserID != req.UserID {
		return nil, fmt.Errorf("access denied")
	}

	// Update fields
	if req.Name != "" {
		cred.Name = req.Name
	}
	if req.Description != "" {
		cred.Description = req.Description
	}
	if req.Data != nil {
		cred.Data = req.Data
		// Re-encrypt
		if err := s.vault.EncryptCredential(ctx, cred); err != nil {
			return nil, fmt.Errorf("failed to encrypt credential: %w", err)
		}
	}
	if req.Tags != nil {
		cred.Tags = req.Tags
	}
	cred.UpdatedAt = time.Now()

	if err := s.repo.UpdateCredential(ctx, cred); err != nil {
		return nil, fmt.Errorf("failed to update credential: %w", err)
	}

	// Publish event
	event := events.NewEventBuilder("credential.updated").
		WithAggregateID(cred.ID).
		WithUserID(req.UserID).
		Build()
	s.eventBus.Publish(ctx, event)

	s.logger.Info("Credential updated", "id", cred.ID)
	return cred, nil
}

// DeleteCredential deletes a credential
func (s *CredentialService) DeleteCredential(ctx context.Context, id, userID string) error {
	cred, err := s.repo.GetCredential(ctx, id)
	if err != nil {
		return fmt.Errorf("credential not found: %w", err)
	}

	if cred.UserID != userID {
		return fmt.Errorf("access denied")
	}

	if err := s.repo.DeleteCredential(ctx, id); err != nil {
		return fmt.Errorf("failed to delete credential: %w", err)
	}

	// Clear from cache
	s.redis.Del(ctx, fmt.Sprintf("credential:%s", id))

	// Publish event
	event := events.NewEventBuilder("credential.deleted").
		WithAggregateID(id).
		WithUserID(userID).
		Build()
	s.eventBus.Publish(ctx, event)

	s.logger.Info("Credential deleted", "id", id)
	return nil
}

// TestCredential tests if a credential is valid
func (s *CredentialService) TestCredential(ctx context.Context, id, userID string) (bool, error) {
	cred, err := s.GetDecryptedCredential(ctx, id, userID)
	if err != nil {
		return false, err
	}

	// Test based on credential type
	switch cred.Type {
	case credential.TypeAPIKey:
		// For API keys, just verify it's not empty
		if apiKey, ok := cred.Data["apiKey"].(string); ok && apiKey != "" {
			return true, nil
		}
		return false, fmt.Errorf("invalid API key")

	case credential.TypeDatabase:
		// Would actually test database connection here
		return true, nil

	case credential.TypeOAuth2:
		// Check if token is expired
		if cred.ExpiresAt != nil && time.Now().After(*cred.ExpiresAt) {
			return false, fmt.Errorf("token expired")
		}
		return true, nil

	default:
		return true, nil
	}
}

// ShareCredential shares a credential with another user
func (s *CredentialService) ShareCredential(ctx context.Context, id, ownerID, targetUserID string) error {
	cred, err := s.repo.GetCredential(ctx, id)
	if err != nil {
		return fmt.Errorf("credential not found: %w", err)
	}

	if cred.UserID != ownerID {
		return fmt.Errorf("access denied")
	}

	cred.IsShared = true
	cred.UpdatedAt = time.Now()

	if err := s.repo.UpdateCredential(ctx, cred); err != nil {
		return fmt.Errorf("failed to share credential: %w", err)
	}

	// Publish event
	event := events.NewEventBuilder("credential.shared").
		WithAggregateID(id).
		WithUserID(ownerID).
		WithPayload("sharedWith", targetUserID).
		Build()
	s.eventBus.Publish(ctx, event)

	return nil
}

// GetCredentialTypes returns all supported credential types
func (s *CredentialService) GetCredentialTypes() []credential.CredentialType {
	return credential.GetCredentialTypes()
}

// CacheCredential caches a credential for quick access
func (s *CredentialService) CacheCredential(ctx context.Context, cred *credential.Credential) error {
	data, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	return s.redis.Set(ctx, fmt.Sprintf("credential:%s", cred.ID), data, 15*time.Minute).Err()
}

// GetCachedCredential retrieves a credential from cache
func (s *CredentialService) GetCachedCredential(ctx context.Context, id string) (*credential.Credential, error) {
	data, err := s.redis.Get(ctx, fmt.Sprintf("credential:%s", id)).Result()
	if err != nil {
		return nil, err
	}

	var cred credential.Credential
	if err := json.Unmarshal([]byte(data), &cred); err != nil {
		return nil, err
	}

	return &cred, nil
}

// Event handlers
func (s *CredentialService) HandleCredentialExpiring(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling credential expiring event", "type", event.Type, "id", event.ID)
	// Would send notification to user
	return nil
}

func (s *CredentialService) HandleCredentialExpired(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling credential expired event", "type", event.Type, "id", event.ID)
	// Would deactivate credential
	return nil
}

func (s *CredentialService) HandleOAuthTokenExpired(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling OAuth token expired event", "type", event.Type, "id", event.ID)
	// Would attempt to refresh token
	return nil
}

func (s *CredentialService) HandleSecurityBreach(ctx context.Context, event events.Event) error {
	s.logger.Warn("Handling security breach event", "type", event.Type, "id", event.ID)
	// Would rotate all affected credentials
	return nil
}

// Request types
type CreateCredentialRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Type        string                 `json:"type" binding:"required"`
	UserID      string                 `json:"-"`
	TeamID      string                 `json:"teamId"`
	Description string                 `json:"description"`
	Data        map[string]interface{} `json:"data" binding:"required"`
	Tags        []string               `json:"tags"`
	ExpiresAt   *time.Time             `json:"expiresAt"`
}

type UpdateCredentialRequest struct {
	UserID      string                 `json:"-"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Data        map[string]interface{} `json:"data"`
	Tags        []string               `json:"tags"`
}
