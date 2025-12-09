package credential

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Credential struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	Name        string                 `json:"name" gorm:"not null"`
	Type        string                 `json:"type" gorm:"not null"`
	UserID      string                 `json:"userId" gorm:"not null;index"`
	TeamID      string                 `json:"teamId" gorm:"index"`
	Data        map[string]interface{} `json:"data" gorm:"serializer:json"`
	Description string                 `json:"description"`
	Tags        []string               `json:"tags" gorm:"serializer:json"`
	IsShared    bool                   `json:"isShared" gorm:"default:false"`
	IsActive    bool                   `json:"isActive" gorm:"default:true"`
	LastUsedAt  *time.Time             `json:"lastUsedAt"`
	ExpiresAt   *time.Time             `json:"expiresAt"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
}

type CredentialType struct {
	Type        string        `json:"type"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Icon        string        `json:"icon"`
	Fields      []FieldConfig `json:"fields"`
	AuthFlow    string        `json:"authFlow"`
}

type FieldConfig struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Label       string      `json:"label"`
	Required    bool        `json:"required"`
	Placeholder string      `json:"placeholder"`
	Help        string      `json:"help"`
	Sensitive   bool        `json:"sensitive"`
	Default     interface{} `json:"default"`
	Options     []string    `json:"options"`
}

type CredentialUsage struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	CredentialID string    `json:"credentialId" gorm:"not null;index"`
	WorkflowID   string    `json:"workflowId" gorm:"not null;index"`
	NodeID       string    `json:"nodeId"`
	ExecutionID  string    `json:"executionId"`
	UsedAt       time.Time `json:"usedAt"`
	Success      bool      `json:"success"`
	Error        string    `json:"error"`
}

// Credential types
const (
	TypeAPIKey      = "apiKey"
	TypeOAuth2      = "oauth2"
	TypeBasicAuth   = "basicAuth"
	TypeBearerToken = "bearerToken"
	TypeSSHKey      = "sshKey"
	TypeDatabase    = "database"
	TypeAWS         = "aws"
	TypeGCP         = "gcp"
	TypeAzure       = "azure"
	TypeCustom      = "custom"
)

// OAuth2 auth flows
const (
	AuthFlowClientCredentials = "client_credentials"
	AuthFlowAuthorizationCode = "authorization_code"
	AuthFlowPassword          = "password"
	AuthFlowImplicit          = "implicit"
)

