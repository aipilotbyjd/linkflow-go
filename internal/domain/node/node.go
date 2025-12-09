package node

import (
	"errors"
	"time"
)

type NodeType struct {
	ID          string     `json:"id" gorm:"primaryKey"`
	Type        string     `json:"type" gorm:"uniqueIndex;not null"`
	Name        string     `json:"name" gorm:"not null"`
	Description string     `json:"description"`
	Category    string     `json:"category"`
	Icon        string     `json:"icon"`
	Color       string     `json:"color"`
	Version     string     `json:"version"`
	Author      string     `json:"author"`
	Schema      NodeSchema `json:"schema" gorm:"serializer:json"`
	Config      NodeConfig `json:"config" gorm:"serializer:json"`
	Status      string     `json:"status" gorm:"default:'active'"`
	IsBuiltin   bool       `json:"isBuiltin" gorm:"default:false"`
	IsPublic    bool       `json:"isPublic" gorm:"default:false"`
	Downloads   int        `json:"downloads" gorm:"default:0"`
	Rating      float32    `json:"rating" gorm:"default:0"`
	Tags        []string   `json:"tags" gorm:"serializer:json"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type NodeSchema struct {
	Inputs     []SchemaField          `json:"inputs"`
	Outputs    []SchemaField          `json:"outputs"`
	Properties map[string]interface{} `json:"properties"`
}

type SchemaField struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"` // string, number, boolean, json, array, file, credential, code, select, date
	Label       string      `json:"label"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default"`
	Placeholder string      `json:"placeholder"`
	Options     []string    `json:"options"` // For select type
	Multiple    bool        `json:"multiple"`
	Min         interface{} `json:"min"`
	Max         interface{} `json:"max"`
	Pattern     string      `json:"pattern"`
	Help        string      `json:"help"`
	Language    string      `json:"language"` // For code type
}

type NodeConfig struct {
	Retryable        bool                   `json:"retryable"`
	MaxRetries       int                    `json:"maxRetries"`
	RetryDelay       int                    `json:"retryDelay"`
	Timeout          int                    `json:"timeout"`
	RateLimit        int                    `json:"rateLimit"`
	Cacheable        bool                   `json:"cacheable"`
	CacheDuration    int                    `json:"cacheDuration"`
	RequiresAuth     bool                   `json:"requiresAuth"`
	AuthType         string                 `json:"authType"`
	SupportsBatching bool                   `json:"supportsBatching"`
	MaxBatchSize     int                    `json:"maxBatchSize"`
	CustomConfig     map[string]interface{} `json:"customConfig"`
}

type NodeInstance struct {
	ID         string                 `json:"id" gorm:"primaryKey"`
	WorkflowID string                 `json:"workflowId" gorm:"not null;index"`
	NodeType   string                 `json:"nodeType" gorm:"not null"`
	Name       string                 `json:"name"`
	Position   Position               `json:"position" gorm:"serializer:json"`
	Parameters map[string]interface{} `json:"parameters" gorm:"serializer:json"`
	Config     NodeInstanceConfig     `json:"config" gorm:"serializer:json"`
	CreatedAt  time.Time              `json:"createdAt"`
	UpdatedAt  time.Time              `json:"updatedAt"`
}

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type NodeInstanceConfig struct {
	Disabled         bool `json:"disabled"`
	ContinueOnFail   bool `json:"continueOnFail"`
	RetryOnFail      bool `json:"retryOnFail"`
	MaxRetries       int  `json:"maxRetries"`
	WaitBetweenTries int  `json:"waitBetweenTries"`
	Timeout          int  `json:"timeout"`
	Notes            string `json:"notes"`
}

type NodeExecution struct {
	ID           string                 `json:"id" gorm:"primaryKey"`
	ExecutionID  string                 `json:"executionId" gorm:"not null;index"`
	NodeID       string                 `json:"nodeId" gorm:"not null"`
	NodeType     string                 `json:"nodeType"`
	Status       string                 `json:"status"`
	StartedAt    time.Time              `json:"startedAt"`
	FinishedAt   *time.Time             `json:"finishedAt"`
	ExecutionTime int64                 `json:"executionTime"`
	InputData    map[string]interface{} `json:"inputData" gorm:"serializer:json"`
	OutputData   map[string]interface{} `json:"outputData" gorm:"serializer:json"`
	Error        string                 `json:"error"`
	RetryCount   int                    `json:"retryCount"`
	Metadata     map[string]interface{} `json:"metadata" gorm:"serializer:json"`
	CreatedAt    time.Time              `json:"createdAt"`
}

