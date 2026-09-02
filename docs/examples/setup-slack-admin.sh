#!/usr/bin/env bash
# setup-slack-admin.sh
#
# One-time admin setup for Slack integration with Formicary.
# Stores Slack tokens as admin-level SystemConfig entries (scope=default, kind=SLACK)
# so all organisations share a single bot without per-org duplication.
#
# Tokens are stored encrypted in the Formicary database.  No K8s secret or
# environment variable restart is needed — the queen reads them at startup.
#
# Usage:
#   source ~/.zshrc   # sets FORMICARY_TOKEN, SLACK_BOT_TOKEN, SLACK_APP_TOKEN
#   ./setup-slack-admin.sh
#   ./setup-slack-admin.sh --server https://YOUR_QUEEN_IP.nip.io
#   ./setup-slack-admin.sh --set-routes   # also push Slack route table
#   ./setup-slack-admin.sh --restart      # also restart the queen pod
#
# Secrets MUST be set via environment variables — never as CLI flags:
#   FORMICARY_TOKEN      Admin Formicary API token
#   SLACK_BOT_TOKEN      xoxb-... bot OAuth token (workers also need this)
#   SLACK_APP_TOKEN      xapp-... Socket Mode app-level token (server/queen admin ONLY)
#   SLACK_SIGNING_SECRET Optional webhook signing secret
#   QUEEN_IP             Queen host IP (required when --restart is used)
#   QUEEN_SSH_KEY        SSH key path (optional — uses SSH agent if unset)
#   QUEEN_SSH_USER       SSH user (optional — uses SSH config default if unset)
#
# Slack token roles:
#   SLACK_APP_TOKEN (xapp-...): used ONLY by the queen server for Socket Mode.
#     Never share with ant workers or regular users.
#   SLACK_BOT_TOKEN (xoxb-...): used by the queen for posting messages (stored in
#     formicary-slack k8s secret + SLACK/BotToken SystemConfig). AI workflow job containers
#     get the same token via the "SlackToken" org config set by deploy-ai-jira-workflows.sh.
#
set -euo pipefail

FORMICARY_URL="${FORMICARY_URL:-https://YOUR_QUEEN_IP.nip.io}"
TOKEN="${FORMICARY_TOKEN:-}"
SET_ROUTES=true
DO_RESTART=false
QUEEN_IP="${QUEEN_IP:-}"
QUEEN_SSH_KEY="${QUEEN_SSH_KEY:-}"
QUEEN_SSH_USER="${QUEEN_SSH_USER:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --server)      FORMICARY_URL="$2"; shift 2 ;;
    --set-routes)  SET_ROUTES=true;    shift ;;
    --restart)     DO_RESTART=true;    shift ;;
    *) echo "Unknown flag: $1" >&2; exit 1 ;;
  esac
done

CURL_OPTS=()
[[ "$FORMICARY_URL" == https://* ]] && CURL_OPTS+=(-k)

log()  { echo "▶ $*"; }
ok()   { echo "  ✓ $*"; }
fail() { echo "  ✗ ERROR: $*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# set_sysconfig — upsert an admin SystemConfig entry
#   $1 = name   (e.g. BotToken)
#   $2 = value
#   $3 = kind   (default: SLACK)
#   $4 = secret (default: true)
# ---------------------------------------------------------------------------
set_sysconfig() {
  local name="$1"
  local value="$2"
  local kind="${3:-SLACK}"
  local secret="${4:-true}"
  [[ -z "$value" ]] && { ok "Skipping $name (empty)"; return 0; }

  local payload
  payload=$(python3 -c "
import json, sys
print(json.dumps({
    'scope': 'default',
    'kind': sys.argv[1],
    'name': sys.argv[2],
    'value': sys.argv[3],
    'secret': sys.argv[4] == 'true',
}))
" "$kind" "$name" "$value" "$secret")

  local url="${FORMICARY_URL}/api/v1/configs"
  local args=(-s "${CURL_OPTS[@]+"${CURL_OPTS[@]}"}" -o /tmp/fmq-sysconfig-resp.json -w "%{http_code}"
              -X POST "$url" -H "Content-Type: application/json" -d "$payload")
  [[ -n "$TOKEN" ]] && args+=(-H "Authorization: Bearer ${TOKEN}")

  local http_code resp
  http_code=$(curl "${args[@]}" 2>/dev/null) || http_code="000"
  resp=$(cat /tmp/fmq-sysconfig-resp.json 2>/dev/null || true)
  rm -f /tmp/fmq-sysconfig-resp.json
  case "$http_code" in
    2*) ok "sysconfig $kind/$name saved" ;;
    000) fail "Cannot connect to ${FORMICARY_URL}" ;;
    401) fail "$name: 401 Unauthorized — use an admin FORMICARY_TOKEN" ;;
    403) fail "$name: 403 Forbidden — admin role required" ;;
    *)   echo "  Response: $resp" >&2; fail "$name: HTTP $http_code" ;;
  esac
}

