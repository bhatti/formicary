#!/usr/bin/env bash
# setup-ant-worker.sh — deploy a Formicary ant worker to a local Kubernetes cluster.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# ── Helpers ───────────────────────────────────────────────────────────────────
log()  { echo "  ▶ $*"; }
ok()   { echo "  ✓ $*"; }
warn() { echo "  ⚠ $*" >&2; }
fail() { echo "  ✗ ERROR: $*" >&2; exit 1; }
sep()  { echo "──────────────────────────────────────────────────────────"; }

usage() {
  cat <<EOF
USAGE
  $(basename "$0") [OPTIONS] [TRACKER]

DESCRIPTION
  Deploys a Formicary ant worker pod to a local Kubernetes cluster and
  optionally sets up issue-tracker / VCS credentials in the same run.

  The ant registers with the Formicary queen over WebSocket. Its org_id is
  derived automatically from the JWT token — no user tag is needed.

TRACKER (optional positional or --tracker flag)
  jira        Set up Jira credentials after deploying
  bitbucket   Set up Bitbucket credentials after deploying
  github      Set up GitHub credentials after deploying
  all         Set up all three

OPTIONS
  -s, --server  URL    Formicary queen URL  (env: FORMICARY_URL)
                       e.g. https://10.8.97.24.nip.io
  -t, --token   JWT    Formicary API token  (env: FORMICARY_TOKEN)
                       Get one at <server>/dashboard/users/tokens
      --port    N      Queen WebSocket port (default: 443 for https, 7777 for http)
      --s3-port N      Queen S3 port        (default: 4443 for https, 19000 for http)
      --namespace NS   Kubernetes namespace (default: default)
  -c, --context CTX    kubectl context      (default: current context)
      --buffer-db PATH SQLite buffer path   (default: :memory:)
      --image   IMG    Formicary image      (default: plexobject/formicary:<version>)
      --tracker T      Same as positional TRACKER argument
      --dry-run        Print what would happen without making changes
  -h, --help           Show this help

ENVIRONMENT VARIABLES
  FORMICARY_URL           Queen base URL (https://...)
  FORMICARY_TOKEN         API token (falls back to ~/.zshrc if not set)
  ANT_IMAGE               Override the formicary image
  AI_DEV_TOOLS_IMAGE      Override the ai-dev-tools image (default: plexobject/ai-dev-tools:latest)
  ANT_TLS_SKIP_VERIFY     Override TLS skip (true/false; derived from URL scheme by default)
  QUEEN_PORT / QUEEN_S3_PORT  Override ports

EXAMPLES
  # Deploy ant only
  export FORMICARY_URL=https://10.8.97.24.nip.io
  export FORMICARY_TOKEN=<token>
  $(basename "$0")

  # Deploy ant + set up Jira credentials in one step
  $(basename "$0") jira

  # Deploy ant + set up all credentials
  $(basename "$0") all

  # Target a specific cluster context
  $(basename "$0") --context kind-dev --server http://localhost:7777 --token <token>

  # Dry run to preview the rendered manifest
  $(basename "$0") --dry-run

  # Remove the ant from a cluster
  kubectl delete deployment formicary-ant

NOTES
  - Requires: kubectl, docker (for image pre-pull into containerd)
  - Images are pre-pulled via 'ctr --platform linux/amd64' to avoid OCI arm64/v8
    variant-mismatch errors on Apple Silicon Docker Desktop.
  - If the pre-pull fails the pod will attempt to pull at startup (imagePullPolicy=Always).
EOF
}

# ── Defaults ──────────────────────────────────────────────────────────────────
FORMICARY_URL="${FORMICARY_URL:-}"
FORMICARY_TOKEN="${FORMICARY_TOKEN:-}"
QUEEN_PORT="${QUEEN_PORT:-}"
QUEEN_S3_PORT="${QUEEN_S3_PORT:-}"
BUFFER_DB_PATH="${BUFFER_DB_PATH:-:memory:}"
_DEFAULT_VERSION="0.1.$(git -C "${REPO_ROOT}" rev-list --count HEAD 2>/dev/null || echo 0)"
ANT_IMAGE="${ANT_IMAGE:-plexobject/formicary:${_DEFAULT_VERSION}}"
AI_DEV_TOOLS_IMAGE="${AI_DEV_TOOLS_IMAGE:-plexobject/ai-dev-tools:latest}"
ANT_IMAGE_PULL_POLICY="Always"
NAMESPACE="default"
KUBE_CONTEXT="${KUBE_CONTEXT:-}"
TRACKER=""
DRY_RUN=false
ANT_TLS_SKIP_VERIFY="${ANT_TLS_SKIP_VERIFY:-}"

# Fall back to ~/.zshrc token to prevent stale shell-env tokens causing 401 errors
if [[ -z "${FORMICARY_TOKEN}" ]] && [[ -f "${HOME}/.zshrc" ]]; then
  FORMICARY_TOKEN="$(grep 'FORMICARY_TOKEN=' "${HOME}/.zshrc" \
    | sed "s/.*FORMICARY_TOKEN=['\"]\\?\\([^'\"]*\\)['\"]\\?.*/\\1/" | tail -1)"
fi

# ── Argument parsing ──────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)        usage; exit 0 ;;
    -s|--server)      FORMICARY_URL="$2";       shift 2 ;;
    -t|--token)       FORMICARY_TOKEN="$2";     shift 2 ;;
    --port)           QUEEN_PORT="$2";          shift 2 ;;
    --s3-port)        QUEEN_S3_PORT="$2";       shift 2 ;;
    --namespace)      NAMESPACE="$2";           shift 2 ;;
    --buffer-db)      BUFFER_DB_PATH="$2";      shift 2 ;;
    --image)          ANT_IMAGE="$2"; ANT_IMAGE_PULL_POLICY="Never"; shift 2 ;;
    --ai-dev-tools-image) AI_DEV_TOOLS_IMAGE="$2"; shift 2 ;;
    -c|--context)     KUBE_CONTEXT="$2";        shift 2 ;;
    --tracker|-T)     TRACKER="$2";             shift 2 ;;
    --dry-run)        DRY_RUN=true;             shift ;;
    --queen)          FORMICARY_URL="https://$2"; shift 2 ;;  # legacy alias
    --user)           shift 2 ;;                # legacy, ignored
    jira|bitbucket|github|all) TRACKER="$1";   shift ;;
    *) echo "Unknown option: $1  (try --help)" >&2; exit 1 ;;
  esac
