## Scanning containers using Trivy

Trivy is a simple and comprehensive vulnerability/misconfiguration scanner for containers and other artifacts. Following
example shows scanning a docker in docker (dind) using Trivy:

```yaml
job_type: trivy-scan-job
description: vulnerability/misconfiguration scanner for containers using Trivy
url: https://aquasecurity.github.io/trivy/v0.19.0/
max_concurrency: 1
job_variables:
  CI_COMMIT_SHA: db65c90a07e753e71db5143c877940f4c11a33e1
tasks:
  - task_type: scan
    working_dir: /trivy-ci-test
    variables:
      DOCKER_HOST: tcp://localhost:2375
      DOCKER_TLS_CERTDIR: ""
      IMAGE: trivy-ci-test:{{.CI_COMMIT_SHA}}
    container:
      image: docker:20.10-git
    privileged: true
    services:
      - name: docker-dind
        alias: docker
        image: docker:20.10-dind
        entrypoint: [ "env", "-u", "DOCKER_HOST" ]
        command: [ "dockerd-entrypoint.sh" ]
    allow_failure: true
    before_script:
      - echo image $IMAGE
      - git clone https://github.com/aquasecurity/trivy-ci-test.git .
      - wget -qO - "https://api.github.com/repos/aquasecurity/trivy/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/'
      - export TRIVY_VERSION=$(wget -qO - "https://api.github.com/repos/aquasecurity/trivy/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
      - echo $TRIVY_VERSION
      - apk add --update-cache --upgrade curl
      - curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin
      - mkdir -p /root/.docker/
      - curl -o /root/.docker/ca.pem https://gist.githubusercontent.com/bhatti/8a37691361c09afbef751cb168715867/raw/118f47230adec566cef72661e66370cf95ba1be8/ca.pem
    script:
      # Build image
      - docker build -t $IMAGE .
      - curl -o tmpl.tpl https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/gitlab-codequality.tpl
      # Build report
      - trivy --exit-code 0 --cache-dir .trivycache/ --no-progress --format template --template "tmpl.tpl" -o gl-container-scanning-report.json $IMAGE
      # Print report
      - trivy --exit-code 0 --cache-dir .trivycache/ --no-progress --severity HIGH $IMAGE
      # Fail on severe vulnerabilities
      - trivy --exit-code 1 --cache-dir .trivycache/ --severity CRITICAL --no-progress $IMAGE
    cache:
      paths:
        - .trivycache/
    artifacts:
      paths:
        - gl-container-scanning-report.json
```

#### Job Type

The `job_type` defines type of the job, e.g.

```yaml
job_type: trivy-scan-job
```

#### Description

The `description` defines explanation of the job, e.g.,

```yaml
description: vulnerability/misconfiguration scanner for containers using Trivy
```

#### URL

The `url` defines external URL about the job, e.g.,

```yaml
url: https://aquasecurity.github.io/trivy/v0.19.0/
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
CI_COMMIT_SHA: db65c90a07e753e71db5143c877940f4c11a33e1
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
  working_dir: /trivy-ci-test
```

##### Variables

The `variables` defines variables used by the task, e.g.,

```yaml
  variables:
  DOCKER_HOST: tcp://localhost:2375
  DOCKER_TLS_CERTDIR: ""
  IMAGE: trivy-ci-test:{{.CI_COMMIT_SHA}}
```

##### Docker Image

The `image` tag within `container` defines docker-image to use for execution commands, e.g.,

```yaml
  container:
    image: docker:20.10-git
```

##### Allow Failure (optional)

The `allow_failure` tag defines the task as optional so that it can fail without failing entire job, e.g.,

```yaml
  allow_failure: true
```

##### Privileged

The `privileged` tag enables executing docker in privileged root because docker in docker requires it, e.g.,

```yaml
  privileged: true
```

##### Services

The `services` tag defines Kubernetes services, which will launch docker-in-docker (dind) service, e.g.,

```yaml
  services:
    - name: docker-dind
      alias: docker
      image: docker:20.10-dind
      entrypoint: [ "env", "-u", "DOCKER_HOST" ]
      command: [ "dockerd-entrypoint.sh" ]
```

##### Before Script Commands

The `before_script` defines an array of shell commands that are executed before the main script, e.g.,

```yaml
    before_script:
      - echo image $IMAGE
      - git clone https://github.com/aquasecurity/trivy-ci-test.git .
      - wget -qO - "https://api.github.com/repos/aquasecurity/trivy/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/'
      - export TRIVY_VERSION=$(wget -qO - "https://api.github.com/repos/aquasecurity/trivy/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
      - echo $TRIVY_VERSION
      - apk add --update-cache --upgrade curl
      - curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin
      - mkdir -p /root/.docker/
      - curl -o /root/.docker/ca.pem https://gist.githubusercontent.com/bhatti/8a37691361c09afbef751cb168715867/raw/118f47230adec566cef72661e66370cf95ba1be8/ca.pem
```

Above script clones a test container and then downloads trivy package.

##### Script Commands

The `script` defines an array of shell commands that are executed inside container, e.g.,

```yaml
  script:
    # Build image
    - docker build -t $IMAGE .
    - curl -o tmpl.tpl https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/gitlab-codequality.tpl
    # Build report
    - trivy --exit-code 0 --cache-dir .trivycache/ --no-progress --format template --template "tmpl.tpl" -o gl-container-scanning-report.json $IMAGE
    # Print report
    - trivy --exit-code 0 --cache-dir .trivycache/ --no-progress --severity HIGH $IMAGE
    # Fail on severe vulnerabilities
    - trivy --exit-code 1 --cache-dir .trivycache/ --severity CRITICAL --no-progress $IMAGE
```

Above script creates docker image and then uses trivy to scan for vulnerabilities.

##### Caching

Formicary also provides caching for directories that store 3rd party dependencies, e.g. following example shows how all
python libraries can be cached:

```yaml
    cache:
      paths:
        - .trivycache/
```

##### Artifacts

Formicary allows uploading artifacts from the task output, e.g.

```yaml
    artifacts:
      paths:
        - gl-container-scanning-report.json
```

### Uploading Job Definition

You can store the job configuration in a `YAML` file and then upload using dashboard or API such as:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/yaml" \
  --data-binary @trivy-scan-job.yaml $SERVER/api/jobs/definitions
```

You will need to create an API token to access the API using [Authentication](apidocs.md#Authentication) to the API
sever defined by $SERVER environment variable passing token via $TOKEN environment variable.

### Submitting Job Request Manually

You can then submit the job as follows:

```yaml
 curl -v -H "Authorization: Bearer $TOKEN" \
   -H "Content-Type: application/json" \
   --data '{"job_type": "trivy-scan-job" }' $SERVER/api/jobs/requests
```

The above example kicks off `trivy-scan-job` job that you can see on the dashboard UI.