# ---------------------------------------------------------------------------
# set_slack_routes — push the Slack route table as a JSON SystemConfig.
# Routes are instance-wide (not per-org).  The queen reloads them at startup.
# ---------------------------------------------------------------------------
set_slack_routes() {
  local routes_json="$1"
  log "Pushing Slack route table to admin SystemConfig ..."
  local payload
  payload=$(python3 -c "
import json, sys
v = sys.argv[1]
json.loads(v)  # validate
print(json.dumps({'scope':'default','kind':'JSON','name':'SlackRoutes','value':v,'secret':False}))
" "$routes_json") || fail "SlackRoutes: invalid JSON array"

  local url="${FORMICARY_URL}/api/v1/configs"
  local args=(-s "${CURL_OPTS[@]+"${CURL_OPTS[@]}"}" -o /tmp/fmq-routes-resp.json -w "%{http_code}"
              -X POST "$url" -H "Content-Type: application/json" -d "$payload")
  [[ -n "$TOKEN" ]] && args+=(-H "Authorization: Bearer ${TOKEN}")

  local http_code resp
  http_code=$(curl "${args[@]}" 2>/dev/null) || http_code="000"
  resp=$(cat /tmp/fmq-routes-resp.json 2>/dev/null || true)
  rm -f /tmp/fmq-routes-resp.json
  case "$http_code" in
    2*) ok "SlackRoutes admin config saved" ;;
    000) fail "Cannot connect to ${FORMICARY_URL}" ;;
    401) fail "SlackRoutes: 401 Unauthorized — use an admin token" ;;
    403) fail "SlackRoutes: 403 Forbidden — admin role required" ;;
    *)   echo "  Response: $resp" >&2; fail "SlackRoutes: HTTP $http_code" ;;
  esac
}

# ---------------------------------------------------------------------------
# Validate connectivity
# ---------------------------------------------------------------------------
log "Checking Formicary at ${FORMICARY_URL} ..."
_chk_args=(-s "${CURL_OPTS[@]+"${CURL_OPTS[@]}"}" -o /dev/null -w "%{http_code}")
[[ -n "$TOKEN" ]] && _chk_args+=(-H "Authorization: Bearer ${TOKEN}")
_http=$(curl "${_chk_args[@]}" "${FORMICARY_URL}/api/jobs/definitions" 2>/dev/null) || _http="000"
case "$_http" in
  2*|4*) ok "Formicary reachable (HTTP $_http)" ;;
  000)   fail "Cannot connect to ${FORMICARY_URL} — is the queen running?" ;;
  *)     fail "Unexpected HTTP $_http from ${FORMICARY_URL}" ;;
esac

# ---------------------------------------------------------------------------
# Update k8s 'formicary-slack' secret on the queen host when QUEEN_IP is set.
# This ensures the pod reads the latest tokens on next restart.
# ---------------------------------------------------------------------------
if [[ -n "$QUEEN_IP" && -n "${SLACK_APP_TOKEN:-}" ]]; then
  log "Updating formicary-slack k8s secret on ${QUEEN_IP} ..."
  SLACK_CHANNEL_VAL="${SLACK_CHANNEL:-}"
  _ssh_host="${QUEEN_SSH_USER:+${QUEEN_SSH_USER}@}${QUEEN_IP}"
  ssh${QUEEN_SSH_KEY:+ -i "$QUEEN_SSH_KEY"} -o StrictHostKeyChecking=no -o ConnectTimeout=10 "${_ssh_host}" \
    "KUBECONFIG=/etc/rancher/k3s/k3s.yaml kubectl create secret generic formicary-slack \
      --from-literal=app-token='${SLACK_APP_TOKEN}' \
      --from-literal=bot-token='${SLACK_BOT_TOKEN:-}' \
      --from-literal=signing-secret='${SLACK_SIGNING_SECRET:-}' \
      --from-literal=slack-channel='${SLACK_CHANNEL_VAL}' \
      --save-config --dry-run=client -o yaml | KUBECONFIG=/etc/rancher/k3s/k3s.yaml kubectl apply -f -" \
    2>&1 | sed 's/^/  /'
  ok "formicary-slack k8s secret updated"
