#!/usr/bin/env bash
# setup-user-creds.sh — register issue-tracker and VCS credentials with the Formicary queen.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES_DIR="$(cd "${SCRIPT_DIR}/../docs/examples" && pwd)"

# ── Helpers ───────────────────────────────────────────────────────────────────
log()  { echo "  ▶ $*"; }
ok()   { echo "  ✓ $*"; }
warn() { echo "  ⚠ $*" >&2; }
fail() { echo "  ✗ ERROR: $*" >&2; exit 1; }

usage() {
  cat <<EOF
USAGE
  $(basename "$0") [OPTIONS] [TRACKER]

DESCRIPTION
  Uploads issue-tracker / VCS credentials to the Formicary queen as a
  Kubernetes secret and registers workflow configuration.  Run this once
  per developer after deploying the ant worker, or whenever credentials change.

TRACKER (positional or --tracker flag)
  jira        Jira issue tracker  (JIRA_EMAIL, JIRA_API_TOKEN, JIRA_BASE_URL)
  bitbucket   Bitbucket VCS       (BITBUCKET_TOKEN)
  github      GitHub issues + VCS (GH_TOKEN)
  all         All three trackers
  (auto)      Omit to auto-detect from environment variables

OPTIONS
  -s, --server URL     Formicary queen URL    (env: FORMICARY_URL)
  -t, --tracker T      Tracker to configure   (same as positional arg)
  -h, --help           Show this help

REQUIRED ENV VARS
  FORMICARY_TOKEN      API token  (get one at <server>/dashboard/users/tokens)
  FORMICARY_URL        Queen URL  (e.g. https://10.8.97.24.nip.io)

  For --tracker jira:
    JIRA_EMAIL         Jira account email
    JIRA_API_TOKEN     Jira API token  (Atlassian account settings → API tokens)
    JIRA_BASE_URL      Jira base URL   (e.g. https://company.atlassian.net)

  For --tracker bitbucket:
    BITBUCKET_TOKEN    Bitbucket app password
    BITBUCKET_USERNAME Bitbucket username  (optional, read from acli config)
    BITBUCKET_WORKSPACE Bitbucket workspace (optional)

  For --tracker github:
    GH_TOKEN           GitHub personal access token (repo + read:org scopes)

EXAMPLES
  # Set up Jira credentials only
  source ~/.zshrc
  $(basename "$0") jira

  # Set up Bitbucket separately from Jira
  $(basename "$0") bitbucket

  # Set up GitHub
  $(basename "$0") github

  # Set up everything in one step
  $(basename "$0") all

  # Auto-detect from environment (sets up whichever tokens are present)
  $(basename "$0")

  # Point at a different server
  $(basename "$0") --server http://localhost:7777 jira

NOTES
  - Credentials are stored in the 'ai-dev-credentials' Kubernetes secret and
    as org-level config variables in Formicary.
  - Jira and Bitbucket share the same underlying deploy script because they
    are typically used together, but each validates only its own token.
  - Re-running is safe (idempotent) — it updates existing secrets and configs.
EOF
}

# ── Defaults ──────────────────────────────────────────────────────────────────
SERVER="${FORMICARY_URL:-}"
TRACKER=""

# ── Argument parsing ──────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)        usage; exit 0 ;;
    -s|--server)      SERVER="$2";  shift 2 ;;
    -t|--tracker)     TRACKER="$2"; shift 2 ;;
    jira|bitbucket|github|all) TRACKER="$1"; shift ;;
    *) echo "Unknown option: $1  (try --help)" >&2; exit 1 ;;
  esac
done

# ── Validation ────────────────────────────────────────────────────────────────
[[ -z "${FORMICARY_TOKEN:-}" ]] && \
  fail "FORMICARY_TOKEN is not set — source ~/.zshrc or export it before running"

[[ -z "${SERVER}" ]] && \
  fail "Formicary URL not set — export FORMICARY_URL=https://... or use --server"

