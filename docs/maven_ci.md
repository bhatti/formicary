## Maven CI/CD Examples


### CI Job Configuration
Following is an example of job configuration for a simple Maven project:
```yaml
job_type: maven-build
tasks:
- task_type: build
  container:
    image: maven:3.8-jdk-11
  working_dir: /sample
  environment:
    MAVEN_CONFIG: /m2_cache
  cache:
    keys:
      - pom.xml
    paths:
      - m2_cache
  before_script:
    - mkdir -p /m2_cache
    - git clone https://github.com/kiat/JavaProjectTemplate.git .
    - echo '<settings xmlns="http://maven.apache.org/SETTINGS/1.0.0" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://maven.apache.org/SETTINGS/1.0.0 https://maven.apache.org/xsd/settings-1.0.0.xsd"> <localRepository>/m2_cache</localRepository></settings>' > settings.xml
  script:
    - find . |head -100 > find1.txt
    - mvn clean -gs settings.xml compile test checkstyle:check spotbugs:check
    - find . |head -100 > find2.txt
  artifacts:
    paths:
      - find1.txt
      - find2.txt
```

#### Job Type
The `job_type` defines type of the job, e.g.
```yaml
job_type: maven-build
```

#### Tasks
The tasks section define the DAG or workflow of the build job where each specifies details for each build step such as:

##### Task Type
The `task_type` defines name of the task, e.g.
```yaml
- task_type: build
```

##### Docker Image
The `image` tag within `container` defines docker-image to use for execution commands, which is `maven:3.8-jdk-11` for java application, e.g.
```yaml
  container:
    image: maven:3.8-jdk-11
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
    - mkdir -p /m2_cache
    - git clone https://github.com/kiat/JavaProjectTemplate.git .
    - echo '<settings xmlns="http://maven.apache.org/SETTINGS/1.0.0" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://maven.apache.org/SETTINGS/1.0.0 https://maven.apache.org/xsd/settings-1.0.0.xsd"> <localRepository>/m2_cache</localRepository></settings>' > settings.xml
```

##### Script Commands
The `script` defines an array of shell commands that are executed inside container, e.g.,
```yaml
  script:
    - mvn clean -gs settings.xml compile test checkstyle:check spotbugs:check
```

Note: As we stored `GitToken` as a secured configuration property, the echo command above will be printed as `[MASKED]`.

##### Vendor Caching
Formicary also provides caching for directories that store 3rd party dependencies, e.g. 
following example shows how all python libraries can be cached:

```yaml
  cache:
    keys:
      - pom.xml
    paths:
      - m2_cache
```


##### Environment Variables
The `environment` section defines environment variables to disable interactive git session so that git checkout
won't ask for the user prompt.

```yaml
   environment:
    MAVEN_CONFIG: /m2_cache
```

### Uploading Job Definition
You can store the job configuration in a `YAML` file and then upload using dashboard or API such as:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/yaml" \
    --data-binary @maven-ci.yaml $SERVER/api/jobs/definitions
```
You will need to create an API token to access the API using [Authentication](apidocs.md#Authentication) to
the API sever defined by $SERVER environment variable passing token via $TOKEN environment variable.

### Submitting Job Request Manually
You can then submit the job as follows:

```yaml
 curl -v -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    --data '{"job_type": "maven-ci", "params": {"GitCommitID": "$COMMIT", "GitBranch": "$BRANCH", "GitCommitMessage": "$COMMIT_MESSAGE"}}' $SERVER/api/jobs/requests
```
The above example kicks off `maven-ci` job that you can see on the dashboard UI.

### Github-Webhooks
See [Github-Webhooks](howto.md#Webhooks) for scheduling above job using GitHub webhooks.

### PostCommit Hooks
See [Post-commit hooks](howto.md#PostCommit) for scheduling above job using git post-commit hooks.

