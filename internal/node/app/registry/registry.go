package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	node "github.com/linkflow-go/internal/node/domain"
	"github.com/linkflow-go/internal/node/ports"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type NodeRegistry struct {
	repository ports.NodeRepository
	redis      *redis.Client
	logger     logger.Logger
	nodes      map[string]*node.NodeType
	nodesMux   sync.RWMutex
	stopCh     chan struct{}
}

func NewNodeRegistry(repo ports.NodeRepository, redis *redis.Client, logger logger.Logger) *NodeRegistry {
	return &NodeRegistry{
		repository: repo,
		redis:      redis,
		logger:     logger,
		nodes:      make(map[string]*node.NodeType),
		stopCh:     make(chan struct{}),
	}
}

func (r *NodeRegistry) Start() {
	r.logger.Info("Starting node registry")

	// Load all node types
	if err := r.loadNodeTypes(); err != nil {
		r.logger.Error("Failed to load node types", "error", err)
	}

	// Start background refresh
	go r.refreshNodeTypes()
}

func (r *NodeRegistry) Stop() {
	r.logger.Info("Stopping node registry")
	close(r.stopCh)
}

func (r *NodeRegistry) RegisterBuiltinNodes() {
	builtinNodes := []*node.NodeType{
		// Trigger nodes
		{
			ID:          uuid.New().String(),
			Type:        "manual-trigger",
			Name:        "Manual Trigger",
			Description: "Manually trigger workflow execution",
			Category:    "trigger",
			Icon:        "play-circle",
			Color:       "#00b894",
			Version:     "1.0.0",
			Schema:      node.NodeSchema{},
			Status:      "active",
			IsBuiltin:   true,
		},
		{
			ID:          uuid.New().String(),
			Type:        "webhook-trigger",
			Name:        "Webhook Trigger",
			Description: "Trigger workflow on webhook",
			Category:    "trigger",
			Icon:        "webhook",
			Color:       "#fdcb6e",
			Version:     "1.0.0",
			Schema: node.NodeSchema{
				Inputs: []node.SchemaField{
					{
						Name:     "method",
						Type:     "select",
						Label:    "HTTP Method",
						Required: true,
						Default:  "POST",
						Options:  []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
					},
					{
						Name:        "path",
						Type:        "string",
						Label:       "Webhook Path",
						Required:    true,
						Placeholder: "/webhook/my-workflow",
					},
				},
			},
			Status:    "active",
			IsBuiltin: true,
		},
		{
			ID:          uuid.New().String(),
			Type:        "schedule-trigger",
			Name:        "Schedule Trigger",
			Description: "Trigger workflow on schedule",
			Category:    "trigger",
			Icon:        "clock",
			Color:       "#a29bfe",
			Version:     "1.0.0",
			Schema: node.NodeSchema{
				Inputs: []node.SchemaField{
					{
						Name:        "cron",
						Type:        "string",
						Label:       "Cron Expression",
						Required:    true,
						Placeholder: "0 0 * * *",
						Help:        "Use cron syntax for scheduling",
					},
					{
						Name:     "timezone",
						Type:     "select",
						Label:    "Timezone",
						Required: true,
						Default:  "UTC",
						Options:  []string{"UTC", "America/New_York", "Europe/London", "Asia/Tokyo"},
					},
				},
			},
			Status:    "active",
			IsBuiltin: true,
		},
		// Action nodes
		{
			ID:          uuid.New().String(),
			Type:        "http-request",
			Name:        "HTTP Request",
			Description: "Make HTTP requests to external APIs",
			Category:    "action",
			Icon:        "globe",
			Color:       "#0984e3",
			Version:     "1.0.0",
			Schema: node.NodeSchema{
				Inputs: []node.SchemaField{
					{
						Name:        "url",
						Type:        "string",
						Label:       "URL",
						Required:    true,
						Placeholder: "https://api.example.com/endpoint",
					},
					{
						Name:     "method",
						Type:     "select",
						Label:    "Method",
						Required: true,
						Default:  "GET",
						Options:  []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD"},
					},
					{
						Name:  "headers",
						Type:  "json",
						Label: "Headers",
						Default: map[string]string{
							"Content-Type": "application/json",
						},
					},
					{
						Name:  "body",
						Type:  "json",
						Label: "Request Body",
					},
					{
						Name:    "timeout",
						Type:    "number",
						Label:   "Timeout (seconds)",
						Default: 30,
					},
				},
				Outputs: []node.SchemaField{
					{
						Name:  "statusCode",
						Type:  "number",
						Label: "Status Code",
					},
					{
						Name:  "headers",
						Type:  "json",
						Label: "Response Headers",
					},
					{
						Name:  "body",
						Type:  "json",
						Label: "Response Body",
					},
				},
			},
			Status:    "active",
			IsBuiltin: true,
		},
		{
			ID:          uuid.New().String(),
			Type:        "database-query",
			Name:        "Database Query",
			Description: "Execute database queries",
			Category:    "action",
			Icon:        "database",
			Color:       "#00cec9",
			Version:     "1.0.0",
			Schema: node.NodeSchema{
				Inputs: []node.SchemaField{
					{
						Name:     "type",
						Type:     "select",
						Label:    "Database Type",
						Required: true,
						Options:  []string{"postgres", "mysql", "mongodb", "redis"},
					},
					{
						Name:        "connectionString",
						Type:        "credential",
						Label:       "Connection String",
						Required:    true,
						Placeholder: "postgres://user:pass@host:port/db",
					},
					{
						Name:     "query",
						Type:     "code",
						Label:    "Query",
						Required: true,
						Language: "sql",
					},
					{
						Name:  "parameters",
						Type:  "json",
						Label: "Query Parameters",
					},
				},
				Outputs: []node.SchemaField{
					{
						Name:  "rows",
						Type:  "array",
						Label: "Result Rows",
					},
					{
						Name:  "rowCount",
						Type:  "number",
						Label: "Row Count",
					},
				},
			},
			Status:    "active",
			IsBuiltin: true,
		},
		{
			ID:          uuid.New().String(),
			Type:        "send-email",
			Name:        "Send Email",
			Description: "Send emails via SMTP or providers",
			Category:    "action",
			Icon:        "mail",
			Color:       "#e17055",
			Version:     "1.0.0",
			Schema: node.NodeSchema{
				Inputs: []node.SchemaField{
					{
						Name:        "to",
						Type:        "string",
						Label:       "To",
						Required:    true,
						Placeholder: "recipient@example.com",
					},
					{
						Name:  "cc",
						Type:  "string",
						Label: "CC",
					},
					{
						Name:  "bcc",
						Type:  "string",
						Label: "BCC",
					},
					{
						Name:     "subject",
						Type:     "string",
						Label:    "Subject",
						Required: true,
					},
					{
						Name:     "body",
						Type:     "text",
						Label:    "Body",
						Required: true,
					},
					{
						Name:    "html",
						Type:    "boolean",
						Label:   "HTML Email",
						Default: false,
					},
				},
			},
			Status:    "active",
			IsBuiltin: true,
		},
		// Transform nodes
		{
			ID:          uuid.New().String(),
			Type:        "code",
			Name:        "Code",
			Description: "Execute custom JavaScript code",
			Category:    "transform",
			Icon:        "code",
			Color:       "#636e72",
			Version:     "1.0.0",
			Schema: node.NodeSchema{
				Inputs: []node.SchemaField{
					{
						Name:        "code",
						Type:        "code",
						Label:       "JavaScript Code",
						Required:    true,
						Language:    "javascript",
						Placeholder: "// Access input data with $input\nreturn {\n  result: $input.value * 2\n}",
					},
				},
				Outputs: []node.SchemaField{
					{
						Name:  "result",
						Type:  "any",
						Label: "Result",
					},
				},
			},
			Status:    "active",
			IsBuiltin: true,
		},
		{
			ID:          uuid.New().String(),
			Type:        "filter",
			Name:        "Filter",
			Description: "Filter data based on conditions",
			Category:    "transform",
			Icon:        "filter",
			Color:       "#74b9ff",
			Version:     "1.0.0",
			Schema: node.NodeSchema{
				Inputs: []node.SchemaField{
					{
						Name:     "data",
						Type:     "array",
						Label:    "Input Data",
						Required: true,
					},
					{
						Name:        "condition",
						Type:        "code",
						Label:       "Filter Condition",
						Required:    true,
						Language:    "javascript",
						Placeholder: "// Return true to keep item\nreturn item.active === true",
					},
				},
				Outputs: []node.SchemaField{
					{
						Name:  "filtered",
						Type:  "array",
						Label: "Filtered Data",
					},
				},
			},
			Status:    "active",
			IsBuiltin: true,
		},
		// Control flow nodes
		{
			ID:          uuid.New().String(),
			Type:        "if-condition",
			Name:        "IF Condition",
			Description: "Conditional branching",
			Category:    "control",
			Icon:        "git-branch",
			Color:       "#fd79a8",
			Version:     "1.0.0",
			Schema: node.NodeSchema{
				Inputs: []node.SchemaField{
					{
						Name:        "condition",
						Type:        "code",
						Label:       "Condition",
						Required:    true,
						Language:    "javascript",
						Placeholder: "// Return true or false\nreturn $input.value > 100",
					},
				},
				Outputs: []node.SchemaField{
					{
						Name:  "true",
						Type:  "any",
						Label: "True Branch",
					},
					{
						Name:  "false",
						Type:  "any",
						Label: "False Branch",
					},
				},
			},
			Status:    "active",
			IsBuiltin: true,
		},
		{
			ID:          uuid.New().String(),
			Type:        "loop",
			Name:        "Loop",
			Description: "Iterate over arrays",
			Category:    "control",
			Icon:        "repeat",
			Color:       "#fab1a0",
			Version:     "1.0.0",
			Schema: node.NodeSchema{
				Inputs: []node.SchemaField{
					{
						Name:     "items",
						Type:     "array",
						Label:    "Items to Loop",
						Required: true,
					},
					{
						Name:    "batchSize",
						Type:    "number",
						Label:   "Batch Size",
						Default: 1,
					},
				},
			},
			Status:    "active",
			IsBuiltin: true,
		},
	}

	ctx := context.Background()
	for _, nodeType := range builtinNodes {
		r.nodesMux.Lock()
		r.nodes[nodeType.Type] = nodeType
		r.nodesMux.Unlock()

		// Save to database
		if err := r.repository.CreateNodeType(ctx, nodeType); err != nil {
			r.logger.Error("Failed to register builtin node",
				"type", nodeType.Type,
				"error", err,
			)
		}
	}

	r.logger.Info("Registered builtin nodes", "count", len(builtinNodes))
}

