#!/usr/bin/env bash
# deploy-formicary.sh — deploy the Formicary queen to a remote k3s host.
#
# Usage:
#   export QUEEN_IP=YOUR_HOST_IP
#   export QUEEN_SSH_KEY=~/.ssh/id_rsa   # optional — uses SSH agent if unset
#   ./scripts/deploy-formicary.sh          # full deploy (secrets + manifest + DNAT)
#   ./scripts/deploy-formicary.sh --restart  # pull latest image, redeploy queen + refresh DNAT
#   ./scripts/deploy-formicary.sh --status   # show pod status
#   ./scripts/deploy-formicary.sh --logs     # tail queen logs
#
# What it does:
#   1. Create/update 'formicary-auth' k8s secret from env vars
#   2. Create/update 'formicary-slack' k8s secret (skipped if SLACK_APP_TOKEN unset)
#   3. Apply k8s/formicary-leader.yaml
#   4. Wait for rollout, then refresh iptables DNAT rules with the new pod IP
#      (port 443 → pod:7777, port 4443 → pod:19000)
#   5. If FORMICARY_TOKEN + SLACK_CHANNEL set: update org-level SlackChannel config via API
#   6. If FORMICARY_TOKEN set: push Slack route table (with tracker_variants) via setup-slack-admin.sh
#   7. Smoke test: verify /api/health returns 200 and Slack routes are loaded
#
# Required env vars:
#   QUEEN_IP                            (or --queen-ip)
#   COMMON_AUTH_JWT_SECRET
#
# Optional env vars:
#   QUEEN_SSH_KEY                       SSH key path (default: uses SSH agent)
#   QUEEN_SSH_USER                      SSH user (default: no explicit user — uses SSH config)
#   COMMON_AUTH_GOOGLE_CLIENT_ID / SECRET / CALLBACK_HOST
#   SLACK_BOT_TOKEN / SLACK_APP_TOKEN / SLACK_SIGNING_SECRET / SLACK_CHANNEL
#   FORMICARY_TOKEN                     JWT token — required for org config API updates
#   FORMICARY_URL                       defaults to https://QUEEN_IP.nip.io
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

QUEEN_IP="${QUEEN_IP:-}"
QUEEN_SSH_KEY="${QUEEN_SSH_KEY:-}"
QUEEN_SSH_USER="${QUEEN_SSH_USER:-}"
ALL_IN_ONE=false
SHOW_STATUS=false
SHOW_LOGS=false
ROLLOUT_RESTART=false
SYNC_SCRIPTS=false

# Compute the image version the same way the Makefile does.
FORMICARY_VERSION="${FORMICARY_VERSION:-0.1.$(git -C "${REPO_ROOT}" rev-list --count HEAD 2>/dev/null || echo 0)}"

log()  { echo "▶ $*"; }
ok()   { echo "  ✓ $*"; }
fail() { echo "  ✗ ERROR: $*" >&2; exit 1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --queen-ip)       QUEEN_IP="$2";       shift 2 ;;
    --queen-ssh-key)  QUEEN_SSH_KEY="$2";  shift 2 ;;
    --queen-ssh-user) QUEEN_SSH_USER="$2"; shift 2 ;;
    --all-in-one)     ALL_IN_ONE=true;     shift ;;
    --status)         SHOW_STATUS=true;    shift ;;
    --logs)           SHOW_LOGS=true;      shift ;;
    --restart|--rollout-restart) ROLLOUT_RESTART=true; shift ;;
    --sync-scripts)   SYNC_SCRIPTS=true;  shift ;;
    *) echo "Unknown flag: $1" >&2; exit 1 ;;
  esac
done

