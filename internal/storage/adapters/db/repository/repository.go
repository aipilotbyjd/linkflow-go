package repository

import (
	"context"

	"github.com/linkflow-go/pkg/database"
)

type StorageRepository struct {
	db *database.DB
}

func NewStorageRepository(db *database.DB) *StorageRepository {
	return &StorageRepository{db: db}
}

func (r *StorageRepository) CreateFileRecord(ctx context.Context, file interface{}) error {
	return r.db.WithContext(ctx).Create(&file).Error
}

func (r *StorageRepository) GetFileRecord(ctx context.Context, id string) (interface{}, error) {
	var file interface{}
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&file).Error
	return file, err
}

func (r *StorageRepository) ListFiles(ctx context.Context, userID string) ([]interface{}, error) {
	var files []interface{}
	return files, nil
}
