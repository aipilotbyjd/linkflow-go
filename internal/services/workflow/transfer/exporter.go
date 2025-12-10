package transfer

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/pkg/logger"
	"gopkg.in/yaml.v3"
)

// Export formats
const (
	FormatJSON = "json"
	FormatYAML = "yaml"
	FormatN8N  = "n8n"
	FormatZapier = "zapier"
)

// ExportVersion defines the export format version
const ExportVersion = "1.0.0"

var (
	ErrInvalidFormat = errors.New("invalid export format")
	ErrExportFailed  = errors.New("export failed")
)

// WorkflowExport represents an exported workflow
type WorkflowExport struct {
	Version     string                   `json:"version" yaml:"version"`
	ExportedAt  time.Time                `json:"exportedAt" yaml:"exportedAt"`
	Workflow    WorkflowData             `json:"workflow" yaml:"workflow"`
	Nodes       []NodeExport             `json:"nodes" yaml:"nodes"`
	Connections []ConnectionExport       `json:"connections" yaml:"connections"`
	Variables   []VariableExport         `json:"variables,omitempty" yaml:"variables,omitempty"`
	Triggers    []TriggerExport          `json:"triggers,omitempty" yaml:"triggers,omitempty"`
	Credentials []CredentialReference    `json:"requiredCredentials,omitempty" yaml:"requiredCredentials,omitempty"`
	Metadata    map[string]interface{}   `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// WorkflowData contains workflow metadata
type WorkflowData struct {
	ID          string                 `json:"id" yaml:"id"`
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description" yaml:"description"`
	Version     int                    `json:"version" yaml:"version"`
	Tags        []string               `json:"tags,omitempty" yaml:"tags,omitempty"`
	Settings    map[string]interface{} `json:"settings" yaml:"settings"`
}

