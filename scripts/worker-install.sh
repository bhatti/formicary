#!/usr/bin/env bash
# worker-install.sh — Interactive Formicary worker installer.
#
# Guides a new team member through:
#   1. Setting up required env vars (with prompts for missing ones)
#   2. Deploying AI workflow YAMLs (Jira and/or GitHub)
#   3. Deploying the ant worker to their local k8s cluster
#
# Usage (from the formicary repo root):
#   ./scripts/worker-install.sh
#
# Or sparse-clone first (one-liner for a coworker without the full repo):
#   git clone --filter=blob:none --sparse https://github.com/bhatti/formicary formicary-install
#   cd formicary-install
#   git sparse-checkout set scripts k8s docs/examples
#   ./scripts/worker-install.sh
#
# Flags:
#   --skip-workflows  skip AI workflow YAML deployment
#   --skip-ant        skip ant worker deployment
#   --dry-run         check env vars only, don't deploy
#
# Required env vars (can be set in ~/.zshrc or entered interactively):
#   FORMICARY_URL           e.g. https://YOUR_EC2_IP.nip.io
#   FORMICARY_TOKEN         API token from the dashboard (api token type)
#   COMMON_AUTH_JWT_SECRET  JWT signing secret (must match the queen's secret)
#
# Optional (for AI workflows):
#   JIRA_URL / JIRA_USER / JIRA_API_TOKEN / JIRA_PROJECT
#   GITHUB_TOKEN / GH_ORG / GH_REPO
#   SLACK_BOT_TOKEN / SLACK_APP_TOKEN / SLACK_CHANNEL
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

log()  { echo ""; echo "▶  $*"; }
ok()   { echo "   ✓  $*"; }
warn() { echo "   ⚠  $*"; }
info() { echo "   $*"; }
fail() { echo ""; echo "   ✗  ERROR: $*" >&2; exit 1; }
sep()  { echo "   ──────────────────────────────────────────────"; }
hr()   { echo ""; echo "════════════════════════════════════════════════════"; }

# ── Parse flags ────────────────────────────────────────────────────────────────
SKIP_WORKFLOWS=false
SKIP_ANT=false
DRY_RUN=false
NONINTERACTIVE=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-workflows) SKIP_WORKFLOWS=true; shift ;;
    --skip-ant)       SKIP_ANT=true;       shift ;;
    --dry-run)        DRY_RUN=true;        shift ;;
    --non-interactive) NONINTERACTIVE=true; shift ;;
    --help|-h)
      grep '^#' "$0" | sed 's/^# *//'
      exit 0
      ;;
    *) echo "Unknown flag: $1" >&2; exit 1 ;;
  esac
done

hr
echo "  Formicary Worker Installer"
echo "  Sets up AI workflow jobs and an ant worker on your local k8s cluster."
hr

# ── Helper: prompt for a value if not set ─────────────────────────────────────
# Usage: prompt_var VAR_NAME "description" "hint" [required|optional]
prompt_var() {
  local name="$1" desc="$2" hint="${3:-}" req="${4:-required}"
  local current="${!name:-}"
  if [[ -n "$current" ]]; then
    ok "$name is set"
    return
  fi
  if [[ "$NONINTERACTIVE" == true || "$DRY_RUN" == true ]]; then
    [[ "$req" == required ]] && warn "MISSING (required): $name — $hint"
    [[ "$req" == optional ]] && warn "NOT SET (optional): $name — $hint"
    return
  fi
  echo ""
  echo "   $desc"
  [[ -n "$hint" ]] && echo "   Hint: $hint"
  read -rp "   Enter $name (or press Enter to skip): " val
  if [[ -n "$val" ]]; then
    export "$name"="$val"
    ok "$name set for this session (add to ~/.zshrc to make permanent)"
  else
    [[ "$req" == required ]] && warn "$name not set — some steps may fail"
  fi
}

# ── Step 0: Environment check and interactive setup ───────────────────────────
log "Step 0: Environment setup"
sep

info "Formicary URL and token are required. Get your token from:"
info "  ${FORMICARY_URL:-https://YOUR_EC2_IP.nip.io}/dashboard → click your name (bottom nav) → API Tokens"

