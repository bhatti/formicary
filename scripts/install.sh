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
# Local standalone mode (queen + ant in one pod, docker-desktop k8s):
#   source ~/.zshrc
#   ./scripts/install.sh --local
#
#   What --local does:
#   1. Applies k8s/formicary-all-in-one.yaml to docker-desktop
#   2. Waits for rollout, port-forwards 7777 and 19000 in the background
#   3. Waits for /api/health to return 200
#   4. Deploys AI workflow YAMLs (GitHub and/or Jira depending on env vars)
#   5. Pushes Slack tokens + route table via setup-slack-admin.sh
#   6. Prints the local URL and next steps
#
#   Requires: kubectl context docker-desktop, COMMON_AUTH_JWT_SECRET,
#             FORMICARY_TOKEN (created after first login), SLACK_BOT_TOKEN, SLACK_APP_TOKEN
#
# What it does (EC2 mode, default):
#   1. Checks required env vars (prints instructions for missing ones)
#   2. Bootstraps queen host if QUEEN_IP is set (k3s, iptables, CoreDNS — idempotent)
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
#   # Queen (server) deployment target
#   export QUEEN_IP="YOUR_HOST_IP"
#   export QUEEN_SSH_KEY="~/.ssh/id_rsa"     # optional — uses SSH agent if unset
#   export QUEEN_SSH_USER="ec2-user"          # optional — uses SSH config default if unset
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
LOCAL_MODE=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-bootstrap) SKIP_BOOTSTRAP=true; shift ;;
    --skip-queen)     SKIP_QUEEN=true;     shift ;;
    --skip-workflows) SKIP_WORKFLOWS=true; shift ;;
    --skip-ant)       SKIP_ANT=true;       shift ;;
    --dry-run)        DRY_RUN=true;        shift ;;
    --local)          LOCAL_MODE=true;     shift ;;
    --help|-h)
      grep '^#' "$0" | head -60 | sed 's/^# //'
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
# In --local mode FORMICARY_TOKEN can be set after first login; in EC2 mode it's required.
if [[ "$LOCAL_MODE" == false ]]; then
  check_var FORMICARY_TOKEN          yes "get from https://\$QUEEN_IP.nip.io/dashboard after first login"
  check_var COMMON_AUTH_GOOGLE_CLIENT_ID     yes "from Google Cloud Console OAuth credentials"
  check_var COMMON_AUTH_GOOGLE_CLIENT_SECRET yes "from Google Cloud Console OAuth credentials"
  check_var COMMON_AUTH_GOOGLE_CALLBACK_HOST yes "e.g. https://YOUR_QUEEN_IP.nip.io"
  check_var QUEEN_IP      yes "queen host IP or hostname"
  check_var QUEEN_SSH_KEY no  "SSH key path (leave unset to use SSH agent)"
else
  check_var FORMICARY_TOKEN no "get from http://localhost:7777/dashboard after first login; re-run --local to deploy workflows"
  check_var COMMON_AUTH_GOOGLE_CLIENT_ID     no "from Google Cloud Console OAuth credentials"
  check_var COMMON_AUTH_GOOGLE_CLIENT_SECRET no "from Google Cloud Console OAuth credentials"
  check_var COMMON_AUTH_GOOGLE_CALLBACK_HOST no "e.g. http://localhost:7777 for local testing"
fi

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

