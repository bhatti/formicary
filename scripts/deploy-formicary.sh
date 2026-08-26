#!/usr/bin/env bash
# deploy-formicary.sh — deploy the Formicary queen to EC2 k3s.
#
# Usage:
#   export EC2_IP=YOUR_EC2_IP
#   export EC2_KEY=~/Downloads/sbhatti-linux-key.pem
#   ./scripts/deploy-formicary.sh          # full deploy (secrets + manifest + DNAT)
#   ./scripts/deploy-formicary.sh --restart  # pull latest image on EC2, redeploy queen + refresh DNAT
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
#   EC2_IP                              (or --ec2-ip)
#   EC2_KEY                             SSH key path (default: ~/Downloads/sbhatti-linux-key.pem)
#   COMMON_AUTH_JWT_SECRET
#
# Optional env vars:
#   COMMON_AUTH_GOOGLE_CLIENT_ID / SECRET / CALLBACK_HOST
#   SLACK_BOT_TOKEN / SLACK_APP_TOKEN / SLACK_SIGNING_SECRET / SLACK_CHANNEL
#   FORMICARY_TOKEN                     JWT token — required for org config API updates
#   FORMICARY_URL                       defaults to https://YOUR_EC2_IP.nip.io
#   EC2_USER                            (default: ec2-user)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

EC2_IP="${EC2_IP:-}"
EC2_KEY="${EC2_KEY:-${HOME}/Downloads/sbhatti-linux-key.pem}"
EC2_USER="${EC2_USER:-ec2-user}"
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
    --ec2-ip)         EC2_IP="$2";           shift 2 ;;
    --ec2-key)        EC2_KEY="$2";          shift 2 ;;
    --ec2-user)       EC2_USER="$2";         shift 2 ;;
    --all-in-one)     ALL_IN_ONE=true;       shift ;;
    --status)         SHOW_STATUS=true;      shift ;;
    --logs)           SHOW_LOGS=true;        shift ;;
    --restart|--rollout-restart) ROLLOUT_RESTART=true; shift ;;
    --sync-scripts)   SYNC_SCRIPTS=true;    shift ;;
    *) echo "Unknown flag: $1" >&2; exit 1 ;;
  esac
done

# ── EC2 kubectl wrapper ────────────────────────────────────────────────────────
# EC2's k3s API (port 6443) is not open externally. Every kubectl call is
# transparently remapped to run on EC2 via SSH.
if [[ -n "$EC2_IP" ]]; then
  [[ -f "$EC2_KEY" ]] || fail "EC2 SSH key not found: $EC2_KEY (pass --ec2-key or set EC2_KEY)"
  SSH_CMD="ssh -i ${EC2_KEY} -o StrictHostKeyChecking=no -o ConnectTimeout=10 ${EC2_USER}@${EC2_IP}"
  $SSH_CMD "echo ok" > /dev/null 2>&1 || fail "Cannot SSH to ${EC2_USER}@${EC2_IP} with key ${EC2_KEY}"
  ok "SSH to EC2 at ${EC2_IP} verified"
  kubectl() {
    local quoted
    quoted=$(printf ' %q' "$@")
    $SSH_CMD "KUBECONFIG=/etc/rancher/k3s/k3s.yaml kubectl${quoted}"
  }
fi

# ── Sync scripts/k8s manifests to EC2 ────────────────────────────────────────
if [[ "$SYNC_SCRIPTS" == true ]]; then
  [[ -n "$EC2_IP" ]] || fail "--sync-scripts requires EC2_IP"
  log "Syncing scripts/ and k8s/ to EC2 ~/formicary/"
  $SSH_CMD "mkdir -p ~/formicary/scripts ~/formicary/k8s ~/formicary/docs/examples"
  rsync -az --delete -e "ssh -i ${EC2_KEY} -o StrictHostKeyChecking=no" \
    "${REPO_ROOT}/scripts/"       "${EC2_USER}@${EC2_IP}:~/formicary/scripts/"
  rsync -az --delete -e "ssh -i ${EC2_KEY} -o StrictHostKeyChecking=no" \
    "${REPO_ROOT}/k8s/"           "${EC2_USER}@${EC2_IP}:~/formicary/k8s/"
  rsync -az --delete -e "ssh -i ${EC2_KEY} -o StrictHostKeyChecking=no" \
    "${REPO_ROOT}/docs/examples/" "${EC2_USER}@${EC2_IP}:~/formicary/docs/examples/"
  $SSH_CMD "chmod +x ~/formicary/scripts/*.sh ~/formicary/docs/examples/*.sh 2>/dev/null || true"
  ok "scripts/, k8s/, docs/examples/ synced to EC2"
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
    if [[ -n "$EC2_IP" ]]; then
      echo "    $0 --ec2-ip ${EC2_IP}"
    else
      echo "    $0"
    fi
    exit 1
  fi
  kubectl logs -l app=formicary --tail=100 -f 2>&1
  exit 0
