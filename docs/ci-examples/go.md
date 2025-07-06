# CI/CD for Go Projects

This guide provides a complete example of a CI/CD pipeline for a typical Go application. The pipeline includes linting, testing, and building stages.

### Full Job Definition

Here is the complete `job_type` definition.

```yaml:go-ci.yaml
job_type: go-build-ci
max_concurrency: 1
skip_if: '{{if ne .GitBranch "main"}}true{{end}}'
tasks:
  - task_type: lint
    method: DOCKER
    working_dir: /src/app
    container:
      image: golang:1.24
    cache:
      key_paths:
        - go.sum
      paths:
        - /go/pkg/mod
    before_script:
      - git clone https://{{.GithubToken}}@github.com/bhatti/go-cicd.git .
      - go mod download
    script:
      - go get -u golang.org/x/lint/golint
      - golint -set_exit_status ./...
    on_completed: test

  - task_type: test
    method: DOCKER
    container:
      image: golang:1.24
    working_dir: /src/app
    cache:
      key_paths:
        - go.sum
      paths:
        - /go/pkg/mod
    dependencies:
      - lint
    environment:
      CGO_ENABLED: "0"
    script:
      - go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
    artifacts:
      paths:
        - coverage.txt
    on_completed: build

  - task_type: build
    method: DOCKER
    container:
      image: golang:1.24
    working_dir: /src/app
    cache:
      key_paths:
        - go.sum
      paths:
        - /go/pkg/mod
    dependencies:
      - test
    script:
      - go build -v -o myapp .
    artifacts:
      paths:
        - myapp
```

### Key Concepts Explained

-   **`skip_if`:** This job will only run if the Git branch is `main`, preventing it from running on feature branches.
-   **`cache`:** We cache the Go module path (`/go/pkg/mod`). The cache key is generated from the hash of the `go.sum` file. This means dependencies are only re-downloaded when `go.sum` changes.
-   **`dependencies`:** The `test` task depends on `lint`. While it doesn't use artifacts, this enforces execution order. The `build` task depends on `test` for the same reason.
-   **`artifacts`:** The `test` task produces a `coverage.txt` report, and the `build` task produces the final `myapp` binary. Both are saved as artifacts.
-   **`working_dir`:** We set a consistent working directory inside the container for all tasks.

### Running the Job

1.  **Store Your Git Token:** Securely store your GitHub token as a Job Config named `GithubToken`. See the main [CI/CD Pipelines Guide](../11-ci-cd-pipelines.md) for instructions.

2.  **Upload the Definition:**
    ```bash
    curl -H "Authorization: Bearer <API_TOKEN>" \
         -H "Content-Type: application/yaml" \
         --data-binary @go-ci.yaml \
         http://localhost:7777/api/jobs/definitions
    ```

3.  **Submit a Job Request:**
    ```bash
    curl -H "Authorization: Bearer <API_TOKEN>" \
         -H "Content-Type: application/json" \
         -d '{
               "job_type": "go-build-ci",
               "params": { "GitBranch": "main" }
             }' \
         http://localhost:7777/api/jobs/requests
    ```
