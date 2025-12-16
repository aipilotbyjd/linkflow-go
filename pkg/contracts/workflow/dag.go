package workflow

import (
	"fmt"
)

// DAG represents a Directed Acyclic Graph for workflow execution
type DAG struct {
	Nodes      map[string]*Node
	Edges      map[string][]string
	StartNodes []string
	EndNodes   []string
	workflow   *Workflow
}

// NewDAG creates a new DAG from a workflow
func NewDAG(workflow *Workflow) *DAG {
	dag := &DAG{
		Nodes:      make(map[string]*Node),
		Edges:      make(map[string][]string),
		StartNodes: []string{},
		EndNodes:   []string{},
		workflow:   workflow,
	}

	dag.build()
	return dag
}

// build constructs the DAG from the workflow
func (d *DAG) build() {
	// Build node map
	for i := range d.workflow.Nodes {
		node := &d.workflow.Nodes[i]
		d.Nodes[node.ID] = node
	}

	// Build edges from connections
	incoming := make(map[string]int)
	outgoing := make(map[string]int)

	for _, conn := range d.workflow.Connections {
		d.Edges[conn.Source] = append(d.Edges[conn.Source], conn.Target)
		outgoing[conn.Source]++
		incoming[conn.Target]++
	}

	// Identify start nodes (no incoming edges or trigger nodes)
	for nodeID, node := range d.Nodes {
		if incoming[nodeID] == 0 || node.Type == NodeTypeTrigger || node.Type == NodeTypeWebhook {
			d.StartNodes = append(d.StartNodes, nodeID)
		}
	}

	// Identify end nodes (no outgoing edges)
	for nodeID := range d.Nodes {
		if outgoing[nodeID] == 0 {
			d.EndNodes = append(d.EndNodes, nodeID)
		}
	}
}

// Validate performs comprehensive DAG validation
func (d *DAG) Validate() error {
	// Check for cycles
	if d.HasCycle() {
		return ErrWorkflowHasCycle
	}

	// Validate all connections
	for source, targets := range d.Edges {
		if _, ok := d.Nodes[source]; !ok {
			return fmt.Errorf("source node %s not found in DAG", source)
		}

		for _, target := range targets {
			if _, ok := d.Nodes[target]; !ok {
				return fmt.Errorf("target node %s not found in DAG", target)
			}
		}
	}

	// Ensure start and end nodes exist
	if len(d.StartNodes) == 0 {
		return fmt.Errorf("DAG has no start nodes")
	}

	if len(d.EndNodes) == 0 {
		return fmt.Errorf("DAG has no end nodes")
	}

	// Check for unreachable nodes
	unreachable := d.FindUnreachableNodes()
	if len(unreachable) > 0 {
		return fmt.Errorf("DAG contains unreachable nodes: %v", unreachable)
	}

	// Check node dependencies
	if err := d.ValidateNodeDependencies(); err != nil {
		return err
	}

	return nil
}

// HasCycle detects if the DAG contains any cycles
func (d *DAG) HasCycle() bool {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var hasCycleDFS func(nodeID string) bool
	hasCycleDFS = func(nodeID string) bool {
		visited[nodeID] = true
		recStack[nodeID] = true

		for _, neighbor := range d.Edges[nodeID] {
			if !visited[neighbor] {
				if hasCycleDFS(neighbor) {
					return true
				}
			} else if recStack[neighbor] {
				return true
			}
		}

		recStack[nodeID] = false
		return false
	}

	for nodeID := range d.Nodes {
		if !visited[nodeID] {
			if hasCycleDFS(nodeID) {
				return true
			}
		}
	}

	return false
}

