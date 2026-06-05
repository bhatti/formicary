# Formicary Orchestration Engine - Consolidated Improvement Plan

## Context

Formicary is a Go-based DAG/orchestration engine (Queen + Ants architecture) for job management, CI/CD, AI workloads, and async processing. This document consolidates all identified gaps vs. Airflow, Dagster, Dagger, Temporal, Argo Workflows, Prefect, and Ray — ranked by priority and effort with corrections for false positives.

---

## Completed: AI Agent Orchestration (2026-05)

The following changes were implemented to support declarative AI coding agent workflows on top of Formicary. Goal: replace imperative bot infrastructure with 4 YAML job definitions and zero custom coordinator code.

### Core Engine Changes

#### `CountByJobTypeAndState` Template Function

**Files changed**:
- `queen/repository/job_request_repository.go` — added `CountByJobTypeAndState(jobType string, states ...common.RequestState) (int64, error)` to the interface
- `queen/repository/job_request_repository_impl.go` — implemented with GORM `COUNT(*)` query scoped by job-type and state list
- `queen/utils/template_helper.go` — added `JobCountQuerier` interface (string-based to avoid import cycles); `ParseTemplateWithQuerier(body, data, querier)` passes the querier into the funcmap; `CountByJobTypeAndState` template function enabled in `skip_if` and task scripts
- `queen/manager/job_manager.go` — added `CountByJobTypeAndState` (typed) and `CountByJobTypeAndStateStrings` (satisfies `utils.JobCountQuerier`) methods
- `queen/types/job_definition.go` — `ShouldSkip(vars, querier)` now accepts a querier parameter (no global state)
- `queen/fsm/job_execution_state.go` — passes `jsm.JobManager` as querier to `ShouldSkip`
- Tests: `queen/repository/job_request_repository_impl_test.go`, `queen/utils/template_helper_test.go`

**What it enables**: `skip_if` expressions can now query the job database directly without an HTTP call or auth token:
```yaml
skip_if: >-
  {{if ge (CountByJobTypeAndState "ai-gh-implement" "PENDING" "EXECUTING") 10}} true {{end}}
```

### Job Definitions

| File | Job Type | Purpose |
|------|----------|---------|
| `docs/examples/ai-gh-issue-picker.yaml` | `ai-gh-issue-picker` | Cron: polls GitHub, submits jobs within capacity |
| `docs/examples/ai-gh-implement.yaml` | `ai-gh-implement` | DAG: plan → implement → fix-tests → review → PR |
| `docs/examples/ai-gh-pr-feedback.yaml` | `ai-gh-pr-feedback` | Addresses PR reviewer comments |
| `docs/examples/ai-gh-cleanup.yaml` | `ai-gh-cleanup` | Cron: removes stale workspaces and merged branches |

All four use `method: KUBERNETES` with `container: image: ghcr.io/formicary-ai/agent-worker:latest`. Switch to `method: SHELL` (remove container blocks) for local dev — scripts are identical.

### Documentation

- `docs/ai-agents.md` — full deployment guide: architecture, prerequisites, credentials, KUBERNETES vs SHELL, worker Dockerfile, capacity management, diagnostics
- `docs/blog-declarative-ai-agents.md` — 9 lessons learned from replacing imperative bot infrastructure with declarative YAML

---

## Current Strengths (No Action Needed)

These capabilities already exist and are well-implemented:

| Capability | Status | Notes |
|---|---|---|
| YAML DAG definitions | Complete | Full Go template support, dynamic task generation |
| Multi-executor | Complete | Docker, K8s, Shell, HTTP methods, Fork/Join |
| Multi-tenancy + RBAC | Complete | Org scoping, role-based permissions |
| Artifact management | Complete | S3-compatible, upload/download between tasks |
| Cron scheduling | Complete | 7-field cron expressions |
| Event triggers (webhooks) | Partial | GitHub webhooks with HMAC verification |
| Retry/fault tolerance | Complete | Job + task retries, exit code routing, allow_failure |
| Manual approval | Complete | Recently overhauled with approve/reject workflow |
| Public plugins + semver | Complete | Plugin system with semantic versioning |
| Conditional execution | Complete | on_exit_code, skip_if, always_run, allow_failure |
| Dynamic DAG generation | Complete | Go templates with range/if/else |
| Rate limiting | Complete | Per-endpoint via tollbooth middleware |
| Tag-based worker routing | Complete | Tags + methods matching for task placement |
| OAuth authentication | Complete | GitHub + Google OAuth2 |
| Notifications | Complete | Slack, email, webhooks |
| Sensor pattern | Complete | EXECUTING exit code for polling |

