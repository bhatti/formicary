# Formicary

![Formicary Logo](docs/formicary.png)

**Formicary is a distributed, cloud-native orchestration engine for executing complex workflows, data pipelines, and CI/CD jobs.**

[![Go Report Card](https://goreportcard.com/badge/github.com/bhatti/formicary)](https://goreportcard.com/report/github.com/bhatti/formicary)
[![Maintainability](https://api.codeclimate.com/v1/badges/99/maintainability)](https://codeclimate.com/github/bhatti/formicary/maintainability)
[![Test Coverage](https://api.codeclimate.com/v1/badges/99/test_coverage)](https://codeclimate.com/github/bhatti/formicary/test_coverage)
[![Docker Image Version (latest by date)](https://img.shields.io/docker/v/plexobject/formicary?label=Docker%20Image)](https://hub.docker.com/r/plexobject/formicary)
[![License](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

Formicary uses a declarative YAML-based approach to define jobs as a Directed Acyclic Graph (DAG) of tasks. It's built on a robust leader-follower architecture (`Queen` and `Ants`) that efficiently distributes work to executors running on Docker, Kubernetes, or as simple shell commands.

It is designed for use cases that require complex dependency management, parallel execution, and sophisticated error handling.

## Key Features

-   **Declarative Workflows:** Define complex jobs with multiple tasks and dependencies in simple YAML files.
-   **Extensible Executors:** Natively supports **Kubernetes**, **Docker**, **Shell**, **HTTP/REST**, and **Websockets**.
-   **Advanced Flow Control:** Sophisticated retry logic, conditional execution (`on_exit_code`), error handling, and optional tasks.
-   **Parallel & Concurrent Execution:** Fork jobs to run in parallel and join the results, with concurrency limits.
-   **Powerful Templating:** Use Go templates in your job definitions for dynamic workflows.
-   **Built-in Caching:** Speed up jobs with caching for dependencies (e.g., `node_modules`, `vendor`, `.m2`).
-   **Robust Artifacts Management:** Persist task outputs to an S3-compatible object store for use in later stages or for download.
-   **Security First:** RBAC, encrypted secrets, and JWT-based API authentication.
-   **Real-time & Observability:** Stream logs in real-time, get Prometheus metrics, and view detailed job statistics.

## Getting Started

The quickest way to run Formicary is with a single Docker command — no clone required.

Auth is **enabled by default**. Supply your JWT secret and OAuth credentials, or pass `COMMON_AUTH_ENABLED=false` to disable auth for local testing.

**Option A — Google OAuth (default, auth enabled):**
```bash
mkdir -p ~/formicary-data
# Patch kubeconfig: replace 127.0.0.1 with host.docker.internal and skip TLS verify
# (Docker Desktop's k8s cert is valid for localhost, not host.docker.internal)
python3 -c "
import sys, yaml
with open(sys.argv[1]) as f: kc = yaml.safe_load(f)
for c in kc.get('clusters', []):
    cl = c.get('cluster', {})
    cl['server'] = cl.get('server','').replace('https://127.0.0.1','https://host.docker.internal')
    cl.pop('certificate-authority-data', None)
    cl['insecure-skip-tls-verify'] = True
with open(sys.argv[2], 'w') as f: yaml.dump(kc, f)
" ~/.kube/config ~/formicary-data/kubeconfig
docker run --rm -p 7777:7777 -p 19000:19000 \
  -e COMMON_AUTH_JWT_SECRET=<your-jwt-secret> \
  -e COMMON_AUTH_GOOGLE_CLIENT_ID=<your-google-client-id> \
  -e COMMON_AUTH_GOOGLE_CLIENT_SECRET=<your-google-client-secret> \
  -e COMMON_AUTH_GOOGLE_CALLBACK_HOST=localhost \
  -v ~/formicary-data:/data \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ~/formicary-data/kubeconfig:/home/formicary-user/.kube/config:ro \
  plexobject/formicary:latest
```

**Option B — No auth (local testing only):**
```bash
mkdir -p ~/formicary-data
python3 -c "
import sys, yaml
with open(sys.argv[1]) as f: kc = yaml.safe_load(f)
for c in kc.get('clusters', []):
    cl = c.get('cluster', {})
    cl['server'] = cl.get('server','').replace('https://127.0.0.1','https://host.docker.internal')
    cl.pop('certificate-authority-data', None)
    cl['insecure-skip-tls-verify'] = True
with open(sys.argv[2], 'w') as f: yaml.dump(kc, f)
" ~/.kube/config ~/formicary-data/kubeconfig
docker run --rm -p 7777:7777 -p 19000:19000 \
  -e COMMON_AUTH_ENABLED=false \
  -e COMMON_AUTH_JWT_SECRET=<your-jwt-secret> \
  -v ~/formicary-data:/data \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ~/formicary-data/kubeconfig:/home/formicary-user/.kube/config:ro \
  plexobject/formicary:latest
```

Data (SQLite DB + artifacts) is stored in `~/formicary-data/` — inspect with `ls ~/formicary-data/` after first run.

The kubeconfig patch replaces `127.0.0.1` with `host.docker.internal` — Docker Desktop's special hostname that lets containers reach the host's Kubernetes API. `make docker-run` does this automatically.

Both options start the Queen server, an embedded Ant worker, embedded SeaweedFS (artifact storage), and SQLite — no Redis, MinIO, or separate services needed.

To use a locally-built image instead of the published one:
```bash
DOCKER_IMAGE=formicary:latest make docker-run
```

**Or use docker-compose** (persistent data, restart policy):
```bash
curl -fsSL https://raw.githubusercontent.com/bhatti/formicary/main/docker-compose.yaml -o docker-compose.yaml
COMMON_AUTH_JWT_SECRET=<your-jwt-secret> \
COMMON_AUTH_GOOGLE_CLIENT_ID=<your-google-client-id> \
COMMON_AUTH_GOOGLE_CLIENT_SECRET=<your-google-client-secret> \
docker compose up
```

**Explore!**
-   Open the Formicary Dashboard at [http://localhost:7777](http://localhost:7777).
-   Follow the [Quick Start Guide](./docs/03-quick-start.md) to run your first job.

## Documentation

-   **Introduction**
    -   [Introduction to Formicary](./docs/01-introduction.md)
    -   [Core Concepts](./docs/05-concepts.md)
    -   [Architecture Deep Dive](./docs/04-architecture.md)
-   **Getting Started**
    -   [Installation Guide](./docs/02-installation.md)
    -   [Quick Start Tutorial](./docs/03-quick-start.md)
-   **Guides & Tutorials**
    -   [Job & Task Definitions](./docs/06-job-definitions.md)
    -   [Executors](./docs/07-executors.md)
    -   [Scheduling & Triggers](./docs/08-scheduling-and-triggers.md)
    -   [Artifacts & Caching](./docs/09-artifacts-and-caching.md)
    -   [CI/CD Pipelines](./docs/11-ci-cd-pipelines.md)
    -   [Data Pipelines](./docs/12-data-pipelines.md)
    -   [Advanced Workflows](./docs/14-advanced-workflows.md)
    -   [Security Guide](./docs/19-security.md)
-   **Reference**
    -   [Configuration Reference](./docs/15-configuration.md)
    -   [CLI Reference](./docs/10-cli-reference.md)
    -   [API Reference](./docs/16-api-reference.md)
    -   [Troubleshooting & FAQ](./docs/20-troubleshooting-faq.md)
-   **Community**
    -   [Contributing Guide](./docs/17-contributing.md)
    -   [Code of Conduct](./docs/CODE_OF_CONDUCT.md)
    -   [License](./LICENSE.md)

## Comparison with Other Tools
- [See how Formicary compares](./docs/18-comparison.md) to tools like Airflow, Jenkins, GitLab CI, and GitHub Actions.

## Blogs/Articles

-   [Building a distributed orchestration and graph processing system](https://shahbhat.medium.com/building-a-distributed-orchestration-and-graph-processing-system-04f757ae97f4)
-   [Building Resilient, Interactive Playbooks with an Orchestration Engine](https://shahbhat.medium.com/building-resilient-interactive-playbooks-with-formicary-8cc289c9c917)
-   [Task Scheduling Algorithms in Distributed Orchestration Systems](https://weblog.plexobject.com/archives/6960)


## License

Formicary is licensed under the [GNU AGPLv3 License](./LICENSE.md).
