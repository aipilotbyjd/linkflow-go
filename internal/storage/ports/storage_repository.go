package ports

import "context"

type StorageRepository interface {
	CreateFileRecord(ctx context.Context, file interface{}) error
	GetFileRecord(ctx context.Context, id string) (interface{}, error)
	ListFiles(ctx context.Context, userID string) ([]interface{}, error)
}
