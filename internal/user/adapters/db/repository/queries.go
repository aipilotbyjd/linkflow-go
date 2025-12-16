package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/linkflow-go/pkg/contracts/user"
	"gorm.io/gorm"
)

// UserStats represents user statistics
type UserStats struct {
	TotalUsers         int64
	ActiveUsers        int64
	InactiveUsers      int64
	SuspendedUsers     int64
	NewUsersToday      int64
	NewUsersThisWeek   int64
	NewUsersThisMonth  int64
	UsersWithTwoFactor int64
	VerifiedEmails     int64
}

// GetUserStats returns aggregated user statistics
func (r *UserRepository) GetUserStats(ctx context.Context) (*UserStats, error) {
	stats := &UserStats{}

	// Total users
	if err := r.db.WithContext(ctx).Model(&user.User{}).Count(&stats.TotalUsers).Error; err != nil {
		return nil, err
	}

	// Active users
	if err := r.db.WithContext(ctx).Model(&user.User{}).
		Where("status = ?", user.StatusActive).
		Count(&stats.ActiveUsers).Error; err != nil {
		return nil, err
	}

	// Inactive users
	if err := r.db.WithContext(ctx).Model(&user.User{}).
		Where("status = ?", user.StatusInactive).
		Count(&stats.InactiveUsers).Error; err != nil {
		return nil, err
	}

	// Suspended users
	if err := r.db.WithContext(ctx).Model(&user.User{}).
		Where("status = ?", user.StatusSuspended).
		Count(&stats.SuspendedUsers).Error; err != nil {
		return nil, err
	}

	// New users today
	today := time.Now().Truncate(24 * time.Hour)
	if err := r.db.WithContext(ctx).Model(&user.User{}).
		Where("created_at >= ?", today).
		Count(&stats.NewUsersToday).Error; err != nil {
		return nil, err
	}

	// New users this week
	weekAgo := time.Now().AddDate(0, 0, -7)
	if err := r.db.WithContext(ctx).Model(&user.User{}).
		Where("created_at >= ?", weekAgo).
		Count(&stats.NewUsersThisWeek).Error; err != nil {
		return nil, err
	}

	// New users this month
	monthAgo := time.Now().AddDate(0, -1, 0)
	if err := r.db.WithContext(ctx).Model(&user.User{}).
		Where("created_at >= ?", monthAgo).
		Count(&stats.NewUsersThisMonth).Error; err != nil {
		return nil, err
	}

	// Users with two-factor enabled
	if err := r.db.WithContext(ctx).Model(&user.User{}).
		Where("two_factor_enabled = ?", true).
		Count(&stats.UsersWithTwoFactor).Error; err != nil {
		return nil, err
	}

	// Verified emails
	if err := r.db.WithContext(ctx).Model(&user.User{}).
		Where("email_verified = ?", true).
		Count(&stats.VerifiedEmails).Error; err != nil {
		return nil, err
	}

	return stats, nil
}

// FindInactiveUsers finds users who haven't logged in for a specified duration
func (r *UserRepository) FindInactiveUsers(ctx context.Context, inactiveDuration time.Duration) ([]*user.User, error) {
	var users []*user.User

	cutoffDate := time.Now().Add(-inactiveDuration)

	err := r.db.WithContext(ctx).
		Where("last_login_at < ? OR last_login_at IS NULL", cutoffDate).
		Where("status = ?", user.StatusActive).
		Find(&users).Error

	return users, err
}

// FindUsersWithoutRole finds users that don't have any role assigned
func (r *UserRepository) FindUsersWithoutRole(ctx context.Context) ([]*user.User, error) {
	var users []*user.User

	err := r.db.WithContext(ctx).
		Table("users").
		Where("NOT EXISTS (SELECT 1 FROM user_roles WHERE user_roles.user_id = users.id)").
		Find(&users).Error

	return users, err
}

// GetUsersWithPermission finds all users that have a specific permission
func (r *UserRepository) GetUsersWithPermission(ctx context.Context, resource, action string) ([]*user.User, error) {
	var users []*user.User

	err := r.db.WithContext(ctx).
		Distinct("users.*").
		Table("users").
		Joins("JOIN user_roles ON user_roles.user_id = users.id").
		Joins("JOIN role_permissions ON role_permissions.role_id = user_roles.role_id").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Where("permissions.resource = ? AND permissions.action = ?", resource, action).
		Where("users.status = ?", user.StatusActive).
		Find(&users).Error

	return users, err
}