---

## Priority Matrix

| Priority | Meaning | Timeline |
|---|---|---|
| **P0** | Critical / must-fix now | Weeks 1-4 |
| **P1** | High-value competitive parity | Months 1-3 |
| **P2** | Important differentiation | Months 3-6 |
| **P3** | Future / nice-to-have | Months 6-12 |

| Effort | Meaning |
|---|---|
| **S** | < 1 week (single developer) |
| **M** | 1-3 weeks |
| **L** | 3-6 weeks |
| **XL** | 6-12 weeks |

---

## P0 - Critical (Must-Fix Now)

### 0.1 Proto-First Design / gRPC + REST Gateway

**What**: Define ALL data models, request/response types, and services in Protocol Buffers with gRPC services, buf validation, HTTP/JSON annotations (google.api.http), and Swagger/OpenAPI generation. Replace current JSON/YAML/GORM-only type system with proto-generated code.

**MUST HAVE**:
 - make sure we remove current swagger comments in controllers as we will generate swagger using buf from proto files, also create Makefile target to create openapi.json instead of current docs/swagger.yaml (remove docs/swagger.md)
 - make sure we use industry best practices for middleware for grpc apis like compressions, auth, rate limitting, etc.
 - make sure api gateway for rest api is configured for both rest and grpc apis
 - make sure all proto types are fully documented
 - make sure we preserve annotations for json/yaml/mapstructure/gorm (and be consistent, e.g., json/yaml/mapstructure should be supported for all including omitempty etc)
 - the api should support /api/v1/... format 
 - no need for backward compatibility and legacy make sure we have proper design for api and data model
 - here is most important aspsect - for most of the data model we have both struct and behavior/methods so generated code should follow existing directory structure though sub directories can vary, e.g.,  internal/types queen/types and then create _ext files to add methods for the types so that we can use both struct and types
 - current controllers will be replaced with grpc services so update both code and tests
 - make sure dashboard ui is updated to use new apis if endpoints url changes or types for req/resp
 - make sure efficient performant impl and use best practices and offer recommendations if needed
 - make sure apis have proper docs/comments for swagger and validation (buf) and http aip
 - make sure we separate types for apis like req/resp vs internal types like job-def/task-def so that we can change internal types easily if needed, e.g., perhaps add service folder under v1 for service def and req/resp and common folder for local types or offer better organizations - we can improve current module structure, e.e.g, kubernetes folder/mod for kubernetes related types; admin folder for admin service; health for monitoring/health and then generated code in internal, queen can follow similar folders - note following example doesn't use proper subdir structure and use flat dir with proto files so adjust it

**Why**: 
- No `.proto` files exist anywhere in the codebase today
- Current types are Go structs with JSON + YAML + GORM tags scattered across `queen/types/` and `internal/types/`
- Proto-first enables: efficient binary communication between Queen/Ant, auto-generated SDKs (Go, Python, TypeScript), formal API contracts, buf validation replacing hand-rolled validation, gRPC for internal comm + REST gateway for external clients
- Every serious infrastructure tool (Temporal, Argo, Kubernetes, Envoy) uses proto-first design

**Scope**:

