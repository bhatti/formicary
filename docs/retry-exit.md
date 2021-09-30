## Retries and Exit codes

Following example shows how `exit_codes` can be used for retries and error codes:

```yaml
job_type: retry-job
delay_between_retries: 6s
retry: 2
tasks:
- task_type: first
  method: HTTP_GET
  timeout: 15s
  delay_between_retries: 5s
  script:
    - https://jsonplaceholder.typicode.com/todos/1
  on_exit_code:
    10: RESTART_JOB
  on_completed: second
- task_type: second
  container:
    image: alpine
  retry: 2
  script:
    - exit {{ Random 0 3 }}
  on_exit_code:
    1: FATAL
    2: RESTART_TASK
    3: ERR_XYZ
  on_completed: third
- task_type: third
  container:
    image: alpine
  script:
    - date
```

#### Job Type

The `job_type` defines type of the job, which is `retry-job` in above example.

#### Tasks

The tasks section define the DAG or workflow of the build job where each specifies details for each build step such as:

##### Task Type

The `task_type` defines name of the task, e.g.

```yaml
- task_type: first
```

##### Task method

The `method` defines executor type such as KUBENETES, DOCKER, SHELL, etc:

```yaml
  method: HTTP_GET
```

or 

```yaml
  method: KUBERNETES
```

##### Docker Image

The `image` tag within `container` defines docker-image to use for execution commands, which is `python:3.8-buster` for
the python support, e.g.

```yaml
  container:
    image: alpine
```

##### Script Commands

The `script` defines an array of shell commands that are executed inside container, e.g.,

```yaml
  script:
    - exit {{ Random 0 3 }}
```

##### Next Task

The next task can be defined using `on_completed`, `on_failed` or `on_exit`, e.g.

```yaml
  on_completed: second
```

Above task defines `second` task as the next task to execute when it completes successfully. The last task won't use
this property, so the job will end.

##### Exit Codes

The `second` task defines mapping between exit code and actions, task state or error codes, e.g.

```yaml
  on_exit_code:
    1: FATAL
    2: RESTART_TASK
    3: ERR_XYZ
```

In above example, exit-code 1 will mark the task as `FAILED`, exit-code 2 will restart the entire job, exit-code 3 will mark the task as successful, exit-code 4
sets error code to `ERR_XYZ` and exit code will restart the task. You may set `retry` properties to limit number of job or task properties.

### Uploading Job Definition

You can store the job configuration in a `YAML` file and then upload using dashboard or API such as:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/yaml" \
  --data-binary @retry-job.yaml $SERVER/api/jobs/definitions
```

You will need to create an API token to access the API using [Authentication](apidocs.md#Authentication) to the API
sever defined by $SERVER environment variable passing token via $TOKEN environment variable.

### Submitting Job Request Manually

You can then submit the job as follows:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"job_type": "retry-job", "params": { "Key": "Value" } }' $SERVER/jobs/requests
```

The above example kicks off `retry-job` job with `"Key": "Value"` parameters.
