package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/linkflow-go/pkg/contracts/user"
	"github.com/linkflow-go/pkg/database"
	"gorm.io/gorm"
)

type AuthRepository struct {
	db *database.DB
}

func NewAuthRepository(db *database.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

func (r *AuthRepository) CreateUser(ctx context.Context, u *user.User) error {
	return r.db.WithContext(ctx).Create(u).Error
}

func (r *AuthRepository) GetUserByEmail(ctx context.Context, email string) (*user.User, error) {
	var u user.User
	err := r.db.WithContext(ctx).
		Preload("Roles.Permissions").
		Where("email = ?", email).
		First(&u).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("user not found")
	}

	return &u, err
}

func (r *AuthRepository) GetUserByID(ctx context.Context, id string) (*user.User, error) {
	var u user.User
	err := r.db.WithContext(ctx).
		Preload("Roles.Permissions").
		Where("id = ?", id).
		First(&u).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("user not found")
	}

	return &u, err
}

func (r *AuthRepository) UpdateUser(ctx context.Context, u *user.User) error {
	return r.db.WithContext(ctx).Save(u).Error
}

func (r *AuthRepository) DeleteUser(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&user.User{}).Error
}

func (r *AuthRepository) CreateSession(ctx context.Context, session *user.Session) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *AuthRepository) GetSession(ctx context.Context, token string) (*user.Session, error) {
	var session user.Session
	err := r.db.WithContext(ctx).
		Where("token = ?", token).
		First(&session).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("session not found")
	}

	return &session, err
}

func (r *AuthRepository) DeleteSession(ctx context.Context, token string) error {
	return r.db.WithContext(ctx).
		Where("token = ?", token).
		Delete(&user.Session{}).Error
}

func (r *AuthRepository) DeleteUserSessions(ctx context.Context, userID string) error {
	return r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&user.Session{}).Error
}

func (r *AuthRepository) GetUserSessions(ctx context.Context, userID string) ([]*user.Session, error) {
	var sessions []*user.Session
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND expires_at > ?", userID, time.Now()).
		Order("created_at DESC").
		Find(&sessions).Error

	return sessions, err
}

func (r *AuthRepository) GetSessionByID(ctx context.Context, sessionID string) (*user.Session, error) {
	var session user.Session
	err := r.db.WithContext(ctx).
		Where("id = ?", sessionID).
		First(&session).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("session not found")
	}

	return &session, err
}

func (r *AuthRepository) DeleteSessionByID(ctx context.Context, sessionID string) error {
	return r.db.WithContext(ctx).
		Where("id = ?", sessionID).
		Delete(&user.Session{}).Error
}

func (r *AuthRepository) GetUserByEmailVerifyToken(ctx context.Context, token string) (*user.User, error) {
	var u user.User
	err := r.db.WithContext(ctx).
		Where("email_verify_token = ?", token).
		First(&u).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("user not found")
	}

	return &u, err
}

func (r *AuthRepository) CreateRole(ctx context.Context, role *user.Role) error {
	return r.db.WithContext(ctx).Create(role).Error
}

func (r *AuthRepository) GetRole(ctx context.Context, id string) (*user.Role, error) {
	var role user.Role
	err := r.db.WithContext(ctx).
		Preload("Permissions").
		Where("id = ?", id).
		First(&role).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("role not found")
	}

	return &role, err
}

func (r *AuthRepository) GetRoleByName(ctx context.Context, name string) (*user.Role, error) {
	var role user.Role
	err := r.db.WithContext(ctx).
		Preload("Permissions").
		Where("name = ?", name).
		First(&role).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("role not found")
	}

	return &role, err
}

func (r *AuthRepository) GetAllRoles(ctx context.Context) ([]*user.Role, error) {
	var roles []*user.Role
	err := r.db.WithContext(ctx).
		Preload("Permissions").
		Find(&roles).Error

	return roles, err
}

func (r *AuthRepository) AssignRoleToUser(ctx context.Context, userID, roleID string) error {
	u, err := r.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	role, err := r.GetRole(ctx, roleID)
	if err != nil {
		return err
	}

	// Check if role already assigned
	for _, r := range u.Roles {
		if r.ID == roleID {
			return nil // Already assigned
		}
	}

	// Assign role
	u.Roles = append(u.Roles, *role)
	return r.db.WithContext(ctx).Save(u).Error
}

func (r *AuthRepository) RemoveRoleFromUser(ctx context.Context, userID, roleID string) error {
	return r.db.WithContext(ctx).
		Exec("DELETE FROM user_roles WHERE user_id = ? AND role_id = ?", userID, roleID).
		Error
}

func (r *AuthRepository) CreateOAuthToken(ctx context.Context, token *user.OAuthToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *AuthRepository) GetOAuthToken(ctx context.Context, userID, provider string) (*user.OAuthToken, error) {
	var token user.OAuthToken
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND provider = ?", userID, provider).
		First(&token).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("OAuth token not found")
	}

	return &token, err
}

func (r *AuthRepository) UpdateOAuthToken(ctx context.Context, token *user.OAuthToken) error {
	return r.db.WithContext(ctx).Save(token).Error
}
