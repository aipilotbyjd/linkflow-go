package apikey

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// GormAPIKeyRepository implements APIKeyRepository using GORM
type GormAPIKeyRepository struct {
	db *gorm.DB
}

// NewGormAPIKeyRepository creates a new GORM-based repository
func NewGormAPIKeyRepository(db *gorm.DB) *GormAPIKeyRepository {
	return &GormAPIKeyRepository{db: db}
}

// Create saves a new API key
func (r *GormAPIKeyRepository) Create(ctx context.Context, key *APIKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

// GetByKeyHash retrieves an API key by its hash
func (r *GormAPIKeyRepository) GetByKeyHash(ctx context.Context, hash string) (*APIKey, error) {
	var key APIKey
	err := r.db.WithContext(ctx).Where("key_hash = ? AND revoked_at IS NULL", hash).First(&key).Error
	if err != nil {
		return nil, err
	}
	return &key, nil
}

// GetByID retrieves an API key by ID
func (r *GormAPIKeyRepository) GetByID(ctx context.Context, id string) (*APIKey, error) {
	var key APIKey
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&key).Error
	if err != nil {
		return nil, err
	}
	return &key, nil
}

// GetByUserID retrieves all API keys for a user
func (r *GormAPIKeyRepository) GetByUserID(ctx context.Context, userID string) ([]*APIKey, error) {
	var keys []*APIKey
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&keys).Error
	if err != nil {
		return nil, err
	}
	return keys, nil
}

// UpdateLastUsed updates the last used timestamp
func (r *GormAPIKeyRepository) UpdateLastUsed(ctx context.Context, id string, t time.Time) error {
	return r.db.WithContext(ctx).Model(&APIKey{}).Where("id = ?", id).Update("last_used_at", t).Error
}

// Revoke marks an API key as revoked
func (r *GormAPIKeyRepository) Revoke(ctx context.Context, id string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&APIKey{}).Where("id = ?", id).Update("revoked_at", now).Error
}

// Delete permanently removes an API key
func (r *GormAPIKeyRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&APIKey{}, "id = ?", id).Error
}

// Migrate runs database migrations for API keys
func (r *GormAPIKeyRepository) Migrate() error {
	return r.db.AutoMigrate(&APIKey{})
}
