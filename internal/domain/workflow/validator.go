package workflow

import (
	"errors"
	"fmt"
)

var (
	ErrWorkflowHasCycle       = errors.New("workflow contains a cycle")
	ErrInvalidConnection      = errors.New("invalid connection: node not found")
	ErrNoTriggerNode          = errors.New("workflow must have at least one trigger node")
	ErrOrphanedNode           = errors.New("workflow contains orphaned nodes")
	ErrInvalidNodeType        = errors.New("invalid node type")
	ErrDuplicateNodeID        = errors.New("duplicate node ID found")
	ErrInvalidPortConnection  = errors.New("invalid port connection")
	ErrMissingRequiredInputs  = errors.New("node is missing required inputs")
)

// Validator provides comprehensive workflow validation
type Validator struct {
	workflow *Workflow
	nodeMap  map[string]*Node
	errors   []string
	warnings []string
}

// NewValidator creates a new workflow validator
func NewValidator(workflow *Workflow) *Validator {
	return &Validator{
		workflow: workflow,
		nodeMap:  make(map[string]*Node),
		errors:   []string{},
		warnings: []string{},
	}
}

// Validate performs complete workflow validation
func (v *Validator) Validate() ([]string, []string, error) {
	// Reset errors and warnings
	v.errors = []string{}
	v.warnings = []string{}
	
	// Build node map and check for duplicates
	if err := v.buildNodeMap(); err != nil {
		v.errors = append(v.errors, err.Error())
		return v.errors, v.warnings, err
	}
	
	// Check for required trigger node
	if err := v.validateTriggerExists(); err != nil {
		v.errors = append(v.errors, err.Error())
	}
	
	// Validate all connections
	if err := v.validateConnections(); err != nil {
		v.errors = append(v.errors, err.Error())
	}
	
	// Check for cycles
	if err := v.validateNoCycles(); err != nil {
		v.errors = append(v.errors, err.Error())
	}
	
	// Check for orphaned nodes
	if err := v.validateNoOrphanedNodes(); err != nil {
		v.warnings = append(v.warnings, err.Error())
	}
	
	// Validate node configurations
	v.validateNodeConfigurations()
	
	// Validate node dependencies and schemas
	v.validateNodeDependencies()
	
	if len(v.errors) > 0 {
		return v.errors, v.warnings, fmt.Errorf("validation failed with %d errors", len(v.errors))
	}
	
	return v.errors, v.warnings, nil
}

// buildNodeMap creates a map of nodes and checks for duplicates
func (v *Validator) buildNodeMap() error {
	for i := range v.workflow.Nodes {
		node := &v.workflow.Nodes[i]
		if _, exists := v.nodeMap[node.ID]; exists {
			return fmt.Errorf("%w: %s", ErrDuplicateNodeID, node.ID)
		}
		v.nodeMap[node.ID] = node
	}
	return nil
}

// validateTriggerExists ensures at least one trigger node exists
func (v *Validator) validateTriggerExists() error {
	for _, node := range v.workflow.Nodes {
		if node.Type == NodeTypeTrigger || node.Type == NodeTypeWebhook {
			return nil
		}
	}
	return ErrNoTriggerNode
}

// validateConnections validates all workflow connections
func (v *Validator) validateConnections() error {
	for _, conn := range v.workflow.Connections {
		// Check source node exists
		sourceNode, sourceExists := v.nodeMap[conn.Source]
		if !sourceExists {
			return fmt.Errorf("%w: source node '%s' not found", ErrInvalidConnection, conn.Source)
		}
		
		// Check target node exists
		targetNode, targetExists := v.nodeMap[conn.Target]
		if !targetExists {
			return fmt.Errorf("%w: target node '%s' not found", ErrInvalidConnection, conn.Target)
		}
		
		// Validate port compatibility
		if err := v.validatePortCompatibility(sourceNode, targetNode, &conn); err != nil {
			v.warnings = append(v.warnings, fmt.Sprintf("Connection %s: %v", conn.ID, err))
		}
	}
	return nil
}

