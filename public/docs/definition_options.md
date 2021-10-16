## Job and Task Definition Options

## Directed Acyclic Graph (DAG)

The formicary uses directed acyclic graph for specifying dependencies between tasks where each task defines a unit of work. 

## Workflow

A workflow/orchestration in formicary can be defined using directed acyclic graph where the execution flow from one to another task in the workflow/DAG is determined by the exit-code 
or status of parent node/task.

## Data Pipeline

A formicary job can spawn multiple jobs and provides building blocks such as tasks, jobs, dags and object-store for storing intermediate results, which can be used to implement data pipelines for processing complex workflows.

## Job

A job specifies workload in terms of directed-acyclic-graph of tasks, where each task specifies environment, commands/APIs and configuration parameters for the execution of the task.

A job defines following properties:

#### cron_trigger

The cron_trigger defines a cron syntax for executing the job periodically, e.g., following job is scheduled every
minute:

```yaml
 job_type: cron-kube-build
 cron_trigger: 0 * * * * * *
 tasks:
   ...
```

In above example, `cron_trigger` will execute this job every minute. Note: you will only need to upload scheduled jobs
without submitting them as they will automatically be scheduled.

You can also submit a job at scheduled time as follows:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"job_type": "hello_world", "scheduled_at": "2025-06-15T00:00:00.0-00:00", "params": { "Target": "bob" } }' $SERVER/api/jobs/requests
```

The above example will kick off `hello_world` job based on `scheduled_at` time in the future, however the job will be
immediately scheduled if the `scheduled_at` is in the past.

#### delay_between_retries

A job can use `retry` option to retry based on specified number of times and the
`delay_between_retries` defines delay between those retries, e.g.,

```yaml
job_type: test-job
retry: 3
delay_between_retries: 10s
```

Above example shows that `test-job` job can be retried up-to 3 times with 10 seconds delay between each retry.

#### description

The `description` is an optional property to specify details about the job, e.g.,

```yaml
job_type: test-job
description: A test job for building a node application.
```

#### filter

The `filter` allows job to skip execution based on a conditional logic using GO template and variables, e.g.

```yaml
job_type: test-job
filter: {{if eq .Target "charlie"}} true {{end}}
```

In above example, the job will not run if `Target` variable is "charlie", e.g., you can pass these parameters when you
submit jobs:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
    --data '{"job_type": "hello_world", "params": { "Target": "charlie" } }' $SERVER/api/jobs/requests
```

Following example shows how you can limit execution on a branch, e.g.,

```yaml
job_type: node_build
filter: {{if ne .GitBranch "main"}} true {{end}}
```

In above example, the scheduled job will not run if `Branch` is not "main", e.g., you can pass these parameters when you
submit jobs:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
    --data '{"job_type": "node_build", "params": { "Branch": "feature-x" } }' $SERVER/api/jobs/requests
```
#### hard_reset_after_retries

When a job fails, and the job is retried manually or automatically via `retry` parameters, only the failed tasks are
executed. However, you can use `hard_reset_after_retries` to reset the job so that all tasks are executed, e.g.

```yaml
job_type: test-job
retry: 10
delay_between_retries: 10s
hard_reset_after_retries: 3
```

In above example, if a job continue to fail after first three retry, then on the fourth retry, all tasks will be
executed. You can also `allow_start_if_completed` property of the task to re-execute a previously successful task on the
job retry.

#### job_type

The `job_type` specifies the type or name of the job, e.g.

```yaml
job_type: test-job
```

#### job_variables

You can define variables on the job level or on a task level where the `job_variables` specifies variables at the job
level, so they are accessible to all tasks, e.g.

```yaml
job_variables:
  OSVersion: 10.1
  Architecture: ARM64
```

The job configuration uses GO template and these variables can be used to initialize any template variables, e.g.

```yaml
script:
  - echo OS version is {{.OSVersion}}
```

#### max_concurrency

The `max_concurrency` defines max number of jobs that can be run concurrently, e.g.

```yaml
job_type: test-job
max_concurrency: 5
```

In above, if multiple jobs for `test-job` are submitted at the same time, at most 5 jobs will run concurrently, while
others will wait in the queue until capacity becomes available.

#### public_plugin

The `public_plugin` indicates the job is a public plugin so it can be shared by any other user in the system. For
example, you may upload a plugin to scan an application for security vulnerabilities and this plugin can be reused by
other people for free or fee based on your license policies, e.g.

#### required_params

The `required_params` specifies list of parameter names that must be defined when submitting a job request, e.g.,

```yaml
job_type: test-job
required_params:
  - Name
  - Age
