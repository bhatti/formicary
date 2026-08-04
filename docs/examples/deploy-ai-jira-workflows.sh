#!/usr/bin/env bash
# deploy-ai-jira-workflows.sh
#
# Uploads the Jira/Bitbucket AI agent workflow YAMLs to a running Formicary queen
# and stores all required org configs (non-secret settings only).
#
# Reads Jira and Bitbucket credentials from ~/.config/acli/config.json by default.
#
# Usage:
#   ./deploy-ai-jira-workflows.sh --create-k8s-secret
#   ./deploy-ai-jira-workflows.sh --create-k8s-secret --set-configs \
#       --jira-project MYPROJ \
#       --bb-workspace myworkspace --bb-repo myrepo
#   ./deploy-ai-jira-workflows.sh --set-configs \
#       --jira-project MYPROJ \
#       --bb-workspace myworkspace --bb-repo myrepo \
#       --bedrock --bedrock-url http://ai/bedrock \
#       --git-user "AI Agent" --git-email "ai@example.com"
#   ./deploy-ai-jira-workflows.sh --server http://host:7777
#
# Credentials are stored in the 'ai-dev-credentials' Kubernetes secret.
# Use --create-k8s-secret to create/update the secret from env vars (one-time setup).
#
# Secrets MUST be supplied via environment variables — never as CLI flags:
#   FORMICARY_TOKEN        Formicary API token
#   JIRA_API_TOKEN         Jira API token  (also read from ~/.config/acli/config.json)
#   JIRA_BASE_URL          Jira base URL   (also read from ~/.config/acli/config.json)
#   JIRA_EMAIL             Jira user email (also read from ~/.config/acli/config.json)
#   BITBUCKET_TOKEN        Bitbucket app password (also read from ~/.config/acli/config.json)
#   BITBUCKET_USERNAME     Bitbucket username     (also read from ~/.config/acli/config.json)
#   SSH_PRIVATE_KEY        PEM-encoded SSH private key for git operations
#   ANTHROPIC_API_KEY      Optional — for direct Anthropic API (not needed with Bedrock)
#   ANTHROPIC_BEDROCK_BASE_URL  Bedrock proxy URL (also BEDROCK_URL)
#
# AnthropicApiKey is optional — Claude is accessed via Bedrock through Tailscale VPN.
# Set ANTHROPIC_API_KEY only if using direct Anthropic API instead of Bedrock.
#
set -euo pipefail

# ── Defaults ──────────────────────────────────────────────────────────────────
FORMICARY_URL="${FORMICARY_URL:-http://localhost:7777}"
TOKEN="${FORMICARY_TOKEN:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ACLI_CONFIG="${HOME}/.config/acli/config.json"

SET_CONFIGS=false
CREATE_K8S_SECRET=true
# Accept explicit flags, fall back to env vars matching the scripts' expected names
JIRA_URL="${JIRA_BASE_URL:-${JIRA_URL:-}}"
JIRA_EMAIL_ARG="${JIRA_EMAIL:-}"
JIRA_TOKEN_ARG="${JIRA_API_TOKEN:-}"
JIRA_PROJECT="${JIRA_PROJECT:-}"
BB_WORKSPACE="${BITBUCKET_WORKSPACE:-}"
BB_REPO="${BITBUCKET_REPO:-}"
BB_USERNAME_ARG="${BITBUCKET_USERNAME:-}"
BB_TOKEN_ARG="${BITBUCKET_TOKEN:-}"
ANTHROPIC_KEY="${ANTHROPIC_KEY:-${ANTHROPIC_API_KEY:-}}"
SSH_KEY="${SSH_KEY:-${SSH_PRIVATE_KEY:-}}"
SSH_KEY_FILE=""
USE_BEDROCK="${CLAUDE_CODE_USE_BEDROCK:-}"
BEDROCK_URL="${ANTHROPIC_BEDROCK_BASE_URL:-http://ai/bedrock}"
GIT_USER_NAME="${GIT_USER_NAME:-}"
GIT_USER_EMAIL="${GIT_USER_EMAIL:-}"
SLACK_CHANNEL="${SLACK_CHANNEL:-}"

