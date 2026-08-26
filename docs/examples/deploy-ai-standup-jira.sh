#!/usr/bin/env bash
# deploy-ai-standup-jira.sh
#
# Uploads the ai-standup-jira.yaml workflow to a running Formicary queen and
# stores all required org configs for the Jira + Bitbucket + Slack standup.
#
# Reads Jira/Bitbucket credentials from ~/.config/acli/config.json by default
# (same as deploy-ai-jira-workflows.sh).
#
# Usage:
#   ./deploy-ai-standup-jira.sh --create-k8s-secret
#   ./deploy-ai-standup-jira.sh --create-k8s-secret --set-configs \
#       --slack-channel YOUR_CHANNEL
#   ./deploy-ai-standup-jira.sh --set-configs \
#       --slack-channel YOUR_CHANNEL --team-members "Alice,Bob"
#   ./deploy-ai-standup-jira.sh --server http://host:7777 --set-configs \
#       --slack-channel YOUR_CHANNEL
#
#   JiraProject is auto-discovered from the Jira API — no need to pass it.
#
# Credentials are stored in the 'ai-dev-credentials' Kubernetes secret.
# Use --create-k8s-secret to create/update the secret from env vars (one-time setup).
#
# Secrets MUST be supplied via environment variables — never as CLI flags:
#   FORMICARY_TOKEN        Formicary API token
#   JIRA_API_TOKEN         Jira API token  (or ~/.config/acli/config.json)
#   JIRA_BASE_URL          Jira base URL   (or ~/.config/acli/config.json)
#   JIRA_EMAIL             Jira account email (or ~/.config/acli/config.json)
#   BITBUCKET_TOKEN        Bitbucket app password (optional; or acli config)
#   BITBUCKET_USERNAME     Bitbucket email (optional; or acli config)
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
ACLI_CONFIG="${HOME}/.config/acli/config.json"

SET_CONFIGS=false
CREATE_K8S_SECRET=true
JIRA_URL="${JIRA_BASE_URL:-${JIRA_URL:-}}"
JIRA_EMAIL_ARG="${JIRA_EMAIL:-}"
JIRA_TOKEN_ARG="${JIRA_API_TOKEN:-}"
JIRA_PROJECT="${JIRA_PROJECT:-}"
BB_WORKSPACE="${BITBUCKET_WORKSPACE:-}"
BB_REPO="${BITBUCKET_REPO:-}"
BB_USERNAME_ARG="${BITBUCKET_USERNAME:-}"
BB_TOKEN_ARG="${BITBUCKET_TOKEN:-}"
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
    --jira-project)       JIRA_PROJECT="$2";         shift 2 ;;
    --jira-url)      JIRA_URL="$2";             shift 2 ;;
    --jira-email)    JIRA_EMAIL_ARG="$2";       shift 2 ;;
    --bb-workspace)  BB_WORKSPACE="$2";         shift 2 ;;
    --bb-repo)       BB_REPO="$2";              shift 2 ;;
    --bb-username)   BB_USERNAME_ARG="$2";      shift 2 ;;
    --slack-channel) SLACK_CHANNEL="$2";        shift 2 ;;
    --team-members)  STANDUP_TEAM_MEMBERS="$2"; shift 2 ;;
    --bedrock)       USE_BEDROCK="1";           shift ;;
    --no-bedrock)    USE_BEDROCK="0";           shift ;;
    --bedrock-url)   BEDROCK_URL="$2";          shift 2 ;;
    --help|-h)
      sed -n '/^# Usage/,/^[^#]/p' "$0" | head -20
      exit 0 ;;
    --token|--jira-token|--bb-token|--slack-token|--anthropic-key)
      fail "Secrets must be provided via environment variables, not CLI flags (see --help)" ;;
    *) fail "Unknown option: $1" ;;
  esac
done

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
    fail "Failed to set config $name: $resp"
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

# ── Create K8s secret ─────────────────────────────────────────────────────────
create_k8s_secret() {
  log "Creating/updating 'ai-dev-credentials' Kubernetes secret ..."
  [[ -n "$JIRA_URL" ]]       || fail "JIRA_BASE_URL (or --jira-url) is required for --create-k8s-secret"
  [[ -n "$JIRA_EMAIL" ]]     || fail "JIRA_EMAIL is required for --create-k8s-secret"
  [[ -n "$JIRA_API_TOKEN" ]] || fail "JIRA_API_TOKEN is required for --create-k8s-secret"
  [[ -n "$SLACK_TOKEN" ]]    || fail "SLACK_BOT_TOKEN is required for --create-k8s-secret"

  kubectl create secret generic ai-dev-credentials \
    --from-literal=JIRA_BASE_URL="${JIRA_URL}" \
    --from-literal=JIRA_EMAIL="${JIRA_EMAIL}" \
    --from-literal=JIRA_API_TOKEN="${JIRA_API_TOKEN}" \
    --from-literal=JIRA_HOST="${JIRA_HOST:-}" \
    --from-literal=BITBUCKET_WORKSPACE="${BB_WORKSPACE:-}" \
    --from-literal=BITBUCKET_USERNAME="${BB_USERNAME:-}" \
    --from-literal=BITBUCKET_TOKEN="${BB_TOKEN:-}" \
    --from-literal=GH_TOKEN="${GITHUB_TOKEN:-${GH_TOKEN:-}}" \
    --from-literal=GH_ORG="${GH_ORG:-}" \
    --from-literal=GH_REPO="${GH_REPO:-}" \
    --from-literal=SLACK_BOT_TOKEN="${SLACK_TOKEN}" \
    --from-literal=ANTHROPIC_API_KEY="${ANTHROPIC_KEY:-}" \
    --from-literal=SSH_PRIVATE_KEY="${SSH_PRIVATE_KEY:-}" \
    --save-config --dry-run=client -o yaml | kubectl apply -f -
  ok "Secret 'ai-dev-credentials' created/updated."
}

