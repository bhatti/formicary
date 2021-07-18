## Python CI/CD Examples


### CI Job Configuration
Following is an example of job configuration for a simple Python project:
```yaml
job_type: python-ci
max_concurrency: 1
# only run on main branch
filter: {{if ne .GitBranch "main"}} true {{end}}
tasks:
- task_type: test
  method: KUBERNETES
  working_dir: /
  container:
    image: python:3.9-buster
  environment:
    PIP_CACHE_DIR: /.cache/pip
  before_script:
    - python -V
    - pip install virtualenv
    - virtualenv venv
    - chmod 755 venv/bin/activate
    - venv/bin/activate
    - git clone https://github.com/pypa/sampleproject.git sample
  script:
    - cd sample && python setup.py test
  cache:
    key: cache-key
    paths:
      - .cache/pip
      - venv
  on_completed: release
- task_type: release
  method: KUBERNETES
  working_dir: /
  container:
    image: python:3.9-buster
  environment:
    PIP_CACHE_DIR: /.cache/pip
  script:
    - ls -al .cache/pip venv
  cache:
    key: cache-key
    paths:
      - .cache/pip
      - venv
```

#### Job Type
The `job_type` defines type of the job, e.g.
```yaml
job_type: python-ci
```

#### Filtering
The `filter` will not execute the ci/cd job if branch is not main, e.g.,
```yaml
filter: {{if ne .GitBranch "main"}} true {{end}}
```

#### Tasks
The tasks section define the DAG or workflow of the build job where each specifies details for each build step such as:

##### Task Type
The `task_type` defines name of the task, e.g.
```yaml
- task_type: test
```

##### Task method
The `method` defines executor type such as KUBENETES, DOCKER, SHELL, etc:
```yaml
  method: KUBERNETES
```

##### Docker Image
The `image` tag within `container` defines docker-image to use for execution commands, which is `golang:1.16-buster` for node application, e.g.
```yaml
  container:
    image: python:3.9-buster
```

##### Working Directory
The `working_dir` tag specifies the working directory within the container, e.g.,
```yaml
  working_dir: /
```

##### Before Script Commands
The `before_script` defines an array of shell commands that are executed before the main script, e.g. `build`
task checks out code in the `before_script`.
```yaml
  before_script:
    - python -V
    - pip install virtualenv
    - virtualenv venv
    - chmod 755 venv/bin/activate
    - venv/bin/activate
    - git clone https://github.com/pypa/sampleproject.git sample
```

##### Script Commands
The `script` defines an array of shell commands that are executed inside container, e.g.,
```yaml
  script:
    - cd sample && python setup.py test
```

Note: As we stored `GitToken` as a secured configuration property, the echo command above will be printed as `[MASKED]`.

##### Vendor Caching
Formicary also provides caching for directories that store 3rd party dependencies, e.g. 
following example shows how all python libraries can be cached:

```yaml
  cache:
    key: cache-key
    paths:
      - .cache/pip
      - venv
```

In above example `.cache/pip` and `venv` folders will be cached between the runs of the job.
This key allows sharing cache between tasks, e.g., `release` tag is reusing this cache with the same key:
```yaml
- task_type: release
  method: KUBERNETES
  working_dir: /
  container:
    image: python:3.9-buster
  environment:
    PIP_CACHE_DIR: /.cache/pip
  script:
    - ls -al .cache/pip venv
  cache:
    key: cache-key
    paths:
      - .cache/pip
      - venv
```

##### Environment Variables
The `environment` section defines environment variables to disable interactive git session so that git checkout
won't ask for the user prompt.

```yaml
   environment:
     GO111MODULE: on
     CGO_ENABLED: 0
```

##### Next Task
The next task can be defined using `on_completed`, `on_failed` or `on_exit`, e.g.
```yaml
on_completed: test
```
Above task defines `test` task as the next task to execute when it completes successfully.
The last task won't use this property, so the job will end.

### Uploading Job Definition
You can store the job configuration in a `YAML` file and then upload using dashboard or API such as:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/yaml" \
    --data-binary @go-build-ci.yaml $SERVER/api/jobs/definitions
```
You will need to create an API token to access the API using [Authentication](apidocs.md#Authentication) to
the API sever defined by $SERVER environment variable passing token via $TOKEN environment variable.

### Submitting Job Request Manually
You can then submit the job as follows:

```yaml
 curl -v -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    --data '{"job_type": "go-build-ci", "params": {"GitCommitID": "$COMMIT", "GitBranch": "$BRANCH", "GitCommitMessage": "$COMMIT_MESSAGE"}}' $SERVER/api/jobs/requests
```
The above example kicks off `go-build-ci` job that you can see on the dashboard UI.

### Github-Webhooks
See [Github-Webhooks](howto.md#Webhooks) for scheduling above job using GitHub webhooks.

### PostCommit Hooks
See [Post-commit hooks](howto.md#PostCommit) for scheduling above job using git post-commit hooks.

