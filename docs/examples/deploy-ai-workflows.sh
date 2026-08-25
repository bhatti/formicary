#!/usr/bin/env bash
# deploy-ai-workflows.sh
#
# Uploads the GitHub AI agent workflow YAMLs to a running Formicary queen
# and stores all required org configs (non-secret settings only).
#
# Usage:
#   ./deploy-ai-workflows.sh --create-k8s-secret
#   ./deploy-ai-workflows.sh --create-k8s-secret --set-configs \
#       --gh-org MY_ORG --gh-repo MY_REPO
#   ./deploy-ai-workflows.sh --set-configs --gh-org MY_ORG --gh-repo MY_REPO \
#       --bedrock --bedrock-url http://ai/bedrock \
#       --git-user "AI Agent" --git-email "ai@example.com"
#   ./deploy-ai-workflows.sh --server http://host:7777
#
# Credentials are stored in the 'ai-dev-credentials' Kubernetes secret.
# Use --create-k8s-secret to create/update the secret from env vars (one-time setup).
#
# Secrets MUST be supplied via environment variables — never as CLI flags:
#   FORMICARY_TOKEN   Formicary API token
#   GITHUB_TOKEN      GitHub personal access token (also GH_TOKEN)
#   SSH_PRIVATE_KEY   PEM-encoded SSH private key for git operations
#   ANTHROPIC_API_KEY Optional — for direct Anthropic API (not needed with Bedrock)
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

# Model ID defaults — override via env or by editing models.env.
ANTHROPIC_SONNET_MODEL="${ANTHROPIC_SONNET_MODEL:-us.anthropic.claude-sonnet-4-6}"
ANTHROPIC_OPUS_MODEL="${ANTHROPIC_OPUS_MODEL:-us.anthropic.claude-opus-4-6-v1}"
ANTHROPIC_HAIKU_MODEL="${ANTHROPIC_HAIKU_MODEL:-us.anthropic.claude-haiku-4-5-20251001-v1:0}"
# Complexity-tier defaults — can be overridden by models.env or env vars.
ANTHROPIC_COMPLEXITY_LOW_MODEL="${ANTHROPIC_COMPLEXITY_LOW_MODEL:-${ANTHROPIC_HAIKU_MODEL}}"
ANTHROPIC_COMPLEXITY_HIGH_MODEL="${ANTHROPIC_COMPLEXITY_HIGH_MODEL:-${ANTHROPIC_OPUS_MODEL}}"
# models.env can override the above defaults (sourced after so it wins).
[[ -f "$SCRIPT_DIR/models.env" ]] && source "$SCRIPT_DIR/models.env"

SET_CONFIGS=false
SETUP_LABELS=false
CREATE_K8S_SECRET=true
# Accept both GH_* and GITHUB_* prefixes; explicit flags override both
GH_ORG="${GH_ORG:-${GITHUB_ORG:-}}"
GH_REPO="${GH_REPO:-${GITHUB_REPO:-}}"
GH_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
ANTHROPIC_KEY="${ANTHROPIC_KEY:-${ANTHROPIC_API_KEY:-}}"
SSH_KEY="${SSH_KEY:-${SSH_PRIVATE_KEY:-}}"
SSH_KEY_FILE=""
USE_BEDROCK=""
BEDROCK_URL="${BEDROCK_URL:-${ANTHROPIC_BEDROCK_BASE_URL:-}}"
GIT_USER_NAME=""
GIT_USER_EMAIL=""
SLACK_CHANNEL="${SLACK_CHANNEL:-}"
STANDUP_TEAM="${STANDUP_TEAM_MEMBERS:-}"
JIRA_BOARDS_ARG="${JIRA_BOARDS:-}"
SET_SLACK_ROUTES=false

