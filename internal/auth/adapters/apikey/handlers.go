package apikey

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/pkg/logger"
)

// Handlers contains API key HTTP handlers
type Handlers struct {
	service *APIKeyService
	logger  logger.Logger
}

// NewHandlers creates new API key handlers
func NewHandlers(service *APIKeyService, log logger.Logger) *Handlers {
	return &Handlers{
		service: service,
		logger:  log,
	}
}

// CreateRequest represents a request to create an API key
type CreateRequest struct {
	Name        string   `json:"name" binding:"required,min=1,max=255"`
	Permissions []string `json:"permissions"`
	ExpiresIn   string   `json:"expiresIn,omitempty"` // e.g., "30d", "90d", "1y", or empty for no expiry
}

// CreateResponse contains the created API key with the raw key value
type CreateResponse struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Key         string     `json:"key"` // Only shown once!
	KeyPrefix   string     `json:"keyPrefix"`
	Permissions []string   `json:"permissions"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// ListResponse contains a list of API keys (without raw key values)
type ListResponse struct {
	Keys []APIKeyResponse `json:"keys"`
}

// APIKeyResponse represents an API key in list responses
type APIKeyResponse struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	KeyPrefix   string     `json:"keyPrefix"`
	Permissions []string   `json:"permissions"`
	LastUsedAt  *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	RevokedAt   *time.Time `json:"revokedAt,omitempty"`
}

// Create creates a new API key
// POST /api/v1/auth/api-keys
func (h *Handlers) Create(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse expiry duration
	var expiresIn *time.Duration
	if req.ExpiresIn != "" {
		duration, err := parseDuration(req.ExpiresIn)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid expiresIn format. Use '30d', '90d', '1y', etc."})
			return
		}
		expiresIn = &duration
	}

	// Default permissions if none provided
	permissions := req.Permissions
	if len(permissions) == 0 {
		permissions = []string{"workflows:read", "workflows:write", "executions:read"}
	}

	result, err := h.service.Create(c.Request.Context(), CreateAPIKeyRequest{
		UserID:      userID.(string),
		Name:        req.Name,
		Permissions: permissions,
		ExpiresIn:   expiresIn,
	})
	if err != nil {
		h.logger.Error("Failed to create API key", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create API key"})
		return
	}

	c.JSON(http.StatusCreated, CreateResponse{
		ID:          result.APIKey.ID,
		Name:        result.APIKey.Name,
		Key:         result.RawKey, // Only shown once!
		KeyPrefix:   result.APIKey.KeyPrefix,
		Permissions: result.APIKey.Permissions,
		ExpiresAt:   result.APIKey.ExpiresAt,
		CreatedAt:   result.APIKey.CreatedAt,
	})
}

// List returns all API keys for the current user
// GET /api/v1/auth/api-keys
func (h *Handlers) List(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	keys, err := h.service.List(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Error("Failed to list API keys", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list API keys"})
		return
	}

	response := make([]APIKeyResponse, len(keys))
	for i, key := range keys {
		response[i] = APIKeyResponse{
			ID:          key.ID,
			Name:        key.Name,
			KeyPrefix:   key.KeyPrefix,
			Permissions: key.Permissions,
			LastUsedAt:  key.LastUsedAt,
			ExpiresAt:   key.ExpiresAt,
			CreatedAt:   key.CreatedAt,
			RevokedAt:   key.RevokedAt,
		}
	}

	c.JSON(http.StatusOK, ListResponse{Keys: response})
}

// Revoke revokes an API key
// DELETE /api/v1/auth/api-keys/:id
func (h *Handlers) Revoke(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	keyID := c.Param("id")
	if keyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API key ID required"})
		return
	}

	err := h.service.Revoke(c.Request.Context(), userID.(string), keyID)
	if err != nil {
		if err.Error() == "API key not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "unauthorized: API key does not belong to user" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("Failed to revoke API key", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke API key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key revoked successfully"})
}

// Delete permanently deletes an API key
// DELETE /api/v1/auth/api-keys/:id/permanent
func (h *Handlers) Delete(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	keyID := c.Param("id")
	if keyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API key ID required"})
		return
	}

	err := h.service.Delete(c.Request.Context(), userID.(string), keyID)
	if err != nil {
		if err.Error() == "API key not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "unauthorized: API key does not belong to user" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("Failed to delete API key", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete API key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key deleted permanently"})
}

// parseDuration parses duration strings like "30d", "90d", "1y"
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration")
	}

	unit := s[len(s)-1]
	value := s[:len(s)-1]

	var multiplier time.Duration
	switch unit {
	case 'd':
		multiplier = 24 * time.Hour
	case 'w':
		multiplier = 7 * 24 * time.Hour
	case 'm':
		multiplier = 30 * 24 * time.Hour
	case 'y':
		multiplier = 365 * 24 * time.Hour
	case 'h':
		return time.ParseDuration(s)
	default:
		return time.ParseDuration(s)
	}

	var num int
	if _, err := fmt.Sscanf(value, "%d", &num); err != nil {
		return 0, err
	}

	return time.Duration(num) * multiplier, nil
}
