package ports

import (
	"context"

	"github.com/linkflow-go/internal/workflow/adapters/templates"
	"github.com/linkflow-go/pkg/contracts/workflow"
)

type TemplateManager interface {
	CreateTemplate(ctx context.Context, template *templates.Template) error
	ListTemplates(ctx context.Context, category string, isPublic *bool) ([]*templates.Template, error)
	GetTemplate(ctx context.Context, templateID string) (*templates.Template, error)
	InstantiateTemplate(ctx context.Context, templateID, userID, name string, variables map[string]interface{}) (*workflow.Workflow, error)
	GetCategories() []map[string]interface{}
}