# ── Argument parsing ───────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --server)             FORMICARY_URL="$2";    shift 2 ;;
    --set-configs)        SET_CONFIGS=true;      shift ;;
    --set-slack-routes)   SET_SLACK_ROUTES=true; shift ;;
    --setup-labels)       SETUP_LABELS=true;     shift ;;
    --create-k8s-secret)  CREATE_K8S_SECRET=true; shift ;;
    --gh-org)        GH_ORG="$2";           shift 2 ;;
    --gh-repo)       GH_REPO="$2";          shift 2 ;;
    --bedrock)       USE_BEDROCK="1";       shift ;;
    --no-bedrock)    USE_BEDROCK="0";       shift ;;
    --bedrock-url)   BEDROCK_URL="$2";      shift 2 ;;
    --git-user)      GIT_USER_NAME="$2";    shift 2 ;;
    --git-email)     GIT_USER_EMAIL="$2";   shift 2 ;;
    --slack-channel)   SLACK_CHANNEL="$2";   shift 2 ;;
    --standup-team)    STANDUP_TEAM="$2";       shift 2 ;;
    --jira-boards)     JIRA_BOARDS_ARG="$2";    shift 2 ;;
    --help|-h)
      sed -n '/^# Usage/,/^[^#]/p' "$0" | head -22
      exit 0 ;;
    # Reject secret flags to prevent credentials leaking via ps/shell history.
    --token|--gh-token|--anthropic-key|--ssh-key|--ssh-key-file|--jira-token|--bb-token)
      fail "Secrets must be provided via environment variables, not CLI flags (see --help)" ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# Pass -k when connecting to an HTTPS server with a self-signed cert (nip.io / EC2 deployments).
CURL_OPTS=()
[[ "$FORMICARY_URL" == https://* ]] && CURL_OPTS+=(-k)

# ── Helpers ────────────────────────────────────────────────────────────────────
log()  { echo "▶ $*"; }
ok()   { echo "  ✓ $*"; }
fail() { echo "  ✗ ERROR: $*" >&2; echo "  ✗ ERROR: $*"; exit 1; }

# ── Create K8s secret ─────────────────────────────────────────────────────────
create_k8s_secret() {
  log "Creating/updating 'ai-dev-credentials' Kubernetes secret ..."
  [[ -n "$GH_TOKEN" ]] || fail "GITHUB_TOKEN / GH_TOKEN is required for --create-k8s-secret"

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
    --from-literal=SLACK_BOT_TOKEN="${SLACK_BOT_TOKEN:-}" \
    --from-literal=ANTHROPIC_API_KEY="${ANTHROPIC_KEY:-}" \
    --from-literal=SSH_PRIVATE_KEY="${SSH_KEY:-}" \
    --save-config --dry-run=client -o yaml | kubectl apply -f -
  ok "Secret 'ai-dev-credentials' created/updated."
}

resolve_org_id() {
  [[ -z "$TOKEN" ]] && fail "FORMICARY_TOKEN is required to resolve org ID"
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
  local args=(-s ${CURL_OPTS[@]+"${CURL_OPTS[@]}"} -o /tmp/formicary-config-resp.json -w "%{http_code}"
              -X POST "$url" -H "Content-Type: application/json" -d "$payload")
  [[ -n "$TOKEN" ]] && args+=(-H "Authorization: Bearer ${TOKEN}")
  local http_code resp
  http_code=$(curl "${args[@]}" 2>/dev/null) || http_code="000"
  resp=$(cat /tmp/formicary-config-resp.json 2>/dev/null || true)
  rm -f /tmp/formicary-config-resp.json
  case "$http_code" in
    2*)
      if echo "$resp" | grep -q '"name"'; then
        ok "Config set: $name"
      else
        echo "  Response: $resp" >&2
        fail "Failed to set config $name (HTTP $http_code)"
      fi ;;
    000) fail "Cannot connect to ${FORMICARY_URL} — is the server running and reachable?" ;;
    401) fail "Config $name: 401 Unauthorized — check FORMICARY_TOKEN" ;;
    403) fail "Config $name: 403 Forbidden — token lacks permission" ;;
    *)   echo "  Response: $resp" >&2; fail "Failed to set config $name (HTTP $http_code)" ;;
  esac
}

# set_org_config stores a shared team setting under the organisation.
set_org_config() {
  _post_config "${FORMICARY_URL}/api/orgs/${ORG_ID}/configs" "$1" "$2" "${3:-auto}"
}

# set_user_config stores a personal secret under the calling user's account.
set_user_config() {
  _post_config "${FORMICARY_URL}/api/users/configs" "$1" "$2" "${3:-auto}"
}

