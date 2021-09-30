## Sensor or Polling Examples

Following example shows how `exit_codes` with `EXECUTING` state can be used for polling tasks:

```yaml
job_type: sensor-job
tasks:
- task_type: first
  method: HTTP_GET
  environment:
    OLD_ENV_VAR: ijk
  allow_failure: true
  timeout: 15s
  delay_between_retries: 5s
  script:
    {{ if lt .JobElapsedSecs 3 }}
    - https://jsonplaceholder.typicode.com/blaaaaahtodos/1
    {{ else }}
    - https://jsonplaceholder.typicode.com/todos/1
    {{ end }}
  on_completed: second
  on_exit_code:
    404: EXECUTING
- task_type: second
  container:
    image: alpine
  script:
      - echo nonce {{.Nonce}}
      - exit {{ Random 0 5 }}
  on_exit_code:
    1: FAILED
    2: RESTART_JOB
    3: COMPLETED
    4: ERR_BLAH
    5: RESTART_TASK
  on_completed: third
- task_type: third
  container:
    image: alpine
  environment:
    OLD_ENV_VAR: ijk
  script:
    - date > date.txt
    - env NEW_ENV_VAR=xyz
    - echo variable value is $NEW_ENV_VAR
  artifacts:
    paths:
      - date.txt
```

#### Job Type

The `job_type` defines type of the job, which is `sensor-job` in above example.

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
    {{ if lt .JobElapsedSecs 3 }}
    - https://jsonplaceholder.typicode.com/blaaaaahtodos/1
    {{ else }}
    - https://jsonplaceholder.typicode.com/todos/1
    {{ end }}
```

In above example, different URLs will be invoked based on `JobElapsedSecs` value.

##### Artifacts

The output of commands can be stored in an artifact-store so that you can easily download it, e.g.

```yaml
  artifacts:
    paths:
      - date.txt
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
    1: FAILED
    2: RESTART_JOB
    3: COMPLETED
    4: ERR_BLAH
    5: RESTART_TASK
```

In above example, exit-code 1 will mark the task as `FAILED`, exit-code 2 will restart the entire job, exit-code 3 will mark the task as successful, exit-code 4
sets error code to `ERR_BLAH` and exit code will restart the task. You may set `retry` properties to limit number of job or task properties.

### Uploading Job Definition

You can store the job configuration in a `YAML` file and then upload using dashboard or API such as:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/yaml" \
  --data-binary @sensor-job.yaml $SERVER/api/jobs/definitions
```

You will need to create an API token to access the API using [Authentication](apidocs.md#Authentication) to the API
sever defined by $SERVER environment variable passing token via $TOKEN environment variable.

### Submitting Job Request Manually

You can then submit the job as follows:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"job_type": "sensor-job", "params": { "Key": "Value" } }' $SERVER/jobs/requests
```

The above example kicks off `sensor-job` job with `"Key": "Value"` parameters.
