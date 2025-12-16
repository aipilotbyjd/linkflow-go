package auth

import "context"

// APIKeyInfo is the minimal API key payload needed by middleware.
type APIKeyInfo struct {
	ID          string
	UserID      string
	Permissions []string
}

// APIKeyValidator validates an API key and returns key metadata for request context.
type APIKeyValidator interface {
	Validate(ctx context.Context, rawKey string) (*APIKeyInfo, error)
}

// PermissionChecker is the minimal RBAC interface used by authorization middleware.
type PermissionChecker interface {
	CheckPermission(subject, object, action string) (bool, error)
	GetRoles(subject string) ([]string, error)
}
