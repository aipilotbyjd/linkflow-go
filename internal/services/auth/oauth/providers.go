package oauth

import (
	"context"
	"errors"
	"time"
)

var (
	ErrProviderNotFound = errors.New("OAuth provider not found")
	ErrInvalidCode      = errors.New("invalid authorization code")
	ErrTokenExpired     = errors.New("token has expired")
)

// Provider represents an OAuth2 provider
type Provider interface {
	GetName() string
	GetAuthURL(state, redirectURI string) string
	ExchangeCode(ctx context.Context, code, redirectURI string) (*TokenResponse, error)
	GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error)
}

// TokenResponse represents OAuth token response
type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	Scope        string    `json:"scope"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// UserInfo represents user information from OAuth provider
type UserInfo struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Picture   string `json:"picture"`
	Verified  bool   `json:"verified"`
}

// ProviderConfig holds OAuth provider configuration
type ProviderConfig struct {
	ClientID     string
	ClientSecret string
	Scopes       []string
}