# set_admin_slack_routes pushes the Slack route table as an admin SystemConfig.
# Routes are instance-wide (not per-org) — requires an admin token.
# The queen reloads them on next startup (or restart).
# Value must be a JSON array of SlackRouteConfig objects:
#   [{"triggers":["standup"],"job_type":"ai-standup-jira","description":"..."},...]
set_admin_slack_routes() {
  local routes_json="$1"
  local url="${FORMICARY_URL}/api/v1/configs"
  local payload
  payload=$(python3 -c "
import json, sys
v = sys.argv[1]
# Validate it parses as JSON
json.loads(v)
print(json.dumps({'scope':'default','kind':'JSON','name':'SlackRoutes','value':v,'secret':False}))
" "$routes_json") || fail "SlackRoutes: invalid JSON array"
  local args=(-s ${CURL_OPTS[@]+"${CURL_OPTS[@]}"} -o /tmp/formicary-config-resp.json -w "%{http_code}"
              -X POST "$url" -H "Content-Type: application/json" -d "$payload")
  [[ -n "$TOKEN" ]] && args+=(-H "Authorization: Bearer ${TOKEN}")
  local http_code resp
  http_code=$(curl "${args[@]}" 2>/dev/null) || http_code="000"
  resp=$(cat /tmp/formicary-config-resp.json 2>/dev/null || true)
  rm -f /tmp/formicary-config-resp.json
  case "$http_code" in
    2*) ok "SlackRoutes admin config saved (restart queen to reload)" ;;
    000) fail "Cannot connect to ${FORMICARY_URL}" ;;
    401) fail "SlackRoutes: 401 Unauthorized — use an admin token" ;;
    403) fail "SlackRoutes: 403 Forbidden — admin role required" ;;
    *)   echo "  Response: $resp" >&2; fail "SlackRoutes: HTTP $http_code" ;;
  esac
}

upload() {
  local file="$1"
  local name
  name=$(basename "$file")
  log "Uploading $name ..."

  local curl_args=(-s ${CURL_OPTS[@]+"${CURL_OPTS[@]}"} -o /tmp/formicary-upload-resp.json -w "%{http_code}"
                   -X POST "${FORMICARY_URL}/api/jobs/definitions"
                   -H "Content-Type: application/yaml"
                   --data-binary "@${file}")
  [[ -n "$TOKEN" ]] && curl_args+=(-H "Authorization: Bearer ${TOKEN}")

  local http_code response
  http_code=$(curl "${curl_args[@]}" 2>/dev/null) || true
  response=$(cat /tmp/formicary-upload-resp.json 2>/dev/null || true)
  rm -f /tmp/formicary-upload-resp.json

  if [[ "$http_code" == 401 ]]; then
    echo "  HTTP 401 response: $response" >&2
    fail "Upload failed for $name: 401 Unauthorized — set FORMICARY_TOKEN or pass --token"
  elif [[ "$http_code" == 403 ]]; then
    echo "  HTTP 403 response: $response" >&2
    fail "Upload failed for $name: 403 — server error detail above"
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

# ── Resolve remaining fallbacks (after arg parsing may have overridden defaults) ─
BEDROCK_URL="${BEDROCK_URL:-http://ai/bedrock}"
if [[ -z "$SSH_KEY" && -n "$SSH_KEY_FILE" ]]; then
  SSH_KEY=$(cat "$SSH_KEY_FILE") || fail "Cannot read SSH key file: $SSH_KEY_FILE"
fi

# ── Validate token early ──────────────────────────────────────────────────────
[[ -n "$TOKEN" ]] || fail "FORMICARY_TOKEN is not set — export it before running this script"

# ── Verify server is reachable (fail fast before any API calls) ───────────────
log "Checking Formicary at ${FORMICARY_URL} ..."
_check_args=(-s ${CURL_OPTS[@]+"${CURL_OPTS[@]}"} -o /dev/null -w "%{http_code}" "${FORMICARY_URL}/api/jobs/definitions")
[[ -n "$TOKEN" ]] && _check_args+=(-H "Authorization: Bearer ${TOKEN}")
_check_status=$(curl "${_check_args[@]}" 2>/dev/null) || _check_status="000"
case "$_check_status" in
  2*) ok "Server reachable (HTTP ${_check_status})" ;;
  000) fail "Cannot connect to ${FORMICARY_URL} — is the server running and reachable?" ;;
  401) fail "Server returned 401 — check FORMICARY_TOKEN" ;;
  403) fail "Server returned 403 — token invalid or expired" ;;
  *)   fail "Server returned HTTP ${_check_status}" ;;
esac
echo ""

# ── Create K8s secret (if requested) ──────────────────────────────────────────
if [[ "$CREATE_K8S_SECRET" == true ]]; then
  create_k8s_secret
  echo ""
fi

# ── Resolve user's org ID (required for config storage) ───────────────────────
ORG_ID=$(resolve_org_id)

