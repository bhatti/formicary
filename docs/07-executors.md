# Guide: Executors

An **Executor** is the runtime environment where a task's script is executed. Formicary provides a variety of executors, and you select one for each task using the `method` property in your job definition.

## `SHELL`

The `SHELL` executor runs commands directly on the host machine of the Ant worker.

-   **Use Case:** Simple tasks, interacting with the host system, or when containerization is not necessary.
-   **Security:** Use with caution. The job has the same permissions as the user running the Ant worker process. It's recommended to run Ants with a dedicated, unprivileged user.

**Example:**
```yaml
- task_type: cleanup-host
  method: SHELL
  script:
    - echo "Cleaning up temporary files on the worker..."
    - rm -rf /tmp/my-app-*
```

## `DOCKER`

The `DOCKER` executor runs tasks inside a Docker container. This is a common choice for creating isolated and reproducible build environments.

-   **Use Case:** CI/CD builds, running applications with specific dependencies, and ensuring a consistent environment.

### Configuration

You can specify the container image, resource limits, and more within the `container` block of a task.

```yaml
- task_type: build-node-app
  method: DOCKER
  container:
    image: node:16-buster
    memory_request: "512Mi"
    cpu_request: "500m"
  script:
    - npm install
    - npm run build
```

You can also run sidecar containers using the `services` block, which is useful for databases or other dependencies.

## `KUBERNETES`

The `KUBERNETES` executor runs tasks as pods within a Kubernetes cluster. This is the most powerful and scalable executor, ideal for production workloads.

-   **Use Case:** Production-grade CI/CD, scalable data processing, and any workflow that needs robust orchestration, resource management, and isolation.

### Configuration

The Kubernetes executor offers a rich set of configuration options that map directly to Kubernetes concepts.

```yaml
- task_type: build-and-push
  method: KUBERNETES
  privileged: true # Required for building images within a pod (Docker-in-Docker)
  container:
    image: docker:20.10-git
    cpu_limit: "1"
    memory_limit: "2Gi"
    volumes:
      empty_dir: # Share a volume between main container and service
        - name: certs
          mount_path: /certs
  services:
    - name: docker-dind
      image: docker:20.10-dind
      volumes:
        empty_dir:
          - name: certs
            mount_path: /certs
  script:
    - docker build -t my-registry/my-app:latest .
    - docker push my-registry/my-app:latest
```

## `HTTP` Methods

These executors allow you to make REST API calls as part of your workflow.

-   `HTTP_GET`, `HTTP_POST_JSON`, `HTTP_PUT_JSON`, `HTTP_DELETE`, `HTTP_POST_FORM`

-   **Use Case:** Integrating with external services, triggering other systems, or fetching data.

**Example:**
```yaml
- task_type: trigger-deployment
  method: HTTP_POST_JSON
  url: "https://api.my-cloud-provider.com/v1/deployments"
  headers:
    Authorization: "Bearer {{.DEPLOY_TOKEN}}"
    Content-Type: "application/json"
  variables:
    image_tag: "{{.CI_COMMIT_SHA}}"
    environment: "production"
```
The content of the `variables` block will be marshaled into the JSON request body.

## Advanced Executors

### `FORK_JOB` & `AWAIT_FORKED_JOB`

This pair of methods allows you to implement the Fork/Join pattern.
-   `FORK_JOB`: Spawns a new instance of another job definition and continues immediately. It makes the child job's ID available to subsequent tasks.
-   `AWAIT_FORKED_JOB`: Pauses the workflow until one or more forked jobs (specified in `await_forked_tasks`) have completed.

**Use Case:** Running multiple processing pipelines in parallel and then aggregating the results. See the [Advanced Workflows Guide](./14-advanced-workflows.md) for a full example.

### `MESSAGING` & `WEBSOCKET`

These executors are for advanced, custom integrations. They allow Formicary to communicate with custom workers over a message queue or a WebSocket connection, enabling you to build executors in any language.