# ── Autodetect credentials from shell RC files ────────────────────────────────
# Reads `export VAR=value` lines; expands ${VAR} and $(...) references.
_autodetect_var() {
  local _var="$1"
  if [[ -z "${!_var:-}" ]]; then
    for _rc in "$HOME/.zshrc" "$HOME/.bashrc"; do
      [[ -f "$_rc" ]] || continue
      local _line _val
      _line="$(grep -E "^export ${_var}=" "$_rc" | tail -1)" || true
      if [[ -n "$_line" ]]; then
        _val="$(echo "$_line" | sed "s/^export ${_var}=//;s/^['\"]//;s/['\"]$//")"
        _val="$(eval echo "\"${_val}\"" 2>/dev/null || echo "${_val}")"
        if [[ -n "$_val" ]]; then
          export "${_var}=${_val}"
          break
        fi
      fi
    done
  fi
}
# QUEEN_IP first so FORMICARY_URL="https://${QUEEN_IP}.nip.io" expands correctly
for _v in QUEEN_IP QUEEN_SSH_KEY QUEEN_SSH_USER FORMICARY_URL FORMICARY_TOKEN \
          COMMON_AUTH_JWT_SECRET COMMON_AUTH_GOOGLE_CLIENT_ID COMMON_AUTH_GOOGLE_CLIENT_SECRET \
          COMMON_AUTH_GOOGLE_CALLBACK_HOST COMMON_AUTH_SECURE \
          SLACK_APP_TOKEN SLACK_BOT_TOKEN SLACK_SIGNING_SECRET SLACK_CHANNEL; do
  _autodetect_var "$_v"
done

# ── SSH command builder ───────────────────────────────────────────────────────
# Builds SSH command respecting optional key and user.
_ssh_cmd() {
  local cmd=(ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10)
  [[ -n "$QUEEN_SSH_KEY" ]] && cmd+=(-i "$QUEEN_SSH_KEY")
  if [[ -n "$QUEEN_SSH_USER" ]]; then
    cmd+=("${QUEEN_SSH_USER}@${QUEEN_IP}")
  else
    cmd+=("${QUEEN_IP}")
  fi
  printf '%q ' "${cmd[@]}"
}

if [[ -n "$QUEEN_IP" ]]; then
  [[ -n "$QUEEN_SSH_KEY" && ! -f "$QUEEN_SSH_KEY" ]] && \
    fail "SSH key not found: $QUEEN_SSH_KEY (set QUEEN_SSH_KEY or leave unset to use SSH agent)"
  SSH_CMD="$(_ssh_cmd)"
  $SSH_CMD "echo ok" > /dev/null 2>&1 || fail "Cannot SSH to ${QUEEN_SSH_USER:+${QUEEN_SSH_USER}@}${QUEEN_IP}"
  ok "SSH to ${QUEEN_SSH_USER:+${QUEEN_SSH_USER}@}${QUEEN_IP} verified"
  kubectl() {
    local quoted
    quoted=$(printf ' %q' "$@")
    $SSH_CMD "KUBECONFIG=/etc/rancher/k3s/k3s.yaml kubectl${quoted}"
  }
fi

# ── Sync scripts/k8s manifests to remote host ────────────────────────────────
if [[ "$SYNC_SCRIPTS" == true ]]; then
  [[ -n "$QUEEN_IP" ]] || fail "--sync-scripts requires QUEEN_IP"
  log "Syncing scripts/ and k8s/ to ~/formicary/ on ${QUEEN_IP}"
  $SSH_CMD "mkdir -p ~/formicary/scripts ~/formicary/k8s ~/formicary/docs/examples"
  _rsync_ssh="ssh -o StrictHostKeyChecking=no${QUEEN_SSH_KEY:+ -i $QUEEN_SSH_KEY}"
  _rsync_dest="${QUEEN_SSH_USER:+${QUEEN_SSH_USER}@}${QUEEN_IP}"
  rsync -az --delete -e "$_rsync_ssh" \
    "${REPO_ROOT}/scripts/"       "${_rsync_dest}:~/formicary/scripts/"
  rsync -az --delete -e "$_rsync_ssh" \
    "${REPO_ROOT}/k8s/"           "${_rsync_dest}:~/formicary/k8s/"
  rsync -az --delete -e "$_rsync_ssh" \
    "${REPO_ROOT}/docs/examples/" "${_rsync_dest}:~/formicary/docs/examples/"
  $SSH_CMD "chmod +x ~/formicary/scripts/*.sh ~/formicary/docs/examples/*.sh 2>/dev/null || true"
  ok "scripts/, k8s/, docs/examples/ synced"
fi

# ── Status / logs shortcuts ───────────────────────────────────────────────────
if [[ "$SHOW_STATUS" == true ]]; then
  kubectl get pods -l app=formicary 2>&1 || true
  exit 0
fi

if [[ "$SHOW_LOGS" == true ]]; then
  if ! kubectl get deployment formicary > /dev/null 2>&1; then
    echo ""
    echo "  ✗ No formicary deployment found on the cluster."
    echo "  Deploy it first:"
    if [[ -n "$QUEEN_IP" ]]; then
      echo "    $0 --queen-ip ${QUEEN_IP}"
    else
      echo "    $0"
    fi
    exit 1
  fi
  kubectl logs -l app=formicary --tail=100 -f 2>&1
  exit 0
