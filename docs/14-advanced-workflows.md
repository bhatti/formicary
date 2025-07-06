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

The `FORK_JOB` method starts a new job request for the specified `fork_job_type` and immediately proceeds to the next task in the parent job.

```yaml
- task_type: fork-video-job-1
  method: FORK_JOB
  fork_job_type: video-encoding # The job_type of the child job to run
  variables:
    InputFile: "segment_1.mp4"
    OutputQuality: "1080p"
  on_completed: fork-video-job-2
```

### Awaiting Child Jobs (`AWAIT_FORKED_JOB`)

The `AWAIT_FORKED_JOB` method pauses the parent job until all child jobs listed in `await_forked_tasks` have finished. The artifacts from all completed child jobs are automatically downloaded and made available to subsequent tasks in the parent job.

```yaml
- task_type: await-all-videos
  method: AWAIT_FORKED_JOB
  # The list contains the task_type names of the FORK_JOB tasks
  await_forked_tasks:
    - fork-video-job-1
    - fork-video-job-2
  on_completed: combine-results
```

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


