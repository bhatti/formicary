# Guide: Scheduling & Triggers

A job in Formicary can be initiated in several ways: manually via an API call, automatically on a schedule, or in response to an external event like a Git push.

## 1. Manual Submission

The most direct way to run a job is by submitting a **Job Request** to the Formicary API. This is useful for on-demand tasks or for testing your job definitions.

You can use a tool like `curl` to submit a request.

```bash
curl -X POST http://localhost:7777/api/jobs/requests \
     -H "Content-Type: application/json" \
     -d '{"job_type": "your-job-type"}'
```

### Passing Parameters

You can pass parameters to a job run, which can be used by templates in your job definition.

```bash
curl -X POST http://localhost:7777/api/jobs/requests \
     -H "Content-Type: application/json" \
     -d '{
           "job_type": "go-build-ci",
           "params": {
             "GitBranch": "feature/new-login",
             "GitCommitID": "a1b2c3d4"
           }
         }'
```

## 2. Time-Based Scheduling

### Future-Dated Jobs

You can submit a job request that is scheduled to run at a specific time in the future by including the `scheduled_at` field in your API call. The timestamp must be in RFC3339 format.

```bash
curl -X POST http://localhost:7777/api/jobs/requests \
     -H "Content-Type: application/json" \
     -d '{
           "job_type": "daily-report",
           "scheduled_at": "2024-10-26T02:00:00Z"
         }'
```
The job will remain in the `PENDING` state until the specified time.

### Cron Triggers

For recurring schedules, you can add a `cron_trigger` property directly to your job definition YAML. Formicary's scheduler will automatically create a new job request each time the cron expression matches.

The format uses 7 fields, including seconds, providing fine-grained control.

```yaml
# Format: <second> <minute> <hour> <day-of-month> <month> <day-of-week> <year>
job_type: hourly-cleanup
cron_trigger: "0 0 * * * *" # Runs at the beginning of every hour

tasks:
  - task_type: cleanup
    method: SHELL
    script:
      - echo "Running hourly cleanup..."
```

## 3. Event-Driven Triggers

Formicary supports three types of event-driven triggers that **create JobRequests** when external events occur. Triggers are declared in the `triggers:` section of a job definition YAML.

All trigger types share common fields for filtering, parameter extraction, and deduplication:

| Field | Purpose |
|-------|---------|
| `filter` | Go template expression; event is processed only when the trimmed result is `"true"` |
| `params` | Map of job param names to Go template expressions evaluated against the event |
| `dedup_key` | Go template expression whose result becomes `JobRequest.user_key`; duplicate keys are silently dropped |
| `rate_limit` | Cap on job requests per time window (`max` and `window` fields) |

Template expressions use Go's `text/template` syntax and have access to the same functions as job YAML templates (`split`, `join`, `default`, `hasPrefix`, `trimPrefix`, etc.) plus `atoi` and `atof` for numeric comparisons.

---

### 3.1 HTTP Webhook Triggers

A webhook trigger registers `POST /api/triggers/{job_type}/{trigger_name}` on every Formicary instance. This is the correct endpoint for external services (GitHub, GitLab, Stripe, etc.) to call.

```yaml
job_type: deploy-on-push
triggers:
  - type: webhook
    name: on-github-push
    auth:
      method: hmac_sha256          # hmac_sha256 | bearer_token | api_key_header
      secret_config: WebhookSecret # name of a JobDefinitionConfig holding the secret
      header: X-Hub-Signature-256  # header where the signature appears
    filter: '{{ if eq .Body.ref "refs/heads/main" }}true{{ end }}'
    params:
      branch:     '{{ trimPrefix .Body.ref "refs/heads/" }}'
      commit_sha: '{{ .Body.head_commit.id }}'
      repo:       '{{ .Body.repository.full_name }}'
    dedup_key: 'push-{{ .Body.head_commit.id }}'
    rate_limit:
      max: 20
      window: 1m

tasks:
  - task_type: deploy
    method: SHELL
    script:
      - echo "Deploying {{.repo}}@{{.commit_sha}} (branch {{.branch}})"
```

**Template context fields:**

| Field | Contents |
|-------|----------|
| `.Body` | Parsed JSON request body (map) |
| `.Headers` | HTTP request headers (map of string → string) |
| `.Query` | URL query parameters (map of string → string) |

**Authentication methods:**

