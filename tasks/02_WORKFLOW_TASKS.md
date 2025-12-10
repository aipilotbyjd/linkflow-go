# ⚙️ Workflow Engine Tasks

## Prerequisites
- Database setup complete (DATA-001)
- Auth service operational (AUTH-001, AUTH-002)
- Event bus configured (EVENT-001)

---

## WORK-001: Implement Workflow CRUD Operations
**Priority**: P0 | **Hours**: 4 | **Dependencies**: DATA-002

### Context
Basic workflow creation, reading, updating, and deletion operations are fundamental to the platform.

### Implementation
**Files to modify:**
- `internal/services/workflow/handlers/handlers.go`
- `internal/services/workflow/service/service.go`
- `internal/services/workflow/repository/repository.go`

### Steps
1. Implement CreateWorkflow handler:
```go
func (s *WorkflowService) CreateWorkflow(ctx context.Context, req CreateWorkflowRequest) (*Workflow, error) {
    // Validate workflow structure
    // Generate unique ID
    // Set initial version
    // Store in database
    // Publish WorkflowCreated event
}
```

2. Implement GetWorkflow with versioning
3. Implement UpdateWorkflow with version increment
4. Implement DeleteWorkflow (soft delete)
5. Add pagination for ListWorkflows

### Testing
```bash
# Create workflow
curl -X POST localhost:8083/api/workflows \
  -H "Authorization: Bearer <token>" \
  -d @test/fixtures/sample_workflow.json

# Get workflow
curl -X GET localhost:8083/api/workflows/<workflow_id>

# Update workflow
curl -X PUT localhost:8083/api/workflows/<workflow_id> \
  -d @test/fixtures/updated_workflow.json

# Delete workflow
curl -X DELETE localhost:8083/api/workflows/<workflow_id>
```

### Acceptance Criteria
- ✅ CRUD operations work correctly
- ✅ Workflow versioning implemented
- ✅ User ownership enforced
- ✅ Events published for all operations
- ✅ Pagination works for listing

---

## WORK-002: Implement DAG Validation
**Priority**: P0 | **Hours**: 6 | **Dependencies**: WORK-001

### Context
Workflows must be valid Directed Acyclic Graphs (DAGs) without cycles or invalid connections.

### Implementation
**Files to modify:**
- `internal/domain/workflow/validator.go` (create)
- `internal/domain/workflow/dag.go` (create)
- `internal/services/workflow/service/validation_service.go` (create)

### Steps
1. Implement DAG structure:
```go
type DAG struct {
    Nodes       map[string]*Node
    Edges       map[string][]string
    StartNodes  []string
    EndNodes    []string
}

func (d *DAG) Validate() error {
    // Check for cycles
    // Validate all connections
    // Ensure start/end nodes exist
    // Check node dependencies
}
```

2. Implement cycle detection (DFS/Topological sort)
3. Validate node connections match schemas
4. Check for unreachable nodes
5. Validate required/optional inputs

### Example Validation:
```go
func HasCycle(workflow *Workflow) bool {
    visited := make(map[string]bool)
    recStack := make(map[string]bool)
    
    for nodeID := range workflow.Nodes {
        if !visited[nodeID] {
            if hasCycleDFS(nodeID, visited, recStack, workflow) {
                return true
            }
        }
    }
    return false
}
```

### Acceptance Criteria
- ✅ Cycles detected and rejected
- ✅ Invalid connections rejected
- ✅ Orphaned nodes detected
- ✅ Schema validation works
- ✅ Performance < 100ms for 1000 nodes

---

## WORK-003: Implement Workflow Templates
**Priority**: P1 | **Hours**: 4 | **Dependencies**: WORK-001

### Context
Pre-built workflow templates help users get started quickly.

### Implementation
**Files to modify:**
- `internal/services/workflow/templates/manager.go` (create)
- `internal/services/workflow/templates/library/` (create templates)
- `migrations/seed_templates.sql` (create)

### Steps
1. Create template structure:
```go
type Template struct {
    ID          string
    Name        string
    Description string
    Category    string
    Icon        string
    Workflow    Workflow
    Variables   []Variable
    Tags        []string
}
```

