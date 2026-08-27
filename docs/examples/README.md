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

The implement pipeline includes two enhancements:
- **Self-review**: after `implement`, Claude runs a self-review pass (`/ygs-review-pr --mode self-review`). If it returns BLOCKED (exit 2), the job pauses before creating the PR.
- **Complexity-tiered models**: `plan` writes `plan_complexity.txt` (low/medium/high); `implement` picks Haiku/Sonnet/Opus automatically.

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
# With auth enabled, org-based routing is automatic — no --ant-user-tag flag needed.

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

The implement pipeline includes two enhancements:
- **Self-review**: after `implement`, Claude runs a self-review pass (`/ygs-review-pr --mode self-review`). If it returns BLOCKED (exit 2), the job pauses before creating the PR.
- **Complexity-tiered models**: `plan` writes `plan_complexity.txt` (low/medium/high); `implement` picks Haiku/Sonnet/Opus automatically.

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
# With auth enabled, org-based routing is automatic — no --ant-user-tag flag needed.

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

### PR Review (`ai-gh-review` / `ai-jira-review`)

Claude reviews a PR across four domains (correctness, security, API surface, SRE), posts findings as a Slack Block Kit message with three buttons, then pauses:

- **✅ Approve** — posts `gh pr review --approve` to the PR immediately
- **🔄 Request Changes** — posts a draft review to Slack; reply `post it` to publish as-is, or reply with your edited text; Claude posts an inline file:line review to the PR (`ai-bot` prefix so `poll_pr.py` picks up replies)
- **🔍 Verify** — Claude re-reads each flagged file to remove false positives, then re-posts the Block Kit for a fresh decision

**Deploy (GitHub):**
```bash
source ~/.zshrc
cd ~/workplace/formicary/docs/examples

./deploy-ai-workflows.sh \
  --create-k8s-secret --set-configs \
  --gh-org "$GH_ORG" --gh-repo "$GH_REPO" \
  --slack-channel "$SLACK_CHANNEL" --bedrock
```

**Deploy (Jira/Bitbucket):**
```bash
./deploy-ai-jira-workflows.sh \
  --create-k8s-secret --set-configs \
  --slack-channel "$SLACK_CHANNEL" --bedrock
```

**Trigger via curl:**
```bash
curl -s -X POST "$FORMICARY_URL/api/jobs/requests" \
  -H "Authorization: Bearer $FORMICARY_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"job_type\":\"ai-gh-review\",\"params\":{\"PRUrl\":\"https://github.com/$GH_ORG/$GH_REPO/pull/1\",\"SlackChannel\":\"$SLACK_CHANNEL\"}}"
```