```

In above example, the `Name` and `Age` parameters must be given when submitting the request, e.g.,

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"job_type": "test-job", "params": { "Name": "Bob", "Age": 30 } }' $SERVER/api/jobs/requests
```

#### resources

The resources can be used to implement locks, mutex or semaphores when executing jobs if they require any external
limited resources such as license keys, API tokens, etc.

#### retry

When a job fails, it can be retried automatically via `retry` parameters, e.g.,

```yaml
job_type: test-job
retry: 10
```

In above example, if a job fails, it will be retried up-to 10 times.

#### sem_version

When using a `public_plugin` to annotate the job as a public plugin, you must specify a semantic version of the public
using `sem_version` property such as:

```yaml
job_type: my-plugin
public_plugin: true
sem_version: 1.2.5
```

#### timeout

The `timeout` property defines the maximum time that a job can take for the execution and if the job takes longer, then
it's cancelled, e.g.,

```yaml
job_type: test-job
timeout: 5m
```

Above configuration specifies job-time out as 5 minutes.

#### tasks

The `tasks` property defines an array of task definitions. The order of tasks is not important as formicary creates a
graph based on dependencies between the tasks for execution. The formicary will reject job definition with any cyclic
dependencies.

### Task definition

A task represents a unit of work that is executed by an ant worker. It defines following properties under task
definition:

#### allow_failure

The `allow_failure` property defines the task is optional and can fail without failing entire job, e.g.,
```yaml
   - task_type: task
     container:
         image: alpine
     allow_failure: true
```

#### allow_start_if_completed

You can retry a failed job manually or automatically using `retry` configuration. Upon restart, only
failed tasks are re-executed, but you can mark certain tasks to execute previously completed task, e.g.
```yaml
   - task_type: task
     container:
         image: alpine
     allow_start_if_completed: true
```

#### always_run

You can mark certain tasks as `always_run` so that they are run even when the job fails. For example, you
may define a cleanup task, which is executed whether the job fails or succeeds, i.e.,
```yaml
   - task_type: task
     container:
         image: alpine
     always_run: true
```

#### artifacts

The artifacts generated by a task can be stored in an artifact-store using following syntax:

```yaml
 - task_type: extract
   script:
     - python -c 'import yfinance as yf;import json;stock = yf.Ticker("{{.Symbol}}");j = json.dumps(stock.info);print(j);' > stock.json
   artifacts:
     paths:
       - stock.json
```

In above example, `stock.json` will be uploaded to the artifact-store so that it can be downloaded or reused by another
task such as:

```yaml
 - task_type: load
   script:
     - cat stock.json
   dependencies:
     - extract
```

In above example, `dependencies` tag indicates that the `load` task depends on `extract` task so the `stock.json` will
be downloaded automatically, which can be then used by the `load` task.

#### after_script
The `after_script` tags is used to list commands that are executed after the main script regardless 
the main script succeeds or fails, e.g.
```yaml
   script:
     - make lint
   after_script:
     - echo cleaning up
```
In above example, `after_script` is executed even if the lint fails.

#### before_script 
The `before_script` tags is used to list commands that are executed before the main script, e.g.
```yaml
   before_script:
     - git clone https://{{.GithubToken}}@github.com/bhatti/go-cicd.git .
     - git checkout -t origin/{{.GitBranch}} || git checkout {{.GitBranch}}
     - go get -u golang.org/x/lint/golint
     - go mod download
     - go mod vendor
   script:
     - make lint
```
In above example, the code is checked out and setup before the main script commands.

#### cache

The cache option allows caching for directories that store 3rd party dependencies, e.g., following example shows how all
node_modules will be cached:

```yaml
   cache:
     key_paths:
       - go.mod
     paths:
       - vendor
```

In above example `vendor` folder will be cached between the runs of the job, and the cache key will be based on contents
of
`go.mod`.

You can also specify a `key` instead of file based `key_paths`, e.g.

```yaml
   cache:
     key: {{.CommitID}}
     paths:
       - vendor
```

This key allows sharing cache between tasks, e.g., `release` tag is reusing this cache with the same key:

```yaml
- task_type: release
  method: KUBERNETES
  script:
    - ls -al .cache/pip venv
  cache:
    key: cache-key
    paths:
      - .cache/pip
      - venv
```
#### delay_between_retries

