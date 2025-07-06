# Contributing to Formicary

Thank you for your interest in contributing to Formicary! We welcome contributions from the community to help make this project better. Whether it's reporting a bug, proposing a new feature, or writing code, your help is appreciated.

## How to Contribute

### Reporting Bugs

If you find a bug, please open an issue on our [GitHub Issues](https://github.com/bhatti/formicary/issues) page. Please include the following:
-   A clear and descriptive title.
-   The version of Formicary you are using (`formicary version`).
-   Steps to reproduce the behavior.
-   The expected behavior and the actual behavior you observed.
-   Relevant logs, error messages, or screenshots.

### Suggesting Enhancements

We're always open to new ideas! If you have a suggestion for a new feature or an improvement to an existing one, please open an issue. This allows for discussion before any code is written.

### Submitting Pull Requests

1.  **Fork the repository** on GitHub.
2.  **Clone your fork** locally: `git clone https://github.com/your-username/formicary.git`
3.  **Create a new branch** for your feature or bugfix: `git checkout -b feature/my-new-feature`
4.  **Make your changes.** Ensure your code adheres to the project's style and conventions.
5.  **Run tests** to ensure you haven't introduced any regressions: `make test`
6.  **Commit your changes** with a clear and descriptive commit message.
7.  **Push your branch** to your fork: `git push origin feature/my-new-feature`
8.  **Open a Pull Request** against the `main` branch of the `bhatti/formicary` repository.

## Development Setup

To get started with local development, follow these steps.

### Prerequisites

-   Go 1.24+
-   Docker and Docker Compose
-   `make`

### Code Structure Overview

-   `queen/`: Source code for the Queen server (leader).
    -   `controller/`: API and UI endpoint handlers.
    -   `scheduler/`: The core job scheduler logic.
    -   `supervisor/`: Job and Task lifecycle management.
    -   `repository/`: Database interaction layer (GORM).
-   `ants/`: Source code for the Ant worker (follower).
    -   `executor/`: Implementations for Docker, Kubernetes, Shell, etc.
    -   `handler/`: Logic for handling incoming task requests.
-   `internal/`: Shared code used by both Queen and Ant services.
    -   `types/`: Core data structures and domain models.
    -   `queue/`: Message queue client abstractions.
-   `cmd/`: The main entry point and CLI command definitions.
-   `public/`: Static assets for the web dashboard.
-   `docs/`: All project documentation.

### Building and Running Locally

1.  **Start Dependencies:**
    You can run the required services (Redis, MinIO, a database) using Docker Compose:
    ```bash
    docker-compose up -d redis minio mysql
    ```

2.  **Build the Binary:**
    The `Makefile` contains helpers for common tasks. To build the `formicary` executable:
    ```bash
    make build
    ```
    The binary will be located at `out/bin/formicary`.

3.  **Run Tests:**
    To run the full suite of unit tests:
    ```bash
    make test
    ```
    To generate a coverage report:
    ```bash
    make coverage
    go tool cover -html=coverage.out
    ```

4.  **Run the Queen and Ant:**
    Open two separate terminals.

    In terminal 1, run the Queen server:
    ```bash
    ./out/bin/formicary queen --config ./.formicary-queen.yaml
    ```

    In terminal 2, run a standalone Ant worker:
    ```bash
    ./out/bin/formicary ant --config ./.formicary-ant.yaml
    ```

### Debugging

-   **Stack Dumps:** To get a stack trace of all running goroutines for a `formicary` process, send it the `SIGHUP` signal.
    ```bash
    # Find the process ID
    pgrep formicary
    # Send the signal
    kill -HUP <pid>
    ```
-   **Graceful Shutdown:** To gracefully shut down a process (allowing in-progress jobs to finish), send it the `SIGQUIT` signal.
    ```bash
    kill -QUIT <pid>
    ```

## Code Style

-   Please run `go fmt ./...` and `go vet ./...` before submitting your code.
-   We use `golangci-lint` for linting. You can run it with `make lint`.
