# AI Agent Orchestration with Formicary

## Overview

Formicary can orchestrate AI coding agents declaratively — no custom coordinator code, no hand-rolled state machines, just YAML job definitions and encrypted secrets.

This guide covers a complete, production-ready setup where:
- A cron job polls GitHub for issues and submits work to a capacity-limited queue
- An AI implementation workflow plans, implements, reviews, and opens pull requests
- PR feedback is addressed automatically when reviewers comment
- Periodic cleanup handles stale workspaces and merged branches

All jobs are prefixed `ai-gh-*` so they can be cloned to `ai-jira-*` or `ai-bb-*` for other issue trackers.

---

## Architecture

```
GitHub Issues (labeled: ai-ready)
        │
        ▼ every minute
┌───────────────────────────────────────────────────────┐
│  ai-gh-issue-picker (Formicary cron job)              │
│  - skip_if: CountByJobTypeAndState >= 10              │
│  - Checks capacity: pending ai-gh-implement jobs      │
│  - Fetches issues up to MaxPendingJobs limit          │
│  - Submits ai-gh-implement with hyperlinked params    │
│  - Transitions labels: ai-ready → ai-in-progress      │
└───────────────┬───────────────────────────────────────┘
                │ submits
                ▼
┌───────────────────────────────────────────────────────┐
│  ai-gh-implement (Formicary DAG job, max 5 parallel)  │
│                                                       │
│  setup → plan → implement → fix-tests → review        │
│                                  ↓            ↓       │
│                            notify-blocked  create-pr  │
│                                  └────→ cleanup ←─┘   │
└───────────────────────────────────────────────────────┘
                │ (on PR review comments)
                ▼
┌───────────────────────────────────────────────────────┐
│  ai-gh-pr-feedback (submitted manually or via webhook)│
│  - Checks out PR branch                               │
│  - Fetches review comments                            │
│  - Runs Claude to address feedback                    │
│  - Posts summary comment on PR                        │
└───────────────────────────────────────────────────────┘

Every 4 hours:
┌───────────────────────────────────────────────────────┐
│  ai-gh-cleanup                                        │
│  - Removes stale workspaces (> 4 hours old)           │
│  - Deletes merged ai/* branches from GitHub           │
└───────────────────────────────────────────────────────┘
```

---

## Job Definitions

| File | Job Type | Trigger |
|------|----------|---------|
| `docs/examples/ai-gh-issue-picker.yaml` | `ai-gh-issue-picker` | Cron: every minute |
| `docs/examples/ai-gh-implement.yaml` | `ai-gh-implement` | Submitted by picker |
| `docs/examples/ai-gh-pr-feedback.yaml` | `ai-gh-pr-feedback` | Manual or webhook |
| `docs/examples/ai-gh-cleanup.yaml` | `ai-gh-cleanup` | Cron: every 4 hours |

---

## Prerequisites

### 1. Ant Worker Setup

#### Option A: SHELL Execution (Local / Development)

The ant worker needs these tools installed on the host:

```bash
# Claude Code CLI
npm install -g @anthropic-ai/claude-code

# Authenticate — choose one:
#   Option 1: Direct API key
claude auth login                    # interactive, or set ANTHROPIC_API_KEY

#   Option 2: AWS Bedrock via Tailscale (no API key needed)
#   ~/.claude/settings.json is read automatically by the claude CLI.
#   The ant worker process inherits it with no extra configuration.
#   Example settings.json for Tailscale/Bedrock:
#   {
#     "env": {
#       "ANTHROPIC_BEDROCK_BASE_URL": "http://<tailscale-ip>/bedrock",
#       "CLAUDE_CODE_USE_BEDROCK": "1",
#       "CLAUDE_CODE_SKIP_BEDROCK_AUTH": "1",
#       "ANTHROPIC_DEFAULT_OPUS_MODEL": "us.anthropic.claude-opus-4-6-v1",
#       "ANTHROPIC_DEFAULT_SONNET_MODEL": "us.anthropic.claude-sonnet-4-6",
#       "ANTHROPIC_DEFAULT_HAIKU_MODEL": "us.anthropic.claude-haiku-4-5-20251001-v1:0"
#     }
#   }

# GitHub CLI
gh auth login        # sets GITHUB_TOKEN in the process environment

# Optional: you-got-skills (skill methodology)
git clone https://github.com/bhatti/you-got-skills.git
cd you-got-skills && ./setup
```

