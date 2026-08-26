# Ant Worker Setup — Developer Onboarding

Connect your personal ant worker to the shared Formicary queen. Your ant runs
on your laptop, registers over WebSocket, and executes job pods locally so your
work stays isolated from teammates'.

**Time to complete:** ~5 minutes after the queen is up.

---

## Prerequisites

- Docker Desktop (or k3s / Rancher Desktop) running locally
- `kubectl` pointing at your local cluster (`kubectl get nodes` returns your machine)
- Credentials exported in `~/.zshrc` (see Step 1)

---

## Step 1 — Export credentials to ~/.zshrc

Add all of the following to `~/.zshrc`. The setup scripts autodetect these — you
**never** need to `source ~/.zshrc` or pass flags manually.

```bash
# Formicary queen connection
export QUEEN_IP="<server-ip>"
export QUEEN_SSH_KEY="~/.ssh/your-key.pem"     # path to SSH key (omit = use ssh-agent)
export QUEEN_SSH_USER="ec2-user"               # ec2-user (Amazon Linux) or ubuntu (Ubuntu)
export FORMICARY_URL="https://${QUEEN_IP}.nip.io"
export FORMICARY_TOKEN="<token from dashboard → Profile → API Tokens>"

# GitHub AI workflows
export GH_TOKEN="ghp_..."
export GH_ORG="your-org"
export GH_REPO="your-default-repo"
export SSH_PRIVATE_KEY="$(cat ~/.ssh/id_ed25519)"   # key CONTENT not path

# Jira / Bitbucket (skip if not using Jira)
export JIRA_BASE_URL="https://yourcompany.atlassian.net"
export JIRA_HOST="yourcompany.atlassian.net"
export JIRA_EMAIL="you@company.com"
export JIRA_API_TOKEN="ATATT3x..."
export BITBUCKET_WORKSPACE="your-workspace"
export BITBUCKET_USERNAME="you@company.com"
export BITBUCKET_TOKEN="..."

# Slack (bot token only — NOT the xapp- Socket Mode token)
export SLACK_BOT_TOKEN="xoxb-..."
```

> **SSH_PRIVATE_KEY**: Must be the actual key **content**, not a file path.
> Use `export SSH_PRIVATE_KEY="$(cat ~/.ssh/id_ed25519)"`.

---

## Step 2 — Verify credentials locally

Before deploying, run the credential doctor from a fresh terminal (no source needed):

```bash
cd ~/workplace/formicary
bash scripts/check-credentials.sh
```

All checks should pass. Fix any failures before proceeding.

---

## Step 3 — Deploy ant worker + credentials + workflows

```bash
bash scripts/setup-ant-worker.sh
```

No arguments needed. The script autodetects all credentials from `~/.zshrc` and:
1. Creates the `formicary-ant-credentials` k8s secret
2. Deploys the `formicary-ant` pod
3. Sets up credentials for every tracker whose token is present (GitHub, Jira, Bitbucket)
4. Deploys AI workflow job definitions (`ai-gh-implement`, `ai-jira-implement`, etc.)
5. Runs the credential doctor
6. Verifies ant connection

To limit scope:
```bash
bash scripts/setup-ant-worker.sh github        # GitHub only
bash scripts/setup-ant-worker.sh jira bb       # Jira + Bitbucket only
bash scripts/setup-ant-worker.sh --skip-worker # credentials + workflows only (ant already running)
bash scripts/setup-ant-worker.sh --check-only  # credential check only, no deploy
```

With auth enabled, the queen extracts `org_id` from your JWT at connect time and
routes your org's Slack commands to your ant automatically.

---

## Step 4 — Verify the ant is connected

```bash
kubectl get pods | grep formicary-ant
kubectl logs deployment/formicary-ant --tail=20
# Expected: "connected to queen at wss://..."
```

Dashboard: `${FORMICARY_URL}/dashboard/ants` — your ant should appear within 30 seconds.

