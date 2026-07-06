#!/usr/bin/env bash
# deploy-ai-jira-workflows.sh
#
# Uploads the Jira/Bitbucket AI agent workflow YAMLs to a running Formicary queen
# and stores all required org configs (tokens, keys, settings).
#
# Reads Jira and Bitbucket credentials from ~/.config/acli/config.json by default.
#
# Usage:
#   ./deploy-ai-jira-workflows.sh
#   ./deploy-ai-jira-workflows.sh --set-configs \
#       --jira-project MYPROJ \
#       --bb-workspace myworkspace --bb-repo myrepo
#   ./deploy-ai-jira-workflows.sh --set-configs \
#       --jira-project MYPROJ \
#       --bb-workspace myworkspace --bb-repo myrepo \
#       --bedrock --bedrock-url http://ai/bedrock \
#       --git-user "AI Agent" --git-email "ai@example.com"
#   ./deploy-ai-jira-workflows.sh --server http://host:7777
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

# ── Argument parsing ───────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --server)        FORMICARY_URL="$2";    shift 2 ;;
    --set-configs)   SET_CONFIGS=true;      shift ;;
    --jira-url)      JIRA_URL="$2";         shift 2 ;;
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
    --help|-h)
      sed -n '/^# Usage/,/^[^#]/p' "$0" | head -24
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
fail() { echo "  ✗ $*" >&2; exit 1; }

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

# ── Resolve user's org ID (required for config storage) ───────────────────────
ORG_ID=$(resolve_org_id)

# ── Set org configs ────────────────────────────────────────────────────────────
if [[ "$SET_CONFIGS" == true ]]; then
  # Required: Jira
  [[ -n "$JIRA_URL" ]]       || fail "JiraUrl is required — set via --jira-url, JIRA_BASE_URL env, or ~/.config/acli/config.json"
  [[ -n "$JIRA_EMAIL" ]]     || fail "JiraEmail is required — set via --jira-email, JIRA_EMAIL env, or ~/.config/acli/config.json"
  [[ -n "$JIRA_API_TOKEN" ]] || fail "JiraApiToken is required — set via --jira-token, JIRA_API_TOKEN env, or ~/.config/acli/config.json"
  [[ -n "$JIRA_PROJECT" ]]   || fail "JiraProject is required — set via --jira-project, JIRA_PROJECT env, or ~/.config/acli/config.json"

  # Required: Bitbucket
  [[ -n "$BB_WORKSPACE" ]]   || fail "BitbucketWorkspace is required — set via --bb-workspace, BITBUCKET_WORKSPACE env, or ~/.config/acli/config.json"
  [[ -n "$BB_REPO" ]]        || fail "BitbucketRepo is required — set via --bb-repo or BITBUCKET_REPO env"
  # NOTE: BitbucketUsername must be the account email (e.g. user@example.com), NOT the
  # nickname — Bitbucket REST API v2 requires email for Basic Auth with app passwords.
  [[ -n "$BB_USERNAME" ]]    || fail "BitbucketUsername is required — set via --bb-username, BITBUCKET_USERNAME env (use email, not nickname), or ~/.config/acli/config.json"
  [[ -n "$BB_TOKEN" ]]       || fail "BitbucketToken is required — set via --bb-token, BITBUCKET_TOKEN env, or ~/.config/acli/config.json"

  log "Setting org configs (shared team settings) ..."
  set_org_config "JiraUrl"             "$JIRA_URL"
  set_org_config "JiraProject"         "$JIRA_PROJECT"
  set_org_config "BitbucketWorkspace"  "$BB_WORKSPACE"
  set_org_config "BitbucketRepo"       "$BB_REPO"
  if [[ -n "$USE_BEDROCK" ]]; then
    set_org_config "ClaudeUseBedrock"        "$USE_BEDROCK"
    set_org_config "ClaudeSkipBedrockAuth"   "1"
    set_org_config "AnthropicBedrockBaseUrl" "$BEDROCK_URL"
  fi

  log "Setting user configs (personal secrets and identity) ..."
  set_user_config "JiraEmail"       "$JIRA_EMAIL"
  set_user_config "JiraApiToken"    "$JIRA_API_TOKEN"
  set_user_config "BitbucketUsername" "$BB_USERNAME"
  set_user_config "BitbucketToken"  "$BB_TOKEN"
  [[ -n "$ANTHROPIC_KEY" ]] && set_user_config "AnthropicApiKey" "$ANTHROPIC_KEY"
  [[ -n "$SSH_KEY" ]]        && set_user_config "SshPrivateKey"   "$SSH_KEY"
  [[ -n "$GIT_USER_NAME" ]]  && set_user_config "GitUserName"     "$GIT_USER_NAME"
  [[ -n "$GIT_USER_EMAIL" ]] && set_user_config "GitUserEmail"    "$GIT_USER_EMAIL"

  echo ""
  ok "Configs set."
  echo ""