func (r *NodeRegistry) GetNodeType(nodeType string) (*node.NodeType, error) {
	r.nodesMux.RLock()
	defer r.nodesMux.RUnlock()

	if n, ok := r.nodes[nodeType]; ok {
		return n, nil
	}

	// Try to load from database
	ctx := context.Background()
	n, err := r.repository.GetNodeType(ctx, nodeType)
	if err != nil {
		return nil, fmt.Errorf("node type not found: %s", nodeType)
	}

	// Cache it
	r.nodesMux.Lock()
	r.nodes[nodeType] = n
	r.nodesMux.Unlock()

	return n, nil
}

func (r *NodeRegistry) GetAllNodeTypes() []*node.NodeType {
	r.nodesMux.RLock()
	defer r.nodesMux.RUnlock()

	nodeTypes := make([]*node.NodeType, 0, len(r.nodes))
	for _, n := range r.nodes {
		nodeTypes = append(nodeTypes, n)
	}

	return nodeTypes
}

func (r *NodeRegistry) RegisterNodeType(nodeType *node.NodeType) error {
	// Validate node type
	if err := nodeType.Validate(); err != nil {
		return fmt.Errorf("invalid node type: %w", err)
	}

	// Save to database
	ctx := context.Background()
	if err := r.repository.CreateNodeType(ctx, nodeType); err != nil {
		return fmt.Errorf("failed to save node type: %w", err)
	}

	// Add to registry
	r.nodesMux.Lock()
	r.nodes[nodeType.Type] = nodeType
	r.nodesMux.Unlock()

	// Cache in Redis
	data, _ := json.Marshal(nodeType)
	r.redis.Set(ctx, fmt.Sprintf("node:type:%s", nodeType.Type), data, 1*time.Hour)

	r.logger.Info("Registered node type", "type", nodeType.Type)
	return nil
}

