package ports

import (
	"context"

	"github.com/linkflow-go/pkg/contracts/user"
)

type AuthRepository interface {
	CreateUser(ctx context.Context, user *user.User) error
	GetUserByEmail(ctx context.Context, email string) (*user.User, error)
	GetUserByID(ctx context.Context, id string) (*user.User, error)
	GetUserByEmailVerifyToken(ctx context.Context, token string) (*user.User, error)
	UpdateUser(ctx context.Context, user *user.User) error
	CreateSession(ctx context.Context, session *user.Session) error
	GetSession(ctx context.Context, token string) (*user.Session, error)
	GetUserSessions(ctx context.Context, userID string) ([]*user.Session, error)
	GetSessionByID(ctx context.Context, sessionID string) (*user.Session, error)
	DeleteSession(ctx context.Context, token string) error
	DeleteSessionByID(ctx context.Context, sessionID string) error
	DeleteUserSessions(ctx context.Context, userID string) error
}
