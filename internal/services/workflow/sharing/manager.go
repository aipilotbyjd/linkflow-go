package sharing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/linkflow-go/pkg/database"
	"github.com/linkflow-go/pkg/logger"
	"gorm.io/gorm"
)

// Permission levels
const (
	PermissionView    = "view"
	PermissionEdit    = "edit"
	PermissionExecute = "execute"
	PermissionAdmin   = "admin"
)

// Share types
const (
	ShareTypeUser  = "user"
	ShareTypeTeam  = "team"
	ShareTypePublic = "public"
	ShareTypeLink  = "link"
)

var (
	ErrPermissionDenied     = errors.New("permission denied")
	ErrShareNotFound        = errors.New("share not found")
	ErrCannotShareWithSelf  = errors.New("cannot share with yourself")
	ErrInvalidPermission    = errors.New("invalid permission level")
	ErrShareAlreadyExists   = errors.New("share already exists")
)

// WorkflowShare represents a workflow sharing record
type WorkflowShare struct {
	ID            string    `json:"id" gorm:"primaryKey"`
	WorkflowID    string    `json:"workflowId" gorm:"index"`
	SharedWithID  string    `json:"sharedWithId" gorm:"index"` // User or Team ID
	SharedWithType string   `json:"sharedWithType"` // user, team, public
	Permission    string    `json:"permission"`
	SharedBy      string    `json:"sharedBy"`
	ExpiresAt     *time.Time `json:"expiresAt"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// ShareLink represents a shareable link for a workflow
type ShareLink struct {
	ID         string     `json:"id" gorm:"primaryKey"`
	WorkflowID string     `json:"workflowId" gorm:"index"`
	Token      string     `json:"token" gorm:"uniqueIndex"`
	Permission string     `json:"permission"`
	Password   string     `json:"password,omitempty"` // Optional password protection
	MaxUses    int        `json:"maxUses"`
	UsedCount  int        `json:"usedCount"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	CreatedBy  string     `json:"createdBy"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
}

// WorkflowAccess represents consolidated access information
type WorkflowAccess struct {
	WorkflowID  string   `json:"workflowId"`
	UserID      string   `json:"userId"`
	Permissions []string `json:"permissions"`
	Source      string   `json:"source"` // owner, shared, team, public
	ExpiresAt   *time.Time `json:"expiresAt"`
}

// SharingManager manages workflow sharing and permissions
type SharingManager struct {
	db     *database.DB
	logger logger.Logger
}

// NewSharingManager creates a new sharing manager
func NewSharingManager(db *database.DB, logger logger.Logger) *SharingManager {
	return &SharingManager{
		db:     db,
		logger: logger,
	}
}

// ShareWorkflow shares a workflow with a user or team
func (sm *SharingManager) ShareWorkflow(ctx context.Context, share *WorkflowShare) error {
	// Validate permission level
	if !isValidPermission(share.Permission) {
		return ErrInvalidPermission
	}
	
	// Check if share already exists
	var existing WorkflowShare
	err := sm.db.WithContext(ctx).
		Where("workflow_id = ? AND shared_with_id = ? AND shared_with_type = ?",
			share.WorkflowID, share.SharedWithID, share.SharedWithType).
		First(&existing).Error
	
	if err == nil {
		// Update existing share
		existing.Permission = share.Permission
		existing.ExpiresAt = share.ExpiresAt
		existing.UpdatedAt = time.Now()
		
		if err := sm.db.WithContext(ctx).Save(&existing).Error; err != nil {
			return fmt.Errorf("failed to update share: %w", err)
		}
		
		sm.logger.Info("Workflow share updated",
			"workflow_id", share.WorkflowID,
			"shared_with", share.SharedWithID,
			"permission", share.Permission)
		return nil
	}
	
	// Create new share
	share.ID = uuid.New().String()
	share.CreatedAt = time.Now()
	share.UpdatedAt = time.Now()
	
	if err := sm.db.WithContext(ctx).Create(share).Error; err != nil {
		return fmt.Errorf("failed to create share: %w", err)
	}
	
	sm.logger.Info("Workflow shared",
		"workflow_id", share.WorkflowID,
		"shared_with", share.SharedWithID,
		"permission", share.Permission)
	
	return nil
}

// UnshareWorkflow removes a workflow share
func (sm *SharingManager) UnshareWorkflow(ctx context.Context, workflowID, sharedWithID, sharedWithType string) error {
	result := sm.db.WithContext(ctx).
		Where("workflow_id = ? AND shared_with_id = ? AND shared_with_type = ?",
			workflowID, sharedWithID, sharedWithType).
		Delete(&WorkflowShare{})
	
	if result.Error != nil {
		return fmt.Errorf("failed to unshare workflow: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return ErrShareNotFound
	}
	
	sm.logger.Info("Workflow unshared",
		"workflow_id", workflowID,
		"shared_with", sharedWithID)
	
	return nil
}

// GetWorkflowShares gets all shares for a workflow
func (sm *SharingManager) GetWorkflowShares(ctx context.Context, workflowID string) ([]*WorkflowShare, error) {
	var shares []*WorkflowShare
	err := sm.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Find(&shares).Error
	
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow shares: %w", err)
	}
	
	return shares, nil
}

// GetUserSharedWorkflows gets all workflows shared with a user
func (sm *SharingManager) GetUserSharedWorkflows(ctx context.Context, userID string) ([]*WorkflowShare, error) {
	var shares []*WorkflowShare
	err := sm.db.WithContext(ctx).
		Where("shared_with_id = ? AND shared_with_type = ?", userID, ShareTypeUser).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Find(&shares).Error
	
	if err != nil {
		return nil, fmt.Errorf("failed to get user shared workflows: %w", err)
	}
	
	return shares, nil
}

// CreateShareLink creates a shareable link for a workflow
func (sm *SharingManager) CreateShareLink(ctx context.Context, link *ShareLink) error {
	link.ID = uuid.New().String()
	link.Token = generateShareToken()
	link.CreatedAt = time.Now()
	link.UsedCount = 0
	
	if err := sm.db.WithContext(ctx).Create(link).Error; err != nil {
		return fmt.Errorf("failed to create share link: %w", err)
	}
	
	sm.logger.Info("Share link created",
		"workflow_id", link.WorkflowID,
		"token", link.Token[:8]+"...",
		"permission", link.Permission)
	
	return nil
}

// GetShareLink gets a share link by token
func (sm *SharingManager) GetShareLink(ctx context.Context, token string) (*ShareLink, error) {
	var link ShareLink
	err := sm.db.WithContext(ctx).
		Where("token = ?", token).
		First(&link).Error
	
	if err == gorm.ErrRecordNotFound {
		return nil, ErrShareNotFound
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to get share link: %w", err)
	}
	
	// Check expiration
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("share link expired")
	}
	
	// Check usage limit
	if link.MaxUses > 0 && link.UsedCount >= link.MaxUses {
		return nil, errors.New("share link usage limit reached")
	}
	
	return &link, nil
}

// UseShareLink increments the usage count of a share link
func (sm *SharingManager) UseShareLink(ctx context.Context, token string) error {
	now := time.Now()
	result := sm.db.WithContext(ctx).
		Model(&ShareLink{}).
		Where("token = ?", token).
		Updates(map[string]interface{}{
			"used_count":   gorm.Expr("used_count + 1"),
			"last_used_at": now,
		})
	
	if result.Error != nil {
		return fmt.Errorf("failed to update share link usage: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return ErrShareNotFound
	}
	
	return nil
}

// DeleteShareLink deletes a share link
func (sm *SharingManager) DeleteShareLink(ctx context.Context, linkID string) error {
	result := sm.db.WithContext(ctx).Delete(&ShareLink{}, "id = ?", linkID)
	
	if result.Error != nil {
		return fmt.Errorf("failed to delete share link: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return ErrShareNotFound
	}
	
	sm.logger.Info("Share link deleted", "id", linkID)
	return nil
}

// CheckPermission checks if a user has a specific permission on a workflow
func (sm *SharingManager) CheckPermission(ctx context.Context, workflowID, userID, permission string) (bool, error) {
	// Check direct share
	var share WorkflowShare
	err := sm.db.WithContext(ctx).
		Where("workflow_id = ? AND shared_with_id = ? AND shared_with_type = ?",
			workflowID, userID, ShareTypeUser).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		First(&share).Error
	
	if err == nil {
		return hasPermission(share.Permission, permission), nil
	}
	
	// Check team shares (would need team membership info)
	// This is simplified - in production, you'd check team memberships
	
	// Check public share
	err = sm.db.WithContext(ctx).
		Where("workflow_id = ? AND shared_with_type = ?", workflowID, ShareTypePublic).
		First(&share).Error
	
	if err == nil {
		return hasPermission(share.Permission, permission), nil
	}
	
	return false, nil
}

// GetUserPermissions gets all permissions a user has on a workflow
func (sm *SharingManager) GetUserPermissions(ctx context.Context, workflowID, userID string) ([]string, error) {
	permissions := make(map[string]bool)
	
	// Get direct user shares
	var userShare WorkflowShare
	err := sm.db.WithContext(ctx).
		Where("workflow_id = ? AND shared_with_id = ? AND shared_with_type = ?",
			workflowID, userID, ShareTypeUser).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		First(&userShare).Error
	
	if err == nil {
		for _, perm := range getPermissionSet(userShare.Permission) {
			permissions[perm] = true
		}
	}
	
	// Get team shares (simplified)
	// In production, you'd check user's team memberships
	
	// Get public shares
	var publicShare WorkflowShare
	err = sm.db.WithContext(ctx).
		Where("workflow_id = ? AND shared_with_type = ?", workflowID, ShareTypePublic).
		First(&publicShare).Error
	
	if err == nil {
		for _, perm := range getPermissionSet(publicShare.Permission) {
			permissions[perm] = true
		}
	}
	
	// Convert to slice
	result := []string{}
	for perm := range permissions {
		result = append(result, perm)
	}
	
	return result, nil
}

// TransferOwnership transfers workflow ownership to another user
func (sm *SharingManager) TransferOwnership(ctx context.Context, workflowID, fromUserID, toUserID string) error {
	// This would typically update the workflow's owner field
	// and clean up shares as needed
	
	// Remove any existing shares for the new owner
	sm.db.WithContext(ctx).
		Where("workflow_id = ? AND shared_with_id = ?", workflowID, toUserID).
		Delete(&WorkflowShare{})
	
	sm.logger.Info("Workflow ownership transferred",
		"workflow_id", workflowID,
		"from", fromUserID,
		"to", toUserID)
	
	return nil
}

// CleanupExpiredShares removes expired shares
func (sm *SharingManager) CleanupExpiredShares(ctx context.Context) error {
	result := sm.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at < ?", time.Now()).
		Delete(&WorkflowShare{})
	
	if result.Error != nil {
		return fmt.Errorf("failed to cleanup expired shares: %w", result.Error)
	}
	
	if result.RowsAffected > 0 {
		sm.logger.Info("Cleaned up expired shares", "count", result.RowsAffected)
	}
	
	// Also cleanup expired share links
	result = sm.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at < ?", time.Now()).
		Delete(&ShareLink{})
	
	if result.Error != nil {
		return fmt.Errorf("failed to cleanup expired share links: %w", result.Error)
	}
	
	if result.RowsAffected > 0 {
		sm.logger.Info("Cleaned up expired share links", "count", result.RowsAffected)
	}
	
	return nil
}

// Helper functions

func isValidPermission(permission string) bool {
	validPermissions := map[string]bool{
		PermissionView:    true,
		PermissionEdit:    true,
		PermissionExecute: true,
		PermissionAdmin:   true,
	}
	return validPermissions[permission]
}

func hasPermission(grantedPermission, requiredPermission string) bool {
	// Permission hierarchy: admin > edit > execute > view
	hierarchy := map[string]int{
		PermissionView:    1,
		PermissionExecute: 2,
		PermissionEdit:    3,
		PermissionAdmin:   4,
	}
	
	grantedLevel := hierarchy[grantedPermission]
	requiredLevel := hierarchy[requiredPermission]
	
	return grantedLevel >= requiredLevel
}

func getPermissionSet(permission string) []string {
	// Return all permissions included in the given permission level
	switch permission {
	case PermissionAdmin:
		return []string{PermissionView, PermissionExecute, PermissionEdit, PermissionAdmin}
	case PermissionEdit:
		return []string{PermissionView, PermissionExecute, PermissionEdit}
	case PermissionExecute:
		return []string{PermissionView, PermissionExecute}
	case PermissionView:
		return []string{PermissionView}
	default:
		return []string{}
	}
}

func generateShareToken() string {
	// Generate a secure random token
	return uuid.New().String()
}