# ── Argument parsing ───────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --server)             FORMICARY_URL="$2";    shift 2 ;;
    --set-configs)        SET_CONFIGS=true;      shift ;;
    --create-k8s-secret)  CREATE_K8S_SECRET=true; shift ;;
    --jira-url)           JIRA_URL="$2";         shift 2 ;;
    --jira-email)    JIRA_EMAIL_ARG="$2";   shift 2 ;;
    --jira-project)  JIRA_PROJECT="$2";     shift 2 ;;
    --bb-workspace)  BB_WORKSPACE="$2";     shift 2 ;;
    --bb-repo)       BB_REPO="$2";          shift 2 ;;
    --bb-username)   BB_USERNAME_ARG="$2";  shift 2 ;;
    --bedrock)       USE_BEDROCK="1";       shift ;;
    --no-bedrock)    USE_BEDROCK="0";       shift ;;
    --bedrock-url)   BEDROCK_URL="$2";      shift 2 ;;
    --git-user)      GIT_USER_NAME="$2";    shift 2 ;;
    --git-email)     GIT_USER_EMAIL="$2";   shift 2 ;;
    --slack-channel) SLACK_CHANNEL="$2";    shift 2 ;;
    --help|-h)
      sed -n '/^# Usage/,/^[^#]/p' "$0" | head -26
      exit 0 ;;
    # Reject secret flags to prevent credentials leaking via ps/shell history.
    --token|--jira-token|--bb-token|--anthropic-key|--ssh-key|--ssh-key-file|--gh-token)
      fail "Secrets must be provided via environment variables, not CLI flags (see --help)" ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# ── Helpers ────────────────────────────────────────────────────────────────────
log()  { echo "▶ $*"; }
ok()   { echo "  ✓ $*"; }
fail() { echo "  ✗ ERROR: $*" >&2; echo "  ✗ ERROR: $*"; exit 1; }

# ── Create K8s secret ─────────────────────────────────────────────────────────
create_k8s_secret() {
  log "Creating/updating 'ai-dev-credentials' Kubernetes secret ..."
  [[ -n "$JIRA_URL" ]]       || fail "JIRA_BASE_URL is required for --create-k8s-secret"
  [[ -n "$JIRA_EMAIL" ]]     || fail "JIRA_EMAIL is required for --create-k8s-secret"
  [[ -n "$JIRA_API_TOKEN" ]] || fail "JIRA_API_TOKEN is required for --create-k8s-secret"

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
    --from-literal=SLACK_BOT_TOKEN="${SLACK_BOT_TOKEN:-}" \
    --from-literal=ANTHROPIC_API_KEY="${ANTHROPIC_KEY:-}" \
    --from-literal=SSH_PRIVATE_KEY="${SSH_KEY:-}" \
    --save-config --dry-run=client -o yaml | kubectl apply -f -
  ok "Secret 'ai-dev-credentials' created/updated."
}

resolve_org_id() {
  [[ -z "$TOKEN" ]] && fail "FORMICARY_TOKEN is required to resolve org ID"
  # Decode org_id from JWT payload (base64url, no network call needed).
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
    sys.stderr.write('ERROR: JWT has no org_id — token may be stale, re-generate it from the UI\n')
    sys.exit(1)
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
    echo "  Response: $resp" >&2
    fail "Failed to set config $name"
  fi
}

# set_org_config stores a shared team setting under the organisation.
set_org_config() {
  _post_config "${FORMICARY_URL}/api/orgs/${ORG_ID}/configs" "$1" "$2" "${3:-auto}"
}

# set_user_config stores a personal secret/identity under the calling user's account.
set_user_config() {
  _post_config "${FORMICARY_URL}/api/users/configs" "$1" "$2" "${3:-auto}"
}

