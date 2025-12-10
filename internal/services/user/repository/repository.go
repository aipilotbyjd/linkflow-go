package repository

import (
	"context"
	"fmt"
	"strings"
	"time"
	
	"github.com/linkflow-go/internal/domain/user"
	"github.com/linkflow-go/pkg/database"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *database.DB
}

func NewUserRepository(db *database.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user
func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	return r.db.WithContext(ctx).Create(u).Error
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(ctx context.Context, id string) (*user.User, error) {
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

// GetByEmail retrieves a user by email
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
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

// GetByUsername retrieves a user by username
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*user.User, error) {
	var u user.User
	err := r.db.WithContext(ctx).
		Preload("Roles.Permissions").
		Where("username = ?", username).
		First(&u).Error
	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("user not found")
	}
	return &u, err
}

// Update updates an existing user
func (r *UserRepository) Update(ctx context.Context, u *user.User) error {
	u.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).
		Model(u).
		Updates(u).Error
}

// UpdateLastLogin updates the user's last login timestamp
func (r *UserRepository) UpdateLastLogin(ctx context.Context, userID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&user.User{}).
		Where("id = ?", userID).
		Update("last_login_at", &now).Error
}

// Delete soft deletes a user
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Model(&user.User{}).
		Where("id = ?", id).
		Update("status", user.StatusDeleted).Error
}

// HardDelete permanently deletes a user
func (r *UserRepository) HardDelete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&user.User{}).Error
}

// ListUsers lists users with pagination and optional filters
func (r *UserRepository) ListUsers(ctx context.Context, opts ListUsersOptions) ([]*user.User, int64, error) {
	var users []*user.User
	var total int64
	
	query := r.db.WithContext(ctx).Model(&user.User{})
	
	// Apply filters
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}
	
	if opts.RoleID != "" {
		query = query.Joins("JOIN user_roles ON user_roles.user_id = users.id").
			Where("user_roles.role_id = ?", opts.RoleID)
	}
	
	if opts.TeamID != "" {
		query = query.Joins("JOIN team_members ON team_members.user_id = users.id").
			Where("team_members.team_id = ?", opts.TeamID)
	}
	
	// Count total records
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	
	// Apply sorting
	if opts.SortBy != "" {
		order := "ASC"
		if opts.SortDesc {
			order = "DESC"
		}
		query = query.Order(fmt.Sprintf("%s %s", opts.SortBy, order))
	} else {
		query = query.Order("created_at DESC")
	}
	
	// Apply pagination
	if opts.Page > 0 && opts.Limit > 0 {
		offset := (opts.Page - 1) * opts.Limit
		query = query.Offset(offset).Limit(opts.Limit)
	}
	
	// Preload associations if requested
	if opts.IncludeRoles {
		query = query.Preload("Roles.Permissions")
	}
	
	err := query.Find(&users).Error
	return users, total, err
}

// Search searches users by name or email
func (r *UserRepository) Search(ctx context.Context, query string, limit int) ([]*user.User, error) {
	var users []*user.User
	
	searchTerm := "%" + strings.ToLower(query) + "%"
	
	err := r.db.WithContext(ctx).
		Where("LOWER(email) LIKE ? OR LOWER(username) LIKE ? OR LOWER(first_name) LIKE ? OR LOWER(last_name) LIKE ?",
			searchTerm, searchTerm, searchTerm, searchTerm).
		Limit(limit).
		Find(&users).Error
		
	return users, err
}

// BulkUpdate updates multiple users at once
func (r *UserRepository) BulkUpdate(ctx context.Context, userIDs []string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	return r.db.WithContext(ctx).
		Model(&user.User{}).
		Where("id IN ?", userIDs).
		Updates(updates).Error
}

// BulkDelete soft deletes multiple users
func (r *UserRepository) BulkDelete(ctx context.Context, userIDs []string) error {
	return r.db.WithContext(ctx).
		Model(&user.User{}).
		Where("id IN ?", userIDs).
		Update("status", user.StatusDeleted).Error
}

// AssignRole assigns a role to a user
func (r *UserRepository) AssignRole(ctx context.Context, userID, roleID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Check if user exists
		var u user.User
		if err := tx.Where("id = ?", userID).First(&u).Error; err != nil {
			return err
		}
		
		// Check if role exists
		var role user.Role
		if err := tx.Where("id = ?", roleID).First(&role).Error; err != nil {
			return err
		}
		
		// Assign role
		return tx.Model(&u).Association("Roles").Append(&role)
	})
}

// RemoveRole removes a role from a user
func (r *UserRepository) RemoveRole(ctx context.Context, userID, roleID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var u user.User
		if err := tx.Where("id = ?", userID).First(&u).Error; err != nil {
			return err
		}
		
		var role user.Role
		if err := tx.Where("id = ?", roleID).First(&role).Error; err != nil {
			return err
		}
		
		return tx.Model(&u).Association("Roles").Delete(&role)
	})
}

// GetUsersByRole gets all users with a specific role
func (r *UserRepository) GetUsersByRole(ctx context.Context, roleID string) ([]*user.User, error) {
	var users []*user.User
	
	err := r.db.WithContext(ctx).
		Joins("JOIN user_roles ON user_roles.user_id = users.id").
		Where("user_roles.role_id = ?", roleID).
		Preload("Roles").
		Find(&users).Error
		
	return users, err
}

// GetUsersByTeam gets all users in a specific team
func (r *UserRepository) GetUsersByTeam(ctx context.Context, teamID string) ([]*user.User, error) {
	var users []*user.User
	
	err := r.db.WithContext(ctx).
		Joins("JOIN team_members ON team_members.user_id = users.id").
		Where("team_members.team_id = ?", teamID).
		Find(&users).Error
		
	return users, err
}

// ExistsByEmail checks if a user exists with the given email
func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&user.User{}).
		Where("email = ?", email).
		Count(&count).Error
	return count > 0, err
}

// ExistsByUsername checks if a user exists with the given username
func (r *UserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&user.User{}).
		Where("username = ?", username).
		Count(&count).Error
	return count > 0, err
}

// GetActiveUsersCount returns the count of active users
func (r *UserRepository) GetActiveUsersCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&user.User{}).
		Where("status = ?", user.StatusActive).
		Count(&count).Error
	return count, err
}

// ListUsersOptions represents options for listing users
type ListUsersOptions struct {
	Page         int
	Limit        int
	Status       string
	RoleID       string
	TeamID       string
	SortBy       string
	SortDesc     bool
	IncludeRoles bool
}

// GetUser is deprecated - use GetByID instead
func (r *UserRepository) GetUser(ctx context.Context, id string) (*user.User, error) {
	return r.GetByID(ctx, id)
}

// UpdateUser is deprecated - use Update instead
func (r *UserRepository) UpdateUser(ctx context.Context, u *user.User) error {
	return r.Update(ctx, u)
}

// DeleteUser is deprecated - use Delete instead
func (r *UserRepository) DeleteUser(ctx context.Context, id string) error {
	return r.Delete(ctx, id)
}
