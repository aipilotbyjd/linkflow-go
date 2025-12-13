package variable

import (
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// Variable types
const (
	TypeString = "string"
	TypeNumber = "number"
	TypeBoolean = "boolean"
	TypeSecret = "secret"
)

var (
	ErrVariableNotFound    = errors.New("variable not found")
	ErrInvalidVariableName = errors.New("invalid variable name")
	ErrVariableExists      = errors.New("variable already exists")
	ErrInvalidVariableType = errors.New("invalid variable type")
)

// Variable represents a global variable available to all workflows
type Variable struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Key         string    `json:"key" gorm:"uniqueIndex;not null"`
	Value       string    `json:"value" gorm:"not null"` // Stored encrypted for secrets
	Type        string    `json:"type" gorm:"not null;default:'string'"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// NewVariable creates a new variable
func NewVariable(key, value, varType string) *Variable {
	if varType == "" {
		varType = TypeString
	}
	return &Variable{
		ID:        uuid.New().String(),
		Key:       key,
		Value:     value,
		Type:      varType,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Validate validates the variable
func (v *Variable) Validate() error {
	if err := ValidateKey(v.Key); err != nil {
		return err
	}

	validTypes := map[string]bool{
		TypeString:  true,
		TypeNumber:  true,
		TypeBoolean: true,
		TypeSecret:  true,
	}
	if !validTypes[v.Type] {
		return ErrInvalidVariableType
	}

	return nil
}

// IsSecret returns true if this is a secret variable
func (v *Variable) IsSecret() bool {
	return v.Type == TypeSecret
}

// MaskedValue returns the value with secrets masked
func (v *Variable) MaskedValue() string {
	if v.IsSecret() {
		return "••••••••"
	}
	return v.Value
}

// ValidateKey validates a variable key
func ValidateKey(key string) error {
	if key == "" {
		return ErrInvalidVariableName
	}

	// Must start with letter, can contain letters, numbers, underscores
	validKey := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)
	if !validKey.MatchString(key) {
		return ErrInvalidVariableName
	}

	// Max length 50 characters
	if len(key) > 50 {
		return ErrInvalidVariableName
	}

	return nil
}

// VariableResponse is the API response (masks secrets)
type VariableResponse struct {
	ID          string    `json:"id"`
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ToResponse converts to API response (masks secrets)
func (v *Variable) ToResponse() VariableResponse {
	return VariableResponse{
		ID:          v.ID,
		Key:         v.Key,
		Value:       v.MaskedValue(),
		Type:        v.Type,
		Description: v.Description,
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
	}
}
