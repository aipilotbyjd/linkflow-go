package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linkflow-go/internal/domain/webhook"
	"github.com/linkflow-go/internal/services/webhook/repository"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type WebhookService struct {
	repo       *repository.WebhookRepository
	eventBus   events.EventBus
	redis      *redis.Client
	logger     logger.Logger
	webhooks   map[string]*webhook.Webhook // path -> webhook
	webhooksMu sync.RWMutex
}

func NewWebhookService(
	repo *repository.WebhookRepository,
	eventBus events.EventBus,
	redis *redis.Client,
	logger logger.Logger,
) *WebhookService {
	return &WebhookService{
		repo:     repo,
		eventBus: eventBus,
		redis:    redis,
		logger:   logger,
		webhooks: make(map[string]*webhook.Webhook),
	}
}

func (s *WebhookService) Start(ctx context.Context) error {
	s.logger.Info("Starting webhook service")

	// Load all active webhooks
	if err := s.loadWebhooks(ctx); err != nil {
		return fmt.Errorf("failed to load webhooks: %w", err)
	}

	// Subscribe to events
	s.eventBus.Subscribe("workflow.activated", func(ctx context.Context, event events.Event) error {
		return s.handleWorkflowActivated(ctx, event)
	})

	s.eventBus.Subscribe("workflow.deactivated", func(ctx context.Context, event events.Event) error {
		return s.handleWorkflowDeactivated(ctx, event)
	})

	return nil
}

func (s *WebhookService) loadWebhooks(ctx context.Context) error {
	webhooks, err := s.repo.ListActive(ctx)
	if err != nil {
		return err
	}

	s.webhooksMu.Lock()
	defer s.webhooksMu.Unlock()

	for _, wh := range webhooks {
		s.webhooks[wh.Path] = wh
	}

	s.logger.Info("Loaded webhooks", "count", len(webhooks))
	return nil
}

// CreateWebhook creates a new webhook
func (s *WebhookService) CreateWebhook(ctx context.Context, req CreateWebhookRequest) (*webhook.Webhook, error) {
	// Generate unique path if not provided
	path := req.Path
	if path == "" {
		path = uuid.New().String()
	}

	// Ensure path starts without leading slash
	path = strings.TrimPrefix(path, "/")

	// Check if path already exists
	existing, _ := s.repo.GetByPath(ctx, path)
	if existing != nil {
		return nil, fmt.Errorf("webhook path already exists")
	}

	wh := webhook.NewWebhook(req.WorkflowID, req.NodeID, req.UserID, path)
	wh.Name = req.Name
	wh.Method = req.Method
	if wh.Method == "" {
		wh.Method = "POST"
	}
	wh.RequireAuth = req.RequireAuth
	wh.AuthType = req.AuthType
	wh.AuthConfig = req.AuthConfig
	wh.Headers = req.Headers
	wh.RateLimit = req.RateLimit
	if wh.RateLimit == 0 {
		wh.RateLimit = 100
	}

	if err := wh.Validate(); err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, wh); err != nil {
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}

	// Add to cache
	s.webhooksMu.Lock()
	s.webhooks[wh.Path] = wh
	s.webhooksMu.Unlock()

	// Publish event
	event := events.NewEventBuilder("webhook.created").
		WithAggregateID(wh.ID).
		WithUserID(req.UserID).
		WithPayload("path", wh.Path).
		WithPayload("workflowId", wh.WorkflowID).
		Build()
	s.eventBus.Publish(ctx, event)

	s.logger.Info("Webhook created", "id", wh.ID, "path", wh.Path)
	return wh, nil
}

// GetWebhook gets a webhook by ID
func (s *WebhookService) GetWebhook(ctx context.Context, id string) (*webhook.Webhook, error) {
	return s.repo.GetByID(ctx, id)
}

// GetWebhookByPath gets a webhook by path
func (s *WebhookService) GetWebhookByPath(ctx context.Context, path string) (*webhook.Webhook, error) {
	// Check cache first
	s.webhooksMu.RLock()
	if wh, ok := s.webhooks[path]; ok {
		s.webhooksMu.RUnlock()
		return wh, nil
	}
	s.webhooksMu.RUnlock()

	// Load from database
	return s.repo.GetByPath(ctx, path)
}

// ListWebhooks lists webhooks for a user
func (s *WebhookService) ListWebhooks(ctx context.Context, userID string) ([]*webhook.Webhook, error) {
	return s.repo.ListByUser(ctx, userID)
}

// ListWebhooksByWorkflow lists webhooks for a workflow
func (s *WebhookService) ListWebhooksByWorkflow(ctx context.Context, workflowID string) ([]*webhook.Webhook, error) {
	return s.repo.ListByWorkflow(ctx, workflowID)
}

