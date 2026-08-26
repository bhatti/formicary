# Getting Started with Formicary

Complete end-to-end guide: deploy the server, onboard as a user, connect a worker, and run AI workflows via Slack.

---

## Overview

Formicary has two roles:

| Role | Who | What they do |
|------|-----|-------------|
| **Server Admin** | One-time setup | Deploy the queen, configure Slack Socket Mode |
| **User / Worker** | Each developer | Create API token, connect worker, register with Slack bot |

---

## Phase 1: Server Setup (Admin Only)

> Skip this phase if you are connecting to an existing Formicary server.

### Prerequisites
- A Linux host with k3s or a Kubernetes cluster
- `kubectl` configured to reach the cluster
- Docker (for building the image)

### 1.1 Deploy the Queen

```bash
# Clone the repo
git clone https://github.com/bhatti/formicary.git
cd formicary

# Add to ~/.zshrc (autodetected by all deploy scripts)
export QUEEN_IP="<your-server-ip>"                 # public IP of the k3s host
export QUEEN_SSH_KEY="~/.ssh/your-key.pem"         # SSH key path (omit to use ssh-agent)
export QUEEN_SSH_USER="ec2-user"                   # SSH user (ec2-user for Amazon Linux, ubuntu for Ubuntu)
export FORMICARY_URL="https://${QUEEN_IP}.nip.io"
export COMMON_AUTH_JWT_SECRET="$(openssl rand -base64 32)"

# OAuth (Google or GitHub — required for login)
export COMMON_AUTH_GOOGLE_CLIENT_ID="..."
export COMMON_AUTH_GOOGLE_CLIENT_SECRET="..."
export COMMON_AUTH_GOOGLE_CALLBACK_HOST="${QUEEN_IP}.nip.io"

# Deploy — reads all vars from ~/.zshrc automatically (no source needed)
./scripts/deploy-formicary.sh
```

Verify: `curl -k https://${QUEEN_IP}.nip.io/api/health`

### 1.2 Configure the Slack Bot (Admin Only)

The Slack bot requires two tokens. **Only the server admin runs this.**

| Token | Where to get it | Who needs it |
|-------|----------------|--------------|
| `SLACK_APP_TOKEN` (`xapp-...`) | Slack App → Socket Mode → App-Level Token | **Server only** — never share with workers |
| `SLACK_BOT_TOKEN` (`xoxb-...`) | Slack App → OAuth → Bot User OAuth Token | Server + workers (for notifications) |

```bash
export SLACK_APP_TOKEN="xapp-1-..."    # server-only Socket Mode token
export SLACK_BOT_TOKEN="xoxb-..."      # bot OAuth token
export SLACK_CHANNEL="#your-channel"   # default notification channel

./docs/examples/setup-slack-admin.sh \
  --server https://${QUEEN_IP}.nip.io \
  --queen-ip "${QUEEN_IP}"
```

After this step the Slack bot will connect in Socket Mode. Verify in the dashboard:
`https://<host>/dashboard` → Health banner should show no warnings.

---

## Phase 2: User Onboarding (Per User)

Every developer who wants to run AI jobs needs to complete these steps.

### 2.1 Sign Up

1. Visit `https://<host>/dashboard`
2. Sign in via Google or GitHub OAuth
3. If your organization uses invitations, use the invitation link provided by the admin

### 2.2 Create an API Token

Dashboard → **Profile** → **API Tokens** → **New Token**

Copy the token and add it to your shell config:

```bash
# Add to ~/.zshrc
export FORMICARY_URL="https://<host>"
export FORMICARY_TOKEN="eyJhbGci..."   # paste your token here
```

### 2.3 Export Credentials

Add all required credentials to `~/.zshrc`. The worker setup scripts autodetect these — no manual sourcing needed.

```bash
# GitHub AI workflows
export GH_TOKEN="ghp_..."                           # GitHub personal access token (repo + issues scopes)
export GH_ORG="your-org"                            # default GitHub org for jobs
export GH_REPO="your-repo"                          # default GitHub repo for jobs
export SSH_PRIVATE_KEY="$(cat ~/.ssh/id_ed25519)"   # actual PEM content, NOT a file path

# Jira / Bitbucket AI workflows (skip if not using Jira)
export JIRA_BASE_URL="https://yourcompany.atlassian.net"
export JIRA_HOST="yourcompany.atlassian.net"
export JIRA_EMAIL="you@company.com"
export JIRA_API_TOKEN="ATATT3x..."
export BITBUCKET_WORKSPACE="your-workspace"
export BITBUCKET_USERNAME="you@company.com"
export BITBUCKET_TOKEN="..."

# Slack (for job notifications — bot token only, NOT the app token)
export SLACK_BOT_TOKEN="xoxb-..."
```