# ── Local standalone mode ─────────────────────────────────────────────────────
if [[ "$LOCAL_MODE" == true ]]; then
  # Always use localhost in local mode — ignore any FORMICARY_URL set to a remote host.
  LOCAL_URL="http://localhost:7777"
  EXAMPLES_DIR="${REPO_ROOT}/docs/examples"
  MANIFEST="${REPO_ROOT}/k8s/formicary-all-in-one.yaml"

  [[ -f "$MANIFEST" ]] || fail "Manifest not found: ${MANIFEST}"
  [[ -d "$EXAMPLES_DIR" ]] || fail "docs/examples not found: ${EXAMPLES_DIR}"

  # Require JWT secret for local mode
  JWT_SECRET="${COMMON_AUTH_JWT_SECRET:-}"
  [[ -n "$JWT_SECRET" ]] || fail "COMMON_AUTH_JWT_SECRET is required — set it in ~/.zshrc"

  log "Local mode: targeting docker-desktop k8s at ${LOCAL_URL}"
  sep

  # Switch to docker-desktop context; bail with a clear message if k8s isn't running.
  if ! kubectl config use-context docker-desktop >/dev/null 2>&1; then
    warn "No 'docker-desktop' context found — trying current context"
  fi
  _KUBE_CTX=$(kubectl config current-context 2>/dev/null || echo "none")
  ok "kubectl context: ${_KUBE_CTX}"
  if ! kubectl cluster-info >/dev/null 2>&1; then
    echo ""
    echo "  ✗ Cannot reach Kubernetes API server."
    echo ""
    echo "  To fix: open Docker Desktop → Settings → Kubernetes → ✓ Enable Kubernetes → Apply & Restart"
    echo "  Then wait ~60s for k8s to start, and re-run: bash scripts/install.sh --local"
    exit 1
  fi

  # Resolve effective Jira URL: accept JIRA_BASE_URL or JIRA_URL (same as deploy-ai-jira-workflows.sh)
  _JIRA_URL="${JIRA_BASE_URL:-${JIRA_URL:-}}"

  # ── 1. Create formicary-auth + formicary-slack secrets ──────────────────────
  log "Creating/updating formicary-auth secret"
  kubectl create secret generic formicary-auth \
    --from-literal=jwt-secret="${JWT_SECRET}" \
    --from-literal=google-client-id="${COMMON_AUTH_GOOGLE_CLIENT_ID:-}" \
    --from-literal=google-client-secret="${COMMON_AUTH_GOOGLE_CLIENT_SECRET:-}" \
    --from-literal=google-callback-host="${COMMON_AUTH_GOOGLE_CALLBACK_HOST:-}" \
    --from-literal=auth-secure="${COMMON_AUTH_SECURE:-false}" \
    --save-config --dry-run=client -o yaml | kubectl apply -f -
  ok "formicary-auth secret applied"

  if [[ -n "${SLACK_APP_TOKEN:-}" ]]; then
    kubectl create secret generic formicary-slack \
      --from-literal=app-token="${SLACK_APP_TOKEN}" \
      --from-literal=bot-token="${SLACK_BOT_TOKEN:-}" \
      --from-literal=signing-secret="${SLACK_SIGNING_SECRET:-}" \
      --from-literal=slack-channel="${SLACK_CHANNEL:-}" \
      --save-config --dry-run=client -o yaml | kubectl apply -f -
    ok "formicary-slack secret applied (Socket Mode enabled)"
  else
    warn "SLACK_APP_TOKEN not set — Slack disabled for local testing"
  fi

  # ── 2. Create ai-dev-credentials k8s secret ─────────────────────────────────
  # Must run BEFORE workflow pods start — contains GH_TOKEN, JIRA_API_TOKEN,
  # SLACK_BOT_TOKEN, ANTHROPIC_API_KEY, SSH_PRIVATE_KEY used by ai-dev-tools pods.
  # This is a kubectl-only operation; does not require FORMICARY_TOKEN.
  _GH_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
  if [[ -n "$_GH_TOKEN" ]]; then
    log "Creating/updating ai-dev-credentials secret"
    kubectl create secret generic ai-dev-credentials \
      --from-literal=GH_TOKEN="${_GH_TOKEN}" \
      --from-literal=GH_ORG="${GH_ORG:-}" \
      --from-literal=GH_REPO="${GH_REPO:-}" \
      --from-literal=JIRA_BASE_URL="${_JIRA_URL}" \
      --from-literal=JIRA_EMAIL="${JIRA_EMAIL:-}" \
      --from-literal=JIRA_API_TOKEN="${JIRA_API_TOKEN:-}" \
      --from-literal=JIRA_HOST="${JIRA_HOST:-}" \
      --from-literal=BITBUCKET_WORKSPACE="${BITBUCKET_WORKSPACE:-}" \
      --from-literal=BITBUCKET_USERNAME="${BITBUCKET_USERNAME:-}" \
      --from-literal=BITBUCKET_TOKEN="${BITBUCKET_TOKEN:-}" \
      --from-literal=SLACK_BOT_TOKEN="${SLACK_BOT_TOKEN:-}" \
      --from-literal=ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-}" \
      --from-literal=SSH_PRIVATE_KEY="${SSH_PRIVATE_KEY:-}" \
      --save-config --dry-run=client -o yaml | kubectl apply -f - --validate=false
    ok "ai-dev-credentials secret applied"
  else
    warn "GITHUB_TOKEN not set — ai-dev-credentials secret not created; workflow pods may fail"
  fi

  # ── 3. Apply all-in-one manifest ────────────────────────────────────────────
  log "Applying formicary-all-in-one.yaml"
  kubectl apply -f "${MANIFEST}"
  if ! kubectl rollout status deployment/formicary --timeout=180s; then
    echo ""
    echo "  ✗ Deployment did not become ready. Diagnostics:"
    echo ""
    kubectl get pods -l app=formicary
    echo ""
    _POD=$(kubectl get pods -l app=formicary -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
    if [[ -n "$_POD" ]]; then
      echo "  Events for ${_POD}:"
      kubectl describe pod "$_POD" | grep -A 20 "^Events:" || true
      echo ""
      echo "  Logs (last 40 lines):"
      kubectl logs "$_POD" --tail=40 2>/dev/null || kubectl logs "$_POD" -c formicary --tail=40 2>/dev/null || true
    fi
    echo ""
    echo "  Common causes:"
    echo "    • Image pull failure: check docker.io/plexobject/formicary:latest is accessible"
    echo "    • OOMKilled: increase Docker Desktop memory (Settings → Resources → Memory ≥ 2GB)"
    echo "    • PVC pending: check storage class with: kubectl get pvc"
    fail "Deployment failed — fix the error above and re-run"
  fi
  ok "Deployment ready"

  # ── 4. Port-forward in background ───────────────────────────────────────────
  log "Port-forwarding 7777 and 19000 in background"
  pkill -f "kubectl port-forward.*formicary" 2>/dev/null || true
  kubectl port-forward svc/formicary 7777:7777 19000:19000 &
  PF_PID=$!
  ok "Port-forward PID=${PF_PID} — stop with: kill ${PF_PID}"

  # ── 5. Wait for /api/health ──────────────────────────────────────────────────
  log "Waiting for ${LOCAL_URL}/api/health ..."
  _HC="000"
  for i in $(seq 1 30); do
    _HC=$(curl -s -o /dev/null -w "%{http_code}" "${LOCAL_URL}/api/health" 2>/dev/null) || _HC="000"
    [[ "$_HC" == "200" ]] && { ok "Health check passed after ${i}s"; break; }
    sleep 1
  done
  [[ "$_HC" == "200" ]] || fail "Queen not healthy after 30s (HTTP ${_HC}) — check: kubectl logs -l app=formicary"

  # ── 6. Deploy workflow YAMLs + org configs ───────────────────────────────────
  # Verify token works against LOCAL instance before calling deploy scripts.
  _LOCAL_TOKEN="${FORMICARY_TOKEN:-}"
  _TOKEN_OK=false
  if [[ -n "$_LOCAL_TOKEN" ]]; then
    _TC=$(curl -s -o /dev/null -w "%{http_code}" \
      -H "Authorization: Bearer ${_LOCAL_TOKEN}" \
      "${LOCAL_URL}/api/users" 2>/dev/null) || _TC="000"
    [[ "$_TC" == "200" ]] && _TOKEN_OK=true
  fi

  if [[ "$_TOKEN_OK" == true ]]; then
    log "Deploying AI workflow YAMLs + org configs to ${LOCAL_URL}"

    if [[ -n "${_JIRA_URL}" && -n "${JIRA_API_TOKEN:-}" ]]; then
      # deploy-ai-jira-workflows.sh reads GH_ORG/GH_REPO from env; no --gh-org flag.
      FORMICARY_URL="${LOCAL_URL}" FORMICARY_TOKEN="${_LOCAL_TOKEN}" \
        "${EXAMPLES_DIR}/deploy-ai-jira-workflows.sh" \
          --set-configs \
          ${JIRA_PROJECT:+--jira-project "$JIRA_PROJECT"} 2>&1 | sed 's/^/  /'
      ok "Jira workflow YAMLs + configs deployed"
    else
      warn "JIRA_BASE_URL or JIRA_API_TOKEN not set — Jira workflows not deployed"
    fi

    if [[ -n "${_GH_TOKEN:-}" && -n "${GH_ORG:-}" && -n "${GH_REPO:-}" ]]; then
      # deploy-ai-workflows.sh requires --gh-org and --gh-repo (or GH_ORG/GH_REPO env vars)
      FORMICARY_URL="${LOCAL_URL}" FORMICARY_TOKEN="${_LOCAL_TOKEN}" \
        "${EXAMPLES_DIR}/deploy-ai-workflows.sh" \
          --set-configs \
          --gh-org "${GH_ORG}" \
          --gh-repo "${GH_REPO}" 2>&1 | sed 's/^/  /'
      ok "GitHub workflow YAMLs + configs deployed"
    elif [[ -n "${_GH_TOKEN:-}" ]]; then
      warn "GH_ORG or GH_REPO not set — GitHub workflows not deployed (set GH_ORG and GH_REPO in ~/.zshrc)"
    fi

    # ── 7. Push Slack tokens + route table (mirrors deploy-formicary.sh step 6) ─
    _SETUP="${EXAMPLES_DIR}/setup-slack-admin.sh"
    if [[ -f "$_SETUP" && -n "${SLACK_APP_TOKEN:-}" ]]; then
      log "Pushing Slack tokens + route table"
      FORMICARY_TOKEN="${_LOCAL_TOKEN}" FORMICARY_URL="${LOCAL_URL}" \
        bash "${_SETUP}" --set-routes --server "${LOCAL_URL}" 2>&1 \
        | grep -E '✓|✗|ERROR|WARNING|⚠|routes' || true
      ok "Slack routes pushed"
    fi

    # ── 8. Update SlackChannel org config (mirrors deploy-formicary.sh step 4) ─
    if [[ -n "${SLACK_CHANNEL:-}" ]]; then
      _ORG_ID=$(python3 -c "
import sys, json, base64
t=sys.argv[1]; p=t.split('.')
if len(p)!=3: sys.exit(1)
pad=4-len(p[1])%4
d=json.loads(base64.urlsafe_b64decode(p[1]+'='*pad))
print(d.get('org_id',''))
" "${_LOCAL_TOKEN}" 2>/dev/null || echo "")
      if [[ -n "$_ORG_ID" ]]; then
        _SC=$(curl -s -o /dev/null -w "%{http_code}" \
          -X POST "${LOCAL_URL}/api/orgs/${_ORG_ID}/configs" \
          -H "Authorization: Bearer ${_LOCAL_TOKEN}" \
          -H "Content-Type: application/json" \
          -d "{\"name\":\"SlackChannel\",\"value\":\"${SLACK_CHANNEL}\",\"secret\":false}" 2>/dev/null) || _SC="000"
        [[ "$_SC" == 2* ]] && ok "SlackChannel=${SLACK_CHANNEL} set" \
                           || warn "SlackChannel update HTTP ${_SC}"
      fi
    fi
  else
    echo ""
    warn "FORMICARY_TOKEN is not set or is invalid for the LOCAL instance."
    warn "Org configs, workflow YAMLs, and Slack routes were NOT pushed."
    echo ""
    echo "  Steps:"
    echo "  1. Open ${LOCAL_URL}/dashboard"
    echo "  2. Register an account (auth.enabled=false → no Google OAuth needed)"
    echo "  3. Go to your name → API Tokens → copy the token"
    echo "  4. Add to ~/.zshrc:  export FORMICARY_TOKEN=\"<local-token>\""
    echo "  5. Re-run:  source ~/.zshrc && bash scripts/install.sh --local"
    echo ""
  fi

  # ── Done ─────────────────────────────────────────────────────────────────────
  echo ""
  echo "════════════════════════════════════════════════════"
  echo "  ✅  Local Formicary running!"
  echo ""
  echo "  Dashboard: ${LOCAL_URL}/dashboard"
  echo "  Artifacts: http://localhost:19000"
  echo ""
  echo "  Stop port-forward: kill ${PF_PID}"
  echo "  View logs:         kubectl logs -l app=formicary -f"
  echo "  Delete cluster:    kubectl delete -f ${MANIFEST}"
  echo ""
  echo "  To re-run just workflows after token setup:"
  echo "    ./scripts/install.sh --local --skip-bootstrap"
  echo "════════════════════════════════════════════════════"
  exit 0
fi

# ── Step 1: Bootstrap EC2 ─────────────────────────────────────────────────────
if [[ "$SKIP_BOOTSTRAP" == false && -n "${QUEEN_IP:-}" ]]; then
  log "Step 1: Bootstrapping queen host ${QUEEN_IP} (k3s + iptables + CoreDNS) ..."
  sep
  "${SCRIPT_DIR}/bootstrap-ec2.sh"
  ok "Queen host bootstrap done"
else
  warn "Skipping bootstrap (--skip-bootstrap or QUEEN_IP not set)"
fi

# ── Step 2: Deploy Formicary queen ────────────────────────────────────────────
if [[ "$SKIP_QUEEN" == false ]]; then
  log "Step 2: Deploying Formicary queen to EC2 ..."
  sep
  "${SCRIPT_DIR}/deploy-formicary.sh" ${QUEEN_IP:+--queen-ip "$QUEEN_IP"}
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

  FORMICARY_URL="${FORMICARY_URL:-https://${QUEEN_IP}.nip.io}"

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
  FORMICARY_URL="${FORMICARY_URL:-https://${QUEEN_IP}.nip.io}" \
  FORMICARY_TOKEN="$FORMICARY_TOKEN" \
    "${SCRIPT_DIR}/setup-ant-worker.sh" 2>&1 | sed 's/^/  /'
  ok "Ant worker deployed"
else
  warn "Skipping ant worker deploy (--skip-ant)"
fi

# ── Done ──────────────────────────────────────────────────────────────────────
FORMICARY_URL="${FORMICARY_URL:-https://${QUEEN_IP:-localhost}.nip.io}"
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