// validateNoCycles uses DFS to detect cycles in the workflow
func (v *Validator) validateNoCycles() error {
	// Build adjacency list
	graph := make(map[string][]string)
	for _, conn := range v.workflow.Connections {
		graph[conn.Source] = append(graph[conn.Source], conn.Target)
	}
	
	// Track visited and recursion stack
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	
	// DFS helper function
	var hasCycleDFS func(nodeID string) bool
	hasCycleDFS = func(nodeID string) bool {
		visited[nodeID] = true
		recStack[nodeID] = true
		
		// Check all neighbors
		for _, neighbor := range graph[nodeID] {
			if !visited[neighbor] {
				if hasCycleDFS(neighbor) {
					return true
				}
			} else if recStack[neighbor] {
				// Found a back edge (cycle)
				return true
			}
		}
		
		recStack[nodeID] = false
		return false
	}
	
	// Check all nodes
	for nodeID := range v.nodeMap {
		if !visited[nodeID] {
			if hasCycleDFS(nodeID) {
				return ErrWorkflowHasCycle
			}
		}
	}
	
	return nil
}

// validateNoOrphanedNodes checks for nodes without any connections
func (v *Validator) validateNoOrphanedNodes() error {
	connected := make(map[string]bool)
	
	// Mark all connected nodes
	for _, conn := range v.workflow.Connections {
		connected[conn.Source] = true
		connected[conn.Target] = true
	}
	
	// Find orphaned nodes
	orphaned := []string{}
	for nodeID, node := range v.nodeMap {
		// Trigger nodes don't need incoming connections
		if node.Type == NodeTypeTrigger || node.Type == NodeTypeWebhook {
			continue
		}
		
		if !connected[nodeID] {
			orphaned = append(orphaned, nodeID)
		}
	}
	
	if len(orphaned) > 0 {
		return fmt.Errorf("%w: nodes %v have no connections", ErrOrphanedNode, orphaned)
	}
	
	return nil
}

// validateNodeConfigurations validates individual node configurations
func (v *Validator) validateNodeConfigurations() {
	validTypes := map[string]bool{
		NodeTypeTrigger:     true,
		NodeTypeAction:      true,
		NodeTypeCondition:   true,
		NodeTypeLoop:        true,
		NodeTypeMerge:       true,
		NodeTypeSplit:       true,
		NodeTypeWebhook:     true,
		NodeTypeHTTPRequest: true,
		NodeTypeDatabase:    true,
		NodeTypeCode:        true,
		NodeTypeEmail:       true,
		NodeTypeSlack:       true,
	}
	
	for _, node := range v.workflow.Nodes {
		// Validate node type
		if !validTypes[node.Type] {
			v.errors = append(v.errors, fmt.Sprintf("Node %s has invalid type: %s", node.ID, node.Type))
		}
		
		// Validate node-specific parameters
		switch node.Type {
		case NodeTypeHTTPRequest:
			v.validateHTTPNode(node)
		case NodeTypeDatabase:
			v.validateDatabaseNode(node)
		case NodeTypeEmail:
			v.validateEmailNode(node)
		}
		
		// Check timeout values
		if node.Timeout < 0 {
			v.warnings = append(v.warnings, fmt.Sprintf("Node %s has negative timeout: %d", node.ID, node.Timeout))
		}
		
		// Check retry count
		if node.RetryCount < 0 {
			v.warnings = append(v.warnings, fmt.Sprintf("Node %s has negative retry count: %d", node.ID, node.RetryCount))
		}
	}
}

// validateHTTPNode validates HTTP request node parameters
func (v *Validator) validateHTTPNode(node *Node) {
	if node.Parameters == nil {
		v.errors = append(v.errors, fmt.Sprintf("HTTP node %s missing parameters", node.ID))
		return
	}
	
	// Check for required fields
	if _, ok := node.Parameters["url"]; !ok {
		v.errors = append(v.errors, fmt.Sprintf("HTTP node %s missing 'url' parameter", node.ID))
	}
	
	if method, ok := node.Parameters["method"]; ok {
		validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true}
		if methodStr, isString := method.(string); isString {
			if !validMethods[methodStr] {
				v.warnings = append(v.warnings, fmt.Sprintf("HTTP node %s has non-standard method: %s", node.ID, methodStr))
			}
		}
	}
}