else
  # Auto-set from environment variables when --set-configs not passed.
  _AUTO=false
  if [[ -n "$JIRA_API_TOKEN" && -n "$JIRA_URL" && -n "$JIRA_EMAIL" ]]; then
    [[ "$_AUTO" == false ]] && log "Auto-setting configs from environment ..."
    set_org_config  "JiraUrl"      "$JIRA_URL"
    set_user_config "JiraEmail"    "$JIRA_EMAIL"
    set_user_config "JiraApiToken" "$JIRA_API_TOKEN"
    [[ -n "$JIRA_PROJECT" ]] && set_org_config "JiraProject" "$JIRA_PROJECT"
    _AUTO=true
  fi
  if [[ -n "$BB_TOKEN" && -n "$BB_WORKSPACE" && -n "$BB_USERNAME" ]]; then
    [[ "$_AUTO" == false ]] && log "Auto-setting configs from environment ..."
    set_org_config  "BitbucketWorkspace" "$BB_WORKSPACE"
    set_user_config "BitbucketUsername"  "$BB_USERNAME"
    set_user_config "BitbucketToken"     "$BB_TOKEN"
    [[ -n "$BB_REPO" ]] && set_org_config "BitbucketRepo" "$BB_REPO"
    _AUTO=true
  fi
  if [[ -n "$ANTHROPIC_KEY" ]]; then
    [[ "$_AUTO" == false ]] && log "Auto-setting configs from environment ..."
    set_user_config "AnthropicApiKey" "$ANTHROPIC_KEY"
    _AUTO=true
  fi
  if [[ -n "$SSH_KEY" ]]; then
    set_user_config "SshPrivateKey" "$SSH_KEY"
    _AUTO=true
  fi
  [[ "$_AUTO" == true ]] && echo ""
fi

# ── Verify server is reachable ─────────────────────────────────────────────────
log "Checking Formicary at ${FORMICARY_URL} ..."
_args=(-s -o /dev/null -w "%{http_code}" "${FORMICARY_URL}/api/jobs/definitions")
[[ -n "$TOKEN" ]] && _args+=(-H "Authorization: Bearer ${TOKEN}")
_status=$(curl "${_args[@]}" 2>/dev/null || echo "000")
case "$_status" in
  2*) ok "Server reachable (HTTP ${_status})" ;;
  000) fail "Cannot connect to ${FORMICARY_URL} — is the server running?" ;;
  401) fail "Server returned 401 — set FORMICARY_TOKEN or pass --token" ;;
  403) fail "Server returned 403 — token invalid or expired" ;;
  *) fail "Server returned HTTP ${_status}" ;;
esac

# ── Upload workflows ───────────────────────────────────────────────────────────
YAMLS=(
  "${SCRIPT_DIR}/ai-jira-issue-picker.yaml"
  "${SCRIPT_DIR}/ai-jira-implement.yaml"
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
echo "  2. Set org configs (if not done above):"
echo "     export FORMICARY_TOKEN=<token>"
echo "     export JIRA_API_TOKEN=<token>  # or use ~/.config/acli/config.json"
echo "     export BITBUCKET_TOKEN=<token>  # or use ~/.config/acli/config.json"
echo "     export SSH_PRIVATE_KEY=\$(cat ~/.ssh/id_rsa)"
echo "     export BEDROCK_URL=http://ai/bedrock   # or ANTHROPIC_API_KEY for direct API"
echo "     $0 --set-configs --jira-project MYPROJ --bb-workspace myworkspace --bb-repo myrepo --bedrock"
echo ""
echo "  3. Add the pickup label to a Jira issue:"
echo "     acli jira issue label add <ISSUE-KEY> ai-ready"
echo ""
echo "  4. Watch jobs at: ${FORMICARY_URL}"
echo "────────────────────────────────────────────────────────────"
