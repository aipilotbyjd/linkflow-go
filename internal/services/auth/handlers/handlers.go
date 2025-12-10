package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/services/auth/service"
	"github.com/linkflow-go/pkg/logger"
)

type AuthHandlers struct {
	service *service.AuthService
	logger  logger.Logger
}

func NewAuthHandlers(service *service.AuthService, logger logger.Logger) *AuthHandlers {
	return &AuthHandlers{
		service: service,
		logger:  logger,
	}
}

type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=8"`
}

func (h *AuthHandlers) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.service.Register(c.Request.Context(), req.Email, req.Password, req.FirstName, req.LastName)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
			return
		}
		h.logger.Error("Failed to register user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user": user,
		"message": "Registration successful. Please verify your email.",
	})
}

func (h *AuthHandlers) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, user, err := h.service.Login(c.Request.Context(), req.Email, req.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		if strings.Contains(err.Error(), "invalid credentials") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}
		h.logger.Error("Failed to login", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to login"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accessToken":  tokens.AccessToken,
		"refreshToken": tokens.RefreshToken,
		"expiresIn":    tokens.ExpiresIn,
		"user":         user,
	})
}

func (h *AuthHandlers) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, err := h.service.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accessToken":  tokens.AccessToken,
		"refreshToken": tokens.RefreshToken,
		"expiresIn":    tokens.ExpiresIn,
	})
}

func (h *AuthHandlers) Logout(c *gin.Context) {
	userID := c.GetString("userId")
	token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")

	if err := h.service.Logout(c.Request.Context(), userID, token); err != nil {
		h.logger.Error("Failed to logout", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func (h *AuthHandlers) GetCurrentUser(c *gin.Context) {
	userID := c.GetString("userId")

	user, err := h.service.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user})
}

func (h *AuthHandlers) UpdateProfile(c *gin.Context) {
	userID := c.GetString("userId")
	
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.service.UpdateProfile(c.Request.Context(), userID, req)
	if err != nil {
		h.logger.Error("Failed to update profile", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user})
}

func (h *AuthHandlers) ChangePassword(c *gin.Context) {
	userID := c.GetString("userId")
	
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		if strings.Contains(err.Error(), "incorrect") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Incorrect old password"})
			return
		}
		h.logger.Error("Failed to change password", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

func (h *AuthHandlers) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification token required"})
		return
	}

	if err := h.service.VerifyEmail(c.Request.Context(), token); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired verification token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email verified successfully"})
}

func (h *AuthHandlers) ForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.ForgotPassword(c.Request.Context(), req.Email); err != nil {
		// Don't reveal if email exists or not
		h.logger.Error("Failed to process forgot password", "error", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "If the email exists, a password reset link has been sent",
	})
}

