package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Provider represents an OAuth provider
type Provider interface {
	GetAuthURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*TokenResponse, error)
	GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error)
}

// UserInfo contains user information from OAuth provider
type UserInfo struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	Avatar      string `json:"avatar"`
	Provider    string `json:"provider"`
	ProviderID  string `json:"providerId"`
	AccessToken string `json:"-"`
}

// TokenResponse contains OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope,omitempty"`
}

// Config holds OAuth provider configuration
type Config struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
}

// GoogleProvider implements OAuth for Google
type GoogleProvider struct {
	config Config
}

// NewGoogleProvider creates a new Google OAuth provider
func NewGoogleProvider(cfg Config) *GoogleProvider {
	return &GoogleProvider{config: cfg}
}

// GetAuthURL returns the Google OAuth authorization URL
func (g *GoogleProvider) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", g.config.ClientID)
	params.Set("redirect_uri", g.config.RedirectURL)
	params.Set("response_type", "code")
	params.Set("scope", "openid email profile")
	params.Set("state", state)
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")

	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}

// ExchangeCode exchanges authorization code for tokens
func (g *GoogleProvider) ExchangeCode(ctx context.Context, code string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", g.config.ClientID)
	data.Set("client_secret", g.config.ClientSecret)
	data.Set("redirect_uri", g.config.RedirectURL)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// GetUserInfo retrieves user information from Google
func (g *GoogleProvider) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to get user info")
	}

	var googleUser struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Picture       string `json:"picture"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, err
	}

	return &UserInfo{
		ID:          googleUser.ID,
		Email:       googleUser.Email,
		Name:        googleUser.Name,
		FirstName:   googleUser.GivenName,
		LastName:    googleUser.FamilyName,
		Avatar:      googleUser.Picture,
		Provider:    "google",
		ProviderID:  googleUser.ID,
		AccessToken: accessToken,
	}, nil
}

// GitHubProvider implements OAuth for GitHub
type GitHubProvider struct {
	config Config
}

// NewGitHubProvider creates a new GitHub OAuth provider
func NewGitHubProvider(cfg Config) *GitHubProvider {
	return &GitHubProvider{config: cfg}
}

// GetAuthURL returns the GitHub OAuth authorization URL
func (gh *GitHubProvider) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", gh.config.ClientID)
	params.Set("redirect_uri", gh.config.RedirectURL)
	params.Set("scope", "user:email read:user")
	params.Set("state", state)

	return "https://github.com/login/oauth/authorize?" + params.Encode()
}

// ExchangeCode exchanges authorization code for tokens
func (gh *GitHubProvider) ExchangeCode(ctx context.Context, code string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", gh.config.ClientID)
	data.Set("client_secret", gh.config.ClientSecret)
	data.Set("redirect_uri", gh.config.RedirectURL)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// GetUserInfo retrieves user information from GitHub
func (gh *GitHubProvider) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	// Get user profile
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to get user info")
	}

	var githubUser struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&githubUser); err != nil {
		return nil, err
	}

	// If email is not public, fetch from emails endpoint
	email := githubUser.Email
	if email == "" {
		email, _ = gh.getPrimaryEmail(ctx, accessToken)
	}

	// Parse name into first/last
	firstName, lastName := parseName(githubUser.Name)
	if firstName == "" {
		firstName = githubUser.Login
	}

	return &UserInfo{
		ID:          fmt.Sprintf("%d", githubUser.ID),
		Email:       email,
		Name:        githubUser.Name,
		FirstName:   firstName,
		LastName:    lastName,
		Avatar:      githubUser.AvatarURL,
		Provider:    "github",
		ProviderID:  fmt.Sprintf("%d", githubUser.ID),
		AccessToken: accessToken,
	}, nil
}

// getPrimaryEmail fetches the primary email from GitHub
func (gh *GitHubProvider) getPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	// Return first verified email if no primary found
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}

	return "", errors.New("no verified email found")
}

// parseName splits a full name into first and last name
func parseName(fullName string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(fullName), " ", 2)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

// ProviderFactory creates OAuth providers
type ProviderFactory struct {
	configs map[string]Config
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory(configs map[string]Config) *ProviderFactory {
	return &ProviderFactory{configs: configs}
}

// GetProvider returns the OAuth provider for the given name
func (f *ProviderFactory) GetProvider(name string) (Provider, error) {
	cfg, exists := f.configs[name]
	if !exists {
		return nil, fmt.Errorf("unknown OAuth provider: %s", name)
	}

	switch name {
	case "google":
		return NewGoogleProvider(cfg), nil
	case "github":
		return NewGitHubProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported OAuth provider: %s", name)
	}
}