// UpdateWebhook updates a webhook
func (s *WebhookService) UpdateWebhook(ctx context.Context, id string, req UpdateWebhookRequest) (*webhook.Webhook, error) {
	wh, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		wh.Name = req.Name
	}
	if req.Method != "" {
		wh.Method = req.Method
	}
	if req.IsActive != nil {
		wh.IsActive = *req.IsActive
	}
	if req.RequireAuth != nil {
		wh.RequireAuth = *req.RequireAuth
	}
	if req.AuthType != "" {
		wh.AuthType = req.AuthType
	}
	if req.AuthConfig != nil {
		wh.AuthConfig = req.AuthConfig
	}
	if req.RateLimit > 0 {
		wh.RateLimit = req.RateLimit
	}

	wh.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, wh); err != nil {
		return nil, fmt.Errorf("failed to update webhook: %w", err)
	}

	// Update cache
	s.webhooksMu.Lock()
	s.webhooks[wh.Path] = wh
	s.webhooksMu.Unlock()

	return wh, nil
}

// DeleteWebhook deletes a webhook
func (s *WebhookService) DeleteWebhook(ctx context.Context, id string) error {
	wh, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}

	// Remove from cache
	s.webhooksMu.Lock()
	delete(s.webhooks, wh.Path)
	s.webhooksMu.Unlock()

	// Publish event
	event := events.NewEventBuilder("webhook.deleted").
		WithAggregateID(id).
		WithPayload("path", wh.Path).
		Build()
	s.eventBus.Publish(ctx, event)

	return nil
}

// HandleWebhook handles an incoming webhook request
func (s *WebhookService) HandleWebhook(ctx context.Context, path string, r *http.Request) (*webhook.WebhookResponse, int, error) {
	// Find webhook
	wh, err := s.GetWebhookByPath(ctx, path)
	if err != nil {
		return nil, http.StatusNotFound, webhook.ErrWebhookNotFound
	}

	// Check if webhook can accept requests
	if err := wh.CanAccept(); err != nil {
		return nil, http.StatusForbidden, err
	}

	// Check method
	if wh.Method != r.Method && wh.Method != "ANY" {
		return nil, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed")
	}

	// Check rate limit
	if err := s.checkRateLimit(ctx, wh); err != nil {
		return nil, http.StatusTooManyRequests, err
	}

	// Verify authentication if required
	if wh.RequireAuth {
		if err := s.verifyAuth(wh, r); err != nil {
			return nil, http.StatusUnauthorized, err
		}
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to read body: %w", err)
	}

	// Verify signature if secret is set
	signature := r.Header.Get("X-Webhook-Signature")
	if wh.Secret != "" && signature != "" {
		if !wh.VerifySignature(body, signature) {
			return nil, http.StatusUnauthorized, webhook.ErrInvalidSignature
		}
	}

	// Parse body
	var payload map[string]interface{}
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		if len(body) > 0 {
			if err := json.Unmarshal(body, &payload); err != nil {
				payload = map[string]interface{}{"raw": string(body)}
			}
		}
	} else {
		payload = map[string]interface{}{"raw": string(body)}
	}

	// Add request metadata
	payload["_webhook"] = map[string]interface{}{
		"path":        path,
		"method":      r.Method,
		"headers":     headerToMap(r.Header),
		"queryParams": queryToMap(r.URL.Query()),
		"remoteAddr":  r.RemoteAddr,
		"timestamp":   time.Now().Unix(),
	}

	// Create execution record
	execution := &webhook.WebhookExecution{
		ID:          uuid.New().String(),
		WebhookID:   wh.ID,
		WorkflowID:  wh.WorkflowID,
		Method:      r.Method,
		Path:        path,
		Headers:     headerToMap(r.Header),
		QueryParams: queryToMap(r.URL.Query()),
		Body:        string(body),
		ContentType: contentType,
		IPAddress:   r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Status:      "received",
		CreatedAt:   time.Now(),
	}

	if err := s.repo.RecordExecution(ctx, execution); err != nil {
		s.logger.Error("Failed to record webhook execution", "error", err)
	}

	// Trigger workflow execution
	event := events.NewEventBuilder("webhook.received").
		WithAggregateID(wh.ID).
		WithPayload("webhookId", wh.ID).
		WithPayload("workflowId", wh.WorkflowID).
		WithPayload("nodeId", wh.NodeID).
		WithPayload("executionId", execution.ID).
		WithPayload("data", payload).
		Build()

	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Error("Failed to publish webhook event", "error", err)
		execution.Status = "failed"
		execution.Error = err.Error()
		s.repo.UpdateExecution(ctx, execution)
		return nil, http.StatusInternalServerError, err
	}

	// Update webhook stats
	wh.RecordCall()
	s.repo.Update(ctx, wh)

	// Update execution
	execution.Status = "processed"
	now := time.Now()
	execution.ProcessedAt = &now
	execution.Duration = now.Sub(execution.CreatedAt).Milliseconds()
	s.repo.UpdateExecution(ctx, execution)

	return &webhook.WebhookResponse{
		Success:     true,
		ExecutionID: execution.ID,
		Message:     "Webhook received and workflow triggered",
	}, http.StatusOK, nil
}