fi

# ── DNAT refresh ─────────────────────────────────────────────────────────────
# Pod IP changes on every rollout restart. This refreshes iptables rules so
# port 443 → pod:7777 (HTTPS/WSS) and port 4443 → pod:19000 (S3) stay correct.
refresh_dnat() {
  if [[ -z "$QUEEN_IP" ]]; then return; fi
  log "Refreshing iptables DNAT rules (pod IP changed after restart)"
  $SSH_CMD "
    NEW_IP=\$(kubectl get pod -l app=formicary -o jsonpath='{.items[0].status.podIP}')
    echo \"  Pod IP: \$NEW_IP\"
    sudo iptables -t nat -L PREROUTING -n --line-numbers \
      | awk '/to:[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+:(7777|19000)/ {print \$1}' \
      | sort -rn \
      | while read -r n; do sudo iptables -t nat -D PREROUTING \"\$n\" 2>/dev/null || true; done

    sudo iptables -t nat -I PREROUTING 1 -p tcp --dport 443  ! -s 10.42.0.0/16 -j DNAT --to-destination \${NEW_IP}:7777
    sudo iptables -t nat -I PREROUTING 2 -p tcp --dport 4443 ! -s 10.42.0.0/16 -j DNAT --to-destination \${NEW_IP}:19000

    if ! sudo iptables -t nat -L PREROUTING -n | grep -q KUBE-SERVICES; then
      echo \"  Reinserting KUBE-SERVICES into PREROUTING\"
      sudo iptables -t nat -I PREROUTING 3 -m comment --comment 'kubernetes service portals' -j KUBE-SERVICES
    fi

    sudo iptables-save | sudo tee /etc/iptables.rules > /dev/null
  "
  ok "DNAT rules updated — https://${QUEEN_IP}.nip.io should be reachable"
}

if [[ "$ROLLOUT_RESTART" == true ]]; then
  [[ -n "$QUEEN_IP" ]] || fail "--restart requires QUEEN_IP (or --queen-ip)"
  IMG="docker.io/plexobject/formicary:${FORMICARY_VERSION}"
  log "Deploying version ${FORMICARY_VERSION} — pulling ${IMG} via crictl"
  $SSH_CMD "
    set -e
    echo '  Pulling ${IMG}...'
    sudo crictl pull '${IMG}'
    echo '  Setting deployment image to ${IMG}'
    KUBECONFIG=/etc/rancher/k3s/k3s.yaml kubectl set image deployment/formicary 'formicary=${IMG}'
    KUBECONFIG=/etc/rancher/k3s/k3s.yaml kubectl rollout restart deployment/formicary
    KUBECONFIG=/etc/rancher/k3s/k3s.yaml kubectl rollout status deployment/formicary --timeout=120s
  "
  refresh_dnat
  ok "Queen deployed at version ${FORMICARY_VERSION}"
  kubectl get pods -l app=formicary

  _FURL="${FORMICARY_URL:-https://${QUEEN_IP}.nip.io}"
  _SETUP="${REPO_ROOT}/docs/examples/setup-slack-admin.sh"
  if [[ -n "${FORMICARY_TOKEN:-}" && -f "$_SETUP" ]]; then
    log "Pushing Slack route table (tracker_variants)"
    FORMICARY_TOKEN="${FORMICARY_TOKEN}" FORMICARY_URL="${_FURL}" \
      bash "${_SETUP}" --set-routes --server "${_FURL}" 2>&1 \
      | grep -E '✓|✗|ERROR|WARNING|⚠|routes' || true
  else
    echo "  ⚠ Set FORMICARY_TOKEN to auto-push Slack routes after restart"
  fi
  _HC=$(curl -sk -o /dev/null -w "%{http_code}" "${_FURL}/api/health" 2>/dev/null) || _HC="000"
  [[ "$_HC" == "200" ]] && ok "Health check passed" || echo "  ⚠ Health HTTP ${_HC} — queen may still be starting"
  echo ""
  echo "  Verify routes: ${_FURL}/dashboard/slack/routes"
  exit 0
fi

# ── Validate required env vars ────────────────────────────────────────────────
JWT_SECRET="${COMMON_AUTH_JWT_SECRET:-}"
if [[ -z "$JWT_SECRET" ]]; then
  fail "COMMON_AUTH_JWT_SECRET is required — set it in ~/.zshrc or export it before running"
