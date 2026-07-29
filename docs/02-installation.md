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

### Steps

**1. Create the auth secret**

```bash
# With Google OAuth (set env vars first):
# export COMMON_AUTH_GOOGLE_CLIENT_ID="<id>.apps.googleusercontent.com"
# export COMMON_AUTH_GOOGLE_CLIENT_SECRET="<secret>"
kubectl create secret generic formicary-auth \
  --from-literal=jwt-secret="$(openssl rand -base64 32)" \
  --from-literal=google-client-id="${COMMON_AUTH_GOOGLE_CLIENT_ID}" \
  --from-literal=google-client-secret="${COMMON_AUTH_GOOGLE_CLIENT_SECRET}"
```

Without OAuth (local testing):
```bash
kubectl create secret generic formicary-auth \
  --from-literal=jwt-secret="$(openssl rand -base64 32)"
```

**2. Deploy**

```bash
kubectl apply -f k8s.yaml
```

**3. Access**

```bash
kubectl port-forward svc/formicary 7777:7777 19000:19000
```

Open [http://localhost:7777](http://localhost:7777).

Google OAuth callback URL to register in Cloud Console:
```
http://localhost:7777/auth/google/callback
```

**4. Stop / remove**

```bash
kubectl delete -f k8s.yaml
kubectl delete secret formicary-auth
kubectl delete pvc formicary-data   # also deletes all persistent data
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
