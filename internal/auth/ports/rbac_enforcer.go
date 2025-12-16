package ports

type RBACEnforcer interface {
	AddRole(userID, role string) error
	RemoveRole(userID, role string) error
	GetRoles(userID string) ([]string, error)
	GetUsersForRole(role string) ([]string, error)
	GetPermissions(role string) ([][]string, error)
	GetAllRoles() []string
	CheckPermission(userID, resource, action string) (bool, error)
}
