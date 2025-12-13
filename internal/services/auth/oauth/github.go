package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	githubAuthURL  = "https://github.com/login/oauth/authorize"
	githubTokenURL = "https://github.com/login/oauth/access_token"
	githubUserURL  = "https://api.github.com/user"
	githubEmailURL = "https://api.github.com/user/emails"
)

// GitHubProvider implements OAuth2 for GitHub
type GitHubProvider struct {
	config ProviderConfig
}

// NewGitHubProvider creates a new GitHub OAuth provider
func NewGitHubProvider(config ProviderConfig) *GitHubProvider {
	if len(config.Scopes) == 0 {
		config.Scopes = []string{"user:email", "read:user"}
	}
	return &GitHubProvider{config: config}
}

func (p *GitHubProvider) GetName() string {
	return "github"
}

func (p *GitHubProvider) GetAuthURL(state, redirectURI string) string {
	params := url.Values{
		"client_id":    {p.config.ClientID},
		"redirect_uri": {redirectURI},
		"scope":        {strings.Join(p.config.Scopes, " ")},
		"state":        {state},
	}
	return githubAuthURL + "?" + params.Encode()
}

func (p *GitHubProvider) ExchangeCode(ctx context.Context, code, redirectURI string) (*TokenResponse, error) {
	data := url.Values{
		"client_id":     {p.config.ClientID},
		"client_secret": {p.config.ClientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", githubTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Error != "" {
		return nil, fmt.Errorf("token exchange failed: %s", result.Error)
	}

	return &TokenResponse{
		AccessToken: result.AccessToken,
		TokenType:   result.TokenType,
		Scope:       result.Scope,
		ExpiresAt:   time.Now().Add(365 * 24 * time.Hour), // GitHub tokens don't expire
	}, nil
}

func (p *GitHubProvider) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", githubUserURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var githubUser struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&githubUser); err != nil {
		return nil, err
	}

	// Get primary email if not public
	email := githubUser.Email
	if email == "" {
		email, _ = p.getPrimaryEmail(ctx, accessToken)
	}

	firstName, lastName := splitName(githubUser.Name)

	return &UserInfo{
		ID:        fmt.Sprintf("%d", githubUser.ID),
		Email:     email,
		Name:      githubUser.Name,
		FirstName: firstName,
		LastName:  lastName,
		Picture:   githubUser.AvatarURL,
		Verified:  true,
	}, nil
}

func (p *GitHubProvider) getPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", githubEmailURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
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

	if len(emails) > 0 {
		return emails[0].Email, nil
	}

	return "", nil
}

func (p *GitHubProvider) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	// GitHub tokens don't expire and don't support refresh
	return nil, fmt.Errorf("GitHub does not support token refresh")
}

func splitName(name string) (string, string) {
	parts := strings.SplitN(name, " ", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return name, ""
}