```
proto/
  buf.yaml
  buf.gen.yaml
  formicary/
    v1/
      # Data models
      job_definition.proto      # JobDefinition, TaskDefinition, FanOutConfig
      job_execution.proto       # JobExecution, TaskExecution, ExecutionSnapshot
      job_request.proto         # JobRequest, JobRequestParams
      artifact.proto            # Artifact metadata
      resource.proto            # AntRegistration, ResourceUsage, BasicResource
      user.proto                # User, Organization, Permission
      config.proto              # CommonConfig, SystemConfig, JobDefinitionConfig
      trigger.proto             # TriggerConfig (cron, webhook, event)
      approval.proto            # ApprovalRequest, ApprovalVote
      asset.proto               # Asset, AssetLineage (for future data-aware scheduling)
      
      # Services
      job_service.proto         # CRUD definitions, submit, cancel, restart
      execution_service.proto   # Query, replay, snapshots
      artifact_service.proto    # Upload, download, list
      admin_service.proto       # Users, orgs, config management
      trigger_service.proto     # Trigger CRUD
      health_service.proto      # Health checks, metrics
      
      # Shared
      common.proto              # Pagination, timestamps, error codes, enums
      annotations.proto         # Custom options for GORM mapping hints
```

**Implementation approach**:
1. Install buf, create `buf.yaml` with `buf.build/googleapis/googleapis` and `buf.build/bufbuild/protovalidate` deps
2. Define protos with annotations:
   - `google.api.http` for REST gateway mapping (grpc-gateway)
   - `buf.validate` for field validation (replaces hand-rolled `Validate()` methods)
   - `protoc-gen-openapiv2` annotations for Swagger docs
   - Custom `gorm` options or use `infobloxopen/protoc-gen-gorm` for DB mapping
3. `buf generate` produces: Go gRPC stubs, gateway code, OpenAPI spec, validation code
4. Migrate internal Queen<->Ant communication to gRPC (replacing queue message JSON serialization)
5. Keep REST API via grpc-gateway for external clients (backward-compatible)
6. Phase migration: new endpoints in proto-first, migrate existing endpoints incrementally

**YAML example** (`proto/buf.gen.yaml`):
```yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen/go
    opt: paths=source_relative
  - remote: buf.build/grpc/go
    out: gen/go
    opt: paths=source_relative
  - remote: buf.build/grpc-ecosystem/gateway
    out: gen/go
    opt: paths=source_relative
  - remote: buf.build/grpc-ecosystem/openapiv2
    out: gen/swagger
  - remote: buf.build/bufbuild/validate-go
    out: gen/go
    opt: paths=source_relative
```

**Proto example** (job_service.proto):
```protobuf
syntax = "proto3";
package formicary.v1;

import "google/api/annotations.proto";
import "buf/validate/validate.proto";
import "protoc-gen-openapiv2/options/annotations.proto";

service JobService {
  rpc SubmitJob(SubmitJobRequest) returns (SubmitJobResponse) {
    option (google.api.http) = {
      post: "/api/v1/jobs/requests"
      body: "*"
    };
  }
  rpc GetJobExecution(GetJobExecutionRequest) returns (JobExecution) {
    option (google.api.http) = {
      get: "/api/v1/jobs/executions/{id}"
    };
  }
  rpc CancelJob(CancelJobRequest) returns (CancelJobResponse) {
    option (google.api.http) = {
      post: "/api/v1/jobs/requests/{id}/cancel"
    };
  }
}

message SubmitJobRequest {
  string job_type = 1 [(buf.validate.field).string.min_len = 1];
  map<string, string> params = 2;
  string scheduled_at = 3; // RFC3339 optional
}
```

**Migration strategy**: 
- Phase 1: Define protos, generate code, add gRPC server alongside existing REST
- Phase 2: Internal Queen<->Ant comms migrate to gRPC (huge perf win over JSON queues)
- Phase 3: Deprecate hand-written REST controllers, route through grpc-gateway
- Phase 4: Remove old type definitions, single source of truth in proto

**Effort**: XL | **Priority**: P0

---

### 0.2 Fix Critical TODOs
| `internal/web/web_server.go` | 70 | No token revocation check | DB-backed revocation list with db migrations |
### 1.2 Helm Chart for Production Kubernetes

**What**: Production-grade Helm chart with HA Queen, autoscaling Ants, and dependency subcharts.

**Why**: Every competitor has official Helm charts. Without one, K8s deployment requires manual YAML. Table-stakes for cloud-native adoption.

