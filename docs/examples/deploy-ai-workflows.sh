#!/usr/bin/env bash
# deploy-ai-workflows.sh
#
# Uploads the four AI agent workflow YAMLs to a running Formicary queen.
# Defaults to SHELL variants (no Kubernetes required).
#
# Usage:
#   ./deploy-ai-workflows.sh                          # SHELL, no auth, localhost:7777
#   ./deploy-ai-workflows.sh --mode k8s               # Kubernetes variants
#   ./deploy-ai-workflows.sh --server http://host:7777
#   ./deploy-ai-workflows.sh --server http://host:7777 --token <TOKEN>  # if auth enabled
#
# Prerequisites:
#   - Formicary queen running (make run)
#   - gh auth login done on the ant worker host (for SHELL mode)
#   - GitHub labels created in target repo (see --setup-labels flag)
#   - Org configs set: GitHubOrg, GitHubRepo, GithubToken, AnthropicApiKey
#     (no FormicaryToken/FormicaryURL needed — jobs submit via DB template functions)
#
set -euo pipefail

# ── Defaults ──────────────────────────────────────────────────────────────────
FORMICARY_URL="${FORMICARY_URL:-http://localhost:7777}"
TOKEN="${FORMICARY_TOKEN:-}"          # only needed when auth is enabled
MODE="shell"          # shell | k8s
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GH_REPO=""
SETUP_LABELS=false
SET_CONFIGS=false
GH_ORG=""
GH_TOKEN=""
ANTHROPIC_KEY=""

# ── Argument parsing ───────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --server)        FORMICARY_URL="$2";  shift 2 ;;
    --token)         TOKEN="$2";          shift 2 ;;
    --mode)          MODE="$2";           shift 2 ;;
    --repo)          GH_REPO="$2";        shift 2 ;;
    --setup-labels)  SETUP_LABELS=true;   shift ;;
    --set-configs)   SET_CONFIGS=true;    shift ;;
    --gh-org)        GH_ORG="$2";         shift 2 ;;
    --gh-repo)       GH_REPO="$2";        shift 2 ;;
    --gh-token)      GH_TOKEN="$2";       shift 2 ;;
    --anthropic-key) ANTHROPIC_KEY="$2";  shift 2 ;;
    --help|-h)
      sed -n '/^# Usage/,/^[^#]/p' "$0" | head -12
      exit 0 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# ── Helpers ────────────────────────────────────────────────────────────────────
log()  { echo "▶ $*"; }
ok()   { echo "  ✓ $*"; }
fail() { echo "  ✗ $*" >&2; exit 1; }

set_config() {
  local name="$1" value="$2" secret="${3:-false}"
  local payload
  payload=$(printf '{"name":"%s","value":"%s","secret":%s}' "$name" "$value" "$secret")
  local args=(-sf -X POST "${FORMICARY_URL}/api/orgs/default/configs"
              -H "Content-Type: application/json"
              -d "$payload")
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
    fail "Upload failed for $name: 401 Unauthorized — FORMICARY_TOKEN is missing, expired, or from a different server. Get a fresh token from ${FORMICARY_URL}/dashboard/users and re-export it."
  elif [[ "$http_code" == 403 ]]; then
    fail "Upload failed for $name: 403 Forbidden — token is valid but lacks permission."
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

# ── Pick YAML set ──────────────────────────────────────────────────────────────
case "$MODE" in
  shell)
    SUFFIX="-shell"
    log "Mode: SHELL (ant worker runs scripts on host, inherits ~/.claude/settings.json)"
    ;;
  k8s|kubernetes)
    SUFFIX=""
    log "Mode: KUBERNETES (pods spawned per task)"
    ;;
  *)
    fail "Unknown mode '$MODE' — use 'shell' or 'k8s'"
    ;;
esac

YAMLS=(
  "${SCRIPT_DIR}/ai-gh-issue-picker${SUFFIX}.yaml"
  "${SCRIPT_DIR}/ai-gh-implement${SUFFIX}.yaml"
  "${SCRIPT_DIR}/ai-gh-pr-feedback${SUFFIX}.yaml"
  "${SCRIPT_DIR}/ai-gh-cleanup${SUFFIX}.yaml"
)

# ── Set org configs (--set-configs or auto from environment) ──────────────────
# When --set-configs is passed, explicit flags are required.
# When only some vars are available via environment (GITHUB_TOKEN, ANTHROPIC_API_KEY),
# those are set automatically without --set-configs.
if [[ "$SET_CONFIGS" == true ]]; then
  # Explicit mode: fall back to environment variables when flags not provided
  GH_ORG="${GH_ORG:-}"
  GH_REPO="${GH_REPO:-}"
  GH_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
  ANTHROPIC_KEY="${ANTHROPIC_KEY:-${ANTHROPIC_API_KEY:-}}"

  [[ -n "$GH_ORG" ]]       || fail "--set-configs requires --gh-org <org>"
  [[ -n "$GH_REPO" ]]      || fail "--set-configs requires --gh-repo <repo>"
  [[ -n "$GH_TOKEN" ]]     || fail "--set-configs requires --gh-token <token> or GITHUB_TOKEN env var"
  [[ -n "$ANTHROPIC_KEY" ]] || fail "--set-configs requires --anthropic-key <key> or ANTHROPIC_API_KEY env var"

  log "Setting org configs on ${FORMICARY_URL}/api/orgs/default/configs ..."
  set_config "GitHubOrg"      "$GH_ORG"       "false"
  set_config "GitHubRepo"     "$GH_REPO"       "false"
  set_config "GithubToken"    "$GH_TOKEN"      "true"
  set_config "AnthropicApiKey" "$ANTHROPIC_KEY" "true"
  echo ""
  ok "Org configs set. No FormicaryToken or FormicaryURL needed."
  echo ""