If the ant is missing, check credentials and re-run:
```bash
bash scripts/setup-ant-worker.sh --skip-worker   # re-push creds + workflows without restarting ant
```

---

## Step 5 — Register with the Slack bot

Each developer must link their Formicary account to Slack before `@bot` commands will work.

1. Visit your profile: `${FORMICARY_URL}/dashboard/users/self`
2. Open the **Connect Slack** tab
3. Copy your one-time code and follow one of the three methods shown

Or from Slack directly (DM the bot):
```
@bot setup <one-time-code>
```

You only need to do this once. Generate a new code at any time from your profile page.

---

## Step 6 — Test

In Slack (in a channel where the bot is invited):

```
@bot help
```

Lists all available commands.

```
@bot standup
```

Standup brief posted to the same thread. The Formicary UI shows the job pod running on your ant (routed by org).

---

## How routing works

Formicary routes jobs to ant workers using two complementary mechanisms, applied
in order:

| Priority | Mechanism | When active | Config needed |
|----------|-----------|-------------|---------------|
| 1st | **Org-based routing** | Auth enabled (`auth.jwt_secret` set) | None — automatic |
| 2nd | **Tag-based routing** | Always available | `pod_labels` on task container |

### Org-based routing (automatic when auth is enabled)

When the Formicary queen has auth enabled, every API token carries an `org_id`
claim in its JWT. When your ant worker connects over WebSocket, the queen extracts
`org_id` from its JWT and records it on the ant registration server-side — the
ant never self-reports its org (this prevents spoofing).

Jobs submitted by users in org `X` are automatically routed **only** to ant workers
whose token carries the same `org_id=X`. No YAML changes, no tags required.

**Fallback**: if no live org-scoped ant is available, the job falls back to
unscoped ants (ants registered with `auth.jwt_secret` not set, or embedded ants).
This supports mixed deployments during migration.

**Auth disabled**: when `auth.jwt_secret` is not set, all ants have `org_id=""`,
org filtering is skipped entirely, and only tag-based routing applies.

### Tag-based routing (optional, for sub-org isolation)

For deployments where multiple developers share the same org and need per-person
isolation, jobs can carry additional `pod_labels` that must match the ant's labels.
Add `pod_labels` to a task's `container:` block and match them to the ant's
`--methods` / label config. Org routing still applies first; tag routing is a
secondary filter within the org.

---

## Stopping, restarting, and updating the ant worker

After the queen restarts (e.g. after `kubectl rollout restart deployment/formicary` on EC2),
ant workers lose their WebSocket connection and attempt automatic reconnection with
exponential backoff. Wait ~60 seconds. If they do not reconnect:

```bash
# Check ant pod status and logs
kubectl get pods | grep formicary-ant
kubectl logs deployment/formicary-ant --tail=30
# Look for: "connected to queen at ws://..." or reconnect backoff messages

# Force reconnect — restart the ant pod
kubectl rollout restart deployment/formicary-ant
kubectl rollout status deployment/formicary-ant --timeout=60s
```

To stop the ant worker entirely (e.g. switching queen URLs or tokens):

```bash
# Delete the deployment — pod is gone, no more job execution
kubectl delete deployment formicary-ant 2>/dev/null || true

# Redeploy with updated credentials when ready:
export FORMICARY_URL=https://new-host.nip.io
./scripts/setup-ant-worker.sh
```

To update the ant image or pull the latest:

```bash
kubectl rollout restart deployment/formicary-ant
kubectl rollout status deployment/formicary-ant --timeout=60s
```

Verify the ant is live on the queen after any restart:

```bash
curl -k -H "Authorization: Bearer $FORMICARY_TOKEN" \
  "${FORMICARY_URL}/api/ants" | python3 -c "
import sys, json
ants = json.load(sys.stdin).get('records', [])
if not ants:
    print('NO ANTS REGISTERED — check ant pod logs')
else:
    for a in ants:
        print(f\"ant={a.get('ant_id','?')}  org={a.get('org_id','?')}  alive={a.get('alive',False)}\")
"
```

