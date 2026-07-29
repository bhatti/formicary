## Running Formicary

Formicary can run in several modes. The configs (`formicary-queen.yaml`, `formicary-queen-embedded.yaml`) default to `/data` paths — correct for Docker where `~/formicary-data` is mounted at `/data`. Local `make run*` targets override these to local relative paths automatically.

| Mode | Command | Data location |
|------|---------|---------------|
| **Queen + embedded ant** (local, from source) | `make run` | `./formicary_db.sqlite`, `./data/seaweedfs` |
| **Queen + embedded ant** (Docker, default) | `make docker-run` | `~/formicary-data/` |
| **Queen + embedded ant** (Docker, explicit) | `make docker-run-embedded` | `~/formicary-data/` |
| **Queen only** (local, from source) | `make run-queen` | `./formicary_db.sqlite`, `./data/seaweedfs` |
| **Queen only** (Docker) | `make docker-run-queen` | `~/formicary-data/` |
| **Ant** (local queen) | `make ant` | — |
| **Ant** (remote queen) | `make ant-remote QUEEN_URL=ws://...` | — |
| **Ant** (remote queen, Docker) | `make ant-docker QUEEN_URL=ws://...` | — |

---

### Queen-only mode (no embedded ant)

Run the queen server without a built-in ant. Ants connect via WebSocket from separate processes or machines.

#### Via Make (local)

```bash
make run-queen          # queen only, no embedded ant
```

Uses `config/formicary-queen.yaml`. Paths are overridden to local relative paths automatically. The WebSocket endpoint is at `ws://localhost:7777/ws/queue`.

#### Via CLI (after `make build`)

```bash
DB_DATA_SOURCE="./formicary_db.sqlite" \
COMMON_S3_LOCAL_DATA_DIR="./data/seaweedfs" \
COMMON_S3_LOCAL_WEED_BIN="./bin/weed" \
COMMON_PUBLIC_DIR="./public/" \
./out/bin/formicary --config config/formicary-queen.yaml
```

#### Via Docker

```bash
make docker-run-queen COMMON_AUTH_ENABLED=false
```

Data persists to `~/formicary-data/` (mounted as `/data` inside the container). No extra env vars needed — the config already uses `/data` paths.

Or with plain `docker run`:

```bash
docker run --rm \
  -p 7777:7777 \
  -p 19000:19000 \
  -e COMMON_AUTH_ENABLED=false \
  -e COMMON_AUTH_JWT_SECRET="$(openssl rand -base64 32)" \
  -v ~/formicary-data:/data \
  -v "$(pwd)/config/formicary-queen.yaml:/config/formicary-queen.yaml:ro" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  plexobject/formicary:latest \
  --config /config/formicary-queen.yaml
```

---

### Starting Ant workers

Ants connect to the queen via WebSocket. The key setting is `common.queue.websocket.server_endpoint`.

#### Via Make

```bash
make ant                    # connects to localhost:7777
```

Uses `config/formicary-ant.yaml`, which already points to `ws://localhost:7777/ws/queue`.

#### Via CLI

```bash
./out/bin/formicary ant \
  --config config/formicary-ant.yaml \
  --id formicary-ant-1 \
  --port 7771 \
  --tags "docker shell builder"
```

#### Overriding the queen URL via environment variable

The config key `common.queue.websocket.server_endpoint` maps to the environment variable
`COMMON_QUEUE_WEBSOCKET_SERVER_ENDPOINT` (dots replaced by underscores, no prefix).

```bash
# Point ant at a remote queen
export COMMON_QUEUE_WEBSOCKET_SERVER_ENDPOINT="ws://queen.example.com:7777/ws/queue"

./out/bin/formicary ant \
  --config config/formicary-ant.yaml \
  --id formicary-ant-1 \
  --tags "docker shell"
```

Or inline:

```bash
COMMON_QUEUE_WEBSOCKET_SERVER_ENDPOINT="ws://192.168.1.10:7777/ws/queue" \
  ./out/bin/formicary ant --config config/formicary-ant.yaml --id ant-1
```

The S3/artifact endpoint follows the same pattern. If the queen's embedded SeaweedFS is on port 19000:

```bash
export COMMON_S3_ENDPOINT="192.168.1.10:19000"
export COMMON_S3_ACCESS_KEY_ID="localkey"
export COMMON_S3_SECRET_ACCESS_KEY="localsecret"
```

#### Via Docker (ant-only container)

```bash
docker run --rm \
  --network host \
  -e COMMON_QUEUE_WEBSOCKET_SERVER_ENDPOINT="ws://localhost:7777/ws/queue" \
  -e COMMON_S3_ENDPOINT="localhost:19000" \
  -e COMMON_S3_ACCESS_KEY_ID="localkey" \
  -e COMMON_S3_SECRET_ACCESS_KEY="localsecret" \
  -v "$(pwd)/config/formicary-ant.yaml:/config/formicary-ant.yaml:ro" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  plexobject/formicary:latest \
  ant --config /config/formicary-ant.yaml --id formicary-ant-1 --tags "docker shell"
```

Or using the provided `ant-docker-compose.yaml`:

```bash
QUEEN_SERVER=192.168.1.10 docker compose -f ant-docker-compose.yaml up
```

---

### Running via Docker Compose (all-in-one)

The default `docker-compose.yaml` runs queen + embedded ant in a single container — no Redis, MinIO, or separate ant needed.

```bash
# Quickstart (no git clone required)
curl -fsSL https://raw.githubusercontent.com/bhatti/formicary/main/docker-compose.yaml -o docker-compose.yaml
mkdir -p config
curl -fsSL https://raw.githubusercontent.com/bhatti/formicary/main/config/formicary-docker.yaml -o config/formicary-docker.yaml

# No-auth mode (local testing only)
COMMON_AUTH_ENABLED=false docker compose up

# With OAuth auth
export COMMON_AUTH_JWT_SECRET=$(openssl rand -base64 32)
export COMMON_AUTH_GOOGLE_CLIENT_ID=<your-client-id>
export COMMON_AUTH_GOOGLE_CLIENT_SECRET=<your-client-secret>
docker compose up
```

Shut down:

```bash
docker compose down
```

---

### Kubernetes (docker-desktop, namespace: default)

The `k8s.yaml` deploys everything to the `default` namespace on docker-desktop.

#### First-time deploy

```bash
kubectl apply -f k8s.yaml
kubectl rollout status deployment/formicary -n default
```

#### Rebuild and redeploy after code changes

```bash
# 1. Build and push the image
make docker-build          # builds plexobject/formicary:latest locally
make docker-release        # tags + pushes to Docker Hub

# 2. Force k8s to re-pull and restart (imagePullPolicy: Always)
kubectl rollout restart deployment/formicary -n default
kubectl rollout status deployment/formicary -n default

# 3. Confirm the new pod is running
kubectl get pods -n default -l app=formicary
```

#### Access the dashboard

```bash
kubectl port-forward svc/formicary 7777:7777 -n default
open http://localhost:7777
```

#### Get a fresh API token (required after every image rebuild)

The JWT signing key changes on each server start — old tokens are immediately invalid.

1. Open http://localhost:7777/login and sign in with Google/GitHub
2. Go to **Dashboard → API Tokens → New Token**
3. Copy the token and export it:

```bash
export FORMICARY_TOKEN=<paste-token-here>
# Add to ~/.zshrc to persist:
echo 'export FORMICARY_TOKEN=<paste-token-here>' >> ~/.zshrc
source ~/.zshrc
```

#### Verify the token works

```bash
curl -sf -H "Authorization: Bearer $FORMICARY_TOKEN" \
  http://localhost:7777/api/orgs/default/configs | python3 -m json.tool | head -20
```

#### Check pod logs

```bash
kubectl logs -n default -l app=formicary --tail=50 -f
```

#### Redeploy workflow YAMLs and org configs

After a fresh token is set:

```bash
cd docs/examples

# Jira + Bitbucket AI workflows (JIRA_PROJECT auto-discovered from API)
./deploy-ai-jira-workflows.sh --set-configs \
  --bb-workspace myworkspace \
  --bb-repo myrepo

# GitHub AI workflows
./deploy-ai-workflows.sh --set-configs \
  --slack-channel my-channel
```