// FindUnreachableNodes finds nodes that cannot be reached from any start node
func (d *DAG) FindUnreachableNodes() []string {
	reachable := make(map[string]bool)

	// BFS from all start nodes
	queue := append([]string{}, d.StartNodes...)
	for _, nodeID := range queue {
		reachable[nodeID] = true
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, neighbor := range d.Edges[current] {
			if !reachable[neighbor] {
				reachable[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}

	// Find unreachable nodes
	unreachable := []string{}
	for nodeID := range d.Nodes {
		if !reachable[nodeID] {
			// Skip disabled nodes
			if d.Nodes[nodeID].Disabled {
				continue
			}
			unreachable = append(unreachable, nodeID)
		}
	}

	return unreachable
}

// ValidateNodeDependencies checks if all node input requirements are satisfied
func (d *DAG) ValidateNodeDependencies() error {
	// Calculate incoming edges for each node
	incoming := make(map[string]int)
	for _, targets := range d.Edges {
		for _, target := range targets {
			incoming[target]++
		}
	}

	// Validate each node's dependencies
	for nodeID, node := range d.Nodes {
		// Skip trigger nodes and disabled nodes
		if node.Type == NodeTypeTrigger || node.Type == NodeTypeWebhook || node.Disabled {
			continue
		}

		// Check if non-trigger nodes have inputs
		if incoming[nodeID] == 0 {
			return fmt.Errorf("node %s (%s) has no incoming connections", nodeID, node.Name)
		}

		// Special validation for specific node types
		switch node.Type {
		case NodeTypeMerge:
			if incoming[nodeID] < 2 {
				return fmt.Errorf("merge node %s requires at least 2 inputs, has %d", nodeID, incoming[nodeID])
			}
		case NodeTypeCondition:
			// Condition nodes should have at least 2 outgoing edges (true/false branches)
			if len(d.Edges[nodeID]) < 2 {
				return fmt.Errorf("condition node %s should have at least 2 output branches", nodeID)
			}
		}
	}

	return nil
}

// GetTopologicalOrder returns nodes in topological sort order
func (d *DAG) GetTopologicalOrder() ([]string, error) {
	// Calculate in-degree for each node
	inDegree := make(map[string]int)
	for nodeID := range d.Nodes {
		inDegree[nodeID] = 0
	}

	for _, targets := range d.Edges {
		for _, target := range targets {
			inDegree[target]++
		}
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
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Process all neighbors
		for _, neighbor := range d.Edges[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// Check if all nodes were processed
	if len(result) != len(d.Nodes) {
		return nil, ErrWorkflowHasCycle
	}

	return result, nil
}

// GetExecutionPaths returns all possible execution paths through the DAG
func (d *DAG) GetExecutionPaths() [][]string {
	paths := [][]string{}

	var dfs func(nodeID string, currentPath []string)
	dfs = func(nodeID string, currentPath []string) {
		currentPath = append(currentPath, nodeID)

		// If this is an end node, save the path
		neighbors := d.Edges[nodeID]
		if len(neighbors) == 0 {
			pathCopy := make([]string, len(currentPath))
			copy(pathCopy, currentPath)
			paths = append(paths, pathCopy)
			return
		}

		// Continue DFS for all neighbors
		for _, neighbor := range neighbors {
			dfs(neighbor, currentPath)
		}
	}

	// Start DFS from all start nodes
	for _, startNode := range d.StartNodes {
		dfs(startNode, []string{})
	}

	return paths
}

// GetNodeLevel returns the level (depth) of a node in the DAG
func (d *DAG) GetNodeLevel(nodeID string) int {
	levels := d.CalculateLevels()
	if level, ok := levels[nodeID]; ok {
		return level
	}
	return -1
}

// CalculateLevels calculates the level (depth) for each node
func (d *DAG) CalculateLevels() map[string]int {
	levels := make(map[string]int)

	// Initialize all nodes to level -1
	for nodeID := range d.Nodes {
		levels[nodeID] = -1
	}

	// BFS to calculate levels
	queue := []string{}
	for _, startNode := range d.StartNodes {
		levels[startNode] = 0
		queue = append(queue, startNode)
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		currentLevel := levels[current]

		for _, neighbor := range d.Edges[current] {
			// Update level if not set or if we found a longer path
			if levels[neighbor] < currentLevel+1 {
				levels[neighbor] = currentLevel + 1
				queue = append(queue, neighbor)
			}
		}
	}

	return levels
}

// GetCriticalPath returns the longest path through the DAG
func (d *DAG) GetCriticalPath() []string {
	// Get topological order
	order, err := d.GetTopologicalOrder()
	if err != nil {
		return []string{}
	}

	// Calculate longest path to each node
	dist := make(map[string]int)
	parent := make(map[string]string)

	for _, nodeID := range order {
		// Initialize start nodes
		isStart := false
		for _, startNode := range d.StartNodes {
			if nodeID == startNode {
				dist[nodeID] = 1
				isStart = true
				break
			}
		}

		if !isStart && dist[nodeID] == 0 {
			dist[nodeID] = 1
		}

		// Update distances for neighbors
		for _, neighbor := range d.Edges[nodeID] {
			if dist[nodeID]+1 > dist[neighbor] {
				dist[neighbor] = dist[nodeID] + 1
				parent[neighbor] = nodeID
			}
		}
	}

	// Find the end node with maximum distance
	maxDist := 0
	endNode := ""
	for _, nodeID := range d.EndNodes {
		if dist[nodeID] > maxDist {
			maxDist = dist[nodeID]
			endNode = nodeID
		}
	}

	// Reconstruct the critical path
	path := []string{}
	current := endNode
	for current != "" {
		path = append([]string{current}, path...)
		current = parent[current]
	}

	return path
}

// IsConnected checks if there's a path between two nodes
func (d *DAG) IsConnected(source, target string) bool {
	if source == target {
		return true
	}

	visited := make(map[string]bool)
	queue := []string{source}
	visited[source] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, neighbor := range d.Edges[current] {
			if neighbor == target {
				return true
			}

			if !visited[neighbor] {
				visited[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}

	return false
}

// GetAncestors returns all ancestor nodes of a given node
func (d *DAG) GetAncestors(nodeID string) []string {
	ancestors := []string{}
	visited := make(map[string]bool)

	// Build reverse edges
	reverseEdges := make(map[string][]string)
	for source, targets := range d.Edges {
		for _, target := range targets {
			reverseEdges[target] = append(reverseEdges[target], source)
		}
	}

	// DFS on reverse edges
	var dfs func(current string)
	dfs = func(current string) {
		for _, ancestor := range reverseEdges[current] {
			if !visited[ancestor] {
				visited[ancestor] = true
				ancestors = append(ancestors, ancestor)
				dfs(ancestor)
			}
		}
	}

	dfs(nodeID)
	return ancestors
}

// GetDescendants returns all descendant nodes of a given node
func (d *DAG) GetDescendants(nodeID string) []string {
	descendants := []string{}
	visited := make(map[string]bool)

	var dfs func(current string)
	dfs = func(current string) {
		for _, descendant := range d.Edges[current] {
			if !visited[descendant] {
				visited[descendant] = true
				descendants = append(descendants, descendant)
				dfs(descendant)
			}
		}
	}

	dfs(nodeID)
	return descendants
}