**Structure**: `deploy/helm/formicary/` with Queen deployment (replicas=2), Ant deployment (HPA), ConfigMaps, Secrets, Ingress, PDB, migration Job (pre-install hook), optional subcharts for Redis/Postgres/MinIO.

**Effort**: M | **Priority**: P1

---

**What**: Address reliability and correctness bugs in the codebase.

**Why**: These are production-safety issues that can cause data loss or incorrect behavior.

| File | Line | Issue | Fix |
|---|---|---|---|
| `queen/fsm/job_execution_state.go` | 534 | Concurrent jobs by user/org race condition | `SELECT ... FOR UPDATE` in `checkMaxConcurrency()` |
| `queen/stats/job_stats_registry.go` | 20 | Stats lost on restart (in-memory only) | Move to Redis HASH |
| `queen/repository/job_execution_repository_impl.go` | 748 | Potential deadlock | Add lock wait timeout + retry |
| `queen/manager/job_manager.go` | 828, 1313 | Background notification failures silently lost | Add error handling + retry |
| `ants/transfer/artifact_transfer.go` | multiple | "TODO this won't work" / "TODO verify" | Fix WebSocket transfer path |

no backward compatibility/legacy/dead code 
**Effort**: M | **Priority**: P0

---

### 0.3 OpenTelemetry Distributed Tracing

**What**: End-to-end tracing across Queen -> Queue -> Ant boundaries using OpenTelemetry.

**Why**: Zero tracing exists despite OTel being a transitive dependency. Cannot debug production workflow failures across distributed components. Table-stakes for any distributed system.

**Implementation**:
- `internal/tracing/provider.go` - TracerProvider init (OTLP exporter)
- `internal/tracing/propagation.go` - Context propagation in queue messages
- Add `TraceHeaders map[string]string` to `MessageEvent` struct
- Instrument: job_scheduler (scheduling spans), job_supervisor (execution spans), task_supervisor (task spans), ant request_handler (extract context), each executor (container/pod spans)
- HTTP middleware via `otelecho` for API endpoints

**Effort**: M | **Priority**: P0

---

### 0.4 Workflow Versioning with Execution Pinning

**What**: Ensure in-flight executions always use the definition version they started with. Add version pinning at submission time if needed. we also added recent change to override version to latest when restarting job, which we should support as well. The intent is to continue using same workflow version unless user specifically passes latest or specific version when restarting failed job.

**Why**: Currently `JobDefinition.Version` increments on update but no mechanism pins an execution to the version it was submitted against. If a definition updates mid-execution, behavior is undefined. This should already work but needs verification and hardening.

**Changes**:
- `JobRequest` stores `DefinitionVersionID` (the specific definition row ID, not just version number)
- `job_execution_state.go` loads definition by version ID, not by latest unless restart overrides
- API: `POST /api/jobs/requests` accepts optional `version` field
- Version listing: `GET /api/jobs/definitions/:type/versions`
- Version promotion: `POST /api/jobs/definitions/:type/versions/:v/promote` (stable/canary alias) - not sure if we need this make recommendations
- note for restart we should use same job version that is pinned unless version is passed as arg but for hard reset we should use latest version.

**Effort**: M | **Priority**: P0

---

## P1 - High Value (Competitive Parity)

### 1.1 Exponential Backoff with Jitter

**What**: Replace linear `noJobsTries += 3` in scheduler with exponential backoff using existing `jpillora/backoff` library.

**Why**: Linear backoff either hammers DB/queue (too aggressive) or delays job pickup (too slow). Also add configurable `BackoffPolicy` to job/task retry definitions.

**Changes**:
- `job_scheduler.go`: Replace `noJobsTries` with `*backoff.Backoff{Min: 100ms, Max: 30s, Factor: 2, Jitter: true}`
- `job_definition.go` + `task_definition.go`: Add `BackoffPolicy` struct for retry delays

**Effort**: S | **Priority**: P1

---

XXXXXXXXXXXXXXXXXXXX XXXXXXXXXXXXXXXXXXXX 
### 1.3 Workflow Composition (Sub-Workflows)

**What**: First-class sub-workflow invocation with input/output mapping and cancel propagation, improving on FORK_JOB/AWAIT_FORKED_JOB.