# ── Read acli config ───────────────────────────────────────────────────────────
JIRA_URL_CFG="" JIRA_EMAIL_CFG="" JIRA_API_TOKEN_CFG=""
BB_API_TOKEN_CFG="" BB_USERNAME_CFG="" BB_WORKSPACE_CFG=""

if [[ -f "$ACLI_CONFIG" ]]; then
  log "Reading credentials from $ACLI_CONFIG ..."
  _py_read() {
    python3 -c "
import json, sys
c = json.load(open('$ACLI_CONFIG'))
p = c['profiles'].get(c.get('default_profile','jira'), {})
bb = c['profiles'].get('bitbucket', {})
print('JIRA_URL=' + p.get('atlassian_url',''))
print('JIRA_EMAIL=' + p.get('email',''))
print('JIRA_API_TOKEN=' + p.get('api_token',''))
print('BB_USERNAME=' + (bb.get('username','') or bb.get('email','')))
print('BB_API_TOKEN=' + bb.get('api_token',''))
print('BB_WORKSPACE=' + bb.get('defaults',{}).get('workspace',''))
print('JIRA_PROJECT=' + p.get('defaults',{}).get('project',''))
" 2>/dev/null || true
  }
  while IFS='=' read -r key val; do
    case "$key" in
      JIRA_URL)       JIRA_URL_CFG="$val" ;;
      JIRA_EMAIL)     JIRA_EMAIL_CFG="$val" ;;
      JIRA_API_TOKEN) JIRA_API_TOKEN_CFG="$val" ;;
      BB_USERNAME)    BB_USERNAME_CFG="$val" ;;
      BB_API_TOKEN)   BB_API_TOKEN_CFG="$val" ;;
      BB_WORKSPACE)   BB_WORKSPACE_CFG="$val" ;;
      JIRA_PROJECT)   [[ -z "$JIRA_PROJECT" ]] && JIRA_PROJECT="$val" ;;
    esac
  done < <(_py_read)
fi

# Merge: flags/env > acli config
JIRA_URL="${JIRA_URL:-${JIRA_URL_CFG}}"
JIRA_EMAIL="${JIRA_EMAIL_ARG:-${JIRA_EMAIL_CFG}}"
JIRA_API_TOKEN="${JIRA_TOKEN_ARG:-${JIRA_API_TOKEN_CFG}}"
BB_USERNAME="${BB_USERNAME_ARG:-${BB_USERNAME_CFG}}"
BB_TOKEN="${BB_TOKEN_ARG:-${BB_API_TOKEN_CFG}}"
BB_WORKSPACE="${BB_WORKSPACE:-${BB_WORKSPACE_CFG}}"

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

  [[ -n "$JIRA_URL" ]]      || fail "JiraUrl is required (--jira-url, JIRA_BASE_URL, or acli config)"
  [[ -n "$SLACK_CHANNEL" ]] || fail "--slack-channel (or SLACK_CHANNEL env) is required"

  log "Setting org configs ..."
  set_org_config "JiraUrl"             "$JIRA_URL"      "false"
  # JiraProject is optional — Python gather script auto-discovers it via the Jira API.
  [[ -n "$JIRA_PROJECT" ]] && set_org_config "JiraProject" "$JIRA_PROJECT" "false"
  set_org_config "SlackChannel" "$SLACK_CHANNEL" "false"
  [[ -n "$BB_WORKSPACE" ]]          && set_org_config "BitbucketWorkspace"  "$BB_WORKSPACE"          "false"
  [[ -n "$BB_REPO" ]]               && set_org_config "BitbucketRepo"       "$BB_REPO"               "false"
  [[ -n "$STANDUP_TEAM_MEMBERS" ]]  && set_org_config "StandupTeamMembers"  "$STANDUP_TEAM_MEMBERS"  "false"
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
  2??) ok "Server reachable (HTTP ${_status})" ;;
  000) fail "Cannot connect to ${FORMICARY_URL} — is the server running? (run: kubectl port-forward svc/formicary 7777:7777 19000:19000)" ;;
  401) fail "Server returned 401 Unauthorized — set FORMICARY_TOKEN env var" ;;
  403) fail "Server returned 403 Forbidden — token invalid or expired" ;;
  *)   fail "Server returned HTTP ${_status}" ;;
esac

# ── Upload ─────────────────────────────────────────────────────────────────────
echo ""
log "Uploading standup workflow ..."
YAML="${SCRIPT_DIR}/ai-standup-jira.yaml"
[[ -f "$YAML" ]] || fail "File not found: $YAML"
upload "$YAML"
echo ""; ok "Standup (Jira) workflow registered."

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
echo "       -d '{\"job_type\": \"ai-standup-jira\"}'"
echo ""
echo "  3. Watch jobs at: ${FORMICARY_URL}"
echo "────────────────────────────────────────────────────────────"
