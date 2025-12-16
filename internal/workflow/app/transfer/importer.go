package transfer

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/linkflow-go/pkg/contracts/workflow"
	"github.com/linkflow-go/pkg/logger"
	"gopkg.in/yaml.v3"
)

var (
	ErrInvalidImportFormat = errors.New("invalid import format")
	ErrVersionMismatch     = errors.New("incompatible export version")
	ErrImportValidation    = errors.New("import validation failed")
)

// Importer handles workflow import operations
type Importer struct {
	logger logger.Logger
}

// NewImporter creates a new importer
func NewImporter(logger logger.Logger) *Importer {
	return &Importer{
		logger: logger,
	}
}

// ImportWorkflow imports a workflow from exported data
func (i *Importer) ImportWorkflow(data []byte, format string, options ImportOptions) (*workflow.Workflow, error) {
	var export WorkflowExport

	// Parse based on format
	switch format {
	case FormatJSON:
		if err := json.Unmarshal(data, &export); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	case FormatYAML:
		if err := yaml.Unmarshal(data, &export); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
	case FormatN8N:
		return i.importN8N(data, options)
	case FormatZapier:
		return i.importZapier(data, options)
	default:
		return nil, ErrInvalidImportFormat
	}

	// Validate export version
	if !i.isVersionCompatible(export.Version) {
		return nil, ErrVersionMismatch
	}

	// Create workflow from export
	return i.createWorkflowFromExport(&export, options)
}

// createWorkflowFromExport creates a workflow from export data
func (i *Importer) createWorkflowFromExport(export *WorkflowExport, options ImportOptions) (*workflow.Workflow, error) {
	// Create new workflow
	wf := &workflow.Workflow{
		ID:          uuid.New().String(),
		Name:        export.Workflow.Name,
		Description: export.Workflow.Description,
		UserID:      options.UserID,
		Version:     1,
		Status:      workflow.StatusInactive,
		IsActive:    false,
		Tags:        export.Workflow.Tags,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Override name if provided
	if options.NewName != "" {
		wf.Name = options.NewName
	}

	// Import settings
	if export.Workflow.Settings != nil {
		wf.Settings = i.mapToSettings(export.Workflow.Settings)
	}

	// Map node IDs if remapping is enabled
	nodeIDMap := make(map[string]string)
	if options.RemapIDs {
		for _, node := range export.Nodes {
			nodeIDMap[node.ID] = uuid.New().String()
		}
	} else {
		for _, node := range export.Nodes {
			nodeIDMap[node.ID] = node.ID
		}
	}

	// Import nodes
	wf.Nodes = []workflow.Node{}
	for _, exportNode := range export.Nodes {
		node := workflow.Node{
			ID:   nodeIDMap[exportNode.ID],
			Name: exportNode.Name,
			Type: exportNode.Type,
			Position: workflow.Position{
				X: exportNode.Position["x"],
				Y: exportNode.Position["y"],
			},
			Parameters: exportNode.Parameters,
			Disabled:   exportNode.Disabled,
			RetryCount: exportNode.RetryCount,
			Timeout:    exportNode.Timeout,
		}

		// Handle credential mapping
		if options.CredentialMapping != nil {
			i.mapCredentials(&node, options.CredentialMapping)
		}

		wf.Nodes = append(wf.Nodes, node)
	}

	// Import connections with remapped IDs
	wf.Connections = []workflow.Connection{}
	for _, exportConn := range export.Connections {
		conn := workflow.Connection{
			ID:         exportConn.ID,
			Source:     nodeIDMap[exportConn.Source],
			Target:     nodeIDMap[exportConn.Target],
			SourcePort: exportConn.SourcePort,
			TargetPort: exportConn.TargetPort,
			Data:       exportConn.Data,
		}

		if options.RemapIDs {
			conn.ID = uuid.New().String()
		}

		wf.Connections = append(wf.Connections, conn)
	}

	// Validate imported workflow
	if options.ValidateOnImport {
		if err := wf.Validate(); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrImportValidation, err)
		}
	}

	i.logger.Info("Workflow imported",
		"name", wf.Name,
		"nodes", len(wf.Nodes),
		"connections", len(wf.Connections))

	return wf, nil
}