**Why**: Current fork/join has no input mapping, no output mapping, and no cancel propagation. Temporal has child workflows with full lifecycle management.

**New method**: `SUB_WORKFLOW` with `SubWorkflowConfig`:
```yaml
tasks:
  - task_type: run-etl
    method: SUB_WORKFLOW
    sub_workflow:
      job_type: etl-pipeline
      version: stable
      input_map: { source_table: "{{ .parent_table }}" }
      output_map: { row_count: etl_row_count }
      cancel_on_parent_cancel: true
      wait_for_completion: true
```

**Effort**: M | **Priority**: P1

---

### 1.4 Enhanced Human-in-the-Loop

**What**: Multi-party approvals, SLA timers, escalation policies, approval audit trails.

**Why**: Current approval is single-approver only. Production CI/CD and data pipelines need approval chains for compliance (SOX, SOC2).

**Additions**: `ApprovalRequest` (required_approvals, approver_roles, sla_deadline, escalation_policy), `ApprovalVote` (per-user decisions with comments), Slack/email action buttons, auto-escalation on SLA breach.

**Effort**: M | **Priority**: P1

---

### 1.5 Thin Worker / Edge Computing (WebSocket Queue)

**What**: Make Ant workers minimal — add with WebSocket-based based queue/pubsub similar to redis for comm between ants/queen for task dispatch. Workers should only need: network access to Queen + object-store. goal is to have minimimum dep for ants.

**Why**: Current ants require a full message queue deployment. For edge computing, IoT, and lightweight deployments, workers should be thin clients that connect back to Queen via WebSocket. This also enables:
- Intermittent connectivity (store-and-forward)
- NAT traversal (outbound WebSocket from worker)
- Zero infrastructure at edge (no Redis/Kafka needed)
- Simplified deployment for small teams
- default queue unless overriden for redis/kafka
- defficient connection management to keep low overhead for connections
**Implementation**:
- New queue provider: `WebSocketMessagingProvider` in `internal/queue/client_websocket.go`
  - Ant establishes persistent WebSocket connection to Queen by default unless configured others like redis/kafka
  - Queen pushes task assignments over WebSocket (replaces queue polling)
  - Ant sends results/heartbeats back over same connection
  - Automatic reconnection with exponential backoff
- Queen-side: `queen/gateway/task_dispatch_gateway.go`
  - Manages WebSocket connections from ants
  - Routes tasks to connected ants based on tags/methods
  - Handles ant registration/deregistration on connect/disconnect
- Offline buffer: `ants/offline/buffer.go`
  - Local SQLite buffer for results when disconnected from queen - ants can keep its own local sqlite independent of queeen, clean up after delivery, maintain TTL for limited storage.
  - Sync on reconnection
- Config: `common.queue.provider: WEBSOCKET` enables this mode
- Ant dependencies reduced to: Queen endpoint + S3 endpoint (optional)

**Effort**: L | **Priority**: P1

---

## P2 - Important Differentiation

### 2.1 Fix Scheduler Leader Election

**What**: Replace broken leader election with proper distributed lock.

**Why**: `job_scheduler_subscription.go:127` has TODO: "Failover mode is not working and is sending events to multiple subscribers." Cannot run multiple Queen instances without this. However, single-Queen deployments work fine, making this P2 not P0.

**Implementation**: Redis-based `SET NX EX` lock with TTL renewal, Lua script for safe release. Guard all scheduler methods with `if !leaderElector.IsLeader() { return }`.

**Effort**: M | **Priority**: P2

---

### 2.2 Improve Event-Driven Triggers

**What**: Extend existing event/webhook system with more trigger sources — generic HTTP webhooks (beyond GitHub), S3 event notifications, message queue topics (Kafka/Pulsar/Redis/LocalEventBus).

**Why**: Foundation exists (GitHub webhooks, internal event bus with 11+ event types) but user-facing triggers are limited to cron + GitHub. Need to expose event system as configurable trigger sources.

**Note**: We already have event support for lifecycle - This is NOT missing — it's an enhancement of existing capability.

