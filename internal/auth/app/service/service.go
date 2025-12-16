package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	authdomain "github.com/linkflow-go/internal/auth/domain"
	"github.com/linkflow-go/internal/auth/ports"
	"github.com/linkflow-go/pkg/auth/jwt"
	"github.com/linkflow-go/pkg/contracts/user"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type AuthService struct {
	repository ports.AuthRepository
	jwtManager *jwt.Manager
	redis      *redis.Client
	eventBus   events.EventBus
	rbac       ports.RBACEnforcer
	logger     logger.Logger
}

type Tokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn"`
}

func NewAuthService(repo ports.AuthRepository, jwtManager *jwt.Manager, redis *redis.Client, eventBus events.EventBus, rbacEnforcer ports.RBACEnforcer, logger logger.Logger) *AuthService {
	return &AuthService{
		repository: repo,
		jwtManager: jwtManager,
		redis:      redis,
		eventBus:   eventBus,
		rbac:       rbacEnforcer,
		logger:     logger,
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

	// Assign default role to new user
	if s.rbac != nil {
		if err := s.rbac.AddRole(newUser.ID, authdomain.RoleUser); err != nil {
			s.logger.Error("Failed to assign default role to user", "error", err, "userID", newUser.ID)
		}
	}

	// Send verification email (async)
	go s.sendVerificationEmail(newUser)

	return newUser, nil
}

func (s *AuthService) Login(ctx context.Context, email, password, ipAddress, userAgent string) (*Tokens, *user.User, error) {
	// Check if account is locked due to too many failed attempts
	lockKey := fmt.Sprintf("lockout:%s", email)
	locked, _ := s.redis.Exists(ctx, lockKey).Result()
	if locked > 0 {
		return nil, nil, errors.New("account is temporarily locked due to too many failed login attempts")
	}

	// Get user by email
	u, err := s.repository.GetUserByEmail(ctx, email)
	if err != nil {
		s.trackFailedLogin(ctx, email, ipAddress)
		return nil, nil, errors.New("invalid credentials")
	}

	// Check password
	if !u.CheckPassword(password) {
		s.trackFailedLogin(ctx, email, ipAddress)
		return nil, nil, errors.New("invalid credentials")
	}

	// Clear failed login attempts on successful login
	s.redis.Del(ctx, fmt.Sprintf("failed_attempts:%s", email))

	// Check if email is verified
	if !u.EmailVerified {
		return nil, nil, errors.New("email not verified")
	}

	// Check if account is active
	if u.Status != user.StatusActive {
		return nil, nil, errors.New("account is not active")
	}

	// Get roles from RBAC
	var roles []string
	if s.rbac != nil {
		roles, _ = s.rbac.GetRoles(u.ID)
		// If no RBAC roles, fall back to database roles
		if len(roles) == 0 {
			roles = u.GetRoleNames()
		}
	} else {
		roles = u.GetRoleNames()
	}

	// Get permissions from RBAC
	var permissions []string
	if s.rbac != nil {
		for _, role := range roles {
			perms, _ := s.rbac.GetPermissions(role)
			for _, perm := range perms {
				if len(perm) >= 3 {
					// Format: resource:action
					permissions = append(permissions, fmt.Sprintf("%s:%s", perm[1], perm[2]))
				}
			}
		}
		// If no RBAC permissions, fall back to database permissions
		if len(permissions) == 0 {
			permissions = u.GetPermissions()
		}
	} else {
		permissions = u.GetPermissions()
	}

	// Generate tokens
	accessToken, err := s.jwtManager.GenerateToken(u.ID, u.Email, roles, permissions)
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
	// Check if refresh token is blacklisted (already used)
	blacklisted, _ := s.redis.Exists(ctx, fmt.Sprintf("blacklist:refresh:%s", refreshToken)).Result()
	if blacklisted > 0 {
		return nil, errors.New("refresh token has already been used")
	}

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

	// Check if account is still active
	if u.Status != user.StatusActive {
		return nil, errors.New("account is not active")
	}

	// Get current roles from RBAC
	var roles []string
	if s.rbac != nil {
		roles, _ = s.rbac.GetRoles(u.ID)
		if len(roles) == 0 {
			roles = u.GetRoleNames()
		}
	} else {
		roles = u.GetRoleNames()
	}

	// Generate new tokens
	accessToken, err := s.jwtManager.GenerateToken(u.ID, u.Email, roles, u.GetPermissions())
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	newRefreshToken, err := s.jwtManager.GenerateRefreshToken(u.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Blacklist the old refresh token (rotation) - prevent reuse
	s.redis.Set(ctx, fmt.Sprintf("blacklist:refresh:%s", refreshToken), "1", 7*24*time.Hour)

	// Publish token refreshed event
	event := events.NewEventBuilder("auth.token.refreshed").
		WithAggregateID(u.ID).
		WithAggregateType("user").
		WithUserID(u.ID).
		Build()

	s.eventBus.Publish(ctx, event)

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
	// Get user by verification token
	u, err := s.repository.GetUserByEmailVerifyToken(ctx, token)
	if err != nil {
		return errors.New("invalid or expired verification token")
	}

	// Check if already verified
	if u.EmailVerified {
		return errors.New("email already verified")
	}

	// Mark email as verified
	u.EmailVerified = true
	u.EmailVerifyToken = "" // Clear the token
	u.UpdatedAt = time.Now()

	if err := s.repository.UpdateUser(ctx, u); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	// Publish email verified event
	event := events.NewEventBuilder("user.email.verified").
		WithAggregateID(u.ID).
		WithAggregateType("user").
		WithUserID(u.ID).
		WithPayload("email", u.Email).
		Build()

	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Error("Failed to publish email verified event", "error", err)
	}

	s.logger.Info("Email verified successfully", "email", u.Email)
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

// trackFailedLogin tracks failed login attempts and locks account after 5 attempts
func (s *AuthService) trackFailedLogin(ctx context.Context, email, ipAddress string) {
	attemptKey := fmt.Sprintf("failed_attempts:%s", email)

	// Increment failed attempt count
	attempts, _ := s.redis.Incr(ctx, attemptKey).Result()

	// Set expiry on first attempt (reset after 15 minutes of no attempts)
	if attempts == 1 {
		s.redis.Expire(ctx, attemptKey, 15*time.Minute)
	}

	// Lock account after 5 failed attempts
	const maxAttempts = 5
	if attempts >= maxAttempts {
		lockKey := fmt.Sprintf("lockout:%s", email)
		s.redis.Set(ctx, lockKey, "1", 15*time.Minute)
		s.logger.Warn("Account locked due to too many failed login attempts",
			"email", email, "ipAddress", ipAddress, "attempts", attempts)

		// Publish account locked event
		event := events.NewEventBuilder("auth.account.locked").
			WithAggregateType("user").
			WithPayload("email", email).
			WithPayload("ipAddress", ipAddress).
			WithPayload("attempts", attempts).
			WithPayload("lockedUntil", time.Now().Add(15*time.Minute).Format(time.RFC3339)).
			Build()

		s.eventBus.Publish(ctx, event)
	}

	// Publish failed login event for audit
	event := events.NewEventBuilder("auth.login.failed").
		WithAggregateType("user").
		WithPayload("email", email).
		WithPayload("ipAddress", ipAddress).
		WithPayload("attempts", attempts).
		Build()

	s.eventBus.Publish(ctx, event)
}

// Session Management Methods

func (s *AuthService) GetUserSessions(ctx context.Context, userID string) ([]*user.Session, error) {
	sessions, err := s.repository.GetUserSessions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}

	// Filter out expired sessions
	activeSessions := make([]*user.Session, 0, len(sessions))
	now := time.Now()
	for _, session := range sessions {
		if session.ExpiresAt.After(now) {
			activeSessions = append(activeSessions, session)
		}
	}

	return activeSessions, nil
}

func (s *AuthService) RevokeSession(ctx context.Context, userID, sessionID string) error {
	// Get the session to verify ownership
	session, err := s.repository.GetSessionByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found")
	}

	// Verify the session belongs to the user
	if session.UserID != userID {
		return errors.New("unauthorized: session does not belong to user")
	}

	// Add token to blacklist
	if session.Token != "" {
		s.redis.Set(ctx, fmt.Sprintf("blacklist:%s", session.Token), "1", 24*time.Hour)
	}

	// Delete the session
	if err := s.repository.DeleteSessionByID(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	// Publish session revoked event
	event := events.NewEventBuilder("user.session.revoked").
		WithAggregateID(userID).
		WithAggregateType("user").
		WithUserID(userID).
		WithPayload("sessionId", sessionID).
		Build()

	s.eventBus.Publish(ctx, event)

	return nil
}

func (s *AuthService) RevokeAllSessions(ctx context.Context, userID string) error {
	// Get all user sessions to blacklist their tokens
	sessions, err := s.repository.GetUserSessions(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user sessions for blacklisting", "error", err)
	} else {
		// Blacklist all session tokens
		for _, session := range sessions {
			if session.Token != "" {
				s.redis.Set(ctx, fmt.Sprintf("blacklist:%s", session.Token), "1", 24*time.Hour)
			}
		}
	}

	// Delete all user sessions
	if err := s.repository.DeleteUserSessions(ctx, userID); err != nil {
		return fmt.Errorf("failed to revoke all sessions: %w", err)
	}

	// Publish all sessions revoked event
	event := events.NewEventBuilder("user.sessions.all.revoked").
		WithAggregateID(userID).
		WithAggregateType("user").
		WithUserID(userID).
		Build()

	s.eventBus.Publish(ctx, event)

	return nil
}

func (s *AuthService) ValidateSession(ctx context.Context, token string) (*user.Session, error) {
	// Check if token is blacklisted
	blacklisted, _ := s.redis.Exists(ctx, fmt.Sprintf("blacklist:%s", token)).Result()
	if blacklisted > 0 {
		return nil, errors.New("session has been revoked")
	}

	// Get session from database
	session, err := s.repository.GetSession(ctx, token)
	if err != nil {
		return nil, errors.New("invalid session")
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		// Delete expired session
		s.repository.DeleteSession(ctx, token)
		return nil, errors.New("session expired")
	}

	return session, nil
}

// RBAC Management Methods

func (s *AuthService) AssignRole(ctx context.Context, userID, role string) error {
	// Verify user exists
	if _, err := s.repository.GetUserByID(ctx, userID); err != nil {
		return fmt.Errorf("user not found")
	}

	// Assign role in RBAC
	if s.rbac != nil {
		if err := s.rbac.AddRole(userID, role); err != nil {
			return fmt.Errorf("failed to assign role: %w", err)
		}

		// Publish role assigned event
		event := events.NewEventBuilder("user.role.assigned").
			WithAggregateID(userID).
			WithAggregateType("user").
			WithUserID(userID).
			WithPayload("role", role).
			Build()

		s.eventBus.Publish(ctx, event)
	}

	return nil
}

func (s *AuthService) RemoveRole(ctx context.Context, userID, role string) error {
	// Verify user exists
	if _, err := s.repository.GetUserByID(ctx, userID); err != nil {
		return fmt.Errorf("user not found")
	}

	// Remove role in RBAC
	if s.rbac != nil {
		if err := s.rbac.RemoveRole(userID, role); err != nil {
			return fmt.Errorf("failed to remove role: %w", err)
		}

		// Publish role removed event
		event := events.NewEventBuilder("user.role.removed").
			WithAggregateID(userID).
			WithAggregateType("user").
			WithUserID(userID).
			WithPayload("role", role).
			Build()

		s.eventBus.Publish(ctx, event)
	}

	return nil
}

func (s *AuthService) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	// Verify user exists
	if _, err := s.repository.GetUserByID(ctx, userID); err != nil {
		return nil, fmt.Errorf("user not found")
	}

	if s.rbac != nil {
		return s.rbac.GetRoles(userID)
	}

	// Fall back to database roles
	u, _ := s.repository.GetUserByID(ctx, userID)
	if u != nil {
		return u.GetRoleNames(), nil
	}

	return []string{}, nil
}

func (s *AuthService) GetAllRoles(ctx context.Context) []string {
	if s.rbac != nil {
		return s.rbac.GetAllRoles()
	}

	// Return predefined roles
	return []string{
		authdomain.RoleSuperAdmin,
		authdomain.RoleAdmin,
		authdomain.RoleUser,
		authdomain.RoleViewer,
	}
}

func (s *AuthService) GetUsersForRole(ctx context.Context, role string) ([]string, error) {
	if s.rbac != nil {
		return s.rbac.GetUsersForRole(role)
	}
	return []string{}, nil
}

func (s *AuthService) CheckPermission(ctx context.Context, userID, resource, action string) (bool, error) {
	if s.rbac != nil {
		return s.rbac.CheckPermission(userID, resource, action)
	}

	// Fall back to checking database roles/permissions
	u, err := s.repository.GetUserByID(ctx, userID)
	if err != nil {
		return false, err
	}

	// Super admin has all permissions
	if u.HasRole("super_admin") || u.HasRole("admin") {
		return true, nil
	}

	// Check specific permission
	return u.HasPermission(resource, action), nil
}
