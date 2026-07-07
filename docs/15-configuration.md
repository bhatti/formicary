# Reference: Configuration

Formicary is configured using YAML files. Configuration values can be overridden by environment variables, which is particularly useful for containerized deployments.

### Configuration Precedence

The system loads configuration in the following order (lower numbers override higher ones):
1.  **Command-line Flags:** (e.g., `--port 8080`)
2.  **Environment Variables:** (e.g., `COMMON_HTTP_PORT=8080`)
3.  **YAML Configuration File:** Specified via the `--config` flag.
4.  **Default YAML Files:** `$HOME/.formicary-queen.yaml` or `$HOME/.formicary-ant.yaml`.
5.  **Application Defaults:** Hard-coded default values.

### Environment Variables

Environment variables directly map to YAML keys. The mapping rule is to convert the YAML path to uppercase and replace dots (`.`) with underscores (`_`).

-   YAML key `common.s3.endpoint` becomes environment variable `COMMON_S3_ENDPOINT`.
-   YAML key `db.data_source` becomes environment variable `DB_DATA_SOURCE`.

---

## Queen Server Configuration

The Queen server is typically configured via a file named `formicary-queen.yaml`.

### Top-Level Keys

| Key | Type | Description |
|---|---|---|
| `common` | Object | Shared configuration between Queen and Ants. See details below. |
| `db` | Object | Database connection settings. |
| `jobs` | Object | Job scheduling, execution, and timeout settings. |
| `smtp` | Object | SMTP settings for sending email notifications. |
| `notify` | Object | Path settings for notification templates. |
| `embedded_ant` | Object | Optional. If present, the Queen server will also run an embedded Ant worker. Its structure is identical to the [Ant Worker Configuration](#ant-worker-configuration). |
| `subscription_quota_enabled` | boolean | If `true`, enables CPU and disk usage quotas based on user subscriptions. |

### `common` Block

This block contains settings shared by both the Queen and Ant services.

| Key | Env Variable | Type | Default | Description |
|---|---|---|---|---|
| `id` | (via `--id` flag) | string | `default-formicary` | A unique ID for this service instance. |
| `http_port` | `COMMON_HTTP_PORT` | int | `7777` | The port for the API and dashboard. |
| `public_dir` | `COMMON_PUBLIC_DIR`| string | `./public/` | Path to static assets for the UI. |
| `external_base_url` | `COMMON_EXTERNAL_BASE_URL` | string | `http://localhost:7777` | The public-facing URL of the server, used for generating links in notifications and OAuth callbacks. |
| `debug` | `COMMON_DEBUG` | boolean | `false` | Enables verbose logging and prints the full configuration on startup. |
| `encryption_key` | `COMMON_ENCRYPTION_KEY` | string | (auto-generated) | A 32-byte key used for encrypting sensitive data in the database. **It's crucial to back this up!** |
| `queue` | Object | Configuration for the messaging system. See `queue` block below. |
| `s3` | Object | Configuration for the S3-compatible object store. See `s3` block below. |
| `redis` | Object | Configuration for Redis. See `redis` block below. |
| `auth` | Object | Authentication settings. See `auth` block below. |
| `monitor_interval`| | duration | `2s` | How often the health monitor checks dependent services. |
| `container_reaper_interval` | | duration | `1m` | How often to check for and terminate orphan containers. |
| `tracing` | Object | | OpenTelemetry distributed tracing. See `tracing` block below. |

#### `common.tracing` Block

Formicary supports OpenTelemetry (OTel) distributed tracing. Traces cover the full request path: HTTP handler → gRPC service → job scheduler → job supervisor → task dispatcher → ant worker. Trace context is propagated across queue messages so every hop appears as a child span in your observability backend.

| Key | Env Variable | Type | Default | Description |
|-----|-------------|------|---------|-------------|
| `enabled` | `COMMON_TRACING_ENABLED` | boolean | `false` | Enable OTel trace export. |
| `endpoint` | `COMMON_TRACING_ENDPOINT` | string | `http://localhost:4318` | OTLP/HTTP exporter endpoint (e.g. Jaeger, Grafana Tempo, Honeycomb). |
| `sample_ratio` | `COMMON_TRACING_SAMPLE_RATIO` | float | `1.0` | Fraction of traces to sample. `1.0` = 100%, `0.1` = 10%. Reduce in high-traffic production deployments. |

**Example configuration:**

```yaml
common:
  tracing:
    enabled: true
    endpoint: http://jaeger:4318
    sample_ratio: 0.1   # sample 10% in production
```

**Span catalogue** — tracer names and span operation names emitted by each component:

| Tracer | Span | Emitted by |
|--------|------|------------|
| `formicary.http` | `HTTP {METHOD} {path}` | Echo HTTP middleware (all API routes) |
| `formicary.grpc` | gRPC method path | gRPC server unary + stream interceptors |
| `formicary.queen` | `job.launch` | Job launcher (consumer, on queue message) |
| `formicary.queen` | `job.supervise` | Job supervisor (full job lifecycle) |
| `formicary.queen` | `task.dispatch` | Task supervisor (producer, sending to ant) |
| `formicary.ant` | `task.receive` | Ant tasklet subscription (consumer) |
| `formicary.ant` | `task.execute` | Request executor (full task lifecycle) |
| `formicary.ant` | `task.pre_process` | Container setup / image pull |
| `formicary.ant` | `task.execute_script` | before_script + script phase |
| `formicary.ant` | `task.post_process` | after_script phase |
| `formicary.trigger` | `trigger.evaluate` | Trigger evaluator |
| `formicary.trigger` | `trigger.submit_job` | Trigger job submitter |
| `formicary.trigger` | `trigger.webhook` | Webhook handler |
| `formicary.trigger` | `trigger.s3_poll` | S3 poll trigger |
| `formicary.trigger` | `trigger.queue_message` | Queue trigger |

#### `common.queue` Block

| Key | Env Variable | Type | Default | Description |
|---|---|---|---|---|
| `provider`|`COMMON_QUEUE_PROVIDER`| string | `REDIS_MESSAGING` | The message queue provider. Options: `REDIS_MESSAGING`, `PULSAR_MESSAGING`, `KAFKA_MESSAGING`, `CHANNEL_MESSAGING` (in-memory), `WEBSOCKET_MESSAGING` (edge/embedded). |
| `endpoints`|`COMMON_QUEUE_ENDPOINTS`| list | `[]` | A list of broker endpoints for Kafka or Pulsar. |
| `token`|`COMMON_QUEUE_TOKEN`| string | `""` | **Ant only.** API JWT used to authenticate the ant to the queen's WebSocket endpoint. Generate via Dashboard → API Tokens. The queen validates this token using its `COMMON_AUTH_JWT_SECRET` — no separate secret needed on the queen. The token must have `token_type=api`; browser session tokens are rejected. |
| `topic_tenant`| | string | `public` | Pulsar topic tenant. |
| `topic_namespace`| | string | `default` | Pulsar topic namespace. |
| `pulsar` | Object | | Pulsar-specific settings. |
| `kafka` | Object | | Kafka-specific settings. |
| `websocket` | Object | | WebSocket messaging settings. Required when `provider: WEBSOCKET_MESSAGING`. See `queue.websocket` block below. |

#### `common.queue.websocket` Block

`WEBSOCKET_MESSAGING` is a lightweight built-in provider designed for edge deployments or local development where installing Redis, Kafka, or Pulsar is undesirable. The queen serves a WebSocket endpoint at `<http_port>/ws/queue`; ants connect to it directly. No external broker process is required.

**Ant clients** maintain an SQLite-backed offline buffer (`buffer_db_path`) — messages sent while disconnected are persisted locally and drained automatically after reconnection.

**Queen configuration** (no `server_endpoint` — it _serves_ connections):

```yaml
common:
  queue:
    provider: WEBSOCKET_MESSAGING
    websocket:
      path: /ws/queue          # endpoint path; default /ws/queue
      ping_interval: 10s
```

**Ant configuration** (`server_endpoint` points at the queen):

```yaml
common:
  queue:
    provider: WEBSOCKET_MESSAGING
    websocket:
      server_endpoint: ws://queen.example.com:7777/ws/queue
      buffer_db_path: /var/lib/formicary/ant-buffer.db
      ping_interval: 10s
      reconnect_min_delay: 1s
      reconnect_max_delay: 30s
```

Full `websocket` field reference:

| Key | Default | Description |
|-----|---------|-------------|
| `server_endpoint` | `""` | Queen WebSocket URL. **Empty = queen mode** (serve). Non-empty = ant mode (connect). |
| `path` | `/ws/queue` | HTTP path for the WebSocket endpoint (queen only). |
| `ping_interval` | `10s` | How often to send WebSocket keepalive ping frames. |
| `reconnect_min_delay` | `1s` | Initial reconnect backoff delay (ants only). |
| `reconnect_max_delay` | `30s` | Maximum reconnect backoff delay (ants only). |
| `buffer_db_path` | `./formicary-buffer.db` | SQLite offline buffer path (ants only). Messages sent while disconnected are stored here and drained after reconnection. |
| `buffer_ttl` | `24h` | How long buffered messages are retained before being discarded. |
| `max_buffer_size` | `10000` | Maximum number of messages held in the offline buffer. |
| `write_timeout` | `10s` | Timeout for writing a single WebSocket frame. |
| `max_message_size` | `1048576` (1 MiB) | Maximum WebSocket message size in bytes. |
| `read_buffer_size` | `4096` | WebSocket upgrader/dialer read buffer size. |
| `write_buffer_size` | `4096` | WebSocket upgrader/dialer write buffer size. |

#### `common.s3` Block

Formicary uses the AWS SDK v2 S3 client for artifact storage. It works with AWS S3, MinIO, SeaweedFS, or any S3-compatible store. For zero-dependency local development, set `local_mode: true` to have the queen start an embedded [SeaweedFS](https://github.com/seaweedfs/seaweedfs) subprocess — no external object store installation needed.

| Key | Env Variable | Type | Default | Description |
|---|---|---|---|---|
| `endpoint` | `COMMON_S3_ENDPOINT`| string|`s3.amazonaws.com`| The S3 API endpoint. For MinIO use `minio:9000`; for embedded SeaweedFS this is set automatically. |
| `access_key_id`|`COMMON_S3_ACCESS_KEY_ID`|string| `localkey` (local mode) | Your S3 access key. |
| `secret_access_key`|`COMMON_S3_SECRET_ACCESS_KEY`|string| `localsecret` (local mode) | Your S3 secret key. |
| `bucket` | `COMMON_S3_BUCKET`| string| `formicary-artifacts` (local mode) | The bucket to use for storing artifacts and caches. |
| `region` | `COMMON_S3_REGION`| string|`us-east-1`| The S3 region. |
| `useSSL` | `COMMON_S3_USE_SSL`| boolean | `false` (local mode), `true` otherwise | Whether to use HTTPS. Set to `false` for local MinIO or embedded SeaweedFS. |
| `prefix` | `COMMON_S3_PREFIX`| string|`formicary/`| A prefix to prepend to all object keys in the bucket. |
| `local_mode` | | boolean | `false` | Start an embedded SeaweedFS subprocess as the artifact store. No external S3 service needed. |
| `local_data_dir` | | string | `./data/seaweedfs` | Directory where SeaweedFS stores its data. Created automatically. |
| `local_weed_bin` | | string | `weed` | Path to the `weed` binary. Must be on `$PATH` or specified as an absolute path. |
| `local_container_host` | | string | `host.docker.internal` | Host used by Docker/Kubernetes helper containers to reach the embedded store. Use `host-gateway` for Linux. |

**Zero-dependency local setup** (queen config):

```yaml
common:
  queue:
    provider: WEBSOCKET_MESSAGING   # no Redis/Kafka needed
    websocket:
      path: /ws/queue
  s3:
    local_mode: true                # start embedded SeaweedFS
    local_data_dir: ./data/seaweedfs
    bucket: formicary-artifacts
    prefix: formicary/
    region: us-east-1
```

**Ant config** for local mode (points at the queen's embedded store):

```yaml
common:
  queue:
    provider: WEBSOCKET_MESSAGING
    websocket:
      server_endpoint: ws://localhost:7777/ws/queue
  s3:
    local_mode: false
    endpoint: localhost:8333        # SeaweedFS S3 port (set automatically by the queen)
    access_key_id: localkey
    secret_access_key: localsecret
    bucket: formicary-artifacts
    prefix: formicary/
    region: us-east-1
    useSSL: false
```

#### `common.redis` Block

| Key | Env Variable | Type | Default | Description |
|---|---|---|---|---|
| `host` | `COMMON_REDIS_HOST` | string | | The hostname or IP address of the Redis server. |
| `port` | `COMMON_REDIS_PORT` | int | `6379` | The port for the Redis server. |
| `password` | `COMMON_REDIS_PASSWORD`| string | | The password for Redis authentication. |

#### `common.auth` Block

| Key | Env Variable | Type | Default | Description |
|---|---|---|---|---|
| `enabled`| `COMMON_AUTH_ENABLED` | boolean | `false`| Enables or disables authentication for the entire system. |
| `jwt_secret`|`COMMON_AUTH_JWT_SECRET`|string | | **Required if enabled.** A strong, 32-byte secret key for signing JWTs. Generate with `openssl rand -base64 32`. |
| `cookie_name` |`COMMON_AUTH_COOKIE_NAME`|string |`formicary-session`| The name of the browser cookie used for session management. |
| `secure` |`COMMON_AUTH_SECURE`|boolean |`false`| Set to `true` in production (requires HTTPS) to mark session cookies as Secure. |
| `google_client_id`|`COMMON_AUTH_GOOGLE_CLIENT_ID`|string | | Client ID from your Google OAuth 2.0 application. |
| `google_client_secret`|`COMMON_AUTH_GOOGLE_CLIENT_SECRET`|string | | Client Secret from your Google OAuth 2.0 application. |
| `google_callback_host`|`COMMON_AUTH_GOOGLE_CALLBACK_HOST`|string |`localhost`| Hostname used in the Google OAuth callback URL. |
| `github_client_id`|`COMMON_AUTH_GITHUB_CLIENT_ID`|string | | Client ID from your GitHub OAuth App. |
| `github_client_secret`|`COMMON_AUTH_GITHUB_CLIENT_SECRET`|string | | Client Secret from your GitHub OAuth App. |
| `github_callback_host`|`COMMON_AUTH_GITHUB_CALLBACK_HOST`|string |`localhost`| Hostname used in the GitHub OAuth callback URL. |

**Keeping secrets out of config files**

Leave the secret fields empty in your YAML files (safe to commit) and supply them via environment variables at runtime. Viper's `AutomaticEnv()` picks them up automatically — env vars always override YAML values.

```bash
# .env.local  (add to .gitignore — never commit this file)
COMMON_AUTH_ENABLED=true
COMMON_AUTH_JWT_SECRET=<output of: openssl rand -base64 32>

# Google OAuth — https://console.cloud.google.com → APIs & Services → Credentials
# Authorized redirect URI: http://<your-host>/auth/google/callback
COMMON_AUTH_GOOGLE_CLIENT_ID=<your-client-id>
COMMON_AUTH_GOOGLE_CLIENT_SECRET=<your-client-secret>

# GitHub OAuth — https://github.com/settings/developers → OAuth Apps
# Authorization callback URL: http://<your-host>/auth/github/callback
COMMON_AUTH_GITHUB_CLIENT_ID=<your-client-id>
COMMON_AUTH_GITHUB_CLIENT_SECRET=<your-client-secret>
```

Run locally:
```bash
source .env.local && ./formicary --config config/formicary-queen.yaml
```

Run with Docker:
```bash
docker run --env-file .env.local -p 7777:7777 plexobject/formicary
```

### `db` Block

| Key | Env Variable | Type | Default | Description |
|---|---|---|---|---|
| `type`|`DB_TYPE`| string | `sqlite` | The database driver. Options: `sqlite`, `mysql`, `postgres`, `sqlserver`. |
| `data_source`|`DB_DATA_SOURCE`| string| | The connection string for the database. |
| `encryption_key`|`DB_ENCRYPTION_KEY`| string| (auto-generated) | A key used to encrypt sensitive configuration values within the database. **It's crucial to back this up!** |

---

## Runtime Configurations (Org & User Configs)

In addition to YAML/env-var configuration, Formicary supports **runtime configuration properties** stored in the database. These are injected as variables into every job execution, allowing credentials and settings to be changed without redeploying.

### Two Scopes

| Scope | API prefix | Who can write | Injected into jobs |
|---|---|---|---|
| **Org Config** | `/api/orgs/{org}/configs` | Org admins | Yes — as base layer |
| **User Config** | `/api/users/configs` | Any authenticated user | Yes — overrides org on collision |

When a job executes, org configs are loaded first. User configs are then merged on top; **user config always wins if the same key appears in both.** This allows individual users to override org defaults (e.g., use a personal API key instead of the shared one).

### Secret Handling

Values marked `secret: true` are encrypted at rest using a key derived from `encryption_key` + an entity salt. They are always returned as `****` from list/get endpoints. Use the `/reveal` endpoint to obtain the plaintext value — this creates an audit record.

If a UI form sends `****` back in a PUT request (because the masked value was displayed), the server detects this and preserves the existing encrypted value rather than overwriting the secret with the literal string `****`.

### Managing Configs via the Dashboard

-   **Org configs:** Dashboard → `Configurations` (visible to org admins).
-   **User configs:** Dashboard → `My Credentials` (visible to all users for their own configs).

### Managing Configs via the CLI / Scripts

Use the deploy helper scripts or call the REST API directly:

```bash
# Store a shared org secret (requires org admin token)
curl -sf -X POST "https://formicary.example.com/api/orgs/${ORG_ID}/configs" \
  -H "Authorization: Bearer ${FORMICARY_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"name":"GithubToken","value":"ghp_...","secret":true}'

# Store a personal user secret
curl -sf -X POST "https://formicary.example.com/api/users/configs" \
  -H "Authorization: Bearer ${FORMICARY_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"name":"AnthropicApiKey","value":"sk-ant-...","secret":true}'
```

See [API Reference — Runtime Configurations](./16-api-reference.md#runtime-configurations) for the full endpoint list.

### `jobs` Block

| Key | Type | Default | Description |
|---|---|---|---|
| `ant_registration_alive_timeout` | duration | `15s` | How long an Ant is considered "alive" without a new registration heartbeat. |
| `job_scheduler_leader_interval` | duration | `15s` | Interval at which the leader election for the scheduler runs. |
| `job_scheduler_check_pending_jobs_interval`| duration | `1s` | How often the lead scheduler checks for pending jobs. |
| `db_object_cache` | duration | `30s` | TTL for cached database objects like job definitions. |
| `max_schedule_attempts` | int | `10000` | Maximum number of times the scheduler will try to find resources for a job before failing it. |

---

## Ant Worker Configuration

A standalone Ant worker is typically configured via a file named `formicary-ant.yaml`.

| Key | Type | Default | Description |
|---|---|---|---|
| `common` | Object | Shared configuration. See the Queen's `common` block for details. |
| `methods` | list | **Required.** A list of executor methods this Ant supports (e.g., `DOCKER`, `KUBERNETES`, `SHELL`). |
| `tags` | list | Optional. A list of tags to identify this worker. Tasks with matching tags will be routed here. |
| `max_capacity`| int | `10` | The maximum number of tasks this Ant can execute concurrently. |
| `output_limit`| int | `67108864` (64MB) | The maximum size in bytes for a task's log output. |
| `docker` | Object | Configuration for the Docker executor. |
| `kubernetes` | Object | Configuration for the Kubernetes executor. |

### `docker` Block

| Key | Env Variable | Type | Default | Description |
|---|---|---|---|---|
| `host`|`DOCKER_HOST`| string | | The Docker daemon socket (e.g., `unix:///var/run/docker.sock` or `tcp://host:port`). |
| `helper_image` | | string |`amazon/aws-cli`| The Docker image to use for the helper container, which manages artifacts. |
| `pull_policy`| | string | `if-not-present`| Image pull policy. Options: `always`, `never`, `if-not-present`. |

### `kubernetes` Block

| Key | Env Variable | Type | Default | Description |
|---|---|---|---|---|
| `kubeconfig`|`KUBECONFIG`| string | | Path to the kubeconfig file. If empty, it will try in-cluster auth then `$HOME/.kube/config`. |
| `cluster_name` |`CLUSTER_NAME`|string | | The name of the cluster context to use from the kubeconfig file. Uses current context if empty. |
| `namespace`|`KUBERNETES_NAMESPACE`|string | `default` | The Kubernetes namespace to create pods in. |
| `helper_image`| | string |`amazon/aws-cli`| The container image for the artifact helper container. |
| `service_account`| | string | `default` | The service account to assign to created pods. |
| `allow_privilege_escalation` | | boolean | `true` | Whether to allow pods to gain more privileges than their parent process. |
| `image_pull_secrets`| | list | `[]` | A list of secret names to use for pulling private container images. |
| `volumes`, `pod_security_context`, etc. | | Object | | Advanced Kubernetes settings that map directly to the PodSpec. See the [Advanced Kubernetes Guide](./12-advanced-workflows.md). |
