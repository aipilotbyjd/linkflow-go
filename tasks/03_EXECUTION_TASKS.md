# 🚀 Execution Engine Tasks

## Prerequisites
- Workflow service operational (WORK-001, WORK-002)
- Node registry available (NODE-001)
- Event bus configured (EVENT-001)
- Message queue ready (INFRA-002)

---

## EXEC-001: Implement Workflow Executor Core
**Priority**: P0 | **Hours**: 6 | **Dependencies**: WORK-001

### Context
The core execution engine that orchestrates workflow runs, manages state, and coordinates node execution.

### Implementation
**Files to modify:**
- `internal/services/execution/orchestrator/orchestrator.go`
- `internal/services/execution/orchestrator/state_machine.go`
- `internal/domain/workflow/execution.go`

### Steps
1. Implement execution state machine:
```go
type ExecutionStateMachine struct {
    ID         string
    WorkflowID string
    State      ExecutionState
    Context    *ExecutionContext
    History    []StateTransition
}

func (sm *ExecutionStateMachine) Transition(event ExecutionEvent) error {
    // Validate transition
    // Update state
    // Record history
    // Publish event
}
```

2. States: Pending → Running → (Success|Failed|Cancelled)
3. Handle node execution order
4. Implement parallel execution
5. Error handling and retry logic

### Execution Context:
```go
type ExecutionContext struct {
    ExecutionID string
    Variables   map[string]interface{}
    NodeOutputs map[string]interface{}
    Errors      []ExecutionError
    StartTime   time.Time
    Metadata    map[string]string
}
```

### Testing
```bash
# Start execution
curl -X POST localhost:8084/api/executions \
  -d '{"workflow_id":"wf_123","input_data":{"key":"value"}}'

# Get execution status
curl -X GET localhost:8084/api/executions/<exec_id>

# Cancel execution
curl -X POST localhost:8084/api/executions/<exec_id>/cancel
```

### Acceptance Criteria
- ✅ State transitions work correctly
- ✅ Parallel nodes execute simultaneously
- ✅ Error handling works
- ✅ Execution context maintained
- ✅ Events published for state changes

---

## EXEC-002: Implement Node Executor
**Priority**: P0 | **Hours**: 5 | **Dependencies**: NODE-001

### Context
Execute individual nodes within a workflow, handling different node types and their specific requirements.

### Implementation
**Files to modify:**
- `internal/services/executor/worker/node_executor.go`
- `internal/services/executor/worker/sandbox.go`
- `internal/services/executor/types/` (create node type handlers)

### Steps
1. Create node executor interface:
```go
type NodeExecutor interface {
    Execute(ctx context.Context, node Node, input map[string]interface{}) (output map[string]interface{}, err error)
    ValidateInput(node Node, input map[string]interface{}) error
    GetTimeout() time.Duration
}
```

2. Implement node types:
   - HTTP Request node
   - Database Query node
   - Transform node
   - Condition node
   - Code execution node

3. Input/output mapping
4. Error handling per node
5. Timeout management

### Node Execution Example:
```go
func (e *HTTPNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
    config := node.Config.(HTTPNodeConfig)
    
    // Build request
    req, err := http.NewRequestWithContext(ctx, config.Method, config.URL, nil)
    
    // Add headers
    for k, v := range config.Headers {
        req.Header.Set(k, v)
    }
    
    // Execute request
    resp, err := e.client.Do(req)
    
    // Parse response
    return parseResponse(resp)
}
```

### Acceptance Criteria
- ✅ All node types execute correctly
- ✅ Input validation works
- ✅ Output mapping correct
- ✅ Timeouts enforced
- ✅ Errors handled gracefully

---

## EXEC-003: Implement Execution Queue Management
**Priority**: P0 | **Hours**: 4 | **Dependencies**: INFRA-002

### Context
Manage execution queue with priorities, scheduling, and worker pool management.

### Implementation
**Files to modify:**
- `internal/services/execution/queue/manager.go` (create)
- `internal/services/execution/queue/priority_queue.go` (create)
- `internal/services/executor/worker/pool.go`

### Steps
1. Implement priority queue:
```go
type ExecutionQueue struct {
    high   chan *ExecutionRequest
    normal chan *ExecutionRequest
    low    chan *ExecutionRequest
}

func (q *ExecutionQueue) Enqueue(req *ExecutionRequest) error {
    switch req.Priority {
    case PriorityHigh:
        q.high <- req
    case PriorityLow:
        q.low <- req
    default:
        q.normal <- req
    }
}
```

2. Worker pool management
3. Dynamic scaling based on load
4. Queue persistence for reliability
5. Dead letter queue for failures

### Testing
```bash
# Submit high priority execution
curl -X POST localhost:8084/api/executions \
  -H "X-Priority: high" \
  -d '{"workflow_id":"wf_123"}'

# Get queue status
curl -X GET localhost:8084/api/admin/queue/status
```

