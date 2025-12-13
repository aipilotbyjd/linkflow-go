package repository

import (
	"context"

	"github.com/linkflow-go/internal/domain/variable"
	"github.com/linkflow-go/pkg/database"
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, v *variable.Variable) error {
	return r.db.WithContext(ctx).Create(v).Error
}

func (r *Repository) GetByID(ctx context.Context, id string) (*variable.Variable, error) {
	var v variable.Variable
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&v).Error
	if err != nil {
		return nil, variable.ErrVariableNotFound
	}
	return &v, nil
}

func (r *Repository) GetByKey(ctx context.Context, key string) (*variable.Variable, error) {
	var v variable.Variable
	err := r.db.WithContext(ctx).Where("key = ?", key).First(&v).Error
	if err != nil {
		return nil, variable.ErrVariableNotFound
	}
	return &v, nil
}

func (r *Repository) List(ctx context.Context) ([]*variable.Variable, error) {
	var variables []*variable.Variable
	err := r.db.WithContext(ctx).Order("key ASC").Find(&variables).Error
	return variables, err
}

func (r *Repository) Update(ctx context.Context, v *variable.Variable) error {
	return r.db.WithContext(ctx).Save(v).Error
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&variable.Variable{}).Error
}

func (r *Repository) Exists(ctx context.Context, key string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&variable.Variable{}).Where("key = ?", key).Count(&count).Error
	return count > 0, err
}

func (r *Repository) GetAllAsMap(ctx context.Context) (map[string]string, error) {
	variables, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, v := range variables {
		result[v.Key] = v.Value
	}
	return result, nil
}