// NodeExport represents an exported node
type NodeExport struct {
	ID         string                 `json:"id" yaml:"id"`
	Name       string                 `json:"name" yaml:"name"`
	Type       string                 `json:"type" yaml:"type"`
	Position   map[string]float64     `json:"position" yaml:"position"`
	Parameters map[string]interface{} `json:"parameters" yaml:"parameters"`
	Disabled   bool                   `json:"disabled,omitempty" yaml:"disabled,omitempty"`
	RetryCount int                    `json:"retryCount,omitempty" yaml:"retryCount,omitempty"`
	Timeout    int                    `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// ConnectionExport represents an exported connection
type ConnectionExport struct {
	ID         string                 `json:"id" yaml:"id"`
	Source     string                 `json:"source" yaml:"source"`
	Target     string                 `json:"target" yaml:"target"`
	SourcePort string                 `json:"sourcePort,omitempty" yaml:"sourcePort,omitempty"`
	TargetPort string                 `json:"targetPort,omitempty" yaml:"targetPort,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty" yaml:"data,omitempty"`
}

// VariableExport represents an exported variable
type VariableExport struct {
	Key         string      `json:"key" yaml:"key"`
	Type        string      `json:"type" yaml:"type"`
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool        `json:"required,omitempty" yaml:"required,omitempty"`
	DefaultValue interface{} `json:"defaultValue,omitempty" yaml:"defaultValue,omitempty"`
}

// TriggerExport represents an exported trigger
type TriggerExport struct {
	ID          string                 `json:"id" yaml:"id"`
	Type        string                 `json:"type" yaml:"type"`
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Config      map[string]interface{} `json:"config" yaml:"config"`
}

// CredentialReference represents a required credential
type CredentialReference struct {
	ID       string `json:"id" yaml:"id"`
	Type     string `json:"type" yaml:"type"`
	Name     string `json:"name" yaml:"name"`
	Required bool   `json:"required" yaml:"required"`
}

// Exporter handles workflow export operations
type Exporter struct {
	logger logger.Logger
}

// NewExporter creates a new exporter
func NewExporter(logger logger.Logger) *Exporter {
	return &Exporter{
		logger: logger,
	}
}

// ExportWorkflow exports a workflow in the specified format
func (e *Exporter) ExportWorkflow(wf *workflow.Workflow, format string, options ExportOptions) ([]byte, error) {
	// Create export structure
	export := e.createExportStructure(wf, options)
	
	// Export based on format
	switch format {
	case FormatJSON:
		return e.exportJSON(export)
	case FormatYAML:
		return e.exportYAML(export)
	case FormatN8N:
		return e.exportN8N(wf)
	case FormatZapier:
		return e.exportZapier(wf)
	default:
		return nil, ErrInvalidFormat
	}
}

// createExportStructure creates the export structure from a workflow
func (e *Exporter) createExportStructure(wf *workflow.Workflow, options ExportOptions) *WorkflowExport {
	export := &WorkflowExport{
		Version:    ExportVersion,
		ExportedAt: time.Now(),
		Workflow: WorkflowData{
			ID:          wf.ID,
			Name:        wf.Name,
			Description: wf.Description,
			Version:     wf.Version,
			Tags:        wf.Tags,
			Settings:    structToMap(wf.Settings),
		},
		Nodes:       []NodeExport{},
		Connections: []ConnectionExport{},
		Metadata:    make(map[string]interface{}),
	}
	
	// Export nodes
	for _, node := range wf.Nodes {
		export.Nodes = append(export.Nodes, NodeExport{
			ID:   node.ID,
			Name: node.Name,
			Type: node.Type,
			Position: map[string]float64{
				"x": node.Position.X,
				"y": node.Position.Y,
			},
			Parameters: node.Parameters,
			Disabled:   node.Disabled,
			RetryCount: node.RetryCount,
			Timeout:    node.Timeout,
		})
		
		// Extract credential requirements
		if creds := e.extractCredentials(node); len(creds) > 0 {
			export.Credentials = append(export.Credentials, creds...)
		}
	}
	
	// Export connections
	for _, conn := range wf.Connections {
		export.Connections = append(export.Connections, ConnectionExport{
			ID:         conn.ID,
			Source:     conn.Source,
			Target:     conn.Target,
			SourcePort: conn.SourcePort,
			TargetPort: conn.TargetPort,
			Data:       conn.Data,
		})
	}
	
	// Add metadata
	if options.IncludeMetadata {
		export.Metadata["exportedBy"] = options.ExportedBy
		export.Metadata["originalID"] = wf.ID
		export.Metadata["createdAt"] = wf.CreatedAt
		export.Metadata["updatedAt"] = wf.UpdatedAt
	}
	
	// Sanitize if requested
	if options.Sanitize {
		e.sanitizeExport(export)
	}
	
	return export
}

// exportJSON exports workflow as JSON
func (e *Exporter) exportJSON(export *WorkflowExport) ([]byte, error) {
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return data, nil
}

// exportYAML exports workflow as YAML
func (e *Exporter) exportYAML(export *WorkflowExport) ([]byte, error) {
	data, err := yaml.Marshal(export)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal YAML: %w", err)
	}
	return data, nil
}

// exportN8N exports workflow in n8n format
func (e *Exporter) exportN8N(wf *workflow.Workflow) ([]byte, error) {
	// Convert to n8n format
	n8nExport := map[string]interface{}{
		"name":        wf.Name,
		"nodes":       []map[string]interface{}{},
		"connections": map[string]interface{}{},
		"active":      wf.IsActive,
		"settings":    structToMap(wf.Settings),
		"id":          wf.ID,
	}
	
	// Convert nodes
	for _, node := range wf.Nodes {
		n8nNode := map[string]interface{}{
			"id":         node.ID,
			"name":       node.Name,
			"type":       e.mapToN8NNodeType(node.Type),
			"position":   []float64{node.Position.X, node.Position.Y},
			"parameters": node.Parameters,
		}
		n8nExport["nodes"] = append(n8nExport["nodes"].([]map[string]interface{}), n8nNode)
	}
	
	// Convert connections
	connections := make(map[string]map[string][][]map[string]interface{})
	for _, conn := range wf.Connections {
		if connections[conn.Source] == nil {
			connections[conn.Source] = make(map[string][][]map[string]interface{})
		}
		
		mainPort := "main"
		if conn.SourcePort != "" {
			mainPort = conn.SourcePort
		}
		
		connections[conn.Source][mainPort] = append(
			connections[conn.Source][mainPort],
			[]map[string]interface{}{
				{
					"node":  conn.Target,
					"type":  "main",
					"index": 0,
				},
			},
		)
	}
	n8nExport["connections"] = connections
	
	return json.MarshalIndent(n8nExport, "", "  ")
}

// exportZapier exports workflow in Zapier format (simplified)
func (e *Exporter) exportZapier(wf *workflow.Workflow) ([]byte, error) {
	// Zapier uses a different structure
	zapExport := map[string]interface{}{
		"name":        wf.Name,
		"description": wf.Description,
		"steps":       []map[string]interface{}{},
	}
	
	// Convert nodes to Zapier steps
	for i, node := range wf.Nodes {
		step := map[string]interface{}{
			"id":       node.ID,
			"position": i + 1,
			"app":      e.mapToZapierApp(node.Type),
			"action":   node.Name,
			"config":   node.Parameters,
		}
		zapExport["steps"] = append(zapExport["steps"].([]map[string]interface{}), step)
	}
	
	return json.MarshalIndent(zapExport, "", "  ")
}

// extractCredentials extracts credential requirements from a node
func (e *Exporter) extractCredentials(node workflow.Node) []CredentialReference {
	creds := []CredentialReference{}
	
	// Check for credential parameters
	if credID, ok := node.Parameters["credentialId"].(string); ok {
		creds = append(creds, CredentialReference{
			ID:       credID,
			Type:     node.Type,
			Name:     fmt.Sprintf("%s Credential", node.Name),
			Required: true,
		})
	}
	
	return creds
}

// sanitizeExport removes sensitive information from export
func (e *Exporter) sanitizeExport(export *WorkflowExport) {
	// Remove sensitive parameters from nodes
	for i := range export.Nodes {
		e.sanitizeNodeParameters(&export.Nodes[i])
	}
	
	// Clear credential IDs
	for i := range export.Credentials {
		export.Credentials[i].ID = ""
	}
	
	// Remove sensitive metadata
	delete(export.Metadata, "apiKeys")
	delete(export.Metadata, "secrets")
}

// sanitizeNodeParameters removes sensitive parameters from a node
func (e *Exporter) sanitizeNodeParameters(node *NodeExport) {
	sensitiveKeys := []string{
		"password", "apiKey", "secret", "token",
		"credential", "auth", "privateKey",
	}
	
	for _, key := range sensitiveKeys {
		delete(node.Parameters, key)
	}
}

// mapToN8NNodeType maps internal node types to n8n types
func (e *Exporter) mapToN8NNodeType(nodeType string) string {
	typeMap := map[string]string{
		workflow.NodeTypeWebhook:     "n8n-nodes-base.webhook",
		workflow.NodeTypeHTTPRequest: "n8n-nodes-base.httpRequest",
		workflow.NodeTypeDatabase:    "n8n-nodes-base.postgres",
		workflow.NodeTypeEmail:       "n8n-nodes-base.emailSend",
		workflow.NodeTypeSlack:       "n8n-nodes-base.slack",
		workflow.NodeTypeCode:        "n8n-nodes-base.code",
		workflow.NodeTypeMerge:       "n8n-nodes-base.merge",
		workflow.NodeTypeSplit:       "n8n-nodes-base.splitInBatches",
		workflow.NodeTypeCondition:   "n8n-nodes-base.if",
	}
	
	if mapped, ok := typeMap[nodeType]; ok {
		return mapped
	}
	return nodeType
}

// mapToZapierApp maps internal node types to Zapier apps
func (e *Exporter) mapToZapierApp(nodeType string) string {
	typeMap := map[string]string{
		workflow.NodeTypeWebhook:     "webhook",
		workflow.NodeTypeHTTPRequest: "webhooks",
		workflow.NodeTypeDatabase:    "postgresql",
		workflow.NodeTypeEmail:       "email",
		workflow.NodeTypeSlack:       "slack",
		workflow.NodeTypeCode:        "code",
	}
	
	if mapped, ok := typeMap[nodeType]; ok {
		return mapped
	}
	return "custom"
}

// ExportOptions defines options for export
type ExportOptions struct {
	IncludeCredentials bool
	IncludeVariables   bool
	IncludeTriggers    bool
	IncludeMetadata    bool
	Sanitize           bool
	ExportedBy         string
}

// Helper function to convert struct to map
func structToMap(v interface{}) map[string]interface{} {
	data, _ := json.Marshal(v)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result
}