### Acceptance Criteria
- ✅ Priority queue works
- ✅ Worker pool scales
- ✅ Queue persisted (survives restart)
- ✅ DLQ captures failures
- ✅ Metrics exposed

---

## EXEC-004: Implement Distributed Execution
**Priority**: P1 | **Hours**: 6 | **Dependencies**: EXEC-001, INFRA-005

### Context
Distribute workflow execution across multiple executor instances for scalability.

### Implementation
**Files to modify:**
- `internal/services/executor/distributed/coordinator.go` (create)
- `internal/services/executor/distributed/worker_registry.go` (create)
- `internal/services/executor/distributed/partition.go` (create)

### Steps
1. Implement coordinator:
```go
type Coordinator struct {
    workers    map[string]*WorkerNode
    partitions map[string]string // workflow -> worker mapping
    mu         sync.RWMutex
}

func (c *Coordinator) AssignWork(executionID string) (*WorkerNode, error) {
    // Select worker based on load
    // Consider locality
    // Handle worker failures
}
```

2. Worker registration/discovery
3. Work partitioning strategy
4. Health checking
5. Rebalancing on failure

### Testing
```bash
# Register worker
curl -X POST localhost:8085/api/workers/register \
  -d '{"id":"worker1","capacity":10,"tags":["gpu"]}'

# Get worker status
curl -X GET localhost:8085/api/workers/status
```

### Acceptance Criteria
- ✅ Workers register/deregister
- ✅ Work distributed evenly
- ✅ Failover works
- ✅ Rebalancing automatic
- ✅ No work lost on failure

---

## EXEC-005: Implement Execution Persistence & Recovery
**Priority**: P1 | **Hours**: 4 | **Dependencies**: EXEC-001, DATA-004

### Context
Persist execution state for recovery from failures and debugging.

### Implementation
**Files to modify:**
- `internal/services/execution/persistence/store.go` (create)
- `internal/services/execution/recovery/manager.go` (create)
- `migrations/add_execution_checkpoints.sql` (create)

### Steps
1. Implement checkpointing:
```go
type Checkpoint struct {
    ExecutionID string
    NodeID      string
    State       []byte
    Timestamp   time.Time
}

func (s *Store) SaveCheckpoint(ctx context.Context, cp *Checkpoint) error {
    // Serialize state
    // Store in database
    // Update execution record
}
```

2. Save state after each node
3. Recovery on restart
4. Replay from checkpoint
5. Cleanup old checkpoints

### Testing
```bash
# Trigger checkpoint
curl -X POST localhost:8084/api/executions/<id>/checkpoint

# Recover from checkpoint
curl -X POST localhost:8084/api/executions/<id>/recover
```

### Acceptance Criteria
- ✅ Checkpoints saved reliably
- ✅ Recovery works correctly
- ✅ State consistency maintained
- ✅ Old checkpoints cleaned up
- ✅ Performance impact < 5%

---

## EXEC-006: Implement Execution Monitoring & Metrics
**Priority**: P1 | **Hours**: 3 | **Dependencies**: EXEC-001, MON-001

### Context
Real-time monitoring of execution performance and health metrics.

### Implementation
**Files to modify:**
- `internal/services/execution/metrics/collector.go` (create)
- `internal/services/execution/metrics/exporter.go` (create)

### Steps
1. Define metrics:
```go
var (
    executionsStarted = promauto.NewCounter(prometheus.CounterOpts{
        Name: "linkflow_executions_started_total",
    })
    
    executionDuration = promauto.NewHistogram(prometheus.HistogramOpts{
        Name:    "linkflow_execution_duration_seconds",
        Buckets: prometheus.DefBuckets,
    })
    
    activeExecutions = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "linkflow_executions_active",
    })
)
```

2. Collect execution metrics
3. Node-level metrics
4. Resource usage tracking
5. Export to Prometheus

### Testing
```bash
# Get metrics
curl -X GET localhost:8084/metrics

# Get execution metrics
curl -X GET localhost:8084/api/executions/<id>/metrics
```

### Acceptance Criteria
- ✅ All metrics collected
- ✅ Prometheus export works
- ✅ Real-time updates
- ✅ Historical data available
- ✅ Dashboard visualization ready

---

## EXEC-007: Implement Execution Logs & Tracing
**Priority**: P1 | **Hours**: 3 | **Dependencies**: EXEC-001, MON-002

### Context
Comprehensive logging and distributed tracing for execution debugging.

### Implementation
**Files to modify:**
- `internal/services/execution/logging/logger.go` (create)
- `internal/services/execution/tracing/tracer.go` (create)

### Steps
1. Structured execution logs:
```go
type ExecutionLog struct {
    ExecutionID string
    NodeID      string
    Level       LogLevel
    Message     string
    Data        map[string]interface{}
    Timestamp   time.Time
}
```

2. Log aggregation per execution
3. OpenTelemetry tracing
4. Span creation per node
5. Log streaming via WebSocket

