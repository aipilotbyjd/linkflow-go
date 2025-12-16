package domain

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
	ActionCreate  = "create"
	ActionRead    = "read"
	ActionUpdate  = "update"
	ActionDelete  = "delete"
	ActionExecute = "execute"
	ActionAll     = "*"
)