// SearchByMultipleFields performs an advanced search across multiple fields
func (r *UserRepository) SearchByMultipleFields(ctx context.Context, opts SearchOptions) ([]*user.User, int64, error) {
	var users []*user.User
	var total int64

	query := r.db.WithContext(ctx).Model(&user.User{})

	// Apply search filters
	if opts.Email != "" {
		query = query.Where("email ILIKE ?", "%"+opts.Email+"%")
	}

	if opts.Username != "" {
		query = query.Where("username ILIKE ?", "%"+opts.Username+"%")
	}

	if opts.FirstName != "" {
		query = query.Where("first_name ILIKE ?", "%"+opts.FirstName+"%")
	}

	if opts.LastName != "" {
		query = query.Where("last_name ILIKE ?", "%"+opts.LastName+"%")
	}

	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}

	if opts.EmailVerified != nil {
		query = query.Where("email_verified = ?", *opts.EmailVerified)
	}

	if opts.TwoFactorEnabled != nil {
		query = query.Where("two_factor_enabled = ?", *opts.TwoFactorEnabled)
	}

	if opts.CreatedAfter != nil {
		query = query.Where("created_at >= ?", opts.CreatedAfter)
	}

	if opts.CreatedBefore != nil {
		query = query.Where("created_at <= ?", opts.CreatedBefore)
	}

	// Count total
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

	// Execute query
	err := query.Find(&users).Error
	return users, total, err
}

// SearchOptions represents advanced search options
type SearchOptions struct {
	Email            string
	Username         string
	FirstName        string
	LastName         string
	Status           string
	EmailVerified    *bool
	TwoFactorEnabled *bool
	CreatedAfter     *time.Time
	CreatedBefore    *time.Time
	Page             int
	Limit            int
	SortBy           string
	SortDesc         bool
}

// GetUserActivity returns recent activity for a user
func (r *UserRepository) GetUserActivity(ctx context.Context, userID string, limit int) ([]UserActivity, error) {
	var activities []UserActivity

	// Get audit logs for the user
	query := `
		SELECT 
			'audit' as type,
			action as activity,
			resource_type || ':' || COALESCE(resource_id, '') as details,
			created_at
		FROM audit.audit_logs
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	err := r.db.WithContext(ctx).Raw(query, userID, limit).Scan(&activities).Error
	return activities, err
}

// UserActivity represents a user's activity log entry
type UserActivity struct {
	Type      string    `json:"type"`
	Activity  string    `json:"activity"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"createdAt"`
}

// GetUserPermissionsDetailed returns detailed permission information for a user
func (r *UserRepository) GetUserPermissionsDetailed(ctx context.Context, userID string) ([]PermissionDetail, error) {
	var permissions []PermissionDetail

	query := `
		SELECT DISTINCT
			p.id,
			p.name,
			p.resource,
			p.action,
			r.name as role_name,
			r.id as role_id
		FROM permissions p
		JOIN role_permissions rp ON rp.permission_id = p.id
		JOIN roles r ON r.id = rp.role_id
		JOIN user_roles ur ON ur.role_id = r.id
		WHERE ur.user_id = ?
		ORDER BY p.resource, p.action
	`

	err := r.db.WithContext(ctx).Raw(query, userID).Scan(&permissions).Error
	return permissions, err
}

// PermissionDetail represents detailed permission information
type PermissionDetail struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
	RoleName string `json:"roleName"`
	RoleID   string `json:"roleId"`
}

// UpdateUserMetadata updates user metadata in a transaction
func (r *UserRepository) UpdateUserMetadata(ctx context.Context, userID string, metadata map[string]interface{}) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update user fields
		if err := tx.Model(&user.User{}).
			Where("id = ?", userID).
			Updates(metadata).Error; err != nil {
			return err
		}

		// Log the update in audit log
		auditEntry := map[string]interface{}{
			"user_id":       userID,
			"action":        "update_metadata",
			"resource_type": "user",
			"resource_id":   userID,
			"changes":       metadata,
			"created_at":    time.Now(),
		}

		return tx.Table("audit.audit_logs").Create(auditEntry).Error
	})
}

// GetTeamMembers returns all members of teams that a user belongs to
func (r *UserRepository) GetTeamMembers(ctx context.Context, userID string) ([]*user.User, error) {
	var users []*user.User

	// First get all teams the user belongs to, then get all members of those teams
	query := `
		SELECT DISTINCT u.*
		FROM users u
		JOIN team_members tm ON tm.user_id = u.id
		WHERE tm.team_id IN (
			SELECT team_id 
			FROM team_members 
			WHERE user_id = ?
		)
		AND u.id != ?
		AND u.status = ?
	`

	err := r.db.WithContext(ctx).
		Raw(query, userID, userID, user.StatusActive).
		Scan(&users).Error

	return users, err
}

// CountUsersByStatus returns count of users grouped by status
func (r *UserRepository) CountUsersByStatus(ctx context.Context) (map[string]int64, error) {
	type StatusCount struct {
		Status string
		Count  int64
	}

	var counts []StatusCount
	result := make(map[string]int64)

	err := r.db.WithContext(ctx).
		Model(&user.User{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&counts).Error

	if err != nil {
		return nil, err
	}

	for _, sc := range counts {
		result[sc.Status] = sc.Count
	}

	return result, nil
}
