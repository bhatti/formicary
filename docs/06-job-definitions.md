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
| `job_variables` | map | Optional. A map of key-value pairs that are available as template variables to all tasks in the job. |
| `cron_trigger` | string | Optional. A cron expression to run the job on a schedule. See the [Scheduling Guide](./08-scheduling-and-triggers.md). |
| `timeout` | duration | Optional. A duration (e.g., `1h`, `30m`) after which the entire job will be terminated if it hasn't completed. |
| `retry` | integer | Optional. The number of times a failed job should be automatically retried. |
| `delay_between_retries` | duration | Optional. The delay between job retry attempts (e.g., `10s`, `1m`). |
| `webhook` | object | Optional. A webhook to call upon job completion or failure. |
| `skip_if` | template | Optional. A Go template string that, if it renders to "true", will cause the job to be skipped. |
| `public_plugin` | boolean | Optional. If `true`, marks this job definition as a public plugin available to other users. |
| `sem_version` | string | Optional. The semantic version for a public plugin (e.g., `1.2.5`). |

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