**Additions**:
- Generic HTTP webhook trigger (configurable path, auth, payload extraction)
- S3 event trigger (poll or notification-based)
- Queue topic trigger (subscribe to Kafka/Pulsar/Redis/LocalEventBus topics)
- `triggers:` section in YAML job definitions
- Trigger deduplication and filtering (Go template expressions)

what should these triggers do? create jobs make recommendations based on other orchestration frameworks like airflow/temporal/dagster?

**Effort**: L | **Priority**: P2

---

### 2.3 Dynamic Task Mapping / Fan-Out

**What**: Single task definition dynamically generates N parallel instances from a runtime variable.

**Why**: Airflow's `expand()`, Temporal child workflows, Argo `withItems`. Current FORK_JOB requires pre-defined separate job definitions.

**YAML syntax**:
```yaml
tasks:
  - task_type: deploy
    method: KUBERNETES
    fan_out:
      source: regions          # context variable (JSON array)
      max_parallel: 5
      fail_fast: false
      item_var: region
    script:
      - deploy --region {{.region}}
```

**Effort**: M | **Priority**: P2

---

### 2.4 OIDC/SAML Authentication Enhancement

**What**: Add OIDC provider support (Okta, Azure AD, Ping) alongside existing Google/GitHub OAuth.

**Why**: Enterprises with Okta/Azure AD need OIDC. However, OAuth2 already works for Google/GitHub — this is enhancement, not missing capability.

**Implementation**: Use `coreos/go-oidc/v3` library, configurable issuer/client/scopes, group-to-org mapping.

**Effort**: L | **Priority**: P2

---

### 2.5 External Secrets Integration

**What**: Support HashiCorp Vault, AWS Secrets Manager, K8s secrets alongside existing DB-encrypted secrets.

**Why**: Current DB encryption works but enterprise security teams prefer dedicated secret stores with rotation and audit. Enhancement, not gap.

**Implementation**: `SecretsProvider` interface with Vault/AWS/K8s backends, `secret_ref:` in YAML variables.

**Effort**: M | **Priority**: P2

---

### 2.6 Data-Aware Scheduling / Asset Tracking

**What**: Named data assets with lineage tracking and freshness-based auto-materialization (Dagster-style).

**Why**: Foundation exists in object-store/artifact system. Enhancement to track assets as first-class entities with freshness policies and dependency graphs.

**Model**: `Asset` with name, produced_by_job, depends_on, freshness_max_lag, last_materialized. Asset scheduler triggers jobs when assets go stale.

**Effort**: XL | **Priority**: P2

---

### 2.7 GPU / AI Workload Support

**What**: Native GPU resource requests, NVIDIA runtime for Docker, GPU-aware ant scheduling.

**Why**: AI/ML is the fastest-growing orchestration use case. Need GPU-aware placement.

**Changes**:
- `BasicResource`: Add `gpu_count`, `gpu_type` fields
- K8s adapter: `nvidia.com/gpu` resource limit + node selector
- Docker executor: `DeviceRequests` + `nvidia` runtime
- Ant registration: GPU capacity tracking

**Effort**: M | **Priority**: P2

---

### 2.9 GitOps Workflow Management

**What**: Sync job definitions from Git repositories. K8s ConfigMap-based sync also supported.

**Why**: Platform teams expect GitOps. Can leverage existing K8s ConfigMap mounting for simpler deployments, or full Git polling for sophisticated setups.

**Implementation options**:
1. K8s ConfigMap: Mount YAML definitions as volume, watch for changes
2. Git sync: Poll repo, diff against DB, auto-update definitions
3. Webhook-triggered: GitHub webhook triggers definition reload

**Effort**: M | **Priority**: P2

---

### 2.10 Cost Tracking / FinOps

**What**: Track compute costs per job, task, user, org with configurable rates.

**Why**: Enterprises need cost allocation for chargebacks. Foundation exists in `resource_usage.go`.

**Implementation**: Calculate cost in `FinalizeTaskState()` based on CPU-hours, memory-GB-hours, GPU-hours. API for cost breakdown by dimensions.

**Effort**: M | **Priority**: P2

---