fi

log "Deploying Formicary queen"
[[ -n "$QUEEN_IP" ]] && echo "  Target: ${QUEEN_IP}" || echo "  Target: local kubectl context"

# ── Step 1: Create/update formicary-auth secret ───────────────────────────────
log "Creating/updating formicary-auth secret"
kubectl create secret generic formicary-auth \
  --from-literal=jwt-secret="${JWT_SECRET}" \
  --from-literal=google-client-id="${COMMON_AUTH_GOOGLE_CLIENT_ID:-}" \
  --from-literal=google-client-secret="${COMMON_AUTH_GOOGLE_CLIENT_SECRET:-}" \
  --from-literal=google-callback-host="${COMMON_AUTH_GOOGLE_CALLBACK_HOST:-}" \
  --from-literal=auth-secure="${COMMON_AUTH_SECURE:-false}" \
  --save-config --dry-run=client -o yaml | kubectl apply -f -
ok "formicary-auth secret applied"

# ── Step 2: Create/update formicary-slack secret (optional) ──────────────────
SLACK_APP_TOKEN="${SLACK_APP_TOKEN:-}"
SLACK_BOT_TOKEN="${SLACK_BOT_TOKEN:-}"
SLACK_CHANNEL="${SLACK_CHANNEL:-}"
if [[ -n "$SLACK_APP_TOKEN" ]]; then
  log "Creating/updating formicary-slack secret"
  kubectl create secret generic formicary-slack \
    --from-literal=app-token="${SLACK_APP_TOKEN}" \
    --from-literal=bot-token="${SLACK_BOT_TOKEN}" \
    --from-literal=signing-secret="${SLACK_SIGNING_SECRET:-}" \
    --from-literal=slack-channel="${SLACK_CHANNEL:-}" \
    --save-config --dry-run=client -o yaml | kubectl apply -f -
  ok "formicary-slack secret applied (Socket Mode enabled)"
else
  ok "SLACK_APP_TOKEN not set — Slack Socket Mode will be disabled"
fi

# ── Step 3: Apply formicary manifest ─────────────────────────────────────────
if [[ "$ALL_IN_ONE" == true ]]; then
  MANIFEST="${REPO_ROOT}/k8s/formicary-all-in-one.yaml"
else
  MANIFEST="${REPO_ROOT}/k8s/formicary-leader.yaml"
fi

[[ -f "$MANIFEST" ]] || fail "Manifest not found: ${MANIFEST}"
log "Applying $(basename "${MANIFEST}") with version ${FORMICARY_VERSION}"

RENDERED_MANIFEST="$(sed "s|plexobject/formicary:FORMICARY_VERSION|plexobject/formicary:${FORMICARY_VERSION}|g" "${MANIFEST}")"

if [[ -n "$QUEEN_IP" ]]; then
  echo "${RENDERED_MANIFEST}" | $SSH_CMD "KUBECONFIG=/etc/rancher/k3s/k3s.yaml kubectl apply -f -"
else
  echo "${RENDERED_MANIFEST}" | kubectl apply -f -
fi

kubectl rollout status deployment/formicary --timeout=120s
refresh_dnat
ok "Formicary deployed"
echo ""
kubectl get pods -l app=formicary

# ── Step 4: Update org-level configs via API (optional) ──────────────────────
FORMICARY_TOKEN="${FORMICARY_TOKEN:-}"
_FURL="${FORMICARY_URL:-https://$(echo "${QUEEN_IP:-localhost}").nip.io}"
if [[ -n "$FORMICARY_TOKEN" && -n "$SLACK_CHANNEL" ]]; then
  log "Updating org config SlackChannel=$SLACK_CHANNEL via API"
  _ORG_ID=$(python3 -c "
import sys, json, base64
t=sys.argv[1]; p=t.split('.')
if len(p)!=3: sys.exit(1)
pad=4-len(p[1])%4
d=json.loads(base64.urlsafe_b64decode(p[1]+'='*pad))
print(d.get('org_id',''))
" "$FORMICARY_TOKEN" 2>/dev/null || echo "")
  if [[ -z "$_ORG_ID" ]]; then
    echo "  ⚠ Could not decode org_id from FORMICARY_TOKEN — skipping SlackChannel update"
  else
    _HC=$(curl -sk -o /tmp/fq-resp.json -w "%{http_code}" \
      -X POST "${_FURL}/api/orgs/${_ORG_ID}/configs" \
      -H "Authorization: Bearer ${FORMICARY_TOKEN}" \
      -H "Content-Type: application/json" \
      -d "{\"name\":\"SlackChannel\",\"value\":\"${SLACK_CHANNEL}\",\"secret\":false}" 2>/dev/null) || _HC="000"
    case "$_HC" in
      2*) ok "OrgConfig SlackChannel=$SLACK_CHANNEL updated" ;;
      000) echo "  ⚠ Cannot reach ${_FURL} — SlackChannel not updated (server may still be starting)" ;;
      401|403) echo "  ⚠ SlackChannel update: HTTP ${_HC} — check FORMICARY_TOKEN" ;;
      *) echo "  ⚠ SlackChannel update HTTP ${_HC}: $(cat /tmp/fq-resp.json 2>/dev/null || true)" ;;
    esac
    rm -f /tmp/fq-resp.json
  fi
