package ports

import (
	"context"

	"github.com/linkflow-go/pkg/contracts/user"
)

type UserRepository interface {
	GetByID(ctx context.Context, id string) (*user.User, error)
	ListUsers(ctx context.Context, opts ListUsersOptions) ([]*user.User, int64, error)
	Update(ctx context.Context, u *user.User) error
	Delete(ctx context.Context, id string) error
	Search(ctx context.Context, query string, limit int) ([]*user.User, error)
	AssignRole(ctx context.Context, userID, roleID string) error
	RemoveRole(ctx context.Context, userID, roleID string) error
}

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
