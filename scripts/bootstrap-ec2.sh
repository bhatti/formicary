#!/usr/bin/env bash
# bootstrap-ec2.sh — idempotent setup for a fresh EC2 instance running Formicary on k3s.
#
# Safe to re-run on an already-configured instance. Every step is guarded.
#
# Usage:
#   # From your laptop:
#   export EC2_IP=10.8.97.24
#   export EC2_KEY=~/Downloads/sbhatti-linux-key.pem   # default
#   ./scripts/bootstrap-ec2.sh
#
#   # Or run the embedded remote script directly on EC2:
#   sudo bash /path/to/bootstrap-ec2-remote.sh
#
# What it does:
#   1. Installs k3s (skipped if already running)
#   2. Creates /data/{seaweed,formicary} persistent dirs with correct perms
#   3. Installs iptables-restore systemd service (persists rules across reboots)
#   4. Configures CoreDNS to forward external queries to 8.8.8.8/1.1.1.1
#      instead of the VPN-only resolver in /etc/resolv.conf
#   5. Writes initial /etc/iptables.rules with KUBE-SERVICES guard
#   6. Adds a boot-time hook that re-inserts KUBE-SERVICES if k3s removed it
#
# After bootstrap, deploy Formicary with:
#   ./scripts/deploy-formicary.sh --ec2-ip $EC2_IP
#
set -euo pipefail

EC2_IP="${EC2_IP:-}"
EC2_KEY="${EC2_KEY:-${HOME}/Downloads/sbhatti-linux-key.pem}"
EC2_USER="${EC2_USER:-ec2-user}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

log()  { echo "▶ $*"; }
ok()   { echo "  ✓ $*"; }
fail() { echo "  ✗ ERROR: $*" >&2; exit 1; }

# ── SSH wrapper (run everything remotely via heredoc) ─────────────────────────
[[ -n "$EC2_IP" ]] || fail "EC2_IP is required — export EC2_IP=<ip>"
[[ -f "$EC2_KEY" ]] || fail "EC2 SSH key not found: $EC2_KEY"

SSH_CMD="ssh -i ${EC2_KEY} -o StrictHostKeyChecking=no -o ConnectTimeout=15 ${EC2_USER}@${EC2_IP}"
$SSH_CMD "echo ok" > /dev/null 2>&1 || fail "Cannot SSH to ${EC2_USER}@${EC2_IP}"
ok "SSH verified"

log "Running bootstrap on EC2 ${EC2_IP} ..."

$SSH_CMD 'sudo bash -s' << 'REMOTE_SCRIPT'
set -euo pipefail

log()  { echo "  ▶ $*"; }
ok()   { echo "    ✓ $*"; }
warn() { echo "    ⚠ $*"; }

# ── 1. k3s ───────────────────────────────────────────────────────────────────
if systemctl is-active --quiet k3s 2>/dev/null; then
  ok "k3s already running — skipping install"
else
  log "Installing k3s ..."
  # --disable traefik: we don't need the ingress controller
  curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik" sh -
  systemctl enable k3s
  # Wait for API server
  for i in $(seq 1 30); do
    if kubectl --kubeconfig /etc/rancher/k3s/k3s.yaml get nodes > /dev/null 2>&1; then
      ok "k3s ready"
      break
    fi
    sleep 2
  done
fi

export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

# ── 2. Persistent data directories ───────────────────────────────────────────
log "Creating persistent data dirs ..."
mkdir -p /data/seaweed /data/formicary
# k3s pods run as root inside containers; directories just need to exist
ok "/data/seaweed and /data/formicary ready"

# ── 3. iptables-restore service (persist rules across reboots) ───────────────
log "Configuring iptables persistence ..."
if ! systemctl is-enabled iptables-restore 2>/dev/null | grep -q enabled; then
  # Write a simple restore service — runs before k3s so rules are in place at boot
  cat > /etc/systemd/system/iptables-restore.service << 'SVC'
[Unit]
Description=Restore iptables rules
Before=k3s.service
DefaultDependencies=no

[Service]
Type=oneshot
ExecStart=/sbin/iptables-restore /etc/iptables.rules
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
SVC
  systemctl daemon-reload
  systemctl enable iptables-restore
  ok "iptables-restore service enabled"
else
  ok "iptables-restore service already enabled"
fi

# ── 4. KUBE-SERVICES guard service ───────────────────────────────────────────
# k3s adds KUBE-SERVICES to PREROUTING at startup.
# If iptables-restore runs WITHOUT KUBE-SERVICES and k3s doesn't re-add it
# (e.g. after a save without it), ClusterIP routing breaks for pods.
# This service checks after k3s is up and re-inserts if missing.
log "Installing KUBE-SERVICES guard service ..."
cat > /usr/local/bin/kube-services-guard.sh << 'GUARD'
#!/usr/bin/env bash
# Ensure KUBE-SERVICES is in PREROUTING so ClusterIP routing works from pods.
# k3s adds it at startup but a bad iptables-save can remove it from the
# persisted rules, breaking CoreDNS → API server and all pod → Service traffic.
for i in $(seq 1 30); do
  if iptables -t nat -L KUBE-SERVICES -n > /dev/null 2>&1; then
    if ! iptables -t nat -L PREROUTING -n | grep -q KUBE-SERVICES; then
      # Find the position of CNI-HOSTPORT-DNAT and insert before it
      POS=$(iptables -t nat -L PREROUTING -n --line-numbers \
            | awk '/CNI-HOSTPORT-DNAT/ {print $1; exit}')
      if [[ -n "$POS" ]]; then
        iptables -t nat -I PREROUTING "$POS" -m comment \
          --comment 'kubernetes service portals' -j KUBE-SERVICES
        echo "kube-services-guard: re-inserted KUBE-SERVICES at position $POS"
      else
        # CNI not yet loaded; append before position 1 safety
        iptables -t nat -A PREROUTING -m comment \
          --comment 'kubernetes service portals' -j KUBE-SERVICES
        echo "kube-services-guard: appended KUBE-SERVICES (CNI not found yet)"
      fi
    else
      echo "kube-services-guard: KUBE-SERVICES already present — nothing to do"
    fi
    exit 0
  fi
  sleep 2
