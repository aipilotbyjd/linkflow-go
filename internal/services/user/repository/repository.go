package repository

import (
	"context"
	
	"github.com/linkflow-go/internal/domain/user"
	"github.com/linkflow-go/pkg/database"
)

type UserRepository struct {
	db *database.DB
}

func NewUserRepository(db *database.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetUser(ctx context.Context, id string) (*user.User, error) {
	var u user.User
	err := r.db.WithContext(ctx).Preload("Roles.Permissions").Where("id = ?", id).First(&u).Error
	return &u, err
}

func (r *UserRepository) ListUsers(ctx context.Context, page, limit int) ([]*user.User, int64, error) {
	var users []*user.User
	var total int64
	
	r.db.Model(&user.User{}).Count(&total)
	
	err := r.db.WithContext(ctx).
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&users).Error
		
	return users, total, err
}

func (r *UserRepository) UpdateUser(ctx context.Context, u *user.User) error {
	return r.db.WithContext(ctx).Save(u).Error
}

func (r *UserRepository) DeleteUser(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&user.User{}).Error
}
