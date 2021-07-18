## Building Docker images using Docker in Docker (dind)

### Using TLS
Following example shows using docker in docker (dind) using TLS to build images:

```yaml
job_type: dind-tls-job
max_concurrency: 1
tasks:
- task_type: build
  working_dir: /sample
  variables:
    DOCKER_HOST: tcp://localhost:2376
    DOCKER_TLS_VERIFY: 1
    DOCKER_TLS: 1
    DOCKER_TLS_CERTDIR: "/mycerts"
  container:
    image: docker:20.10-git
    volumes:
      empty_dir:
        - name: certs
          mount_path: /mycerts/client
  privileged: true
  services:
    - name: docker-dind
      alias: docker
      image: docker:20.10-dind
      entrypoint: ["env", "-u", "DOCKER_HOST"]
      command: ["dockerd-entrypoint.sh"]
      volumes:
        empty_dir:
          - name: certs
            mount_path: /mycerts/client
  before_script:
    - git clone https://github.com/aquasecurity/trivy-ci-test.git .
    - mkdir -p /root/.docker/ && cp /mycerts/client/* /root/.docker
    - apk --no-cache add ca-certificates
    - docker info
  script:
    # Build image
    - docker build -t my-image .
```

### Without TLS
Following example shows using docker in docker (dind) without TLS to build images:

```yaml
job_type: dind-job
max_concurrency: 1
tasks:
- task_type: build
  working_dir: /sample
  variables:
    DOCKER_HOST: tcp://localhost:2375
    DOCKER_TLS_CERTDIR: ""
  container:
    image: docker:20.10-git
  privileged: true
  services:
    - name: docker-dind
      alias: docker
      image: docker:20.10-dind
      entrypoint: ["env", "-u", "DOCKER_HOST"]
      command: ["dockerd-entrypoint.sh"]
  before_script:
    - git clone https://github.com/aquasecurity/trivy-ci-test.git .
    - mkdir -p /root/.docker/
    - docker info
  script:
    # Build image
    - docker build -t my-image .
```

#### Job Type

The `job_type` defines type of the job, e.g.

```yaml
job_type: dind-tls-job
```

#### Concurrency

The `max_concurrency` defines maximum jobs that can be executed concurrently, e.g.,

```yaml
max_concurrency: 1
```

#### Tasks

The tasks section define the DAG or workflow of the build job where each specifies details for each build step such as:

##### Task Type

The `task_type` defines name of the task, e.g.

```yaml
- task_type: build
```

##### Working Directory

The `working_dir` defines default directory for the scripts, e.g.,

```yaml
  working_dir: /sample
```

##### Variables

The `variables` defines variables used by the task, e.g.,

```yaml
  variables:
    DOCKER_HOST: tcp://localhost:2376
    DOCKER_TLS_VERIFY: 1
    DOCKER_TLS: 1
    DOCKER_TLS_CERTDIR: "/mycerts"
```

##### Docker Image

The `image` tag within `container` defines docker-image to use for execution commands, e.g.,

```yaml
  container:
    image: docker:20.10-git
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
      entrypoint: ["env", "-u", "DOCKER_HOST"]
      command: ["dockerd-entrypoint.sh"]
      volumes:
        empty_dir:
          - name: certs
            mount_path: /mycerts/client
```

##### Before Script Commands

The `before_script` defines an array of shell commands that are executed before the main script, e.g.,

```yaml
    before_script:
      - git clone https://github.com/aquasecurity/trivy-ci-test.git .
      - mkdir -p /root/.docker/ && cp /mycerts/client/* /root/.docker
      - apk --no-cache add ca-certificates
      - docker info
```

Above script clones a test container and then downloads trivy package.

##### Script Commands

The `script` defines an array of shell commands that are executed inside container, e.g.,

```yaml
  script:
    # Build image
    - docker build -t my-image .
```

Above script creates docker image and then uses trivy to scan for vulnerabilities.

### Uploading Job Definition

You can store the job configuration in a `YAML` file and then upload using dashboard or API such as:

```yaml
curl -v -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/yaml" \
  --data-binary @dind-tls-job.yaml $SERVER/api/jobs/definitions
```

You will need to create an API token to access the API using [Authentication](apidocs.md#Authentication) to the API
sever defined by $SERVER environment variable passing token via $TOKEN environment variable.

### Submitting Job Request Manually

You can then submit the job as follows:

```yaml
 curl -v -H "Authorization: Bearer $TOKEN" \
   -H "Content-Type: application/json" \
   --data '{"job_type": "dind-tls-job" }' $SERVER/api/jobs/requests
```

The above example kicks off `dind-tls-job` job that you can see on the dashboard UI.

