## Node.jS CI/CD Examples


### CI Job Configuration
Following is an example of job configuration for a simple Node.js project:
```yaml
job_type: node_build
max_concurrency: 1
tasks:
- task_type: build
  working_dir: /sample
  container:
    image: node:16-buster
  before_script:
    - git clone https://{{.GitToken}}@github.com/bhatti/node-crud.git .
    - git checkout -t origin/{{.GitBranch}} || git checkout {{.GitBranch}}
    - npm ci --cache .npm --prefer-offline
  script:
    - npm install
    - tar -czf all.tgz *
  after_script:
    - ls -l
  artifacts:
    paths:
      - all.tgz
  cache:
    key_paths:
      - package.json
    paths:
      - node_modules
      - .npm
  environment:
    GIT_TERMINAL_PROMPT: 0
    GIT_SSH_COMMAND: 'ssh -oBatchMode=yes'
    GIT_ASKPASS: echo
    SSH_ASKPASS: echo
    GCM_INTERACTIVE: never
  on_completed: test
- task_type: test
  container:
    image: node:16-buster
  working_dir: /sample
  script:
    - tar -xzf all.tgz
    - npm install mocha chai supertest
    - chmod 755 ./node_modules/.bin/* && npm test
  dependencies:
    - build
```

#### Tasks
The tasks section define the DAG or workflow of the build job where each specifies details for each build step such as:

##### Task Type
The `task_type` defines name of the task, e.g.
```yaml
- task_type: build
```

##### Task method
The `method` defines executor type such as KUBENETES, DOCKER, SHELL, etc:
```yaml
  method: KUBERNETES
```

##### Docker Image
The `image` tag within `container` defines docker-image to use for execution commands, which is `node:16-buster` for node application, e.g.
```yaml
  container:
    image: node:16-buster
```

##### Working Directory
The `working_dir` tag specifies the working directory within the container, e.g.,
```yaml
  working_dir: /sample
```

##### After Script Commands
The `after_script` defines an array of shell commands that are executed after the main script whether the task fails or succeeds, e.g., 
```yaml
  after_script:
    - ls -l
```

##### Before Script Commands
The `before_script` defines an array of shell commands that are executed before the main script, e.g. `build`
task checks out code in the `before_script`. 
```yaml
  before_script:
    - git clone https://{{.GithubToken}}@github.com/bhatti/node-crud.git .
    - git checkout -t origin/{{.GitBranch}} || git checkout {{.GitBranch}}
    - npm ci --cache .npm --prefer-offline
```

Note: We will store `GitToken` as a configuration variable for the job such as:
```bash
curl -v -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/yaml" \
  $SERVER/api/jobs/definitions/<job-id>/configs -d '{"Name": "GitToken", "Value": "<myvalue>", "Secret": true}'
```

The value of `GitToken` will be encrypted before storing in the database and any reference of this value in logs
will be masked or redacted.

##### Script Commands
The `script` defines an array of shell commands that are executed inside container. Though, we could define entire
build commands in one task, but we separated `test` task for demo, e.g.
```yaml
  script:
    - npm install
```

Note: As we stored `GitToken` as a secured configuration property, the echo command above will be printed as `[MASKED]`.

##### Artifacts
The output of commands can be stored in an artifact-store so that you can easily download it, e.g., build task 
will store `all.tgz` in the artifact-store.
```yaml
  artifacts:
    paths:
      - all.tgz
```

##### NPM Caching
Formicary also provides caching for directories that store 3rd party dependencies such as 
NPM, Gradle, Maven, etc. Following example shows how all node_modules will be cached:

```yaml
  cache:
    key_paths:
      - package.json
    paths:
      - node_modules
      - .npm
```

In above example `node_modules` and `.npm` folders will be cached between the runs of the job.

##### Environment Variables
The `environment` section defines environment variables to disable interactive git session so that git checkout
won't ask for the user prompt.

```yaml
  environment:
    GIT_TERMINAL_PROMPT: 0
    GIT_SSH_COMMAND: 'ssh -oBatchMode=yes'
    GIT_ASKPASS: echo
    SSH_ASKPASS: echo
    GCM_INTERACTIVE: never
```

##### Next Task
The next task can be defined using `on_completed`, `on_failed` or `on_exit`, e.g.
```yaml
on_completed: test
```
Above task defines `test` task as the next task to execute when it completes successfully. 
The last task won't use this property, so the job will end.

##### Dependent Artifacts
The artifacts from one task can be used by other tasks, e.g. `test` task is listing `build` tasks under
`dependencies` so all artifacts from those tasks are automatically made available for the task.
```yaml
- task_type: test
  dependencies:
    - build
```

### Uploading Job Definition
You can store the job configuration in a `YAML` file and then upload using dashboard or API such as:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/yaml" \
    --data-binary @node_build.yaml $SERVER/api/jobs/definitions
```
You will need to create an API token to access the API using [Authentication](apidocs.md#Authentication) to
the API sever defined by $SERVER environment variable passing token via $TOKEN environment variable.

### Submitting Job Request Manually
You can then submit the job as follows:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    --data '{"job_type": "node_build", "params": {"Platform": "Ubuntu"}}' $SERVER/api/jobs/requests
```
The above example kicks off `node_build` job that you can see on the dashboard UI.

### Github-Webhooks
See [Github-Webhooks](howto.md#Webhooks) for scheduling above job using GitHub webhooks.

### PostCommit Hooks
See [Post-commit hooks](howto.md#PostCommit) for scheduling above job using git post-commit hooks.