2. Create built-in templates:
   - Data ETL Pipeline
   - Webhook to Database
   - Scheduled Report
   - API Integration
   - Error Notification

3. Implement template instantiation
4. Variable substitution
5. Template marketplace

### Testing
```bash
# List templates
curl -X GET localhost:8083/api/templates

# Create workflow from template
curl -X POST localhost:8083/api/templates/<template_id>/instantiate \
  -d '{"name":"My ETL Pipeline","variables":{"source":"postgres"}}'
```

### Acceptance Criteria
- ✅ 10+ built-in templates
- ✅ Template instantiation works
- ✅ Variables properly substituted
- ✅ Templates categorized
- ✅ Search/filter works

---

## WORK-004: Implement Workflow Versioning
**Priority**: P1 | **Hours**: 5 | **Dependencies**: WORK-001

### Context
Track all workflow changes with ability to view and restore previous versions.

### Implementation
**Files to modify:**
- `internal/services/workflow/versioning/manager.go` (create)
- `internal/domain/workflow/version.go`
- `migrations/add_workflow_versions.sql` (create)

### Steps
1. Create version tracking:
```go
type WorkflowVersion struct {
    ID         string
    WorkflowID string
    Version    int
    Data       json.RawMessage
    ChangedBy  string
    ChangedAt  time.Time
    ChangeLog  string
}
```

2. Auto-increment version on save
3. Store full workflow snapshot
4. Implement diff generation
5. Add restore capability

### Testing
```bash
# Get version history
curl -X GET localhost:8083/api/workflows/<id>/versions

# Get specific version
curl -X GET localhost:8083/api/workflows/<id>/versions/3

# Restore version
curl -X POST localhost:8083/api/workflows/<id>/restore \
  -d '{"version":3}'
```

### Acceptance Criteria
- ✅ All changes create versions
- ✅ Version history viewable
- ✅ Can restore old versions
- ✅ Diff between versions shown
- ✅ Storage optimized (deltas)

---

## WORK-005: Implement Workflow Variables & Environment
**Priority**: P1 | **Hours**: 3 | **Dependencies**: WORK-001

### Context
Workflows need variables and environment configuration for different deployment contexts.

### Implementation
**Files to modify:**
- `internal/domain/workflow/variables.go` (create)
- `internal/services/workflow/environment/manager.go` (create)

### Steps
1. Define variable types:
```go
type Variable struct {
    Key         string
    Type        VariableType // string, number, boolean, json
    Value       interface{}
    Encrypted   bool
    Environment string
}
```

2. Implement variable interpolation
3. Environment-specific overrides
4. Secure variable storage
5. Runtime variable injection

### Testing
```bash
# Set workflow variables
curl -X PUT localhost:8083/api/workflows/<id>/variables \
  -d '{"API_KEY":"{{secret}}","ENV":"production"}'

# Get variables for environment
curl -X GET localhost:8083/api/workflows/<id>/variables?env=production
```

### Acceptance Criteria
- ✅ Variables properly stored
- ✅ Environment overrides work
- ✅ Secrets encrypted
- ✅ Variable interpolation works
- ✅ Type validation enforced

---

## WORK-006: Implement Workflow Sharing & Permissions
**Priority**: P2 | **Hours**: 4 | **Dependencies**: WORK-001, AUTH-005

### Context
Enable workflow sharing between users and teams with granular permissions.

### Implementation
**Files to modify:**
- `internal/services/workflow/sharing/manager.go` (create)
- `internal/services/workflow/permissions/checker.go` (create)
- `migrations/add_workflow_sharing.sql` (create)

### Steps
1. Define permission levels:
```go
const (
    PermissionView    = "view"
    PermissionEdit    = "edit"
    PermissionExecute = "execute"
    PermissionAdmin   = "admin"
)
```

2. Implement sharing mechanism
3. Team-based sharing
4. Public workflow gallery
5. Permission inheritance

