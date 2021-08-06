## Scanning GO projects using gosec

gosec is a Golang security checker (https://github.com/securego/gosec) that inspects source code for security problems. Following
example shows scanning a GO project using gosec:

```yaml
job_type: gosec-job
url: https://github.com/securego/gosec
max_concurrency: 1
job_variables:
  GitRepo: go-cicd
tasks:
- task_type: scan
  working_dir: /sample
  container:
    image: securego/gosec
  before_script:
    - git clone https://{{.GithubToken}}@github.com/bhatti/{{.GitRepo}}.git .
    - git checkout -t origin/{{.GitBranch}} || git checkout {{.GitBranch}}
  script:
    - echo branch {{.GitBranch}}, Commit {{.GitCommitID}}
    - gosec -no-fail -fmt sarif -out results.sarif ./...
  after_script:
    - ls -l
  artifacts:
    paths:
      - results.sarif
```

#### Job Type

The `job_type` defines type of the job, e.g.

```yaml
job_type: gosec-job
```

#### URL

The `url` defines external URL about the job, e.g.,

```yaml
url: https://github.com/securego/gosec
```

#### Concurrency

The `max_concurrency` defines maximum jobs that can be executed concurrently, e.g.,

```yaml
max_concurrency: 1
```

#### Job Variables

The `job_variables` defines variables that are accessible for entire job and can be used in template variables, e.g.,

```yaml
job_variables:
  GitRepo: go-cicd
```

#### Tasks

The tasks section define the DAG or workflow of the build job where each specifies details for each build step such as:

##### Task Type

The `task_type` defines name of the task, e.g.

```yaml
- task_type: scan
```

##### Working Directory

The `working_dir` defines default directory for the scripts, e.g.,

```yaml
  working_dir: /sample
```

##### Docker Image

The `image` tag within `container` defines docker-image to use for execution commands, e.g.,

```yaml
  container:
    image: securego/gosec
```


##### Before Script Commands

The `before_script` defines an array of shell commands that are executed before the main script, e.g.,

```yaml
    before_script:
    - git clone https://{{.GithubToken}}@github.com/bhatti/{{.GitRepo}}.git .
    - git checkout -t origin/{{.GitBranch}} || git checkout {{.GitBranch}}
```

##### Script Commands

The `script` defines an array of shell commands that are executed inside container, e.g.,

```yaml
  script:
    - echo branch {{.GitBranch}}, Commit {{.GitCommitID}}
    - gosec -no-fail -fmt sarif -out results.sarif ./...
```

##### Artifacts

Formicary allows uploading artifacts from the task output, e.g.

```yaml
    artifacts:
      paths:
      - results.sarif
```

### Uploading Job Definition

You can store the job configuration in a `YAML` file and then upload using dashboard or API such as:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/yaml" \
  --data-binary @gosec-job.yaml $SERVER/api/jobs/definitions
```

You will need to create an API token to access the API using [Authentication](apidocs.md#Authentication) to the API
sever defined by $SERVER environment variable passing token via $TOKEN environment variable.

### Submitting Job Request Manually

You can then submit the job as follows:

```yaml
 curl -v -H "Authorization: Bearer $TOKEN" \
   -H "Content-Type: application/json" \
   --data '{"job_type": "gosec-job" }' $SERVER/api/jobs/requests
```

The above example kicks off `gosec-job` job that you can see on the dashboard UI.

