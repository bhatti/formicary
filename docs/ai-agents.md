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

### 1. Formicary Running in Kubernetes

Deploy the Formicary queen using the deploy script — it creates the required secrets and applies the manifest:

```bash
# Set credentials in env (never as CLI flags)
export COMMON_AUTH_JWT_SECRET="my-strong-secret"
export COMMON_AUTH_GOOGLE_CLIENT_ID="<id>.apps.googleusercontent.com"
export COMMON_AUTH_GOOGLE_CLIENT_SECRET="<secret>"
export COMMON_AUTH_GOOGLE_CALLBACK_HOST="your-host.example.com"
# Optional Slack (omit to disable):
export SLACK_APP_TOKEN="xapp-..."
export SLACK_BOT_TOKEN="xoxb-..."

# Local Docker Desktop k8s:
./scripts/deploy-formicary.sh

# EC2 k3s (kubectl tunneled over SSH):
./scripts/deploy-formicary.sh --queen-ip <QUEEN_IP>
```

All AI workflow YAMLs use `method: KUBERNETES` and `image: plexobject/ai-dev-tools:latest`. Each task runs in a fresh pod that is deleted on completion.

#### Alternative: SHELL Execution (Local Development Only)

For local development without Kubernetes, change every task's `method: KUBERNETES` to `method: SHELL` and remove the `container:` blocks. The scripts are identical. You need these tools on the host:

```bash
npm install -g @anthropic-ai/claude-code
gh auth login
```

SHELL mode is not recommended for production — tasks share the host filesystem and do not have isolated environments.

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

### 3. Deploy Workflows and Configure Secrets

Use the deploy scripts — they create the Kubernetes secret, push org configs, and upload all workflow YAMLs in one step:

```bash
cd docs/examples

export FORMICARY_TOKEN="<jwt-from-dashboard>"
export GH_TOKEN="<github-pat>"          # needs repo + issues + pull_requests scopes
export SSH_PRIVATE_KEY="$(cat ~/.ssh/id_rsa)"

# GitHub workflows — creates K8s secret, sets org configs, uploads YAMLs
./deploy-ai-workflows.sh \
  --create-k8s-secret \
  --set-configs \
  --gh-org YOUR_ORG \
  --gh-repo YOUR_REPO \
  --bedrock                              # omit if using ANTHROPIC_API_KEY directly
# With auth enabled, org-based routing is automatic — no --ant-user-tag flag needed.

# Jira/Bitbucket workflows
export JIRA_API_TOKEN="<token>"          # or reads from ~/.config/acli/config.json
export BITBUCKET_TOKEN="<app-password>"

./deploy-ai-jira-workflows.sh \
  --create-k8s-secret \
  --set-configs \
  --bb-workspace YOUR_WORKSPACE \
  --bb-repo YOUR_REPO \
  --bedrock
```

Org configs (project names, Slack channel) are stored encrypted in the Formicary database and accessible in job YAML as `{{.GitHubOrg}}`, `{{.JiraUrl}}`, etc. Credentials (tokens, SSH keys) are stored in the `ai-dev-credentials` Kubernetes secret and injected directly into pods — never written to the Formicary database.

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

All job definitions use `method: KUBERNETES` and run as fresh pods on the same cluster that hosts Formicary. The embedded ant worker (included in `k8s.yaml`) spawns and manages the pods — no separate ant worker process is needed.

> **Workspace isolation:** Each task pod starts with an empty filesystem. The current AI workflow YAMLs use `empty_dir` volumes at `/workspace` — each task starts fresh and consumes artifacts from upstream tasks via Formicary's artifact store. This works well for stateless tasks (issue-picker, standup gather/synthesize/post).
>
> For multi-task workflows that need a shared git checkout across tasks (plan → implement → create-pr), mount a `ReadWriteMany` PVC so all pods in a job share the same directory:

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

### KUBERNETES vs SHELL

