# 💾 Data & Database Tasks

## Prerequisites
- PostgreSQL running (docker-compose up postgres)
- Redis running (docker-compose up redis)
- Migration tool installed (golang-migrate)

---

## DATA-001: Run Initial Database Migrations
**Priority**: P0 | **Hours**: 2 | **Dependencies**: None

### Context
Apply the initial database schema to create all required tables for the platform.

### Implementation
**Files to modify:**
- `deployments/migrations/000001_init_schema.up.sql`
- `deployments/migrations/000002_indexes.up.sql` (create)
- `scripts/migrate.sh` (create)

### Steps
1. Create migration script:
```bash
#!/bin/bash
# scripts/migrate.sh
DATABASE_URL="postgresql://linkflow:password@localhost:5432/linkflow?sslmode=disable"

migrate -path deployments/migrations \
        -database $DATABASE_URL \
        up
```

2. Verify schema:
```sql
-- Check tables created
SELECT table_name FROM information_schema.tables 
WHERE table_schema = 'public';

-- Check indexes
SELECT indexname FROM pg_indexes 
WHERE schemaname = 'public';
```

3. Add missing indexes
4. Create partitions for large tables
5. Setup read replicas

### Testing
```bash
# Run migrations
./scripts/migrate.sh up

# Verify schema
psql -U linkflow -d linkflow -c "\dt"

# Run migration tests
go test ./migrations/...
```

### Acceptance Criteria
- ✅ All tables created successfully
- ✅ Indexes properly configured
- ✅ Foreign keys established
- ✅ Constraints enforced
- ✅ Migration rollback works

---

## DATA-002: Implement User Repository
**Priority**: P0 | **Hours**: 4 | **Dependencies**: DATA-001

### Context
Complete implementation of user data access layer with all CRUD operations.

### Implementation
**Files to modify:**
- `internal/services/user/repository/repository.go`
- `internal/services/user/repository/queries.go` (create)

### Steps
1. Implement user CRUD:
```go
func (r *UserRepository) Create(ctx context.Context, user *User) error {
    return r.db.WithContext(ctx).Create(user).Error
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
    var user User
    err := r.db.WithContext(ctx).
        Where("email = ?", email).
        First(&user).Error
    return &user, err
}

func (r *UserRepository) Update(ctx context.Context, user *User) error {
    return r.db.WithContext(ctx).
        Model(user).
        Updates(user).Error
}
```

2. Add complex queries:
   - Search with pagination
   - Filter by role/team
   - Bulk operations
   - Soft delete

3. Query optimization
4. Connection pooling
5. Read replica routing

### Testing
```go
func TestUserRepository_Create(t *testing.T) {
    db := setupTestDB(t)
    repo := NewUserRepository(db)
    
    user := &User{
        Email: "test@example.com",
        Name:  "Test User",
    }
    
    err := repo.Create(context.Background(), user)
    assert.NoError(t, err)
    assert.NotEmpty(t, user.ID)
}
```

### Acceptance Criteria
- ✅ All CRUD operations work
- ✅ Complex queries optimized
- ✅ Transactions handled properly
- ✅ Connection pooling configured
- ✅ Tests coverage > 90%

---

## DATA-003: Implement Workflow Repository
**Priority**: P0 | **Hours**: 5 | **Dependencies**: DATA-001

### Context
Complex workflow data access with versioning, relationships, and optimized queries.

### Implementation
**Files to modify:**
- `internal/services/workflow/repository/repository.go`
- `internal/services/workflow/repository/version_repository.go` (create)

### Steps
1. Implement workflow operations:
```go
func (r *WorkflowRepository) CreateWithVersion(ctx context.Context, workflow *Workflow) error {
    return r.db.Transaction(func(tx *gorm.DB) error {
        // Create workflow
        if err := tx.Create(workflow).Error; err != nil {
            return err
        }
        
        // Create initial version
        version := &WorkflowVersion{
            WorkflowID: workflow.ID,
            Version:    1,
            Data:       workflow.ToJSON(),
        }
        
        return tx.Create(version).Error
    })
}
```

2. Preload relationships:
```go
func (r *WorkflowRepository) GetWithNodes(ctx context.Context, id string) (*Workflow, error) {
    var workflow Workflow
    err := r.db.WithContext(ctx).
        Preload("Nodes").
        Preload("Connections").
        Preload("Triggers").
        Where("id = ?", id).
        First(&workflow).Error
    return &workflow, err
}
```

3. Implement versioning
4. Handle large JSON fields
5. Query optimization

### Acceptance Criteria
- ✅ Workflow CRUD with versioning
- ✅ Relationships properly loaded
- ✅ JSON fields handled efficiently
- ✅ Queries optimized (< 50ms)
- ✅ Concurrent updates handled

---

## DATA-004: Implement Execution Repository
**Priority**: P0 | **Hours**: 4 | **Dependencies**: DATA-001

### Context
High-performance execution data access with time-series data and state management.

### Implementation
**Files to modify:**
- `internal/services/execution/repository/repository.go`
- `internal/services/execution/repository/timeseries.go` (create)

### Steps
1. Execution state management:
```go
func (r *ExecutionRepository) UpdateState(ctx context.Context, id string, state ExecutionState) error {
    return r.db.Transaction(func(tx *gorm.DB) error {
        // Update execution state
        if err := tx.Model(&Execution{}).
            Where("id = ?", id).
            Updates(map[string]interface{}{
                "state":      state,
                "updated_at": time.Now(),
            }).Error; err != nil {
            return err
        }
        
        // Record state transition
        transition := &StateTransition{
            ExecutionID: id,
            FromState:   r.getCurrentState(tx, id),
            ToState:     state,
            Timestamp:   time.Now(),
        }
        
        return tx.Create(transition).Error
    })
}
```

2. Time-series data handling
3. Execution history queries
4. Performance metrics storage
5. Archive old executions

### Testing
```go
func BenchmarkExecutionRepository_UpdateState(b *testing.B) {
    repo := setupTestRepo(b)
    
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            repo.UpdateState(context.Background(), "exec_123", StateRunning)
        }
    })
}
```

### Acceptance Criteria
- ✅ State transitions atomic
- ✅ Time-series data efficient
- ✅ History queries fast
- ✅ Archives work correctly
- ✅ Handles 1000+ updates/sec

---

## DATA-005: Implement Caching Layer
**Priority**: P1 | **Hours**: 4 | **Dependencies**: DATA-002, DATA-003

### Context
Redis-based caching layer to reduce database load and improve performance.

### Implementation
**Files to modify:**
- `pkg/cache/cache.go` (create)
- `pkg/cache/redis_cache.go` (create)
- `internal/services/*/repository/cached_repository.go` (create for each service)

### Steps
1. Create cache interface:
```go
type Cache interface {
    Get(ctx context.Context, key string, dest interface{}) error
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Invalidate(ctx context.Context, pattern string) error
}
```

2. Implement Redis cache:
```go
type RedisCache struct {
    client *redis.Client
    codec  Codec
}

func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
    data, err := c.client.Get(ctx, key).Bytes()
    if err != nil {
        return err
    }
    return c.codec.Decode(data, dest)
}
```

3. Cache strategies:
   - Write-through
   - Cache-aside
   - TTL management
   - Cache invalidation

4. Add to repositories

### Testing
```go
func TestCachedRepository_GetUser(t *testing.T) {
    cache := NewMockCache()
    repo := NewCachedUserRepository(db, cache)
    
    // First call - cache miss
    user1, err := repo.GetByID(ctx, "user_123")
    assert.NoError(t, err)
    
    // Second call - cache hit
    user2, err := repo.GetByID(ctx, "user_123")
    assert.NoError(t, err)
    assert.Equal(t, user1, user2)
    assert.Equal(t, 1, cache.Misses)
    assert.Equal(t, 1, cache.Hits)
}
```

### Acceptance Criteria
- ✅ Cache layer works transparently
- ✅ Cache invalidation correct
- ✅ TTL properly managed
- ✅ Performance improved 10x
- ✅ Cache metrics available

---

## DATA-006: Implement Event Sourcing Store
**Priority**: P1 | **Hours**: 5 | **Dependencies**: DATA-001, EVENT-001

