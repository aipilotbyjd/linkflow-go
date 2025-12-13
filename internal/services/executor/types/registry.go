package types

import (
	"fmt"
	"sync"
	"time"

	"github.com/linkflow-go/pkg/logger"
)

// NodeRegistry manages all available node types
type NodeRegistry struct {
	executors map[string]NodeExecutor
	mu        sync.RWMutex
	logger    logger.Logger
}

var globalRegistry *NodeRegistry
var once sync.Once

// GetRegistry returns the global node registry
func GetRegistry() *NodeRegistry {
	once.Do(func() {
		globalRegistry = NewNodeRegistry(nil)
		globalRegistry.RegisterBuiltinNodes()
	})
	return globalRegistry
}

// NewNodeRegistry creates a new node registry
func NewNodeRegistry(log logger.Logger) *NodeRegistry {
	return &NodeRegistry{
		executors: make(map[string]NodeExecutor),
		logger:    log,
	}
}

// Register adds a node executor to the registry
func (r *NodeRegistry) Register(nodeType string, executor NodeExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executors[nodeType] = executor
}

// Get returns an executor for a node type
func (r *NodeRegistry) Get(nodeType string) (NodeExecutor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	executor, ok := r.executors[nodeType]
	if !ok {
		return nil, fmt.Errorf("unknown node type: %s", nodeType)
	}

	return executor, nil
}

// List returns all registered node types
func (r *NodeRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.executors))
	for t := range r.executors {
		types = append(types, t)
	}
	return types
}

// RegisterBuiltinNodes registers all built-in node types
func (r *NodeRegistry) RegisterBuiltinNodes() {
	// HTTP nodes
	httpExecutor := NewHTTPNodeExecutor(r.logger)
	r.Register("http", httpExecutor)
	r.Register("httpRequest", httpExecutor)

	// Database nodes
	dbExecutor := NewDatabaseNodeExecutor(r.logger)
	r.Register("database", dbExecutor)
	r.Register("postgres", dbExecutor)
	r.Register("mysql", dbExecutor)

	// Transform nodes
	transformExecutor := NewTransformNodeExecutor()
	r.Register("transform", transformExecutor)

	// Conditional nodes
	r.Register("if", NewConditionalNodeExecutor())
	r.Register("conditional", NewConditionalNodeExecutor())
	r.Register("switch", NewSwitchNodeExecutor())

	// Loop nodes
	r.Register("loop", NewLoopNodeExecutor())
	r.Register("forEach", NewForEachNodeExecutor())
	r.Register("while", NewWhileNodeExecutor())
	r.Register("split", NewSplitNodeExecutor())
	r.Register("merge", NewMergeNodeExecutor())
	r.Register("aggregate", NewAggregateNodeExecutor())

	// Utility nodes
	r.Register("set", NewSetNodeExecutor())
	r.Register("function", NewFunctionNodeExecutor())
	r.Register("wait", NewWaitNodeExecutor())
	r.Register("dateTime", NewDateTimeNodeExecutor())
	r.Register("crypto", NewCryptoNodeExecutor())
	r.Register("json", NewJSONNodeExecutor())
	r.Register("math", NewMathNodeExecutor())
	r.Register("text", NewTextNodeExecutor())

	// Integration nodes
	r.Register("email", NewEmailNodeExecutor())
	r.Register("slack", NewSlackNodeExecutor())
	r.Register("discord", NewDiscordNodeExecutor())
	r.Register("telegram", NewTelegramNodeExecutor())

	// Trigger nodes
	r.Register("webhookTrigger", NewTriggerNodeExecutor("webhookTrigger"))
	r.Register("scheduleTrigger", NewTriggerNodeExecutor("scheduleTrigger"))
	r.Register("manualTrigger", NewTriggerNodeExecutor("manualTrigger"))
	r.Register("errorTrigger", NewTriggerNodeExecutor("errorTrigger"))

	// Response nodes
	r.Register("respondToWebhook", NewRespondToWebhookExecutor())

	// Control nodes
	r.Register("stopAndError", NewStopAndErrorExecutor())
	r.Register("noOp", NewNoOpExecutor())
}

// BaseNodeExecutor provides common functionality for node executors
type BaseNodeExecutor struct {
	timeout time.Duration
}

func (b *BaseNodeExecutor) GetTimeout() time.Duration {
	if b.timeout == 0 {
		return 30 * time.Second
	}
	return b.timeout
}

func (b *BaseNodeExecutor) ValidateInput(node Node, input map[string]interface{}) error {
	return nil // Default: no validation
}