A task can use `retry` option to retry based on specified number of times and the
`delay_between_retries` defines delay between those retries, e.g.,

```yaml
 - task_type: lint
   method: KUBERNETES
   delay_between_retries: 10s
```

#### description

The `description` is an optional property to specify details about the task , e.g.,

```yaml
 - task_type: lint
   method: KUBERNETES
   description: This task verifies code quality with the lint tool.
```

#### environment

A task can define environment variables that will be available for commands that are executed, e.g.

```yaml
 - task_type: task1
   method: KUBERNETES
   script:
     - echo region is $REGION
   environment:
     REGION: seattle
```

#### except

The `except` property is used to skip task execution based on certain condition, e.g.
```yaml
 - task_type: integ-test
   method: KUBERNETES
   except: {{if ne .Branch "main" }} true {{end}}
   script:
     - make integ-test
```
Above example will skip `integ-test` if the branch is not `main`.

#### task_type

The `task_type` defines type or name of the task, e.g.
```yaml
 - task_type: lint
```


#### method

The method defines executor to use for the task such as 
    - DOCKER 
    - KUBERNETES 
    - SHELL 
    - HTTP_GET 
    - HTTP_POST_FORM 
    - HTTP_POST_JSON 
    - HTTP_PUT_FORM 
    - HTTP_PUT_JSON 
    - HTTP_DELETE 
    - MESSAGING
    - FORK_JOB 
    - AWAIT_FORKED_JOB
    - EXPIRE_ARTIFACTS

```yaml
 - task_type: lint
   method: KUBERNETES
```

#### on_completed/on_failed

The on_completed defines next task to run if task completes successfully and on_failed defines the next task to run if
task fails, e.g.

```yaml
  on_completed: build
  on_failed: cleanup
```

Note: once a non-optional task fails, the entire job is considered `failed` but `on_failed` can be used to execute
post-processing. Alternatively, you can use `always_run` property to execute cleanup tasks.

#### on_exit

In addition to specifying next task based on `on_completed` `on_failed`, you can also use `on_exit` to run the next task
based on exit-code returned by the task. The exit code is independent of task status and is returned by the command
defined in the `script`. Note, `on_exit` defines special exit codes for `COMPLETED` and `FAILED`
so that you can define all exit criteria in one place, e.g.

```yaml
  on_exit_code:
    101: cleanup
    COMPLETED: deploy
```

Following workflow shows example of multiple exit paths from a task, e.g. 


```yaml
job_type: taco-job
tasks:
- task_type: allocate
  container:
    image: alpine
  script:
    - echo allocating
  on_completed: check-date
- task_type: check-date
  container:
    image: alpine
  script:
    - echo monday && exit {{.ExitCode}}
  on_exit_code:
    1: monday
    2: tuesday
    3: friday 
  on_completed: deallocate
- task_type: monday
  container:
    image: alpine
  script:
    - echo monday
  on_completed: deallocate
- task_type: tuesday
  container:
    image: alpine
  script:
    - echo tuesday
  on_completed: taco-tuesday
- task_type: taco-tuesday
  container:
    image: alpine
  script:
    - echo taco tuesday
  on_completed: deallocate
- task_type: friday
  container:
    image: alpine
  script:
    - echo friday
  on_completed: party
- task_type: party
  container:
    image: alpine
  script:
    - echo tgif party
  on_completed: deallocate
- task_type: deallocate
  container:
    image: alpine
  always_run: true
  script:
    - echo deallocating
```

![taco-job](examples/taco-job.png) 

The `check-date` task will execute different tasks based on the exit code defined under `on_exit_code`. Note: the `deallocate` task is always run because it defines `always_run` property as true.


#### services
The `services` configuration allows starting sidecar container(s) with the given image, e.g.
```
- task_type: k8
  script:
    - sleep 30
  method: KUBERNETES
  services:
    - name: redis
      image: redis:6.2.2-alpine
      ports:
        - number: 6379
  container:
    image: ubuntu:16.04
```
Above config specifies starting redis as a sidecar container along with the task container.

#### resources

The resources can be used to implement locks, mutex or semaphores when executing jobs if they require any external
limited resources such as license keys, API tokens, etc.

#### retry

When a task fails, it can be retried automatically via `retry` parameters, e.g.,

```yaml
 - task_type: lint
   method: KUBERNETES
   retry: 3
```

