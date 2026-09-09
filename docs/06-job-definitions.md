# Guide: Job & Task Definitions

A **Job Definition** is the heart of Formicary. It's a YAML file where you declare your entire workflow, from the individual tasks to the logic that connects them.

## Job-Level Properties

These properties are defined at the root of your YAML file and apply to the job as a whole.

| Property | Type | Description |
|---|---|---|
| `job_type` | string | **Required.** A unique name for your job (e.g., `go-build-ci`, `daily-etl-report`). |
| `description` | string | Optional. A human-readable description of what the job does. |
| `max_concurrency`| integer | Optional. Limits how many instances of this job can run simultaneously. Defaults to `1`. |
| `tasks` | list | **Required.** A list of all the task definitions that make up this job. |
| `job_variables` | map | Optional. Default template variable values. **Overridden at runtime** by org configs and job submission params. See [Variable Precedence](#variable-precedence) below. |
| `cron_trigger` | string | Optional. A cron expression to run the job on a schedule. See the [Scheduling Guide](./08-scheduling-and-triggers.md). |
| `timeout` | duration | Optional. A duration (e.g., `1h`, `30m`) after which the entire job will be terminated if it hasn't completed. |
| `retry` | integer | Optional. The number of times a failed job should be automatically retried. |
| `delay_between_retries` | duration | Optional. The delay between job retry attempts (e.g., `10s`, `1m`). |
| `webhook` | object | Optional. A webhook to call upon job completion or failure. |
| `skip_if` | template | Optional. A Go template string that, if it renders to "true", will cause the job to be skipped. |
| `public_plugin` | boolean | Optional. If `true`, marks this job definition as a public plugin available to other users. |
| `sem_version` | string | Optional. The semantic version for a public plugin (e.g., `1.2.5`). |

---

## Variable Precedence

Formicary resolves template variables (`{{.VarName}}`) using this priority order (highest wins):

```
job submission params  >  org configs  >  user configs  >  job_variables (YAML defaults)
```

**`job_variables`** in the YAML are defaults only. They are always overridden by org configs set via `--set-configs` or by params passed at submission time.

**Example:** A YAML with `job_variables: BitbucketRepoBranch: "main"` is correct — it provides a safe default. Setting `BitbucketRepoBranch=dev` via org configs (or passing it as a job param) overrides it at runtime without touching the YAML.

**Consequence:** Never hard-code environment-specific values into `job_variables`. Use them as YAML-level defaults and rely on org configs or job params to override per environment.

**Do NOT** work around this by removing variables from the environment section or by reading them directly from secrets — the override chain is the intended design.

---

## Secrets vs Org Configs — What Goes Where

This distinction is critical. Getting it wrong causes silent failures where empty template values override k8s secret values.

| Category | Where to store | Examples |
|----------|---------------|---------|
| **Actual secrets** (tokens, passwords, private keys) | k8s secret (`ai-dev-credentials`) | `BITBUCKET_TOKEN`, `GH_TOKEN`, `SLACK_BOT_TOKEN`, `SSH_PRIVATE_KEY`, `ANTHROPIC_API_KEY` |
| **Normal config** (workspace, repo, usernames, branches, URLs, model IDs) | Org configs (`deploy-ai-workflows.sh --set-configs`) | `BitbucketWorkspace`, `BitbucketRepo`, `BitbucketUsername`, `GH_ORG`, `GH_REPO`, `DefaultTracker`, `BitbucketRepoBranch`, `GitHubRepoBranch` |
| **YAML defaults** (safe fallbacks when no org config is set) | `job_variables` in YAML | Default model names, empty-string placeholders for optional fields |

### Why this matters — Kubernetes env precedence

In a Kubernetes pod, an explicit `env:` entry **always overrides** `envFrom: secretRef:` for the same key.

If a YAML has:
```yaml
env_from:
  - secret_ref: ai-dev-credentials   # has BITBUCKET_WORKSPACE=xxx
environment:
  BITBUCKET_WORKSPACE: "{{.BitbucketWorkspace}}"  # renders to "" if not set in org config
```

The empty string `""` from the template **silently wins** over the secret's `cxxx` value. The pod sees `BITBUCKET_WORKSPACE=""` and the script fails.

### Correct pattern

- Secrets only in `env_from: secret_ref` — never also in `environment:`
- Config values (workspace, repo, etc.) in `environment:` via templates `{{.VarName}}`
- Those template vars set via org configs (`set_org_config "BitbucketWorkspace" "$BITBUCKET_WORKSPACE"`)
- Org configs override `job_variables` defaults at runtime

---

## Task-Level Properties

Each item in the `tasks` list is a task definition.

| Property | Type | Description |
|---|---|---|
| `task_type` | string | **Required.** A unique name for the task within the job (e.g., `build`, `test`, `deploy`). |
| `method` | string | **Required.** The executor to use. See the [Executors Guide](./07-executors.md) for a full list. Examples: `DOCKER`, `KUBERNETES`, `SHELL`, `HTTP_GET`. |
| `script` | list | A list of shell commands to execute. This is the primary work of most tasks. |
| `container` | object | For `DOCKER` or `KUBERNETES` methods. Defines the container image, resource limits, volumes, etc. |
| `services` | list | A list of sidecar containers to run alongside the main task container. Useful for databases or other dependencies. |
| `dependencies` | list | A list of `task_type` names. Artifacts from these tasks will be automatically downloaded into the current task's working directory. |
| `artifacts` | object | Defines files or directories to be uploaded as artifacts upon task completion. |
| `cache` | object | Defines directories to be cached and restored between runs to speed up jobs. |
| `variables` | map | Task-specific variables, available inside the script via templating. |
| `environment` | map | Environment variables to set inside the execution environment (e.g., the container). |
| `tags` | list | A list of tags used to route this task to specific Ant workers. |
| `on_completed` | string | The `task_type` of the next task to run if this one completes successfully. |
| `on_failed` | string | The `task_type` of the next task to run if this one fails. |
| `on_exit_code` | map | A map of exit codes to next actions. This provides more granular control than `on_completed`/`on_failed`. See example below. |
| `allow_failure` | boolean | If `true`, the job will continue even if this task fails. Defaults to `false`. |
| `always_run` | boolean | If `true`, this task will run even if a previous, required task has failed. Ideal for cleanup steps. Defaults to `false`. |
| `retry` | integer | Number of times to retry this specific task if it fails. |
| `timeout` | duration | A timeout specific to this task. |

### Example: `on_exit_code`

The `on_exit_code` property allows for powerful conditional workflows.

```yaml
- task_type: check-status
  script:
    - /usr/bin/check_service
    # This script exits with 0 for success, 1 for warning, 2 for critical error
  on_exit_code:
    "0": next-task-success      # If exit code is 0, run 'next-task-success'
    "1": notify-warning-task    # If exit code is 1, run 'notify-warning-task'
    "2": FATAL                  # If exit code is 2, mark the entire job as FAILED
    COMPLETED: next-task-success # Redundant here, but shows you can use named statuses
    FAILED: notify-failure-task # If the task fails for other reasons (e.g., timeout)
```
