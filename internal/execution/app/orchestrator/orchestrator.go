package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linkflow-go/internal/execution/ports"
	"github.com/linkflow-go/pkg/contracts/workflow"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// Orchestrator is the main workflow orchestrator
type Orchestrator struct {
	repository   ports.ExecutionRepository
	eventBus     events.EventBus
	redis        *redis.Client
	logger       logger.Logger
	executors    map[string]*WorkflowExecutor
	executorsMux sync.RWMutex
	pendingMux   sync.Mutex
	pending      map[string]chan map[string]interface{}
	stopCh       chan struct{}
}

// WorkflowOrchestrator is an alias for Orchestrator for backward compatibility
type WorkflowOrchestrator = Orchestrator

type WorkflowExecutor struct {
	workflow     *workflow.Workflow
	execution    *workflow.WorkflowExecution
	orchestrator *Orchestrator
	context      *ExecutionContext
	stateMachine *ExecutionStateMachine
	cancelFunc   context.CancelFunc
}

type ExecutionContext struct {
	ExecutionID string                 `json:"execution_id"`
	Variables   map[string]interface{} `json:"variables"`
	NodeOutputs map[string]interface{} `json:"node_outputs"`
	Errors      []ExecutionErrorDetail `json:"errors"`
	StartTime   time.Time              `json:"start_time"`
	Metadata    map[string]string      `json:"metadata"`
	mu          sync.RWMutex
}

type ExecutionErrorDetail struct {
	NodeID    string    `json:"node_id"`
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
	Retryable bool      `json:"retryable"`
}

func NewOrchestrator(repo ports.ExecutionRepository, eventBus events.EventBus, redis *redis.Client, logger logger.Logger) *Orchestrator {
	return &Orchestrator{
		repository: repo,
		eventBus:   eventBus,
		redis:      redis,
		logger:     logger,
		executors:  make(map[string]*WorkflowExecutor),
		pending:    make(map[string]chan map[string]interface{}),
		stopCh:     make(chan struct{}),
	}
}

func (o *Orchestrator) registerPending(requestID string) chan map[string]interface{} {
	o.pendingMux.Lock()
	defer o.pendingMux.Unlock()

	ch := make(chan map[string]interface{}, 1)
	o.pending[requestID] = ch
	return ch
}

func (o *Orchestrator) resolvePending(requestID string, result map[string]interface{}) {
	o.pendingMux.Lock()
	ch, ok := o.pending[requestID]
	if ok {
		delete(o.pending, requestID)
	}
	o.pendingMux.Unlock()

	if !ok {
		return
	}

	select {
	case ch <- result:
	default:
	}
}

func (o *Orchestrator) rejectPending(requestID string) {
	o.pendingMux.Lock()
	delete(o.pending, requestID)
	o.pendingMux.Unlock()
}

func (o *Orchestrator) HandleNodeExecuteResponse(ctx context.Context, event events.Event) error {
	_ = ctx

	reqID, _ := event.Payload["requestId"].(string)
	if reqID == "" {
		return fmt.Errorf("missing requestId in node.execute.response")
	}

	resAny, ok := event.Payload["result"]
	if !ok {
		return fmt.Errorf("missing result in node.execute.response")
	}

	res, ok := resAny.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid result type in node.execute.response")
	}

	o.resolvePending(reqID, res)
	return nil
}

func (o *Orchestrator) Start() {
	o.logger.Info("Starting workflow orchestrator")

	// Start background workers
	go o.monitorExecutions()
	go o.cleanupStaleExecutions()
}

func (o *Orchestrator) Stop() {
	o.logger.Info("Stopping workflow orchestrator")
	close(o.stopCh)

	// Cancel all running executions
	o.executorsMux.Lock()
	for _, executor := range o.executors {
		executor.cancelFunc()
	}
	o.executorsMux.Unlock()
}

