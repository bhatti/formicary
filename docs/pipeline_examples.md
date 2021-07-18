## Pipeline Examples

### Job Configuration
Following example defines job-definition with a simple pipeline:
```yaml
job_type: pipeline-example1
description: Simple Pipeline example
max_concurrency: 1
tasks:
- task_type: task1
  tags:
  - builder
  pod_labels:
    foor: bar
  script:
    - date
  method: DOCKER
  container:
    image: alpine
  on_completed: task2
- task_type: task2
  method: SHELL
  tags:
  - builder
  script:
    - sleep 25
  on_completed: task3
- task_type: task3
  method: KUBERNETES
  tags:
  - builder
  script:
    - echo "{{.param1}} $MYENV"
  container:
    image: alpine
  environment:
    MYENV: foo bar
  variables:
    param1: hello there
```

Above definition defines three tasks, `task1` uses `DOCKER` container to execute `date` command under `script`. 
If the `task1` completes successfully, next task `task2` is executed, which executes `sleep 25` using `SHELL` 
executor. Next, `task3` uses `KUBERNETES` executor to echo parameters that are variables and environment
variables defined in the task definition. You can store above definition in a file 
such as `pipeline-example1.yaml` and then upload to formicary using:
```bash
curl -H "Content-Type: application/yaml" --data-binary @pipeline-example1.yaml http://localhost:7777/jobs/definitions
```
Then submit your job using:

```bash
curl -H "Content-Type: application/json" --data '{"job_type": "pipeline-example1.yaml", "params": {"Platform": "Test"}}' http://localhost:7777/jobs/requests
```

Next, open 'http://localhost:7777/dashboard/jobs/requests?job_state=RUNNING' on your browser to view the running jobs. 
