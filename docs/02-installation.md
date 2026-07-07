# Installation

This guide covers how to get Formicary up and running. The recommended method for local evaluation and testing is using Docker Compose.

## Prerequisites

-   **Docker & Docker Compose:** You need a recent version of Docker and Docker Compose installed on your system.
    -   [Install Docker Engine](https://docs.docker.com/engine/install/)
    -   [Install Docker Compose](https://docs.docker.com/compose/install/)
-   **Git:** To clone the repository.
-   **(Optional) Kubernetes:** If you want to use the Kubernetes executor, you need access to a Kubernetes cluster (e.g., [MicroK8s](https://microk8s.io/), [k3s](https://k3s.io/), minikube, or a cloud provider's managed service).
-   **(Optional) Go:** To build from source, you need Go version 1.22 or newer.

## Quickstart: Run from Docker Hub (no git clone required)

The official image is on Docker Hub as `plexobject/formicary:latest`. You only need two files — grab them from the repo or copy them manually:

| File | Purpose |
|------|---------|
| `config/formicary-docker.yaml` | Self-contained config; all paths point inside the container |
| `docker-compose.yaml` | Compose file (already includes everything) |

### Steps

1. **Download the two required files** (no git clone needed)

   ```bash
   mkdir -p formicary && cd formicary
   curl -fsSL https://raw.githubusercontent.com/bhatti/formicary/main/docker-compose.yaml -o docker-compose.yaml
   mkdir -p config
   curl -fsSL https://raw.githubusercontent.com/bhatti/formicary/main/config/formicary-docker.yaml -o config/formicary-docker.yaml
   ```

2. **Set credentials**

   ```bash
   export COMMON_AUTH_JWT_SECRET=$(openssl rand -base64 32)
   export COMMON_AUTH_GOOGLE_CLIENT_ID=<your-client-id>
   export COMMON_AUTH_GOOGLE_CLIENT_SECRET=<your-client-secret>
   # Google OAuth redirect URI must be: http://localhost:7777/auth/google/callback
   ```

3. **Start**

   ```bash
   docker compose up
   ```

   All state (SQLite DB + SeaweedFS blobs) lands in the `formicary-data` Docker volume. To use a host directory instead, set the env var before starting:

   ```bash
   DATA_DIR=~/formicary-data docker compose up
   ```

4. **Open the dashboard:** [http://localhost:7777](http://localhost:7777)

### Makefile shortcut

```bash
make docker-run
```

Reads `COMMON_AUTH_JWT_SECRET`, `COMMON_AUTH_GOOGLE_CLIENT_ID`, and `COMMON_AUTH_GOOGLE_CLIENT_SECRET` from your environment. Data stored in `~/formicary-data`; override with `DATA_DIR=/some/path`.

### Disabling auth for local testing

If you just want to explore the UI without setting up OAuth credentials, disable auth entirely:

```bash
# docker compose
COMMON_AUTH_ENABLED=false docker compose up

# make
make docker-run COMMON_AUTH_ENABLED=false

# plain docker (download config first, or copy from the repo)
curl -fsSL https://raw.githubusercontent.com/bhatti/formicary/main/config/formicary-docker.yaml \
  -o /tmp/formicary-docker.yaml
docker run --rm -p 7777:7777 \
  -e COMMON_AUTH_ENABLED=false \
  -v ~/formicary-data:/data \
  -v /tmp/formicary-docker.yaml:/config/formicary-queen.yaml:ro \
  plexobject/formicary:latest
```

With auth disabled, no login is required and no OAuth credentials are needed.

---

## Recommended: Running with Docker Compose

This is the fastest way to start a complete Formicary environment. A single container runs everything:

- **Queen server** — web UI, API, job scheduler
- **Embedded Ant worker** — executes jobs (SHELL, DOCKER, KUBERNETES, HTTP)
- **Embedded SeaweedFS** — artifact storage (no external S3/MinIO needed)
- **SQLite** — database (no external Postgres/MySQL needed)

No Redis, no MinIO, no separate ant container — everything is self-contained in `plexobject/formicary:latest`.

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

Download the `weed` binary (SeaweedFS) — `make run` and `make run-queen` handle this automatically via `make download-weed`.

### Queen + embedded ant (all-in-one, simplest)

```bash
make run
```

Uses `config/formicary-queen-embedded.yaml`. Starts the queen, an embedded ant worker, and embedded SeaweedFS in one process. Open [http://localhost:7777](http://localhost:7777).

### Queen-only (ants connect separately)

```bash
make run-queen
```

Uses `config/formicary-queen.yaml`. The WebSocket queue endpoint is at `ws://localhost:7777/ws/queue`.

Start one or more ants in separate terminals:

```bash
make ant
```

Uses `config/formicary-ant.yaml`, which points at `ws://localhost:7777/ws/queue` by default.

### Connecting an ant to a remote queen

The queen URL is controlled by the env var `COMMON_QUEUE_WEBSOCKET_SERVER_ENDPOINT` (maps to `common.queue.websocket.server_endpoint` in YAML, with `.` replaced by `_`):

```bash
COMMON_QUEUE_WEBSOCKET_SERVER_ENDPOINT="ws://queen.example.com:7777/ws/queue" \
COMMON_S3_ENDPOINT="queen.example.com:19000" \
COMMON_S3_ACCESS_KEY_ID="localkey" \
COMMON_S3_SECRET_ACCESS_KEY="localsecret" \
  ./out/bin/formicary ant \
    --config config/formicary-ant.yaml \
    --id formicary-ant-1 \
    --tags "docker shell"
```

Or via Docker:

```bash
docker run --rm \
  --network host \
  -e COMMON_QUEUE_WEBSOCKET_SERVER_ENDPOINT="ws://queen.example.com:7777/ws/queue" \
  -e COMMON_S3_ENDPOINT="queen.example.com:19000" \
  -e COMMON_S3_ACCESS_KEY_ID="localkey" \
  -e COMMON_S3_SECRET_ACCESS_KEY="localsecret" \
  -v "$(pwd)/config/formicary-ant.yaml:/config/formicary-ant.yaml:ro" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  plexobject/formicary:latest \
  ant --config /config/formicary-ant.yaml --id formicary-ant-1 --tags "docker shell"
```

If the ant is restarted while the queen is unreachable, undelivered messages are buffered in a local SQLite file (default `/tmp/formicary-ant-buffer.db`) and drained automatically after reconnection.

See [Running Formicary](./running.md) for the full env-variable reference and more deployment patterns.

---

## Running from Source (for Development)

If you plan to contribute to Formicary, you'll want to run it directly from source.

1.  **Queen + embedded ant (simplest)**

    ```bash
    make run
    ```

    No external services needed — everything is embedded.

2.  **Queen-only + separate ant**

    ```bash
    # Terminal 1: queen
    make run-queen

    # Terminal 2: ant worker
    make ant
    ```

3.  **Manual CLI (after `make build`)**

    ```bash
    # Queen only (no embedded ant)
    ./out/bin/formicary --config config/formicary-queen.yaml --id=queen-server-1

    # Ant worker (connects to queen via WebSocket)
    ./out/bin/formicary ant \
      --config config/formicary-ant.yaml \
      --id=local-ant-1 \
      --tags="shell,docker"
    ```
