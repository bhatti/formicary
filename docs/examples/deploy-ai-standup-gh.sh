#!/usr/bin/env bash
# deploy-ai-standup-gh.sh
#
# Uploads the ai-standup-gh.yaml workflow to a running Formicary queen and
# stores all required org configs for the GitHub + Slack standup.
#
# Usage:
#   ./deploy-ai-standup-gh.sh --create-k8s-secret
#   ./deploy-ai-standup-gh.sh --create-k8s-secret --set-configs \
#       --gh-org myorg --gh-repo myrepo --slack-channel standup
#   ./deploy-ai-standup-gh.sh --set-configs \
#       --gh-org myorg --gh-repo myrepo --slack-channel standup \
#       --team-members "alice,bob,charlie"
#   ./deploy-ai-standup-gh.sh --server http://host:7777 --set-configs \
#       --gh-org myorg --gh-repo myrepo --slack-channel standup
#
# Credentials are stored in the 'ai-dev-credentials' Kubernetes secret.
# Use --create-k8s-secret to create/update the secret from env vars (one-time setup).
#
# Secrets MUST be supplied via environment variables — never as CLI flags:
#   FORMICARY_TOKEN        Formicary API token
#   GH_TOKEN / GITHUB_TOKEN  GitHub personal access token
#   SLACK_BOT_TOKEN        Slack bot xoxb-... token
#   ANTHROPIC_API_KEY      Optional — omit when using Bedrock
#   SSH_PRIVATE_KEY        Optional — SSH key for git operations
#
# Slack bot scopes needed:
#   channels:history  channels:read  groups:history  groups:read
#   chat:write  users:read
#
set -euo pipefail

# ── Helpers (defined first) ────────────────────────────────────────────────────
log()  { echo "▶ $*"; }
ok()   { echo "  ✓ $*"; }
fail() { echo "  ✗ ERROR: $*" >&2; echo "  ✗ ERROR: $*"; exit 1; }

# ── Defaults ──────────────────────────────────────────────────────────────────
FORMICARY_URL="${FORMICARY_URL:-http://localhost:7777}"
TOKEN="${FORMICARY_TOKEN:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

SET_CONFIGS=false
CREATE_K8S_SECRET=true
GH_ORG="${GH_ORG:-${GITHUB_ORG:-}}"
GH_REPO="${GH_REPO:-${GITHUB_REPO:-}}"
GH_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
SLACK_TOKEN="${SLACK_BOT_TOKEN:-}"
SLACK_CHANNEL="${SLACK_CHANNEL:-}"
STANDUP_TEAM_MEMBERS="${STANDUP_TEAM_MEMBERS:-}"
ANTHROPIC_KEY="${ANTHROPIC_API_KEY:-}"
USE_BEDROCK="${CLAUDE_CODE_USE_BEDROCK:-}"
BEDROCK_URL="${ANTHROPIC_BEDROCK_BASE_URL:-http://ai/bedrock}"

# ── Argument parsing ───────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --server)             FORMICARY_URL="$2";        shift 2 ;;
    --set-configs)        SET_CONFIGS=true;          shift ;;
    --create-k8s-secret)  CREATE_K8S_SECRET=true;    shift ;;
    --gh-org)             GH_ORG="$2";              shift 2 ;;
    --gh-repo)       GH_REPO="$2";             shift 2 ;;
    --slack-channel) SLACK_CHANNEL="$2";        shift 2 ;;
    --team-members)  STANDUP_TEAM_MEMBERS="$2"; shift 2 ;;
    --bedrock)       USE_BEDROCK="1";           shift ;;
    --no-bedrock)    USE_BEDROCK="0";           shift ;;
    --bedrock-url)   BEDROCK_URL="$2";          shift 2 ;;
    --help|-h)
      sed -n '/^# Usage/,/^[^#]/p' "$0" | head -18
      exit 0 ;;
    --token|--gh-token|--slack-token|--anthropic-key)
      fail "Secrets must be provided via environment variables, not CLI flags (see --help)" ;;
    *) fail "Unknown option: $1" ;;
  esac
done

