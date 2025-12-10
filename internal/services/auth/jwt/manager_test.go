package jwt

import (
	"testing"
	"time"

	"github.com/linkflow-go/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTManager(t *testing.T) {
	// Create test config
	cfg := config.AuthConfig{
		JWT: config.JWTConfig{
			SecretKey:   "test-secret-key",
			ExpiryHours: 1,
			RefreshDays: 7,
			Issuer:      "test-issuer",
			Algorithm:   "HS256",
		},
	}

	// Create JWT manager
	manager, err := NewManager(cfg)
	require.NoError(t, err)
	assert.NotNil(t, manager)

	t.Run("GenerateAndValidateToken", func(t *testing.T) {
		// Test data
		userID := "user-123"
		email := "test@example.com"
		roles := []string{"user", "admin"}
		permissions := []string{"read:users", "write:users"}

		// Generate token
		token, err := manager.GenerateToken(userID, email, roles, permissions)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Validate token
		claims, err := manager.ValidateToken(token)
		require.NoError(t, err)
		assert.NotNil(t, claims)

		// Verify claims
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, email, claims.Email)
		assert.Equal(t, roles, claims.Roles)
		assert.Equal(t, permissions, claims.Permissions)
		assert.Equal(t, "test-issuer", claims.Issuer)
	})

	t.Run("GenerateAndValidateRefreshToken", func(t *testing.T) {
		userID := "user-456"

		// Generate refresh token
		refreshToken, err := manager.GenerateRefreshToken(userID)
		require.NoError(t, err)
		assert.NotEmpty(t, refreshToken)

		// Validate refresh token
		validatedUserID, err := manager.ValidateRefreshToken(refreshToken)
		require.NoError(t, err)
		assert.Equal(t, userID, validatedUserID)
	})

	t.Run("RefreshToken", func(t *testing.T) {
		// Generate original token
		userID := "user-789"
		email := "refresh@example.com"
		roles := []string{"user"}
		permissions := []string{"read:profile"}

		originalToken, err := manager.GenerateToken(userID, email, roles, permissions)
		require.NoError(t, err)

		// Refresh the token
		newToken, err := manager.RefreshToken(originalToken)
		require.NoError(t, err)
		assert.NotEmpty(t, newToken)
		assert.NotEqual(t, originalToken, newToken) // Should be different

		// Validate new token
		claims, err := manager.ValidateToken(newToken)
		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, email, claims.Email)
		assert.Equal(t, roles, claims.Roles)
		assert.Equal(t, permissions, claims.Permissions)
	})

	t.Run("InvalidToken", func(t *testing.T) {
		// Try to validate an invalid token
		_, err := manager.ValidateToken("invalid-token")
		assert.Error(t, err)
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		// Create manager with very short expiry
		shortCfg := config.AuthConfig{
			JWT: config.JWTConfig{
				SecretKey:   "test-secret-key",
				ExpiryHours: 0, // This will create immediately expired tokens (for testing)
				RefreshDays: 7,
				Issuer:      "test-issuer",
				Algorithm:   "HS256",
			},
		}

		shortManager, err := NewManager(shortCfg)
		require.NoError(t, err)

		// Generate token with 0 expiry (immediately expired)
		shortManager.expiry = -1 * time.Second // Make it expired
		token, err := shortManager.GenerateToken("user", "test@test.com", []string{}, []string{})
		require.NoError(t, err)

		// Try to validate expired token
		_, err = shortManager.ValidateToken(token)
		assert.Error(t, err)
	})
}