else
  # Auto-set from environment: set any config where the env var is present
  _AUTO_SET=false
  if [[ -n "${GITHUB_TOKEN:-}" && -n "${GH_ORG:-}" && -n "${GH_REPO:-}" ]]; then
    log "Auto-setting GitHub org configs from environment ..."
    set_config "GitHubOrg"   "${GH_ORG}"      "false"
    set_config "GitHubRepo"  "${GH_REPO}"      "false"
    set_config "GithubToken" "${GITHUB_TOKEN}" "true"
    _AUTO_SET=true
  fi
  if [[ -n "${ANTHROPIC_API_KEY:-}" ]]; then
    [[ "$_AUTO_SET" == false ]] && log "Auto-setting Anthropic config from environment ..."
    set_config "AnthropicApiKey" "${ANTHROPIC_API_KEY}" "true"
    _AUTO_SET=true
  fi
  [[ "$_AUTO_SET" == true ]] && echo ""
fi

# ── Verify server is reachable ─────────────────────────────────────────────────
log "Checking Formicary at ${FORMICARY_URL} ..."
_health_args=(-s -o /dev/null -w "%{http_code}" "${FORMICARY_URL}/api/jobs/definitions")
[[ -n "$TOKEN" ]] && _health_args+=(-H "Authorization: Bearer ${TOKEN}")
_http_status=$(curl "${_health_args[@]}" 2>/dev/null || echo "000")
case "$_http_status" in
  2*) ok "Server reachable (HTTP ${_http_status})" ;;
  000) fail "Cannot connect to ${FORMICARY_URL} — is the server running?" ;;
  401) if [[ -z "$TOKEN" ]]; then
         fail "Server returned 401 — auth is enabled but FORMICARY_TOKEN is not set. Export your API token: export FORMICARY_TOKEN=<token>"
       else
         fail "Server returned 401 — token rejected. It may be expired or for a different server. Get a fresh token from ${FORMICARY_URL}/dashboard/users and re-export FORMICARY_TOKEN."
       fi ;;
  403) fail "Server returned 403 — token is invalid or expired. Get a fresh token from the UI: ${FORMICARY_URL}/dashboard/users and update FORMICARY_TOKEN in ~/.zshrc." ;;
  *)   fail "Server returned HTTP ${_http_status} — unexpected response from ${FORMICARY_URL}" ;;
esac
ok "Server reachable"

# ── Upload workflows ───────────────────────────────────────────────────────────
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
  [[ -n "$GH_REPO" ]] || fail "--setup-labels requires --repo <org>/<repo>"
  echo ""
  log "Creating GitHub labels in ${GH_REPO} ..."

  create_label() {
    local name="$1" color="$2" desc="$3"
    if gh label create "$name" --repo "$GH_REPO" --color "$color" --description "$desc" 2>/dev/null; then
      ok "Created: $name"
    else
      ok "Already exists (or failed): $name"
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
echo "  1. Set org configs (GitHubOrg, GitHubRepo, GithubToken, AnthropicApiKey):"
echo "     # Easiest: export env vars then pass --set-configs:"
echo "     #   export GITHUB_TOKEN=ghp_..."
echo "     #   $0 --set-configs --gh-org YOUR_ORG --gh-repo YOUR_REPO"
echo ""
echo "     # Or curl directly (GithubToken read from \$GITHUB_TOKEN):"
echo "     curl -X POST ${FORMICARY_URL}/api/orgs/default/configs \\"
echo "       -H 'Content-Type: application/json' \\"
echo "       -d '{\"name\":\"GitHubOrg\",\"value\":\"YOUR_ORG\"}'"
echo ""
echo "     curl -X POST ${FORMICARY_URL}/api/orgs/default/configs \\"
echo "       -H 'Content-Type: application/json' \\"
echo "       -d '{\"name\":\"GitHubRepo\",\"value\":\"YOUR_REPO\"}'"
echo ""
echo "     curl -X POST ${FORMICARY_URL}/api/orgs/default/configs \\"
echo "       -H 'Content-Type: application/json' \\"
echo "       -d \"{\\\"name\\\":\\\"GithubToken\\\",\\\"value\\\":\\\"${GITHUB_TOKEN:-ghp_...}\\\",\\\"secret\\\":true}\""
echo ""
echo "     curl -X POST ${FORMICARY_URL}/api/orgs/default/configs \\"
echo "       -H 'Content-Type: application/json' \\"
echo "       -d '{\"name\":\"AnthropicApiKey\",\"value\":\"sk-ant-...\",\"secret\":true}'"
echo ""
echo "  2. Create GitHub labels (if not done):"
echo "     $0 --setup-labels --repo YOUR_ORG/YOUR_REPO"
echo ""
echo "  3. Label an issue to trigger the picker:"
echo "     gh issue edit <N> --repo YOUR_ORG/YOUR_REPO --add-label 'ai-ready'"
echo ""
echo "  4. Watch jobs at: ${FORMICARY_URL}"
echo "────────────────────────────────────────────────────────────"
