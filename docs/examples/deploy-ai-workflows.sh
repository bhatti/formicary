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

# ── Argument parsing ───────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --server)             FORMICARY_URL="$2";    shift 2 ;;
    --set-configs)        SET_CONFIGS=true;      shift ;;
    --setup-labels)       SETUP_LABELS=true;     shift ;;
    --create-k8s-secret)  CREATE_K8S_SECRET=true; shift ;;
    --gh-org)        GH_ORG="$2";           shift 2 ;;
    --gh-repo)       GH_REPO="$2";          shift 2 ;;
    --bedrock)       USE_BEDROCK="1";       shift ;;
    --no-bedrock)    USE_BEDROCK="0";       shift ;;
    --bedrock-url)   BEDROCK_URL="$2";      shift 2 ;;
    --git-user)      GIT_USER_NAME="$2";    shift 2 ;;
    --git-email)     GIT_USER_EMAIL="$2";   shift 2 ;;
    --help|-h)
      sed -n '/^# Usage/,/^[^#]/p' "$0" | head -22
      exit 0 ;;
    # Reject secret flags to prevent credentials leaking via ps/shell history.
    --token|--gh-token|--anthropic-key|--ssh-key|--ssh-key-file|--jira-token|--bb-token)
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

# set_user_config stores a personal secret under the calling user's account.
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
  set_org_config "GitHubOrg"  "$GH_ORG"
  set_org_config "GitHubRepo" "$GH_REPO"
  if [[ -n "$USE_BEDROCK" ]]; then
    set_org_config "ClaudeUseBedrock"        "$USE_BEDROCK"
    set_org_config "ClaudeSkipBedrockAuth"   "1"
    set_org_config "AnthropicBedrockBaseUrl" "$BEDROCK_URL"
  fi
  [[ -n "$GIT_USER_NAME" ]]  && set_org_config "GitUserName"  "$GIT_USER_NAME"
  [[ -n "$GIT_USER_EMAIL" ]] && set_org_config "GitUserEmail" "$GIT_USER_EMAIL"

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
  "${SCRIPT_DIR}/ai-gh-issue-picker.yaml"
  "${SCRIPT_DIR}/ai-gh-implement.yaml"
  "${SCRIPT_DIR}/ai-gh-cleanup.yaml"
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
echo "     $0 --create-k8s-secret --set-configs --gh-org YOUR_ORG --gh-repo YOUR_REPO --bedrock"
echo ""
echo "  3. Create GitHub labels (if not done):"
echo "     $0 --setup-labels --gh-org YOUR_ORG --gh-repo YOUR_REPO"
echo ""
echo "  4. Label an issue to trigger the picker:"
echo "     gh issue edit <N> --repo YOUR_ORG/YOUR_REPO --add-label 'ai-ready'"
echo ""
echo "  5. Watch jobs at: ${FORMICARY_URL}"
echo "────────────────────────────────────────────────────────────"
