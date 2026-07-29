# Formicary Examples

Runnable workflow definitions and deploy scripts for common use cases.

---

## Quick Start: Run the Server

The single-container image bundles the queen, an embedded ant worker, SeaweedFS (artifact storage), and SQLite. No Redis, MinIO, or separate database needed.

### Option 1: Kubernetes (recommended)

Running inside Kubernetes gives the server in-cluster credentials automatically — no kubeconfig mount or host address rewriting needed. This is the same cluster where job pods run.

**With Google OAuth:**
```bash
# export COMMON_AUTH_GOOGLE_CLIENT_ID="<id>.apps.googleusercontent.com"
# export COMMON_AUTH_GOOGLE_CLIENT_SECRET="<secret>"
kubectl create secret generic formicary-auth \
  --from-literal=jwt-secret="$(openssl rand -base64 32)" \
  --from-literal=google-client-id="${COMMON_AUTH_GOOGLE_CLIENT_ID}" \
  --from-literal=google-client-secret="${COMMON_AUTH_GOOGLE_CLIENT_SECRET}"

kubectl apply -f ../../k8s.yaml
kubectl port-forward svc/formicary 7777:7777 19000:19000
```

**Without auth (local dev/testing):**
```bash
kubectl create secret generic formicary-auth \
  --from-literal=jwt-secret="$(openssl rand -base64 32)"

kubectl apply -f ../../k8s.yaml
kubectl port-forward svc/formicary 7777:7777 19000:19000
```

UI → `http://localhost:7777` · Artifacts → `http://localhost:19000`

Google OAuth callback URL to register in Cloud Console:
```
http://localhost:7777/auth/google/callback
```

**Stop / remove:**
```bash
kubectl delete -f ../../k8s.yaml
kubectl delete secret formicary-auth
kubectl delete pvc formicary-data   # also deletes persistent data
```

---

### Option 2: Docker

```bash
docker run -d \
  --name formicary \
  -p 7777:7777 \
  -p 19000:19000 \
  -e COMMON_AUTH_JWT_SECRET="${COMMON_AUTH_JWT_SECRET}" \
  -e COMMON_AUTH_GOOGLE_CLIENT_ID="${COMMON_AUTH_GOOGLE_CLIENT_ID}" \
  -e COMMON_AUTH_GOOGLE_CLIENT_SECRET="${COMMON_AUTH_GOOGLE_CLIENT_SECRET}" \
  -v formicary-data:/data \
  -v /var/run/docker.sock:/var/run/docker.sock \
  --restart unless-stopped \
  plexobject/formicary:latest
```

> **Note:** Kubernetes jobs will not work when running via plain `docker run` because the server has no access to the Kubernetes API. Use the Kubernetes deployment above for AI workflows.

UI → `http://localhost:7777` · Artifacts → `http://localhost:19000`

**Stop / remove data:**
```bash
docker stop formicary && docker rm formicary
docker volume rm formicary-data
```

### Get an API token

Log in at `http://localhost:7777`, go to **Profile → API Token**, copy the JWT. Set it in your shell for all deploy scripts:

```bash
export FORMICARY_TOKEN="<jwt-from-ui>"
```

---

## AI Agent Workflows