# ── Set org configs ────────────────────────────────────────────────────────────
if [[ "$SET_CONFIGS" == true ]]; then
  [[ -n "$GH_ORG" ]]  || fail "--set-configs requires --gh-org <org> (or GH_ORG env var)"
  [[ -n "$GH_REPO" ]] || fail "--set-configs requires --gh-repo <repo> (or GH_REPO env var)"

  log "Setting org configs (shared team settings) ..."
  set_org_config "GitHubOrg"       "$GH_ORG"
  set_org_config "GitHubRepo"      "$GH_REPO"
  set_org_config "DefaultTracker"  "github"
  if [[ -n "$USE_BEDROCK" ]]; then
    set_org_config "ClaudeUseBedrock"        "$USE_BEDROCK"
    set_org_config "ClaudeSkipBedrockAuth"   "1"
    set_org_config "AnthropicBedrockBaseUrl" "$BEDROCK_URL"
  fi
  # Push model IDs from models.env (single source of truth — overrides YAML job_variables).
  set_org_config "AnthropicSonnetModel" "${ANTHROPIC_SONNET_MODEL}" "false"
  set_org_config "AnthropicOpusModel"   "${ANTHROPIC_OPUS_MODEL}"   "false"
  set_org_config "AnthropicHaikuModel"  "${ANTHROPIC_HAIKU_MODEL}"  "false"
  # Complexity-tiered model selection (used by implement task based on plan_complexity.txt).
  set_org_config "AnthropicComplexityLowModel"  "${ANTHROPIC_COMPLEXITY_LOW_MODEL}"  "false"
  set_org_config "AnthropicComplexityHighModel" "${ANTHROPIC_COMPLEXITY_HIGH_MODEL}" "false"
  [[ -n "$GIT_USER_NAME" ]]  && set_org_config "GitUserName"      "$GIT_USER_NAME"
  [[ -n "$GIT_USER_EMAIL" ]] && set_org_config "GitUserEmail"     "$GIT_USER_EMAIL"
  [[ -n "$SLACK_CHANNEL" ]]  && set_org_config "SlackChannel"     "$SLACK_CHANNEL"     "false"
  [[ -n "$STANDUP_TEAM" ]]      && set_org_config "StandupTeamMembers" "$STANDUP_TEAM"     "false"
  [[ -n "$JIRA_BOARDS_ARG" ]]      && set_org_config "JiraBoards"         "$JIRA_BOARDS_ARG"      "false"
  [[ -n "${EXTRA_SKILLS_REPOS:-}" ]]   && set_org_config "ExtraSkillsRepos"   "${EXTRA_SKILLS_REPOS}"   "false"
  [[ -n "${MAX_CLAUDE_PROCESS_TIMEOUT:-}" ]] && set_org_config "MaxClaudeProcessTimeout" "${MAX_CLAUDE_PROCESS_TIMEOUT}" "false"

  echo ""
  ok "Org configs set. (Credentials stored in K8s secret 'ai-dev-credentials'.)"
  echo ""
else
  # Auto-set org configs from environment variables when --set-configs not passed.
  _AUTO=false
  if [[ -n "$GH_ORG" && -n "$GH_REPO" ]]; then
    [[ "$_AUTO" == false ]] && log "Auto-setting org configs from environment ..."
    set_org_config "GitHubOrg"  "$GH_ORG"
    set_org_config "GitHubRepo" "$GH_REPO"
    _AUTO=true
  fi
  if [[ -n "$SLACK_CHANNEL" ]]; then
    [[ "$_AUTO" == false ]] && log "Auto-setting org configs from environment ..."
    set_org_config "SlackChannel" "$SLACK_CHANNEL" "false"
    _AUTO=true
  fi
  if [[ -n "$STANDUP_TEAM" ]]; then
    [[ "$_AUTO" == false ]] && log "Auto-setting org configs from environment ..."
    set_org_config "StandupTeamMembers" "$STANDUP_TEAM" "false"
    _AUTO=true
  fi
  if [[ -n "$JIRA_BOARDS_ARG" ]]; then
    [[ "$_AUTO" == false ]] && log "Auto-setting org configs from environment ..."
    set_org_config "JiraBoards" "$JIRA_BOARDS_ARG" "false"
    _AUTO=true
  fi
  if [[ -n "${EXTRA_SKILLS_REPOS:-}" ]]; then
    [[ "$_AUTO" == false ]] && log "Auto-setting org configs from environment ..."
    set_org_config "ExtraSkillsRepos" "${EXTRA_SKILLS_REPOS}" "false"
    _AUTO=true
  fi
  if [[ -n "${MAX_CLAUDE_PROCESS_TIMEOUT:-}" ]]; then
    [[ "$_AUTO" == false ]] && log "Auto-setting org configs from environment ..."
    set_org_config "MaxClaudeProcessTimeout" "${MAX_CLAUDE_PROCESS_TIMEOUT}" "false"
    _AUTO=true
  fi
  [[ "$_AUTO" == true ]] && echo ""
