## CI Examples

### Job Definition / Organization Configs
In addition to specifying variables in the job-configuration and pass as request parameters, you can 
store common or sensitive configuration separately, which can be references within the job definition. 
These configurations can be updated using dashboard UI or API, e.g. following example stores
organization specific configurations:

```bash
curl -v -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/yaml" \
  $SERVER/api/orgs/<org-id>/configs -d '{"Name": "MyToken", "Value": "TokenValue"}'
```

Similarly, following example adds configuration for a specific job:
```bash
curl -v -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/yaml" \
  $SERVER/api/jobs/definitions/<job-id>/configs -d '{"Name": "MyToken", "Value": "TokenValue"}'
```


### Access Tokens for Source Code Repositories
The formicary supports encrypted storage for tokens and password that can be used to access the source code repositories. 
For example, if you are using github you can create 
[personal access token](https://docs.github.com/en/github/authenticating-to-github/keeping-your-account-and-data-secure/creating-a-personal-access-token) and 
then checkout code, e.g.:
```bash
git clone https://<your-token>@github.com/<yourid>/<your-project>.git
```

Alternatively, you can use username/password such as:
```bash
git clone https://<username>:<password>@github.com/<yourid>/<your-project>.git
```

Or you can store ssh keys in job-definition / organization configs for accessing the git repositories:
```bash
git clone https://<username>:<password>@github.com/<yourid>/<your-project>.git
```

### CI Job Configuration
Following is an example of job configuration for a simple Node.js project:
```yaml
job_type: node_build
max_concurrency: 1
tasks:
- task_type: build
  method: KUBERNETES
  container:
    image: node:16-buster
  before_script:
    - echo git token {{.GitToken}}
    - git clone https://{{.GitToken}}@github.com/bhatti/node-crud.git sample
  script:
    - cd sample && npm install
  artifacts:
    paths:
      - sample
  cache:
    paths:
      - sample/node_modules
  environment:
    GIT_TERMINAL_PROMPT: 0
    GIT_SSH_COMMAND: 'ssh -oBatchMode=yes'
    GIT_ASKPASS: echo
    SSH_ASKPASS: echo
    GCM_INTERACTIVE: never
  on_completed: test
- task_type: test
  method: KUBERNETES
  container:
    image: node:16-buster
  before_script:
  script:
    - cd sample && chmod 755 ./node_modules/.bin/* && npm test
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

##### Before Script Commands
The `before_script` defines an array of shell commands that are executed before the main script, e.g. `build`
task checks out code in the `before_script`. 
```yaml
  before_script:
    - git clone https://{{.GitToken}}@github.com/bhatti/node-crud.git sample
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
build commands in one task but we separated `test` task for demo, e.g.
```yaml
  script:
    - echo git token {{.GitToken}}
    - cd sample && npm install
```

Note: As we stored `GitToken` as a secured configruation property, the echo command above will be printed as `[MASKED]`.

##### Artifacts
The output of commands can be stored in an artifact-store so that you can easily download it, e.g. build task will store 
entire code directory `sample` in the artifact-store.
```yaml
  artifacts:
    paths:
      - sample
  cache:
    paths:
      - sample/node_modules
  environment:
    GIT_TERMINAL_PROMPT: 0
    GIT_SSH_COMMAND: 'ssh -oBatchMode=yes'
    GIT_ASKPASS: echo
    SSH_ASKPASS: echo
    GCM_INTERACTIVE: never
```

##### NPM Caching
Formicary also provides caching for directories that store 3rd party dependencies such as 
NPM, Gradle, Maven, etc. Following example shows how all node_modules will be cached:

```yaml
  cache:
    paths:
      - sample/node_modules
```

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
The last task won't use this property so the job will end.

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
curl -v -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/yaml" --data-binary node_build.yaml http://$SERVER/api/jobs/definitions
```
You will need to create an API token to access the API using [Authentication](apidocs.md#Authentication) to
the API sever defined by $SERVER environment variable passing token via $TOKEN environment variable.

### Submitting Job Request Manually
You can store the job configuration in a `YAML` file and then upload using dashboard or API such as:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" --data '{"job_type": "node_build", "params": {"Platform": "Ubuntu"}}' $SERVER/api/jobs/requests
```
The above example kicks off `node_build` job that you can see on the dashboard UI.
