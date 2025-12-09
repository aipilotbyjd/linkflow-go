package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/linkflow-go/internal/domain/user"
	"github.com/linkflow-go/internal/services/auth/jwt"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type AuthService struct {
	repository  AuthRepository
	jwtManager  *jwt.Manager
	redis       *redis.Client
	eventBus    events.EventBus
	logger      logger.Logger
}

type AuthRepository interface {
	CreateUser(ctx context.Context, user *user.User) error
	GetUserByEmail(ctx context.Context, email string) (*user.User, error)
	GetUserByID(ctx context.Context, id string) (*user.User, error)
	UpdateUser(ctx context.Context, user *user.User) error
	CreateSession(ctx context.Context, session *user.Session) error
	GetSession(ctx context.Context, token string) (*user.Session, error)
	DeleteSession(ctx context.Context, token string) error
}

type Tokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn"`
}

func NewAuthService(repo AuthRepository, jwtManager *jwt.Manager, redis *redis.Client, eventBus events.EventBus, logger logger.Logger) *AuthService {
	return &AuthService{
		repository:  repo,
		jwtManager:  jwtManager,
		redis:       redis,
		eventBus:    eventBus,
		logger:      logger,
	}
}

func (s *AuthService) Register(ctx context.Context, email, password, firstName, lastName string) (*user.User, error) {
	// Check if user already exists
	existingUser, _ := s.repository.GetUserByEmail(ctx, email)
	if existingUser != nil {
		return nil, errors.New("user already exists")
	}

	// Create new user
	newUser, err := user.NewUser(email, password, firstName, lastName)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Save user to database
	if err := s.repository.CreateUser(ctx, newUser); err != nil {
		return nil, fmt.Errorf("failed to save user: %w", err)
	}

	// Publish user registered event
	event := events.NewEventBuilder(events.UserRegistered).
		WithAggregateID(newUser.ID).
		WithAggregateType("user").
		WithUserID(newUser.ID).
		WithPayload("email", newUser.Email).
		WithPayload("firstName", newUser.FirstName).
		WithPayload("lastName", newUser.LastName).
		Build()

	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Error("Failed to publish user registered event", "error", err)
	}

	// Send verification email (async)
	go s.sendVerificationEmail(newUser)

	return newUser, nil
}