// NewCredential creates a new credential
func NewCredential(name, credType, userID string) *Credential {
	return &Credential{
		ID:        uuid.New().String(),
		Name:      name,
		Type:      credType,
		UserID:    userID,
		Data:      make(map[string]interface{}),
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Validate validates the credential
func (c *Credential) Validate() error {
	if c.Name == "" {
		return errors.New("credential name is required")
	}
	if c.Type == "" {
		return errors.New("credential type is required")
	}
	if c.UserID == "" {
		return errors.New("user ID is required")
	}
	
	// Validate based on type
	switch c.Type {
	case TypeAPIKey:
		return c.validateAPIKey()
	case TypeOAuth2:
		return c.validateOAuth2()
	case TypeBasicAuth:
		return c.validateBasicAuth()
	case TypeSSHKey:
		return c.validateSSHKey()
	case TypeDatabase:
		return c.validateDatabase()
	}
	
	return nil
}

func (c *Credential) validateAPIKey() error {
	if _, ok := c.Data["apiKey"]; !ok {
		return errors.New("API key is required")
	}
	return nil
}

func (c *Credential) validateOAuth2() error {
	if _, ok := c.Data["clientId"]; !ok {
		return errors.New("client ID is required")
	}
	if _, ok := c.Data["clientSecret"]; !ok {
		return errors.New("client secret is required")
	}
	return nil
}

func (c *Credential) validateBasicAuth() error {
	if _, ok := c.Data["username"]; !ok {
		return errors.New("username is required")
	}
	if _, ok := c.Data["password"]; !ok {
		return errors.New("password is required")
	}
	return nil
}

func (c *Credential) validateSSHKey() error {
	if _, ok := c.Data["privateKey"]; !ok {
		return errors.New("private key is required")
	}
	return nil
}

func (c *Credential) validateDatabase() error {
	// Either connection string or individual fields
	if _, ok := c.Data["connectionString"]; ok {
		return nil
	}
	
	if _, ok := c.Data["host"]; !ok {
		return errors.New("host is required")
	}
	if _, ok := c.Data["database"]; !ok {
		return errors.New("database name is required")
	}
	if _, ok := c.Data["username"]; !ok {
		return errors.New("username is required")
	}
	if _, ok := c.Data["password"]; !ok {
		return errors.New("password is required")
	}
	
	return nil
}

// IsExpired checks if the credential has expired
func (c *Credential) IsExpired() bool {
	if c.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*c.ExpiresAt)
}

// RecordUsage records the usage of a credential
func (c *Credential) RecordUsage() {
	now := time.Now()
	c.LastUsedAt = &now
}

// GetCredentialTypes returns all supported credential types
func GetCredentialTypes() []CredentialType {
	return []CredentialType{
		{
			Type:        TypeAPIKey,
			Name:        "API Key",
			Description: "Simple API key authentication",
			Icon:        "key",
			Fields: []FieldConfig{
				{
					Name:        "apiKey",
					Type:        "string",
					Label:       "API Key",
					Required:    true,
					Sensitive:   true,
					Placeholder: "Enter your API key",
				},
				{
					Name:        "headerName",
					Type:        "string",
					Label:       "Header Name",
					Required:    false,
					Default:     "X-API-Key",
					Placeholder: "X-API-Key",
				},
			},
		},
		{
			Type:        TypeOAuth2,
			Name:        "OAuth2",
			Description: "OAuth2 authentication",
			Icon:        "shield",
			Fields: []FieldConfig{
				{
					Name:        "authFlow",
					Type:        "select",
					Label:       "Auth Flow",
					Required:    true,
					Default:     AuthFlowClientCredentials,
					Options:     []string{AuthFlowClientCredentials, AuthFlowAuthorizationCode, AuthFlowPassword},
				},
				{
					Name:        "clientId",
					Type:        "string",
					Label:       "Client ID",
					Required:    true,
					Placeholder: "Your OAuth2 client ID",
				},
				{
					Name:        "clientSecret",
					Type:        "string",
					Label:       "Client Secret",
					Required:    true,
					Sensitive:   true,
					Placeholder: "Your OAuth2 client secret",
				},
				{
					Name:        "tokenUrl",
					Type:        "string",
					Label:       "Token URL",
					Required:    true,
					Placeholder: "https://oauth.provider.com/token",
				},
				{
					Name:        "authUrl",
					Type:        "string",
					Label:       "Authorization URL",
					Required:    false,
					Placeholder: "https://oauth.provider.com/authorize",
				},
				{
					Name:        "scope",
					Type:        "string",
					Label:       "Scope",
					Required:    false,
					Placeholder: "read write",
				},
			},
		},
		{
			Type:        TypeBasicAuth,
			Name:        "Basic Authentication",
			Description: "Username and password authentication",
			Icon:        "user",
			Fields: []FieldConfig{
				{
					Name:        "username",
					Type:        "string",
					Label:       "Username",
					Required:    true,
					Placeholder: "Enter username",
				},
				{
					Name:        "password",
					Type:        "string",
					Label:       "Password",
					Required:    true,
					Sensitive:   true,
					Placeholder: "Enter password",
				},
			},
		},
		{
			Type:        TypeDatabase,
			Name:        "Database",
			Description: "Database connection credentials",
			Icon:        "database",
			Fields: []FieldConfig{
				{
					Name:        "type",
					Type:        "select",
					Label:       "Database Type",
					Required:    true,
					Options:     []string{"postgres", "mysql", "mongodb", "redis", "mssql"},
				},
				{
					Name:        "connectionString",
					Type:        "string",
					Label:       "Connection String",
					Required:    false,
					Sensitive:   true,
					Placeholder: "postgres://user:pass@host:port/db",
					Help:        "Provide either connection string or individual fields",
				},
				{
					Name:        "host",
					Type:        "string",
					Label:       "Host",
					Required:    false,
					Placeholder: "localhost",
				},
				{
					Name:        "port",
					Type:        "number",
					Label:       "Port",
					Required:    false,
					Default:     5432,
				},
				{
					Name:        "database",
					Type:        "string",
					Label:       "Database Name",
					Required:    false,
					Placeholder: "mydb",
				},
				{
					Name:        "username",
					Type:        "string",
					Label:       "Username",
					Required:    false,
					Placeholder: "dbuser",
				},
				{
					Name:        "password",
					Type:        "string",
					Label:       "Password",
					Required:    false,
					Sensitive:   true,
					Placeholder: "Enter password",
				},
				{
					Name:        "ssl",
					Type:        "boolean",
					Label:       "Use SSL",
					Required:    false,
					Default:     false,
				},
			},
		},
	}
}
