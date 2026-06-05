# Event-Driven Triggers

Event-driven triggers create **JobRequests** automatically when external events occur — HTTP webhooks, new S3 objects, or queue messages. This lets Formicary react to the outside world without polling from user code.

> **Quick model**: triggers are like Dagster sensors or Airflow sensors, but simpler — they evaluate a Go template expression and call `SaveJobRequest` when conditions match. No polling loop to write; just declare the trigger in YAML.

---

## Architecture

```
External Event
      │
      ▼
┌─────────────────────────────────────────────────────┐
│                  TriggerManager                     │
│  (leader-only for S3/queue; all nodes for webhook)  │
└──────┬──────────────┬──────────────┬────────────────┘
       │              │              │
  WebhookHandler  S3Poller      QueueSubscriber
  (HTTP POST)   (periodic      (topic callback)
                 ListObjects)
       │              │              │
       └──────────────┴──────────────┘
                       │
                  ┌────▼────┐
                  │Evaluator│  filter → params → dedup_key → rate_limit
                  └────┬────┘
                       │
                  ┌────▼────┐
                  │Submitter│  NewJobRequestFromDefinition → SaveJobRequest
                  └─────────┘
```

**Leader awareness:** S3 pollers and queue subscribers start only on the scheduler leader (determined by heartbeat on `GetJobSchedulerLeaderTopic()`). If the leader changes, triggers start/stop automatically. Webhook routes are registered on all instances since HTTP load balancers can route to any node.

---

## YAML Reference

```yaml
triggers:
  - type: webhook | s3 | queue   # required
    name: <string>                # required; unique within the job definition

    # --- Webhook fields ---
    path: <string>                # informational only; route is always
                                  # POST /api/triggers/{job_type}/{name}
    auth:
      method: hmac_sha256 | bearer_token | api_key_header
      secret_config: <string>     # name of a JobDefinitionConfig holding the secret
      header: <string>            # e.g. "X-Hub-Signature-256" or "X-API-Key"

    # --- S3 fields ---
    mode: poll | notification     # default: poll
    bucket: <string>              # required for s3
    prefix: <string>              # key prefix filter
    suffix: <string>              # key suffix filter (e.g. ".parquet")
    poll_interval: <duration>     # e.g. 30s; default: jobs.trigger_poll_default_interval (60s)
    topic: <string>               # required for notification mode (queue topic for S3 events)

    # --- Queue fields ---
    topic: <string>               # required for queue type
    group: <string>               # consumer group; auto-generated if empty
    shared: <bool>                # default false

    # --- Common fields ---
    filter: '<go-template>'       # fires only when trimmed result == "true"
    params:
      <param-name>: '<go-template>'
    dedup_key: '<go-template>'    # becomes JobRequest.user_key; duplicates dropped silently
    rate_limit:
      max: <int>                  # max job requests allowed in the window
      window: <duration>          # e.g. 1m, 5m, 1h
```

---

## Template Context

Templates are standard Go `text/template` expressions. The data available depends on trigger type.

### Webhook

```
.Body        map  Parsed JSON request body
.Headers     map  HTTP request headers (string → string)
.Query       map  URL query parameters (string → string)
```

### S3 (poll and notification)

```
.Object.Key          string   S3 object key
.Object.Bucket       string   Bucket name
.Object.Size         int64    Size in bytes
.Object.ETag         string   ETag (quotes stripped)
.Object.LastModified *time    Last-modified time (poll mode only)
```

### Queue

```
.Message      interface{}  Parsed JSON payload, or raw string if not valid JSON
.Properties   map          Message properties / headers (string → string)
```

### Available template functions

All standard Formicary template functions are available: `split`, `join`, `default`, `hasPrefix`, `trimPrefix`, `hasSuffix`, `trimSuffix`, `upper`, `lower`, `title`, `replace`, `contains`, `now`, `formatTime`, plus:

| Function | Purpose |
|----------|---------|
| `atoi s` | Parse string to int (returns 0 on parse failure) |
| `atof s` | Parse string to float64 (returns 0.0 on parse failure) |

---

## Examples

### Webhook: CI pipeline on git push

```yaml
job_type: ci-on-push
triggers:
  - type: webhook
    name: on-push
    auth:
      method: hmac_sha256
      secret_config: CIWebhookSecret
      header: X-Hub-Signature-256
    filter: '{{ if eq .Body.ref "refs/heads/main" }}true{{ end }}'
    params:
      branch:     '{{ trimPrefix .Body.ref "refs/heads/" }}'
      commit_sha: '{{ index .Body.head_commit "id" }}'
      repo:       '{{ index .Body.repository "full_name" }}'
    dedup_key: 'ci-{{ index .Body.head_commit "id" }}'
    rate_limit:
      max: 30
      window: 5m
tasks:
  - task_type: build
    method: SHELL
    script: [echo "Building {{.repo}} @ {{.commit_sha}}"]
```

**Test without auth (local dev):**

```bash
# Register the job definition first
curl -X POST http://localhost:7777/api/jobs/definitions \
  -H "Content-Type: application/x-yaml" --data-binary @fixtures/webhook_trigger_job.yaml

# Fire a test event (use on-push-dev trigger which has no auth)
curl -s -X POST http://localhost:7777/api/triggers/webhook-triggered-pipeline/on-push-dev \
  -H "Content-Type: application/json" \
  -d '{"ref":"refs/heads/main","head_commit":{"id":"testcommit001"}}'

# Check request was created
curl -s http://localhost:7777/api/jobs/requests?jobType=webhook-triggered-pipeline | jq .
```