### 2.11 Edge Worker - Thin Dependencies

**What**: Related to 1.5 (WebSocket queue) — ensure ant binary has minimal compile-time and runtime dependencies.

**Why**: Edge deployment requires small binary, minimal config, outbound-only networking.

**Goals**:
- Ant binary with only: WebSocket client + HTTP client (for S3) + executor (Docker or Shell)
- No Kafka/Pulsar/Redis client libraries linked unless explicitly enabled via build tags
- Static binary option for edge deployment
- ARM64 cross-compilation for IoT/edge devices

**Effort**: M | **Priority**: P2

---

## P3 - Future / Nice-to-Have

### 3.1 Workflow Replay and State Inspection

**What**: Replay failed workflows from any task with state snapshots at each step boundary.

**Why**: Temporal's key differentiator. Currently only retry-from-beginning or retry-last-task. Useful but not critical for initial use cases.

**Implementation**: `ExecutionSnapshot` table, snapshot after each task completion, replay API with `from_task` parameter.

**Effort**: L | **Priority**: P3

---

### 3.2 Workflow Marketplace / Catalog

**What**: Extend public plugin system into searchable, versioned catalog with ratings and usage metrics.

**Why**: Community growth. Existing plugin system is the foundation.

**Effort**: M | **Priority**: P3

---

### 3.3 Increase Ant Test Coverage

**What**: Current ant package: 11 test files for 44 source files (25%). Target: 80%+.

**Priority test files**: K8s executor, Docker runner, request handler, artifact transfer, container registry.

**Effort**: L | **Priority**: P2 (ongoing)

---

### 3.4 UI Modernization

**What**: Modernize the current Go template SSR UI. NOT a full React SPA rewrite — improve existing UI with better UX, real-time updates via existing WebSocket gateway, and Mermaid DAG visualization (already partially done).

**Why**: Full React rewrite is overkill and massive effort. Incremental improvements to existing SSR + client-side enhancements (HTMX, Alpine.js, or Stimulus) give 80% of the value at 20% of the cost.

**Approach**: 
- Add HTMX for real-time partial updates (no full page reloads)
- Improve DAG visualization (Mermaid already integrated)
- Better log streaming UX
- Mobile-responsive layout
- Dark mode

**Effort**: M | **Priority**: P3

---

### 3.5 Durable Execution Semantics

**What**: Survive mid-task crashes with checkpoint/resume (Temporal-style).

**Why**: Long-running AI training or data processing jobs benefit from checkpointing. Complex to implement correctly.

**Implementation**: Activity heartbeating with progress, checkpoint storage, resume from last checkpoint on retry.

**Effort**: XL | **Priority**: P3

---

### 3.6 Signal/Query Running Workflows

**What**: Send signals to and query state of running workflow executions.

**Why**: Temporal's signals/queries enable interactive long-running workflows. Useful for human-in-the-loop beyond approval gates.

**Effort**: L | **Priority**: P3

---

### 3.7 Backfill/Catchup Scheduling

**What**: When a cron job is paused or fails, automatically schedule missed intervals on resume.

