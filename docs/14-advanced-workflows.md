# Guide: Advanced Workflows

Formicary's real power comes from its advanced features for creating dynamic, parallel, and resilient workflows. This guide covers templating, fork/join patterns, and advanced error handling.

## Dynamic Workflows with Go Templates

Your entire job definition YAML is processed as a [Go template](https://pkg.go.dev/text/template) before being executed. This allows you to dynamically generate tasks, scripts, and configurations.

### Using Variables

You can access parameters passed in a job request or variables defined in the `job_variables` block.

**Example: Simple Substitution**
```yaml
job_variables:
  GREETING: "Hello"
tasks:
- task_type: greet
  script:
    # Assuming `Target` is passed as a job request parameter
    - echo "{{.GREETING}}, {{.Target}}!"
```

### Using Control Structures

You can use template actions like `if/else` and `range` to dynamically alter the workflow. To enable dynamic task generation, you must set `dynamic_template_tasks: true` at the top level of your job.

**Example: Generating Tasks with a `range` Loop**
This definition uses the built-in `Iterate` function to create a chain of 5 tasks (`task-0` to `task-4`).

```yaml
job_type: iterate-job
dynamic_template_tasks: true # Required for generating tasks
tasks:
{{- range $val := Iterate 5 }}
- task_type: task-{{$val}}
  script:
    - echo "Executing task for number {{$val}}"
  container:
    image: alpine
  # Link to the next task, but not for the last one
  {{ if lt $val 4 }}
  on_completed: task-{{ Add $val 1}}
  {{ end }}
{{ end }}
```

## Parallel Execution with Fork/Join

You can execute entire jobs in parallel and wait for their completion using the `FORK_JOB` and `AWAIT_FORKED_JOB` methods.

### Spawning Child Jobs (`FORK_JOB`)

The `FORK_JOB` method starts a new child job and immediately continues the parent. An optional `sub_workflow` block explicitly declares which parent params are forwarded to the child (`input_params`) and which child results are promoted back (`output_variables`). **When `sub_workflow` is absent, no parent variables are forwarded to the child.** All forked children are automatically cascade-cancelled when their parent is cancelled.

```yaml
- task_type: fork-video-job-1
  method: FORK_JOB
  fork_job_type: video-encoding
  sub_workflow:
    input_params:
      - name: InputFile
        value: "{{ .input_file }}"
      - name: OutputQuality
        value: "{{ .output_quality }}"
  on_completed: fork-video-job-2
```

### Awaiting Child Jobs (`AWAIT_FORKED_JOB`)

The `AWAIT_FORKED_JOB` method pauses the parent job until all listed child jobs finish. Artifacts from completed child jobs are automatically made available to subsequent tasks.

```yaml
- task_type: await-all-videos
  method: AWAIT_FORKED_JOB
  await_forked_tasks:
    - fork-video-job-1
    - fork-video-job-2
  on_completed: combine-results
```

### Sub-Workflow Composition

All `FORK_JOB` tasks use the `sub_workflow` block for explicit parameter control — [Temporal](https://temporal.io/)-style child workflow semantics built into Formicary's execution model.

```yaml
- task_type: run-etl
  method: FORK_JOB
  fork_job_type: etl-pipeline
  fork_job_version: "1.0"
  sub_workflow:
    input_params:
      # Each entry: name = child param, value = Go template expression (secrets excluded).
      - name: source_table
        value: "{{ .parent_table }}"
      - name: batch_size
        value: "{{ .batch_size }}"
    output_variables:
      # Each entry: name = child context key, value = parent context key to publish under.
      - name: row_count
        value: etl_row_count
      - name: status
        value: child_status
    wait_for_completion: true  # Combine fork + await in one task step
  on_completed: summarize
```

| Field | Description |
|-------|-------------|
| `input_params` | List of `{name, value}` pairs. `name` is the child job param name; `value` is a Go template expression resolved against the parent context (secrets excluded). When empty or absent, no params are forwarded. |
| `output_variables` | List of `{name, value}` pairs. `name` is the child execution-context variable; `value` is the parent context key to publish under. When empty or absent, all child context variables are promoted verbatim. |
| `wait_for_completion` | `true` — block this task until the child reaches a terminal state (combines fork + await in one step). `false` or absent — fire-and-forget; use a separate `AWAIT_FORKED_JOB` task. |

> **Cancellation**: All forked child jobs are automatically cascade-cancelled when their parent job is cancelled.

**Full example**: see [`docs/examples/sub-workflow-etl.yaml`](examples/sub-workflow-etl.yaml).

## Dynamic Task Fan-Out {#dynamic-task-fan-out}

Fan-out expands a **single task definition** into N parallel instances at runtime, one per element in a JSON array from the job context. This is the Formicary equivalent of Airflow's `expand()`, Temporal child workflows, and Argo's `withItems`.

### Two modes

| Mode | When | How |
|------|------|-----|
| **Task fan-out** | `fork_job_type` is absent in `fan_out` | Dispatches one `TaskRequest` per item directly to ant workers using the task's original execution method. Children share the parent `JobExecutionID` — no separate job records created. |
| **Job fan-out** | `fork_job_type` is set in `fan_out` | Spawns one child `JobRequest` per item using the same `FORK_JOB` machinery. Each child appears as a real job record with `cascade_cancel=true`. Supports full `sub_workflow` input/output variable mapping. |

### How it works internally

```
TaskDefinition.fan_out set?
        │
        ▼
  Validate():
  • Saves ExecutionMethod = original Method (SHELL/KUBERNETES/…)
  • Overwrites Method = FAN_OUT_JOB
  • Routes task to FanOutTasklet (not to ant workers directly)
        │
        ▼
  GetDynamicTaskWithQuerier():
  • Re-parses task from raw_yaml on every execution
  • Copies task.FanOut → executor opts
  • TaskRequest.ExecutorOpts.FanOut is populated for the tasklet
        │
        ▼
  FanOutTasklet.Execute():
  • Reads source variable from job context → JSON array
  •
  • fan_out.fork_job_type set? (IsJobFanOut)
  │
  ├─ NO  → dispatchTasksAndWait()
  │         • Semaphore of size max_parallel controls concurrency
  │         • Per item: ResourceManager.Reserve() → QueueClient.SendReceive()
  │         • Same path as TaskSupervisor.invoke — same OTel tracing
  │         • item_var injected into child TaskRequest.Variables
  │         • Results prefixed {item_var}_{index}_{key} in parent context
  │
  └─ YES → dispatchJobsAndWait()
            • Phase 1: spawn N child JobRequests concurrently
              - CascadeCancel=true, ParentID=parent JobRequestID
              - item_var injected as job param
              - sub_workflow.input_params resolved via Go templates
            • Phase 2: JobWaiter.RunAndWait (same as AWAIT_FORKED_JOB)
            • Results from waiter merged into parent task response
```

### Task fan-out YAML

```yaml
- task_type: deploy
  method: SHELL               # original execution method — preserved in fan_out.ExecutionMethod
  fan_out:
    source: regions           # context variable holding a JSON array
    item_var: region          # variable name injected into each child task
    max_parallel: 2           # at most 2 tasks run at a time (0 = unlimited)
    fail_fast: false          # continue remaining items even if one fails
  script:
    - echo "deploying to {{.region}}"
  on_completed: report
```

### Job fan-out YAML

Set `fork_job_type` inside `fan_out` to spawn a real child `JobRequest` per item. Combine with `sub_workflow` for explicit variable mapping:

```yaml
- task_type: process-datasets
  method: FORK_JOB
  fan_out:
    source: datasets                          # context variable — JSON array
    item_var: dataset                         # injected into each child as a job param
    fork_job_type: io.formicary.etl-child     # activates job fan-out mode
    fork_job_version: "1.0"
    max_parallel: 3
    fail_fast: true
  sub_workflow:
    input_params:
      - name: source_table
        value: "{{ .dataset }}"              # resolved against parent context
    output_variables:
      - name: row_count
        value: etl_row_count                 # promoted to parent as etl_row_count
    wait_for_completion: true
  on_completed: summarize
```

> All fan-out child jobs are automatically cascade-cancelled when their parent is cancelled.

### `fan_out` fields

| Field | Required | Description |
|-------|----------|-------------|
| `source` | ✅ | Name of the job execution context variable holding a JSON array. |
| `item_var` | ✅ | Variable name injected into each child task or job. Available as `{{.item_var}}` in scripts and as a job param in child jobs. |
| `max_parallel` | optional | Maximum concurrent children. `0` or absent means unlimited. |
| `fail_fast` | optional | `true` — cancel remaining siblings on first failure. `false` (default) — collect all results regardless of failures. |
| `fork_job_type` | optional | Registered job type to spawn as child `JobRequest`s. Setting this activates job fan-out mode. |
| `fork_job_version` | optional | Semver of the `fork_job_type` to instantiate (e.g. `"1.0"`). |

### Result aggregation

After all children complete, their output context is merged into the parent task response under prefixed keys:

```
{item_var}_{index}_status         → COMPLETED or FAILED
{item_var}_{index}_exit_code      → exit code from the child
{item_var}_{index}_error_message  → error detail if the child failed
{item_var}_{index}_{custom_key}   → any context variable set by the child

FanOutItemCount   → total number of items dispatched
FanOutSource      → name of the source array variable
FanOutMode        → "task" or "job"
```

**Example** — after fan-out over `regions: ["us-east-1","us-west-2","eu-west-1"]`:
```
region_0_status = COMPLETED
region_1_status = COMPLETED
region_2_status = FAILED
region_2_error_message = helm timeout
FanOutItemCount = 3
FanOutSource = regions
FanOutMode = task
```

### Deploying the examples

```bash
# Deploy all fan-out examples at once:
./docs/examples/deploy-fan-out.sh [BASE_URL]

# Or deploy individually:
curl -X POST http://localhost:7777/api/jobs/definitions \
  -H 'Content-Type: application/yaml' \
  --data-binary @docs/examples/fan-out-task-regions.yaml

# The server also auto-detects YAML when Content-Type is omitted:
curl -X POST http://localhost:7777/api/jobs/definitions \
  --data-binary @docs/examples/fan-out-task-regions.yaml
```

For job fan-out, **register the child job type first**:

```bash
# 1. Register child job type
curl -X POST http://localhost:7777/api/jobs/definitions \
  -H 'Content-Type: application/yaml' \
  --data-binary @docs/examples/sub-workflow-etl-child.yaml

# 2. Register parent job with fan-out
curl -X POST http://localhost:7777/api/jobs/definitions \
  -H 'Content-Type: application/yaml' \
  --data-binary @docs/examples/fan-out-job-etl.yaml
```

### Complete examples

**Task fan-out — SHELL multi-region deploy** ([`fan-out-task-regions.yaml`](examples/fan-out-task-regions.yaml)):

```yaml
job_type: fan-out-task-regions
tasks:
  - task_type: setup
    method: SHELL
    script:
      - echo "Setting up region list"
    variables:
      regions: '["us-east-1","us-west-2","eu-west-1"]'
    on_completed: deploy

  - task_type: deploy
    method: SHELL
    fan_out:
      source: regions
      item_var: region
      max_parallel: 2
      fail_fast: false
    script:
      - echo "Deploying to {{.region}}"
    on_completed: report

  - task_type: report
    method: SHELL
    script:
      - echo "region_0_status={{.region_0_status}}"
      - echo "region_1_status={{.region_1_status}}"
      - echo "total={{.FanOutItemCount}} mode={{.FanOutMode}}"
```

Submit:
```bash
curl -X POST http://localhost:7777/api/jobs/requests \
  -H 'Content-Type: application/json' \
  -d '{"job_type":"fan-out-task-regions"}'
```

**Job fan-out — ETL pipeline per dataset** ([`fan-out-job-etl.yaml`](examples/fan-out-job-etl.yaml)):

```yaml
job_type: fan-out-job-etl
tasks:
  - task_type: setup
    method: SHELL
    variables:
      datasets: '["sales_2024","inventory_2024","orders_2024"]'
    script:
      - echo "preparing"
    on_completed: process-datasets

  - task_type: process-datasets
    method: FORK_JOB
    fan_out:
      source: datasets
      item_var: dataset
      fork_job_type: io.formicary.test.child-etl-workflow
      fork_job_version: "1.0"
      max_parallel: 2
      fail_fast: true
    sub_workflow:
      input_params:
        - name: batch_size
          value: "{{ .batch_size }}"
      output_variables:
        - name: row_count
          value: etl_row_count
      wait_for_completion: true
    on_completed: summarize

  - task_type: summarize
    method: SHELL
    script:
      - echo "total={{.FanOutItemCount}} mode={{.FanOutMode}}"
```

Submit:
```bash
curl -X POST http://localhost:7777/api/jobs/requests \
  -H 'Content-Type: application/json' \
  -d '{"job_type":"fan-out-job-etl"}'
```

**All example files**:
| File | Mode | Description |
|------|------|-------------|
| [`fan-out-task-regions.yaml`](examples/fan-out-task-regions.yaml) | Task | SHELL multi-region deploy |
| [`fan-out-deploy.yaml`](examples/fan-out-deploy.yaml) | Task | Kubernetes multi-region deploy |
| [`fan-out-job-etl.yaml`](examples/fan-out-job-etl.yaml) | Job | ETL pipeline with child jobs + sub_workflow |

> **Note**: In task fan-out mode, no child `JobRequest` records are created — all N sub-tasks run as `TaskRequest`s dispatched directly to ant workers, sharing the parent `JobExecutionID`. In job fan-out mode, N child `JobRequest` records are created with `cascade_cancel=true`.

## Retries and Error Handling

Formicary provides granular control over how to handle task failures.

### Automatic Retries

You can configure retries at both the job and task level.

```yaml
job_type: resilient-job
retry: 3 # If the job fails, retry the entire workflow up to 3 times
delay_between_retries: 30s
tasks:
- task_type: fetch-data
  method: HTTP_GET
  url: "https://api.flaky-service.com/data"
  retry: 5 # If this specific task fails, retry it 5 times before failing the job
  delay_between_retries: 5s
```

### Exponential Backoff for Retries

For resilient retry behaviour under load, use `retry_backoff_policy` instead of a fixed `delay_between_retries`. The policy uses the [jpillora/backoff](https://github.com/jpillora/backoff) algorithm, which multiplies the delay by `factor` on each successive attempt and optionally adds random jitter to avoid thundering-herd problems.

`retry_backoff_policy` can be set at the **job** level (controls how long to wait before re-queuing the whole job after a failure) or at the **task** level (controls the wait between individual task retries within a job run).

```yaml
job_type: resilient-job-with-backoff
retry: 5
retry_backoff_policy:
  min: 1s        # wait at least 1 second before the first retry
  max: 2m        # never wait more than 2 minutes
  factor: 2.0    # double the delay on each attempt: 1s, 2s, 4s, 8s, 16s…
  jitter: true   # add random jitter to spread retries in high-concurrency scenarios

tasks:
- task_type: call-external-api
  method: HTTP_GET
  url: "https://api.example.com/data"
  retry: 4
  retry_backoff_policy:
    min: 500ms
    max: 30s
    factor: 1.5
    jitter: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `min` | duration | `1s` | Starting delay for attempt 0. |
| `max` | duration | `no limit` | Upper bound on the computed delay. |
| `factor` | float | `2.0` | Multiplier applied to the delay each attempt. |
| `jitter` | bool | `false` | Add random noise ±50% to the computed delay. |

When `retry_backoff_policy` is absent, Formicary falls back to the static `delay_between_retries` value (or a small random default if that is also unset).

### Advanced Control with `on_exit_code`

For the most control, use `on_exit_code` to define different workflow paths based on the numeric exit code of a task's script.

| Special Action | Description |
|---|---|
| `FATAL` | Immediately fails the entire job. No further tasks (except `always_run` tasks) will execute. |
| `FAILED` | Marks the task as failed. The job may continue if the task has `allow_failure: true`. |
| `RESTART_TASK`| Marks the task as failed and immediately re-runs it, consuming a `retry` attempt. |
| `RESTART_JOB` | Marks the task as failed and restarts the entire job from the beginning of its failed sequence, consuming a job-level `retry` attempt. |
| `PAUSE_JOB` | Pauses the job. A user must manually restart it from the dashboard. |
| `EXECUTING` | Creates a polling loop. The task will re-run itself after `delay_between_retries` until it returns a different exit code. This is useful for creating "sensor" tasks that wait for an external condition. |

**Example: A Polling Sensor Task**
This task calls an API. If it gets a 404, it waits and tries again. If it gets a 200, it proceeds. Any other error is fatal.

```yaml
- task_type: wait-for-resource
  method: HTTP_GET
  url: https://api.example.com/resource/123
  delay_between_retries: 15s
  retry: 20 # Poll up to 20 times (5 minutes total)
  on_exit_code:
    "200": process-resource # Success
    "404": EXECUTING        # Resource not found yet, poll again
    "FAILED": FATAL         # Any other failure (e.g., 500 server error) is fatal
```