#### tags
The `tags` property is used to route the task to a specific ant worker that supports given tags. When the
ant workers register with the server, they specify the tags and methods that they support so that the server
can route tasks that they support. For example, following task specifies tags for Mac worker:

```yaml
 - task_type: lint
   tags:
    - Mac
```

#### timeout

The `timeout` property defines the maximum time that a task can take for the execution and if the task takes longer, then
it's cancelled, e.g.,

```yaml
 - task_type: lint
    timeout: 5m
```

#### variables

A task can define variables that can be used for scripts as template parameters or pass to the executors, e.g.

```yaml
 - task_type: task2
   method: KUBERNETES
   script:
     - echo "{{.var1}}"
   variables:
     var1: hello there
```

If you need to share variables for all tasks, you can define them on the job level, e.g.,

```yaml
job_variables:
  Target: mytarget
```

The Target variable can also be used for initializing variables in templates.

### Email Notifications
You can configure job to receive email notifications when a job completes successfully or with failure, e.g.,
```yaml
 notify:
   email:
     recipients:
       - myemail@mydomain.cc
     when: always
```
You can specify multiple emails under `recipients` and `when` parameter can take `always`, `onSuccess`, `onFailure` or `never` values.
You can also configure user settings if you need to configure email notifications for all jobs.

### Child Jobs

The formicary allows spawning other related jobs or marketplace plugins from a job, which are run concurrently. The job
definition uses `FORK_JOB` method to spawn the job and `AWAIT_FORKED_JOB` to wait for completion of the spawned job, e.g.,

```yaml
 - task_type: fork-task
   method: FORK_JOB
   fork_job_type: child-job
   on_completed: fork-wait
 - task_type: fork-wait
   method: AWAIT_FORKED_JOB
   await_forked_tasks:
     - fork-task
```

In above example, `fork-task` will fork another job with type `child-job` and then `fork-wait` will 
wait for its completion. The status of `fork-wait` will be set by the job status of `child-job`.

### Expire old artifacts

The `EXPIRE_ARTIFACTS` method can be used to expire old artifacts, e.g.

```yaml
job_type: artifacts-expiration
cron_trigger: 0 * * * *
tasks:
- task_type: expire
  method: EXPIRE_ARTIFACTS
```

In above example, `expire` task will run every hour and expire old artifacts that are no longer needed.


### Messaging
You can implement a customized executor by subscribing to the messaging queue, see [Customized Executor](executors.md#Customized) for example of implementing a messaging executor. 
Following sample job definition shows how to use `MESSAGING` executor:
```yaml
job_type: messaging-job
timeout: 60s
tasks:
- task_type: trigger
  method: MESSAGING
  messaging_request_queue: formicary-message-ant-request
  messaging_reply_queue: formicary-message-ant-response
```

### Templates

The job definition supports GO templates, and you can use variables that are passed by job-request or task definitions,
e.g.

```yaml
- task_type: extract
  method: DOCKER
  container:
    image: python:3.8-buster
  script:
    - python -c 'import yfinance as yf;import json;stock = yf.Ticker("{{.Symbol}}");j = json.dumps(stock.info);print(j);' > stock.json
```

In above example, `Symbol` is defined as a template variable, that can be passed to job-request such as:

```bash
curl -H "Content-Type: application/json" \
    --data '{"job_type": "io.formicary.etl-example1", "params": {"Symbol": "MSFT"}}' 
    $SERVER/jobs/requests
```

In addition, you can also use `if/then` conditions with templates, e.g.

```yaml
   - task_type: task1
     method: DOCKER
     container:
       image: alpine
     script:
       { { if .IsWindowsPlatform } }
       - ls -l > out.txt
       { { else } }
       - find /tmp > out.txt
       { { end } }
```

Above definition will execute different commands under `script` based on `IsWindowsPlatform`, which can be defined as a
variable or passed as a parameter to the request.

### Params

When submitting a job, you can pass request parameters such as:

```bash
curl -H "Content-Type: application/json" \
    --data '{"job_type": "io.formicary.myjob", "params": {"City": "Seattle", "Age": 30, "Flag": true}}' \
    $SERVER/jobs/requests
```

which can be used for as template parameters or pass to the executors, e.g.

```yaml
 - task_type: task3
   method: KUBERNETES
   script:
     { { if .Flag } }
     - echo Lives in {{.Seattle}}
     { { else } }
     - echo Age {{.Age}}
     { { end } }
```

