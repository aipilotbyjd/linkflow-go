package service

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/linkflow-go/internal/domain/user"
	"github.com/linkflow-go/internal/services/auth/jwt"
	"github.com/linkflow-go/pkg/config"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockAuthRepository is a mock implementation of AuthRepository
type MockAuthRepository struct {
	mock.Mock
}

func (m *MockAuthRepository) CreateUser(ctx context.Context, u *user.User) error {
	args := m.Called(ctx, u)
	return args.Error(0)
}

func (m *MockAuthRepository) GetUserByEmail(ctx context.Context, email string) (*user.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user.User), args.Error(1)
}

func (m *MockAuthRepository) GetUserByID(ctx context.Context, id string) (*user.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user.User), args.Error(1)
}

func (m *MockAuthRepository) GetUserByEmailVerifyToken(ctx context.Context, token string) (*user.User, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user.User), args.Error(1)
}

func (m *MockAuthRepository) UpdateUser(ctx context.Context, u *user.User) error {
	args := m.Called(ctx, u)
	return args.Error(0)
}

func (m *MockAuthRepository) CreateSession(ctx context.Context, session *user.Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockAuthRepository) GetSession(ctx context.Context, token string) (*user.Session, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user.Session), args.Error(1)
}

func (m *MockAuthRepository) GetUserSessions(ctx context.Context, userID string) ([]*user.Session, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*user.Session), args.Error(1)
}

func (m *MockAuthRepository) GetSessionByID(ctx context.Context, sessionID string) (*user.Session, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user.Session), args.Error(1)
}

func (m *MockAuthRepository) DeleteSession(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockAuthRepository) DeleteSessionByID(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockAuthRepository) DeleteUserSessions(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// MockEventBus is a mock implementation of events.EventBus
type MockEventBus struct{}

func (m *MockEventBus) Publish(ctx context.Context, event events.Event) error {
	return nil
}

func (m *MockEventBus) Subscribe(topic string, handler events.EventHandler) error {
	return nil
}

func (m *MockEventBus) Close() error {
	return nil
}

// Test helpers
func setupTestService(t *testing.T) (*AuthService, *MockAuthRepository, *miniredis.Miniredis) {
	// Create mock repository
	mockRepo := new(MockAuthRepository)

	// Create in-memory Redis
	mr, err := miniredis.Run()
	require.NoError(t, err)

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Create JWT manager
	cfg := config.AuthConfig{
		JWT: config.JWTConfig{
			SecretKey:   "test-secret-key-for-testing-only",
			ExpiryHours: 1,
			RefreshDays: 7,
			Issuer:      "test-issuer",
			Algorithm:   "HS256",
		},
	}
	jwtManager, err := jwt.NewManager(cfg)
	require.NoError(t, err)

	// Create mock event bus
	eventBus := &MockEventBus{}

	// Create logger
	log := logger.NewNop()

	// Create service
	service := NewAuthService(mockRepo, jwtManager, redisClient, eventBus, nil, log)

	return service, mockRepo, mr
}

func createTestUser(email, password string) *user.User {
	u, _ := user.NewUser(email, password, "Test", "User")
	u.EmailVerified = true
	u.Status = user.StatusActive
	return u
}

// Tests

func TestAuthService_Register(t *testing.T) {
	service, mockRepo, mr := setupTestService(t)
	defer mr.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockRepo.On("GetUserByEmail", ctx, "new@example.com").Return(nil, assert.AnError).Once()
		mockRepo.On("CreateUser", ctx, mock.AnythingOfType("*user.User")).Return(nil).Once()

		u, err := service.Register(ctx, "new@example.com", "SecurePass123!", "John", "Doe")
		require.NoError(t, err)
		assert.NotNil(t, u)
		assert.Equal(t, "new@example.com", u.Email)
		assert.Equal(t, "John", u.FirstName)
		assert.Equal(t, "Doe", u.LastName)

		mockRepo.AssertExpectations(t)
	})

	t.Run("UserAlreadyExists", func(t *testing.T) {
		existingUser := createTestUser("existing@example.com", "password")
		mockRepo.On("GetUserByEmail", ctx, "existing@example.com").Return(existingUser, nil).Once()

		u, err := service.Register(ctx, "existing@example.com", "SecurePass123!", "John", "Doe")
		assert.Error(t, err)
		assert.Nil(t, u)
		assert.Contains(t, err.Error(), "already exists")

		mockRepo.AssertExpectations(t)
	})
}