**Test with netcat to inspect the raw HTTP exchange:**

```bash
# Terminal 1: start netcat listener to echo back requests (port must differ from formicary)
nc -l 9000

# Terminal 2: send a raw HTTP request
(printf 'POST /api/triggers/webhook-triggered-pipeline/on-push-dev HTTP/1.1\r\nHost: localhost:7777\r\nContent-Type: application/json\r\nContent-Length: 50\r\n\r\n{"ref":"refs/heads/main","head_commit":{"id":"1"}}'; sleep 1) | nc localhost 7777
```

### S3: process new data files

```yaml
job_type: etl-new-files
triggers:
  - type: s3
    name: incoming-parquet
    bucket: my-data-lake
    prefix: raw/
    suffix: .parquet
    poll_interval: 60s
    params:
      object_key: '{{ .Object.Key }}'
      size_bytes:  '{{ .Object.Size }}'
    dedup_key: 's3-{{ .Object.ETag }}'
    rate_limit:
      max: 200
      window: 10m
tasks:
  - task_type: transform
    method: SHELL
    script: [echo "Transforming s3://my-data-lake/{{.object_key}}"]
```

### Queue: react to high-value orders

```yaml
job_type: fulfill-order
triggers:
  - type: queue
    name: large-orders
    topic: orders.completed
    group: formicary-fulfill
    filter: '{{ if gt (atoi (printf "%v" .Message.total)) 500 }}true{{ end }}'
    params:
      order_id: '{{ .Message.order_id }}'
      total:    '{{ printf "%v" .Message.total }}'
    dedup_key: 'order-{{ .Message.order_id }}'
    rate_limit:
      max: 1000
      window: 1m
tasks:
  - task_type: fulfill
    method: SHELL
    script: [echo "Fulfilling order {{.order_id}} (${{.total}})"]
```

---

## Trigger State

Each trigger has a state row in `formicary_trigger_states`:

| Column | Purpose |
|--------|---------|
| `last_seen_key` | S3 poll cursor — key of the last processed object |
| `last_seen_time` | Timestamp of last successful poll or notification |
| `window_start` | Start of current rate-limit window |
| `window_count` | Job requests created in the current window |

**Reset a trigger's state** (clears cursor and rate-limit window):

```bash
# REST
curl -X DELETE \
  "http://localhost:7777/api/v1/jobs/definitions/{job_type}/triggers/{trigger_name}/state" \
  -H "Authorization: Bearer <token>"

# Dashboard: open job definition → Triggers tab → Reset button
```

---

## Observability

Every trigger evaluation produces an OpenTelemetry span under the `formicary.trigger` tracer:

| Span | Attributes |
|------|-----------|
| `trigger.evaluate` | `trigger.type`, `trigger.name`, `job.type`, `trigger.rate_limited` |
| `trigger.submit_job` | `job.type`, `trigger.name`, `trigger.dedup_key` |
| `trigger.webhook` | `trigger.name`, `job.type`, `http.method` |
| `trigger.s3_poll` | `trigger.name`, `job.type`, `s3.bucket`, `s3.prefix` |
| `trigger.queue_message` | `trigger.name`, `job.type`, `queue.topic` |
| `trigger.s3_notification` | `trigger.name`, `job.type` |

Errors are recorded via `span.RecordError()` and `span.SetStatus(codes.Error, ...)`.

Structured log fields (`logrus.WithFields`) include `Component`, `JobType`, `TriggerName`, and where applicable `RequestID`, `DedupKey`, `TriggerType`.

---

## Failure Modes and Recovery

| Scenario | Behavior |
|----------|----------|
| Filter template error | Returns `500` (webhook) or logs and nacks (queue/S3); trigger skips that event |
| S3 list API error | Logged; poller retries on next tick |
| Queue message non-JSON | Parsed as raw string; template expressions must use `printf "%v"` to access fields |
| Webhook body too large | Returns `413` |
| Dedup collision | Silently accepted; `request_id: ""` in response |
| Rate limit exceeded | `202 {"filtered": true}` (webhook); silently dropped (queue/S3) |
| Leader failover | TriggerManager on new leader reloads all job definitions and starts pollers/subscribers within 15s |

---

## Configuration

Add to `queen/config/server_config.go` → `jobs:` section:

```yaml
jobs:
  trigger_poll_default_interval: 60s   # default S3 poll interval
  trigger_max_per_job_definition: 5    # max triggers per job definition
  trigger_webhook_body_max_bytes: 1048576  # 1 MiB
  trigger_s3_poll_max_objects: 1000    # informational (no current cap on pagination)
  disable_triggers: false              # set true to disable all triggers globally
```

---

## Security Checklist

- **Always configure `auth`** on public-facing webhook triggers. Unauthenticated endpoints accept any POST from the internet.
- **Store secrets as JobDefinitionConfig** with `Secret: true` (encrypted at rest). Never hard-code secrets in YAML.
- **Use `dedup_key`** to make trigger invocations idempotent. Webhooks can be replayed; dedup prevents double-processing.
- **Set `rate_limit`** to protect downstream systems from event storms.
- **Leader-only execution** for S3/queue ensures no duplicate job submissions in HA deployments.
