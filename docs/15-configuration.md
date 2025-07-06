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

#### `common.queue` Block

| Key | Env Variable | Type | Default | Description |
|---|---|---|---|---|
| `provider`|`COMMON_QUEUE_PROVIDER`| string | `REDIS_MESSAGING` | The message queue provider. Options: `REDIS_MESSAGING`, `PULSAR_MESSAGING`, `KAFKA_MESSAGING`, `CHANNEL_MESSAGING` (in-memory). |
| `endpoints`|`COMMON_QUEUE_ENDPOINTS`| list | `[]` | A list of broker endpoints for Kafka or Pulsar. |
| `topic_tenant`| | string | `public` | Pulsar topic tenant. |
| `topic_namespace`| | string | `default` | Pulsar topic namespace. |
| `pulsar` | Object | Pulsar-specific settings. |
| `kafka` | Object | Kafka-specific settings. |

#### `common.s3` Block

| Key | Env Variable | Type | Default | Description |
|---|---|---|---|---|
| `endpoint` | `COMMON_S3_ENDPOINT`| string|`s3.amazonaws.com`| The S3 API endpoint. For MinIO, this would be `minio:9000`. |
| `access_key_id`|`COMMON_S3_ACCESS_KEY_ID`|string| | Your S3 access key. |
| `secret_access_key`|`COMMON_S3_SECRET_ACCESS_KEY`|string| | Your S3 secret key. |
| `bucket` | `COMMON_S3_BUCKET`| string| | The bucket to use for storing artifacts and caches. |
| `region` | `COMMON_S3_REGION`| string|`us-west-2`| The S3 region. |
| `useSSL` | `COMMON_S3_USE_SSL`| boolean | `true` | Whether to use HTTPS to connect to the S3 endpoint. Set to `false` for local MinIO. |
| `prefix` | `COMMON_S3_PREFIX`| string|`formicary/`| A prefix to prepend to all object keys in the bucket. |

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
| `jwt_secret`|`COMMON_AUTH_JWT_SECRET`|string | | **Required if enabled.** A strong, 32-byte secret key for signing JWTs. |
| `cookie_name` |`COMMON_AUTH_COOKIE_NAME`|string |`formicary-session`| The name of the browser cookie used for session management. |
| `github_client_id`|`COMMON_AUTH_GITHUB_CLIENT_ID`|string | | Client ID from your GitHub OAuth App. |
| `github_client_secret`|`COMMON_AUTH_GITHUB_CLIENT_SECRET`|string | | Client Secret from your GitHub OAuth App. |

### `db` Block

| Key | Env Variable | Type | Default | Description |
|---|---|---|---|---|
| `type`|`DB_TYPE`| string | `sqlite` | The database driver. Options: `sqlite`, `mysql`, `postgres`, `sqlserver`. |
| `data_source`|`DB_DATA_SOURCE`| string| | The connection string for the database. |
| `encryption_key`|`DB_ENCRYPTION_KEY`| string| (auto-generated) | A key used to encrypt sensitive configuration values within the database. **It's crucial to back this up!** |

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