These workflows run [Claude Code](https://github.com/bhatti/ai-dev-tools) inside Kubernetes pods to autonomously implement GitHub/Jira issues, perform standup analysis, and clean up stale branches.

### Prerequisites

Formicary must be running in Kubernetes (see [Installation](../02-installation.md)). The embedded ant worker spawns all AI task pods — no separate setup needed.

```bash
# If not already deployed:
kubectl apply -f ../../k8s.yaml
kubectl port-forward svc/formicary 7777:7777 19000:19000
```

---

### Credentials

All AI workflow tasks read credentials from a Kubernetes secret named `ai-dev-credentials`. Create it once from your environment variables — every deploy script supports `--create-k8s-secret` for this:

```bash
# GitHub workflows
export GH_TOKEN="<github-pat>"          # needs repo + issues + pull_requests scopes
export SLACK_BOT_TOKEN="xoxb-..."
export SSH_PRIVATE_KEY="$(cat ~/.ssh/id_rsa)"
export ANTHROPIC_API_KEY="..."          # optional — omit when using Bedrock

./deploy-ai-workflows.sh --create-k8s-secret

# Jira/Bitbucket workflows (credentials auto-read from ~/.config/acli/config.json)
export JIRA_API_TOKEN="<token>"
export BITBUCKET_TOKEN="<app-password>"
export SSH_PRIVATE_KEY="$(cat ~/.ssh/id_rsa)"

./deploy-ai-jira-workflows.sh --create-k8s-secret
```

The secret is idempotent — re-running `--create-k8s-secret` updates it in place. Non-secret org config (project names, channel names, etc.) is pushed separately with `--set-configs`.

---

### GitHub AI Agent (`ai-gh-implement` + `ai-gh-issue-picker` + `ai-gh-cleanup`)

Polls GitHub for issues labeled `ai-ready`, plans and implements them with Claude Code, opens a PR, and polls until merged or abandoned.

**Deploy:**

```bash
cd docs/examples

export FORMICARY_TOKEN="<jwt>"
export GH_TOKEN="<github-pat>"
export SSH_PRIVATE_KEY="$(cat ~/.ssh/id_rsa)"

# One-time: create K8s secret and set org configs
./deploy-ai-workflows.sh \
  --create-k8s-secret \
  --set-configs \
  --gh-org YOUR_ORG \
  --gh-repo YOUR_REPO

# Subsequent deploys (secret already exists):
./deploy-ai-workflows.sh \
  --set-configs \
  --gh-org YOUR_ORG \
  --gh-repo YOUR_REPO
```

Use `--bedrock` to route Claude through an AWS Bedrock proxy instead of the direct Anthropic API:

```bash
./deploy-ai-workflows.sh --set-configs --gh-org ORG --gh-repo REPO \
  --bedrock --bedrock-url http://ai/bedrock
```

**Create the required GitHub labels** (one-time):

```bash
./deploy-ai-workflows.sh --setup-labels --gh-org ORG --gh-repo REPO
```

**Trigger:** Add the `ai-ready` label to any issue — the picker cron runs every 5 minutes.

---

### Jira AI Agent (`ai-jira-implement` + `ai-jira-issue-picker`)

Same flow as GitHub but sources issues from Jira and commits to Bitbucket.  
Reads credentials from `~/.config/acli/config.json` automatically.

**Deploy:**

```bash
cd docs/examples

export FORMICARY_TOKEN="<jwt>"
# Credentials auto-read from ~/.config/acli/config.json
# Override any with env vars: JIRA_API_TOKEN, BITBUCKET_TOKEN, etc.

# One-time: create K8s secret and set org configs
./deploy-ai-jira-workflows.sh \
  --create-k8s-secret \
  --set-configs \
  --jira-project MYPROJ \
  --bb-workspace myworkspace \
  --bb-repo myrepo

# Subsequent deploys (secret already exists):
./deploy-ai-jira-workflows.sh \
  --set-configs \
  --jira-project MYPROJ \
  --bb-workspace myworkspace \
  --bb-repo myrepo
```

**Trigger:** Add the `ai-ready` label to a Jira issue.

---

### AI Standup Brief — Jira (`ai-standup-jira`)

Runs every weekday at **8:00 AM**. Gathers the active sprint from Jira, open Bitbucket PRs, and recent Slack messages, then uses Claude to produce a standup brief and risk report, and posts it to Slack.

**Deploy:**

```bash
cd docs/examples

export FORMICARY_TOKEN="<jwt>"
export SLACK_BOT_TOKEN="xoxb-..."
# Jira/Bitbucket credentials auto-read from ~/.config/acli/config.json

# One-time: create K8s secret and set org configs
./deploy-ai-standup-jira.sh \
  --create-k8s-secret \
  --set-configs \
  --jira-project MYPROJ \
  --slack-channel standup

# Optional: scope the brief to specific team members
./deploy-ai-standup-jira.sh --set-configs --jira-project MYPROJ \
  --slack-channel standup --team-members "Alice,Bob,Charlie"
```

Invite the bot to the channel first: `/invite @<bot-name>` in Slack.

**Trigger manually** (before the first 8am cron fires):

```bash
curl -X POST http://localhost:7777/api/jobs/requests \
  -H "Authorization: Bearer ${FORMICARY_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"job_type": "ai-standup-jira"}'
```

**Artifacts produced** (visible in the UI under the job run):

| File | Retention | Contents |
|------|-----------|----------|
| `signals.json` | 24h | Raw Jira/Bitbucket/Slack data |
| `standup_brief.md` | 72h | Per-person status + risks (also posted to Slack) |
| `risk_report.md` | 72h | Full ranked risk list with recommended actions |
| `standup_report.md` | 7 days | Final combined report |

---

### AI Standup Brief — GitHub (`ai-standup-gh`)

Same as above but sources issues and PRs from GitHub instead of Jira/Bitbucket.

**Deploy:**

```bash
cd docs/examples

export FORMICARY_TOKEN="<jwt>"
export GH_TOKEN="<github-pat>"
export SLACK_BOT_TOKEN="xoxb-..."

# One-time: create K8s secret and set org configs
./deploy-ai-standup-gh.sh \
  --create-k8s-secret \
  --set-configs \
  --gh-org myorg \
  --gh-repo myrepo \
  --slack-channel standup

# Subsequent deploys (secret already exists):
./deploy-ai-standup-gh.sh \
  --set-configs \
  --gh-org myorg \
  --gh-repo myrepo \
  --slack-channel standup
```

**Trigger manually:**

```bash
curl -X POST http://localhost:7777/api/jobs/requests \
  -H "Authorization: Bearer ${FORMICARY_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"job_type": "ai-standup-gh"}'
```

---

## Deploy Script Reference

| Script | Uploads | Tracker |
|--------|---------|---------|
| `deploy-ai-workflows.sh` | `ai-gh-issue-picker`, `ai-gh-implement`, `ai-gh-cleanup` | GitHub |
| `deploy-ai-jira-workflows.sh` | `ai-jira-issue-picker`, `ai-jira-implement` | Jira + Bitbucket |
| `deploy-ai-standup-jira.sh` | `ai-standup-jira` | Jira + Bitbucket + Slack |
| `deploy-ai-standup-gh.sh` | `ai-standup-gh` | GitHub + Slack |



All scripts support:

```
--server URL           Formicary queen URL (default: http://localhost:7777)
--create-k8s-secret    Create/update the 'ai-dev-credentials' K8s secret from env vars
--set-configs          Push non-secret org configs to the server (requires FORMICARY_TOKEN)
--bedrock              Route Claude through AWS Bedrock proxy
--bedrock-url URL      Bedrock proxy URL (default: http://ai/bedrock)
--help                 Show usage
```

Credentials go in the `ai-dev-credentials` Kubernetes secret (via `--create-k8s-secret`). Non-secret config is pushed via `--set-configs`. Secrets are **always** passed via environment variables, never CLI flags:

```bash
FORMICARY_TOKEN        # Formicary JWT (from Profile → API Token in the UI)
GH_TOKEN               # GitHub PAT (also GITHUB_TOKEN)
JIRA_API_TOKEN         # Jira API token
BITBUCKET_TOKEN        # Bitbucket app password
SLACK_BOT_TOKEN        # Slack bot token (xoxb-...)
ANTHROPIC_API_KEY      # Anthropic API key (optional when using Bedrock)
SSH_PRIVATE_KEY        # SSH key for git push (AI agent workflows)
```

---

## Other Example Workflows

| File | Description |
|------|-------------|
| `hello_world.yaml` | Minimal shell job |
| `hello_world_scheduled.yaml` | Cron-triggered shell job |
| `go-build-ci.yaml` | Go build, test, and lint CI |
| `python-ci.yaml` | Python CI with pytest |
| `maven-build.yaml` | Java/Maven build |
| `parallel-video-encoding.yaml` | Fan-out video transcoding |
| `etl-stock-job.yaml` | ETL pipeline with HTTP data source |
| `approval-unanimous.yaml` | Multi-party approval gate |
| `trivy-scan-job.yaml` | Container security scan |
| `dind-job.yaml` | Docker-in-Docker build |
| `sensor-job.yaml` | Event-driven sensor trigger |

Upload any YAML directly:

```bash
curl -X POST http://localhost:7777/api/jobs/definitions \
  -H "Authorization: Bearer ${FORMICARY_TOKEN}" \
  -H "Content-Type: application/yaml" \
  --data-binary @hello_world.yaml
```

Trigger it:

```bash
curl -X POST http://localhost:7777/api/jobs/requests \
  -H "Authorization: Bearer ${FORMICARY_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"job_type": "hello_world"}'
```

---

## Further Reading

- [Architecture](../architecture.md)
- [Job YAML schema](../06-job-definitions.md)
- [Executors (Kubernetes, Docker, Shell)](../07-executors.md)
- [Scheduling and triggers](../08-scheduling-and-triggers.md)
- [Artifacts and caching](../09-artifacts-and-caching.md)
- [AI agents guide](../ai-agents.md)