| Method | How it works |
|--------|-------------|
| `hmac_sha256` | Verifies `X-Hub-Signature-256` (or custom header) using HMAC-SHA256 over the raw body |
| `bearer_token` | Compares `Authorization: Bearer <value>` against the stored secret |
| `api_key_header` | Compares a named request header against the stored secret |

If `auth` is omitted, the endpoint accepts all requests. **Use only for internal or testing purposes.**

**Setting the secret:**

```bash
# Store the webhook secret as a job config (encrypted at rest)
curl -X POST http://localhost:7777/api/jobs/definitions/<job-id>/configs \
  -H "Authorization: Bearer <YOUR_API_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"Name": "WebhookSecret", "Value": "<YOUR_SECRET>", "Secret": true}'
```

**Testing with curl (no auth):**

```bash
# Unauthenticated webhook for local dev (omit auth: in the trigger definition)
curl -X POST http://localhost:7777/api/triggers/deploy-on-push/on-github-push \
  -H "Content-Type: application/json" \
  -d '{
    "ref": "refs/heads/main",
    "head_commit": {"id": "abc123"},
    "repository": {"full_name": "acme/api"}
  }'
# Response: {"request_id": "01JXYZ..."} — or "" if filtered/deduped
```

**Testing with netcat (minimal HTTP):**

```bash
# Start a local echo server on port 8999 to inspect the formatted payload:
nc -l 8999

# In another terminal, send a test payload:
printf 'POST /api/triggers/deploy-on-push/on-github-push HTTP/1.1\r\nHost: localhost:7777\r\nContent-Type: application/json\r\nContent-Length: 60\r\n\r\n{"ref":"refs/heads/main","head_commit":{"id":"test001"}}' | nc localhost 7777
```

**API response:**

| Scenario | HTTP status | Body |
|----------|-------------|------|
| Job request created | `202 Accepted` | `{"request_id": "01JXY..."}` |
| Filtered or rate limited | `202 Accepted` | `{"filtered": true, "request_id": ""}` |
| Deduped (same `dedup_key`) | `202 Accepted` | `{"request_id": ""}` |
| Auth failure | `401 Unauthorized` | `{"error": "authentication failed"}` |
| Body too large | `413 Request Entity Too Large` | `{"error": "..."}` |

---

### 3.2 S3 / Object-Storage Triggers

An S3 trigger fires when new objects appear in a bucket. Two modes are available:

- **`poll`** (default): Formicary periodically lists objects using `ListObjectsV2` with `StartAfter` to resume from the last processed key. Only the **scheduler leader** runs pollers.
- **`notification`**: Formicary subscribes to a queue topic carrying S3 event notifications (AWS SNS/SQS or MinIO event notifications). Also leader-only.

```yaml
job_type: process-uploaded-file
triggers:
  - type: s3
    name: new-parquet-file
    mode: poll              # poll (default) | notification
    bucket: data-lake
    prefix: incoming/       # only list objects under this prefix
    suffix: .parquet        # only process keys ending with this suffix
    poll_interval: 30s      # how often to check; defaults to jobs.trigger_poll_default_interval
    params:
      object_key:  '{{ .Object.Key }}'
      bucket_name: '{{ .Object.Bucket }}'
      size_bytes:  '{{ .Object.Size }}'
      etag:        '{{ .Object.ETag }}'
    dedup_key: 's3-{{ .Object.ETag }}'
    rate_limit:
      max: 50
      window: 5m

tasks:
  - task_type: transform
    method: SHELL
    script:
      - echo "Processing s3://{{.bucket_name}}/{{.object_key}}"
```

**Template context fields (poll and notification mode):**

| Field | Contents |
|-------|----------|
| `.Object.Key` | S3 object key |
| `.Object.Bucket` | Bucket name |
| `.Object.Size` | Object size in bytes (int64) |
| `.Object.ETag` | ETag (quotes stripped) |
| `.Object.LastModified` | Last-modified timestamp (poll mode only) |

**First-run behavior:** On the first poll cycle, the poller records the current watermark and skips all existing objects. Only objects uploaded *after* the first poll are processed. Use `POST /api/v1/jobs/definitions/{job_type}/triggers/{trigger_name}/state` (DELETE) to reset the watermark.

---

### 3.3 Queue / Message-Bus Triggers

A queue trigger subscribes to a topic on the configured message broker (Redis streams, Kafka, Pulsar, or the in-process channel provider for local dev). Only the **scheduler leader** runs queue subscribers.