func TestAuthService_Login(t *testing.T) {
	service, mockRepo, mr := setupTestService(t)
	defer mr.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		testUser := createTestUser("user@example.com", "SecurePass123!")
		mockRepo.On("GetUserByEmail", ctx, "user@example.com").Return(testUser, nil).Once()
		mockRepo.On("CreateSession", ctx, mock.AnythingOfType("*user.Session")).Return(nil).Once()
		mockRepo.On("UpdateUser", ctx, mock.AnythingOfType("*user.User")).Return(nil).Once()

		tokens, u, err := service.Login(ctx, "user@example.com", "SecurePass123!", "127.0.0.1", "Mozilla/5.0")
		require.NoError(t, err)
		assert.NotNil(t, tokens)
		assert.NotNil(t, u)
		assert.NotEmpty(t, tokens.AccessToken)
		assert.NotEmpty(t, tokens.RefreshToken)
		assert.Equal(t, 900, tokens.ExpiresIn)

		mockRepo.AssertExpectations(t)
	})

	t.Run("InvalidEmail", func(t *testing.T) {
		mockRepo.On("GetUserByEmail", ctx, "unknown@example.com").Return(nil, assert.AnError).Once()

		tokens, u, err := service.Login(ctx, "unknown@example.com", "password", "127.0.0.1", "Mozilla/5.0")
		assert.Error(t, err)
		assert.Nil(t, tokens)
		assert.Nil(t, u)
		assert.Contains(t, err.Error(), "invalid credentials")

		mockRepo.AssertExpectations(t)
	})

	t.Run("InvalidPassword", func(t *testing.T) {
		testUser := createTestUser("user2@example.com", "SecurePass123!")
		mockRepo.On("GetUserByEmail", ctx, "user2@example.com").Return(testUser, nil).Once()

		tokens, u, err := service.Login(ctx, "user2@example.com", "wrongpassword", "127.0.0.1", "Mozilla/5.0")
		assert.Error(t, err)
		assert.Nil(t, tokens)
		assert.Nil(t, u)
		assert.Contains(t, err.Error(), "invalid credentials")

		mockRepo.AssertExpectations(t)
	})

	t.Run("EmailNotVerified", func(t *testing.T) {
		testUser := createTestUser("unverified@example.com", "SecurePass123!")
		testUser.EmailVerified = false
		mockRepo.On("GetUserByEmail", ctx, "unverified@example.com").Return(testUser, nil).Once()

		tokens, u, err := service.Login(ctx, "unverified@example.com", "SecurePass123!", "127.0.0.1", "Mozilla/5.0")
		assert.Error(t, err)
		assert.Nil(t, tokens)
		assert.Nil(t, u)
		assert.Contains(t, err.Error(), "email not verified")

		mockRepo.AssertExpectations(t)
	})

	t.Run("AccountLockout", func(t *testing.T) {
		// Simulate lockout by setting the key in Redis
		mr.Set("lockout:locked@example.com", "1")

		tokens, u, err := service.Login(ctx, "locked@example.com", "password", "127.0.0.1", "Mozilla/5.0")
		assert.Error(t, err)
		assert.Nil(t, tokens)
		assert.Nil(t, u)
		assert.Contains(t, err.Error(), "temporarily locked")
	})
}

func TestAuthService_RefreshToken(t *testing.T) {
	service, mockRepo, mr := setupTestService(t)
	defer mr.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		testUser := createTestUser("refreshuser@example.com", "password")
		mockRepo.On("GetUserByID", ctx, testUser.ID).Return(testUser, nil).Once()

		// Generate initial refresh token
		cfg := config.AuthConfig{
			JWT: config.JWTConfig{
				SecretKey:   "test-secret-key-for-testing-only",
				ExpiryHours: 1,
				RefreshDays: 7,
				Issuer:      "test-issuer",
				Algorithm:   "HS256",
			},
		}
		jwtManager, _ := jwt.NewManager(cfg)
		refreshToken, _ := jwtManager.GenerateRefreshToken(testUser.ID)

		tokens, err := service.RefreshToken(ctx, refreshToken)
		require.NoError(t, err)
		assert.NotNil(t, tokens)
		assert.NotEmpty(t, tokens.AccessToken)
		assert.NotEmpty(t, tokens.RefreshToken)

		mockRepo.AssertExpectations(t)
	})

	t.Run("TokenRotation", func(t *testing.T) {
		testUser := createTestUser("rotateuser@example.com", "password")
		mockRepo.On("GetUserByID", ctx, testUser.ID).Return(testUser, nil).Times(2)

		cfg := config.AuthConfig{
			JWT: config.JWTConfig{
				SecretKey:   "test-secret-key-for-testing-only",
				ExpiryHours: 1,
				RefreshDays: 7,
				Issuer:      "test-issuer",
				Algorithm:   "HS256",
			},
		}
		jwtManager, _ := jwt.NewManager(cfg)
		refreshToken, _ := jwtManager.GenerateRefreshToken(testUser.ID)

		// First refresh should succeed
		tokens1, err := service.RefreshToken(ctx, refreshToken)
		require.NoError(t, err)
		assert.NotNil(t, tokens1)

		// Second refresh with same token should fail (token rotation)
		tokens2, err := service.RefreshToken(ctx, refreshToken)
		assert.Error(t, err)
		assert.Nil(t, tokens2)
		assert.Contains(t, err.Error(), "already been used")
	})
}

func TestAuthService_Logout(t *testing.T) {
	service, mockRepo, mr := setupTestService(t)
	defer mr.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockRepo.On("DeleteSession", ctx, "test-token").Return(nil).Once()

		err := service.Logout(ctx, "user-123", "test-token")
		require.NoError(t, err)

		// Verify token is blacklisted
		exists := mr.Exists("blacklist:test-token")
		assert.True(t, exists)

		mockRepo.AssertExpectations(t)
	})
}