if [[ -n "${TRACKER}" ]]; then
  case "${TRACKER}" in
    jira|bitbucket|github|all) ;;
    *) fail "Unknown tracker '${TRACKER}' — must be one of: jira, bitbucket, github, all" ;;
  esac
fi

# Prerequisite scripts
[[ -f "${EXAMPLES_DIR}/deploy-ai-jira-workflows.sh" ]] || \
  fail "Not found: ${EXAMPLES_DIR}/deploy-ai-jira-workflows.sh — run from the formicary repo root"
[[ -f "${EXAMPLES_DIR}/deploy-ai-workflows.sh" ]] || \
  fail "Not found: ${EXAMPLES_DIR}/deploy-ai-workflows.sh — run from the formicary repo root"

# ── Auto-detect tracker from env ──────────────────────────────────────────────
if [[ -z "${TRACKER}" ]]; then
  _detected=()
  [[ -n "${JIRA_API_TOKEN:-}" ]]   && _detected+=("jira")
  [[ -n "${BITBUCKET_TOKEN:-}" ]]  && _detected+=("bitbucket")
  [[ -n "${GH_TOKEN:-}" ]]         && _detected+=("github")
  [[ ${#_detected[@]} -eq 0 ]] && \
    fail "No tracker specified and no JIRA_API_TOKEN / BITBUCKET_TOKEN / GH_TOKEN found in env. Pass a tracker or source your credentials."
  TRACKER="$(IFS=+; echo "${_detected[*]}")"
  log "Auto-detected trackers: ${TRACKER}"
fi

log "Server: ${SERVER}  User: ${USER:-$(whoami)}"

# ── Runner functions ──────────────────────────────────────────────────────────
run_jira() {
  log "Configuring Jira..."
  [[ -n "${JIRA_API_TOKEN:-}" ]] || fail "JIRA_API_TOKEN is not set"
  [[ -n "${JIRA_EMAIL:-}" ]]     || fail "JIRA_EMAIL is not set"
  [[ -n "${JIRA_BASE_URL:-}" ]]  || fail "JIRA_BASE_URL is not set"
  "${EXAMPLES_DIR}/deploy-ai-jira-workflows.sh" \
    --server "${SERVER}" \
    --create-k8s-secret \
    --set-configs
  ok "Jira configured  (${JIRA_BASE_URL})"
}

run_bitbucket() {
  log "Configuring Bitbucket..."
  [[ -n "${BITBUCKET_TOKEN:-}" ]] || fail "BITBUCKET_TOKEN is not set"
  "${EXAMPLES_DIR}/deploy-ai-jira-workflows.sh" \
    --server "${SERVER}" \
    --create-k8s-secret \
    --set-configs
  ok "Bitbucket configured"
}

run_github() {
  log "Configuring GitHub..."
  [[ -n "${GH_TOKEN:-}" ]] || fail "GH_TOKEN is not set"
  "${EXAMPLES_DIR}/deploy-ai-workflows.sh" \
    --server "${SERVER}" \
    --create-k8s-secret \
    --set-configs
  ok "GitHub configured"
}

run_tracker() {
  case "$1" in
    jira)       run_jira ;;
    bitbucket)  run_bitbucket ;;
    github)     run_github ;;
    all)        run_jira; run_bitbucket; run_github ;;
    *) fail "Unknown tracker: $1" ;;
  esac
}

# ── Execute ───────────────────────────────────────────────────────────────────
# TRACKER may be a single value or a '+'-joined auto-detected list (e.g. "jira+github")
IFS='+' read -ra _TRACKERS <<< "${TRACKER}"
for _t in "${_TRACKERS[@]}"; do
  run_tracker "${_t}"
done

# ── Done ──────────────────────────────────────────────────────────────────────
echo ""
ok "Done — credentials configured for ${USER:-$(whoami)}"
echo ""
echo "  Next steps:"
echo "    1. Invite your bot to a Slack channel: /invite @<bot-name>"
echo "    2. DM the bot: setup ${FORMICARY_TOKEN:0:8}...  (your token)"
echo "       (token page: ${SERVER}/dashboard/users/tokens)"
echo "    3. In the channel: @bot help"
