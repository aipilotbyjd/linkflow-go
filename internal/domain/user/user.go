package user

import (
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID               string    `json:"id" gorm:"primaryKey"`
	Email            string    `json:"email" gorm:"uniqueIndex;not null"`
	Username         string    `json:"username" gorm:"uniqueIndex"`
	Password         string    `json:"-" gorm:"not null"`
	FirstName        string    `json:"firstName"`
	LastName         string    `json:"lastName"`
	Avatar           string    `json:"avatar"`
	EmailVerified    bool      `json:"emailVerified" gorm:"default:false"`
	EmailVerifyToken string    `json:"-"`
	TwoFactorEnabled bool      `json:"twoFactorEnabled" gorm:"default:false"`
	TwoFactorSecret  string    `json:"-"`
	Status           string    `json:"status" gorm:"default:'active'"`
	Roles            []Role    `json:"roles" gorm:"many2many:user_roles"`
	LastLoginAt      *time.Time `json:"lastLoginAt"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type Role struct {
	ID          string       `json:"id" gorm:"primaryKey"`
	Name        string       `json:"name" gorm:"uniqueIndex;not null"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions" gorm:"many2many:role_permissions"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

type Permission struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"uniqueIndex;not null"`
	Resource  string    `json:"resource"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Session struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	UserID       string    `json:"userId" gorm:"not null;index"`
	Token        string    `json:"token" gorm:"uniqueIndex;not null"`
	RefreshToken string    `json:"refreshToken" gorm:"uniqueIndex"`
	IPAddress    string    `json:"ipAddress"`
	UserAgent    string    `json:"userAgent"`
	ExpiresAt    time.Time `json:"expiresAt"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type OAuthToken struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	UserID       string    `json:"userId" gorm:"not null;index"`
	Provider     string    `json:"provider" gorm:"not null"`
	AccessToken  string    `json:"accessToken" gorm:"not null"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
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
	ProviderGoogle   = "google"
	ProviderGitHub   = "github"
	ProviderMicrosoft = "microsoft"
)

// NewUser creates a new user with hashed password
func NewUser(email, password, firstName, lastName string) (*User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	return &User{
		ID:               uuid.New().String(),
		Email:            email,
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