fi

# ── Step 6: Push Slack route table (tracker_variants) ────────────────────────
if [[ -n "$FORMICARY_TOKEN" ]]; then
  log "Pushing Slack route table (tracker_variants) via setup-slack-admin.sh"
  _SETUP="${REPO_ROOT}/docs/examples/setup-slack-admin.sh"
  if [[ -f "$_SETUP" ]]; then
    FORMICARY_TOKEN="${FORMICARY_TOKEN}" FORMICARY_URL="${_FURL}" \
      bash "${_SETUP}" --set-routes --server "${_FURL}" 2>&1 \
      | grep -E '✓|✗|ERROR|WARNING|⚠|routes' || true
  else
    echo "  ⚠ ${_SETUP} not found — Slack routes not pushed (run manually)"
  fi
else
  echo "  ⚠ FORMICARY_TOKEN not set — skipping Slack route push (run ./docs/examples/setup-slack-admin.sh manually)"
fi

# ── Step 7: Smoke test ───────────────────────────────────────────────────────
log "Smoke test: verifying queen health and Slack route config"
_HEALTH_CODE=$(curl -sk -o /tmp/fq-health.json -w "%{http_code}" \
  "${_FURL}/api/health" 2>/dev/null) || _HEALTH_CODE="000"
case "$_HEALTH_CODE" in
  200) ok "Health check passed (HTTP 200)" ;;
  000) echo "  ⚠ Cannot reach ${_FURL}/api/health — queen may still be starting; check with --status or --logs" ;;
  *)   echo "  ⚠ Health check returned HTTP ${_HEALTH_CODE}: $(cat /tmp/fq-health.json 2>/dev/null | head -1 || true)" ;;
esac
rm -f /tmp/fq-health.json

if [[ -n "$FORMICARY_TOKEN" ]]; then
  _ROUTES_CODE=$(curl -sk -o /tmp/fq-routes.json -w "%{http_code}" \
    -H "Authorization: Bearer ${FORMICARY_TOKEN}" \
    "${_FURL}/api/system-configs?kind=JSON&name=SlackRoutes" 2>/dev/null) || _ROUTES_CODE="000"
  if [[ "$_ROUTES_CODE" == "200" ]]; then
    _ROUTE_COUNT=$(python3 -c "
import sys, json
try:
    d = json.load(open('/tmp/fq-routes.json'))
    records = d.get('records', d if isinstance(d, list) else [])
    if records:
        routes = json.loads(records[0].get('value','[]'))
        tv = sum(1 for r in routes if r.get('tracker_variants'))
        print(f'{len(routes)} routes, {tv} with tracker_variants')
    else:
        print('no SlackRoutes config found')
except Exception as e:
    print(f'parse error: {e}')
" 2>/dev/null || echo "parse error")
    ok "Slack routes in DB: ${_ROUTE_COUNT}"
  elif [[ "$_ROUTES_CODE" == "000" ]]; then
    echo "  ⚠ Cannot reach queen — skipping route verification"
  else
    echo "  ⚠ Route check HTTP ${_ROUTES_CODE} — verify manually at ${_FURL}/dashboard/slack/routes"
  fi
  rm -f /tmp/fq-routes.json
fi

echo ""
echo "Deploy complete. Slack routing:"
echo "  • Route table is in DB with tracker_variants (github→ai-gh-implement etc.)"
echo "  • Routes auto-reload within 30 seconds — no restart needed after config changes"
echo "  • Verify: ${_FURL}/dashboard/slack/routes"