fi

# NOTE: Slack bot/app tokens and route table are admin-level (shared across orgs).
# Use setup-slack-admin.sh for those — this script only handles org-level configs.
# See: ./docs/examples/setup-slack-admin.sh --help

# ── Upload workflows ───────────────────────────────────────────────────────────
YAMLS=(
  "${SCRIPT_DIR}/ai-connectivity-check.yaml"
  "${SCRIPT_DIR}/ai-standup-gh.yaml"
  "${SCRIPT_DIR}/ai-standup-jira.yaml"
  "${SCRIPT_DIR}/ai-gh-issue-picker.yaml"
  "${SCRIPT_DIR}/ai-gh-implement.yaml"
  "${SCRIPT_DIR}/ai-gh-cleanup.yaml"
  "${SCRIPT_DIR}/ai-gh-review.yaml"
  "${SCRIPT_DIR}/ai-jira-query.yaml"
  "${SCRIPT_DIR}/ai-adhoc.yaml"
)

echo ""
log "Uploading ${#YAMLS[@]} workflow definition(s) ..."
for f in "${YAMLS[@]}"; do
  [[ -f "$f" ]] || fail "File not found: $f"
  upload "$f"
done

echo ""
ok "All workflows registered."

# ── List registered AI workflows ───────────────────────────────────────────────
echo ""
log "Currently registered AI job types:"
_list_args=(-s ${CURL_OPTS[@]+"${CURL_OPTS[@]}"} "${FORMICARY_URL}/api/jobs/definitions")
[[ -n "$TOKEN" ]] && _list_args+=(-H "Authorization: Bearer ${TOKEN}")
curl "${_list_args[@]}" 2>/dev/null | python3 -c "
import sys, json
defs = json.load(sys.stdin).get('Records', [])
for d in defs:
    jt = d.get('job_type','')
    if jt.startswith('ai-'):
        cron = d.get('cron_trigger','')
        conc = d.get('max_concurrency','')
        print(f'  {jt:<35} cron={cron or \"-\":<20} max_concurrency={conc}')
" 2>/dev/null || true

# ── Optional: create GitHub labels ─────────────────────────────────────────────
if [[ "$SETUP_LABELS" == true ]]; then
  [[ -n "$GH_ORG" && -n "$GH_REPO" ]] || fail "--setup-labels requires --gh-org and --gh-repo"
  FULL_REPO="${GH_ORG}/${GH_REPO}"
  echo ""
  log "Creating GitHub labels in ${FULL_REPO} ..."
  create_label() {
    local name="$1" color="$2" desc="$3"
    if gh label create "$name" --repo "$FULL_REPO" --color "$color" --description "$desc" 2>/dev/null; then
      ok "Created: $name"
    else
      ok "Already exists: $name"
    fi
  }
  create_label "ai-ready"       "0075ca" "Ready for AI agent"
  create_label "ai-in-progress" "e4e669" "AI agent working on this"
  create_label "ai-pr-open"     "0e8a16" "AI agent opened a PR"
  create_label "needs-human"    "d93f0b" "AI was blocked — needs human"
fi

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
echo "     export GITHUB_TOKEN=<token>"
echo "     export SSH_PRIVATE_KEY=\$(cat ~/.ssh/id_rsa)"
echo "     export BEDROCK_URL=http://ai/bedrock   # or set ANTHROPIC_API_KEY for direct API"
echo "     export SLACK_BOT_TOKEN=xoxb-...       # optional: Slack notifications"
echo "     $0 --create-k8s-secret --set-configs --gh-org YOUR_ORG --gh-repo YOUR_REPO --bedrock --slack-channel my-channel
#    (Org-based routing is automatic when auth is enabled — no --ant-user-tag needed)"
echo ""
echo "  3. Create GitHub labels (if not done):"
echo "     $0 --setup-labels --gh-org YOUR_ORG --gh-repo YOUR_REPO"
echo ""
echo "  4. Label an issue to trigger the picker:"
echo "     gh issue edit <N> --repo YOUR_ORG/YOUR_REPO --add-label 'ai-ready'"
echo ""
echo "  5. Watch jobs at: ${FORMICARY_URL}"
echo "────────────────────────────────────────────────────────────"
