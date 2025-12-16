package router

import (
	"context"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/webhook/ports"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type WebhookRouter struct {
	redis  *redis.Client
	logger logger.Logger
	routes map[string]interface{}
	mutex  sync.RWMutex
	stopCh chan struct{}
}

func NewWebhookRouter(redis *redis.Client, logger logger.Logger) *WebhookRouter {
	return &WebhookRouter{
		redis:  redis,
		logger: logger,
		routes: make(map[string]interface{}),
		stopCh: make(chan struct{}),
	}
}

func (r *WebhookRouter) LoadRoutes(ctx context.Context, repo ports.WebhookRepository) error {
	webhooks, err := repo.ListActive(ctx)
	if err != nil {
		return err
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, webhook := range webhooks {
		// Store webhook routes in memory
		_ = webhook
	}

	return nil
}

func (r *WebhookRouter) RouteWebhook(c *gin.Context) {
	path := c.Param("path")
	r.logger.Info("Routing webhook", "path", path)

	// Route to appropriate webhook handler
	c.JSON(http.StatusOK, gin.H{"message": "Webhook received"})
}

func (r *WebhookRouter) StartBackgroundTasks(ctx context.Context) {
	// Background tasks for webhook router
	r.logger.Info("Starting webhook router background tasks")
}

func (r *WebhookRouter) Stop() {
	close(r.stopCh)
}