fi

# ── DNAT refresh (EC2 only) ───────────────────────────────────────────────────
# Pod IP changes on every rollout restart. This refreshes iptables rules so
# port 443 → pod:7777 (HTTPS/WSS) and port 4443 → pod:19000 (S3) stay correct.
refresh_dnat() {
  if [[ -z "$EC2_IP" ]]; then return; fi
  log "Refreshing iptables DNAT rules on EC2 (pod IP changed after restart)"
  $SSH_CMD "
    NEW_IP=\$(kubectl get pod -l app=formicary -o jsonpath='{.items[0].status.podIP}')
    echo \"  Pod IP: \$NEW_IP\"
    # Remove any existing formicary DNAT rules by matching their comment or destination pattern.
    # Using positional delete (-D PREROUTING N) is fragile — iptables accumulates stale rules
    # that can corrupt KUBE-SERVICES insertion order and break ClusterIP routing.
    sudo iptables -t nat -L PREROUTING -n --line-numbers \
      | awk '/to:[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+:(7777|19000)/ {print \$1}' \
      | sort -rn \
      | while read -r n; do sudo iptables -t nat -D PREROUTING "\$n" 2>/dev/null || true; done

    sudo iptables -t nat -I PREROUTING 1 -p tcp --dport 443  ! -s 10.42.0.0/16 -j DNAT --to-destination \${NEW_IP}:7777
    sudo iptables -t nat -I PREROUTING 2 -p tcp --dport 4443 ! -s 10.42.0.0/16 -j DNAT --to-destination \${NEW_IP}:19000

    # Ensure KUBE-SERVICES is in PREROUTING (k3s sometimes loses it after iptables-restore at boot).
    if ! sudo iptables -t nat -L PREROUTING -n | grep -q KUBE-SERVICES; then
      echo "  Reinserting KUBE-SERVICES into PREROUTING"
      sudo iptables -t nat -I PREROUTING 3 -m comment --comment 'kubernetes service portals' -j KUBE-SERVICES
    fi

    sudo iptables-save | sudo tee /etc/iptables.rules > /dev/null
  "
  ok "DNAT rules updated — https://${EC2_IP}.nip.io should be reachable"
}

if [[ "$ROLLOUT_RESTART" == true ]]; then
  [[ -n "$EC2_IP" ]] || fail "--restart requires EC2_IP (or --ec2-ip)"
  IMG="docker.io/plexobject/formicary:${FORMICARY_VERSION}"
  log "Deploying version ${FORMICARY_VERSION} — pulling ${IMG} on EC2 via crictl"
  $SSH_CMD "
    set -e
    # crictl always operates in the k8s.io containerd namespace that kubelet uses.
    # 'ctr' without -n k8s.io pulls into the 'default' namespace — invisible to k3s.
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

  # Push Slack routes and smoke test after restart
  _FURL="${FORMICARY_URL:-https://${EC2_IP}.nip.io}"
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
  JWT_SECRET="${COMMON_AUTH_JWT_SECRET:-}"
fi
if [[ -z "$JWT_SECRET" ]]; then
  fail "COMMON_AUTH_JWT_SECRET is required — set it in ~/.zshrc or export it before running"
fi

log "Deploying Formicary queen"
[[ -n "$EC2_IP" ]] && echo "  Target: EC2 ${EC2_IP}" || echo "  Target: local kubectl context"

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

# Substitute FORMICARY_VERSION placeholder in the manifest before applying.
RENDERED_MANIFEST="$(sed "s|plexobject/formicary:FORMICARY_VERSION|plexobject/formicary:${FORMICARY_VERSION}|g" "${MANIFEST}")"

if [[ -n "$EC2_IP" ]]; then
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
# Requires FORMICARY_TOKEN (JWT) and FORMICARY_URL.
FORMICARY_TOKEN="${FORMICARY_TOKEN:-}"
_FURL="${FORMICARY_URL:-https://$(echo "${EC2_IP:-localhost}").nip.io}"
if [[ -n "$FORMICARY_TOKEN" && -n "$SLACK_CHANNEL" ]]; then
  log "Updating org config SlackChannel=$SLACK_CHANNEL via API"
  # Decode org_id from JWT payload
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
# The route table must be in the DB for tracker-based routing to work.
# Routes are reloaded from DB within 30 seconds — no restart needed.
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
        # Count routes that have tracker_variants
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