func (o *Orchestrator) ExecuteWorkflow(ctx context.Context, workflowID string, inputData map[string]interface{}) (*workflow.WorkflowExecution, error) {
	// Get workflow
	wf, err := o.repository.GetWorkflow(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	// Validate workflow
	if !wf.IsActive {
		return nil, fmt.Errorf("workflow is not active")
	}

	// Create execution record
	execution := &workflow.WorkflowExecution{
		ID:         uuid.New().String(),
		WorkflowID: workflowID,
		Version:    wf.Version,
		Status:     string(workflow.ExecutionRunning),
		StartedAt:  time.Now(),
		Data:       inputData,
		CreatedAt:  time.Now(),
	}

	if err := o.repository.Create(ctx, execution); err != nil {
		return nil, fmt.Errorf("failed to create execution: %w", err)
	}

	// Publish execution started event
	event := events.NewEventBuilder(events.ExecutionStarted).
		WithAggregateID(execution.ID).
		WithAggregateType("execution").
		WithPayload("workflowId", workflowID).
		WithPayload("executionId", execution.ID).
		Build()

	if err := o.eventBus.Publish(ctx, event); err != nil {
		o.logger.Error("Failed to publish execution started event", "error", err)
	}

	// Create execution context
	execContext := &ExecutionContext{
		ExecutionID: execution.ID,
		Variables:   inputData,
		NodeOutputs: make(map[string]interface{}),
		Errors:      []ExecutionErrorDetail{},
		StartTime:   time.Now(),
		Metadata:    make(map[string]string),
	}

	// Create state machine
	stateMachine := NewExecutionStateMachine(
		execution.ID,
		workflowID,
		execContext,
		o.eventBus,
		o.logger,
	)

	// Create executor
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(wf.Settings.Timeout)*time.Second)
	executor := &WorkflowExecutor{
		workflow:     wf,
		execution:    execution,
		orchestrator: o,
		context:      execContext,
		stateMachine: stateMachine,
		cancelFunc:   cancel,
	}

	// Store executor
	o.executorsMux.Lock()
	o.executors[execution.ID] = executor
	o.executorsMux.Unlock()

	// Start execution in background
	go executor.Execute(execCtx)

	return execution, nil
}

func (e *WorkflowExecutor) Execute(ctx context.Context) {
	defer func() {
		// Clean up executor
		e.orchestrator.executorsMux.Lock()
		delete(e.orchestrator.executors, e.execution.ID)
		e.orchestrator.executorsMux.Unlock()

		// Cancel context
		e.cancelFunc()
	}()

	// Transition to running state
	if err := e.stateMachine.Transition(ctx, EventStart, nil); err != nil {
		e.orchestrator.logger.Error("Failed to transition to running state", "error", err)
		e.handleExecutionError(ctx, err)
		return
	}

	// Execute workflow nodes
	if err := e.executeNodes(ctx); err != nil {
		e.handleExecutionError(ctx, err)
		return
	}

	// Mark execution as completed
	e.completeExecution(ctx)
}

func (e *WorkflowExecutor) executeNodes(ctx context.Context) error {
	// Build execution graph
	graph := e.buildExecutionGraph()

	// Find starting nodes (triggers)
	startNodes := e.findStartNodes(graph)

	// Execute nodes in order
	executed := make(map[string]bool)
	queue := startNodes

	for len(queue) > 0 {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("execution cancelled")
		default:
		}

		nodeID := queue[0]
		queue = queue[1:]

		if executed[nodeID] {
			continue
		}

		// Execute node
		if err := e.executeNode(ctx, nodeID); err != nil {
			if e.workflow.Settings.ErrorHandling.ContinueOnFail {
				e.context.mu.Lock()
				e.context.Errors = append(e.context.Errors, ExecutionErrorDetail{
					NodeID:    nodeID,
					Error:     err.Error(),
					Timestamp: time.Now(),
					Retryable: false,
				})
				e.context.mu.Unlock()
			} else {
				return err
			}
		}

		executed[nodeID] = true

		// Add downstream nodes to queue
		for _, conn := range e.workflow.Connections {
			if conn.Source == nodeID && !executed[conn.Target] {
				queue = append(queue, conn.Target)
			}
		}
	}

	return nil
}

