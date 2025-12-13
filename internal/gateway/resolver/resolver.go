package resolver

import (
	"context"
	"net/http"

	"github.com/linkflow-go/pkg/config"
	"github.com/linkflow-go/pkg/logger"
)

// ServiceClients holds HTTP clients for microservices
type ServiceClients struct {
	AuthClient       *http.Client
	WorkflowClient   *http.Client
	ExecutionClient  *http.Client
	CredentialClient *http.Client
	ScheduleClient   *http.Client
	WebhookClient    *http.Client
	VariableClient   *http.Client
	AnalyticsClient  *http.Client
}

// Resolver is the GraphQL resolver root
type Resolver struct {
	config   *config.Config
	logger   logger.Logger
	clients  *ServiceClients
	baseURLs map[string]string
}

// NewResolver creates a new GraphQL resolver
func NewResolver(cfg *config.Config, log logger.Logger) *Resolver {
	clients := &ServiceClients{
		AuthClient:       &http.Client{},
		WorkflowClient:   &http.Client{},
		ExecutionClient:  &http.Client{},
		CredentialClient: &http.Client{},
		ScheduleClient:   &http.Client{},
		WebhookClient:    &http.Client{},
		VariableClient:   &http.Client{},
		AnalyticsClient:  &http.Client{},
	}

	baseURLs := map[string]string{
		"auth":       "http://auth-service:8080",
		"workflow":   "http://workflow-service:8080",
		"execution":  "http://execution-service:8080",
		"credential": "http://credential-service:8080",
		"schedule":   "http://schedule-service:8080",
		"webhook":    "http://webhook-service:8080",
		"variable":   "http://variable-service:8080",
		"analytics":  "http://analytics-service:8080",
	}

	return &Resolver{
		config:   cfg,
		logger:   log,
		clients:  clients,
		baseURLs: baseURLs,
	}
}

// Query returns the query resolver
func (r *Resolver) Query() QueryResolver {
	return &queryResolver{r}
}

// Mutation returns the mutation resolver
func (r *Resolver) Mutation() MutationResolver {
	return &mutationResolver{r}
}

// Subscription returns the subscription resolver
func (r *Resolver) Subscription() SubscriptionResolver {
	return &subscriptionResolver{r}
}

// QueryResolver interface
type QueryResolver interface {
	Me(ctx context.Context) (*User, error)
	User(ctx context.Context, id string) (*User, error)
	Workflow(ctx context.Context, id string) (*Workflow, error)
	Workflows(ctx context.Context, filter *WorkflowFilter, pagination *PaginationInput) (*WorkflowConnection, error)
	Execution(ctx context.Context, id string) (*Execution, error)
	Executions(ctx context.Context, filter *ExecutionFilter, pagination *PaginationInput) (*ExecutionConnection, error)
	NodeTypes(ctx context.Context) ([]*NodeType, error)
	Credentials(ctx context.Context) ([]*Credential, error)
	Schedules(ctx context.Context, workflowID *string) ([]*Schedule, error)
	Webhooks(ctx context.Context, workflowID *string) ([]*Webhook, error)
	Variables(ctx context.Context) ([]*Variable, error)
	Dashboard(ctx context.Context) (*Dashboard, error)
}

// MutationResolver interface
type MutationResolver interface {
	Login(ctx context.Context, input LoginInput) (*AuthPayload, error)
	Register(ctx context.Context, input RegisterInput) (*AuthPayload, error)
	CreateWorkflow(ctx context.Context, input CreateWorkflowInput) (*Workflow, error)
	UpdateWorkflow(ctx context.Context, id string, input UpdateWorkflowInput) (*Workflow, error)
	DeleteWorkflow(ctx context.Context, id string) (bool, error)
	ExecuteWorkflow(ctx context.Context, workflowID string, input map[string]interface{}) (*Execution, error)
	CancelExecution(ctx context.Context, id string) (*Execution, error)
	CreateCredential(ctx context.Context, input CreateCredentialInput) (*Credential, error)
	DeleteCredential(ctx context.Context, id string) (bool, error)
	CreateSchedule(ctx context.Context, input CreateScheduleInput) (*Schedule, error)
	DeleteSchedule(ctx context.Context, id string) (bool, error)
	SetVariable(ctx context.Context, key string, value string, varType *VariableType) (*Variable, error)
	DeleteVariable(ctx context.Context, key string) (bool, error)
}

// SubscriptionResolver interface
type SubscriptionResolver interface {
	ExecutionUpdated(ctx context.Context, executionID string) (<-chan *ExecutionUpdate, error)
	WorkflowExecutions(ctx context.Context, workflowID string) (<-chan *Execution, error)
	Notifications(ctx context.Context) (<-chan *Notification, error)
}

type queryResolver struct{ *Resolver }
type mutationResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }
