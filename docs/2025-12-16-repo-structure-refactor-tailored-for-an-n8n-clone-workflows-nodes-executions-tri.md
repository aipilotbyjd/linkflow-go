## Context: n8n-clone needs “engine-friendly” boundaries
An n8n-like system’s core complexity is **workflow definition + node registry + execution orchestration + trigger ingress**. A Go-friendly structure should optimize for:
- clear ownership of **Workflow** vs **Execution** vs **Executor runtime**
- minimal coupling across services (avoid import cycles)
- easy evolution of node types, triggers, credentials, and scheduling

Below is a structure refactor approach tuned for that.

---

## Recommended structure (n8n-clone oriented)
### 1) Keep entrypoints per microservice
```
cmd/services/<service>/main.go
```
These should do only wiring + graceful shutdown.

### 2) Flatten service code (Option B style) + “platform” shared
```
internal/
  workflow/      # workflow definitions, versions, validation
  execution/     # execution records, state machine, retries
  executor/      # node runtime, sandboxing, step runner
  webhook/       # inbound webhooks, signature validation, enqueue execution
  schedule/      # cron/time-based triggers
  node/          # node catalog/metadata + UI schema (not runtime execution)
  credential/    # encryption + token refresh, secret storage interfaces
  variable/      # env/variables, runtime parameter resolution
  auth/          # auth + roles
  user/          # user profile (if separate from auth)
  notification/  # email/slack/etc.
  audit/         # audit trail
  search/        # index/read models

  platform/      # shared but app-private helpers (optional)
    httpx/
    tracing/
    persistence/
    id/

pkg/             # shared libs/SDK-like, stable surface
  config/
  database/
  events/
  logger/
  telemetry/
  ...
```

### 3) Inside each service: Clean/Hex slices (engine-friendly)
For the *core* services (`workflow`, `execution`, `executor`, `webhook`, `schedule`, `credential`), use a consistent layout:
```
internal/<svc>/
  domain/         # core entities + invariants
  app/            # usecases (orchestrate domain + ports)
  ports/          # interfaces (repo, eventbus, queue, secret store)
  adapters/
    http/         # handlers/controllers
    db/           # persistence implementation
    events/       # pub/sub wiring
    cache/        # redis impls
  server/         # router + wiring
```

This matches how an n8n-clone grows: new triggers/adapters are added without leaking into core domain.

---

## Service ownership rules (critical for n8n-clone)
### Workflow service owns (definition-time)
- Workflow graph: nodes/edges, versions, activation, validation
- “Can this workflow run?” checks (static)
- Emits events like:
  - `workflow.published`, `workflow.activated`, `workflow.updated`

### Execution service owns (control-plane)
- Execution state machine: queued/running/succeeded/failed/canceled
- Retry policy, timeouts, concurrency limits (per workflow/tenant)
- Execution persistence and lifecycle APIs
- Emits events like:
  - `execution.queued`, `execution.started`, `execution.finished`, `execution.failed`

### Executor service owns (data-plane runtime)
- Running node steps (often in workers)
- Node execution context, step outputs, error handling, partial retries
- Sandboxing/isolation strategy (even if currently simple)
- Should *not* own workflow persistence

### Webhook + Schedule are ingress services
- Translate trigger events into “start execution” commands
- Validate signatures/paths, rate limit
- Prefer enqueue/command to Execution service, not direct DB writes

### Credential service owns secrets
- Encryption-at-rest, token refresh flows
- Provide port interfaces for other services to request ephemeral usage tokens

---

## Shared contracts: how to avoid cross-service imports
**Never** import `internal/<other-service>`.
If a type must be shared (common in n8n clones):
- Put **contracts only** in `pkg/contracts/...`:
  - `WorkflowSummary`, `ExecutionSummary`, `TriggerRequest`, etc.
- Keep DB tags, gin handlers, and persistence details inside the owning service.

(Alternative: generate contracts from OpenAPI/proto; but only if you want that.)

---

## Concrete refactor steps (incremental, low risk)
### Phase 1: Pilot the core path (recommended)
Pick the “happy path” for n8n:
1) `workflow` service (definitions)
2) `execution` service (state)
3) `executor` service (runtime)
4) `webhook` + `schedule` (triggers)

For each service:
- `git mv internal/services/<svc> internal/<svc>`
- update imports to `github.com/linkflow-go/internal/<svc>/...`
- keep behavior identical

### Phase 2: Move/reshape domain models
- Move domain models under owning service:
  - workflow graph types under `internal/workflow/domain`
  - execution state types under `internal/execution/domain`
- Only extract to `pkg/contracts` if two+ services truly need the type.

### Phase 3: Introduce ports/adapters where it reduces coupling
Start with `execution`↔`executor` boundary:
- define a `ports.Queue` or `ports.ExecutionDispatcher`
- `execution/app` emits a dispatch command; `executor/adapters/events` consumes it

---

## Acceptance criteria (n8n-clone aligned)
- Core flow still works:
  - create/update workflow → trigger (webhook/schedule) → execution created → executor runs nodes
- `make test` passes
- `make lint` passes
- No cross-service imports (`internal/<svc>` importing other `internal/<svc2>`)

---

## Decisions I need from you to implement this
1) Do you want **Option B only** (flatten paths) first, or **B + Clean slices** (`domain/app/ports/adapters`) as we go?
2) Which service should be the pilot for your n8n clone right now:
   - `workflow` (recommended) or `auth` (easier) or `execution` (most impactful)?
3) Do you want shared contracts placed in `pkg/contracts`, or keep everything service-owned unless forced?
