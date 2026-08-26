#!/usr/bin/env bash
# install.sh — Formicary + ai-dev-tools quick installer.
#
# Sparse-clones only what's needed from the formicary repo (docs/examples + scripts),
# then guides you through env var setup and deploys everything.
#
# Usage (one-liner on a fresh machine):
#   export FORMICARY_REPO=https://github.com/bhatti/formicary
#   bash <(curl -sfL ${FORMICARY_REPO}/raw/main/scripts/install.sh)
#
#   Or clone first:
#   git clone --filter=blob:none --sparse https://github.com/bhatti/formicary formicary-install
#   cd formicary-install
#   git sparse-checkout set scripts k8s docs/examples
#   ./scripts/install.sh
#
# What it does:
#   1. Checks required env vars (prints instructions for missing ones)
#   2. Bootstraps EC2 if EC2_IP is set (k3s, iptables, CoreDNS — idempotent)
#   3. Deploys the Formicary queen to EC2 k3s
#   4. Deploys workflow YAMLs to the queen
#   5. Deploys the ant worker to the local k8s cluster
#
# Required env vars — add these to ~/.zshrc:
#
#   # Core auth
#   export COMMON_AUTH_JWT_SECRET="<random-secret-min-32-chars>"
#   export FORMICARY_TOKEN="<api-token-from-dashboard-after-first-login>"
#
#   # Google OAuth (get from Google Cloud Console)
#   export COMMON_AUTH_GOOGLE_CLIENT_ID="<client-id>.apps.googleusercontent.com"
#   export COMMON_AUTH_GOOGLE_CLIENT_SECRET="<client-secret>"
#   export COMMON_AUTH_GOOGLE_CALLBACK_HOST="https://<your-domain>"
#
#   # Slack (Socket Mode — get from api.slack.com/apps)
#   export SLACK_APP_TOKEN="xapp-..."      # App-Level Token (connections:write)
#   export SLACK_BOT_TOKEN="xoxb-..."      # Bot User OAuth Token
#   export SLACK_CHANNEL="C0XXXXXXX"       # Default channel ID
#
#   # EC2 deployment target
#   export EC2_IP="YOUR_EC2_IP"
#   export EC2_KEY="~/Downloads/sbhatti-linux-key.pem"
#
#   # Jira (optional — for ai-jira-* workflows)
#   export JIRA_URL="https://yourorg.atlassian.net"
#   export JIRA_USER="user@example.com"
#   export JIRA_API_TOKEN="<jira-api-token>"
#   export JIRA_PROJECT="PROJ"
#
#   # GitHub (optional — for ai-gh-* workflows)
#   export GITHUB_TOKEN="ghp_..."
#   export GH_ORG="your-org"
#   export GH_REPO="your-repo"
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

log()  { echo ""; echo "▶ $*"; }
ok()   { echo "  ✓ $*"; }
warn() { echo "  ⚠ $*"; }
fail() { echo ""; echo "  ✗ ERROR: $*" >&2; exit 1; }
sep()  { echo "  ──────────────────────────────────────────"; }

# ── Parse flags ───────────────────────────────────────────────────────────────
SKIP_BOOTSTRAP=false
SKIP_QUEEN=false
SKIP_WORKFLOWS=false
SKIP_ANT=false
DRY_RUN=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-bootstrap) SKIP_BOOTSTRAP=true; shift ;;
    --skip-queen)     SKIP_QUEEN=true;     shift ;;
    --skip-workflows) SKIP_WORKFLOWS=true; shift ;;
    --skip-ant)       SKIP_ANT=true;       shift ;;
    --dry-run)        DRY_RUN=true;        shift ;;
    --help|-h)
      grep '^#' "$0" | head -40 | sed 's/^# //'
      exit 0
      ;;
    *) echo "Unknown flag: $1"; exit 1 ;;
  esac
done

echo ""
echo "════════════════════════════════════════════════════"
echo "  Formicary Installer"
echo "════════════════════════════════════════════════════"

# ── Step 0: Check required env vars ──────────────────────────────────────────
log "Checking environment variables ..."

MISSING=0
check_var() {
  local name="$1" required="${2:-yes}" hint="${3:-}"
  local val="${!name:-}"
  if [[ -z "$val" ]]; then
    if [[ "$required" == "yes" ]]; then
      echo "  ✗ $name is not set${hint:+ — $hint}"
      MISSING=$((MISSING+1))
    else
      warn "$name not set (optional${hint:+ — $hint})"
    fi
  else
    ok "$name is set"
  fi
}

check_var COMMON_AUTH_JWT_SECRET   yes "generate with: openssl rand -hex 32"
check_var FORMICARY_TOKEN          yes "get from https://\$EC2_IP.nip.io/dashboard after first login"
check_var COMMON_AUTH_GOOGLE_CLIENT_ID     yes "from Google Cloud Console OAuth credentials"
check_var COMMON_AUTH_GOOGLE_CLIENT_SECRET yes "from Google Cloud Console OAuth credentials"
check_var COMMON_AUTH_GOOGLE_CALLBACK_HOST yes "e.g. https://YOUR_EC2_IP.nip.io"

check_var EC2_IP  yes "EC2 instance IP"
check_var EC2_KEY no  "default: ~/Downloads/sbhatti-linux-key.pem"

check_var SLACK_APP_TOKEN no "xapp-... from api.slack.com/apps"
check_var SLACK_BOT_TOKEN no "xoxb-... from api.slack.com/apps"
check_var SLACK_CHANNEL   no "channel ID (e.g. C0XXXXXXX)"

