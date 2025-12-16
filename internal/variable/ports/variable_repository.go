package ports

import (
	"context"

	variable "github.com/linkflow-go/internal/variable/domain"
)

type VariableRepository interface {
	Exists(ctx context.Context, key string) (bool, error)
	Create(ctx context.Context, v *variable.Variable) error
	GetByID(ctx context.Context, id string) (*variable.Variable, error)
	GetByKey(ctx context.Context, key string) (*variable.Variable, error)
	List(ctx context.Context) ([]*variable.Variable, error)
	Update(ctx context.Context, v *variable.Variable) error
	Delete(ctx context.Context, id string) error
}