func (e *WorkflowExecutor) executeNode(ctx context.Context, nodeID string) error {
	// Find node
	var node *workflow.Node
	for _, n := range e.workflow.Nodes {
		if n.ID == nodeID {
			node = &n
			break
		}
	}

	if node == nil {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	// Skip disabled nodes
	if node.Disabled {
		return nil
	}

	// Create node execution record
	nodeExec := &workflow.NodeExecution{
		ID:          uuid.New().String(),
		ExecutionID: e.execution.ID,
		NodeID:      nodeID,
		Status:      string(workflow.NodeExecutionRunning),
		StartedAt:   time.Now(),
		InputData:   e.context.Variables,
	}

	if err := e.orchestrator.repository.CreateNodeExecution(ctx, nodeExec); err != nil {
		return fmt.Errorf("failed to create node execution: %w", err)
	}

	// Publish node execution started event
	event := events.NewEventBuilder(events.NodeExecutionStarted).
		WithAggregateID(nodeExec.ID).
		WithAggregateType("node_execution").
		WithPayload("executionId", e.execution.ID).
		WithPayload("nodeId", nodeID).
		WithPayload("nodeType", node.Type).
		Build()

	e.orchestrator.eventBus.Publish(ctx, event)

	// Execute node based on type
	outputData, err := e.executeNodeByType(ctx, node)

	// Update node execution
	finishedAt := time.Now()
	nodeExec.FinishedAt = &finishedAt

	if err != nil {
		nodeExec.Status = string(workflow.NodeExecutionFailed)
		nodeExec.Error = err.Error()

		// Retry if configured
		if node.RetryCount > 0 && nodeExec.RetryCount < node.RetryCount {
			nodeExec.RetryCount++
			time.Sleep(time.Second * 2) // Basic retry delay
			return e.executeNode(ctx, nodeID)
		}
	} else {
		nodeExec.Status = string(workflow.NodeExecutionCompleted)
		nodeExec.OutputData = outputData

		// Update execution context with output data
		e.context.mu.Lock()
		e.context.NodeOutputs[nodeID] = outputData
		// Merge output into variables for next nodes
		if outputData != nil {
			for k, v := range outputData {
				e.context.Variables[k] = v
			}
		}
		e.context.mu.Unlock()
	}

	e.orchestrator.repository.UpdateNodeExecution(ctx, nodeExec)

	// Publish node execution completed event
	event = events.NewEventBuilder(events.NodeExecutionCompleted).
		WithAggregateID(nodeExec.ID).
		WithAggregateType("node_execution").
		WithPayload("status", nodeExec.Status).
		Build()

	e.orchestrator.eventBus.Publish(ctx, event)

	return err
}

func (e *WorkflowExecutor) executeNodeByType(ctx context.Context, node *workflow.Node) (map[string]interface{}, error) {
	switch node.Type {
	case workflow.NodeTypeTrigger:
		return e.executeTriggerNode(ctx, node)
	case workflow.NodeTypeHTTPRequest:
		return e.executeHTTPNode(ctx, node)
	case workflow.NodeTypeCode:
		return e.executeCodeNode(ctx, node)
	case workflow.NodeTypeCondition:
		return e.executeConditionNode(ctx, node)
	case workflow.NodeTypeLoop:
		return e.executeLoopNode(ctx, node)
	default:
		// Send to executor service for processing
		return e.sendToExecutorService(ctx, node)
	}
}

func (e *WorkflowExecutor) executeTriggerNode(ctx context.Context, node *workflow.Node) (map[string]interface{}, error) {
	// Trigger nodes just pass through the input data
	e.context.mu.RLock()
	data := e.context.Variables
	e.context.mu.RUnlock()
	return data, nil
}

func (e *WorkflowExecutor) executeHTTPNode(ctx context.Context, node *workflow.Node) (map[string]interface{}, error) {
	// This would make actual HTTP requests
	// For now, return mock data
	return map[string]interface{}{
		"status": 200,
		"body":   "HTTP request executed",
	}, nil
}

func (e *WorkflowExecutor) executeCodeNode(ctx context.Context, node *workflow.Node) (map[string]interface{}, error) {
	// This would execute custom code in a sandbox
	// For now, return mock data
	return map[string]interface{}{
		"result": "Code executed successfully",
	}, nil
}

func (e *WorkflowExecutor) executeConditionNode(ctx context.Context, node *workflow.Node) (map[string]interface{}, error) {
	// Evaluate condition and determine next path
	e.context.mu.RLock()
	data := e.context.Variables
	e.context.mu.RUnlock()
	return data, nil
}

func (e *WorkflowExecutor) executeLoopNode(ctx context.Context, node *workflow.Node) (map[string]interface{}, error) {
	// Execute loop logic
	e.context.mu.RLock()
	data := e.context.Variables
	e.context.mu.RUnlock()
	return data, nil
}

func (e *WorkflowExecutor) sendToExecutorService(ctx context.Context, node *workflow.Node) (map[string]interface{}, error) {
	// Send node to executor service via event bus
	e.context.mu.RLock()
	inputData := e.context.Variables
	e.context.mu.RUnlock()

	requestID := uuid.New().String()
	ch := e.orchestrator.registerPending(requestID)
	defer e.orchestrator.rejectPending(requestID)

	event := events.NewEventBuilder("node.execute.request").
		WithAggregateID(e.execution.ID).
		WithPayload("requestId", requestID).
		WithPayload("nodeId", node.ID).
		WithPayload("nodeType", node.Type).
		WithPayload("parameters", node.Parameters).
		WithPayload("inputData", inputData).
		Build()

	if err := e.orchestrator.eventBus.Publish(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to send to executor service: %w", err)
	}

	// Wait for response
	select {
	case result := <-ch:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout waiting for node execution response")
	}
}

func (e *WorkflowExecutor) buildExecutionGraph() map[string][]string {
	graph := make(map[string][]string)
	for _, conn := range e.workflow.Connections {
		graph[conn.Source] = append(graph[conn.Source], conn.Target)
	}
	return graph
}

func (e *WorkflowExecutor) findStartNodes(graph map[string][]string) []string {
	var startNodes []string
	for _, node := range e.workflow.Nodes {
		if node.Type == workflow.NodeTypeTrigger {
			startNodes = append(startNodes, node.ID)
		}
	}
	return startNodes
}

func (e *WorkflowExecutor) handleExecutionError(ctx context.Context, err error) {
	// Transition to failed state
	metadata := map[string]interface{}{
		"error":     err.Error(),
		"timestamp": time.Now(),
	}

	if transErr := e.stateMachine.Transition(ctx, EventFail, metadata); transErr != nil {
		e.orchestrator.logger.Error("Failed to transition to failed state", "error", transErr)
	}

	e.execution.Status = string(workflow.ExecutionFailed)
	e.execution.Error = err.Error()
	finishedAt := time.Now()
	e.execution.FinishedAt = &finishedAt
	e.execution.ExecutionTime = int64(finishedAt.Sub(e.execution.StartedAt).Milliseconds())

	e.orchestrator.repository.Update(ctx, e.execution)

	// Publish execution failed event
	event := events.NewEventBuilder(events.ExecutionFailed).
		WithAggregateID(e.execution.ID).
		WithAggregateType("execution").
		WithPayload("error", err.Error()).
		Build()

	e.orchestrator.eventBus.Publish(ctx, event)
}

func (e *WorkflowExecutor) completeExecution(ctx context.Context) {
	// Transition to success state
	if err := e.stateMachine.Transition(ctx, EventComplete, nil); err != nil {
		e.orchestrator.logger.Error("Failed to transition to success state", "error", err)
	}

	e.execution.Status = string(workflow.ExecutionCompleted)
	finishedAt := time.Now()
	e.execution.FinishedAt = &finishedAt
	e.execution.ExecutionTime = int64(finishedAt.Sub(e.execution.StartedAt).Milliseconds())

	// Store final data
	e.context.mu.RLock()
	e.execution.Data = e.context.Variables
	e.context.mu.RUnlock()

	e.orchestrator.repository.Update(ctx, e.execution)

	// Publish execution completed event
	event := events.NewEventBuilder(events.ExecutionCompleted).
		WithAggregateID(e.execution.ID).
		WithAggregateType("execution").
		WithPayload("workflowId", e.workflow.ID).
		WithPayload("duration", e.execution.ExecutionTime).
		Build()

	e.orchestrator.eventBus.Publish(ctx, event)
}

func (o *Orchestrator) monitorExecutions() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			o.checkExecutionTimeouts()
		case <-o.stopCh:
			return
		}
	}
}

func (o *Orchestrator) checkExecutionTimeouts() {
	o.executorsMux.RLock()
	defer o.executorsMux.RUnlock()

	for id, executor := range o.executors {
		if time.Since(executor.execution.StartedAt) > time.Duration(executor.workflow.Settings.Timeout)*time.Second {
			o.logger.Warn("Execution timeout", "executionId", id)
			executor.cancelFunc()
		}
	}
}

func (o *Orchestrator) cleanupStaleExecutions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Clean up old execution data from Redis
			// This would implement actual cleanup logic
		case <-o.stopCh:
			return
		}
	}
}