// importN8N imports a workflow from n8n format
func (i *Importer) importN8N(data []byte, options ImportOptions) (*workflow.Workflow, error) {
	var n8nExport map[string]interface{}
	if err := json.Unmarshal(data, &n8nExport); err != nil {
		return nil, fmt.Errorf("failed to parse n8n export: %w", err)
	}

	wf := &workflow.Workflow{
		ID:        uuid.New().String(),
		Name:      n8nExport["name"].(string),
		UserID:    options.UserID,
		Version:   1,
		Status:    workflow.StatusInactive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if options.NewName != "" {
		wf.Name = options.NewName
	}

	// Import nodes
	if nodes, ok := n8nExport["nodes"].([]interface{}); ok {
		for _, n := range nodes {
			node := n.(map[string]interface{})
			position := node["position"].([]interface{})

			wfNode := workflow.Node{
				ID:   node["id"].(string),
				Name: node["name"].(string),
				Type: i.mapFromN8NNodeType(node["type"].(string)),
				Position: workflow.Position{
					X: position[0].(float64),
					Y: position[1].(float64),
				},
			}

			if params, ok := node["parameters"].(map[string]interface{}); ok {
				wfNode.Parameters = params
			}

			wf.Nodes = append(wf.Nodes, wfNode)
		}
	}

	// Import connections
	if connections, ok := n8nExport["connections"].(map[string]interface{}); ok {
		connID := 1
		for sourceID, sourceConns := range connections {
			for port, portConns := range sourceConns.(map[string]interface{}) {
				for _, connGroup := range portConns.([]interface{}) {
					for _, conn := range connGroup.([]interface{}) {
						connData := conn.(map[string]interface{})
						wfConn := workflow.Connection{
							ID:         fmt.Sprintf("conn_%d", connID),
							Source:     sourceID,
							Target:     connData["node"].(string),
							SourcePort: port,
						}
						wf.Connections = append(wf.Connections, wfConn)
						connID++
					}
				}
			}
		}
	}

	return wf, nil
}

// importZapier imports a workflow from Zapier format
func (i *Importer) importZapier(data []byte, options ImportOptions) (*workflow.Workflow, error) {
	var zapExport map[string]interface{}
	if err := json.Unmarshal(data, &zapExport); err != nil {
		return nil, fmt.Errorf("failed to parse Zapier export: %w", err)
	}

	wf := &workflow.Workflow{
		ID:          uuid.New().String(),
		Name:        zapExport["name"].(string),
		Description: zapExport["description"].(string),
		UserID:      options.UserID,
		Version:     1,
		Status:      workflow.StatusInactive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if options.NewName != "" {
		wf.Name = options.NewName
	}

	// Import steps as nodes
	if steps, ok := zapExport["steps"].([]interface{}); ok {
		var lastNodeID string

		for idx, s := range steps {
			step := s.(map[string]interface{})

			nodeID := step["id"].(string)
			if nodeID == "" {
				nodeID = fmt.Sprintf("step_%d", idx+1)
			}

			wfNode := workflow.Node{
				ID:   nodeID,
				Name: step["action"].(string),
				Type: i.mapFromZapierApp(step["app"].(string)),
				Position: workflow.Position{
					X: float64(100 + idx*200),
					Y: 100,
				},
			}

			if config, ok := step["config"].(map[string]interface{}); ok {
				wfNode.Parameters = config
			}

			wf.Nodes = append(wf.Nodes, wfNode)

			// Create linear connections
			if lastNodeID != "" {
				wf.Connections = append(wf.Connections, workflow.Connection{
					ID:     fmt.Sprintf("conn_%d", idx),
					Source: lastNodeID,
					Target: nodeID,
				})
			}

			lastNodeID = nodeID
		}
	}

	return wf, nil
}

// isVersionCompatible checks if the export version is compatible
func (i *Importer) isVersionCompatible(version string) bool {
	// Simple version check - in production, use semantic versioning
	return version == ExportVersion || version == "1.0.0"
}

// mapToSettings converts map to workflow settings
func (i *Importer) mapToSettings(m map[string]interface{}) workflow.Settings {
	settings := workflow.Settings{}

	if timeout, ok := m["timeout"].(float64); ok {
		settings.Timeout = int(timeout)
	}
	if retry, ok := m["retryOnFailure"].(bool); ok {
		settings.RetryOnFailure = retry
	}
	if maxRetries, ok := m["maxRetries"].(float64); ok {
		settings.MaxRetries = int(maxRetries)
	}
	if tz, ok := m["timezone"].(string); ok {
		settings.Timezone = tz
	}

	// Handle error handling settings
	if eh, ok := m["errorHandling"].(map[string]interface{}); ok {
		if continueOnFail, ok := eh["continueOnFail"].(bool); ok {
			settings.ErrorHandling.ContinueOnFail = continueOnFail
		}
		if retryInterval, ok := eh["retryInterval"].(float64); ok {
			settings.ErrorHandling.RetryInterval = int(retryInterval)
		}
		if maxRetries, ok := eh["maxRetries"].(float64); ok {
			settings.ErrorHandling.MaxRetries = int(maxRetries)
		}
	}

	return settings
}

// mapCredentials maps credentials based on mapping configuration
func (i *Importer) mapCredentials(node *workflow.Node, mapping map[string]string) {
	if credID, ok := node.Parameters["credentialId"].(string); ok {
		if newID, exists := mapping[credID]; exists {
			node.Parameters["credentialId"] = newID
		} else {
			// Remove credential if not mapped
			delete(node.Parameters, "credentialId")
		}
	}
}

// mapFromN8NNodeType maps n8n node types to internal types
func (i *Importer) mapFromN8NNodeType(n8nType string) string {
	typeMap := map[string]string{
		"n8n-nodes-base.webhook":        workflow.NodeTypeWebhook,
		"n8n-nodes-base.httpRequest":    workflow.NodeTypeHTTPRequest,
		"n8n-nodes-base.postgres":       workflow.NodeTypeDatabase,
		"n8n-nodes-base.emailSend":      workflow.NodeTypeEmail,
		"n8n-nodes-base.slack":          workflow.NodeTypeSlack,
		"n8n-nodes-base.code":           workflow.NodeTypeCode,
		"n8n-nodes-base.merge":          workflow.NodeTypeMerge,
		"n8n-nodes-base.splitInBatches": workflow.NodeTypeSplit,
		"n8n-nodes-base.if":             workflow.NodeTypeCondition,
	}

	if mapped, ok := typeMap[n8nType]; ok {
		return mapped
	}
	return workflow.NodeTypeAction
}

// mapFromZapierApp maps Zapier apps to internal types
func (i *Importer) mapFromZapierApp(app string) string {
	typeMap := map[string]string{
		"webhook":    workflow.NodeTypeWebhook,
		"webhooks":   workflow.NodeTypeHTTPRequest,
		"postgresql": workflow.NodeTypeDatabase,
		"email":      workflow.NodeTypeEmail,
		"slack":      workflow.NodeTypeSlack,
		"code":       workflow.NodeTypeCode,
	}

	if mapped, ok := typeMap[app]; ok {
		return mapped
	}
	return workflow.NodeTypeAction
}

// ImportOptions defines options for import
type ImportOptions struct {
	UserID            string
	NewName           string
	RemapIDs          bool
	ValidateOnImport  bool
	CredentialMapping map[string]string
	VariableMapping   map[string]interface{}
}