| Concern | KUBERNETES (default) | SHELL (dev only) |
|---------|----------------------|------------------|
| Isolation | Fresh pod per task | Shared host filesystem |
| Cleanup | Pod deleted on completion | Manual (`rm -rf`) |
| Scaling | Pod-per-task, any node | One task at a time per worker |
| Tool versions | Pinned in image | Depends on host |
| Secrets | K8s secrets or Formicary configs | Env vars on host |
| Setup | `kubectl apply -f k8s.yaml` | Tools installed on host |

KUBERNETES is the default and recommended mode. SHELL is useful for local development without a cluster.

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

## Slack Integration

Formicary includes a built-in Slack Socket Mode router. When `SLACK_APP_TOKEN` is set, the queen process listens for bot mentions and routes commands directly to Formicary jobs — no separate service required.

### Architecture

```
Slack (Socket Mode)
      │  @bot <command> [entity]
      ▼
┌─────────────────────────────────────────────┐
│  Formicary queen — SlackService             │
│  - Strips mention prefix                    │
│  - Looks up registered user → OrganizationID│
│  - CommandRouter: verb → job_type + params  │
│  - Submits job via JobManager               │
│  - Replies in thread with job link          │
└──────────────────┬──────────────────────────┘
                   │ job params (SlackChannel,
                   │  SlackThreadTs, IdVar, Params…)
                   ▼
         Formicary job container
         (ai-dev-tools + you-got-skills)
                   │
                   ▼
         Slack thread reply (via SlackChannel/SlackThreadTs)
```

**Layers:**
- **Formicary** — generic job orchestration, routing table, user registry, secrets. No AI-specific logic.
- **ai-dev-tools** — CLI layer that runs inside job containers; talks to Claude, GitHub, Jira APIs.
- **you-got-skills** — skill library (SKILL.md files) invoked by ai-dev-tools (`Skill` param).
- **Formicary workflow YAMLs** — job definitions in `docs/examples/`; declare params, tasks, cron schedule.

### Route Configuration

Routes live in `queen.slack.routes` in the queen config (or `k8s/formicary-leader.yaml`). Each route is generic — Formicary does not interpret `params`; it passes them verbatim to the job container:

```yaml
slack:
  routes:
    - triggers: ["standup", "status"]
      job_type: ai-standup-jira
      description: "Daily standup from Jira"

    - triggers: ["review"]
      job_type: ai-gh-review
      id_var: PRUrl          # trailing text bound to this param
      description: "Review a PR"

    - triggers: ["risk", "risk scan"]
      job_type: ai-adhoc
      params:                # merged verbatim — job container decides what to do with them
        Skill: ygs-risk-scan
      description: "Scan sprint for risks"

    - triggers: ["adhoc"]
      job_type: ai-adhoc
      id_var: Prompt         # trailing text → Prompt param
      description: "Run any ad-hoc prompt"
```

**How `id_var` works:** when a user types `@bot review https://github.com/org/repo/pull/42`, the router sets `PRUrl=https://github.com/org/repo/pull/42`. The job container (ai-dev-tools) uses `$PRUrl` to fetch the PR. Formicary never inspects the value.

**How `params` works:** key/value pairs merged into every job submitted by this route. Useful for fixed values like skill names, modes, or feature flags that the job container reads as env vars.

### Multi-Tenant Isolation

Every Slack command is fully scoped to the individual user's org — no shared token, no cross-tenant leakage.

**End-to-end flow:**

```
@mention arrives (Slack user ID: U0A1HQL0C9J)
        │
        ▼
LookupBySlackID("U0A1HQL0C9J")
  → queries user_configs (admin read-only lookup)
  → returns User{ID: "u123", OrganizationID: "org456"}
        │
        ▼
QueryContext = NewQueryContextFromIDs("u123", "org456")
SaveJobRequest(qc, req)
  → request.UserID = "u123"            ← set from QC, not from params
  → request.OrganizationID = "org456"  ← set from QC, cannot be spoofed
        │
        ▼
Ant scheduler: Reserve(method, tags, orgID="org456")
  → prefers ants registered with orgID="org456" (user's own ant worker)
  → falls back to unscoped ants (OrgID="") if no personal ant is online
```