### Testing
```bash
# Get execution logs
curl -X GET localhost:8084/api/executions/<id>/logs

# Stream logs
wscat -c ws://localhost:8084/api/executions/<id>/logs/stream
```

### Acceptance Criteria
- ✅ Logs captured at all levels
- ✅ Tracing spans created
- ✅ Logs aggregated properly
- ✅ Real-time streaming works
- ✅ Searchable logs

---

## EXEC-008: Implement Retry & Error Handling
**Priority**: P1 | **Hours**: 4 | **Dependencies**: EXEC-001, EXEC-002

### Context
Robust retry mechanisms and error handling strategies for resilient execution.

### Implementation
**Files to modify:**
- `internal/services/execution/retry/manager.go` (create)
- `internal/services/execution/error/handler.go` (create)

### Steps
1. Implement retry strategies:
```go
type RetryStrategy interface {
    ShouldRetry(err error, attempt int) bool
    NextDelay(attempt int) time.Duration
}

type ExponentialBackoff struct {
    MaxAttempts int
    InitialDelay time.Duration
    MaxDelay     time.Duration
    Multiplier   float64
}
```

2. Error classification
3. Conditional retry logic
4. Circuit breaker per node
5. Error workflow triggers

### Testing
```bash
# Configure retry
curl -X PUT localhost:8084/api/workflows/<id>/retry \
  -d '{"strategy":"exponential","max_attempts":3}'

# Test retry behavior
curl -X POST localhost:8084/api/executions/<id>/test-retry
```

### Acceptance Criteria
- ✅ Retry strategies work
- ✅ Errors properly classified
- ✅ Circuit breaker functions
- ✅ Error workflows trigger
- ✅ Retry metrics tracked

---

## EXEC-009: Implement Execution Cancellation & Timeout
**Priority**: P2 | **Hours**: 3 | **Dependencies**: EXEC-001

### Context
Ability to cancel running executions and enforce timeouts at execution and node levels.

### Implementation
**Files to modify:**
- `internal/services/execution/cancellation/manager.go` (create)
- `internal/services/execution/timeout/enforcer.go` (create)

### Steps
1. Implement cancellation:
```go
func (m *Manager) CancelExecution(ctx context.Context, executionID string) error {
    // Send cancellation signal
    // Stop running nodes
    // Cleanup resources
    // Update state
    // Publish event
}
```

2. Graceful shutdown of nodes
3. Global execution timeout
4. Per-node timeouts
5. Timeout escalation

### Testing
```bash
# Cancel execution
curl -X POST localhost:8084/api/executions/<id>/cancel

# Set timeout
curl -X PUT localhost:8084/api/executions/<id>/timeout \
  -d '{"timeout_seconds":300}'
```

### Acceptance Criteria
- ✅ Cancellation works immediately
- ✅ Resources cleaned up
- ✅ Timeouts enforced
- ✅ Graceful shutdown
- ✅ State consistent after cancel

---

## EXEC-010: Implement Execution Cost Tracking
**Priority**: P3 | **Hours**: 3 | **Dependencies**: EXEC-001, BILL-001

### Context
Track execution costs based on resource usage for billing and optimization.

### Implementation
**Files to modify:**
- `internal/services/execution/cost/calculator.go` (create)
- `internal/services/execution/cost/tracker.go` (create)

### Steps
1. Define cost model:
```go
type ExecutionCost struct {
    ExecutionID   string
    ComputeTime   time.Duration
    MemoryUsage   int64
    NetworkBytes  int64
    StorageBytes  int64
    TotalCost     float64
}
```

2. Track resource usage
3. Calculate costs
4. Aggregate per user/team
5. Cost optimization suggestions

### Testing
```bash
# Get execution cost
curl -X GET localhost:8084/api/executions/<id>/cost

# Get cost report
curl -X GET localhost:8084/api/costs/report?period=month
```

### Acceptance Criteria
- ✅ Resource usage tracked
- ✅ Costs calculated accurately
- ✅ Reports generated
- ✅ Optimization suggestions work
- ✅ Billing integration ready

---

## Summary Stats
- **Total Tasks**: 10
- **Total Hours**: 41
- **Critical (P0)**: 3
- **High (P1)**: 5
- **Medium (P2)**: 1
- **Low (P3)**: 1

## Execution Order
1. EXEC-001, EXEC-002, EXEC-003 (parallel - critical foundation)
2. EXEC-004, EXEC-005, EXEC-006 (parallel)
3. EXEC-007, EXEC-008, EXEC-009
4. EXEC-010

## Team Assignment Suggestion
- **Senior Dev**: EXEC-001, EXEC-004
- **Mid Dev 1**: EXEC-002, EXEC-008
- **Mid Dev 2**: EXEC-003, EXEC-005, EXEC-007
- **Junior Dev**: EXEC-006, EXEC-009, EXEC-010