Credentials (`JIRA_API_TOKEN`, `BITBUCKET_TOKEN`, `SSH_PRIVATE_KEY`, etc.) must be set
as environment variables or live in `~/.config/acli/config.json` — never passed as CLI flags.

#### Wipe and start fresh (nuclear option)

```bash
kubectl delete -f k8s.yaml
kubectl apply -f k8s.yaml
kubectl rollout status deployment/formicary -n default
```

---

### Running manually from source

```bash
# Start the queen (queen-only, ants connect separately)
make run-queen

# In a second terminal: start an ant
make ant

# Or start queen + embedded ant together
make run
```

See `config/formicary-queen.yaml` for queen-only settings and `config/formicary-ant.yaml` for ant settings.

---

### Data storage

| Run mode | SQLite DB | SeaweedFS blobs |
|----------|-----------|-----------------|
| `make run` / `make run-queen` | `./formicary_db.sqlite` (project dir) | `./data/seaweedfs/` (project dir) |
| `make docker-run*` / `make docker-run-queen` | `~/formicary-data/formicary.db` | `~/formicary-data/seaweedfs/` |
| `docker compose up` | Docker volume `formicary-data` | same volume |

The configs default to `/data` paths (correct for Docker). The `make run*` targets pass `DB_DATA_SOURCE` and `COMMON_S3_LOCAL_DATA_DIR` env vars to redirect to local relative paths — no manual config editing needed.

---

### Environment variable reference

All config keys map to environment variables by replacing `.` with `_` (no prefix).

| Environment variable | Config key | Description |
|---------------------|-----------|-------------|
| `COMMON_QUEUE_WEBSOCKET_SERVER_ENDPOINT` | `common.queue.websocket.server_endpoint` | Queen WebSocket URL for ants to connect to |
| `COMMON_HTTP_PORT` | `common.http_port` | HTTP port (default `7777` for queen, `0` for ant) |
| `COMMON_PUBLIC_DIR` | `common.public_dir` | Path to static UI assets |
| `COMMON_S3_ENDPOINT` | `common.s3.endpoint` | Artifact storage S3 endpoint |
| `COMMON_S3_ACCESS_KEY_ID` | `common.s3.access_key_id` | S3 access key |
| `COMMON_S3_SECRET_ACCESS_KEY` | `common.s3.secret_access_key` | S3 secret key |
| `COMMON_S3_LOCAL_DATA_DIR` | `common.s3.local_data_dir` | SeaweedFS data directory (default `/data/seaweedfs`) |
| `COMMON_S3_LOCAL_WEED_BIN` | `common.s3.local_weed_bin` | Path to `weed` binary (default `/usr/local/bin/weed`) |
| `COMMON_QUEUE_TOKEN` | `common.queue.token` | Ant's API JWT (`token_type=api`) for WebSocket auth — generate via Dashboard → API Tokens; set on ant only |
| `COMMON_AUTH_ENABLED` | `common.auth.enabled` | Enable/disable OAuth auth |
| `COMMON_AUTH_JWT_SECRET` | `common.auth.jwt_secret` | JWT signing secret |
| `COMMON_AUTH_GOOGLE_CLIENT_ID` | `common.auth.google_client_id` | Google OAuth client ID |
| `COMMON_AUTH_GOOGLE_CLIENT_SECRET` | `common.auth.google_client_secret` | Google OAuth client secret |
| `COMMON_AUTH_GITHUB_CLIENT_ID` | `common.auth.github_client_id` | GitHub OAuth client ID |
| `COMMON_AUTH_GITHUB_CLIENT_SECRET` | `common.auth.github_client_secret` | GitHub OAuth client secret |
| `DB_DATA_SOURCE` | `db.data_source` | SQLite file path or DB connection string (default `/data/formicary.db`) |

---

### Running behind a proxy

```bash
export HTTP_PROXY=http://myproxy:3128
export HTTPS_PROXY=http://myproxy:3128
export NO_PROXY=localhost,127.0.0.1
```

Or set `proxy_url` in the config:

```yaml
common:
    proxy_url: "https://myproxy:3128"
```

---

### Open the Dashboard

After starting the queen, open [http://localhost:7777](http://localhost:7777) in a browser.
