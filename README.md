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

The easiest way to get started with Formicary is using `docker-compose`.

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/bhatti/formicary.git
    cd formicary
    ```

2.  **Prepare environment:**
    Create a `.env` file from the example and generate a secure session key.
    ```bash
    cp .env.example .env
    # Replace the placeholder in .env with a real secret
    # On Linux/macOS:
    # SECRET_KEY=$(openssl rand -base64 32); sed -i.bak "s/your_strong_secret_key_here/$SECRET_KEY/" .env
    ```
    
3.  **Run Formicary:**
    This command starts the Queen server, a local Ant worker, Redis, and MinIO.
    ```bash
    docker-compose -f sqlite-docker-compose.yaml up
    ```

4.  **Explore!**
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
    -   [Code of Conduct](./CODE_OF_CONDUCT.md)
    -   [License](./LICENSE.md)

## Comparison with Other Tools
- [See how Formicary compares](./docs/18-comparison.md) to tools like Airflow, Jenkins, GitLab CI, and GitHub Actions.

## License

Formicary is licensed under the [GNU AGPLv3 License](./LICENSE.md).
