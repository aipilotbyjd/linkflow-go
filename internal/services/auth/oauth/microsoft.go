package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	microsoftAuthURL  = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	microsoftTokenURL = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	microsoftUserURL  = "https://graph.microsoft.com/v1.0/me"
)

// MicrosoftProvider implements OAuth2 for Microsoft
type MicrosoftProvider struct {
	config ProviderConfig
}

// NewMicrosoftProvider creates a new Microsoft OAuth provider
func NewMicrosoftProvider(config ProviderConfig) *MicrosoftProvider {
	if len(config.Scopes) == 0 {
		config.Scopes = []string{"openid", "email", "profile", "User.Read"}
	}
	return &MicrosoftProvider{config: config}
}

func (p *MicrosoftProvider) GetName() string {
	return "microsoft"
}

func (p *MicrosoftProvider) GetAuthURL(state, redirectURI string) string {
	params := url.Values{
		"client_id":     {p.config.ClientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {strings.Join(p.config.Scopes, " ")},
		"state":         {state},
		"response_mode": {"query"},
	}
	return microsoftAuthURL + "?" + params.Encode()
}

func (p *MicrosoftProvider) ExchangeCode(ctx context.Context, code, redirectURI string) (*TokenResponse, error) {
	data := url.Values{
		"client_id":     {p.config.ClientID},
		"client_secret": {p.config.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
		"scope":         {strings.Join(p.config.Scopes, " ")},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", microsoftTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	return &token, nil
}

func (p *MicrosoftProvider) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", microsoftUserURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var msUser struct {
		ID                string `json:"id"`
		DisplayName       string `json:"displayName"`
		GivenName         string `json:"givenName"`
		Surname           string `json:"surname"`
		Mail              string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&msUser); err != nil {
		return nil, err
	}

	email := msUser.Mail
	if email == "" {
		email = msUser.UserPrincipalName
	}

	return &UserInfo{
		ID:        msUser.ID,
		Email:     email,
		Name:      msUser.DisplayName,
		FirstName: msUser.GivenName,
		LastName:  msUser.Surname,
		Verified:  true,
	}, nil
}

func (p *MicrosoftProvider) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	data := url.Values{
		"client_id":     {p.config.ClientID},
		"client_secret": {p.config.ClientSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
		"scope":         {strings.Join(p.config.Scopes, " ")},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", microsoftTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token refresh failed: %s", string(body))
	}

	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	return &token, nil
}