done

# ── Validation ────────────────────────────────────────────────────────────────
_errors=0
check() {
  if [[ -z "${!1:-}" ]]; then
    echo "  ✗ $2" >&2
    (( _errors++ )) || true
  fi
}

[[ -z "${FORMICARY_URL}" ]]   && { echo "  ✗ --server / FORMICARY_URL is required" >&2;   (( _errors++ )) || true; }
[[ -z "${FORMICARY_TOKEN}" ]] && { echo "  ✗ --token / FORMICARY_TOKEN is required" >&2;   (( _errors++ )) || true; }

if [[ -n "${TRACKER}" ]]; then
  case "${TRACKER}" in
    jira|bitbucket|github|all) ;;
    *) echo "  ✗ --tracker must be one of: jira, bitbucket, github, all (got: ${TRACKER})" >&2
       (( _errors++ )) || true ;;
  esac
fi

if [[ "${_errors}" -gt 0 ]]; then
  echo "" >&2
  echo "Run '$(basename "$0") --help' for usage." >&2
  exit 1
fi

# Prerequisite tools
for _cmd in kubectl docker; do
  command -v "${_cmd}" &>/dev/null || fail "'${_cmd}' not found in PATH — install it before running this script"
done

# ── Derived settings ──────────────────────────────────────────────────────────
QUEEN_HOST="$(echo "${FORMICARY_URL}" | sed 's|^[^/]*//||' | sed 's|/.*||')"
_SCHEME="$(echo "${FORMICARY_URL}" | sed 's|://.*||')"