done
echo "kube-services-guard: KUBE-SERVICES chain not found after 60s — k3s may not be ready"
exit 1
GUARD
chmod +x /usr/local/bin/kube-services-guard.sh

cat > /etc/systemd/system/kube-services-guard.service << 'SVC'
[Unit]
Description=Ensure KUBE-SERVICES is in iptables PREROUTING after k3s starts
After=k3s.service
Requires=k3s.service

[Service]
Type=oneshot
ExecStart=/usr/local/bin/kube-services-guard.sh
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
SVC
systemctl daemon-reload
systemctl enable kube-services-guard
ok "kube-services-guard service installed and enabled"

# ── 5. CoreDNS: use public DNS forwarders instead of /etc/resolv.conf ─────────
# /etc/resolv.conf on EC2 points to 10.8.0.2 (VPN internal resolver).
# CoreDNS uses "forward . /etc/resolv.conf" by default and forwards to it,
# but that resolver is unreachable from pod CIDR → external DNS fails for pods.
# Override to use 8.8.8.8/1.1.1.1 which are reachable from pods.
log "Patching CoreDNS to use public DNS forwarders ..."

# Wait for CoreDNS to be running before patching
for i in $(seq 1 30); do
  if kubectl get deployment coredns -n kube-system > /dev/null 2>&1; then
    break
  fi
  sleep 2
done

CURRENT_FORWARD=$(kubectl get configmap coredns -n kube-system \
  -o jsonpath='{.data.Corefile}' 2>/dev/null \
  | grep 'forward \.' | head -1 | xargs || true)

if echo "$CURRENT_FORWARD" | grep -q '8\.8\.8\.8'; then
  ok "CoreDNS already using public forwarders — skipping"
else
  # Patch the Corefile in place
  COREFILE=$(kubectl get configmap coredns -n kube-system \
    -o jsonpath='{.data.Corefile}')
  PATCHED=$(echo "$COREFILE" \
    | sed 's|forward \. /etc/resolv\.conf|forward . 8.8.8.8 1.1.1.1|g')

  # Apply via kubectl patch (no jq needed)
  python3 - << PYEOF
import subprocess, json, sys

cm = json.loads(subprocess.check_output([
    'kubectl', 'get', 'configmap', 'coredns', '-n', 'kube-system', '-o', 'json'
]).decode())

corefile = cm['data']['Corefile']
if 'forward . /etc/resolv.conf' in corefile:
    cm['data']['Corefile'] = corefile.replace(
        'forward . /etc/resolv.conf',
        'forward . 8.8.8.8 1.1.1.1'
    )
    patch_input = json.dumps(cm).encode()
    result = subprocess.run(
        ['kubectl', 'apply', '-f', '-'],
        input=patch_input, capture_output=True
    )
    if result.returncode == 0:
        print('    CoreDNS Corefile patched')
    else:
        print('    ERROR patching CoreDNS:', result.stderr.decode(), file=sys.stderr)
        sys.exit(1)
else:
    print('    CoreDNS already patched')
PYEOF

  # Restart CoreDNS to pick up the new config
  kubectl rollout restart deployment/coredns -n kube-system
  # Wait with a longer timeout — first pod needs to connect to API which takes a few seconds
  for i in $(seq 1 40); do
    READY=$(kubectl get deployment coredns -n kube-system \
      -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo 0)
    if [[ "$READY" == "1" ]]; then
      ok "CoreDNS restarted and ready"
      break
    fi
    sleep 3
    if [[ "$i" == "40" ]]; then
      warn "CoreDNS not ready after 120s — check 'kubectl logs -n kube-system deploy/coredns'"
    fi
  done
fi

# ── 6. Write initial /etc/iptables.rules if missing ──────────────────────────
if [[ ! -f /etc/iptables.rules ]]; then
  log "Writing initial /etc/iptables.rules ..."
  iptables-save > /etc/iptables.rules
  ok "/etc/iptables.rules created"
else
  ok "/etc/iptables.rules already exists"
fi

# ── 7. Run kube-services-guard now (in case we're on a live system) ───────────
log "Running KUBE-SERVICES guard now ..."
/usr/local/bin/kube-services-guard.sh

echo ""
echo "  ════════════════════════════════════════"
echo "  ✅  EC2 bootstrap complete"
echo ""
echo "  Next: deploy Formicary queen from your laptop:"
echo "    export EC2_IP=$(hostname -I | awk '{print $1}')"
echo "    export EC2_KEY=~/Downloads/sbhatti-linux-key.pem"
echo "    export COMMON_AUTH_JWT_SECRET=<secret>"
echo "    export SLACK_APP_TOKEN=<xapp-...>"
echo "    export SLACK_BOT_TOKEN=<xoxb-...>"
echo "    ./scripts/deploy-formicary.sh"
echo "  ════════════════════════════════════════"

REMOTE_SCRIPT

ok "Bootstrap complete on EC2 ${EC2_IP}"
