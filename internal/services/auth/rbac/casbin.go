package rbac

import (
	"fmt"
	"path/filepath"

	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/linkflow-go/pkg/database"
	"github.com/linkflow-go/pkg/logger"
	"gorm.io/gorm"
)

// Enforcer wraps Casbin enforcer with additional helper methods
type Enforcer struct {
	enforcer *casbin.Enforcer
	logger   logger.Logger
}

// NewEnforcer creates a new RBAC enforcer with database adapter
func NewEnforcer(db *database.DB, modelPath, policyPath string, log logger.Logger) (*Enforcer, error) {
	// Create a GORM adapter
	adapter, err := gormadapter.NewAdapterByDB(db.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter: %w", err)
	}

	// Load model configuration
	if modelPath == "" {
		modelPath = "deployments/rbac/model.conf"
	}

	e, err := casbin.NewEnforcer(modelPath, adapter)
	if err != nil {
		return nil, fmt.Errorf("failed to create enforcer: %w", err)
	}

	// Load initial policy from CSV file if provided
	if policyPath != "" {
		if err := e.LoadPolicy(); err != nil {
			// If no policy exists in DB, load from file
			if err == gorm.ErrRecordNotFound || isEmptyPolicy(e) {
				log.Info("Loading initial policy from file", "path", policyPath)
				if err := loadPolicyFromFile(e, policyPath); err != nil {
					return nil, fmt.Errorf("failed to load policy from file: %w", err)
				}
				// Save policy to database
				if err := e.SavePolicy(); err != nil {
					log.Error("Failed to save initial policy to database", "error", err)
				}
			} else {
				return nil, fmt.Errorf("failed to load policy: %w", err)
			}
		}
	}

	// Enable auto-save (automatically save policy to DB when changed)
	e.EnableAutoSave(true)

	// Enable log
	e.EnableLog(false) // Set to true for debugging

	return &Enforcer{
		enforcer: e,
		logger:   log,
	}, nil
}

// CheckPermission checks if a user has permission to perform an action on a resource
func (e *Enforcer) CheckPermission(userID, resource, action string) (bool, error) {
	allowed, err := e.enforcer.Enforce(userID, resource, action)
	if err != nil {
		e.logger.Error("Failed to check permission", "error", err, "user", userID, "resource", resource, "action", action)
		return false, err
	}
	
	e.logger.Debug("Permission check", "user", userID, "resource", resource, "action", action, "allowed", allowed)
	return allowed, nil
}

// AddRole assigns a role to a user
func (e *Enforcer) AddRole(userID, role string) error {
	added, err := e.enforcer.AddGroupingPolicy(userID, role)
	if err != nil {
		return fmt.Errorf("failed to add role: %w", err)
	}
	if !added {
		e.logger.Warn("Role already assigned", "user", userID, "role", role)
		return nil
	}
	
	e.logger.Info("Role assigned", "user", userID, "role", role)
	return nil
}

// RemoveRole removes a role from a user
func (e *Enforcer) RemoveRole(userID, role string) error {
	removed, err := e.enforcer.RemoveGroupingPolicy(userID, role)
	if err != nil {
		return fmt.Errorf("failed to remove role: %w", err)
	}
	if !removed {
		e.logger.Warn("Role not found for user", "user", userID, "role", role)
		return nil
	}
	
	e.logger.Info("Role removed", "user", userID, "role", role)
	return nil
}

// GetRoles returns all roles assigned to a user
func (e *Enforcer) GetRoles(userID string) ([]string, error) {
	roles, err := e.enforcer.GetRolesForUser(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles: %w", err)
	}
	return roles, nil
}

// GetUsersForRole returns all users with a specific role
func (e *Enforcer) GetUsersForRole(role string) ([]string, error) {
	users, err := e.enforcer.GetUsersForRole(role)
	if err != nil {
		return nil, fmt.Errorf("failed to get users for role: %w", err)
	}
	return users, nil
}

// AddPermission adds a permission policy
func (e *Enforcer) AddPermission(role, resource, action string) error {
	added, err := e.enforcer.AddPolicy(role, resource, action)
	if err != nil {
		return fmt.Errorf("failed to add permission: %w", err)
	}
	if !added {
		e.logger.Warn("Permission already exists", "role", role, "resource", resource, "action", action)
		return nil
	}
	
	e.logger.Info("Permission added", "role", role, "resource", resource, "action", action)
	return nil
}

// RemovePermission removes a permission policy
func (e *Enforcer) RemovePermission(role, resource, action string) error {
	removed, err := e.enforcer.RemovePolicy(role, resource, action)
	if err != nil {
		return fmt.Errorf("failed to remove permission: %w", err)
	}
	if !removed {
		e.logger.Warn("Permission not found", "role", role, "resource", resource, "action", action)
		return nil
	}
	
	e.logger.Info("Permission removed", "role", role, "resource", resource, "action", action)
	return nil
}

// GetPermissions returns all permissions for a role
func (e *Enforcer) GetPermissions(role string) ([][]string, error) {
	perms, err := e.enforcer.GetPermissionsForUser(role)
	if err != nil {
		return nil, err
	}
	return perms, nil
}

// GetAllRoles returns all available roles
func (e *Enforcer) GetAllRoles() []string {
	roles, err := e.enforcer.GetAllRoles()
	if err != nil {
		e.logger.Error("Failed to get all roles", "error", err)
		return []string{}
	}
	return roles
}

// GetAllPolicies returns all policies
func (e *Enforcer) GetAllPolicies() [][]string {
	policies, err := e.enforcer.GetPolicy()
	if err != nil {
		e.logger.Error("Failed to get all policies", "error", err)
		return [][]string{}
	}
	return policies
}

// DeleteUser removes all roles and permissions for a user
func (e *Enforcer) DeleteUser(userID string) error {
	// Remove all roles
	if _, err := e.enforcer.DeleteRolesForUser(userID); err != nil {
		return fmt.Errorf("failed to delete user roles: %w", err)
	}
	
	// Remove all permissions directly assigned to the user (if any)
	if _, err := e.enforcer.DeletePermissionsForUser(userID); err != nil {
		return fmt.Errorf("failed to delete user permissions: %w", err)
	}
	
	e.logger.Info("User deleted from RBAC", "user", userID)
	return nil
}

// Helper function to check if policy is empty
func isEmptyPolicy(e *casbin.Enforcer) bool {
	policies, err := e.GetPolicy()
	if err != nil {
		return true
	}
	groupingPolicies, err := e.GetGroupingPolicy()
	if err != nil {
		return true
	}
	return len(policies) == 0 && len(groupingPolicies) == 0
}

// Helper function to load policy from CSV file  
func loadPolicyFromFile(e *casbin.Enforcer, path string) error {
	// Convert relative path to absolute if needed
	_, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	
	// Load policy from CSV file
	return e.LoadPolicy()
}

// Predefined roles
const (
	RoleSuperAdmin = "super_admin"
	RoleAdmin      = "admin"
	RoleUser       = "user"
	RoleViewer     = "viewer"
)

// Predefined resources
const (
	ResourceUser       = "/api/v1/users"
	ResourceWorkflow   = "/api/v1/workflows"
	ResourceNode       = "/api/v1/nodes"
	ResourceCredential = "/api/v1/credentials"
	ResourceAnalytics  = "/api/v1/analytics"
	ResourceSchedule   = "/api/v1/schedules"
	ResourceAudit      = "/api/v1/audit"
)

// Predefined actions
const (
	ActionCreate = "create"
	ActionRead   = "read"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionExecute = "execute"
	ActionAll    = "*"
)