func (r *NodeRegistry) UpdateNodeType(nodeType *node.NodeType) error {
	ctx := context.Background()

	// Update in database
	if err := r.repository.UpdateNodeType(ctx, nodeType); err != nil {
		return fmt.Errorf("failed to update node type: %w", err)
	}

	// Update in registry
	r.nodesMux.Lock()
	r.nodes[nodeType.Type] = nodeType
	r.nodesMux.Unlock()

	// Update cache
	data, _ := json.Marshal(nodeType)
	r.redis.Set(ctx, fmt.Sprintf("node:type:%s", nodeType.Type), data, 1*time.Hour)

	return nil
}

func (r *NodeRegistry) DeleteNodeType(nodeType string) error {
	ctx := context.Background()

	// Check if it's a builtin node
	r.nodesMux.RLock()
	if n, ok := r.nodes[nodeType]; ok && n.IsBuiltin {
		r.nodesMux.RUnlock()
		return fmt.Errorf("cannot delete builtin node type")
	}
	r.nodesMux.RUnlock()

	// Delete from database
	if err := r.repository.DeleteNodeType(ctx, nodeType); err != nil {
		return fmt.Errorf("failed to delete node type: %w", err)
	}

	// Remove from registry
	r.nodesMux.Lock()
	delete(r.nodes, nodeType)
	r.nodesMux.Unlock()

	// Remove from cache
	r.redis.Del(ctx, fmt.Sprintf("node:type:%s", nodeType))

	return nil
}

func (r *NodeRegistry) loadNodeTypes() error {
	ctx := context.Background()

	nodeTypes, err := r.repository.GetAllNodeTypes(ctx)
	if err != nil {
		return fmt.Errorf("failed to load node types: %w", err)
	}

	r.nodesMux.Lock()
	defer r.nodesMux.Unlock()

	for _, nodeType := range nodeTypes {
		r.nodes[nodeType.Type] = nodeType
	}

	r.logger.Info("Loaded node types", "count", len(nodeTypes))
	return nil
}

func (r *NodeRegistry) refreshNodeTypes() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := r.loadNodeTypes(); err != nil {
				r.logger.Error("Failed to refresh node types", "error", err)
			}
		case <-r.stopCh:
			return
		}
	}
}