> **SSH_PRIVATE_KEY**: Must be the actual PEM key content, not a file path.
> Use `export SSH_PRIVATE_KEY="$(cat ~/.ssh/id_ed25519)"`.

### 2.4 Register with the Slack Bot

Visit your profile and use the one-time code to register:

```
https://<host>/dashboard/users/self
```

Open the **Connect Slack** tab and follow the instructions (three registration methods available).

---

## Phase 3: Worker Setup

### 3.1 Prerequisites

- `kubectl` configured and pointing at the Formicary cluster
- Credentials exported in `~/.zshrc` (Phase 2.3)

### 3.2 Verify Credentials Locally

Before deploying, check that all your credentials are valid:

```bash
cd formicary
bash scripts/check-credentials.sh
```

Fix any failures before proceeding.

### 3.3 Deploy the Worker

```bash
# For GitHub workflows:
bash scripts/setup-ant-worker.sh github

# For Jira/Bitbucket workflows:
bash scripts/setup-ant-worker.sh jira

# For both:
bash scripts/setup-ant-worker.sh github jira

# Check credentials only, no deploy:
bash scripts/setup-ant-worker.sh --check-only
```

The script autodetects all credentials from `~/.zshrc` automatically. It will:
1. Create the `formicary-ant` k8s deployment
2. Populate the `ai-dev-credentials` k8s secret with all your credentials
3. Deploy the AI workflow job definitions (`ai-gh-implement`, `ai-jira-implement`, etc.)
4. Run the credential doctor — verify all checks pass

### 3.4 Verify in the Dashboard

Dashboard → **Ants** — your worker should appear as connected.

---

## Phase 4: Verify End-to-End

### 4.1 Slack Doctor

In any Slack channel where the bot is present:

```
@bot doctor
```

This triggers the `ai-connectivity-check` job, which tests GitHub, Jira, Slack, and Claude API reachability from inside the worker pod.

> **Note**: `@bot doctor` requires workflow definitions to be deployed first (Phase 3.3). If it says "unknown command", run `setup-ant-worker.sh github` first.

### 4.2 Test an Implement Job

```
@bot implement https://github.com/<org>/<repo>/issues/<num>
```

The worker will:
1. Fetch the issue details
2. Generate an implementation plan
3. Clone the repo, implement changes
4. Create a pull request
5. Reply in the Slack thread with the PR link

For Jira:
```
@bot implement https://<jira-host>/browse/<PROJECT-123>
```

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| Job fails at `clone_repo` | Empty `GH_ORG`, `GH_REPO`, or `SSH_PRIVATE_KEY` in k8s secret | Re-run `setup-ant-worker.sh github` |
| `SSH_PRIVATE_KEY invalid` in credential check | `SSH_PRIVATE_KEY` is a file path, not key content | `export SSH_PRIVATE_KEY="$(cat ~/.ssh/id_rsa)"` |
| `@bot` doesn't respond | Bot not registered in Slack channel | `/invite @<bot-name>` then try again |
| Job stays in WAITING | No ant worker registered | `kubectl get pods`, check `formicary-ant` pod |
| `GH_TOKEN` fails | Token missing `repo` or `issues` scope | Regenerate at github.com/settings/tokens |
| Jira 401 | Wrong email or expired API token | Regenerate at id.atlassian.com/manage-profile/security/api-tokens |
| No Slack replies | Wrong `SLACK_BOT_TOKEN` | Verify with `scripts/check-credentials.sh` |

### Re-run credential setup after changes

If you update credentials in `~/.zshrc`, re-run setup to push them to the k8s secret:

```bash
source ~/.zshrc
bash scripts/setup-user-creds.sh github  # or jira, or all
```

---

## Next Steps

- [Ant Worker Setup](ant-worker-setup.md) — detailed worker configuration reference
- [AI Agents Guide](ai-agents.md) — architecture, job types, Slack commands
- [Configuration Reference](15-configuration.md) — all config keys and env vars
- [API Reference](16-api-reference.md) — REST API for job submission and management