if [[ "${_SCHEME}" == "https" ]]; then
  ANT_WS_SCHEME="${ANT_WS_SCHEME:-wss}"
  [[ -z "${QUEEN_PORT}" ]]    && QUEEN_PORT="443"
  [[ -z "${QUEEN_S3_PORT}" ]] && QUEEN_S3_PORT="4443"
else
  ANT_WS_SCHEME="${ANT_WS_SCHEME:-ws}"
  [[ -z "${QUEEN_PORT}" ]]    && QUEEN_PORT="7777"
  [[ -z "${QUEEN_S3_PORT}" ]] && QUEEN_S3_PORT="19000"
fi

if [[ -z "${ANT_TLS_SKIP_VERIFY}" ]]; then
  [[ "${ANT_WS_SCHEME}" == "wss" ]] && ANT_TLS_SKIP_VERIFY="true" || ANT_TLS_SKIP_VERIFY="false"
fi

# Wrap kubectl to always target the right context
KUBECTL_CONTEXT_FLAG=""
[[ -n "${KUBE_CONTEXT}" ]] && KUBECTL_CONTEXT_FLAG="--context=${KUBE_CONTEXT}"
kubectl() { command kubectl ${KUBECTL_CONTEXT_FLAG} "$@"; }

# ── Summary ───────────────────────────────────────────────────────────────────
sep
echo "  Formicary Ant Worker Setup"
echo "  Queen:      ${QUEEN_HOST}:${QUEEN_PORT}  (S3: ${QUEEN_S3_PORT})"
echo "  WS:         ${ANT_WS_SCHEME}  TLS-skip: ${ANT_TLS_SKIP_VERIFY}"
echo "  Image:      ${ANT_IMAGE}  (pull: ${ANT_IMAGE_PULL_POLICY})"
echo "  AI tools:   ${AI_DEV_TOOLS_IMAGE}"
echo "  Namespace:  ${NAMESPACE}  Context: ${KUBE_CONTEXT:-$(command kubectl config current-context 2>/dev/null || echo default)}"
echo "  Buffer DB:  ${BUFFER_DB_PATH}"
[[ -n "${TRACKER}" ]] && echo "  Tracker:    ${TRACKER}"
[[ "${DRY_RUN}" == true ]] && echo "  Mode:       DRY RUN"
sep

# ── Step 1: Credentials secret ────────────────────────────────────────────────
echo ""
log "Step 1: Updating formicary-ant-credentials secret..."
SECRET_CMD=(
  kubectl create secret generic formicary-ant-credentials
  --namespace "${NAMESPACE}"
  "--from-literal=queen-host=${QUEEN_HOST}"
  "--from-literal=queue-token=${FORMICARY_TOKEN}"
  "--from-literal=buffer-db-path=${BUFFER_DB_PATH}"
  --save-config --dry-run=client -o yaml
)
if "${DRY_RUN}"; then
  echo "  [dry-run] would apply formicary-ant-credentials secret"
else
  "${SECRET_CMD[@]}" | kubectl apply -f - --validate=false
  ok "secret updated"
fi

# ── Step 2: Render manifest ───────────────────────────────────────────────────
echo ""
log "Step 2: Rendering k8s/formicary-ant.yaml..."
ANT_TEMPLATE="${REPO_ROOT}/k8s/formicary-ant.yaml"
[[ -f "${ANT_TEMPLATE}" ]] || fail "Template not found: ${ANT_TEMPLATE} — run from the formicary repo root"