### Context
Event sourcing for audit trail and ability to rebuild state from events.

### Implementation
**Files to modify:**
- `pkg/eventsourcing/store.go` (create)
- `pkg/eventsourcing/aggregate.go` (create)
- `migrations/add_event_store.sql` (create)

### Steps
1. Create event store:
```go
type EventStore interface {
    Save(ctx context.Context, events []Event) error
    Load(ctx context.Context, aggregateID string) ([]Event, error)
    LoadSnapshot(ctx context.Context, aggregateID string) (*Snapshot, error)
    SaveSnapshot(ctx context.Context, snapshot *Snapshot) error
}

type Event struct {
    ID           string
    AggregateID  string
    Type         string
    Version      int
    Payload      json.RawMessage
    Metadata     map[string]string
    Timestamp    time.Time
}
```

2. Implement aggregate root:
```go
type AggregateRoot struct {
    ID            string
    Version       int
    Changes       []Event
    EventHandlers map[string]EventHandler
}

func (a *AggregateRoot) ApplyChange(event Event) {
    handler := a.EventHandlers[event.Type]
    if handler != nil {
        handler(event)
    }
    a.Changes = append(a.Changes, event)
    a.Version++
}
```

3. Event replay mechanism
4. Snapshot optimization
5. Event projection

### Acceptance Criteria
- ✅ Events stored reliably
- ✅ State rebuilt from events
- ✅ Snapshots improve performance
- ✅ Event ordering maintained
- ✅ Projections update correctly

---

## DATA-007: Implement Read Model Projections
**Priority**: P2 | **Hours**: 4 | **Dependencies**: DATA-006

### Context
CQRS read models optimized for queries, updated via event projections.

### Implementation
**Files to modify:**
- `pkg/cqrs/projection.go` (create)
- `internal/services/*/projections/` (create for each service)

### Steps
1. Define projection interface:
```go
type Projection interface {
    Handle(ctx context.Context, event Event) error
    Reset(ctx context.Context) error
    GetName() string
}

type WorkflowListProjection struct {
    db *gorm.DB
}

func (p *WorkflowListProjection) Handle(ctx context.Context, event Event) error {
    switch event.Type {
    case "WorkflowCreated":
        return p.handleWorkflowCreated(event)
    case "WorkflowUpdated":
        return p.handleWorkflowUpdated(event)
    }
    return nil
}
```

2. Projection manager
3. Async projection updates
4. Projection rebuilding
5. Consistency monitoring

### Testing
```go
func TestProjection_ConsistencyCheck(t *testing.T) {
    projection := NewWorkflowListProjection(db)
    
    // Apply events
    events := loadTestEvents()
    for _, event := range events {
        err := projection.Handle(ctx, event)
        assert.NoError(t, err)
    }
    
    // Verify projection state
    workflows, err := projection.GetWorkflows()
    assert.NoError(t, err)
    assert.Equal(t, expectedCount, len(workflows))
}
```

### Acceptance Criteria
- ✅ Projections update async
- ✅ Read models optimized
- ✅ Rebuilding works
- ✅ Consistency monitored
- ✅ Query performance < 10ms

---

## DATA-008: Implement Data Archival
**Priority**: P2 | **Hours**: 3 | **Dependencies**: DATA-004

### Context
Archive old execution data to cold storage while maintaining query capability.

### Implementation
**Files to modify:**
- `internal/services/execution/archival/archiver.go` (create)
- `internal/services/execution/archival/retriever.go` (create)
- `scripts/archive_job.sh` (create)

### Steps
1. Implement archiver:
```go
type Archiver struct {
    db          *gorm.DB
    storage     Storage
    compression Compressor
}

func (a *Archiver) ArchiveExecutions(ctx context.Context, before time.Time) error {
    // Query old executions
    var executions []Execution
    err := a.db.Where("created_at < ?", before).Find(&executions).Error
    
    // Compress data
    data, err := a.compression.Compress(executions)
    
    // Store in S3/cold storage
    key := fmt.Sprintf("archive/executions/%s.gz", time.Now().Format("2006-01-02"))
    err = a.storage.Upload(ctx, key, data)
    
    // Delete from hot storage
    return a.db.Where("created_at < ?", before).Delete(&Execution{}).Error
}
```