Key properties:
- **Job auth is set by the server** — `SaveJobRequest` overwrites `UserID`/`OrganizationID` from the `QueryContext` derived from the registered user's token, not from any user-supplied job param.
- **Unregistered users are blocked** — if `LookupBySlackID` returns nil, the bot replies with the registration prompt and no job is submitted.
- **Credentials stored encrypted** — the user's Formicary API token is stored using AES-256-GCM in `user_configs`, same infrastructure used for all org secrets. The DM is deleted after registration.
- **Ant routing is automatic** — when a user connects their ant worker with `setup-ant-worker.sh --token <their-token>`, the queen records `org_id` from the JWT. Jobs submitted via that user's Slack commands prefer their org's ant. If their ant is offline, an unscoped embedded ant (e.g., the one in `formicary-all-in-one.yaml`) handles the job.

**What `UserTag` and `DefaultTracker` do:**
Two job params added by the dispatcher are informational, not auth-related:
- `UserTag=username` — passed to ai-dev-tools containers as `$UserTag`; used for attribution in Slack replies and PR descriptions.
- `DefaultTracker=jira|github` — read from the user's org config (set by `deploy-ai-jira-workflows.sh --set-configs`); tells the ai-dev-tools Python router which tracker to use when the Slack message contains no URL (`@bot standup` → jira or github). This is resolved inside the job container, not by Formicary itself.

Set `DefaultTracker` via the deploy script:
```bash
./deploy-ai-jira-workflows.sh --set-configs --jira-project MYPROJ  # sets DefaultTracker=jira
./deploy-ai-workflows.sh --set-configs --gh-org ORG --gh-repo REPO  # sets DefaultTracker=github
```

### User Registration

Users DM the bot once to register their Formicary token:

```
DM: setup <api-token-from-formicary>
```

Get the token at `https://<formicary-url>/dashboard/users/tokens`. Once registered:
1. The queen validates the JWT inline (no HTTP round-trip)
2. `slack_user_id` and `slack_api_token` are stored in `user_configs` (encrypted)
3. The DM is deleted so the token doesn't persist in chat
4. All subsequent `@bot` commands from this Slack user are scoped to their org automatically

### Supported Commands (default routes)

| Command | Job type | Notes |
|---------|----------|-------|
| `@bot standup` | `ai-standup-jira` | Also: `status`, `daily` |
| `@bot standup gh` | `ai-standup-gh` | GitHub standup |
| `@bot review <pr-url>` | `ai-gh-review` | AI code review posted to PR |
| `@bot security review <pr-url>` | `ai-gh-review` | Security-focused review |
| `@bot sre review <pr-url>` | `ai-gh-review` | SRE/reliability review |
| `@bot implement <issue>` | `ai-jira-implement` | Jira issue number |
| `@bot implement gh <issue>` | `ai-gh-implement` | GitHub issue number |
| `@bot risk` | `ai-adhoc` | Sprint risk scan |
| `@bot prs` | `ai-adhoc` | Open PR queue |
| `@bot pr feedback <pr-url>` | `ai-adhoc` | PR comments summary |
| `@bot jira query <text>` | `ai-jira-query` | Search Jira |
| `@bot doctor` | `ai-connectivity-check` | Credential check |
| `@bot adhoc <prompt>` | `ai-adhoc` | Any free-form prompt |
| `@bot help` | _(builtin)_ | List available commands |

### Testing Without Slack

Trigger any command via the Formicary API directly — identical to a Slack submission but with no bot token required:

```bash
export FORMICARY_URL=https://YOUR_QUEEN_IP.nip.io
export T=$FORMICARY_TOKEN

# standup
curl -sk -X POST "$FORMICARY_URL/api/jobs/requests" \
  -H "Authorization: Bearer $T" -H "Content-Type: application/json" \
  -d '{"job_type":"ai-standup-jira","params":{"SlackChannel":"","SlackThreadTs":""}}'

# review a PR
curl -sk -X POST "$FORMICARY_URL/api/jobs/requests" \
  -H "Authorization: Bearer $T" -H "Content-Type: application/json" \
  -d '{"job_type":"ai-gh-review","params":{"PRUrl":"https://github.com/ORG/REPO/pull/42"}}'

# implement a Jira issue
curl -sk -X POST "$FORMICARY_URL/api/jobs/requests" \
  -H "Authorization: Bearer $T" -H "Content-Type: application/json" \
  -d '{"job_type":"ai-jira-implement","params":{"IssueNumber":"PROJ-123"}}'

# risk scan (Skill param consumed by ai-dev-tools container)
curl -sk -X POST "$FORMICARY_URL/api/jobs/requests" \
  -H "Authorization: Bearer $T" -H "Content-Type: application/json" \
  -d '{"job_type":"ai-adhoc","params":{"Skill":"ygs-risk-scan"}}'

# open PR queue
curl -sk -X POST "$FORMICARY_URL/api/jobs/requests" \
  -H "Authorization: Bearer $T" -H "Content-Type: application/json" \
  -d '{"job_type":"ai-adhoc","params":{"Skill":"ygs-pr-queue"}}'

# ad-hoc prompt
curl -sk -X POST "$FORMICARY_URL/api/jobs/requests" \
  -H "Authorization: Bearer $T" -H "Content-Type: application/json" \
  -d '{"job_type":"ai-adhoc","params":{"Prompt":"summarize open blockers this sprint"}}'

# doctor — connectivity check
curl -sk -X POST "$FORMICARY_URL/api/jobs/requests" \
  -H "Authorization: Bearer $T" -H "Content-Type: application/json" \
  -d '{"job_type":"ai-connectivity-check","params":{}}'
```

Watch job output at `$FORMICARY_URL/dashboard/jobs/requests/<id>`. When `SlackChannel` is empty the result appears in the job artifacts only — set it to a real channel ID to have the bot post back.

### Adding a New Command

No code changes needed — add a route entry to `queen.slack.routes` in the config and redeploy:

```yaml
- triggers: ["deploy", "release"]
  job_type: my-deploy-job
  id_var: Environment      # e.g. "@bot deploy staging" → Environment=staging
  params:
    Region: us-west-2      # fixed param passed to every invocation
  description: "Deploy to an environment"
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

---

---

## Task Execution Context — Capturing Debug Info

Scripts can write structured key/value pairs that appear in the Formicary dashboard under each task's **Execution Context** view. Two mechanisms are available:

### Stdout markers (runtime, from scripts)

Print these lines to stdout during task execution:

```bash
# Writes to this task's execution context (visible under the task in the dashboard)
echo "::add-task-context SELECTED_MODEL::claude-3-5-sonnet"
echo "::add-task-context ISSUE_COUNT::42"
echo "::add-task-context SELECTED_TRACKER::jira"

# Writes to the job execution context (shared across all tasks in the job)
echo "::add-job-context PR_URL::https://github.com/org/repo/pull/1"
echo "::add-job-context ISSUE_ID::PROJ-123"
```

**Format:** `KEY::VALUE` — key must be non-empty; value may be empty or contain `::`.

All ai-dev-tools scripts emit `::add-task-context` markers at exit with at minimum:
- `SELECTED_MODEL` — the Claude model used (resolved from `AI_MODEL` env)
- `SELECTED_TRACKER` — `jira` or `github`
- `ISSUE_COUNT`, `PR_COUNT`, `FINDINGS_COUNT`, etc. — counts of items processed

### `context_variables` YAML field (static, rendered by the queen)

Declare values directly in the YAML task definition. They are rendered through Go templates before the task runs and appear in the task context automatically — no script change needed.

```yaml
tasks:
  - task_type: synthesize
    context_variables:
      - name: SELECTED_MODEL
        value: "{{.AnthropicSonnetModel}}"
      - name: SELECTED_TRACKER
        value: jira
      - name: MAX_TURNS
        value: "{{.MaxTurnsStandup}}"
```

`context_variables` is rendered server-side; stdout markers are emitted client-side. Use `context_variables` for job params and model labels; use stdout markers for runtime values (counts, verdicts, etc.).

---

## See Also

- [Ant worker setup](ant-worker-setup.md) — connect your laptop as a personal ant worker, register with the bot
- [Examples README](examples/README.md) — full Slack integration setup, supported commands, deploy scripts
- [Configuration Reference](15-configuration.md) — queen config including Slack routes
