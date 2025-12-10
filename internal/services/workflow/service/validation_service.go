package service

import (
	"context"
	"fmt"
	"time"

	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// ValidationService handles workflow validation with caching
type ValidationService struct {
	redis  *redis.Client
	logger logger.Logger
}

// NewValidationService creates a new validation service
func NewValidationService(redis *redis.Client, logger logger.Logger) *ValidationService {
	return &ValidationService{
		redis:  redis,
		logger: logger,
	}
}

// ValidateWorkflow performs comprehensive workflow validation
func (vs *ValidationService) ValidateWorkflow(ctx context.Context, wf *workflow.Workflow) ([]string, []string, error) {
	startTime := time.Now()
	defer func() {
		vs.logger.Info("Workflow validation completed", 
			"workflow_id", wf.ID,
			"duration_ms", time.Since(startTime).Milliseconds())
	}()
	
	// Check cache for recent validation results
	cacheKey := fmt.Sprintf("validation:%s:v%d", wf.ID, wf.Version)
	if cached, err := vs.getValidationCache(ctx, cacheKey); err == nil && cached != nil {
		vs.logger.Debug("Using cached validation result", "workflow_id", wf.ID)
		return cached.Errors, cached.Warnings, nil
	}
	
	// Create validator
	validator := workflow.NewValidator(wf)
	
	// Perform validation
	errors, warnings, err := validator.Validate()
	
	// Log validation results
	if err != nil {
		vs.logger.Error("Workflow validation failed", 
			"workflow_id", wf.ID,
			"errors", len(errors),
			"warnings", len(warnings),
			"error", err)
	} else {
		vs.logger.Info("Workflow validation passed", 
			"workflow_id", wf.ID,
			"warnings", len(warnings))
	}
	
	// Cache validation results
	vs.cacheValidationResult(ctx, cacheKey, &ValidationResult{
		Errors:   errors,
		Warnings: warnings,
		Valid:    err == nil,
	})
	
	return errors, warnings, err
}

// ValidateDAG performs DAG-specific validation
func (vs *ValidationService) ValidateDAG(ctx context.Context, wf *workflow.Workflow) error {
	// Create DAG
	dag := workflow.NewDAG(wf)
	
	// Validate DAG structure
	if err := dag.Validate(); err != nil {
		vs.logger.Error("DAG validation failed", 
			"workflow_id", wf.ID,
			"error", err)
		return err
	}
	
	// Log DAG statistics
	topOrder, _ := dag.GetTopologicalOrder()
	criticalPath := dag.GetCriticalPath()
	
	vs.logger.Info("DAG validation successful",
		"workflow_id", wf.ID,
		"nodes", len(dag.Nodes),
		"start_nodes", len(dag.StartNodes),
		"end_nodes", len(dag.EndNodes),
		"topological_order", len(topOrder),
		"critical_path_length", len(criticalPath))
	
	return nil
}

// ValidateNode validates a single node configuration
func (vs *ValidationService) ValidateNode(ctx context.Context, node *workflow.Node) []string {
	errors := []string{}
	
	// Validate node type
	validTypes := map[string]bool{
		workflow.NodeTypeTrigger:     true,
		workflow.NodeTypeAction:      true,
		workflow.NodeTypeCondition:   true,
		workflow.NodeTypeLoop:        true,
		workflow.NodeTypeMerge:       true,
		workflow.NodeTypeSplit:       true,
		workflow.NodeTypeWebhook:     true,
		workflow.NodeTypeHTTPRequest: true,
		workflow.NodeTypeDatabase:    true,
		workflow.NodeTypeCode:        true,
		workflow.NodeTypeEmail:       true,
		workflow.NodeTypeSlack:       true,
	}
	
	if !validTypes[node.Type] {
		errors = append(errors, fmt.Sprintf("Invalid node type: %s", node.Type))
	}
	
	// Validate node-specific parameters
	switch node.Type {
	case workflow.NodeTypeHTTPRequest:
		errors = append(errors, vs.validateHTTPNode(node)...)
	case workflow.NodeTypeDatabase:
		errors = append(errors, vs.validateDatabaseNode(node)...)
	case workflow.NodeTypeEmail:
		errors = append(errors, vs.validateEmailNode(node)...)
	case workflow.NodeTypeSlack:
		errors = append(errors, vs.validateSlackNode(node)...)
	case workflow.NodeTypeCode:
		errors = append(errors, vs.validateCodeNode(node)...)
	}
	
	return errors
}

// validateHTTPNode validates HTTP request node parameters
func (vs *ValidationService) validateHTTPNode(node *workflow.Node) []string {
	errors := []string{}
	
	if node.Parameters == nil {
		return []string{"HTTP node missing parameters"}
	}
	
	// Check required fields
	if _, ok := node.Parameters["url"]; !ok {
		errors = append(errors, "HTTP node missing 'url' parameter")
	}
	
	if _, ok := node.Parameters["method"]; !ok {
		errors = append(errors, "HTTP node missing 'method' parameter")
	} else {
		method := node.Parameters["method"]
		validMethods := map[string]bool{
			"GET": true, "POST": true, "PUT": true, 
			"DELETE": true, "PATCH": true, "HEAD": true,
		}
		if methodStr, ok := method.(string); ok {
			if !validMethods[methodStr] {
				errors = append(errors, fmt.Sprintf("Invalid HTTP method: %s", methodStr))
			}
		}
	}
	
	// Validate timeout if present
	if timeout, ok := node.Parameters["timeout"]; ok {
		if timeoutInt, ok := timeout.(int); ok {
			if timeoutInt <= 0 || timeoutInt > 300 {
				errors = append(errors, fmt.Sprintf("Invalid timeout: %d (must be 1-300 seconds)", timeoutInt))
			}
		}
	}
	
	return errors
}

// validateDatabaseNode validates database node parameters
func (vs *ValidationService) validateDatabaseNode(node *workflow.Node) []string {
	errors := []string{}
	
	if node.Parameters == nil {
		return []string{"Database node missing parameters"}
	}
	
	// Check required fields
	requiredFields := []string{"operation", "table"}
	for _, field := range requiredFields {
		if _, ok := node.Parameters[field]; !ok {
			errors = append(errors, fmt.Sprintf("Database node missing '%s' parameter", field))
		}
	}
	
	// Validate operation type
	if op, ok := node.Parameters["operation"]; ok {
		validOps := map[string]bool{
			"select": true, "insert": true, "update": true, 
			"delete": true, "upsert": true,
		}
		if opStr, ok := op.(string); ok {
			if !validOps[opStr] {
				errors = append(errors, fmt.Sprintf("Invalid database operation: %s", opStr))
			}
		}
	}
	
	return errors
}

// validateEmailNode validates email node parameters
func (vs *ValidationService) validateEmailNode(node *workflow.Node) []string {
	errors := []string{}
	
	if node.Parameters == nil {
		return []string{"Email node missing parameters"}
	}
	
	// Check required fields
	requiredFields := []string{"to", "subject"}
	for _, field := range requiredFields {
		if _, ok := node.Parameters[field]; !ok {
			errors = append(errors, fmt.Sprintf("Email node missing '%s' parameter", field))
		}
	}
	
	// Validate email format if present
	if to, ok := node.Parameters["to"]; ok {
		if toStr, ok := to.(string); ok {
			// Basic email validation
			if len(toStr) < 3 || len(toStr) > 254 {
				errors = append(errors, "Invalid email address length")
			}
		}
	}
	
	return errors
}

// validateSlackNode validates Slack node parameters
func (vs *ValidationService) validateSlackNode(node *workflow.Node) []string {
	errors := []string{}
	
	if node.Parameters == nil {
		return []string{"Slack node missing parameters"}
	}
	
	// Check required fields
	if _, ok := node.Parameters["channel"]; !ok {
		errors = append(errors, "Slack node missing 'channel' parameter")
	}
	
	if _, ok := node.Parameters["message"]; !ok {
		errors = append(errors, "Slack node missing 'message' parameter")
	}
	
	return errors
}

// validateCodeNode validates code execution node parameters
func (vs *ValidationService) validateCodeNode(node *workflow.Node) []string {
	errors := []string{}
	
	if node.Parameters == nil {
		return []string{"Code node missing parameters"}
	}
	
	// Check required fields
	if _, ok := node.Parameters["code"]; !ok {
		errors = append(errors, "Code node missing 'code' parameter")
	}
	
	// Check language if specified
	if lang, ok := node.Parameters["language"]; ok {
		validLangs := map[string]bool{
			"javascript": true, "python": true, "go": true,
		}
		if langStr, ok := lang.(string); ok {
			if !validLangs[langStr] {
				errors = append(errors, fmt.Sprintf("Unsupported language: %s", langStr))
			}
		}
	}
	
	return errors
}

// ValidateConnection validates a connection between two nodes
func (vs *ValidationService) ValidateConnection(source, target *workflow.Node, conn *workflow.Connection) error {
	// Check if source can have outputs
	if source.Type == workflow.NodeTypeMerge {
		// Merge nodes typically have single output
		if conn.SourcePort != "" && conn.SourcePort != "output" {
			return fmt.Errorf("merge node can only use 'output' port")
		}
	}
	
	// Check if target can have inputs
	if target.Type == workflow.NodeTypeTrigger {
		return fmt.Errorf("trigger nodes cannot have incoming connections")
	}
	
	// Validate split node outputs
	if source.Type == workflow.NodeTypeSplit {
		validPorts := map[string]bool{"true": true, "false": true, "output": true}
		if !validPorts[conn.SourcePort] {
			return fmt.Errorf("split node has invalid output port: %s", conn.SourcePort)
		}
	}
	
	// Validate condition node outputs
	if source.Type == workflow.NodeTypeCondition {
		validPorts := map[string]bool{"true": true, "false": true}
		if !validPorts[conn.SourcePort] {
			return fmt.Errorf("condition node has invalid output port: %s", conn.SourcePort)
		}
	}
	
	return nil
}

// GetExecutionOrder returns the order in which nodes should be executed
func (vs *ValidationService) GetExecutionOrder(ctx context.Context, wf *workflow.Workflow) ([]string, error) {
	dag := workflow.NewDAG(wf)
	
	// Check for cycles first
	if dag.HasCycle() {
		return nil, workflow.ErrWorkflowHasCycle
	}
	
	// Get topological order
	order, err := dag.GetTopologicalOrder()
	if err != nil {
		return nil, err
	}
	
	vs.logger.Debug("Calculated execution order",
		"workflow_id", wf.ID,
		"nodes", len(order))
	
	return order, nil
}

// ValidationResult represents cached validation results
type ValidationResult struct {
	Errors   []string
	Warnings []string
	Valid    bool
}

// getValidationCache retrieves cached validation results
func (vs *ValidationService) getValidationCache(ctx context.Context, key string) (*ValidationResult, error) {
	// For now, return error to skip cache
	// In production, implement proper Redis caching
	return nil, fmt.Errorf("cache not implemented")
}

// cacheValidationResult caches validation results
func (vs *ValidationService) cacheValidationResult(ctx context.Context, key string, result *ValidationResult) {
	// In production, implement proper Redis caching with TTL
	// For now, just log
	vs.logger.Debug("Would cache validation result", "key", key, "valid", result.Valid)
}

// AnalyzeComplexity analyzes workflow complexity metrics
func (vs *ValidationService) AnalyzeComplexity(ctx context.Context, wf *workflow.Workflow) map[string]interface{} {
	dag := workflow.NewDAG(wf)
	
	// Calculate various complexity metrics
	topOrder, _ := dag.GetTopologicalOrder()
	criticalPath := dag.GetCriticalPath()
	executionPaths := dag.GetExecutionPaths()
	levels := dag.CalculateLevels()
	
	// Calculate max depth
	maxDepth := 0
	for _, level := range levels {
		if level > maxDepth {
			maxDepth = level
		}
	}
	
	// Count node types
	nodeTypes := make(map[string]int)
	for _, node := range wf.Nodes {
		nodeTypes[node.Type]++
	}
	
	// Calculate branching factor
	branchingFactor := 0
	for _, edges := range dag.Edges {
		if len(edges) > branchingFactor {
			branchingFactor = len(edges)
		}
	}
	
	metrics := map[string]interface{}{
		"total_nodes":       len(wf.Nodes),
		"total_connections": len(wf.Connections),
		"max_depth":         maxDepth,
		"critical_path_length": len(criticalPath),
		"execution_paths":   len(executionPaths),
		"branching_factor":  branchingFactor,
		"start_nodes":       len(dag.StartNodes),
		"end_nodes":         len(dag.EndNodes),
		"node_types":        nodeTypes,
		"has_loops":         nodeTypes[workflow.NodeTypeLoop] > 0,
		"has_conditions":    nodeTypes[workflow.NodeTypeCondition] > 0,
		"complexity_score":  calculateComplexityScore(len(wf.Nodes), len(wf.Connections), maxDepth),
	}
	
	vs.logger.Info("Workflow complexity analysis",
		"workflow_id", wf.ID,
		"metrics", metrics)
	
	return metrics
}

// calculateComplexityScore calculates a simple complexity score
func calculateComplexityScore(nodes, connections, depth int) int {
	// Simple formula: nodes + connections * 2 + depth * 3
	return nodes + connections*2 + depth*3
}
