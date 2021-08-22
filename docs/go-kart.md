## Static checking GO projects using go-kart

go-kart is a Golang security static analyzer (https://github.com/praetorian-inc/gokart) that inspects source code for security vulnerabilities using using the SSA (single static assignment) . Following example shows scanning a GO project using go-kart:

```yaml
job_type: go-kart
url: https://github.com/praetorian-inc/gokart
tasks:
- task_type: scan
  method: KUBERNETES
  working_dir: /sample
  container:
    image: golang:1.16-buster
  before_script:
    - go install github.com/praetorian-inc/gokart@latest
    - git clone https://github.com/Contrast-Security-OSS/go-test-bench.git
  script:
    - gokart scan go-test-bench/ -v -g -s > results.sarif
  after_script:
    - ls -l
  artifacts:
    paths:
      - results.sarif
```

#### Job Type

The `job_type` defines type of the job, e.g.

```yaml
job_type: go-kart
```

#### URL

The `url` defines external URL about the job, e.g.,

```yaml
url: https://github.com/praetorian-inc/gokart
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
    image: golang:1.16-buster
```


##### Before Script Commands

The `before_script` defines an array of shell commands that are executed before the main script, e.g.,

```yaml
    before_script:
    - go install github.com/praetorian-inc/gokart@latest
    - git clone https://github.com/Contrast-Security-OSS/go-test-bench.git
```

##### Script Commands

The `script` defines an array of shell commands that are executed inside container, e.g.,

```yaml
  script:
    - gokart scan go-test-bench/ -v -g -s > results.sarif
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
  --data-binary @go-kart.yaml $SERVER/api/jobs/definitions
```

You will need to create an API token to access the API using [Authentication](apidocs.md#Authentication) to the API
sever defined by $SERVER environment variable passing token via $TOKEN environment variable.

### Submitting Job Request Manually

You can then submit the job as follows:

```yaml
 curl -v -H "Authorization: Bearer $TOKEN" \
   -H "Content-Type: application/json" \
   --data '{"job_type": "go-kart" }' $SERVER/api/jobs/requests
```

The above example kicks off `go-kart` job that you can see on the dashboard UI.

