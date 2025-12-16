package repository

import (
	"context"

	"github.com/linkflow-go/pkg/contracts/credential"
	"github.com/linkflow-go/pkg/database"
)

type CredentialRepository struct {
	db *database.DB
}

func NewCredentialRepository(db *database.DB) *CredentialRepository {
	return &CredentialRepository{db: db}
}

func (r *CredentialRepository) CreateCredential(ctx context.Context, cred *credential.Credential) error {
	return r.db.WithContext(ctx).Create(cred).Error
}

func (r *CredentialRepository) GetCredential(ctx context.Context, id string) (*credential.Credential, error) {
	var cred credential.Credential
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&cred).Error
	return &cred, err
}

func (r *CredentialRepository) ListCredentials(ctx context.Context, userID string) ([]*credential.Credential, error) {
	var creds []*credential.Credential
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&creds).Error
	return creds, err
}

func (r *CredentialRepository) UpdateCredential(ctx context.Context, cred *credential.Credential) error {
	return r.db.WithContext(ctx).Save(cred).Error
}

func (r *CredentialRepository) DeleteCredential(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&credential.Credential{}).Error
}
