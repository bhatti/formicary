## Android CI/CD Examples


### CI Job Configuration
Following is an example of job configuration for a simple Android project:
```yaml
job_type: android-app
max_concurrency: 1
tasks:
- task_type: build
  working_dir: /sample
  artifacts:
    paths:
      - find1.txt
  before_script:
    - git clone https://{{.GithubToken}}@github.com/android/sunflower.git .
  script:
    - ./gradlew build
    - find . > find1.txt
  container:
    image: gradle:6.8.3-jdk8
  on_exit_code:
    COMPLETED: unit-tests
- task_type: unit-tests
  working_dir: /sample
  dependencies:
    - build
  artifacts:
    paths:
      - find2.txt
  before_script:
    - git clone https://{{.GithubToken}}@github.com/android/sunflower.git .
  script:
    - ./gradlew test
    - find . > find2.txt
  container:
    image: gradle:6.8.3-jdk8
```

#### Job Type
The `job_type` defines type of the job, e.g.
```yaml
job_type: android-app
```

#### Tasks
The tasks section define the DAG or workflow of the build job where each specifies details for each build step such as:

##### Task Type
The `task_type` defines name of the task, e.g.
```yaml
- task_type: build
```


##### Docker Image
The `image` tag within `container` defines docker-image to use for execution commands, which is `gradle:6.8.3-jdk8` for android application, e.g.
```yaml
  container:
    image: gradle:6.8.3-jdk8
```

##### Working Directory
The `working_dir` tag specifies the working directory within the container, e.g.,
```yaml
  working_dir: /sample
```

##### Before Script Commands
The `before_script` defines an array of shell commands that are executed before the main script, e.g. `build`
task checks out code in the `before_script`.
```yaml
  before_script:
    - git clone https://{{.GithubToken}}@github.com/android/sunflower.git .
```

##### Script Commands
The `script` defines an array of shell commands that are executed inside container, e.g.,
```yaml
  script:
    - ./gradlew build
```

Note: As we stored `GitToken` as a secured configuration property, the echo command above will be printed as `[MASKED]`.

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
    --data-binary @android-ci.yaml $SERVER/api/jobs/definitions
```
You will need to create an API token to access the API using [Authentication](apidocs.md#Authentication) to
the API sever defined by $SERVER environment variable passing token via $TOKEN environment variable.

### Submitting Job Request Manually
You can then submit the job as follows:

```yaml
 curl -v -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    --data '{"job_type": "android-ci", "params": {"GitCommitID": "$COMMIT", "GitBranch": "$BRANCH", "GitCommitMessage": "$COMMIT_MESSAGE"}}' $SERVER/api/jobs/requests
```
The above example kicks off `android-ci` job that you can see on the dashboard UI.

### Github-Webhooks
See [Github-Webhooks](howto.md#Webhooks) for scheduling above job using GitHub webhooks.

### PostCommit Hooks
See [Post-commit hooks](howto.md#PostCommit) for scheduling above job using git post-commit hooks.