RENDERED_YAML="$(mktemp /tmp/formicary-ant-XXXXXX.yaml)"
sed \
  -e "s|QUEEN_HOST|${QUEEN_HOST}|g" \
  -e "s|QUEEN_PORT|${QUEEN_PORT}|g" \
  -e "s|QUEEN_S3_PORT|${QUEEN_S3_PORT}|g" \
  -e "s|ANT_WS_SCHEME|${ANT_WS_SCHEME}|g" \
  -e "s|ANT_TLS_SKIP_VERIFY|${ANT_TLS_SKIP_VERIFY}|g" \
  -e "s|ANT_IMAGE_PULL_POLICY|${ANT_IMAGE_PULL_POLICY}|g" \
  -e "s|ANT_IMAGE|${ANT_IMAGE}|g" \
  "${ANT_TEMPLATE}" > "${RENDERED_YAML}"

if "${DRY_RUN}"; then
  echo "  [dry-run] rendered manifest:"
  cat "${RENDERED_YAML}"
  rm -f "${RENDERED_YAML}"
  exit 0
fi
ok "manifest rendered"

# ── Step 3: Apply + pre-pull images ──────────────────────────────────────────
echo ""
log "Step 3: Applying manifest..."
kubectl apply -f "${RENDERED_YAML}" --namespace "${NAMESPACE}" --validate=false
rm -f "${RENDERED_YAML}"
ok "manifest applied"

# Pre-pull an image into containerd k8s.io namespace on the cluster node.
# 'ctr images pull --platform linux/amd64' sidesteps the OCI arm64/v8 variant
# mismatch that causes 'crictl pull' to fail on Apple Silicon Docker Desktop.
pull_image() {
  local img="$1" label="$2" node="$3"
  local pull_img="${img/#docker.io\//registry-1.docker.io/}"
  log "${label}: ${img}"
  local attempt
  for attempt in 1 2 3; do
    if docker exec "${node}" ctr -n k8s.io images pull \
        --platform linux/amd64 "${pull_img}" 2>&1 | tail -1; then
      docker exec "${node}" ctr -n k8s.io images tag "${pull_img}" "${img}" 2>/dev/null || true
      ok "${label} pulled"
      return 0
    fi
    [[ ${attempt} -lt 3 ]] && warn "attempt ${attempt}/3 failed, retrying..." && sleep 2
  done
  warn "${label}: pre-pull failed — pod will retry at startup"
  return 0
}

echo ""
log "Step 3b: Pre-pulling images on cluster node..."
NODE_NAME="$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')"
pull_image "${ANT_IMAGE}"          "formicary" "${NODE_NAME}"
pull_image "${AI_DEV_TOOLS_IMAGE}" "ai-dev-tools" "${NODE_NAME}"

echo ""
log "Step 3c: Rolling restart..."
kubectl rollout restart deployment/formicary-ant --namespace "${NAMESPACE}"

# ── Step 4: Wait for rollout ──────────────────────────────────────────────────
echo ""
log "Step 4: Waiting for rollout (timeout: 120s)..."
kubectl rollout status deployment/formicary-ant --namespace "${NAMESPACE}" --timeout=120s
ok "ant worker is running"

# ── Step 5: Optional credential setup ────────────────────────────────────────
if [[ -n "${TRACKER}" ]]; then
  echo ""
  log "Step 5: Setting up ${TRACKER} credentials..."
  "${SCRIPT_DIR}/setup-user-creds.sh" --tracker "${TRACKER}" --server "${FORMICARY_URL}"
fi

# ── Done ──────────────────────────────────────────────────────────────────────
echo ""
sep
echo "  ✅  Ant worker running — ${FORMICARY_URL}/dashboard/ants"
if [[ -z "${TRACKER}" ]]; then
  echo ""
  echo "  Set up credentials with:"
  echo "    $(basename "$0") jira       # Jira"
  echo "    $(basename "$0") bitbucket  # Bitbucket"
  echo "    $(basename "$0") github     # GitHub"
  echo "    $(basename "$0") all        # all three"
fi
sep