// Node categories
const (
	CategoryTrigger   = "trigger"
	CategoryAction    = "action"
	CategoryTransform = "transform"
	CategoryControl   = "control"
	CategoryStorage   = "storage"
	CategoryAnalytics = "analytics"
	CategoryAI        = "ai"
	CategoryCustom    = "custom"
)

// Node status
const (
	StatusActive     = "active"
	StatusInactive   = "inactive"
	StatusDeprecated = "deprecated"
	StatusBeta       = "beta"
)

// Execution status
const (
	ExecutionPending   = "pending"
	ExecutionRunning   = "running"
	ExecutionCompleted = "completed"
	ExecutionFailed    = "failed"
	ExecutionSkipped   = "skipped"
)

// Field types
const (
	FieldTypeString     = "string"
	FieldTypeNumber     = "number"
	FieldTypeBoolean    = "boolean"
	FieldTypeJSON       = "json"
	FieldTypeArray      = "array"
	FieldTypeFile       = "file"
	FieldTypeCredential = "credential"
	FieldTypeCode       = "code"
	FieldTypeSelect     = "select"
	FieldTypeDate       = "date"
	FieldTypeText       = "text"
	FieldTypeAny        = "any"
)

// Validate validates the node type
func (n *NodeType) Validate() error {
	if n.Type == "" {
		return errors.New("node type is required")
	}
	if n.Name == "" {
		return errors.New("node name is required")
	}
	if n.Category == "" {
		return errors.New("node category is required")
	}
	
	// Validate schema
	for _, field := range n.Schema.Inputs {
		if err := validateSchemaField(field); err != nil {
			return err
		}
	}
	
	for _, field := range n.Schema.Outputs {
		if err := validateSchemaField(field); err != nil {
			return err
		}
	}
	
	return nil
}

func validateSchemaField(field SchemaField) error {
	if field.Name == "" {
		return errors.New("field name is required")
	}
	if field.Type == "" {
		return errors.New("field type is required")
	}
	
	// Validate field type
	validTypes := []string{
		FieldTypeString, FieldTypeNumber, FieldTypeBoolean,
		FieldTypeJSON, FieldTypeArray, FieldTypeFile,
		FieldTypeCredential, FieldTypeCode, FieldTypeSelect,
		FieldTypeDate, FieldTypeText, FieldTypeAny,
	}
	
	valid := false
	for _, t := range validTypes {
		if field.Type == t {
			valid = true
			break
		}
	}
	
	if !valid {
		return errors.New("invalid field type: " + field.Type)
	}
	
	// Validate select fields have options
	if field.Type == FieldTypeSelect && len(field.Options) == 0 {
		return errors.New("select field must have options")
	}
	
	return nil
}

// GetInputField returns an input field by name
func (s *NodeSchema) GetInputField(name string) *SchemaField {
	for _, field := range s.Inputs {
		if field.Name == name {
			return &field
		}
	}
	return nil
}

// GetOutputField returns an output field by name
func (s *NodeSchema) GetOutputField(name string) *SchemaField {
	for _, field := range s.Outputs {
		if field.Name == name {
			return &field
		}
	}
	return nil
}

// ValidateParameters validates node parameters against schema
func (n *NodeType) ValidateParameters(parameters map[string]interface{}) error {
	// Check required fields
	for _, field := range n.Schema.Inputs {
		if field.Required {
			if _, ok := parameters[field.Name]; !ok {
				return errors.New("required field missing: " + field.Name)
			}
		}
	}
	
	// Validate field types
	for name, value := range parameters {
		field := n.Schema.GetInputField(name)
		if field == nil {
			// Allow extra parameters for flexibility
			continue
		}
		
		if err := validateFieldValue(field, value); err != nil {
			return err
		}
	}
	
	return nil
}

func validateFieldValue(field *SchemaField, value interface{}) error {
	// Type validation would go here
	// This is simplified - in production would have comprehensive type checking
	
	if field.Type == FieldTypeSelect {
		strValue, ok := value.(string)
		if !ok {
			return errors.New("select field must be a string")
		}
		
		// Check if value is in options
		valid := false
		for _, option := range field.Options {
			if option == strValue {
				valid = true
				break
			}
		}
		
		if !valid {
			return errors.New("invalid option for select field: " + field.Name)
		}
	}
	
	return nil
}