```yaml
job_type: order-processor
triggers:
  - type: queue
    name: high-value-orders
    topic: orders.completed
    group: formicary-order-processor  # consumer group name
    shared: false                     # false = each instance gets all messages
    filter: '{{ if gt (atoi (printf "%v" .Message.total)) 1000 }}true{{ end }}'
    params:
      order_id:      '{{ .Message.order_id }}'
      customer_name: '{{ .Message.customer_name }}'
      total:         '{{ printf "%v" .Message.total }}'
    dedup_key: 'order-{{ .Message.order_id }}'
    rate_limit:
      max: 100
      window: 1m

tasks:
  - task_type: process
    method: SHELL
    script:
      - echo "Processing order {{.order_id}} for {{.customer_name}}"
```

**Template context fields:**

| Field | Contents |
|-------|----------|
| `.Message` | Parsed JSON message payload (map). Falls back to raw string if not valid JSON. |
| `.Properties` | Message properties / headers (map of string → string) |

---

### 3.4 Trigger Management APIs

All trigger management endpoints are available via gRPC and REST (auto-generated by grpc-gateway).

```bash
# List runtime state for all triggers on a job definition
curl http://localhost:7777/api/v1/jobs/definitions/deploy-on-push/triggers \
  -H "Authorization: Bearer <token>"

# Reset trigger state (clears S3 poll cursor and rate-limit window)
curl -X DELETE \
  http://localhost:7777/api/v1/jobs/definitions/deploy-on-push/triggers/on-github-push/state \
  -H "Authorization: Bearer <token>"

# Programmatically fire a webhook trigger (bypasses HTTP auth — caller is JWT-authenticated)
curl -X POST \
  http://localhost:7777/api/v1/jobs/definitions/deploy-on-push/triggers/on-github-push/fire \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"payload": "<base64-encoded-json>", "headers": {"Content-Type": "application/json"}}'
```

**Dashboard UI:** Open a job definition in the dashboard and click the **Triggers** tab to see live state (last fired time, rate-limit window counts) and a **Reset** button for each trigger.

---

### 3.5 Rate Limiting

Rate limits are enforced per-trigger using a sliding window stored in `formicary_trigger_states`. If a trigger fires more than `max` times within `window`, subsequent events return `202` with `{"filtered": true}` (webhooks) or are silently dropped (queue/S3).

```yaml
rate_limit:
  max: 10     # allow at most 10 job requests ...
  window: 1m  # ... per 1-minute window
```

The window resets when `now - window_start >= window`. To manually reset a rate-limit counter, use the Reset API or the dashboard Reset button.

---

### 3.6 Deduplication

If `dedup_key` is set, its evaluated value becomes `JobRequest.user_key`. Formicary enforces a unique index on `user_key`, so submitting the same key twice creates only one job request. The second attempt is silently accepted (`request_id: ""`).

```yaml
dedup_key: 'deploy-{{ .Body.head_commit.id }}'
```

This is equivalent to Dagster's `RunRequest(run_key=...)` — idempotent re-triggering is safe.

---

### GitHub Webhooks (legacy integration)

You can also configure a GitHub repository to use Formicary's dedicated GitHub webhook endpoint. This pre-dates the generic `triggers:` feature and provides fixed parameter extraction (GitBranch, GitCommitID, etc.).

**Setup Steps:**

1. **Generate an API Token:** In the Formicary dashboard, go to your user settings and create a new API token.

2. **Configure the Webhook in GitHub:**
   - In your GitHub repository, go to `Settings > Webhooks > Add webhook`.
   - **Payload URL:** `https://<YOUR_FORMICARY_HOST>/api/auth/github/webhook?job_type=<YOUR_JOB_TYPE>`
   - **Content type:** `application/json`
   - **Secret:** Create a strong secret string.

3. **Configure the Secret in Formicary:**
   ```bash
   curl -X POST http://localhost:7777/api/jobs/definitions/<job-id>/configs \
     -H "Authorization: Bearer <YOUR_API_TOKEN>" \
     -H "Content-Type: application/json" \
     -d '{"Name": "GithubWebhookSecret", "Value": "<YOUR_SECRET>", "Secret": true}'
   ```

When the webhook fires, Formicary populates: `GitBranch`, `GitCommitID`, `GitCommitMessage`, `GitRepository`.

For new integrations, prefer the generic `triggers: [{type: webhook, ...}]` approach — it supports any webhook source, not just GitHub.