upload() {
  local file="$1"
  local name
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

  if [[ "$http_code" == 401 ]]; then
    fail "Upload failed for $name: 401 Unauthorized — set FORMICARY_TOKEN or pass --token"
  elif [[ "$http_code" == 403 ]]; then
    fail "Upload failed for $name: 403 Forbidden — token lacks permission"
  elif [[ "$http_code" != 2?? ]]; then
    echo "  HTTP $http_code: $response" >&2
    fail "Upload failed for $name (HTTP $http_code)"
  fi

  local job_type
  job_type=$(echo "$response" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('job_type','?'))" 2>/dev/null || echo "?")
  if echo "$response" | grep -q '"job_type"'; then
    ok "Registered job_type=$job_type"
  else
    echo "  Response: $response" >&2
    fail "Upload failed for $name — unexpected response"
  fi
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
  [[ -n "$JIRA_URL_CFG" ]]       && ok "JiraUrl=$JIRA_URL_CFG"
  [[ -n "$JIRA_EMAIL_CFG" ]]     && ok "JiraEmail=$JIRA_EMAIL_CFG"
  [[ -n "$JIRA_API_TOKEN_CFG" ]] && ok "JiraApiToken=***"
  [[ -n "$BB_API_TOKEN_CFG" ]]   && ok "BitbucketToken=***"
  [[ -n "$BB_USERNAME_CFG" ]]    && ok "BitbucketUsername=$BB_USERNAME_CFG"
  [[ -n "$BB_WORKSPACE_CFG" ]]   && ok "BitbucketWorkspace=$BB_WORKSPACE_CFG"
  [[ -n "$JIRA_PROJECT" ]]       && ok "JiraProject=$JIRA_PROJECT"
else
  log "No acli config found at $ACLI_CONFIG — skipping credential read"
fi

# Merge: env vars > acli config
JIRA_URL="${JIRA_URL:-${JIRA_URL_CFG}}"
JIRA_EMAIL="${JIRA_EMAIL_ARG:-${JIRA_EMAIL_CFG}}"
JIRA_API_TOKEN="${JIRA_API_TOKEN:-${JIRA_API_TOKEN_CFG}}"
BB_USERNAME="${BB_USERNAME_ARG:-${BB_USERNAME_CFG}}"
BB_TOKEN="${BITBUCKET_TOKEN:-${BB_API_TOKEN_CFG}}"
BB_WORKSPACE="${BB_WORKSPACE:-${BB_WORKSPACE_CFG}}"

# ── Validate token early ──────────────────────────────────────────────────────
[[ -n "$TOKEN" ]] || fail "FORMICARY_TOKEN is not set — export it before running this script"

# ── Create K8s secret (if requested) ──────────────────────────────────────────
if [[ "$CREATE_K8S_SECRET" == true ]]; then
  create_k8s_secret
  echo ""
fi

# ── Resolve user's org ID (required for config storage) ───────────────────────
ORG_ID=$(resolve_org_id)

# ── Set org configs ────────────────────────────────────────────────────────────
if [[ "$SET_CONFIGS" == true ]]; then
  # Required: Jira
  [[ -n "$JIRA_URL" ]]     || fail "JiraUrl is required — set via --jira-url, JIRA_BASE_URL env, or ~/.config/acli/config.json"

  # Required: Bitbucket
  [[ -n "$BB_WORKSPACE" ]] || fail "BitbucketWorkspace is required — set via --bb-workspace, BITBUCKET_WORKSPACE env, or ~/.config/acli/config.json"
  [[ -n "$BB_REPO" ]]      || fail "BitbucketRepo is required — set via --bb-repo or BITBUCKET_REPO env"

  log "Setting org configs (shared team settings) ..."
  set_org_config "JiraUrl"            "$JIRA_URL"
  # JiraProject is optional — Python gather script auto-discovers it via the Jira API.
  [[ -n "$JIRA_PROJECT" ]] && set_org_config "JiraProject" "$JIRA_PROJECT"
  set_org_config "BitbucketWorkspace" "$BB_WORKSPACE"
  set_org_config "BitbucketRepo"      "$BB_REPO"
  set_org_config "DefaultTracker"     "jira"
  if [[ -n "$USE_BEDROCK" ]]; then
    set_org_config "ClaudeUseBedrock"        "$USE_BEDROCK"
    set_org_config "ClaudeSkipBedrockAuth"   "1"
    set_org_config "AnthropicBedrockBaseUrl" "$BEDROCK_URL"
  fi
  [[ -n "$GIT_USER_NAME" ]]  && set_org_config "GitUserName"  "$GIT_USER_NAME"
  [[ -n "$GIT_USER_EMAIL" ]] && set_org_config "GitUserEmail" "$GIT_USER_EMAIL"
  [[ -n "$SLACK_CHANNEL" ]]  && set_org_config "SlackChannel" "$SLACK_CHANNEL" "false"

  echo ""
  ok "Org configs set. (Credentials stored in K8s secret 'ai-dev-credentials'.)"
  echo ""
else
  # Auto-set org configs from environment variables when --set-configs not passed.
  _AUTO=false
  if [[ -n "$JIRA_URL" ]]; then
    [[ "$_AUTO" == false ]] && log "Auto-setting org configs from environment ..."
    set_org_config "JiraUrl" "$JIRA_URL"
    [[ -n "$JIRA_PROJECT" ]] && set_org_config "JiraProject" "$JIRA_PROJECT"
    _AUTO=true
  fi
  if [[ -n "$BB_WORKSPACE" ]]; then
    [[ "$_AUTO" == false ]] && log "Auto-setting org configs from environment ..."
    set_org_config "BitbucketWorkspace" "$BB_WORKSPACE"
    [[ -n "$BB_REPO" ]] && set_org_config "BitbucketRepo" "$BB_REPO"
    _AUTO=true
  fi
  if [[ -n "$SLACK_CHANNEL" ]]; then
    [[ "$_AUTO" == false ]] && log "Auto-setting org configs from environment ..."
    set_org_config "SlackChannel" "$SLACK_CHANNEL" "false"
    _AUTO=true
  fi
  [[ "$_AUTO" == true ]] && echo ""
fi

# ── Verify server is reachable ─────────────────────────────────────────────────
log "Checking Formicary at ${FORMICARY_URL} ..."
_args=(-s -o /dev/null -w "%{http_code}" "${FORMICARY_URL}/api/jobs/definitions")
[[ -n "$TOKEN" ]] && _args+=(-H "Authorization: Bearer ${TOKEN}")
_status=$(curl "${_args[@]}" 2>&1) || _status="000"
case "$_status" in
  2*) ok "Server reachable (HTTP ${_status})" ;;
  000) fail "Cannot connect to ${FORMICARY_URL} — is the server running? (run: kubectl port-forward svc/formicary 7777:7777 19000:19000)" ;;
  401) fail "Server returned 401 — set FORMICARY_TOKEN or pass --token" ;;
  403) fail "Server returned 403 — token invalid or expired" ;;
  *) fail "Server returned HTTP ${_status}" ;;
