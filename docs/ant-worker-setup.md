# Ant Worker Setup — Developer Onboarding

Connect your personal ant worker to the shared Formicary leader. Your ant runs
on your laptop, registers with the EC2 leader over WebSocket, and runs all job
pods locally so your work stays isolated from your teammates'.

**Time to complete:** ~5 minutes after the leader is up.

---

## Prerequisites

- Docker Desktop (or k3s / Rancher Desktop) running locally
- `kubectl` pointing at your local cluster (`kubectl get nodes` returns your machine)
- The Formicary leader URL from your team lead, e.g. `https://formicary.example.com` or `http://EC2_IP:7777`
- A personal API token (generate in the Formicary UI)

---

## Step 1 — Generate your API token

Open the Formicary dashboard in your browser, register / log in, then:

```
Dashboard → (your name top-right) → API Tokens → New Token
```

Copy the token. It is a JWT — treat it like a password.

```bash
export FORMICARY_URL="https://<EC2_IP_OR_HOSTNAME>.nip.io"   # same URL used by deploy-ai-workflows.sh
export FORMICARY_TOKEN="<token from UI>"
```

> **TLS note:** If the leader has HTTPS via nginx, `QUEEN_HOST` is the domain
> name (`formicary.example.com`). The ant connects to the WebSocket endpoint
> `ws://QUEEN_HOST:7777/ws/queue`. If nginx terminates TLS and proxies to 7777
> internally, use the IP/hostname directly without `https://`; the ant only
> needs the raw WebSocket port, not the HTTPS port.

---

## Step 2 — Run the setup script

From the formicary repo on your laptop:

```bash
cd ~/workplace/formicary

./scripts/setup-ant-worker.sh
  # Uses FORMICARY_URL and FORMICARY_TOKEN from env (same as deploy-ai-workflows.sh)
  # Optional overrides:
  # --server  $FORMICARY_URL   (or set FORMICARY_URL env var)
  # --token   $FORMICARY_TOKEN (or set FORMICARY_TOKEN env var)
  # --port    7777             (WebSocket port, default: 7777)
  # --s3-port 19000            (S3 port, default: 19000)
  # --namespace default        (kubectl namespace, default: default)
  # --dry-run                  (prints rendered YAML without applying)
```

The script does three things:
1. Creates a `formicary-ant-credentials` Kubernetes secret with your queen host and token
2. Renders `k8s/formicary-ant.yaml` with your values substituted in
3. `kubectl apply`s the manifest — ant pod starts and registers with the leader

With auth enabled the queen extracts `org_id` from your token's JWT at connect time
and automatically routes your org's jobs to this ant. No `--user` tag is needed.

---

## Step 3 — Verify the ant is connected

Open the Formicary dashboard:

```
${FORMICARY_URL}/dashboard/ants
```

Your ant should appear in the list within 30 seconds.

Check local pod status:

```bash
kubectl get pods | grep formicary-ant
kubectl logs deployment/formicary-ant --tail=20
# Expected: "connected to queen at ws://..."
```

---

## Step 4 — Upload your workflow YAMLs

Each developer uploads the shared workflow definitions to the leader. With auth
enabled, org-based routing automatically directs your Slack commands to your ant —
no additional per-user config is required.

```bash
cd ~/workplace/formicary/docs/examples

export FORMICARY_URL="${FORMICARY_URL}"   # already set from Step 1

# GitHub workflows
export GH_TOKEN="ghp_..."
export SSH_PRIVATE_KEY="$(cat ~/.ssh/id_rsa)"
export SLACK_BOT_TOKEN="xoxb-..."

./deploy-ai-workflows.sh \
  --server "$FORMICARY_URL" \
  --create-k8s-secret \
  --set-configs \
  --gh-org YOUR_ORG \
  --gh-repo YOUR_REPO
# With auth enabled, org-based routing is automatic — no --ant-user-tag flag needed.

# OR: Jira + Bitbucket workflows
export JIRA_API_TOKEN="..."
export BITBUCKET_TOKEN="..."
./deploy-ai-jira-workflows.sh \
  --server "$FORMICARY_URL" \
  --create-k8s-secret \
  --set-configs \
  --jira-project MYPROJ \
  --bb-workspace YOUR_WS \
  --bb-repo YOUR_REPO
```

---

## Step 5 — Register your Slack user

Each developer must register their Formicary token with the bot before Slack commands will work.

Open a DM to your bot in Slack and type:
```
setup <your-formicary-token>
```

The bot validates the token, stores it encrypted, and deletes the DM. You only need to do this once.

Get your token at: `https://<formicary-host>/dashboard/users/tokens`

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

## Slack k8s secret (required for Socket Mode bot)

The inbound Slack bot (`@bot help`, `@bot standup`, etc.) uses Socket Mode and requires an App-level token (xapp-).
The `deploy-formicary.sh` script creates the `formicary-slack` k8s secret automatically when `SLACK_APP_TOKEN` is set:

```bash
export SLACK_APP_TOKEN=xapp-1-...
export SLACK_BOT_TOKEN=xoxb-...
export SLACK_SIGNING_SECRET=...      # optional
export SLACK_CHANNEL=sb-bot-notify  # optional default channel

EC2_IP=<your-ec2-ip> ./scripts/deploy-formicary.sh
```

To create or update the secret manually:
```bash
kubectl create secret generic formicary-slack \
  --from-literal=app-token="${SLACK_APP_TOKEN}" \
  --from-literal=bot-token="${SLACK_BOT_TOKEN}" \
  --from-literal=signing-secret="${SLACK_SIGNING_SECRET:-}" \
  --dry-run=client -o yaml | kubectl apply -f -
```

If `SLACK_APP_TOKEN` is not set, the bot is disabled with a warning in queen logs. Bot posting (outbound notifications) still works as long as `SLACK_BOT_TOKEN` is set (either via env or via Admin > System Config `kind=SLACK name=BotToken`).

See [Configuration Reference — Slack](15-configuration.md#slack-configuration) for the full 2-level token override table and Admin UI settings.

---

## Custom Slack commands (no code changes needed)

Add your own Slack commands by adding routes to the queen config. Edit `k8s/formicary-leader.yaml`:

```yaml
# In the formicary ConfigMap, under slack.routes:
slack:
  routes:
    - triggers: ["standup", "status"]
      job_type: ai-standup-jira
      description: "Daily standup brief from Jira"
    - triggers: ["deploy"]
      job_type: ai-adhoc
      description: "My custom deploy workflow"
```

Then redeploy:
```bash
./scripts/deploy-formicary.sh --ec2-ip $EC2_IP
```

Trailing text after the trigger is passed as the `Prompt` param to the job container.

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