// checkRateLimit checks if the webhook has exceeded its rate limit
func (s *WebhookService) checkRateLimit(ctx context.Context, wh *webhook.Webhook) error {
	key := fmt.Sprintf("webhook:ratelimit:%s", wh.ID)

	// Get current count
	count, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		s.logger.Error("Failed to check rate limit", "error", err)
		return nil // Allow on error
	}

	// Set expiry on first request
	if count == 1 {
		s.redis.Expire(ctx, key, time.Minute)
	}

	if int(count) > wh.RateLimit {
		return webhook.ErrRateLimitExceeded
	}

	return nil
}

// verifyAuth verifies the authentication for a webhook request
func (s *WebhookService) verifyAuth(wh *webhook.Webhook, r *http.Request) error {
	switch wh.AuthType {
	case "header":
		headerName := wh.AuthConfig["headerName"]
		expectedValue := wh.AuthConfig["headerValue"]
		if r.Header.Get(headerName) != expectedValue {
			return fmt.Errorf("invalid auth header")
		}
	case "basic":
		username, password, ok := r.BasicAuth()
		if !ok {
			return fmt.Errorf("basic auth required")
		}
		if username != wh.AuthConfig["username"] || password != wh.AuthConfig["password"] {
			return fmt.Errorf("invalid credentials")
		}
	case "bearer":
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token != wh.AuthConfig["token"] {
			return fmt.Errorf("invalid bearer token")
		}
	}
	return nil
}

// Event handlers
func (s *WebhookService) handleWorkflowActivated(ctx context.Context, event events.Event) error {
	workflowID, _ := event.Payload["workflowId"].(string)
	s.logger.Info("Workflow activated, enabling webhooks", "workflowId", workflowID)

	webhooks, _ := s.repo.ListByWorkflow(ctx, workflowID)
	for _, wh := range webhooks {
		wh.IsActive = true
		s.repo.Update(ctx, wh)

		s.webhooksMu.Lock()
		s.webhooks[wh.Path] = wh
		s.webhooksMu.Unlock()
	}

	return nil
}

func (s *WebhookService) handleWorkflowDeactivated(ctx context.Context, event events.Event) error {
	workflowID, _ := event.Payload["workflowId"].(string)
	s.logger.Info("Workflow deactivated, disabling webhooks", "workflowId", workflowID)

	webhooks, _ := s.repo.ListByWorkflow(ctx, workflowID)
	for _, wh := range webhooks {
		wh.IsActive = false
		s.repo.Update(ctx, wh)

		s.webhooksMu.Lock()
		delete(s.webhooks, wh.Path)
		s.webhooksMu.Unlock()
	}

	return nil
}

func (s *WebhookService) HandleWorkflowExecuted(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling workflow executed event for webhook")
	return nil
}

func (s *WebhookService) HandleWorkflowFailed(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling workflow failed event for webhook")
	return nil
}

func (s *WebhookService) HandleExecutionCompleted(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling execution completed event for webhook")
	return nil
}

// Helper functions
func headerToMap(h http.Header) map[string]string {
	result := make(map[string]string)
	for k, v := range h {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}

func queryToMap(q map[string][]string) map[string]string {
	result := make(map[string]string)
	for k, v := range q {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}

// Request types
type CreateWebhookRequest struct {
	WorkflowID  string            `json:"workflowId" binding:"required"`
	NodeID      string            `json:"nodeId" binding:"required"`
	UserID      string            `json:"-"`
	Name        string            `json:"name"`
	Path        string            `json:"path"`
	Method      string            `json:"method"`
	RequireAuth bool              `json:"requireAuth"`
	AuthType    string            `json:"authType"`
	AuthConfig  map[string]string `json:"authConfig"`
	Headers     map[string]string `json:"headers"`
	RateLimit   int               `json:"rateLimit"`
}

type UpdateWebhookRequest struct {
	Name        string            `json:"name"`
	Method      string            `json:"method"`
	IsActive    *bool             `json:"isActive"`
	RequireAuth *bool             `json:"requireAuth"`
	AuthType    string            `json:"authType"`
	AuthConfig  map[string]string `json:"authConfig"`
	RateLimit   int               `json:"rateLimit"`
}
