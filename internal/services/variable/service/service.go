package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/linkflow-go/internal/domain/variable"
	"github.com/linkflow-go/internal/services/variable/repository"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type Service struct {
	repo          *repository.Repository
	eventBus      events.EventBus
	redis         *redis.Client
	logger        logger.Logger
	encryptionKey []byte
}

func NewService(repo *repository.Repository, eventBus events.EventBus, redis *redis.Client, logger logger.Logger, encryptionKey string) *Service {
	key := []byte(encryptionKey)
	if len(key) != 32 {
		newKey := make([]byte, 32)
		copy(newKey, key)
		key = newKey
	}

	return &Service{
		repo:          repo,
		eventBus:      eventBus,
		redis:         redis,
		logger:        logger,
		encryptionKey: key,
	}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*variable.Variable, error) {
	if err := variable.ValidateKey(req.Key); err != nil {
		return nil, err
	}

	exists, _ := s.repo.Exists(ctx, req.Key)
	if exists {
		return nil, variable.ErrVariableExists
	}

	v := variable.NewVariable(req.Key, req.Value, req.Type)
	v.Description = req.Description

	if err := v.Validate(); err != nil {
		return nil, err
	}

	if v.IsSecret() {
		encrypted, err := s.encrypt(v.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt secret: %w", err)
		}
		v.Value = encrypted
	}

	if err := s.repo.Create(ctx, v); err != nil {
		return nil, fmt.Errorf("failed to create variable: %w", err)
	}

	s.invalidateCache(ctx)

	event := events.NewEventBuilder("variable.created").
		WithAggregateID(v.ID).
		WithPayload("key", v.Key).
		Build()
	s.eventBus.Publish(ctx, event)

	s.logger.Info("Variable created", "key", v.Key, "type", v.Type)
	return v, nil
}

func (s *Service) Get(ctx context.Context, id string) (*variable.Variable, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) GetByKey(ctx context.Context, key string) (*variable.Variable, error) {
	return s.repo.GetByKey(ctx, key)
}

func (s *Service) List(ctx context.Context) ([]*variable.Variable, error) {
	return s.repo.List(ctx)
}

func (s *Service) Update(ctx context.Context, id string, req UpdateRequest) (*variable.Variable, error) {
	v, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Key != "" && req.Key != v.Key {
		if err := variable.ValidateKey(req.Key); err != nil {
			return nil, err
		}
		exists, _ := s.repo.Exists(ctx, req.Key)
		if exists {
			return nil, variable.ErrVariableExists
		}
		v.Key = req.Key
	}

	if req.Value != "" {
		if v.IsSecret() {
			encrypted, err := s.encrypt(req.Value)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt secret: %w", err)
			}
			v.Value = encrypted
		} else {
			v.Value = req.Value
		}
	}

	if req.Description != "" {
		v.Description = req.Description
	}

	v.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, v); err != nil {
		return nil, fmt.Errorf("failed to update variable: %w", err)
	}

	s.invalidateCache(ctx)

	event := events.NewEventBuilder("variable.updated").
		WithAggregateID(v.ID).
		WithPayload("key", v.Key).
		Build()
	s.eventBus.Publish(ctx, event)

	s.logger.Info("Variable updated", "key", v.Key)
	return v, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	v, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete variable: %w", err)
	}

	s.invalidateCache(ctx)

	event := events.NewEventBuilder("variable.deleted").
		WithAggregateID(id).
		WithPayload("key", v.Key).
		Build()
	s.eventBus.Publish(ctx, event)

	s.logger.Info("Variable deleted", "key", v.Key)
	return nil
}

func (s *Service) GetDecryptedValue(ctx context.Context, key string) (string, error) {
	v, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		return "", err
	}

	if v.IsSecret() {
		decrypted, err := s.decrypt(v.Value)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt secret: %w", err)
		}
		return decrypted, nil
	}

	return v.Value, nil
}

func (s *Service) GetAllForExecution(ctx context.Context) (map[string]string, error) {
	cached, err := s.redis.HGetAll(ctx, "variables:all").Result()
	if err == nil && len(cached) > 0 {
		return cached, nil
	}

	variables, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, v := range variables {
		if v.IsSecret() {
			decrypted, err := s.decrypt(v.Value)
			if err != nil {
				s.logger.Error("Failed to decrypt variable", "key", v.Key, "error", err)
				continue
			}
			result[v.Key] = decrypted
		} else {
			result[v.Key] = v.Value
		}
	}

	if len(result) > 0 {
		s.redis.HSet(ctx, "variables:all", result)
		s.redis.Expire(ctx, "variables:all", 5*time.Minute)
	}

	return result, nil
}

func (s *Service) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *Service) decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func (s *Service) invalidateCache(ctx context.Context) {
	s.redis.Del(ctx, "variables:all")
}

type CreateRequest struct {
	Key         string `json:"key" binding:"required"`
	Value       string `json:"value" binding:"required"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type UpdateRequest struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
}
