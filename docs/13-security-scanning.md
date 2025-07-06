# Tutorials: Security Scanning Pipelines

Integrating security scanning into your CI/CD pipelines is a critical practice. Formicary's task-based model makes it easy to add security stages to your workflows, allowing you to scan source code (SAST), containers, and other artifacts.

The general pattern is:
1.  Define a task that uses a container image with the desired security tool.
2.  Run the tool in the `script` section.
3.  Configure the tool to output a report (e.g., SARIF, JSON).
4.  Save the report as an `artifact`.
5.  Use the tool's exit code to control the pipeline's success or failure.

---

## Example 1: SAST for Go with Gosec

This example shows how to perform Static Application Security Testing (SAST) on a Go project using `gosec`.

```yaml:gosec-job.yaml
job_type: go-sast-scan
tasks:
- task_type: scan-with-gosec
  method: DOCKER
  working_dir: /app
  container:
    image: securego/gosec:latest
  before_script:
    - git clone https://{{.GithubToken}}@github.com/my-org/my-go-project.git .
  script:
    # -no-fail: Don't exit with non-zero status if issues are found.
    # We will upload the report and can decide to fail later.
    - gosec -no-fail -fmt sarif -out results.sarif ./...
  artifacts:
    paths:
      - results.sarif
```

**Explanation:**
-   We use the official `securego/gosec` Docker image.
-   The `script` runs `gosec`, telling it to format the output as SARIF (a standard for static analysis results) and save it to `results.sarif`.
-   The `results.sarif` file is then uploaded as an artifact for review or further processing.

---

## Example 2: Container Vulnerability Scanning with Trivy

This example demonstrates how to build a Docker image and then scan it for vulnerabilities using `trivy`. This requires a Docker-in-Docker setup.

```yaml:trivy-scan-job.yaml
job_type: trivy-container-scan
max_concurrency: 1
tasks:
- task_type: build-and-scan
  method: KUBERNETES # Also works with DOCKER
  working_dir: /app
  privileged: true # Docker-in-Docker requires privileged mode
  variables:
    IMAGE_NAME: my-app:{{.GitCommitID}}
  container:
    image: docker:20.10-git
  services:
    - name: dind-service
      alias: docker
      image: docker:20.10-dind
  before_script:
    - git clone https://github.com/my-org/my-app-code.git .
    # Install Trivy
    - apk add --no-cache curl
    - curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin
  script:
    # 1. Build the container image
    - docker build -t $IMAGE_NAME .

    # 2. Scan for HIGH and CRITICAL vulnerabilities. Exit with 0 so the report is always generated.
    - trivy image --exit-code 0 --severity HIGH,CRITICAL --format json -o report.json $IMAGE_NAME
    
    # 3. Fail the job if any CRITICAL vulnerabilities are found.
    - trivy image --exit-code 1 --severity CRITICAL $IMAGE_NAME
  artifacts:
    paths:
      - report.json
```

**Explanation:**
-   **`services`:** We run a `docker:dind` (Docker-in-Docker) container as a service, which provides a Docker daemon for the main task container to use.
-   **`privileged: true`:** This is required for the main container to interact with the Docker daemon.
-   **Two-Step Scan:** We run `trivy` twice.
    1.  The first run has `--exit-code 0` to ensure the `report.json` artifact is always created, even if vulnerabilities are found.
    2.  The second run has `--exit-code 1` and checks only for `CRITICAL` severity. If any are found, the command will fail, which in turn fails the Formicary task and stops the pipeline.