**Trigger via Slack** (Slack integration built into the queen — see [Slack Integration](#slack-integration-built-into-the-queen--no-separate-pod) below):
```
@ai-agent review https://github.com/ORG/REPO/pull/1
```

**Task Context Variables** (visible on the job request dashboard after completion):

| Key | Description |
|-----|-------------|
| `SKILL` | Skill name used for the review (e.g. `ygs-review-pr`, `ygs-review-deep`) |
| `SKILL_LOADED` | `yes` if the skill's `SKILL.md` was found; `no` if fallback prompt used |
| `YGS_SKILLS_COUNT` | Number of you-got-skills skills installed in the pod |
| `YGS_SKILLS_INSTALLED` | Comma-separated list of installed skill names |
| `YGS_SKILLS_REPO_COMMIT` | Short commit hash of the cloned you-got-skills repo |
| `SKILLS_INVOKED` | Skills Claude actually called during the review; `none` if none detected |
| `FINDINGS_COUNT` | Number of findings in the review output |
| `REVIEW_VERDICT` | Final verdict (`APPROVED`, `CONCERNS`, `REJECTED`) |
| `SELECTED_MODEL` | Model ID used for the Claude invocation |

---

### Ad-hoc Skill (`ai-adhoc`)

Run any [you-got-skills](https://github.com/bhatti/you-got-skills) skill with a free-form prompt and get the result posted to a Slack thread.

```bash
curl -s -X POST "$FORMICARY_URL/api/jobs/requests" \
  -H "Authorization: Bearer $FORMICARY_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"job_type\":\"ai-adhoc\",\"params\":{\"Skill\":\"ygs-standup\",\"Prompt\":\"summarize open PRs\",\"SlackChannel\":\"$SLACK_CHANNEL\"}}"
```

Via Slack:
```
@ai-agent standup
@ai-agent risk
```

Deployed automatically by both `deploy-ai-workflows.sh` and `deploy-ai-jira-workflows.sh`.

---

### Slack Integration (built into the queen — no separate pod)

Slack Socket Mode is built directly into the Formicary queen. There is no separate router pod or Python process — the queen listens on an outbound WebSocket and dispatches @mentions as jobs.

**How it works:**
1. You @mention the bot in a Slack channel
2. The queen maps the verb to a job type and extracts any trailing text as a `Prompt` param
3. A job container (ai-dev-tools) runs the AI logic using you-got-skills formatting
4. The output is posted back to the same Slack thread

Formicary is a thin passthrough — it never builds prompts, invokes skills, or formats output. All AI logic lives in the job container.

---

#### One-time Slack app setup

See [full guide with screenshots](https://github.com/bhatti/ai-dev-tools/blob/main/docs/slack-setup.md). Quick summary:

1. **Socket Mode** → enable → **Generate an app-level token** → add `connections:write` scope → copy `xapp-...` → `SLACK_APP_TOKEN`
2. **OAuth & Permissions → Bot Token Scopes** → add exactly these 9 scopes:

   | Scope | Purpose |
   |-------|---------|
   | `app_mentions:read` | Receive @bot mentions in channels |
   | `channels:history` | Read public channel messages |
   | `channels:read` | List public channels |
   | `chat:write` | Post messages and replies |
   | `groups:history` | Read private channel messages |
   | `groups:read` | List private channels |
   | `im:history` | Read DMs (for `setup <token>` registration) |
   | `im:write` | Reply to DMs (required to confirm registration) |
   | `users:read` | Look up user info |

3. **Event Subscriptions** → enable → **Subscribe to bot events** → add both:
   - `app_mention` — @bot mentions in channels
   - `message.im` — DMs for `setup <token>` registration
   → Save Changes

4. **Interactivity & Shortcuts** → enable (required for Block Kit Approve/Request Changes buttons)
5. **Install App** → Install to Workspace → copy `xoxb-...` → `SLACK_BOT_TOKEN`
   > If your workspace requires admin approval, click **Request to install** and have the workspace admin approve at: Slack → Settings & administration → Manage apps → Requests
6. In Slack: `/invite @<your-bot-name>`

To find your bot's name: `curl -s "https://slack.com/api/auth.test" -H "Authorization: Bearer $SLACK_BOT_TOKEN" | python3 -c "import json,sys; print('@'+json.load(sys.stdin)['user'])"`

---

#### Admin deploy (one-time per cluster)

```bash
source ~/.zshrc   # loads SLACK_BOT_TOKEN, SLACK_APP_TOKEN, SLACK_SIGNING_SECRET, etc.

# Local Docker Desktop k8s:
./scripts/deploy-formicary.sh

# EC2 k3s (kubectl tunneled over SSH — port 6443 not open externally):
./scripts/deploy-formicary.sh --queen-ip YOUR_QUEEN_IP
./scripts/deploy-formicary.sh --queen-ip YOUR_QUEEN_IP --queen-ssh-key ~/path/to/key.pem

# All-in-one (leader + embedded ant, good for single-node):
./scripts/deploy-formicary.sh --all-in-one
```

Reads from environment (never CLI flags):

```
COMMON_AUTH_JWT_SECRET            (required)
COMMON_AUTH_GOOGLE_CLIENT_ID      (optional)
COMMON_AUTH_GOOGLE_CLIENT_SECRET  (optional)
SLACK_BOT_TOKEN                   (optional — omit to disable Slack)
SLACK_APP_TOKEN                   (optional — omit to disable Slack)
SLACK_SIGNING_SECRET              (optional)
```

**Verify the queen connected:**
```bash
# Local:
kubectl logs -l app=formicary --tail=30 | grep -i slack

# EC2:
./scripts/deploy-formicary.sh --queen-ip YOUR_QUEEN_IP --logs
# Expected: "Slack Socket Mode starting" then "connected to Slack"
```

---

#### Per-user registration (each developer, one-time)

Each developer registers their personal Formicary API token with the bot so their jobs run on their ant worker under their credentials:

```bash
# Step 1: connect your local Kubernetes cluster as an ant worker
source ~/.zshrc   # loads FORMICARY_TOKEN, etc.
export QUEEN_HOST="<QUEEN_IP_OR_HOSTNAME>"

./scripts/setup-ant-worker.sh \
  --queen "$QUEEN_HOST" \
  --token "$FORMICARY_TOKEN"
# Your ant registers with pod_label user=$USER — jobs route to your machine, not teammates'

# Step 2: upload your workflow YAMLs and set your org configs
./scripts/setup-user-creds.sh                   # auto-detect tracker from env
./scripts/setup-user-creds.sh --tracker jira    # Jira + Bitbucket
./scripts/setup-user-creds.sh --tracker github  # GitHub
# setup-user-creds.sh calls deploy-ai-workflows.sh (or deploy-ai-jira-workflows.sh)
# with --create-k8s-secret --set-configs (org routing is automatic when auth is enabled)

# Step 3: register your token with the bot in Slack
# Open a DM to your bot and type:
#   setup <your-formicary-token>
# The bot confirms registration and deletes your DM so the token is not stored in chat.
# Get your token at: https://<formicary-host>/dashboard/users/tokens
```

**Invite the bot to your channel — mandatory:**
```
/invite @<your-bot-name>
```

---

#### Supported commands

Mention the bot in any channel it has been invited to:

| Command | What it does | Workflow |
|---------|-------------|---------|
| `@bot help` | List all available commands | — |
| `@bot standup` | Compact daily brief: board status, per-person status, risks, discussion | `ai-standup-jira` / `ai-standup-gh` |
| `@bot status` | Same as standup | `ai-standup-jira` |
| `@bot risk` / `@bot risks` | Ranked sprint risks: stale work, PR bottlenecks, dependency chains | `ai-adhoc` |
| `@bot prs` | Open PRs grouped by author vs reviewer, sorted by age | `ai-adhoc` |
| `@bot open prs` | Same as prs | `ai-adhoc` |
| `@bot review queue` | Same as prs | `ai-adhoc` |
| `@bot pr comments <url>` | All comments, inline feedback, and open tasks for a PR | `ai-adhoc` |
| `@bot review <github-pr-url>` | Full PR review: correctness, security, API, SRE | `ai-gh-review` |
| `@bot review <bitbucket-pr-url>` | Same for Bitbucket | `ai-jira-review` |
| `@bot security review <pr-url>` | OWASP-style security audit | `ai-gh-review` |
| `@bot sre review <pr-url>` | Failure mode and operational risk review | `ai-gh-review` |
| `@bot implement PROJ-123` | Implement a Jira issue end-to-end | `ai-jira-implement` |
| `@bot implement 42` | Implement a GitHub issue end-to-end | `ai-gh-implement` |
| `@bot jira query <keywords>` | Search Jira issues by keyword | `ai-jira-query` |
| `@bot search jira <keywords>` | Same as jira query | `ai-jira-query` |
| `@bot jira-analyze PROJ-1,PROJ-2` | Analyze and summarize a set of Jira issues | `ai-jira-query` (Mode=analyze) |
| `@bot gh-query <keywords>` | Search GitHub issues by keyword | `ai-jira-query` (DefaultTracker=github) |
| `@bot gh-analyze #123,#456` | Analyze GitHub issues for root cause and fixes | `ai-jira-query` (Mode=analyze, DefaultTracker=github) |
| `@bot doctor` | Connectivity check against all configured services | `ai-connectivity-check` |
| `@bot adhoc <free-form prompt>` | Run any you-got-skills skill with a free-form prompt | `ai-adhoc` |

Replace `@bot` with your bot's actual name (find it with `curl -s https://slack.com/api/auth.test -H "Authorization: Bearer $SLACK_BOT_TOKEN" | python3 -m json.tool | grep '"user"'`).

Thread replies on a paused job resume it — no button required.

> **If @mention does nothing**, run this diagnostic:
> ```bash
> # 1. Confirm bot identity and that the token is valid
> curl -s https://slack.com/api/auth.test \
>   -H "Authorization: Bearer $SLACK_BOT_TOKEN" | python3 -m json.tool
>
> # 2. Confirm bot is in the channel — empty output = not invited
> curl -s "https://slack.com/api/conversations.list?types=public_channel,private_channel&limit=200" \
>   -H "Authorization: Bearer $SLACK_BOT_TOKEN" | python3 -c "
> import json,sys
> data=json.load(sys.stdin)
> for c in data.get('channels',[]):
>     if c.get('is_member'): print(c['name'], c['id'])
> "
> ```
> Required scopes: `app_mentions:read`, `channels:history`, `channels:read`, `chat:write`,
> `groups:history`, `groups:read`, `im:history`, `im:write`, `users:read`
>
> Required events: `app_mention`, `message.im`
>
> If any are missing: add them in OAuth & Permissions / Event Subscriptions, then reinstall the app.
> If reinstall requires admin approval: Slack → Settings & administration → Manage apps → Requests.

---

#### Adding custom commands

Slack routes are configured via the admin API (`/api/v1/configs` with `kind=JSON`, `name=SlackRoutes`). The deploy scripts handle this with `--set-slack-routes`. Each route maps one or more trigger words to a job type and an optional variable binding:

```json
[
  {"triggers": ["standup", "status"], "job_type": "ai-standup-jira", "description": "Daily standup"},
  {"triggers": ["review"], "job_type": "ai-gh-review", "id_var": "PRUrl", "description": "Review a PR"},
  {"triggers": ["implement"], "job_type": "ai-jira-implement", "id_var": "JiraIssueKey", "description": "Implement a Jira issue"},
  {"triggers": ["security review"], "job_type": "ai-gh-review", "id_var": "PRUrl",
   "params": {"Skill": "ygs-security-review"}, "description": "Security audit a PR"}
]
```

**Field reference:**

| Field | Required | Description |
|-------|----------|-------------|
| `triggers` | yes | List of trigger phrases (matched against first word(s) of mention) |
| `job_type` | yes | Formicary job type to submit |
| `description` | yes | Shown in `@bot help` output |
| `id_var` | no | Job param name to receive the trailing text (e.g. `PRUrl`, `JiraIssueKey`) |
| `params` | no | Static params merged into every job submission (e.g. `{"Skill": "ygs-security-review"}`) |

**Important:** `SlackRouteConfig` uses `json:` struct tags — all field names are snake_case in the JSON payload (`job_type`, `id_var`, not `JobType`, `IdVar`). The deploy scripts use the correct casing.

**Upload routes:**
```bash
cd docs/examples
./deploy-ai-workflows.sh --set-configs --set-slack-routes --gh-org ORG --gh-repo REPO
# or for Jira:
./deploy-ai-jira-workflows.sh --set-configs --set-slack-routes
```

**View routes in the UI:** `https://<formicary-host>/dashboard/slack/routes` (requires Admin role)

**YAML config also works** (static config, no admin API needed):
```yaml
slack:
  routes:
    - triggers: ["standup", "status"]
      job_type: ai-standup-jira
      description: "Daily standup brief from Jira"
```
The trailing text after the trigger is bound to `id_var` (if set) or discarded — Formicary never reads or transforms the content.

---

## Deploy Script Reference

### Admin scripts (once per cluster)

| Script | What it does |
|--------|-------------|
| `../../scripts/deploy-formicary.sh` | Deploy/update the Formicary queen (auth + Slack secrets, k8s manifest) |

### Per-user scripts (once per developer)

| Script | What it does |
|--------|-------------|
| `../../scripts/setup-user-creds.sh` | Upload workflow YAMLs + push org configs for a developer |
| `deploy-ai-workflows.sh` | Upload GitHub workflow YAMLs (called by setup-user-creds.sh) |
| `deploy-ai-jira-workflows.sh` | Upload Jira workflow YAMLs (called by setup-user-creds.sh) |
| `deploy-ai-standup-jira.sh` | Upload `ai-standup-jira` YAML only | 
| `deploy-ai-standup-gh.sh` | Upload `ai-standup-gh` YAML only |

Workflow deploy scripts (`deploy-ai-workflows.sh`, `deploy-ai-jira-workflows.sh`, etc.) support:

```
--server URL           Formicary queen URL (default: http://localhost:7777)
--create-k8s-secret    Create/update the 'ai-dev-credentials' K8s secret from env vars
--set-configs          Push non-secret org configs to the server (requires FORMICARY_TOKEN)
--bedrock              Route Claude through AWS Bedrock proxy
--bedrock-url URL      Bedrock proxy URL (default: http://ai/bedrock)
--help                 Show usage
```

`setup-user-creds.sh` supports:

```
--server URL           Formicary queen URL (default: value of $FORMICARY_URL)
--tracker jira|github  Override auto-detected tracker
```

Credentials go in the `ai-dev-credentials` Kubernetes secret (via `--create-k8s-secret`). Non-secret config is pushed via `--set-configs`. Secrets are **always** passed via environment variables, never CLI flags:

```bash
FORMICARY_TOKEN        # Formicary JWT (from Profile → API Token in the UI)
GH_TOKEN               # GitHub PAT (also GITHUB_TOKEN)
JIRA_API_TOKEN         # Jira API token
BITBUCKET_TOKEN        # Bitbucket app password
SLACK_BOT_TOKEN        # Slack bot token (xoxb-...) — for Slack integration
SLACK_APP_TOKEN        # Slack app-level token (xapp-...) — for Socket Mode
SLACK_SIGNING_SECRET   # Slack signing secret — for request verification
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

## Testing AI Workflows

### Prerequisites for testing

```bash
source ~/.zshrc   # loads FORMICARY_TOKEN, FORMICARY_URL, SLACK_CHANNEL, etc.
export BASE="${FORMICARY_URL:-http://localhost:7777}"

# Confirm server is up
curl -s "$BASE/api/health" | python3 -m json.tool

# Confirm port-forward is active (Kubernetes)
kubectl port-forward svc/formicary 7777:7777 -n default &
```

---

### Test standup (Jira)

The standup is a cron job. A PENDING slot is created automatically when the job definition is registered. Trigger it manually by finding and firing that slot.

**Check for a PENDING slot:**
```bash
curl -s "$BASE/api/v1/jobs/requests?job_type=ai-standup-jira&job_state=WAITING&pageSize=5" \
  -H "Authorization: Bearer $FORMICARY_TOKEN" | python3 -c "
import json,sys
d=json.load(sys.stdin)
items = d if isinstance(d,list) else (d.get('records') or d.get('Records') or [])
print(f'{len(items)} WAITING slot(s)')
for j in items:
    print(f'  id={j[\"id\"]} cron_triggered={j.get(\"cron_triggered\")}')
"
```

**Trigger the PENDING slot** (sends output to your Slack channel):
```bash
cd /path/to/ai-dev-tools
python3 -c "
import os, sys
sys.path.insert(0, '.')
from scripts.slack.formicary_client import FormicaryClient
c = FormicaryClient(base_url=os.environ['FORMICARY_URL'], token=os.environ['FORMICARY_TOKEN'])
result = c.trigger_pending_or_submit('ai-standup-jira', {
    'SlackChannel': os.environ['SLACK_CHANNEL'],
    'SlackThreadTs': '',
})
print(result)
"
```

Expected outcomes:

| `result` contains | Meaning |
|---|---|
| `id` | Triggered — check the job at `http://localhost:7777/dashboard/jobs/requests/{id}` |
| `_already_executing: True` | A standup is already running — wait for it to finish |
| `_no_cron_slot: True` | No slot exists — redeploy to recreate it (see below) |

**If `_no_cron_slot`** — the cron slot was consumed or cancelled without a replacement being created. Redeploy to fix:
```bash
cd docs/examples
./deploy-ai-jira-workflows.sh --set-configs   # re-registers the job, recreates PENDING slot
```

**Watch job progress:**
```bash
JOB_ID=<id from above>
curl -s "$BASE/api/v1/jobs/requests/$JOB_ID" \
  -H "Authorization: Bearer $FORMICARY_TOKEN" | python3 -c "
import json,sys; d=json.load(sys.stdin)
jr=d.get('job_request',d)
print('state:', jr.get('job_state'))
print('error:', jr.get('error_message',''))
"
```

---

### Test standup (GitHub)

Same as above, but use `job_type=ai-standup-gh` and `DEFAULT_TRACKER=github`. Deploy with `deploy-ai-standup-gh.sh`.

---

### Test PR queue

The PR queue is triggered ad-hoc (not cron). It gathers sprint PRs from Jira's dev-status API and posts a formatted table to Slack.

**Trigger via curl:**
```bash
curl -s -X POST "$BASE/api/v1/jobs/requests" \
  -H "Authorization: Bearer $FORMICARY_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"job_type\": \"ai-adhoc\",
    \"params\": {
      \"Skill\": \"ygs-pr-queue\",
      \"Prompt\": \"\",
      \"SlackChannel\": \"$SLACK_CHANNEL\",
      \"SlackThreadTs\": \"\",
      \"DefaultTracker\": \"jira\"
    }
  }" | python3 -c "import json,sys; d=json.load(sys.stdin); print('id:', d.get('job_request',d).get('id'))"
```

**Test the gather step locally** (no Kubernetes needed):
```bash
cd /path/to/ai-dev-tools

JIRA_BASE_URL="$JIRA_BASE_URL" \
JIRA_EMAIL="$JIRA_EMAIL" \
JIRA_API_TOKEN="$JIRA_API_TOKEN" \
JIRA_PROJECT="$JIRA_PROJECT" \
BITBUCKET_WORKSPACE="$BITBUCKET_WORKSPACE" \
BITBUCKET_REPO="$BITBUCKET_REPO" \
BITBUCKET_USERNAME="$BITBUCKET_USERNAME" \
BITBUCKET_TOKEN="$BITBUCKET_TOKEN" \
DEFAULT_TRACKER="jira" \
WORKSPACE_DIR="/tmp/pr_queue_test" \
python3 -m scripts.standup.gather_pr_queue

# Inspect the result
cat /tmp/pr_queue_test/pr_queue.json | python3 -m json.tool | head -40
```

Expected: `pr_count` > 0 and each PR entry has `jira_key`, `url`, `approved_by`, `reviewers`.

---

### Test PR review

**GitHub:**
```bash
curl -s -X POST "$BASE/api/v1/jobs/requests" \
  -H "Authorization: Bearer $FORMICARY_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"job_type\":\"ai-gh-review\",\"params\":{\"PRUrl\":\"https://github.com/$GH_ORG/$GH_REPO/pull/1\",\"SlackChannel\":\"$SLACK_CHANNEL\"}}" \
  | python3 -c "import json,sys; print(json.load(sys.stdin).get('job_request',{}).get('id'))"
```

**Bitbucket:**
```bash
curl -s -X POST "$BASE/api/v1/jobs/requests" \
  -H "Authorization: Bearer $FORMICARY_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"job_type\":\"ai-jira-review\",\"params\":{\"PRUrl\":\"https://bitbucket.org/$BITBUCKET_WORKSPACE/$BITBUCKET_REPO/pull-requests/1\",\"SlackChannel\":\"$SLACK_CHANNEL\"}}" \
  | python3 -c "import json,sys; print(json.load(sys.stdin).get('job_request',{}).get('id'))"
```

The job pauses after posting findings to Slack. Click **Approve** or **Request Changes** in Slack to resume it.

---

### Test Slack commands — interactive REPL (no Slack workspace needed)

The fastest way to test all Slack commands locally is the interactive REPL from `ai-dev-tools`:

```bash
source ~/.zshrc   # loads FORMICARY_URL, FORMICARY_TOKEN, SLACK_CHANNEL, DEFAULT_TRACKER
cd /path/to/ai-dev-tools

# Live mode — submits real jobs to your Formicary server:
python3 scripts/slack/slack_repl.py

# Dry-run — no network calls, prints what would be submitted:
python3 scripts/slack/slack_repl.py --dry-run
```

> **TLS note:** If your server uses a self-signed cert (e.g. `*.nip.io`), the REPL auto-sets `FORMICARY_TLS_VERIFY=false` for nip.io URLs. For other hosts, `export FORMICARY_TLS_VERIFY=false` before launching.

**Full command test matrix:**

| Slack message | Expected job | Key params |
|---|---|---|
| `@bot standup` | `ai-standup-jira` | `SlackChannel`, `SlackThreadTs` |
| `@bot status` | same as standup | — |
| `@bot prs` | `ai-adhoc` (ygs-pr-queue) | `Skill=ygs-pr-queue`, `DefaultTracker` |
| `@bot risk` / `@bot risks` | `ai-adhoc` (ygs-risk-scan) | `Skill=ygs-risk-scan` |
| `@bot review <github-pr-url>` | `ai-gh-review` | `PRUrl=<url>` |
| `@bot review <bitbucket-pr-url>` | `ai-jira-review` | `PRUrl=<url>` |
| `@bot security review <pr-url>` | `ai-gh-review` | `PRUrl=<url>`, `Skill=ygs-security-review` |
| `@bot sre review <pr-url>` | `ai-gh-review` | `PRUrl=<url>`, `Skill=ygs-sre-review` |
| `@bot implement PROJ-123` | `ai-jira-implement` | `JiraIssueKey=PROJ-123` |
| `@bot implement 42` | `ai-gh-implement` | `GitHubIssueNumber=42` |
| `@bot jira query <keywords>` | `ai-jira-query` | `Query=<keywords>` |
| `@bot search jira <keywords>` | `ai-jira-query` | `Query=<keywords>` |
| `@bot jira-analyze PROJ-1` | `ai-jira-query` | `Mode=analyze`, `Query=PROJ-1` |
| `@bot gh-query <keywords>` | `ai-jira-query` | `Query=<keywords>`, `DefaultTracker=github` |
| `@bot gh-analyze #123` | `ai-jira-query` | `Mode=analyze`, `DefaultTracker=github` |
| `@bot doctor` | `ai-connectivity-check` | `SlackChannel` |

**Standup "no scheduled slot" error** — the cron PENDING slot was consumed. Fix:
```bash
cd docs/examples
./deploy-ai-jira-workflows.sh --set-configs   # re-registers the job, recreates PENDING slot
```

### Test Slack commands (built-in queen integration)

After deploying with `scripts/deploy-formicary.sh` and registering via `@bot setup`, test via real Slack:

```bash
# Local:
kubectl logs -l app=formicary --tail=50 -f | grep -i slack

# EC2:
./scripts/deploy-formicary.sh --queen-ip YOUR_QUEEN_IP --logs
```

**Verify job was submitted:**
```bash
curl -s "$FORMICARY_URL/api/v1/jobs/requests?pageSize=5" \
  -H "Authorization: Bearer $FORMICARY_TOKEN" | python3 -c "
import json,sys
d=json.load(sys.stdin)
items = d if isinstance(d,list) else (d.get('records') or d.get('Records') or [])
for j in items[:5]:
    print(f'{j[\"id\"]:26s} {j[\"job_type\"]:30s} {j[\"job_state\"]}')
"
```

---

### Test the implement pipeline

**GitHub:**
```bash
# 1. Label an issue ai-ready, then run picker
curl -s -X POST "$BASE/api/v1/jobs/requests" \
  -H "Authorization: Bearer $FORMICARY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"job_type":"ai-gh-issue-picker"}' \
  | python3 -c "import json,sys; print(json.load(sys.stdin).get('job_request',{}).get('id'))"

# 2. Or trigger implement directly on a known issue
curl -s -X POST "$BASE/api/v1/jobs/requests" \
  -H "Authorization: Bearer $FORMICARY_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"job_type\":\"ai-gh-implement\",\"params\":{\"GitHubIssueNumber\":\"42\",\"SlackChannel\":\"$SLACK_CHANNEL\"}}" \
  | python3 -c "import json,sys; print(json.load(sys.stdin).get('job_request',{}).get('id'))"
```

**Jira:**
```bash
curl -s -X POST "$BASE/api/v1/jobs/requests" \
  -H "Authorization: Bearer $FORMICARY_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"job_type\":\"ai-jira-implement\",\"params\":{\"JiraIssueKey\":\"PROJ-123\",\"SlackChannel\":\"$SLACK_CHANNEL\"}}" \
  | python3 -c "import json,sys; print(json.load(sys.stdin).get('job_request',{}).get('id'))"
```

---

### Inspect job artifacts

After any job completes, artifacts are visible in the UI at `http://localhost:7777/dashboard/jobs/requests/{id}` and downloadable via the API.

> **TLS note:** All curl commands below use `-sk` to skip certificate verification for self-signed certs on `*.nip.io` / EC2 deployments. Omit `-k` for localhost.

```bash
# Set these once from your shell (loaded from ~/.zshrc on EC2 deployments):
BASE="${FORMICARY_URL:-https://YOUR_QUEEN_IP.nip.io}"   # or http://localhost:7777
TOKEN="${FORMICARY_TOKEN}"
JOB_ID="<job-request-id>"
```

**List all artifacts for a job** (returns name + SHA-256 digest for each file):
```bash
curl -sk -H "Authorization: Bearer ${TOKEN}" \
  "${BASE}/api/artifacts?job_request_id=${JOB_ID}" | python3 -m json.tool
```

**Get job details** (state, error_message, task results):
```bash
curl -sk -H "Authorization: Bearer ${TOKEN}" \
  "${BASE}/api/jobs/requests/${JOB_ID}" | python3 -c "
import json,sys; d=json.load(sys.stdin)
jr=d.get('job_request',d)
print('state:', jr.get('job_state'))
print('error:', jr.get('error_message',''))
"
```

**Download a specific artifact by SHA-256 digest** — use the SHA from the artifact list above:
```bash
SHA="<sha256-from-artifact-list>"
# Download to stdout:
curl -sk -H "Authorization: Bearer ${TOKEN}" \
  "${BASE}/api/artifacts/${SHA}/download"

# Download to a file:
curl -sk -H "Authorization: Bearer ${TOKEN}" \
  "${BASE}/api/artifacts/${SHA}/download" -o findings.json
```

**Get the console log for a task** — the console log SHA is in `.task_executions[].tasks[].console_sha`:
```bash
# Step 1: extract console SHA for each task
curl -sk -H "Authorization: Bearer ${TOKEN}" \
  "${BASE}/api/jobs/requests/${JOB_ID}" | python3 -c "
import json,sys
d=json.load(sys.stdin)
jr=d.get('job_request',d)
for te in jr.get('task_executions') or []:
    for t in te.get('tasks') or []:
        sha = t.get('console_sha','')
        typ = t.get('task_type','?')
        if sha:
            print(f'{typ}: {sha}')
"

# Step 2: download the log (same /api/artifacts/<sha>/download endpoint)
curl -sk -H "Authorization: Bearer ${TOKEN}" \
  "${BASE}/api/artifacts/<CONSOLE_SHA>/download"
```

**One-liner: tail the console log of the most-recent job of a given type**:
```bash
JOB_TYPE="ai-jira-review"
curl -sk -H "Authorization: Bearer ${TOKEN}" \
  "${BASE}/api/jobs/requests?job_type=${JOB_TYPE}&pageSize=1" | python3 -c "
import json,sys,subprocess,os
d=json.load(sys.stdin)
items=d.get('records') or d.get('Records') or (d if isinstance(d,list) else [])
if not items: sys.exit('no jobs found')
jr=items[0]
print('job:', jr['id'], 'state:', jr.get('job_state'))
for te in jr.get('task_executions') or []:
    for t in te.get('tasks') or []:
        sha=t.get('console_sha','')
        if sha:
            print(f'--- task: {t[\"task_type\"]} ---')
            subprocess.run(['curl','-sk','-H',f'Authorization: Bearer {os.environ[\"FORMICARY_TOKEN\"]}',
                f'{os.environ[\"FORMICARY_URL\"]}/api/artifacts/{sha}/download'])
"
```

Key artifact files by workflow:

| Workflow | Files produced |
|---|---|
| `ai-standup-jira/gh` | `signals.json`, `standup_brief.md`, `risk_report.md`, `standup_report.md` |
| `ai-adhoc` (pr-queue) | `adhoc_result.json` |
| `ai-gh-review` / `ai-jira-review` | `findings.json`, `review_result.json` |
| `ai-gh-implement` / `ai-jira-implement` | `issue.json`, `plan.md`, `pr.json`, `learnings.md` |

---

### Troubleshooting

**Standup shows "already running"** — A cron slot is currently EXECUTING. Check:
```bash
curl -s "$BASE/api/v1/jobs/requests?job_type=ai-standup-jira&job_state=EXECUTING&pageSize=1" \
  -H "Authorization: Bearer $FORMICARY_TOKEN" | python3 -c "
import json,sys; d=json.load(sys.stdin)
items = d if isinstance(d,list) else (d.get('records') or d.get('Records') or [])
for j in items: print(j['id'], j['job_state'])
"
```

**Standup shows "no scheduled slot"** — Cron slot was consumed without replacement. Re-run the deploy script to recreate the PENDING slot:
```bash
./deploy-ai-jira-workflows.sh --set-configs
```

**PR queue shows 0 results** — Check that `DEFAULT_TRACKER=jira` is set and `JIRA_PROJECT` is configured. Run the gather script locally (see Test PR queue above) to see errors before submitting a job.

**Job stuck in EXECUTING** — View pod logs:
```bash
kubectl logs -l formicary-job-id=$JOB_ID --tail=100
```

**Read console log via API** (when kubectl isn't available):
```bash
# Get console SHA from job details, then download it
curl -sk -H "Authorization: Bearer ${FORMICARY_TOKEN}" \
  "${FORMICARY_URL}/api/jobs/requests/${JOB_ID}" | python3 -c "
import json,sys
d=json.load(sys.stdin)
jr=d.get('job_request',d)
for te in jr.get('task_executions') or []:
    for t in te.get('tasks') or []:
        sha=t.get('console_sha','')
        if sha: print(t['task_type'], sha)
"
# Then: curl -sk -H "Authorization: Bearer ${FORMICARY_TOKEN}" "${FORMICARY_URL}/api/artifacts/<SHA>/download"
```

**Slack bot does nothing** — Slack I/O is built into the queen; there is no separate router pod. Check queen logs and confirm the bot is in the channel:
```bash
# Local:
kubectl logs -l app=formicary --tail=50 | grep -i slack

# EC2 (kubectl tunneled over SSH):
./scripts/deploy-formicary.sh --queen-ip YOUR_QUEEN_IP --logs
# Expected: "Slack Socket Mode starting"

# Confirm bot is invited: /invite @<bot-name> in the channel
# Confirm the user has registered: DM the bot with "setup <formicary-token>"
```

---

## Further Reading

- [Architecture](../architecture.md)
- [Job YAML schema](../06-job-definitions.md)
- [Executors (Kubernetes, Docker, Shell)](../07-executors.md)
- [Scheduling and triggers](../08-scheduling-and-triggers.md)
- [Artifacts and caching](../09-artifacts-and-caching.md)
- [AI agents guide](../ai-agents.md)
- [Ant worker setup](../ant-worker-setup.md)
