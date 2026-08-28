#!/usr/bin/env bash
# refresh-node-images.sh — purge cached container images from a k8s node's containerd
# and force-pull fresh versions.  Works on Docker Desktop k8s and k3s nodes.
#
# USAGE
#   refresh-node-images.sh [--node NODE] [--namespace NS] [--images IMG,IMG,...] [--no-restart]
#
# DEFAULTS
#   --node      desktop-control-plane
#   --namespace default
#   --images    plexobject/formicary:latest,plexobject/ai-dev-tools:latest
#
# EXAMPLES
#   refresh-node-images.sh
#   refresh-node-images.sh --node k3s-node
#   refresh-node-images.sh --node desktop-control-plane --images plexobject/formicary:latest

set -euo pipefail

log()  { printf "  ▶ %s\n" "$*"; }
ok()   { printf "  ✓ %s\n" "$*"; }
warn() { printf "  ⚠ %s\n" "$*" >&2; }
fail() { printf "  ✗ ERROR: %s\n" "$*" >&2; exit 1; }

NODE="${NODE:-desktop-control-plane}"
NAMESPACE="${NAMESPACE:-default}"
IMAGES="${IMAGES:-plexobject/formicary:latest,plexobject/ai-dev-tools:latest}"
NO_RESTART=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --node)       NODE="$2";      shift 2 ;;
    --namespace)  NAMESPACE="$2"; shift 2 ;;
    --images)     IMAGES="$2";    shift 2 ;;
    --no-restart) NO_RESTART=true; shift ;;
    -h|--help)
      grep '^#' "$0" | grep -v '#!/' | sed 's/^# *//'
      exit 0 ;;
    *) fail "Unknown argument: $1" ;;
  esac
done

# ── Build grep pattern and ctr rm list from comma-separated image list ─────────
# Turn "plexobject/formicary:latest,plexobject/ai-dev-tools:latest"
# into grep pattern "formicary\|ai-dev-tools" and explicit ctr rm calls
IFS=',' read -ra IMAGE_LIST <<< "$IMAGES"

GREP_PATTERN=""
CTR_RM_CMDS=""
PULL_PODS_JSON=""

for img in "${IMAGE_LIST[@]}"; do
  img="${img// /}"   # trim whitespace
  # short name for grep (strip registry prefix and tag)
  short=$(echo "$img" | sed 's|.*/||; s|:.*||')
  GREP_PATTERN="${GREP_PATTERN:+${GREP_PATTERN}\\|}${short}"
  # explicit crictl rmi call
  CTR_RM_CMDS="${CTR_RM_CMDS}crictl rmi '${img}' 2>/dev/null || true; "
done

# ── Step 1: Purge images from node containerd via privileged pod ───────────────
printf "\n"
log "Step 1: Purging cached images from node '${NODE}'..."
log "  Images: ${IMAGES}"

PURGE_CMD="ctr -n k8s.io images ls 2>/dev/null | grep -E '${GREP_PATTERN}' | awk '{print \$1}' | xargs -r -I{} ctr -n k8s.io images rm {} 2>/dev/null || true; ${CTR_RM_CMDS}echo 'purge done'"

kubectl run img-cleaner-$$ \
  --image=docker.io/library/alpine:latest \
  --restart=Never \
  --namespace="${NAMESPACE}" \
  --overrides="$(cat <<EOF
{
  "spec": {
    "nodeName": "${NODE}",
    "hostPID": true,
    "containers": [{
      "name": "c",
      "image": "alpine:latest",
      "command": ["sh", "-c", "${PURGE_CMD}"],
      "securityContext": {"privileged": true}
    }],
    "tolerations": [{"operator": "Exists"}]
  }
}
EOF
)" --rm -i --timeout=60s 2>/dev/null && ok "Images purged from containerd cache" \
  || warn "Purge pod failed or timed out — images may still be cached"

# ── Step 2: Force-pull each image on the node ─────────────────────────────────
printf "\n"
log "Step 2: Force-pulling fresh images on node '${NODE}'..."

for img in "${IMAGE_LIST[@]}"; do
  img="${img// /}"
  short=$(echo "$img" | sed 's|.*/||; s|:.*||')
  log "  Pulling ${img}..."
  kubectl run "img-pull-${short}-$$" \
    --image="${img}" \
    --restart=Never \
    --namespace="${NAMESPACE}" \
    --overrides="$(cat <<EOF
{
  "spec": {
    "nodeName": "${NODE}",
    "containers": [{
      "name": "c",
      "image": "${img}",
      "imagePullPolicy": "Always",
      "command": ["sh", "-c", "echo 'pulled ${img}'"]
    }],
    "tolerations": [{"operator": "Exists"}]
  }
}
EOF
)" --rm -i --timeout=120s 2>/dev/null \
    && ok "  Pulled ${img}" \
    || warn "  Pull pod failed for ${img} — check registry access"
done

# ── Step 3: Restart ant deployment ────────────────────────────────────────────
if ! ${NO_RESTART}; then
  printf "\n"
  log "Step 3: Restarting formicary-ant deployment..."
  if kubectl get deployment formicary-ant --namespace="${NAMESPACE}" &>/dev/null; then
    kubectl rollout restart deployment/formicary-ant --namespace="${NAMESPACE}"
    kubectl rollout status deployment/formicary-ant --namespace="${NAMESPACE}" --timeout=90s \
      && ok "formicary-ant restarted with fresh images" \
      || warn "Rollout timed out — check: kubectl get pods -n ${NAMESPACE}"
  else
    warn "formicary-ant deployment not found in namespace '${NAMESPACE}' — skipping restart"
  fi
fi

printf "\n"
ok "Done. Images refreshed on node '${NODE}'."