2. Retrieval mechanism
3. Scheduled archival jobs
4. Archive querying
5. Compliance requirements

### Acceptance Criteria
- ✅ Data archived successfully
- ✅ Retrieval works
- ✅ Compression effective (>70%)
- ✅ Queries span hot/cold data
- ✅ Compliance maintained

---

## DATA-009: Implement Database Monitoring
**Priority**: P2 | **Hours**: 3 | **Dependencies**: DATA-001, MON-001

### Context
Monitor database performance, slow queries, and connection pool health.

### Implementation
**Files to modify:**
- `pkg/database/monitoring.go` (create)
- `pkg/database/slow_query_logger.go` (create)
- `configs/grafana/dashboards/database.json` (create)

### Steps
1. Implement monitoring:
```go
type DBMonitor struct {
    db      *gorm.DB
    metrics *DBMetrics
}

type DBMetrics struct {
    ConnectionsActive    prometheus.Gauge
    ConnectionsIdle      prometheus.Gauge
    QueriesTotal        prometheus.Counter
    QueryDuration       prometheus.Histogram
    SlowQueries         prometheus.Counter
}

func (m *DBMonitor) RecordQuery(query string, duration time.Duration) {
    m.metrics.QueriesTotal.Inc()
    m.metrics.QueryDuration.Observe(duration.Seconds())
    
    if duration > SlowQueryThreshold {
        m.metrics.SlowQueries.Inc()
        m.logSlowQuery(query, duration)
    }
}
```

2. Slow query logging
3. Connection pool metrics
4. Table statistics
5. Grafana dashboard

### Acceptance Criteria
- ✅ All queries monitored
- ✅ Slow queries logged
- ✅ Pool metrics accurate
- ✅ Dashboard informative
- ✅ Alerts configured

---

## DATA-010: Implement Data Backup & Recovery
**Priority**: P3 | **Hours**: 4 | **Dependencies**: DATA-001, INFRA-008

### Context
Automated backup strategy with point-in-time recovery capability.

### Implementation
**Files to modify:**
- `scripts/backup/backup.sh` (create)
- `scripts/backup/restore.sh` (create)
- `deployments/k8s/cronjob-backup.yaml` (create)

### Steps
1. Create backup script:
```bash
#!/bin/bash
# Daily backup with retention
BACKUP_DIR="/backups/$(date +%Y%m%d)"
RETENTION_DAYS=30

# Backup PostgreSQL
pg_dump -h $DB_HOST -U $DB_USER -d $DB_NAME | \
  gzip > $BACKUP_DIR/postgres.sql.gz

# Backup Redis
redis-cli --rdb $BACKUP_DIR/redis.rdb

# Upload to S3
aws s3 sync $BACKUP_DIR s3://linkflow-backups/$(date +%Y%m%d)/

# Clean old backups
find /backups -type d -mtime +$RETENTION_DAYS -exec rm -rf {} \;
```

2. Point-in-time recovery
3. Automated testing of backups
4. Disaster recovery plan
5. Cross-region replication

### Acceptance Criteria
- ✅ Daily backups automated
- ✅ Recovery tested monthly
- ✅ RPO < 24 hours
- ✅ RTO < 4 hours
- ✅ Cross-region backup exists

---

## Summary Stats
- **Total Tasks**: 10
- **Total Hours**: 38
- **Critical (P0)**: 4
- **High (P1)**: 3
- **Medium (P2)**: 2
- **Low (P3)**: 1

## Execution Order
1. DATA-001 (foundation - must be first)
2. DATA-002, DATA-003, DATA-004 (parallel - critical repositories)
3. DATA-005, DATA-006 (parallel - performance)
4. DATA-007, DATA-008, DATA-009 (parallel)
5. DATA-010

## Team Assignment Suggestion
- **Senior Dev**: DATA-006, DATA-007 (event sourcing/CQRS)
- **Mid Dev 1**: DATA-002, DATA-003 (repositories)
- **Mid Dev 2**: DATA-004, DATA-005 (execution/caching)
- **Junior Dev**: DATA-001, DATA-008, DATA-009, DATA-010 (operations)