# ── Create K8s secret ─────────────────────────────────────────────────────────
create_k8s_secret() {
  log "Creating/updating 'ai-dev-credentials' Kubernetes secret ..."
  [[ -n "$GH_TOKEN" ]]    || fail "GH_TOKEN / GITHUB_TOKEN is required for --create-k8s-secret"
  [[ -n "$SLACK_TOKEN" ]] || fail "SLACK_BOT_TOKEN is required for --create-k8s-secret"

  kubectl create secret generic ai-dev-credentials \
    --from-literal=JIRA_BASE_URL="${JIRA_BASE_URL:-}" \
    --from-literal=JIRA_EMAIL="${JIRA_EMAIL:-}" \
    --from-literal=JIRA_API_TOKEN="${JIRA_API_TOKEN:-}" \
    --from-literal=JIRA_HOST="${JIRA_HOST:-}" \
    --from-literal=BITBUCKET_WORKSPACE="${BITBUCKET_WORKSPACE:-}" \
    --from-literal=BITBUCKET_USERNAME="${BITBUCKET_USERNAME:-}" \
    --from-literal=BITBUCKET_TOKEN="${BITBUCKET_TOKEN:-}" \
    --from-literal=GH_TOKEN="${GH_TOKEN}" \
    --from-literal=GH_ORG="${GH_ORG:-}" \
    --from-literal=GH_REPO="${GH_REPO:-}" \
    --from-literal=SLACK_BOT_TOKEN="${SLACK_TOKEN}" \
    --from-literal=ANTHROPIC_API_KEY="${ANTHROPIC_KEY:-}" \
    --from-literal=SSH_PRIVATE_KEY="${SSH_PRIVATE_KEY:-}" \
    --save-config --dry-run=client -o yaml | kubectl apply -f -
  ok "Secret 'ai-dev-credentials' created/updated."
}

# ── Shared helpers ─────────────────────────────────────────────────────────────
resolve_org_id() {
  [[ -z "$TOKEN" ]] && fail "FORMICARY_TOKEN is required to set configs"
  python3 -c "
import sys, json, base64
token = sys.argv[1]
parts = token.split('.')
if len(parts) != 3:
    sys.stderr.write('ERROR: FORMICARY_TOKEN is not a valid JWT\n'); sys.exit(1)
padding = 4 - len(parts[1]) % 4
payload = json.loads(base64.urlsafe_b64decode(parts[1] + '=' * padding))
oid = payload.get('org_id', '')
if not oid:
    sys.stderr.write('ERROR: JWT has no org_id\n'); sys.exit(1)
print(oid)
" "$TOKEN" || exit 1
}

_post_config() {
  local url="$1" name="$2" value="$3" secret_arg="${4:-auto}"
  local payload
  payload=$(python3 -c "
import json, sys, re
name, value, secret_arg = sys.argv[1], sys.argv[2], sys.argv[3]
if secret_arg == 'auto':
    secret = bool(re.search(r'(?i)(token|secret|key|password|api|credential|private)', name))
else:
    secret = secret_arg == 'true'
print(json.dumps({'name': name, 'value': value, 'secret': secret}))
" "$name" "$value" "$secret_arg") || fail "Failed to build JSON payload for $name"
  local args=(-sf -X POST "$url" -H "Content-Type: application/json" -d "$payload")
  [[ -n "$TOKEN" ]] && args+=(-H "Authorization: Bearer ${TOKEN}")
  local resp
  resp=$(curl "${args[@]}" 2>&1) || fail "Failed to set config $name: $resp"
  if echo "$resp" | grep -q '"name"'; then
    ok "Config set: $name"
  else
    echo "  Response: $resp" >&2; fail "Failed to set config $name"
  fi
}

set_org_config()  { _post_config "${FORMICARY_URL}/api/orgs/${ORG_ID}/configs" "$1" "$2" "${3:-auto}"; }
set_user_config() { _post_config "${FORMICARY_URL}/api/users/configs" "$1" "$2" "${3:-auto}"; }

upload() {
  local file="$1" name
  name=$(basename "$file")
  log "Uploading $name ..."
  local curl_args=(-s -o /tmp/formicary-upload-resp.json -w "%{http_code}"
                   -X POST "${FORMICARY_URL}/api/jobs/definitions"
                   -H "Content-Type: application/yaml"
                   --data-binary "@${file}")
  [[ -n "$TOKEN" ]] && curl_args+=(-H "Authorization: Bearer ${TOKEN}")
  local http_code response
  http_code=$(curl "${curl_args[@]}" 2>/dev/null) || true
  response=$(cat /tmp/formicary-upload-resp.json 2>/dev/null || true)
  rm -f /tmp/formicary-upload-resp.json
  case "$http_code" in
    2??) ;;
    401) fail "Upload failed for $name: 401 Unauthorized" ;;
    403) fail "Upload failed for $name: 403 Forbidden" ;;
    *)   echo "  HTTP $http_code: $response" >&2; fail "Upload failed for $name" ;;
  esac
  if echo "$response" | grep -q '"job_type"'; then
    local job_type
    job_type=$(echo "$response" | python3 -c "import sys,json; print(json.load(sys.stdin).get('job_type','?'))" 2>/dev/null || echo "?")
    ok "Registered job_type=$job_type"
  else
    echo "  Response: $response" >&2; fail "Unexpected response for $name"
  fi
}