esac

# ── Upload workflows ───────────────────────────────────────────────────────────
YAMLS=(
  "${SCRIPT_DIR}/ai-connectivity-check.yaml"
  "${SCRIPT_DIR}/ai-jira-issue-picker.yaml"
  "${SCRIPT_DIR}/ai-jira-implement.yaml"
  "${SCRIPT_DIR}/ai-jira-review.yaml"
  "${SCRIPT_DIR}/ai-adhoc.yaml"
)

echo ""
log "Uploading ${#YAMLS[@]} workflow definition(s) ..."
for f in "${YAMLS[@]}"; do
  [[ -f "$f" ]] || fail "File not found: $f"
  upload "$f"
done

echo ""
ok "All Jira workflows registered."

# ── List registered AI workflows ───────────────────────────────────────────────
echo ""
log "Currently registered AI job types:"
_list_args=(-s "${FORMICARY_URL}/api/jobs/definitions")
[[ -n "$TOKEN" ]] && _list_args+=(-H "Authorization: Bearer ${TOKEN}")
curl "${_list_args[@]}" 2>/dev/null | python3 -c "
import sys, json
defs = json.load(sys.stdin).get('Records', [])
for d in defs:
    jt = d.get('job_type','')
    if jt.startswith('ai-'):
        cron = d.get('cron_trigger','')
        conc = d.get('max_concurrency','')
        print(f'  {jt:<40} cron={cron or \"-\":<20} max_concurrency={conc}')
" 2>/dev/null || true

# ── Next steps ─────────────────────────────────────────────────────────────────
echo ""
echo "────────────────────────────────────────────────────────────"
echo "Next steps:"
echo ""
echo "  1. Ensure workspace directory exists on every ant worker host:"
echo "     sudo mkdir -p /var/formicary/ai-workspace"
echo "     sudo chmod 777 /var/formicary/ai-workspace"
echo ""
echo "  2. Create K8s secret (one-time setup) and set org configs:"
echo "     export FORMICARY_TOKEN=<token>"
echo "     export JIRA_API_TOKEN=<token>  # or use ~/.config/acli/config.json"
echo "     export BITBUCKET_TOKEN=<token>  # or use ~/.config/acli/config.json"
echo "     export SSH_PRIVATE_KEY=\$(cat ~/.ssh/id_rsa)"
echo "     export BEDROCK_URL=http://ai/bedrock   # or ANTHROPIC_API_KEY for direct API"
echo "     export SLACK_BOT_TOKEN=xoxb-...       # optional: Slack notifications"
echo "     $0 --create-k8s-secret --set-configs --bb-workspace myworkspace --bb-repo myrepo --bedrock --slack-channel my-channel"
echo ""
echo "  3. Add the pickup label to a Jira issue:"
echo "     acli jira issue label add <ISSUE-KEY> ai-ready"
echo ""
echo "  4. Watch jobs at: ${FORMICARY_URL}"
echo "────────────────────────────────────────────────────────────"
