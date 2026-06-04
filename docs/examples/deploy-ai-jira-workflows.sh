#!/usr/bin/env bash
# deploy-ai-jira-workflows.sh
#
# Uploads the Jira/Bitbucket AI agent workflow YAMLs to a running Formicary queen.
# Reads Jira credentials from ~/.config/acli/config.json by default.
#
# Usage:
#   ./deploy-ai-jira-workflows.sh                          # SHELL, localhost:7777
#   ./deploy-ai-jira-workflows.sh --mode k8s               # Kubernetes variants
#   ./deploy-ai-jira-workflows.sh --server http://host:7777
#   ./deploy-ai-jira-workflows.sh --server http://host:7777 --token <TOKEN>
#   ./deploy-ai-jira-workflows.sh --set-configs \
#       --jira-project MYPROJ \
#       --bb-workspace myworkspace --bb-repo myrepo \
#       --anthropic-key sk-ant-... \
#       --git-user "Bot" --git-email "bot@example.com"
#
set -euo pipefail

FORMICARY_URL="${FORMICARY_URL:-http://localhost:7777}"
TOKEN="${FORMICARY_TOKEN:-}"
MODE="shell"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ACLI_CONFIG="${HOME}/.config/acli/config.json"

SET_CONFIGS=false
JIRA_PROJECT=""
BB_WORKSPACE=""
BB_REPO=""
ANTHROPIC_KEY=""
GIT_USER_NAME=""
GIT_USER_EMAIL=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --server)        FORMICARY_URL="$2";  shift 2 ;;
    --token)         TOKEN="$2";          shift 2 ;;
    --mode)          MODE="$2";           shift 2 ;;
    --set-configs)   SET_CONFIGS=true;    shift ;;
    --jira-project)  JIRA_PROJECT="$2";   shift 2 ;;
    --bb-workspace)  BB_WORKSPACE="$2";   shift 2 ;;
    --bb-repo)       BB_REPO="$2";        shift 2 ;;
    --anthropic-key) ANTHROPIC_KEY="$2";  shift 2 ;;
    --git-user)      GIT_USER_NAME="$2";  shift 2 ;;
    --git-email)     GIT_USER_EMAIL="$2"; shift 2 ;;
    --help|-h)
      sed -n '/^# Usage/,/^[^#]/p' "$0" | head -12
      exit 0 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

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

  # Patch job_variables with values read from acli config
  local patched
  patched=$(python3 - "$file" <<PYEOF
import sys, re

path = sys.argv[1]
with open(path) as f:
    content = f.read()

patches = {
    'JiraProject':        '${JIRA_PROJECT}',
    'BitbucketWorkspace': '${BB_WORKSPACE}',
    'BitbucketRepo':      '${BB_REPO}',
}

for key, val in patches.items():
    if not val:
        continue
    # Replace  key: "anything"  under job_variables
    content = re.sub(
        rf'(\b{key}:\s*")[^"]*(")',
        rf'\g<1>{val}\g<2>',
        content
    )

print(content, end='')
PYEOF
  )

  local tmpfile
  tmpfile=$(mktemp /tmp/deploy-jira-XXXXXX)
  mv "$tmpfile" "${tmpfile}.yaml"
  tmpfile="${tmpfile}.yaml"
  printf '%s' "$patched" > "$tmpfile"

  local args=(-sf -X POST "${FORMICARY_URL}/api/jobs/definitions"
              -H "Content-Type: application/yaml"
              --data-binary "@${tmpfile}")
  [[ -n "$TOKEN" ]] && args+=(-H "Authorization: Bearer ${TOKEN}")

  local response
  response=$(curl "${args[@]}" 2>&1)
  rm -f "$tmpfile"
  [[ $? -ne 0 ]] && fail "curl failed for $name: $response"

  local job_type
  job_type=$(echo "$response" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('job_type','?'))" 2>/dev/null || echo "?")

  if echo "$response" | grep -q '"job_type"'; then
    ok "Registered job_type=$job_type"
  else
    echo "  Response: $response" >&2
    fail "Upload failed for $name"
  fi
}

case "$MODE" in
  shell)
    SUFFIX="-shell"
    log "Mode: SHELL"
    ;;
  k8s|kubernetes)
    SUFFIX=""
    log "Mode: KUBERNETES"
    ;;
  *)
    fail "Unknown mode '$MODE' — use 'shell' or 'k8s'"
    ;;
esac

YAMLS=(
  "${SCRIPT_DIR}/ai-jira-issue-picker${SUFFIX}.yaml"
  "${SCRIPT_DIR}/ai-jira-implement${SUFFIX}.yaml"
)