fi

# ---------------------------------------------------------------------------
# Store Slack tokens in admin SystemConfig (shared across all orgs)
# ---------------------------------------------------------------------------
log "Storing Slack tokens in admin SystemConfig ..."
set_sysconfig "BotToken" "${SLACK_BOT_TOKEN:-}"     "SLACK" "true"
set_sysconfig "AppToken" "${SLACK_APP_TOKEN:-}"     "SLACK" "true"
set_sysconfig "SigningSecret" "${SLACK_SIGNING_SECRET:-}" "SLACK" "true"

# ---------------------------------------------------------------------------
# Optionally push Slack route table
# ---------------------------------------------------------------------------
if [[ "$SET_ROUTES" == true ]]; then
  log "Setting up Slack routes ..."
  SLACK_ROUTES_JSON='[
    {"triggers":["standup","daily","scrum","sync",
                 "jira standup","jira daily","jira scrum",
                 "gh standup","gh daily","github standup","github daily"],
     "job_type":"ai-standup-jira","description":"Daily standup summary",
     "tracker_variants":{"github":"ai-standup-gh","jira":"ai-standup-jira"}},

    {"triggers":["query","search","find","jira query","jira search","jira find","jira-query","gh query","gh search","gh find","gh-query","github query","github search"],"job_type":"ai-jira-query","description":"Query issues","id_var":"Query"},
    {"triggers":["analyze","analysis","jira analyze","jira analysis","jira-analyze","gh analyze","gh analysis","gh-analyze","github analyze","github analysis"],"job_type":"ai-jira-query","description":"Analyze issues","id_var":"Query","params":{"Mode":"analyze"}},

    {"triggers":["pr comments","show pr comments","pr feedback","pr discussion"],"job_type":"ai-adhoc","description":"Show existing PR comments","id_var":"Prompt","params":{"Skill":"ygs-pr-comments"}},

    {"triggers":["review","pr review","code review",
                 "jira review","jira pr review",
                 "gh review","gh pr review","github review"],"job_type":"ai-jira-review","description":"Review a PR","id_var":"PRUrl",
     "tracker_variants":{"github":"ai-gh-review","jira":"ai-jira-review"}},

    {"triggers":["implement","fix","create pr","open pr",
                 "jira implement","jira fix",
                 "gh implement","gh fix","github implement"],"job_type":"ai-jira-implement","description":"Implement an issue","id_var":"IssueNumber",
     "tracker_variants":{"github":"ai-gh-implement","jira":"ai-jira-implement"}},

    {"triggers":["risks","risk scan","security",
                 "jira risks","jira risk scan",
                 "gh risks","github risks"],"job_type":"ai-adhoc","description":"Security or risk scan","params":{"Skill":"ygs-risk-scan"}},
    {"triggers":["prs","pr queue","open prs","list prs",
                 "jira prs","jira pr queue","jira open prs",
                 "gh prs","gh pr queue","github prs","github pr queue"],"job_type":"ai-adhoc","description":"List open PRs","params":{"Skill":"ygs-pr-queue"}},
    {"triggers":["ask","question"],"job_type":"ai-adhoc","description":"Answer a question","id_var":"Prompt","params":{"Skill":"ygs-ask"}}
  ]'
  set_slack_routes "$SLACK_ROUTES_JSON"
fi

# ---------------------------------------------------------------------------
# Optionally restart the queen to apply new tokens
# ---------------------------------------------------------------------------
if [[ "$DO_RESTART" == true ]]; then
  [[ -z "$QUEEN_IP" ]] && fail "--restart requires QUEEN_IP to be set"
  log "Restarting Formicary queen on ${QUEEN_IP} ..."
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  QUEEN_IP="$QUEEN_IP" QUEEN_SSH_KEY="$QUEEN_SSH_KEY" QUEEN_SSH_USER="$QUEEN_SSH_USER" \
    "${SCRIPT_DIR}/../../scripts/deploy-formicary.sh" --restart
  ok "Queen restarted"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Slack admin setup complete."
echo "  Restart the queen to apply new tokens:"
echo "    QUEEN_IP=\$QUEEN_IP ./docs/examples/setup-slack-admin.sh --restart"
echo "  Or manually:"
echo "    QUEEN_IP=\$QUEEN_IP ./scripts/deploy-formicary.sh --restart"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