func (h *AuthHandlers) ResetPassword(c *gin.Context) {
	var req struct {
		Token    string `json:"token" binding:"required"`
		Password string `json:"password" binding:"required,min=8"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.ResetPassword(c.Request.Context(), req.Token, req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}

func (h *AuthHandlers) OAuthLogin(c *gin.Context) {
	provider := c.Param("provider")
	
	authURL, err := h.service.GetOAuthURL(provider)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OAuth provider"})
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

func (h *AuthHandlers) OAuthCallback(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")
	
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization code required"})
		return
	}

	tokens, user, err := h.service.HandleOAuthCallback(c.Request.Context(), provider, code)
	if err != nil {
		h.logger.Error("OAuth callback failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OAuth authentication failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accessToken":  tokens.AccessToken,
		"refreshToken": tokens.RefreshToken,
		"expiresIn":    tokens.ExpiresIn,
		"user":         user,
	})
}

func (h *AuthHandlers) Setup2FA(c *gin.Context) {
	userID := c.GetString("userId")
	
	secret, qrCode, err := h.service.Setup2FA(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to setup 2FA", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to setup 2FA"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"secret": secret,
		"qrCode": qrCode,
	})
}

func (h *AuthHandlers) Verify2FA(c *gin.Context) {
	userID := c.GetString("userId")
	
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.Verify2FA(c.Request.Context(), userID, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid verification code"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "2FA enabled successfully"})
}

func (h *AuthHandlers) Disable2FA(c *gin.Context) {
	userID := c.GetString("userId")
	
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.Disable2FA(c.Request.Context(), userID, req.Password); err != nil {
		if strings.Contains(err.Error(), "incorrect") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Incorrect password"})
			return
		}
		h.logger.Error("Failed to disable 2FA", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable 2FA"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "2FA disabled successfully"})
}

// Session management handlers
func (h *AuthHandlers) GetSessions(c *gin.Context) {
	userID := c.GetString("userId")
	
	sessions, err := h.service.GetUserSessions(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user sessions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get sessions"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

func (h *AuthHandlers) RevokeSession(c *gin.Context) {
	userID := c.GetString("userId")
	sessionID := c.Param("sessionId")
	
	if err := h.service.RevokeSession(c.Request.Context(), userID, sessionID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}
		if strings.Contains(err.Error(), "unauthorized") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot revoke this session"})
			return
		}
		h.logger.Error("Failed to revoke session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke session"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Session revoked successfully"})
}

func (h *AuthHandlers) RevokeAllSessions(c *gin.Context) {
	userID := c.GetString("userId")
	
	if err := h.service.RevokeAllSessions(c.Request.Context(), userID); err != nil {
		h.logger.Error("Failed to revoke all sessions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke sessions"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "All sessions revoked successfully"})
}

func (h *AuthHandlers) ValidateToken(c *gin.Context) {
	token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token required"})
		return
	}
	
	session, err := h.service.ValidateSession(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"valid": true,
		"session": session,
	})
}

// RBAC handlers
func (h *AuthHandlers) AssignRole(c *gin.Context) {
	userID := c.Param("userId")
	
	var req struct {
		Role string `json:"role" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if err := h.service.AssignRole(c.Request.Context(), userID, req.Role); err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		h.logger.Error("Failed to assign role", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign role"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Role assigned successfully"})
}

func (h *AuthHandlers) RemoveRole(c *gin.Context) {
	userID := c.Param("userId")
	role := c.Param("role")
	
	if err := h.service.RemoveRole(c.Request.Context(), userID, role); err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		h.logger.Error("Failed to remove role", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove role"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Role removed successfully"})
}

func (h *AuthHandlers) GetUserRoles(c *gin.Context) {
	userID := c.Param("userId")
	
	roles, err := h.service.GetUserRoles(c.Request.Context(), userID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		h.logger.Error("Failed to get user roles", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user roles"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"roles": roles})
}

func (h *AuthHandlers) GetAllRoles(c *gin.Context) {
	roles := h.service.GetAllRoles(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"roles": roles})
}

func (h *AuthHandlers) GetUsersForRole(c *gin.Context) {
	role := c.Param("role")
	
	users, err := h.service.GetUsersForRole(c.Request.Context(), role)
	if err != nil {
		h.logger.Error("Failed to get users for role", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get users"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *AuthHandlers) CheckPermission(c *gin.Context) {
	var req struct {
		UserID   string `json:"userId" binding:"required"`
		Resource string `json:"resource" binding:"required"`
		Action   string `json:"action" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	allowed, err := h.service.CheckPermission(c.Request.Context(), req.UserID, req.Resource, req.Action)
	if err != nil {
		h.logger.Error("Failed to check permission", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permission"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"allowed": allowed,
		"userId": req.UserID,
		"resource": req.Resource,
		"action": req.Action,
	})
}

// Health check handlers
func (h *AuthHandlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"service": "auth-service",
	})
}

func (h *AuthHandlers) Ready(c *gin.Context) {
	// Check if service is ready (database, redis, etc.)
	if err := h.service.CheckReadiness(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"error": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"service": "auth-service",
	})
}
