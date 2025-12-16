package ports

import (
	"context"

	"github.com/linkflow-go/pkg/contracts/credential"
)

type CredentialRepository interface {
	CreateCredential(ctx context.Context, cred *credential.Credential) error
	GetCredential(ctx context.Context, id string) (*credential.Credential, error)
	UpdateCredential(ctx context.Context, cred *credential.Credential) error
	ListCredentials(ctx context.Context, userID string) ([]*credential.Credential, error)
	DeleteCredential(ctx context.Context, id string) error
}