---

## Per-Method Health Gating

The ant worker probes each registered executor method (e.g. `KUBERNETES`, `DOCKER`) every 30 seconds. A method is marked **unhealthy** only after **3 consecutive probe failures** — transient blips (network hiccup, slow kubelet) do not immediately gate new work. Once healthy again (1 successful probe), it is re-enabled immediately.

The health checker reuses the executor's existing Kubernetes client (`GetClient()`) rather than creating a new connection on every probe, which avoids credential and connection overhead.

**Impact on job routing:** The queen receives method health in each heartbeat. If a method is unhealthy, the queen will not route jobs requiring that executor to that ant until health is restored.

**Logs to watch:**

```
[health] KUBERNETES UNHEALTHY after 3 consecutive failures — gating new work
[health] KUBERNETES recovered — marking healthy
```

---

## Slack tokens — worker vs server

| Token | Starts with | Who needs it | Purpose |
|-------|-------------|--------------|---------|
| `SLACK_BOT_TOKEN` | `xoxb-` | **Workers** + queen | Job notifications, API calls |
| `SLACK_APP_TOKEN` | `xapp-` | **Queen only** | Socket Mode inbound commands |

Workers only need `SLACK_BOT_TOKEN` (set in `~/.zshrc`, autodetected by `setup-ant-worker.sh`).
The `SLACK_APP_TOKEN` is a server-admin concern — see the queen setup docs.

For custom Slack commands (new `@bot` triggers), ask the server admin to update the
route table via `docs/examples/setup-slack-admin.sh --set-routes`.
See [Getting Started — Phase 1](00-getting-started.md) for the queen setup guide.

---

## EC2 Database Migration (existing installs)

Fresh installs pick up schema changes automatically. If your EC2 queen is running an existing SQLite database, apply the following one-time ALTER statements after pulling the new image:

```bash
# SSH into EC2 and run against formicary.db (adjust path as needed):
sqlite3 /path/to/formicary.db <<'SQL'
ALTER TABLE formicary_log_events ADD COLUMN level VARCHAR(10) NOT NULL DEFAULT 'info';
CREATE INDEX IF NOT EXISTS formicary_log_events_level_ndx ON formicary_log_events(level);
SQL
```

> These columns are `NOT NULL DEFAULT 'info'` so all existing rows are safe; no data migration needed.

---

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| Ant pod stuck in `ContainerCreating` | `kubectl describe pod <ant-pod>` — often a missing or wrong secret key |
| Ant not appearing in dashboard after 60s | Check pod logs: `kubectl logs deployment/formicary-ant`. Verify `QUEEN_HOST` is reachable from inside the pod (`kubectl exec -it <ant-pod> -- curl http://$QUEEN_HOST:7777/api/jobs/definitions`). |
| Jobs don't route to my ant | **With auth enabled**: Verify your ant's JWT carries `org_id` matching your org — check queen logs: `kubectl logs deployment/formicary \| grep OrgID`. If org routing passes but no ant is selected, check that your ant is live in the dashboard and that its `OrgID` matches the submitting user's org. **Without auth**: only `pod_labels` tag routing applies — verify the ant's registered tags match any `pod_labels` specified in the job YAML. |
| `kubectl: no context` | Run `kubectl config current-context` — Docker Desktop must be running and context set to `docker-desktop` or `rancher-desktop`. |
| Old k8s.yaml missing | Removed. Use `k8s/formicary-all-in-one.yaml` for single-node or `k8s/formicary-leader.yaml` + `formicary-ant.yaml` for multi-user. |

---

## See Also

- [AI Agents Guide](ai-agents.md) — GitHub/Jira agent workflow details
- [Examples README](examples/README.md) — deploy scripts and Slack integration
- [Configuration Reference](15-configuration.md) — queen config including Slack routes
