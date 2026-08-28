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

IFS=',' read -ra IMAGE_LIST <<< "$IMAGES"

# ── Helper: run a privileged pod, wait for completion, print logs, delete ──────
# Usage: _run_privileged_pod POD_NAME CMD TIMEOUT_SECS
_run_privileged_pod() {
  local POD="$1" CMD="$2" TIMEOUT="${3:-90}"

  # Delete any leftover pod from a previous run
  kubectl delete pod "${POD}" --namespace="${NAMESPACE}" --ignore-not-found --grace-period=0 2>/dev/null || true

  kubectl run "${POD}" \
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
      "command": ["sh", "-c", "${CMD}"],
      "securityContext": {"privileged": true},
      "volumeMounts": [{"name": "run", "mountPath": "/run"}]
    }],
    "volumes": [{"name": "run", "hostPath": {"path": "/run"}}],
    "tolerations": [{"operator": "Exists"}]
  }
}
EOF
)" 2>/dev/null || true  # kubectl run failure is non-fatal; pod phase check below will catch it

  # Wait for pod to reach Succeeded or Failed (no -i/--rm — they hang without a TTY)
  local elapsed=0
  local phase=""
  while [[ $elapsed -lt $TIMEOUT ]]; do
    phase=$(kubectl get pod "${POD}" --namespace="${NAMESPACE}" \
              -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
    if [[ "$phase" == "Succeeded" || "$phase" == "Failed" ]]; then
      break
    fi
    sleep 2
    (( elapsed += 2 ))
  done

  # Print logs regardless of exit status so the user can see what happened
  kubectl logs "${POD}" --namespace="${NAMESPACE}" 2>/dev/null || true

  # Capture exit code
  local exit_code
  exit_code=$(kubectl get pod "${POD}" --namespace="${NAMESPACE}" \
    -o jsonpath='{.status.containerStatuses[0].state.terminated.exitCode}' 2>/dev/null || echo "1")

  # Clean up
  kubectl delete pod "${POD}" --namespace="${NAMESPACE}" --ignore-not-found --grace-period=0 2>/dev/null || true

  # Warn clearly if pod never completed (Pending/Running/Unknown/empty after timeout)
  if [[ "$phase" != "Succeeded" && "$phase" != "Failed" ]]; then
    warn "Pod '${POD}' timed out after ${TIMEOUT}s (phase: ${phase:-unknown}) — images may not be refreshed"
    return 1
  fi
  return "${exit_code:-0}"
}

# ── Step 1: Purge images from node containerd via privileged pod ───────────────
printf "\n"
log "Step 1: Purging cached images from node '${NODE}'..."
log "  Images: ${IMAGES}"

PURGE_CMD=""
for img in "${IMAGE_LIST[@]}"; do
  img="${img// /}"
  PURGE_CMD="${PURGE_CMD}echo '  removing ${img}...'; crictl rmi '${img}' 2>/dev/null && echo '  removed: ${img}' || echo '  not cached (ok): ${img}'; "
done
PURGE_CMD="${PURGE_CMD}echo 'purge done'"

if _run_privileged_pod "img-purge-$$" "${PURGE_CMD}" 60; then
  ok "Purge step complete"
else
  warn "Purge had errors (non-fatal — images may not have been cached)"
fi

# ── Step 2: Force-pull fresh images on the node via crictl ────────────────────
printf "\n"
log "Step 2: Force-pulling fresh images on node '${NODE}'..."

PULL_CMD=""
for img in "${IMAGE_LIST[@]}"; do
  img="${img// /}"
  PULL_CMD="${PULL_CMD}echo '  pulling ${img}...'; crictl pull '${img}' && echo '  pulled: ${img}' || echo '  WARN: pull failed for ${img}'; "
done
PULL_CMD="${PULL_CMD}echo 'pull done'"

if _run_privileged_pod "img-pull-$$" "${PULL_CMD}" 180; then
  ok "Pull step complete"
else
  warn "Pull had errors — check output above for per-image status"
fi

# ── Step 3: Restart ant deployment and wait for pod ready ─────────────────────
if ! ${NO_RESTART}; then
  printf "\n"
  log "Step 3: Restarting formicary-ant deployment..."
  if ! kubectl get deployment formicary-ant --namespace="${NAMESPACE}" &>/dev/null; then
    warn "formicary-ant deployment not found in namespace '${NAMESPACE}' — skipping restart"
  else
    kubectl rollout restart deployment/formicary-ant --namespace="${NAMESPACE}"

    # With strategy: Recreate, old pod is killed before the new one starts.
    # Wait for old pod to terminate first — otherwise kubectl wait matches the old
    # (still-Ready) pod and returns immediately before the new one exists.
    log "Waiting for old pod to terminate..."
    kubectl wait pod \
      --for=delete \
      --selector=app=formicary-ant \
      --namespace="${NAMESPACE}" \
      --timeout=20s 2>/dev/null || true

    # kubectl rollout status uses an http2 watch that drops on API server pod churn.
    # Poll kubectl wait pod instead — retries cleanly on disconnect.
    log "Waiting for ant pod to become ready (up to 120s)..."
    DEADLINE=$(( $(date +%s) + 120 ))
    READY=false
    while [[ $(date +%s) -lt $DEADLINE ]]; do
      if kubectl wait pod \
          --for=condition=Ready \
          --selector=app=formicary-ant \
          --namespace="${NAMESPACE}" \
          --timeout=10s \
          2>/dev/null; then
        READY=true
        break
      fi
      sleep 3
    done

    if ${READY}; then
      NEW_POD=$(kubectl get pod -l app=formicary-ant -n "${NAMESPACE}" \
                  --sort-by=.metadata.creationTimestamp \
                  -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null)
      NEW_IMAGE=$(kubectl get pod "${NEW_POD}" -n "${NAMESPACE}" \
                    -o jsonpath='{.status.containerStatuses[0].imageID}' 2>/dev/null)
      ok "formicary-ant is ready"
      ok "  pod:     ${NEW_POD}"
      ok "  imageID: ${NEW_IMAGE}"
    else
      warn "Pod not ready after 120s — current state:"
      kubectl get pods -l app=formicary-ant -n "${NAMESPACE}" 2>/dev/null || true
    fi
  fi
fi

printf "\n"
ok "Done. Images refreshed on node '${NODE}'."
