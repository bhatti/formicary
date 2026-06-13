# Installation

This guide covers how to get Formicary up and running. The recommended method for local evaluation and testing is using Docker Compose.

## Prerequisites

-   **Docker & Docker Compose:** You need a recent version of Docker and Docker Compose installed on your system.
    -   [Install Docker Engine](https://docs.docker.com/engine/install/)
    -   [Install Docker Compose](https://docs.docker.com/compose/install/)
-   **Git:** To clone the repository.
-   **(Optional) Kubernetes:** If you want to use the Kubernetes executor, you need access to a Kubernetes cluster (e.g., [MicroK8s](https://microk8s.io/), [k3s](https://k3s.io/), minikube, or a cloud provider's managed service).
-   **(Optional) Go:** To build from source, you need Go version 1.22 or newer.

## Recommended: Running with Docker Compose

This is the fastest way to start a complete Formicary environment, including the Queen server, an embedded Ant worker, Redis for messaging, and MinIO for artifact storage. This setup uses a local SQLite database for simplicity.

1.  **Clone the Repository**
    ```bash
    git clone https://github.com/bhatti/formicary.git
    cd formicary
    ```

2.  **Create a Secrets File**
    The `.env` file in the repository contains the template. Copy it to `.env.local` for your secrets — this file is gitignored and never committed.

    ```bash
    cp .env .env.local
    ```

3.  **Generate a JWT Secret**
    You **must** set `COMMON_AUTH_JWT_SECRET` in `.env.local`. This key signs all user sessions.

    ```bash
    echo "COMMON_AUTH_JWT_SECRET=$(openssl rand -base64 32)" >> .env.local
    ```

4.  **Configure OAuth**
    To enable login via Google or GitHub, create an OAuth application with the provider and add the credentials to `.env.local`:

    ```bash
    # Google — https://console.cloud.google.com → APIs & Services → Credentials
    # Authorized redirect URI: http://localhost:7777/auth/google/callback
    COMMON_AUTH_GOOGLE_CLIENT_ID=<your-client-id>
    COMMON_AUTH_GOOGLE_CLIENT_SECRET=<your-client-secret>

    # GitHub — https://github.com/settings/developers → OAuth Apps
    # Authorization callback URL: http://localhost:7777/auth/github/callback
    COMMON_AUTH_GITHUB_CLIENT_ID=<your-client-id>
    COMMON_AUTH_GITHUB_CLIENT_SECRET=<your-client-secret>
    ```

    See [Configuration — common.auth](./15-configuration.md#commonauth-block) for the full list of auth environment variables.

5.  **Start the System**
    From the root of the repository, run the `sqlite-docker-compose.yaml` file:
    ```bash
    docker-compose -f sqlite-docker-compose.yaml up --build
    ```
    This will build the Formicary image and start all the necessary services. You'll see logs from all services in your terminal.

6.  **Verify the Installation**
    -   **Formicary Dashboard:** Open your web browser to [http://localhost:7777](http://localhost:7777).
    -   **MinIO Console:** Check the object storage console at [http://localhost:9001](http://localhost:9001) (Use the credentials from your `.env` file, default is `admin`/`password`).

## Zero-Dependency Local Setup (WebSocket + Embedded SeaweedFS)

For local development or edge deployments where you don't want to install Redis, Kafka, MinIO, or any external broker, Formicary can run in a fully self-contained mode:

- **`WEBSOCKET_MESSAGING`** — the queen serves a WebSocket endpoint that ants connect to directly; no external message broker needed.
- **`s3.local_mode: true`** — the queen starts an embedded [SeaweedFS](https://github.com/seaweedfs/seaweedfs) subprocess as the artifact store; no external S3/MinIO needed.

**Prerequisites:**

1. Install the `weed` binary from [SeaweedFS releases](https://github.com/seaweedfs/seaweedfs/releases) and ensure it is on your `$PATH`.

**Start the queen:**

```bash
./formicary-queen --config docs/examples/websocket-queen.yaml
```

The queen prints the SeaweedFS S3 endpoint at startup (default port 8333). The WebSocket queue endpoint is always at `ws://localhost:7777/ws/queue`.

**Start an ant** (in a separate terminal):

```bash
./formicary-ant --config docs/examples/websocket-ant.yaml
```

The ant connects to the queen via WebSocket and uses the embedded SeaweedFS for artifacts. If the ant is restarted while the queen is unreachable, undelivered messages are buffered in a local SQLite file (`/tmp/formicary-ant-buffer.db`) and drained automatically after reconnection.

See `docs/examples/websocket-queen.yaml` and `docs/examples/websocket-ant.yaml` for annotated configuration files, and [Configuration — queue.websocket](./15-configuration.md#commonqueuewebsocket-block) for the full field reference.

---

## Running from Source (for Development)

If you plan to contribute to Formicary, you'll want to run it directly from source.

1.  **Start Dependencies:**
    You still need the database, message queue, and object store. You can start just these services from the `docker-compose.yaml` file:
    ```bash
    docker-compose up -d redis minio mysql
    ```
    Or use the zero-dependency WebSocket + embedded SeaweedFS mode described above — no Docker needed.

2.  **Configure Queen Server:**
    Ensure your configuration file points to the correct addresses for your chosen queue provider, object store, and database.

3.  **Run the Queen Server:**
    ```bash
    go run ./main.go --config ./.formicary-queen.yaml --id=queen-server-1
    ```

4.  **Run an Ant Worker:**
    In a separate terminal, run the Ant worker:
    ```bash
    go run ./main.go ant --config=./.formicary-ant.yaml --id=local-ant-1 --tags="shell,docker"
    ```
