# Installation

This guide covers how to get Formicary up and running. The recommended method is deploying to Kubernetes — it gives the server in-cluster credentials automatically so Kubernetes job execution works without any extra configuration.

## Prerequisites

- **Kubernetes:** A running cluster — Docker Desktop (enable Kubernetes in Settings), [k3s](https://k3s.io/), [MicroK8s](https://microk8s.io/), or any cloud provider. `kubectl` must be configured.
- **Docker:** To build images or run the Docker option.
- **(Optional) Git:** To clone the repository.
- **(Optional) Go 1.22+:** To build from source.

---

## Option 1: Kubernetes (Recommended)

Running inside Kubernetes gives the server in-cluster credentials automatically — no kubeconfig mount or address rewriting needed. The same cluster is used to schedule job pods.

### 1a: All-in-one local deployment (Docker Desktop)

The fastest way to get a fully functional local instance — queen, embedded ant, and artifact storage in a single pod:

```bash
git clone https://github.com/bhatti/formicary.git
cd formicary

# Set required env vars (add to ~/.zshrc)
export COMMON_AUTH_JWT_SECRET="$(openssl rand -hex 32)"
# Optional Slack tokens:
# export SLACK_APP_TOKEN="xapp-..."
# export SLACK_BOT_TOKEN="xoxb-..."

source ~/.zshrc
bash scripts/install.sh --local
```

Open [http://localhost:7777](http://localhost:7777). The script:
1. Creates Kubernetes secrets (`formicary-auth`, `formicary-slack`, `ai-dev-credentials`)
2. Deploys `k8s/formicary-all-in-one.yaml` to docker-desktop
3. Port-forwards 7777 and 19000 in the background
4. Deploys AI workflow YAMLs and pushes Slack routes once `FORMICARY_TOKEN` is set

> **First login:** With `auth.enabled: false` (default for `--local`) no OAuth is needed — register at [http://localhost:7777/dashboard](http://localhost:7777/dashboard), then copy your API token and add `export FORMICARY_TOKEN="<token>"` to `~/.zshrc`. Re-run `install.sh --local` to finish deploying workflows.

Stop: `kill $(cat /tmp/formicary-pf.pid)` then `kubectl delete -f k8s/formicary-all-in-one.yaml`.

### 1b: Manual Kubernetes steps

```bash
# With Google OAuth:
kubectl create secret generic formicary-auth \
  --from-literal=jwt-secret="$(openssl rand -base64 32)" \
  --from-literal=google-client-id="${COMMON_AUTH_GOOGLE_CLIENT_ID}" \
  --from-literal=google-client-secret="${COMMON_AUTH_GOOGLE_CLIENT_SECRET}"

# Without OAuth (local testing):
kubectl create secret generic formicary-auth \
  --from-literal=jwt-secret="$(openssl rand -base64 32)"

kubectl apply -f k8s/formicary-all-in-one.yaml
kubectl rollout status deployment/formicary --timeout=120s
kubectl port-forward svc/formicary 7777:7777 19000:19000
```

Open [http://localhost:7777](http://localhost:7777).

Stop / remove:
```bash
kubectl delete -f k8s/formicary-all-in-one.yaml
kubectl delete secret formicary-auth
kubectl delete pvc formicary-data
```

### What `k8s.yaml` creates

| Resource | Purpose |
|----------|---------|
| `ServiceAccount` | Identity for the pod |
| `ClusterRole` + `ClusterRoleBinding` | Permission to create/delete job pods |
| `PersistentVolumeClaim` (10Gi) | SQLite DB + SeaweedFS artifact storage |
| `ConfigMap` | Formicary server config |
| `Deployment` (1 replica) | Queen + embedded ant + SeaweedFS |
| `Service` | Exposes ports 7777 and 19000 |

---

## Option 2: Docker (no Kubernetes job support)

Use this for quick local exploration. Kubernetes-based job execution will not work because the container has no access to the Kubernetes API.

```bash
docker run -d \
  --name formicary \
  -p 7777:7777 \
  -p 19000:19000 \
  -e COMMON_AUTH_JWT_SECRET="$(openssl rand -base64 32)" \
  -e COMMON_AUTH_GOOGLE_CLIENT_ID="${COMMON_AUTH_GOOGLE_CLIENT_ID}" \
  -e COMMON_AUTH_GOOGLE_CLIENT_SECRET="${COMMON_AUTH_GOOGLE_CLIENT_SECRET}" \
  -v formicary-data:/data \
  -v /var/run/docker.sock:/var/run/docker.sock \
  plexobject/formicary:latest
```

Without OAuth:

```bash
docker run -d \
  --name formicary \
  -p 7777:7777 \
  -p 19000:19000 \
  -e COMMON_AUTH_ENABLED=false \
  -v formicary-data:/data \
  -v /var/run/docker.sock:/var/run/docker.sock \
  plexobject/formicary:latest
```

Open [http://localhost:7777](http://localhost:7777).

Stop:

```bash
docker stop formicary && docker rm formicary
docker volume rm formicary-data   # also deletes persistent data
```

---

## Option 3: Docker Compose

```bash
export COMMON_AUTH_JWT_SECRET="$(openssl rand -base64 32)"
export COMMON_AUTH_GOOGLE_CLIENT_ID="<your-client-id>"
export COMMON_AUTH_GOOGLE_CLIENT_SECRET="<your-client-secret>"

docker compose up -d
```

---

## Option 4: Run from Source

### Via install script (recommended for dev iteration)

```bash
git clone https://github.com/bhatti/formicary.git
cd formicary
source ~/.zshrc

# Build and run the binary directly — no Docker image rebuild needed
bash scripts/install.sh --local-dev
```

`--local-dev` builds `./out/bin/formicary` via `make build`, starts it with `auth.enabled=false` (no OAuth needed), SQLite at `./formicary_db.sqlite`, and local SeaweedFS. Deploys AI workflow YAMLs if `FORMICARY_TOKEN` is set. Logs to `/tmp/formicary-dev.log`. Workflow pods still run on docker-desktop Kubernetes. Use this to test Go code changes immediately without pushing a Docker image.

Stop: `kill $(cat /tmp/formicary-dev.pid)`

### Via Makefile directly

```bash
git clone https://github.com/bhatti/formicary.git
cd formicary

# Queen + embedded ant (simplest — no external services)
make run

# Queen only + separate ant in another terminal
make run-queen   # terminal 1
make ant         # terminal 2
```

Open [http://localhost:7777](http://localhost:7777).

---

## Verifying the Installation

1. Open [http://localhost:7777](http://localhost:7777) — you should see the dashboard.
2. Upload and run the hello-world example from the [Quick Start](./03-quick-start.md) guide.
3. For AI workflows, see [AI Agents](./ai-agents.md) and the deploy scripts in `docs/examples/`.

---

## Database Schema Migrations

Formicary uses GORM AutoMigrate to add new columns automatically on fresh installs. For **existing production databases** (e.g., a running EC2 instance), new columns must be added manually.

### report_files_serialized column (added in report-viewer feature)

For an existing SQLite database on EC2, run once after deploying the new queen image:

```bash
kubectl exec -n default <queen-pod-name> -- sqlite3 /data/formicary.db \
  "ALTER TABLE formicary_artifacts ADD COLUMN report_files_serialized TEXT;"
```

For MySQL/PostgreSQL:

```sql
ALTER TABLE formicary_artifacts ADD COLUMN report_files_serialized TEXT;
```

New instances and fresh deployments do not need this step — AutoMigrate handles it automatically.