func TestAuthService_VerifyEmail(t *testing.T) {
	service, mockRepo, mr := setupTestService(t)
	defer mr.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		testUser := createTestUser("verifyuser@example.com", "password")
		testUser.EmailVerified = false
		testUser.EmailVerifyToken = "valid-token"

		mockRepo.On("GetUserByEmailVerifyToken", ctx, "valid-token").Return(testUser, nil).Once()
		mockRepo.On("UpdateUser", ctx, mock.AnythingOfType("*user.User")).Return(nil).Once()

		err := service.VerifyEmail(ctx, "valid-token")
		require.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("InvalidToken", func(t *testing.T) {
		mockRepo.On("GetUserByEmailVerifyToken", ctx, "invalid-token").Return(nil, assert.AnError).Once()

		err := service.VerifyEmail(ctx, "invalid-token")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired")

		mockRepo.AssertExpectations(t)
	})
}

func TestAuthService_ForgotPassword(t *testing.T) {
	service, mockRepo, mr := setupTestService(t)
	defer mr.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		testUser := createTestUser("forgotpw@example.com", "password")
		mockRepo.On("GetUserByEmail", ctx, "forgotpw@example.com").Return(testUser, nil).Once()

		err := service.ForgotPassword(ctx, "forgotpw@example.com")
		require.NoError(t, err)

		// Verify a reset token was created
		keys := mr.Keys()
		hasResetKey := false
		for _, k := range keys {
			if len(k) > 6 && k[:6] == "reset:" {
				hasResetKey = true
				break
			}
		}
		assert.True(t, hasResetKey)

		mockRepo.AssertExpectations(t)
	})

	t.Run("UnknownEmail_NoError", func(t *testing.T) {
		// Should not reveal if user exists
		mockRepo.On("GetUserByEmail", ctx, "unknown@example.com").Return(nil, assert.AnError).Once()

		err := service.ForgotPassword(ctx, "unknown@example.com")
		assert.NoError(t, err) // No error to prevent email enumeration

		mockRepo.AssertExpectations(t)
	})
}

func TestAuthService_ResetPassword(t *testing.T) {
	service, mockRepo, mr := setupTestService(t)
	defer mr.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		testUser := createTestUser("resetpw@example.com", "oldpassword")
		resetToken := "valid-reset-token"

		// Set up reset token in Redis
		mr.Set("reset:"+resetToken, testUser.ID)

		mockRepo.On("GetUserByID", ctx, testUser.ID).Return(testUser, nil).Once()
		mockRepo.On("UpdateUser", ctx, mock.AnythingOfType("*user.User")).Return(nil).Once()

		err := service.ResetPassword(ctx, resetToken, "NewSecurePass456!")
		require.NoError(t, err)

		// Verify token was deleted
		exists := mr.Exists("reset:" + resetToken)
		assert.False(t, exists)

		mockRepo.AssertExpectations(t)
	})

	t.Run("InvalidToken", func(t *testing.T) {
		err := service.ResetPassword(ctx, "invalid-token", "NewPass123!")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired")
	})
}

func TestAuthService_SessionManagement(t *testing.T) {
	service, mockRepo, mr := setupTestService(t)
	defer mr.Close()

	ctx := context.Background()

	t.Run("GetUserSessions", func(t *testing.T) {
		sessions := []*user.Session{
			{ID: "session-1", UserID: "user-123", ExpiresAt: time.Now().Add(1 * time.Hour)},
			{ID: "session-2", UserID: "user-123", ExpiresAt: time.Now().Add(2 * time.Hour)},
		}
		mockRepo.On("GetUserSessions", ctx, "user-123").Return(sessions, nil).Once()

		result, err := service.GetUserSessions(ctx, "user-123")
		require.NoError(t, err)
		assert.Len(t, result, 2)

		mockRepo.AssertExpectations(t)
	})

	t.Run("RevokeSession", func(t *testing.T) {
		session := &user.Session{
			ID:     "session-1",
			UserID: "user-123",
			Token:  "access-token",
		}
		mockRepo.On("GetSessionByID", ctx, "session-1").Return(session, nil).Once()
		mockRepo.On("DeleteSessionByID", ctx, "session-1").Return(nil).Once()

		err := service.RevokeSession(ctx, "user-123", "session-1")
		require.NoError(t, err)

		// Verify token is blacklisted
		exists := mr.Exists("blacklist:access-token")
		assert.True(t, exists)

		mockRepo.AssertExpectations(t)
	})

	t.Run("RevokeSession_Unauthorized", func(t *testing.T) {
		session := &user.Session{
			ID:     "session-2",
			UserID: "other-user",
			Token:  "access-token-2",
		}
		mockRepo.On("GetSessionByID", ctx, "session-2").Return(session, nil).Once()

		err := service.RevokeSession(ctx, "user-123", "session-2")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")

		mockRepo.AssertExpectations(t)
	})
}