# ── Validate token early ──────────────────────────────────────────────────────
[[ -n "$TOKEN" ]] || fail "FORMICARY_TOKEN is not set — export it before running this script"

# ── Create K8s secret (if requested) ──────────────────────────────────────────
if [[ "$CREATE_K8S_SECRET" == true ]]; then
  create_k8s_secret
  echo ""
fi

# ── Set configs ────────────────────────────────────────────────────────────────
if [[ "$SET_CONFIGS" == true ]]; then
  ORG_ID=$(resolve_org_id)

  [[ -n "$GH_ORG" ]]        || fail "--gh-org (or GH_ORG env) is required"
  [[ -n "$GH_REPO" ]]       || fail "--gh-repo (or GH_REPO env) is required"
  [[ -n "$SLACK_CHANNEL" ]] || fail "--slack-channel (or SLACK_CHANNEL env) is required"

  log "Setting org configs ..."
  set_org_config "GHOrg"               "$GH_ORG"       "false"
  set_org_config "GHRepo"              "$GH_REPO"      "false"
  set_org_config "SlackChannel" "$SLACK_CHANNEL" "false"
  [[ -n "$STANDUP_TEAM_MEMBERS" ]] && set_org_config "StandupTeamMembers" "$STANDUP_TEAM_MEMBERS" "false"
  if [[ -n "$USE_BEDROCK" ]]; then
    set_org_config "ClaudeUseBedrock"        "$USE_BEDROCK" "false"
    set_org_config "ClaudeSkipBedrockAuth"   "1"            "false"
    set_org_config "AnthropicBedrockBaseUrl" "$BEDROCK_URL" "false"
  fi

  echo ""; ok "Org configs set. (Credentials stored in K8s secret 'ai-dev-credentials'.)"; echo ""
fi

# ── Verify server ──────────────────────────────────────────────────────────────
log "Checking Formicary at ${FORMICARY_URL} ..."
_args=(-s -o /dev/null -w "%{http_code}" "${FORMICARY_URL}/api/jobs/definitions")
[[ -n "$TOKEN" ]] && _args+=(-H "Authorization: Bearer ${TOKEN}")
_status=$(curl "${_args[@]}" 2>&1) || _status="000"
case "$_status" in
  2*) ok "Server reachable (HTTP ${_status})" ;;
  000) fail "Cannot connect to ${FORMICARY_URL} — is the server running? (run: kubectl port-forward svc/formicary 7777:7777 19000:19000)" ;;
  401) fail "Server returned 401 — set FORMICARY_TOKEN" ;;
  403) fail "Server returned 403 — token invalid or expired" ;;
  *)   fail "Server returned HTTP ${_status}" ;;
esac

# ── Upload ─────────────────────────────────────────────────────────────────────
echo ""
log "Uploading standup workflow ..."
YAML="${SCRIPT_DIR}/ai-standup-gh.yaml"
[[ -f "$YAML" ]] || fail "File not found: $YAML"
upload "$YAML"
echo ""; ok "Standup (GitHub) workflow registered."

# ── List AI workflows ──────────────────────────────────────────────────────────
echo ""
log "Currently registered AI job types:"
_list_args=(-s "${FORMICARY_URL}/api/jobs/definitions")
[[ -n "$TOKEN" ]] && _list_args+=(-H "Authorization: Bearer ${TOKEN}")
curl "${_list_args[@]}" 2>/dev/null | python3 -c "
import sys, json
for d in json.load(sys.stdin).get('Records', []):
    jt = d.get('job_type','')
    if jt.startswith('ai-'):
        cron = d.get('cron_trigger','')
        print(f'  {jt:<42} cron={cron or \"-\"}')
" 2>/dev/null || true

# ── Next steps ─────────────────────────────────────────────────────────────────
echo ""
echo "────────────────────────────────────────────────────────────"
echo "Next steps:"
echo ""
echo "  1. Invite the Slack bot to your standup channel:"
echo "     /invite @<your-bot-name>   (in #${SLACK_CHANNEL:-standup})"
echo ""
echo "  2. Trigger manually (before first 8am cron):"
echo "     curl -X POST ${FORMICARY_URL}/api/jobs/requests \\"
echo "       -H 'Authorization: Bearer \${FORMICARY_TOKEN}' \\"
echo "       -H 'Content-Type: application/json' \\"
echo "       -d '{\"job_type\": \"ai-standup-gh\"}'"
echo ""
echo "  3. Watch jobs at: ${FORMICARY_URL}"
echo "────────────────────────────────────────────────────────────"