### Testing
```bash
# Share workflow
curl -X POST localhost:8083/api/workflows/<id>/share \
  -d '{"user_id":"user2","permission":"view"}'

# List shared workflows
curl -X GET localhost:8083/api/workflows/shared
```

### Acceptance Criteria
- ✅ Sharing works for users
- ✅ Team sharing works
- ✅ Permissions enforced
- ✅ Public gallery works
- ✅ Audit trail maintained

---

## WORK-007: Implement Workflow Import/Export
**Priority**: P2 | **Hours**: 3 | **Dependencies**: WORK-001

### Context
Users need to import/export workflows for backup and sharing.

### Implementation
**Files to modify:**
- `internal/services/workflow/transfer/exporter.go` (create)
- `internal/services/workflow/transfer/importer.go` (create)

### Steps
1. Define export format:
```go
type WorkflowExport struct {
    Version     string          `json:"version"`
    Workflow    Workflow        `json:"workflow"`
    Nodes       []NodeExport    `json:"nodes"`
    Credentials []string        `json:"required_credentials"`
    Variables   []Variable      `json:"variables"`
}
```

2. Export to JSON/YAML
3. Import with validation
4. Handle credential mapping
5. Bulk import/export

### Testing
```bash
# Export workflow
curl -X GET localhost:8083/api/workflows/<id>/export > workflow.json

# Import workflow
curl -X POST localhost:8083/api/workflows/import \
  -F "file=@workflow.json"
```

### Acceptance Criteria
- ✅ Export includes all data
- ✅ Import validates structure
- ✅ Credentials mapped correctly
- ✅ Bulk operations work
- ✅ Version compatibility handled

---

## WORK-008: Implement Workflow Tags & Categories
**Priority**: P3 | **Hours**: 2 | **Dependencies**: WORK-001

### Context
Organize workflows with tags and categories for better discovery.

### Implementation
**Files to modify:**
- `internal/services/workflow/tagging/manager.go` (create)
- `internal/services/workflow/handlers/tag_handlers.go` (create)

### Steps
1. Implement tagging system
2. Pre-defined categories
3. Tag-based search
4. Popular tags tracking
5. Auto-tagging suggestions

### Testing
```bash
# Add tags
curl -X PUT localhost:8083/api/workflows/<id>/tags \
  -d '{"tags":["automation","daily","report"]}'

# Search by tag
curl -X GET localhost:8083/api/workflows?tags=automation,daily
```

### Acceptance Criteria
- ✅ Tags added/removed easily
- ✅ Search by tags works
- ✅ Categories properly defined
- ✅ Popular tags tracked
- ✅ Auto-suggestions work

---

## WORK-009: Implement Workflow Triggers
**Priority**: P0 | **Hours**: 5 | **Dependencies**: WORK-001, NODE-002

### Context
Workflows need various trigger mechanisms: webhook, schedule, event, manual.

### Implementation
**Files to modify:**
- `internal/services/workflow/triggers/manager.go` (create)
- `internal/services/workflow/triggers/types/` (create types)
- `internal/domain/workflow/trigger.go`

### Steps
1. Define trigger interface:
```go
type Trigger interface {
    GetType() string
    Validate() error
    GetConfig() map[string]interface{}
    ShouldFire(event interface{}) bool
}
```

2. Implement trigger types:
   - Webhook trigger
   - Cron schedule trigger
   - Event trigger
   - Manual trigger
   - Email trigger

3. Trigger registration
4. Trigger activation/deactivation
5. Trigger testing mechanism

### Testing
```bash
# Add webhook trigger
curl -X POST localhost:8083/api/workflows/<id>/triggers \
  -d '{"type":"webhook","config":{"path":"/hook/data"}}'

# Test trigger
curl -X POST localhost:8083/api/workflows/<id>/triggers/<trigger_id>/test
```

### Acceptance Criteria
- ✅ All trigger types work
- ✅ Multiple triggers per workflow
- ✅ Triggers can be disabled
- ✅ Test mode available
- ✅ Trigger events logged

---

## WORK-010: Implement Workflow Cloning
**Priority**: P3 | **Hours**: 2 | **Dependencies**: WORK-001