**Why**: Critical for data pipelines (Airflow's catchup=True). Less critical for CI/CD or async processing.

**Effort**: M | **Priority**: P3

---

### 3.8 Partition-Aware Scheduling

**What**: Time or key-based partitions for data processing jobs (Dagster-style).

**Why**: Data pipelines process data in partitions (daily, hourly). Enables selective backfill of specific partitions.

**Effort**: L | **Priority**: P3

---

### 3.9 Circuit Breaker Pattern

**What**: Automatic circuit breaking for external service calls (HTTP executor, sub-workflows).

**Why**: Prevents cascade failures. Rate limiting exists but no circuit breaker.

**Effort**: S | **Priority**: P3

---

## Items Excluded (False Positives / Not Needed)

| Previous Suggestion | Why Excluded |
|---|---|
| Python SDK for workflow definition | API + declarative YAML already cover this. Users can use Docker images with any language. Adding a transpile-to-YAML SDK adds maintenance burden without clear value. |
| React SPA UI rewrite | Massive effort, overkill. Incremental SSR improvements with HTMX give most of the value. |
| Full Temporal-style deterministic replay | Extremely complex, requires fundamental architecture change. Snapshot-based replay (P3) covers 90% of use cases. |

---

## Implementation Sequence

### Phase 1: Foundation (Weeks 1-8)

| Week | Items | Deliverables |
|---|---|---|
| 1-2 | 0.2 (Critical TODOs) | Race condition fixes, token revocation, stats to Redis |
| 2-4 | 0.3 (OpenTelemetry) | End-to-end tracing Queen<->Ant |
| 3-4 | 0.4 (Version Pinning) | Execution pinning, version listing API |
| 4-6 | 1.1 (Backoff) + 1.2 (Helm) | Scheduler backoff, production Helm chart |
| 5-8 | 0.1 (Proto-First) - Phase 1 | Proto definitions, buf setup, generate code, add gRPC server |

### Phase 2: Competitive Parity (Months 2-4)

| Month | Items | Deliverables |
|---|---|---|
| 2 | 0.1 Phase 2 (Proto migration) | Migrate Queen<->Ant comms to gRPC |
| 2 | 1.3 (Sub-workflows) | SUB_WORKFLOW method with full lifecycle |
| 2-3 | 1.4 (Approvals) | Multi-party approval chains |
| 3-4 | 1.5 (WebSocket Queue) | Thin worker with WebSocket dispatch |

### Phase 3: Differentiation (Months 4-8)

| Month | Items | Deliverables |
|---|---|---|
| 4 | 2.1 (Leader Election) | HA multi-Queen |
| 4-5 | 2.2 (Event Triggers) | Generic webhooks, S3, queue triggers |
| 5 | 2.3 (Fan-Out) | Dynamic parallel task mapping |
| 5-6 | 2.7 (GPU) + 2.8 (LLM) | AI workload support |
| 6-7 | 2.6 (Data-Aware) | Asset tracking + freshness scheduling |
| 7-8 | 2.9 (GitOps) + 2.10 (Cost) | GitOps sync, cost tracking |

### Phase 4: Polish (Months 8-12)

| Month | Items | Deliverables |
|---|---|---|
| 8-9 | 3.1 (Replay) + 3.6 (Signals) | Workflow debugging |
| 9-10 | 2.4 (OIDC) + 2.5 (Secrets) | Enterprise auth + secrets |
| 10-11 | 3.4 (UI) + 3.2 (Marketplace) | UX improvements, plugin catalog |
| 11-12 | 3.5 (Durable) + 3.7 (Backfill) | Advanced scheduling |

---

## Verification Criteria

After each item:
1. `make build` passes
2. `make test` passes (all existing + new tests)
3. `make test-examples` passes
4. New feature has >90% test coverage
5. Proto changes: `buf lint` + `buf breaking` pass
6. Documentation updated

---

## Competitor Comparison (Post-Implementation)

After completing P0+P1, formicary will match or exceed:

| Capability | Formicary | Temporal | Airflow | Dagster | Argo |
|---|---|---|---|---|---|
| Proto/gRPC API | **Yes** (P0) | Yes | No | No | Yes |
| Distributed Tracing | **Yes** (P0) | Yes | Plugin | Built-in | Plugin |
| Version Pinning | **Yes** (P0) | Yes | Yes | Yes | Yes |
| Helm Chart | **Yes** (P1) | Yes | Yes | Yes | Yes |
| Sub-Workflows | **Yes** (P1) | Yes | SubDAGs | Asset deps | Nested |
| Multi-Party Approval | **Yes** (P1) | Signal | No | No | Suspend |
| Thin Edge Workers | **Yes** (P1) | No | No | No | No |
| HA Leader Election | **Yes** (P2) | Yes | Celery | Cloud | K8s |

After completing P2, formicary differentiates with:
- **Proto-first** gRPC + REST dual-protocol (unique among Go orchestrators)
- **LLM orchestration primitives** (no competitor has this)
- **Edge computing with WebSocket workers** (unique architecture)
- **Data-aware scheduling** in a Go orchestrator (currently Dagster-only in Python)
- **GPU-native scheduling** with cost tracking
