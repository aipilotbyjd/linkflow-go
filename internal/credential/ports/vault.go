package ports

import (
	"context"

	"github.com/linkflow-go/pkg/contracts/credential"
)

type Vault interface {
	EncryptCredential(ctx context.Context, cred *credential.Credential) error
	DecryptCredential(ctx context.Context, cred *credential.Credential) error
}