Start the ant worker with SHELL method enabled:

```bash
formicary-ant \
  --server-url http://localhost:7777 \
  --tags "ai-worker" \
  --methods "SHELL"
```

To run with SHELL, change every task's `method: KUBERNETES` to `method: SHELL` and remove the `container:` blocks. Scripts are identical — only the execution environment changes.

#### Option B: KUBERNETES Execution (Production)

See [KUBERNETES Deployment](#kubernetes-deployment) below. No script changes required — just change the method and provide the container image.

### 2. Create GitHub Labels

The workflows transition issues through a set of labels. These must exist in the repo before the picker runs — GitHub returns an error if you try to apply a label that doesn't exist.

```bash
gh label create "ai-ready" \
  --repo <org>/<repo> \
  --color "0075ca" \
  --description "Ready for AI agent implementation"

gh label create "ai-in-progress" \
  --repo <org>/<repo> \
  --color "e4e669" \
  --description "AI agent is working on this"

gh label create "ai-pr-open" \
  --repo <org>/<repo> \
  --color "0e8a16" \
  --description "AI agent opened a PR"

gh label create "needs-human" \
  --repo <org>/<repo> \
  --color "d93f0b" \
  --description "AI agent was blocked — needs human review"
```

Label lifecycle managed by the workflows:

| Label | Applied by | Removed by |
|-------|-----------|-----------|
| `ai-ready` | Human | picker (on pickup) |
| `ai-in-progress` | picker | create-pr or notify-blocked |
| `ai-pr-open` | create-pr | — |
| `needs-human` | notify-blocked | — |

To trigger the workflow on an issue:

```bash
gh issue edit <number> --repo <org>/<repo> --add-label "ai-ready"
```

### 3. Configure Encrypted Secrets

Store API keys as org-level configs (encrypted at rest, redacted from logs):

```bash
BASE="http://localhost:7777/api/orgs/{org-id}/configs"
AUTH="-H 'Authorization: Bearer <admin-token>'"

# Anthropic API key
curl -X POST $BASE $AUTH -H 'Content-Type: application/json' \
  -d '{"name":"AnthropicApiKey","value":"sk-ant-...","secret":true}'

# GitHub personal access token (needs: repo, issues, pull_requests)
curl -X POST $BASE $AUTH -H 'Content-Type: application/json' \
  -d '{"name":"GithubToken","value":"ghp_...","secret":true}'

# Formicary API token (for capacity checks in picker)
curl -X POST $BASE $AUTH -H 'Content-Type: application/json' \
  -d '{"name":"FormicaryToken","value":"<formicary-api-token>","secret":true}'
```

Secrets are accessible in job YAML as `{{.AnthropicApiKey}}`, `{{.GithubToken}}`, etc.

### 4. Register Job Definitions

```bash
API="http://localhost:7777/api/jobs/definitions"
AUTH="-H 'Authorization: Bearer <token>'"

curl -X POST $API $AUTH -H 'Content-Type: application/yaml' \
  --data-binary @docs/examples/ai-gh-issue-picker.yaml

curl -X POST $API $AUTH -H 'Content-Type: application/yaml' \
  --data-binary @docs/examples/ai-gh-implement.yaml

curl -X POST $API $AUTH -H 'Content-Type: application/yaml' \
  --data-binary @docs/examples/ai-gh-pr-feedback.yaml

curl -X POST $API $AUTH -H 'Content-Type: application/yaml' \
  --data-binary @docs/examples/ai-gh-cleanup.yaml
```

### 5. Configure the Picker

Update `job_variables` in `ai-gh-issue-picker.yaml` before registering:

```yaml
job_variables:
  MaxPendingJobs: "10"   # max concurrent AI implementation jobs
  FormicaryURL: "http://localhost:7777"
  GitHubOrg: "your-org"
  GitHubRepo: "your-repo"
  PickupLabel: "ai-ready"
  InProgressLabel: "ai-in-progress"
```

---

## How It Works

### Context Flow Between Tasks

Formicary does not auto-parse stdout into template variables. Instead, tasks use **artifacts** to pass data downstream:

```
setup  ──[meta.env, branch.txt]──▶  plan
plan   ──[plan_result.json]──────▶  implement
impl   ──[impl_result.json]──────▶  fix-tests, review
review ──[review_result.json]────▶  create-pr
```

`meta.env` is a shell-sourceable file that downstream tasks `source` to get the workspace path and branch name:

```bash
source meta.env
# WS=/tmp/formicary-ai/42-a3f1
# BRANCH=ai/42-fix-login-a3f1
cd "$WS/repo"
```

### Restart Safety

Each job gets a **nonce** (4-byte random hex) appended to the branch name:
```
ai/42-fix-login-a3f1
```

If a job is retried (`retry: 1` on `ai-gh-implement`), the picker submits a new job with a fresh nonce, producing a fresh branch. The old branch is eventually cleaned up by `ai-gh-cleanup`.

### Capacity Management

The picker uses two layers of capacity enforcement:

**Layer 1 — `skip_if` (fast path, DB query, no HTTP):**

```yaml
skip_if: >-
  {{if ge (CountByJobTypeAndState "ai-gh-implement" "PENDING" "EXECUTING") 10}} true {{end}}
```

`CountByJobTypeAndState` is a Go template function built into Formicary that queries the job database directly. It takes a job type and one or more state names, and returns the count as an integer. If the count meets or exceeds the threshold, the entire picker invocation is skipped without creating any tasks — no HTTP call, no token required.

**Layer 2 — Script check (configurable limit):**

```bash
PENDING=$(curl .../api/jobs/requests?job_type=ai-gh-implement&state=PENDING,EXECUTING | jq .total_records)
SLOTS=$(( MAX_PENDING - PENDING ))
[ "$SLOTS" -le 0 ] && exit 0
```

The script check uses `MaxPendingJobs` from `job_variables`, which operators can tune without touching the `skip_if` expression. The `skip_if` hard-codes 10 as a fast-path guard; the script enforces the exact configured limit.

**Why two layers?** The `skip_if` fires before any ant worker is allocated, making it extremely cheap. The script check runs inside the task and uses the operator-configured value.

### Timeouts (No More Hanging Agents)

Every task has an explicit timeout. Without per-phase timeouts, a hung AI session can block a worker for the full job timeout (90 minutes):

| Task | Timeout | Reason |
|------|---------|--------|
| setup | 3m | Clone + branch |
| plan | 15m | Codebase exploration + WBS |
| implement | 45m | Code generation (most expensive phase) |
| fix-tests | 20m | Per-attempt (retries: 2) |
| review | 15m | Self-review with potential fixes |
| create-pr | 5m | Git push + API calls |
| cleanup | 1m | Workspace deletion |

### Dashboard Visibility

The `description` field on each submitted job is a markdown string visible in the Formicary dashboard:

```
#42: Fix login timeout | [myorg/myrepo](https://github.com/myorg/myrepo)
```

The PR link is added to artifacts once created. Future improvement: update job metadata during execution.

### Skill Integration (ygs-* Skills)

Claude Code tasks embed skill instructions directly in the prompt. If `you-got-skills` is installed on the ant worker, Claude also discovers `/ygs-*` slash commands automatically:

| Task | Skill | What it does |
|------|-------|--------------|
| plan | ygs-wbs | Work Breakdown Structure — vertical slice decomposition |
| implement | ygs-implement | Execution with scope guardrails, per-task commits |
| fix-tests | ygs-investigate | Root-cause debugging, not symptom masking |
| review | ygs-code-review | Two-pass review: critical first, informational second |
| create-pr | ygs-ship | PR creation with rich metadata |

---

## KUBERNETES Deployment

### Switching from SHELL to KUBERNETES

All four job definitions default to `method: KUBERNETES`. To run on a bare ant worker (SHELL), change every task's `method:` to `SHELL` and remove the `container:` blocks. The scripts are identical.

> **Important:** In KUBERNETES mode, each task runs in a **fresh pod** with an empty filesystem. The `setup` task clones the repository into `/tmp/formicary-ai/<workspace>`, but that directory only exists inside the `setup` pod — subsequent tasks (`plan`, `implement`, etc.) cannot access it without a shared volume.
>
> **Production requirement:** Mount a `ReadWriteMany` PersistentVolumeClaim (NFS, EFS, or similar) on every task so pods in the same job share the workspace. SHELL mode uses the host filesystem naturally.

To use a shared PVC across all tasks in a job, add a `volumes` block to each task's `container`:

```yaml
# Shared PVC definition (created once by the cluster admin)
# kubectl create -f - <<EOF
# apiVersion: v1
# kind: PersistentVolumeClaim
# metadata:
#   name: formicary-ai-workspace
# spec:
#   accessModes: [ReadWriteMany]
#   resources:
#     requests:
#       storage: 10Gi
# EOF

- task_type: setup
  method: KUBERNETES
  container:
    image: ghcr.io/formicary-ai/agent-worker:latest
    volumes:
      host_paths:
        - name: ai-workspace
          host_path: /tmp/formicary-ai          # shared NFS mount, NOT local node path
          mount_path: /tmp/formicary-ai

- task_type: plan
  method: KUBERNETES
  container:
    image: ghcr.io/formicary-ai/agent-worker:latest
    volumes:
      host_paths:
        - name: ai-workspace
          host_path: /tmp/formicary-ai
          mount_path: /tmp/formicary-ai
```

Alternatively, use SHELL executor for development (no shared volume needed) and switch to KUBERNETES for production once shared storage is provisioned.

Start the ant worker with KUBERNETES method enabled:

```bash
formicary-ant \
  --server-url http://localhost:7777 \
  --tags "ai-worker" \
  --methods "KUBERNETES" \
  --kubernetes-namespace "formicary-ai"
```

### Kubernetes Secret Injection (Industry Best Practice)

Formicary supports three patterns for injecting secrets into AI agent pods. All patterns keep secret values out of Formicary's database and task logs — the kubelet resolves them at pod start time.

#### Pattern 1: Individual key reference (`env_value_from`)

Inject a single named key from a Secret or ConfigMap as a container env var. Equivalent to K8s `env[].valueFrom.secretKeyRef`.

```yaml
- task_type: plan
  method: KUBERNETES
  container:
    image: ghcr.io/formicary-ai/agent-worker:latest
    env_value_from:
      - name: ANTHROPIC_API_KEY
        secret_name: ai-agent-secrets   # K8s Secret name
        key: anthropic-api-key           # Key within the Secret
      - name: GITHUB_TOKEN
        secret_name: ai-agent-secrets
        key: github-token
      - name: MODEL_NAME
        config_map_name: ai-agent-config  # K8s ConfigMap name
        key: default-model
```

**Create the K8s Secret once:**
```bash
kubectl create secret generic ai-agent-secrets \
  --from-literal=anthropic-api-key=sk-ant-... \
  --from-literal=github-token=ghp_... \
  --namespace=formicary-ai
```

#### Pattern 2: Bulk load all keys (`env_from`)

Load every key from a Secret or ConfigMap as environment variables. Equivalent to K8s `envFrom`. Use this when you want all keys available without listing them individually.

```yaml
container:
  image: ghcr.io/formicary-ai/agent-worker:latest
  env_from:
    - secret_ref: ai-agent-secrets    # loads ALL keys as env vars
    - config_map_ref: ai-agent-config
    - secret_ref: github-secrets
      prefix: GH_                      # optional prefix on each key
```

#### Pattern 3: Formicary encrypted org configs (cross-platform)

For mixed SHELL/KUBERNETES deployments or non-K8s environments, store secrets as encrypted Formicary org configs. Values are never written to task logs.

```bash
curl -X POST http://localhost:7777/api/orgs/{org}/configs \
  -d '{"name":"AnthropicApiKey","value":"sk-ant-...","secret":true}'
```

Reference in job YAML with `{{.AnthropicApiKey}}`. The value is injected at job dispatch time and redacted from all logs.

**Comparison:**

| Pattern | Mechanism | When to use |
|---------|-----------|-------------|
| `env_value_from` | K8s `secretKeyRef` / `configMapKeyRef` | Production K8s; individual keys; IRSA pattern |
| `env_from` | K8s `envFrom` | Production K8s; bulk load; avoid repetition |
| Formicary org configs | DB-encrypted template vars | Multi-platform; SHELL executor; non-K8s envs |

#### Per-task Service Account (IRSA / Workload Identity)

For AWS IRSA or GCP Workload Identity, assign a different IAM-annotated service account per task without modifying the ant worker config:

```yaml
container:
  image: ghcr.io/formicary-ai/agent-worker:latest
  service_account: ai-agent-irsa-sa   # overrides ant-worker default
  env_value_from:
    - name: ANTHROPIC_API_KEY
      secret_name: ai-agent-secrets
      key: anthropic-api-key
```

The ant worker's `kubernetes.service_account` config remains the fallback for tasks that don't specify one.

### Worker Image Requirements

The container image must include:

| Tool | Purpose |
|------|---------|
| `bash`, `jq`, `curl` | Script execution and JSON parsing |
| `git` | Repository operations |
| `gh` (GitHub CLI) | Issue/PR management |
| `claude` (Claude Code CLI) | AI code generation |
| `xxd` | Nonce generation for branch names |
| `you-got-skills` (optional) | Skill slash commands for Claude |

### Building the Worker Image

The full Dockerfile is at [`docs/examples/agent-worker/Dockerfile`](examples/agent-worker/Dockerfile).

Build and push:

```bash
cd docs/examples/agent-worker
docker build -t ghcr.io/formicary-ai/agent-worker:latest .
docker push ghcr.io/formicary-ai/agent-worker:latest
```

### Why KUBERNETES for Production?

| Concern | SHELL | KUBERNETES |
|---------|-------|-----------|
| Isolation | Shared ant worker filesystem | Fresh pod per task |
| Cleanup | Manual (`rm -rf`) | Pod deleted on completion |
| Scaling | One task at a time per worker | Pod-per-task, any node |
| Tool versions | Depends on host | Pinned in image |
| Secrets | Env vars on host | K8s secrets or Formicary configs |

The scripts work identically in both modes. SHELL is recommended for local development; KUBERNETES is recommended for production.

---

## Diagnostics and Artifacts

Every task uploads artifacts regardless of success or failure (`when: always`):

| Artifact | Produced by | Contains |
|----------|-------------|----------|
| `picker_{{JobID}}.log` | issue-picker | Capacity check, submission log |
| `meta.env` | setup | Workspace path, branch name |
| `branch.txt` | setup | Markdown branch hyperlink |
| `plan_result.json` | plan | Status, complexity, task count, summary |
| `plan_summary.md` | plan | Full PLAN.md from Claude |
| `impl_result.json` | implement | Status, files changed, commits, test status |
| `fix_result.json` | fix-tests | Fixed/still-failing tests, root cause |
| `review_result.json` | review | Status, critical findings, fixes applied |
| `pr_url.txt` | create-pr | PR URL |
| `notification.log` | notify-blocked | Reason + issue comment confirmation |
| `cleanup_{{JobID}}.log` | cleanup | Workspace deletion confirmation |
| `workspace_logs_{{JobID}}.tar.gz` | cleanup | All logs from workspace (for post-mortem) |

Access artifacts via the Formicary dashboard or API:

```bash
curl http://localhost:7777/api/artifacts?job_id=<id> -H 'Authorization: Bearer <token>'
```

---

## Configuration Reference

### `ai-gh-implement` job_variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PlanModel` | `opus` | Claude model for planning phase |
| `ImplementModel` | `sonnet` | Claude model for implementation |
| `ReviewModel` | `opus` | Claude model for self-review |
| `FormicaryURL` | `http://localhost:7777` | Formicary queen URL |

### `ai-gh-issue-picker` job_variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MaxPendingJobs` | `10` | Max concurrent ai-gh-implement jobs (script check) |
| `PickupLabel` | `ai-ready` | GitHub label to pick up |
| `InProgressLabel` | `ai-in-progress` | GitHub label to apply on pickup |
| `FormicaryURL` | `http://localhost:7777` | Formicary queen URL |

Note: The `skip_if` expression hard-codes `10` as the fast-path guard. If you raise `MaxPendingJobs` above 10, also update the `skip_if` threshold in the job definition.

---

## Adapting to Other Issue Trackers

To add Jira support, copy the YAML files and rename:

```bash
cp ai-gh-issue-picker.yaml ai-jira-issue-picker.yaml
cp ai-gh-implement.yaml ai-jira-implement.yaml
```

Then update:
- `job_type`: `ai-jira-*`
- `pick-issues` task: use Jira REST API instead of `gh issue list`
- `create-pr` task: use Bitbucket/GitLab API instead of `gh pr create`
- `notify-blocked` task: comment on Jira issue instead of GitHub

The implementation phases (plan, implement, fix-tests, review) are tracker-agnostic and can be shared.

---

## Extending the Workflow

### Adding a Security Review Phase

Insert between `review` and `create-pr`:

```yaml
- task_type: security-review
  method: KUBERNETES
  timeout: 15m
  container:
    image: ghcr.io/formicary-ai/agent-worker:latest
  dependencies:
    - setup
    - review
  environment:
    ANTHROPIC_API_KEY: "{{.AnthropicApiKey}}"
  script:
    - |
      #!/bin/bash
      source meta.env
      cd "$WS/repo"
      claude --print --model "{{.ReviewModel}}" --max-turns 20 \
        --output-format json \
        "Run ygs-security-review on the changes in this branch.
         Check OWASP top 10, auth, injection, secrets exposure.
         Fix any HIGH severity issues. Output:
         {\"status\":\"CLEAN|FIXED\",\"high_severity\":N,\"fixes\":N}" \
        | tee security_result.json
  artifacts:
    paths:
      - security_result.json
    when: always
  on_completed: create-pr
```

---

## Comparison with Imperative Orchestrators

| Dimension | Imperative Bot | Formicary AI Agents |
|-----------|----------------|---------------------|
| **Code to maintain** | ~15K LOC | 4 YAML files (~500 lines) |
| **State management** | Complex state machine | Formicary task DAG |
| **Per-phase timeouts** | ❌ None | ✅ Per-task `timeout` |
| **Capacity control** | ❌ Silent drop when full | ✅ `skip_if` + `max_concurrency` |
| **Adding a phase** | Code change + deploy | Add task block to YAML |
| **Local development** | ❌ K8s required | ✅ `SHELL` executor, laptop |
| **Restart safety** | ❌ Complex state recovery | ✅ Nonce → fresh branch |
| **Context between phases** | ❌ Lost (fresh pod/session) | ✅ Artifacts + shared workspace (RWX PVC required for K8s; SHELL uses host fs) |
| **Diagnostics** | ❌ Text logs only | ✅ Structured JSON artifacts per task |
| **Dashboard visibility** | ❌ None | ✅ Job description with markdown links |
| **Multi-tracker** | ❌ Hardcoded | ✅ Clone YAML, change API calls |
| **Secrets** | K8s secrets only | DB-encrypted org configs (cross-platform) |