func (s *AuthService) Login(ctx context.Context, email, password, ipAddress, userAgent string) (*Tokens, *user.User, error) {
	// Get user by email
	u, err := s.repository.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, nil, errors.New("invalid credentials")
	}

	// Check password
	if !u.CheckPassword(password) {
		return nil, nil, errors.New("invalid credentials")
	}

	// Check if email is verified
	if !u.EmailVerified {
		return nil, nil, errors.New("email not verified")
	}

	// Check if account is active
	if u.Status != user.StatusActive {
		return nil, nil, errors.New("account is not active")
	}

	// Generate tokens
	accessToken, err := s.jwtManager.GenerateToken(u.ID, u.Email, u.GetRoleNames(), u.GetPermissions())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(u.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Create session
	session := &user.Session{
		ID:           uuid.New().String(),
		UserID:       u.ID,
		Token:        accessToken,
		RefreshToken: refreshToken,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.repository.CreateSession(ctx, session); err != nil {
		return nil, nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Update last login time
	now := time.Now()
	u.LastLoginAt = &now
	s.repository.UpdateUser(ctx, u)

	// Publish login event
	event := events.NewEventBuilder(events.UserLoggedIn).
		WithAggregateID(u.ID).
		WithAggregateType("user").
		WithUserID(u.ID).
		WithPayload("ipAddress", ipAddress).
		WithPayload("userAgent", userAgent).
		Build()

	s.eventBus.Publish(ctx, event)

	tokens := &Tokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900, // 15 minutes
	}

	return tokens, u, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*Tokens, error) {
	// Validate refresh token
	userID, err := s.jwtManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	// Get user
	u, err := s.repository.GetUserByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Generate new tokens
	accessToken, err := s.jwtManager.GenerateToken(u.ID, u.Email, u.GetRoleNames(), u.GetPermissions())
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	newRefreshToken, err := s.jwtManager.GenerateRefreshToken(u.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &Tokens{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    900,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, userID, token string) error {
	// Delete session
	if err := s.repository.DeleteSession(ctx, token); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// Add token to blacklist
	s.redis.Set(ctx, fmt.Sprintf("blacklist:%s", token), "1", 24*time.Hour)

	// Publish logout event
	event := events.NewEventBuilder(events.UserLoggedOut).
		WithAggregateID(userID).
		WithAggregateType("user").
		WithUserID(userID).
		Build()

	s.eventBus.Publish(ctx, event)

	return nil
}

func (s *AuthService) GetUser(ctx context.Context, userID string) (*user.User, error) {
	return s.repository.GetUserByID(ctx, userID)
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) (*user.User, error) {
	u, err := s.repository.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Update allowed fields
	if firstName, ok := updates["firstName"].(string); ok {
		u.FirstName = firstName
	}
	if lastName, ok := updates["lastName"].(string); ok {
		u.LastName = lastName
	}
	if avatar, ok := updates["avatar"].(string); ok {
		u.Avatar = avatar
	}

	u.UpdatedAt = time.Now()

	if err := s.repository.UpdateUser(ctx, u); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Publish user updated event
	event := events.NewEventBuilder(events.UserUpdated).
		WithAggregateID(u.ID).
		WithAggregateType("user").
		WithUserID(u.ID).
		WithPayload("updates", updates).
		Build()

	s.eventBus.Publish(ctx, event)

	return u, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	u, err := s.repository.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// Verify old password
	if !u.CheckPassword(oldPassword) {
		return errors.New("incorrect old password")
	}

	// Set new password
	if err := u.SetPassword(newPassword); err != nil {
		return fmt.Errorf("failed to set password: %w", err)
	}

	if err := s.repository.UpdateUser(ctx, u); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	// In production, this would look up the token in database
	// For now, we'll just log it
	s.logger.Info("Email verification", "token", token)
	return nil
}

func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	u, err := s.repository.GetUserByEmail(ctx, email)
	if err != nil {
		// Don't reveal if user exists
		return nil
	}

	// Generate reset token
	resetToken := uuid.New().String()
	
	// Store token in Redis with expiry
	s.redis.Set(ctx, fmt.Sprintf("reset:%s", resetToken), u.ID, 1*time.Hour)

	// Send reset email (async)
	go s.sendPasswordResetEmail(u, resetToken)

	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	// Get user ID from token
	userID, err := s.redis.Get(ctx, fmt.Sprintf("reset:%s", token)).Result()
	if err != nil {
		return errors.New("invalid or expired reset token")
	}

	// Get user
	u, err := s.repository.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// Set new password
	if err := u.SetPassword(newPassword); err != nil {
		return fmt.Errorf("failed to set password: %w", err)
	}

	if err := s.repository.UpdateUser(ctx, u); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	// Delete reset token
	s.redis.Del(ctx, fmt.Sprintf("reset:%s", token))

	return nil
}

func (s *AuthService) GetOAuthURL(provider string) (string, error) {
	// Generate OAuth URL based on provider
	// This would integrate with OAuth providers
	return fmt.Sprintf("https://oauth.provider.com/authorize?client_id=xxx&provider=%s", provider), nil
}

func (s *AuthService) HandleOAuthCallback(ctx context.Context, provider, code string) (*Tokens, *user.User, error) {
	// Handle OAuth callback
	// This would exchange code for tokens and get user info
	// For now, return mock data
	return &Tokens{
		AccessToken:  "mock-oauth-token",
		RefreshToken: "mock-oauth-refresh",
		ExpiresIn:    3600,
	}, nil, nil
}

func (s *AuthService) Setup2FA(ctx context.Context, userID string) (string, string, error) {
	// Generate 2FA secret and QR code
	// This would integrate with TOTP libraries
	secret := "mock-2fa-secret"
	qrCode := "data:image/png;base64,mock-qr-code"
	return secret, qrCode, nil
}

func (s *AuthService) Verify2FA(ctx context.Context, userID, code string) error {
	// Verify 2FA code
	// This would validate TOTP code
	return nil
}

func (s *AuthService) Disable2FA(ctx context.Context, userID, password string) error {
	// Disable 2FA after password verification
	return nil
}

func (s *AuthService) CheckReadiness(ctx context.Context) error {
	// Check database connection
	if _, err := s.repository.GetUserByID(ctx, "test"); err != nil {
		// Ignore not found error
	}
	
	// Check Redis connection
	if err := s.redis.Ping(ctx).Err(); err != nil {
		return err
	}
	
	return nil
}

func (s *AuthService) sendVerificationEmail(u *user.User) {
	// Send verification email
	s.logger.Info("Sending verification email", "email", u.Email, "token", u.EmailVerifyToken)
}

func (s *AuthService) sendPasswordResetEmail(u *user.User, token string) {
	// Send password reset email
	s.logger.Info("Sending password reset email", "email", u.Email, "token", token)
}
