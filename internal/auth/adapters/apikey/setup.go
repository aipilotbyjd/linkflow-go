package apikey

import (
	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/pkg/logger"
	"gorm.io/gorm"
)

// SetupRoutes registers API key routes on the given router group
// This should be called with the protected auth routes group
func SetupRoutes(protected *gin.RouterGroup, db *gorm.DB, log logger.Logger) *Handlers {
	// Create repository
	// Note: Database migrations are handled via SQL migration files in /migrations
	repo := NewGormAPIKeyRepository(db)

	// Create service
	service := NewAPIKeyService(repo)

	// Create handlers
	handlers := NewHandlers(service, log)

	// Register routes
	apiKeys := protected.Group("/api-keys")
	{
		apiKeys.POST("", handlers.Create)
		apiKeys.GET("", handlers.List)
		apiKeys.DELETE("/:id", handlers.Revoke)
		apiKeys.DELETE("/:id/permanent", handlers.Delete)
	}

	return handlers
}

// GetService returns the API key service for use in middleware
func (h *Handlers) GetService() *APIKeyService {
	return h.service
}