// validateDatabaseNode validates database node parameters
func (v *Validator) validateDatabaseNode(node *Node) {
	if node.Parameters == nil {
		v.errors = append(v.errors, fmt.Sprintf("Database node %s missing parameters", node.ID))
		return
	}
	
	// Check for required fields
	requiredFields := []string{"operation", "table"}
	for _, field := range requiredFields {
		if _, ok := node.Parameters[field]; !ok {
			v.errors = append(v.errors, fmt.Sprintf("Database node %s missing '%s' parameter", node.ID, field))
		}
	}
}

// validateEmailNode validates email node parameters
func (v *Validator) validateEmailNode(node *Node) {
	if node.Parameters == nil {
		v.errors = append(v.errors, fmt.Sprintf("Email node %s missing parameters", node.ID))
		return
	}
	
	// Check for required fields
	requiredFields := []string{"to", "subject"}
	for _, field := range requiredFields {
		if _, ok := node.Parameters[field]; !ok {
			v.errors = append(v.errors, fmt.Sprintf("Email node %s missing '%s' parameter", node.ID, field))
		}
	}
}

// validateNodeDependencies checks if all node inputs are satisfied
func (v *Validator) validateNodeDependencies() {
	// Build incoming connections map
	incoming := make(map[string][]string)
	for _, conn := range v.workflow.Connections {
		incoming[conn.Target] = append(incoming[conn.Target], conn.Source)
	}
	
	// Check each node's dependencies
	for nodeID, node := range v.nodeMap {
		// Skip trigger nodes
		if node.Type == NodeTypeTrigger || node.Type == NodeTypeWebhook {
			continue
		}
		
		// Check if node has required inputs
		if len(incoming[nodeID]) == 0 && !node.Disabled {
			v.warnings = append(v.warnings, fmt.Sprintf("Node %s (%s) has no incoming connections", nodeID, node.Name))
		}
		
		// Special validation for merge nodes
		if node.Type == NodeTypeMerge && len(incoming[nodeID]) < 2 {
			v.warnings = append(v.warnings, fmt.Sprintf("Merge node %s should have at least 2 inputs, has %d", nodeID, len(incoming[nodeID])))
		}
	}
}

// validatePortCompatibility checks if source and target ports are compatible
func (v *Validator) validatePortCompatibility(source, target *Node, conn *Connection) error {
	// Basic port validation
	if conn.SourcePort == "" {
		conn.SourcePort = "output" // Default port
	}
	if conn.TargetPort == "" {
		conn.TargetPort = "input" // Default port
	}
	
	// Check for split/merge node special cases
	if source.Type == NodeTypeSplit && conn.SourcePort != "output" && conn.SourcePort != "true" && conn.SourcePort != "false" {
		return fmt.Errorf("split node %s has invalid output port: %s", source.ID, conn.SourcePort)
	}
	
	return nil
}

// GetTopologicalOrder returns nodes in topological order (for execution)
func (v *Validator) GetTopologicalOrder() ([]string, error) {
	// Build adjacency list and in-degree map
	graph := make(map[string][]string)
	inDegree := make(map[string]int)
	
	// Initialize in-degree for all nodes
	for nodeID := range v.nodeMap {
		inDegree[nodeID] = 0
	}
	
	// Build graph and calculate in-degrees
	for _, conn := range v.workflow.Connections {
		graph[conn.Source] = append(graph[conn.Source], conn.Target)
		inDegree[conn.Target]++
	}
	
	// Find all nodes with no incoming edges
	queue := []string{}
	for nodeID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, nodeID)
		}
	}
	
	// Process nodes in topological order
	result := []string{}
	for len(queue) > 0 {
		// Dequeue
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)
		
		// Process neighbors
		for _, neighbor := range graph[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}
	
	// Check if all nodes were processed (no cycle)
	if len(result) != len(v.nodeMap) {
		return nil, ErrWorkflowHasCycle
	}
	
	return result, nil
}