prompt_var FORMICARY_URL   "Formicary server URL"    "e.g. https://YOUR_EC2_IP.nip.io" required
prompt_var FORMICARY_TOKEN "Your Formicary API token" "generate at \$FORMICARY_URL/dashboard → API Tokens" required

# Prompt for integration type
DEPLOY_JIRA=false
DEPLOY_GH=false
if [[ -z "${JIRA_URL:-}" && -z "${GITHUB_TOKEN:-}" && "$NONINTERACTIVE" != true && "$DRY_RUN" != true ]]; then
  echo ""
  echo "   Which AI workflow integrations do you want to set up?"
  echo "   [j] Jira   (ai-jira-review, ai-jira-standup, etc.)"
  echo "   [g] GitHub (ai-gh-review, ai-gh-standup, etc.)"
  echo "   [b] Both"
  echo "   [n] Neither (ant worker only)"
  read -rp "   Choose [j/g/b/n]: " choice
  case "$choice" in
    j|J) DEPLOY_JIRA=true ;;
    g|G) DEPLOY_GH=true ;;
    b|B) DEPLOY_JIRA=true; DEPLOY_GH=true ;;
    n|N) SKIP_WORKFLOWS=true ;;
    *) warn "Unknown choice, skipping workflows" ; SKIP_WORKFLOWS=true ;;
  esac
else
  [[ -n "${JIRA_URL:-}" ]] && DEPLOY_JIRA=true
  [[ -n "${GITHUB_TOKEN:-}" ]] && DEPLOY_GH=true
fi

# Prompt for relevant credentials
if [[ "$DEPLOY_JIRA" == true ]]; then
  prompt_var JIRA_URL       "Jira base URL"    "e.g. https://yourorg.atlassian.net" required
  prompt_var JIRA_USER      "Jira user email"  "e.g. you@company.com" required
  prompt_var JIRA_API_TOKEN "Jira API token"   "generate at id.atlassian.com/manage-profile/security/api-tokens" required
  prompt_var JIRA_PROJECT   "Jira project key" "e.g. PROJ" optional
fi

if [[ "$DEPLOY_GH" == true ]]; then
  prompt_var GITHUB_TOKEN "GitHub personal access token" "needs repo + read:org scopes" required
  prompt_var GH_ORG       "GitHub organization"          "e.g. your-org" optional
  prompt_var GH_REPO      "GitHub repository name"       "e.g. your-repo" optional
fi

# Slack integration (optional)
if [[ "$NONINTERACTIVE" != true && "$DRY_RUN" != true && -z "${SLACK_BOT_TOKEN:-}" ]]; then
  echo ""
  read -rp "   Set up Slack integration? [y/N]: " slk
  if [[ "$slk" =~ ^[yY] ]]; then
    prompt_var SLACK_BOT_TOKEN "Slack Bot OAuth Token"  "xoxb-... from api.slack.com/apps" required
    prompt_var SLACK_APP_TOKEN "Slack App-Level Token"  "xapp-... (connections:write)" required
    prompt_var SLACK_CHANNEL   "Default Slack channel"  "e.g. C0XXXXXXX (channel ID)" optional
  fi
fi

[[ "$DRY_RUN" == true ]] && { echo ""; echo "   [dry-run] Done."; exit 0; }

# ── Validate required vars ──────────────────────────────────────────────────────
MISSING=0
for v in FORMICARY_URL FORMICARY_TOKEN; do
  [[ -z "${!v:-}" ]] && { warn "MISSING required: $v"; MISSING=$((MISSING+1)); }
done
if [[ "$DEPLOY_JIRA" == true ]]; then
  for v in JIRA_URL JIRA_USER JIRA_API_TOKEN; do
    [[ -z "${!v:-}" ]] && { warn "MISSING required for Jira: $v"; MISSING=$((MISSING+1)); }
  done
fi
if [[ "$DEPLOY_GH" == true ]]; then
  for v in GITHUB_TOKEN; do
    [[ -z "${!v:-}" ]] && { warn "MISSING required for GitHub: $v"; MISSING=$((MISSING+1)); }
  done
fi
[[ "$MISSING" -gt 0 ]] && fail "$MISSING required variable(s) missing. Add them to ~/.zshrc and re-run."