### Context
Users need to duplicate workflows for variations and testing.

### Implementation
**Files to modify:**
- `internal/services/workflow/service/clone_service.go` (create)
- `internal/services/workflow/handlers/clone_handler.go` (create)

### Steps
1. Deep clone workflow
2. Generate new IDs
3. Copy or link credentials
4. Update ownership
5. Clone with modifications

### Testing
```bash
# Clone workflow
curl -X POST localhost:8083/api/workflows/<id>/clone \
  -d '{"name":"Cloned Workflow","include_credentials":true}'
```

### Acceptance Criteria
- ✅ Complete workflow cloned
- ✅ New IDs generated
- ✅ Credentials handled properly
- ✅ Ownership updated
- ✅ Clone history tracked

---

## WORK-011: Implement Workflow Statistics
**Priority**: P2 | **Hours**: 3 | **Dependencies**: WORK-001, EXEC-003

### Context
Track workflow execution statistics for optimization and monitoring.

### Implementation
**Files to modify:**
- `internal/services/workflow/analytics/collector.go` (create)
- `internal/services/workflow/analytics/aggregator.go` (create)

### Steps
1. Track execution metrics:
```go
type WorkflowStats struct {
    TotalExecutions   int64
    SuccessfulRuns    int64
    FailedRuns        int64
    AverageRuntime    time.Duration
    LastExecution     time.Time
    ErrorRate         float64
    ThroughputPerHour float64
}
```

2. Real-time statistics
3. Historical aggregation
4. Performance metrics
5. Cost tracking

### Testing
```bash
# Get workflow statistics
curl -X GET localhost:8083/api/workflows/<id>/stats

# Get aggregated stats
curl -X GET localhost:8083/api/workflows/stats/summary?period=7d
```

### Acceptance Criteria
- ✅ Execution metrics tracked
- ✅ Real-time stats available
- ✅ Historical data aggregated
- ✅ Performance metrics accurate
- ✅ Dashboard-ready format

---

## WORK-012: Implement Workflow Debugging
**Priority**: P2 | **Hours**: 4 | **Dependencies**: WORK-001, EXEC-001

### Context
Provide debugging capabilities for workflow development and troubleshooting.

### Implementation
**Files to modify:**
- `internal/services/workflow/debug/debugger.go` (create)
- `internal/services/workflow/debug/breakpoint.go` (create)

### Steps
1. Implement debug mode:
```go
type DebugSession struct {
    ID          string
    WorkflowID  string
    Breakpoints []Breakpoint
    StepMode    bool
    Variables   map[string]interface{}
    Logs        []LogEntry
}
```

2. Set breakpoints on nodes
3. Step-through execution
4. Variable inspection
5. Mock data injection

### Testing
```bash
# Start debug session
curl -X POST localhost:8083/api/workflows/<id>/debug/start

# Set breakpoint
curl -X POST localhost:8083/api/workflows/<id>/debug/breakpoints \
  -d '{"node_id":"node_123"}'

# Step execution
curl -X POST localhost:8083/api/workflows/<id>/debug/step
```

### Acceptance Criteria
- ✅ Debug mode works
- ✅ Breakpoints functional
- ✅ Step-through works
- ✅ Variables inspectable
- ✅ Mock data injection works

---

## Summary Stats
- **Total Tasks**: 12
- **Total Hours**: 46
- **Critical (P0)**: 3
- **High (P1)**: 4
- **Medium (P2)**: 3
- **Low (P3)**: 2

## Execution Order
1. WORK-001 (foundation)
2. WORK-002, WORK-009 (parallel - critical)
3. WORK-003, WORK-004, WORK-005 (parallel)
4. WORK-006, WORK-007, WORK-011
5. WORK-008, WORK-010, WORK-012

## Team Assignment Suggestion
- **Senior Dev**: WORK-002, WORK-009, WORK-012
- **Mid Dev 1**: WORK-001, WORK-004, WORK-005
- **Mid Dev 2**: WORK-003, WORK-006, WORK-007
- **Junior Dev**: WORK-008, WORK-010, WORK-011
