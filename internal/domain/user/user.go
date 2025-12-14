package user

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID               string     `json:"id" gorm:"primaryKey"`
	Email            string     `json:"email" gorm:"uniqueIndex;not null"`
	Username         string     `json:"username" gorm:"uniqueIndex"`
	Password         string     `json:"-" gorm:"column:password_hash;not null"`
	FirstName        string     `json:"firstName" gorm:"column:first_name"`
	LastName         string     `json:"lastName" gorm:"column:last_name"`
	Avatar           string     `json:"avatar" gorm:"column:avatar_url"`
	EmailVerified    bool       `json:"emailVerified" gorm:"column:email_verified;default:false"`
	EmailVerifyToken string     `json:"-" gorm:"column:email_verify_token"`
	TwoFactorEnabled bool       `json:"twoFactorEnabled" gorm:"column:two_factor_enabled;default:false"`
	TwoFactorSecret  string     `json:"-" gorm:"column:two_factor_secret"`
	Status           string     `json:"status" gorm:"default:'active'"`
	Roles            []Role     `json:"roles" gorm:"many2many:auth.user_roles"`
	LastLoginAt      *time.Time `json:"lastLoginAt" gorm:"column:last_login_at"`
	CreatedAt        time.Time  `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt        time.Time  `json:"updatedAt" gorm:"column:updated_at"`
}

// TableName specifies the table name for GORM
func (User) TableName() string {
	return "auth.users"
}

type Role struct {
	ID          string       `json:"id" gorm:"primaryKey"`
	Name        string       `json:"name" gorm:"uniqueIndex;not null"`
	Description string       `json:"description"`
	IsSystem    bool         `json:"isSystem" gorm:"column:is_system;default:false"`
	Permissions []Permission `json:"permissions" gorm:"many2many:auth.role_permissions"`
	CreatedAt   time.Time    `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt   time.Time    `json:"updatedAt" gorm:"column:updated_at"`
}

// TableName specifies the table name for GORM
func (Role) TableName() string {
	return "auth.roles"
}

type Permission struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"uniqueIndex;not null"`
	Resource    string    `json:"resource"`
	Action      string    `json:"action"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at"`
}

// TableName specifies the table name for GORM
func (Permission) TableName() string {
	return "auth.permissions"
}

type Session struct {
	ID           string     `json:"id" gorm:"primaryKey"`
	UserID       string     `json:"userId" gorm:"column:user_id;not null;index"`
	Token        string     `json:"token" gorm:"column:token_hash;uniqueIndex;not null"`
	RefreshToken string     `json:"refreshToken" gorm:"column:refresh_token_hash;uniqueIndex"`
	IPAddress    string     `json:"ipAddress" gorm:"column:ip_address"`
	UserAgent    string     `json:"userAgent" gorm:"column:user_agent"`
	ExpiresAt    time.Time  `json:"expiresAt" gorm:"column:expires_at"`
	RevokedAt    *time.Time `json:"revokedAt" gorm:"column:revoked_at"`
	CreatedAt    time.Time  `json:"createdAt" gorm:"column:created_at"`
}

// TableName specifies the table name for GORM
func (Session) TableName() string {
	return "auth.sessions"
}

type OAuthToken struct {
	ID             string    `json:"id" gorm:"primaryKey"`
	UserID         string    `json:"userId" gorm:"column:user_id;not null;index"`
	Provider       string    `json:"provider" gorm:"not null"`
	ProviderUserID string    `json:"providerUserId" gorm:"column:provider_user_id"`
	AccessToken    string    `json:"accessToken" gorm:"column:access_token"`
	RefreshToken   string    `json:"refreshToken" gorm:"column:refresh_token"`
	ExpiresAt      time.Time `json:"expiresAt" gorm:"column:token_expires_at"`
	CreatedAt      time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt      time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

// TableName specifies the table name for GORM
func (OAuthToken) TableName() string {
	return "auth.oauth_connections"
}

// User status constants
const (
	StatusActive    = "active"
	StatusInactive  = "inactive"
	StatusSuspended = "suspended"
	StatusDeleted   = "deleted"
)

// OAuth provider constants
const (
	ProviderGoogle    = "google"
	ProviderGitHub    = "github"
	ProviderMicrosoft = "microsoft"
)

// NewUser creates a new user with hashed password
func NewUser(email, password, firstName, lastName string) (*User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Generate username from email (part before @) + random suffix
	username := strings.Split(email, "@")[0] + "-" + uuid.New().String()[:8]

	return &User{
		ID:               uuid.New().String(),
		Email:            email,
		Username:         username,
		Password:         string(hashedPassword),
		FirstName:        firstName,
		LastName:         lastName,
		Status:           StatusActive,
		EmailVerifyToken: uuid.New().String(),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil
}

// CheckPassword verifies the password
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

// SetPassword updates the user's password
func (u *User) SetPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	u.UpdatedAt = time.Now()
	return nil
}

// FullName returns the user's full name
func (u *User) FullName() string {
	if u.FirstName == "" && u.LastName == "" {
		return u.Username
	}
	return u.FirstName + " " + u.LastName
}

// HasRole checks if user has a specific role
func (u *User) HasRole(roleName string) bool {
	for _, role := range u.Roles {
		if role.Name == roleName {
			return true
		}
	}
	return false
}

// HasPermission checks if user has a specific permission
func (u *User) HasPermission(resource, action string) bool {
	for _, role := range u.Roles {
		for _, perm := range role.Permissions {
			if perm.Resource == resource && perm.Action == action {
				return true
			}
		}
	}
	return false
}

// GetPermissions returns all user permissions
func (u *User) GetPermissions() []string {
	permMap := make(map[string]bool)
	for _, role := range u.Roles {
		for _, perm := range role.Permissions {
			permMap[perm.Resource+":"+perm.Action] = true
		}
	}

	permissions := make([]string, 0, len(permMap))
	for perm := range permMap {
		permissions = append(permissions, perm)
	}
	return permissions
}

// GetRoleNames returns all user role names
func (u *User) GetRoleNames() []string {
	roles := make([]string, len(u.Roles))
	for i, role := range u.Roles {
		roles[i] = role.Name
	}
	return roles
}