# ── Read acli config ───────────────────────────────────────────────────────────
if [[ -f "$ACLI_CONFIG" ]]; then
  log "Reading credentials from $ACLI_CONFIG ..."
  JIRA_URL=$(python3 -c "
import json, sys
c = json.load(open('$ACLI_CONFIG'))
p = c['profiles'].get(c.get('default_profile','jira'), {})
print(p.get('atlassian_url',''))
" 2>/dev/null || true)
  JIRA_EMAIL=$(python3 -c "
import json, sys
c = json.load(open('$ACLI_CONFIG'))
p = c['profiles'].get(c.get('default_profile','jira'), {})
print(p.get('email',''))
" 2>/dev/null || true)
  JIRA_API_TOKEN=$(python3 -c "
import json, sys
c = json.load(open('$ACLI_CONFIG'))
p = c['profiles'].get(c.get('default_profile','jira'), {})
print(p.get('api_token',''))
" 2>/dev/null || true)
  BB_API_TOKEN=$(python3 -c "
import json, sys
c = json.load(open('$ACLI_CONFIG'))
p = c['profiles'].get('bitbucket', {})
print(p.get('api_token',''))
" 2>/dev/null || true)
  # Pull default project/workspace from acli config if not supplied via flags
  if [[ -z "$JIRA_PROJECT" ]]; then
    JIRA_PROJECT=$(python3 -c "
import json, sys
c = json.load(open('$ACLI_CONFIG'))
p = c['profiles'].get(c.get('default_profile','jira'), {})
print(p.get('defaults', {}).get('project',''))
" 2>/dev/null || true)
  fi
  if [[ -z "$BB_WORKSPACE" ]]; then
    BB_WORKSPACE=$(python3 -c "
import json, sys
c = json.load(open('$ACLI_CONFIG'))
p = c['profiles'].get('bitbucket', {})
print(p.get('defaults', {}).get('workspace',''))
" 2>/dev/null || true)
  fi
  [[ -n "$JIRA_URL" ]]       && ok "JiraUrl=$JIRA_URL"
  [[ -n "$JIRA_EMAIL" ]]     && ok "JiraEmail=$JIRA_EMAIL"
  [[ -n "$JIRA_API_TOKEN" ]] && ok "JiraApiToken=***"
  [[ -n "$BB_API_TOKEN" ]]   && ok "BitbucketApiToken=***"
  [[ -n "$JIRA_PROJECT" ]]   && ok "JiraProject=$JIRA_PROJECT"
  [[ -n "$BB_WORKSPACE" ]]   && ok "BitbucketWorkspace=$BB_WORKSPACE"
else
  log "No acli config found at $ACLI_CONFIG — skipping credential read"
fi

# ── Set org configs ────────────────────────────────────────────────────────────
if [[ "$SET_CONFIGS" == true ]]; then
  [[ -n "$JIRA_PROJECT" ]]   || fail "--set-configs requires --jira-project <key>"
  [[ -n "$BB_WORKSPACE" ]]   || fail "--set-configs requires --bb-workspace <workspace>"
  [[ -n "$BB_REPO" ]]        || fail "--set-configs requires --bb-repo <repo>"
  [[ -n "$ANTHROPIC_KEY" ]]  || fail "--set-configs requires --anthropic-key <key>"

  log "Setting org configs on ${FORMICARY_URL}/api/orgs/default/configs ..."

  [[ -n "$JIRA_URL" ]]       && set_config "JiraUrl"          "$JIRA_URL"       "false"
  [[ -n "$JIRA_EMAIL" ]]     && set_config "JiraEmail"        "$JIRA_EMAIL"     "false"
  [[ -n "$JIRA_API_TOKEN" ]] && set_config "JiraApiToken"     "$JIRA_API_TOKEN" "true"
  [[ -n "$BB_API_TOKEN" ]]   && set_config "BitbucketApiToken" "$BB_API_TOKEN"  "true"

  set_config "JiraProject"       "$JIRA_PROJECT"  "false"
  set_config "BitbucketWorkspace" "$BB_WORKSPACE" "false"
  set_config "BitbucketRepo"     "$BB_REPO"       "false"
  set_config "AnthropicApiKey"   "$ANTHROPIC_KEY" "true"

  [[ -n "$GIT_USER_NAME" ]]  && set_config "GitUserName"  "$GIT_USER_NAME"  "false"
  [[ -n "$GIT_USER_EMAIL" ]] && set_config "GitUserEmail" "$GIT_USER_EMAIL" "false"

  echo ""
fi

# ── Verify server ──────────────────────────────────────────────────────────────
log "Checking Formicary at ${FORMICARY_URL} ..."
curl -sf "${FORMICARY_URL}/api/jobs/definitions" -o /dev/null \
  || fail "Cannot reach ${FORMICARY_URL} — is 'make run' running?"
ok "Server reachable"

# ── Upload ─────────────────────────────────────────────────────────────────────
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
CURL_ARGS=(-sf "${FORMICARY_URL}/api/jobs/definitions")
[[ -n "$TOKEN" ]] && CURL_ARGS+=(-H "Authorization: Bearer ${TOKEN}")
curl "${CURL_ARGS[@]}" \
  | python3 -c "
import sys, json
defs = json.load(sys.stdin).get('Records', [])
for d in defs:
    jt = d.get('job_type','')
    if jt.startswith('ai-'):
        cron = d.get('cron_trigger','')
        conc = d.get('max_concurrency','')
        print(f'  {jt:<40} cron={cron or \"-\":<20} max_concurrency={conc}')
"

# ── Next steps ─────────────────────────────────────────────────────────────────
echo ""
echo "────────────────────────────────────────────────────────────"
echo "Next steps:"
echo ""
echo "  1. Set org configs (if not done via --set-configs):"
echo "     $0 --set-configs \\"
echo "         --jira-project MYPROJ \\"
echo "         --bb-workspace myworkspace --bb-repo myrepo \\"
echo "         --anthropic-key sk-ant-... \\"
echo "         --git-user 'AI Bot' --git-email 'ai@example.com'"
echo ""
echo "  2. Add the pickup label to a Jira issue:"
echo "     acli jira issue label add <ISSUE-KEY> ai-ready"
echo ""
echo "  3. Watch jobs at: ${FORMICARY_URL}"
echo "────────────────────────────────────────────────────────────"