check_var JIRA_URL       no "https://yourorg.atlassian.net"
check_var JIRA_USER      no "user@example.com"
check_var JIRA_API_TOKEN no "Jira API token"
check_var GITHUB_TOKEN   no "ghp_..."

if [[ "$MISSING" -gt 0 ]]; then
  echo ""
  echo "  Add missing variables to ~/.zshrc, then run: source ~/.zshrc"
  echo "  See the comments at the top of this script for the full list."
  [[ "$DRY_RUN" == true ]] || fail "$MISSING required variable(s) missing — cannot continue"
fi

[[ "$DRY_RUN" == true ]] && { echo ""; echo "  [dry-run] Env check complete. Exiting."; exit 0; }

# ── Step 1: Bootstrap EC2 ─────────────────────────────────────────────────────
if [[ "$SKIP_BOOTSTRAP" == false && -n "${EC2_IP:-}" ]]; then
  log "Step 1: Bootstrapping EC2 ${EC2_IP} (k3s + iptables + CoreDNS) ..."
  sep
  "${SCRIPT_DIR}/bootstrap-ec2.sh"
  ok "EC2 bootstrap done"
else
  warn "Skipping EC2 bootstrap (--skip-bootstrap or EC2_IP not set)"
fi

# ── Step 2: Deploy Formicary queen ────────────────────────────────────────────
if [[ "$SKIP_QUEEN" == false ]]; then
  log "Step 2: Deploying Formicary queen to EC2 ..."
  sep
  "${SCRIPT_DIR}/deploy-formicary.sh" ${EC2_IP:+--ec2-ip "$EC2_IP"}
  ok "Queen deployed"
else
  warn "Skipping queen deploy (--skip-queen)"
fi

# ── Step 3: Deploy workflow YAMLs ─────────────────────────────────────────────
if [[ "$SKIP_WORKFLOWS" == false ]]; then
  log "Step 3: Deploying AI workflow YAMLs ..."
  sep
  EXAMPLES_DIR="${REPO_ROOT}/docs/examples"
  [[ -d "$EXAMPLES_DIR" ]] || fail "docs/examples not found: ${EXAMPLES_DIR}"

  FORMICARY_URL="${FORMICARY_URL:-https://${EC2_IP}.nip.io}"

  # Determine which deploy script to use based on available env vars
  if [[ -n "${JIRA_URL:-}" && -n "${JIRA_API_TOKEN:-}" ]]; then
    log "  Deploying Jira workflows ..."
    FORMICARY_URL="$FORMICARY_URL" FORMICARY_TOKEN="$FORMICARY_TOKEN" \
      "${EXAMPLES_DIR}/deploy-ai-jira-workflows.sh" \
        --set-configs \
        ${JIRA_PROJECT:+--jira-project "$JIRA_PROJECT"} \
        ${GH_ORG:+--gh-org "$GH_ORG"} \
        ${GH_REPO:+--gh-repo "$GH_REPO"} 2>&1 | sed 's/^/  /'
  fi

  if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    log "  Deploying GitHub workflows ..."
    FORMICARY_URL="$FORMICARY_URL" FORMICARY_TOKEN="$FORMICARY_TOKEN" \
      "${EXAMPLES_DIR}/deploy-ai-workflows.sh" \
        --set-configs \
        ${GH_ORG:+--gh-org "$GH_ORG"} \
        ${GH_REPO:+--gh-repo "$GH_REPO"} 2>&1 | sed 's/^/  /'
  fi

  if [[ -z "${JIRA_URL:-}" && -z "${GITHUB_TOKEN:-}" ]]; then
    warn "Neither JIRA_URL nor GITHUB_TOKEN set — skipping workflow deploy"
    warn "Set them in ~/.zshrc and re-run with --skip-bootstrap --skip-queen"
  fi
else
  warn "Skipping workflow deploy (--skip-workflows)"
fi

# ── Step 4: Deploy ant worker ─────────────────────────────────────────────────
if [[ "$SKIP_ANT" == false ]]; then
  log "Step 4: Deploying ant worker ..."
  sep
  FORMICARY_URL="${FORMICARY_URL:-https://${EC2_IP}.nip.io}" \
  FORMICARY_TOKEN="$FORMICARY_TOKEN" \
    "${SCRIPT_DIR}/setup-ant-worker.sh" 2>&1 | sed 's/^/  /'
  ok "Ant worker deployed"
else
  warn "Skipping ant worker deploy (--skip-ant)"
fi

# ── Done ──────────────────────────────────────────────────────────────────────
FORMICARY_URL="${FORMICARY_URL:-https://${EC2_IP:-localhost}.nip.io}"
echo ""
echo "════════════════════════════════════════════════════"
echo "  ✅  Installation complete!"
echo ""
echo "  Dashboard: ${FORMICARY_URL}/dashboard"
echo ""
echo "  Next steps:"
echo "  1. Log in via Google OAuth at ${FORMICARY_URL}/dashboard"
echo "  2. Click your name (bottom nav) → API Tokens → copy your token"
echo "  3. Add it to ~/.zshrc:  export FORMICARY_TOKEN=\"<token>\""
echo "  4. Slack: DM @sb-slack with:  setup <your-api-token>"
echo "  5. Try:  @sb-slack help"
echo "════════════════════════════════════════════════════"
