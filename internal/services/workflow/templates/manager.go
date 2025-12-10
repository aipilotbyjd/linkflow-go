package templates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/pkg/database"
	"github.com/linkflow-go/pkg/logger"
	"gorm.io/gorm"
)

var (
	ErrTemplateNotFound     = errors.New("template not found")
	ErrInvalidTemplate      = errors.New("invalid template")
	ErrDuplicateTemplate    = errors.New("template already exists")
	ErrVariableRequired     = errors.New("required variable not provided")
	ErrInvalidVariableType  = errors.New("invalid variable type")
)

// Variable types
const (
	VariableTypeString  = "string"
	VariableTypeNumber  = "number"
	VariableTypeBoolean = "boolean"
	VariableTypeJSON    = "json"
	VariableTypeSecret  = "secret"
)

// Template categories
const (
	CategoryDataPipeline  = "data-pipeline"
	CategoryIntegration   = "integration"
	CategoryAutomation    = "automation"
	CategoryNotification  = "notification"
	CategoryAnalytics     = "analytics"
	CategoryDevOps        = "devops"
	CategoryCustom        = "custom"
)

// Template represents a workflow template
type Template struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	Name        string                 `json:"name" gorm:"not null;uniqueIndex"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"`
	Icon        string                 `json:"icon"`
	Workflow    json.RawMessage        `json:"workflow" gorm:"type:jsonb"`
	Variables   []Variable             `json:"variables" gorm:"serializer:json"`
	Tags        []string               `json:"tags" gorm:"serializer:json"`
	IsPublic    bool                   `json:"isPublic" gorm:"default:false"`
	IsBuiltIn   bool                   `json:"isBuiltIn" gorm:"default:false"`
	CreatorID   string                 `json:"creatorId"`
	UsageCount  int64                  `json:"usageCount" gorm:"default:0"`
	Rating      float32                `json:"rating" gorm:"default:0"`
	Config      map[string]interface{} `json:"config" gorm:"serializer:json"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
}

// Variable represents a template variable
type Variable struct {
	Key          string      `json:"key"`
	Name         string      `json:"name"`
	Type         string      `json:"type"`
	Description  string      `json:"description"`
	Required     bool        `json:"required"`
	DefaultValue interface{} `json:"defaultValue"`
	Options      []Option    `json:"options,omitempty"`
	Validation   Validation  `json:"validation,omitempty"`
}

// Option represents a variable option
type Option struct {
	Label string      `json:"label"`
	Value interface{} `json:"value"`
}

// Validation represents variable validation rules
type Validation struct {
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	MinLength *int     `json:"minLength,omitempty"`
	MaxLength *int     `json:"maxLength,omitempty"`
	Pattern   string   `json:"pattern,omitempty"`
}

// TemplateManager manages workflow templates
type TemplateManager struct {
	db         *database.DB
	logger     logger.Logger
	builtInTemplates map[string]*Template
}

// NewTemplateManager creates a new template manager
func NewTemplateManager(db *database.DB, logger logger.Logger) *TemplateManager {
	tm := &TemplateManager{
		db:         db,
		logger:     logger,
		builtInTemplates: make(map[string]*Template),
	}
	
	// Initialize built-in templates
	tm.initBuiltInTemplates()
	
	return tm
}

// initBuiltInTemplates initializes built-in templates
func (tm *TemplateManager) initBuiltInTemplates() {
	// Data ETL Pipeline
	tm.registerBuiltInTemplate(&Template{
		ID:          "template-etl-pipeline",
		Name:        "Data ETL Pipeline",
		Description: "Extract, Transform, and Load data between systems",
		Category:    CategoryDataPipeline,
		Icon:        "database",
		IsBuiltIn:   true,
		IsPublic:    true,
		Tags:        []string{"etl", "data", "pipeline", "database"},
		Variables: []Variable{
			{
				Key:         "source_type",
				Name:        "Source Type",
				Type:        VariableTypeString,
				Description: "Type of data source",
				Required:    true,
				Options: []Option{
					{Label: "PostgreSQL", Value: "postgres"},
					{Label: "MySQL", Value: "mysql"},
					{Label: "MongoDB", Value: "mongodb"},
					{Label: "API", Value: "api"},
					{Label: "CSV File", Value: "csv"},
				},
			},
			{
				Key:         "source_connection",
				Name:        "Source Connection String",
				Type:        VariableTypeSecret,
				Description: "Connection string for the data source",
				Required:    true,
			},
			{
				Key:         "target_type",
				Name:        "Target Type",
				Type:        VariableTypeString,
				Description: "Type of target system",
				Required:    true,
				Options: []Option{
					{Label: "PostgreSQL", Value: "postgres"},
					{Label: "MySQL", Value: "mysql"},
					{Label: "Data Warehouse", Value: "warehouse"},
					{Label: "S3", Value: "s3"},
				},
			},
			{
				Key:         "schedule",
				Name:        "Schedule",
				Type:        VariableTypeString,
				Description: "Cron expression for scheduling",
				DefaultValue: "0 0 * * *",
			},
		},
	})

	// Webhook to Database
	tm.registerBuiltInTemplate(&Template{
		ID:          "template-webhook-db",
		Name:        "Webhook to Database",
		Description: "Receive webhook data and store in database",
		Category:    CategoryIntegration,
		Icon:        "webhook",
		IsBuiltIn:   true,
		IsPublic:    true,
		Tags:        []string{"webhook", "database", "integration"},
		Variables: []Variable{
			{
				Key:         "webhook_path",
				Name:        "Webhook Path",
				Type:        VariableTypeString,
				Description: "URL path for the webhook endpoint",
				Required:    true,
				DefaultValue: "/webhook/data",
			},
			{
				Key:         "database_table",
				Name:        "Database Table",
				Type:        VariableTypeString,
				Description: "Target database table name",
				Required:    true,
			},
			{
				Key:         "validation_schema",
				Name:        "Validation Schema",
				Type:        VariableTypeJSON,
				Description: "JSON schema for webhook data validation",
			},
		},
	})

	// Scheduled Report
	tm.registerBuiltInTemplate(&Template{
		ID:          "template-scheduled-report",
		Name:        "Scheduled Report Generator",
		Description: "Generate and send reports on a schedule",
		Category:    CategoryAnalytics,
		Icon:        "chart",
		IsBuiltIn:   true,
		IsPublic:    true,
		Tags:        []string{"report", "schedule", "email", "analytics"},
		Variables: []Variable{
			{
				Key:         "report_type",
				Name:        "Report Type",
				Type:        VariableTypeString,
				Description: "Type of report to generate",
				Required:    true,
				Options: []Option{
					{Label: "Daily Summary", Value: "daily"},
					{Label: "Weekly Analytics", Value: "weekly"},
					{Label: "Monthly KPIs", Value: "monthly"},
					{Label: "Custom Query", Value: "custom"},
				},
			},
			{
				Key:         "recipients",
				Name:        "Email Recipients",
				Type:        VariableTypeString,
				Description: "Comma-separated list of email addresses",
				Required:    true,
			},
			{
				Key:         "schedule",
				Name:        "Schedule",
				Type:        VariableTypeString,
				Description: "Cron expression for report schedule",
				Required:    true,
				DefaultValue: "0 8 * * 1",
			},
		},
	})

	// API Integration
	tm.registerBuiltInTemplate(&Template{
		ID:          "template-api-integration",
		Name:        "API Integration Pipeline",
		Description: "Integrate with external APIs for data synchronization",
		Category:    CategoryIntegration,
		Icon:        "api",
		IsBuiltIn:   true,
		IsPublic:    true,
		Tags:        []string{"api", "integration", "sync", "rest"},
		Variables: []Variable{
			{
				Key:         "api_url",
				Name:        "API Base URL",
				Type:        VariableTypeString,
				Description: "Base URL of the API to integrate",
				Required:    true,
			},
			{
				Key:         "api_key",
				Name:        "API Key",
				Type:        VariableTypeSecret,
				Description: "API authentication key",
				Required:    true,
			},
			{
				Key:         "sync_interval",
				Name:        "Sync Interval",
				Type:        VariableTypeNumber,
				Description: "Sync interval in minutes",
				Required:    true,
				DefaultValue: 60,
				Validation: Validation{
					Min: floatPtr(1),
					Max: floatPtr(1440),
				},
			},
		},
	})

	// Error Notification
	tm.registerBuiltInTemplate(&Template{
		ID:          "template-error-notification",
		Name:        "Error Notification System",
		Description: "Monitor errors and send notifications",
		Category:    CategoryNotification,
		Icon:        "alert",
		IsBuiltIn:   true,
		IsPublic:    true,
		Tags:        []string{"error", "monitoring", "notification", "alert"},
		Variables: []Variable{
			{
				Key:         "error_source",
				Name:        "Error Source",
				Type:        VariableTypeString,
				Description: "System or service to monitor",
				Required:    true,
			},
			{
				Key:         "notification_channels",
				Name:        "Notification Channels",
				Type:        VariableTypeString,
				Description: "Channels for notifications",
				Required:    true,
				Options: []Option{
					{Label: "Email", Value: "email"},
					{Label: "Slack", Value: "slack"},
					{Label: "SMS", Value: "sms"},
					{Label: "PagerDuty", Value: "pagerduty"},
				},
			},
			{
				Key:         "severity_threshold",
				Name:        "Severity Threshold",
				Type:        VariableTypeString,
				Description: "Minimum error severity to trigger notification",
				DefaultValue: "error",
				Options: []Option{
					{Label: "Debug", Value: "debug"},
					{Label: "Info", Value: "info"},
					{Label: "Warning", Value: "warning"},
					{Label: "Error", Value: "error"},
					{Label: "Critical", Value: "critical"},
				},
			},
		},
	})
}

// registerBuiltInTemplate registers a built-in template
func (tm *TemplateManager) registerBuiltInTemplate(template *Template) {
	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()
	tm.builtInTemplates[template.ID] = template
}

// CreateTemplate creates a new template
func (tm *TemplateManager) CreateTemplate(ctx context.Context, template *Template) error {
	// Validate template
	if err := tm.validateTemplate(template); err != nil {
		return err
	}
	
	// Generate ID if not provided
	if template.ID == "" {
		template.ID = "template-" + uuid.New().String()
	}
	
	// Set timestamps
	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()
	
	// Save to database
	if err := tm.db.WithContext(ctx).Create(template).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return ErrDuplicateTemplate
		}
		return fmt.Errorf("failed to create template: %w", err)
	}
	
	tm.logger.Info("Template created", "id", template.ID, "name", template.Name)
	return nil
}

// GetTemplate retrieves a template by ID
func (tm *TemplateManager) GetTemplate(ctx context.Context, templateID string) (*Template, error) {
	// Check built-in templates first
	if template, ok := tm.builtInTemplates[templateID]; ok {
		return template, nil
	}
	
	// Check database
	var template Template
	err := tm.db.WithContext(ctx).Where("id = ?", templateID).First(&template).Error
	if err == gorm.ErrRecordNotFound {
		return nil, ErrTemplateNotFound
	}
	
	return &template, err
}

// ListTemplates lists templates with optional filtering
func (tm *TemplateManager) ListTemplates(ctx context.Context, category string, isPublic *bool) ([]*Template, error) {
	templates := []*Template{}
	
	// Add built-in templates
	for _, template := range tm.builtInTemplates {
		if category != "" && template.Category != category {
			continue
		}
		if isPublic != nil && template.IsPublic != *isPublic {
			continue
		}
		templates = append(templates, template)
	}
	
	// Query database templates
	query := tm.db.WithContext(ctx).Model(&Template{})
	
	if category != "" {
		query = query.Where("category = ?", category)
	}
	
	if isPublic != nil {
		query = query.Where("is_public = ?", *isPublic)
	}
	
	var dbTemplates []*Template
	if err := query.Order("usage_count DESC").Find(&dbTemplates).Error; err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}
	
	templates = append(templates, dbTemplates...)
	
	return templates, nil
}

// InstantiateTemplate creates a workflow from a template
func (tm *TemplateManager) InstantiateTemplate(ctx context.Context, templateID, userID, name string, variables map[string]interface{}) (*workflow.Workflow, error) {
	// Get template
	template, err := tm.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, err
	}
	
	// Validate and apply variables
	processedVars, err := tm.processVariables(template.Variables, variables)
	if err != nil {
		return nil, fmt.Errorf("variable processing failed: %w", err)
	}
	
	// Parse workflow from template
	var templateWorkflow workflow.Workflow
	if err := json.Unmarshal(template.Workflow, &templateWorkflow); err != nil {
		return nil, fmt.Errorf("failed to parse template workflow: %w", err)
	}
	
	// Create new workflow instance
	wf := workflow.NewWorkflow(name, template.Description, userID)
	wf.Nodes = templateWorkflow.Nodes
	wf.Connections = templateWorkflow.Connections
	wf.Settings = templateWorkflow.Settings
	wf.Tags = template.Tags
	
	// Apply variable substitutions
	if err := tm.applyVariables(wf, processedVars); err != nil {
		return nil, fmt.Errorf("failed to apply variables: %w", err)
	}
	
	// Increment template usage count
	if !template.IsBuiltIn {
		tm.db.Model(&Template{}).Where("id = ?", templateID).
			UpdateColumn("usage_count", gorm.Expr("usage_count + 1"))
	}
	
	tm.logger.Info("Workflow instantiated from template",
		"template_id", templateID,
		"workflow_id", wf.ID,
		"user_id", userID)
	
	return wf, nil
}

// UpdateTemplate updates a template
func (tm *TemplateManager) UpdateTemplate(ctx context.Context, templateID string, updates map[string]interface{}) error {
	// Built-in templates cannot be updated
	if _, ok := tm.builtInTemplates[templateID]; ok {
		return errors.New("built-in templates cannot be modified")
	}
	
	// Update in database
	result := tm.db.WithContext(ctx).Model(&Template{}).
		Where("id = ?", templateID).
		Updates(updates)
	
	if result.Error != nil {
		return fmt.Errorf("failed to update template: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return ErrTemplateNotFound
	}
	
	tm.logger.Info("Template updated", "id", templateID)
	return nil
}

// DeleteTemplate deletes a template
func (tm *TemplateManager) DeleteTemplate(ctx context.Context, templateID string) error {
	// Built-in templates cannot be deleted
	if _, ok := tm.builtInTemplates[templateID]; ok {
		return errors.New("built-in templates cannot be deleted")
	}
	
	result := tm.db.WithContext(ctx).Delete(&Template{}, "id = ?", templateID)
	if result.Error != nil {
		return fmt.Errorf("failed to delete template: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return ErrTemplateNotFound
	}
	
	tm.logger.Info("Template deleted", "id", templateID)
	return nil
}

// validateTemplate validates a template
func (tm *TemplateManager) validateTemplate(template *Template) error {
	if template.Name == "" {
		return errors.New("template name is required")
	}
	
	if template.Category == "" {
		template.Category = CategoryCustom
	}
	
	// Validate category
	validCategories := map[string]bool{
		CategoryDataPipeline:  true,
		CategoryIntegration:   true,
		CategoryAutomation:    true,
		CategoryNotification:  true,
		CategoryAnalytics:     true,
		CategoryDevOps:        true,
		CategoryCustom:        true,
	}
	
	if !validCategories[template.Category] {
		return fmt.Errorf("invalid category: %s", template.Category)
	}
	
	// Validate workflow JSON
	if len(template.Workflow) > 0 {
		var wf workflow.Workflow
		if err := json.Unmarshal(template.Workflow, &wf); err != nil {
			return fmt.Errorf("invalid workflow JSON: %w", err)
		}
	}
	
	// Validate variables
	for _, v := range template.Variables {
		if err := tm.validateVariable(&v); err != nil {
			return fmt.Errorf("invalid variable %s: %w", v.Key, err)
		}
	}
	
	return nil
}

// validateVariable validates a template variable
func (tm *TemplateManager) validateVariable(v *Variable) error {
	if v.Key == "" {
		return errors.New("variable key is required")
	}
	
	// Validate type
	validTypes := map[string]bool{
		VariableTypeString:  true,
		VariableTypeNumber:  true,
		VariableTypeBoolean: true,
		VariableTypeJSON:    true,
		VariableTypeSecret:  true,
	}
	
	if !validTypes[v.Type] {
		return ErrInvalidVariableType
	}
	
	// Validate default value type matches variable type
	if v.DefaultValue != nil {
		switch v.Type {
		case VariableTypeString, VariableTypeSecret:
			if _, ok := v.DefaultValue.(string); !ok {
				return errors.New("default value must be a string")
			}
		case VariableTypeNumber:
			switch v.DefaultValue.(type) {
			case int, int32, int64, float32, float64:
				// Valid
			default:
				return errors.New("default value must be a number")
			}
		case VariableTypeBoolean:
			if _, ok := v.DefaultValue.(bool); !ok {
				return errors.New("default value must be a boolean")
			}
		}
	}
	
	return nil
}

// processVariables processes and validates template variables
func (tm *TemplateManager) processVariables(templateVars []Variable, providedVars map[string]interface{}) (map[string]interface{}, error) {
	processed := make(map[string]interface{})
	
	for _, tv := range templateVars {
		value, exists := providedVars[tv.Key]
		
		// Check required variables
		if !exists {
			if tv.Required && tv.DefaultValue == nil {
				return nil, fmt.Errorf("%w: %s", ErrVariableRequired, tv.Key)
			}
			if tv.DefaultValue != nil {
				value = tv.DefaultValue
			} else {
				continue
			}
		}
		
		// Validate type
		if err := tm.validateVariableValue(&tv, value); err != nil {
			return nil, fmt.Errorf("variable %s: %w", tv.Key, err)
		}
		
		processed[tv.Key] = value
	}
	
	return processed, nil
}

// validateVariableValue validates a variable value
func (tm *TemplateManager) validateVariableValue(v *Variable, value interface{}) error {
	switch v.Type {
	case VariableTypeString, VariableTypeSecret:
		str, ok := value.(string)
		if !ok {
			return ErrInvalidVariableType
		}
		
		// Check length constraints
		if v.Validation.MinLength != nil && len(str) < *v.Validation.MinLength {
			return fmt.Errorf("string too short (min: %d)", *v.Validation.MinLength)
		}
		if v.Validation.MaxLength != nil && len(str) > *v.Validation.MaxLength {
			return fmt.Errorf("string too long (max: %d)", *v.Validation.MaxLength)
		}
		
	case VariableTypeNumber:
		var num float64
		switch val := value.(type) {
		case int:
			num = float64(val)
		case int32:
			num = float64(val)
		case int64:
			num = float64(val)
		case float32:
			num = float64(val)
		case float64:
			num = val
		default:
			return ErrInvalidVariableType
		}
		
		// Check range constraints
		if v.Validation.Min != nil && num < *v.Validation.Min {
			return fmt.Errorf("number too small (min: %f)", *v.Validation.Min)
		}
		if v.Validation.Max != nil && num > *v.Validation.Max {
			return fmt.Errorf("number too large (max: %f)", *v.Validation.Max)
		}
		
	case VariableTypeBoolean:
		if _, ok := value.(bool); !ok {
			return ErrInvalidVariableType
		}
		
	case VariableTypeJSON:
		// Try to marshal to validate JSON
		if _, err := json.Marshal(value); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
	}
	
	return nil
}

// applyVariables applies variable substitutions to a workflow
func (tm *TemplateManager) applyVariables(wf *workflow.Workflow, variables map[string]interface{}) error {
	// Convert workflow to JSON for string replacement
	wfJSON, err := json.Marshal(wf)
	if err != nil {
		return err
	}
	
	wfStr := string(wfJSON)
	
	// Replace variable placeholders
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		
		// Convert value to string for replacement
		var valueStr string
		switch v := value.(type) {
		case string:
			valueStr = v
		case int, int32, int64, float32, float64:
			valueStr = fmt.Sprintf("%v", v)
		case bool:
			valueStr = fmt.Sprintf("%v", v)
		default:
			// For complex types, marshal to JSON
			jsonBytes, _ := json.Marshal(v)
			valueStr = string(jsonBytes)
		}
		
		wfStr = strings.ReplaceAll(wfStr, placeholder, valueStr)
	}
	
	// Parse back to workflow
	return json.Unmarshal([]byte(wfStr), wf)
}

// GetCategories returns all available template categories
func (tm *TemplateManager) GetCategories() []map[string]interface{} {
	return []map[string]interface{}{
		{"id": CategoryDataPipeline, "name": "Data Pipeline", "icon": "database"},
		{"id": CategoryIntegration, "name": "Integration", "icon": "link"},
		{"id": CategoryAutomation, "name": "Automation", "icon": "robot"},
		{"id": CategoryNotification, "name": "Notification", "icon": "bell"},
		{"id": CategoryAnalytics, "name": "Analytics", "icon": "chart"},
		{"id": CategoryDevOps, "name": "DevOps", "icon": "server"},
		{"id": CategoryCustom, "name": "Custom", "icon": "code"},
	}
}

// Helper function to create float64 pointer
func floatPtr(f float64) *float64 {
	return &f
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}