# ── Step 1: Deploy workflow YAMLs ─────────────────────────────────────────────
if [[ "$SKIP_WORKFLOWS" == false ]]; then
  EXAMPLES_DIR="${REPO_ROOT}/docs/examples"
  [[ -d "$EXAMPLES_DIR" ]] || fail "docs/examples not found: ${EXAMPLES_DIR}"

  if [[ "$DEPLOY_JIRA" == true ]]; then
    log "Step 1a: Deploying Jira AI workflows ..."
    sep
    FORMICARY_URL="$FORMICARY_URL" FORMICARY_TOKEN="$FORMICARY_TOKEN" \
      "${EXAMPLES_DIR}/deploy-ai-jira-workflows.sh" \
        --set-configs \
        ${JIRA_PROJECT:+--jira-project "$JIRA_PROJECT"} \
        ${GH_ORG:+--gh-org "$GH_ORG"} \
        ${GH_REPO:+--gh-repo "$GH_REPO"} 2>&1 | sed 's/^/   /'
    ok "Jira workflows deployed"
  fi

  if [[ "$DEPLOY_GH" == true ]]; then
    log "Step 1b: Deploying GitHub AI workflows ..."
    sep
    FORMICARY_URL="$FORMICARY_URL" FORMICARY_TOKEN="$FORMICARY_TOKEN" \
      "${EXAMPLES_DIR}/deploy-ai-workflows.sh" \
        --set-configs \
        ${GH_ORG:+--gh-org "$GH_ORG"} \
        ${GH_REPO:+--gh-repo "$GH_REPO"} 2>&1 | sed 's/^/   /'
    ok "GitHub workflows deployed"
  fi
fi

# ── Step 2: Deploy ant worker ─────────────────────────────────────────────────
if [[ "$SKIP_ANT" == false ]]; then
  log "Step 2: Deploying ant worker to your local k8s cluster ..."
  sep
  info "The ant worker runs on your local machine and connects to the Formicary queen."
  info "It needs access to your local k8s cluster (Docker Desktop, kind, etc.)."
  echo ""

  # Detect available k8s contexts
  CONTEXTS=$(kubectl config get-contexts -o name 2>/dev/null | tr '\n' ' ' || echo "")
  if [[ -z "$CONTEXTS" ]]; then
    warn "No k8s contexts found. Install Docker Desktop or kind and try again."
    warn "Skipping ant worker deployment."
  else
    info "Available k8s contexts: $CONTEXTS"
    CURRENT_CTX=$(kubectl config current-context 2>/dev/null || echo "")
    info "Current context: $CURRENT_CTX"

    if [[ "$NONINTERACTIVE" != true ]]; then
      read -rp "   Deploy ant worker to context '$CURRENT_CTX'? [Y/n]: " deploy_ant
      [[ "$deploy_ant" =~ ^[nN] ]] && { warn "Skipping ant deployment"; SKIP_ANT=true; }
    fi

    if [[ "$SKIP_ANT" == false ]]; then
      FORMICARY_URL="$FORMICARY_URL" FORMICARY_TOKEN="$FORMICARY_TOKEN" \
        "${SCRIPT_DIR}/setup-ant-worker.sh" 2>&1 | sed 's/^/   /'
      ok "Ant worker deployed"
    fi
  fi
fi

# ── Done ──────────────────────────────────────────────────────────────────────
hr
echo "  ✅  Worker installation complete!"
echo ""
echo "  Dashboard: ${FORMICARY_URL}/dashboard"
echo "  Ant workers: ${FORMICARY_URL}/dashboard/ants"
echo ""
echo "  Next steps:"
echo "  1. Check ant worker connected at ${FORMICARY_URL}/dashboard/ants"
echo "  2. Connect Slack: visit ${FORMICARY_URL}/dashboard/slack/setup"
echo "     (generates a one-time code — DM it to the bot)"
echo ""
echo "  To persist env vars, add to ~/.zshrc:"
[[ -n "${FORMICARY_URL:-}" ]] && echo "    export FORMICARY_URL=\"${FORMICARY_URL}\""
[[ -n "${JIRA_URL:-}" ]] && echo "    export JIRA_URL=\"${JIRA_URL}\""
[[ -n "${GH_ORG:-}" ]] && echo "    export GH_ORG=\"${GH_ORG}\""
hr
