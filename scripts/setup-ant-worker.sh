#!/usr/bin/env bash
# setup-ant-worker.sh — deploy a Formicary ant worker and optionally set up
# issue-tracker / VCS credentials and AI workflows.
#
# Can be run standalone (outside the repo): the script sparse-clones the
# required files from GitHub into /tmp/formicary-setup if it cannot find a
# local formicary checkout.
#
# USAGE
#   setup-ant-worker.sh [TRACKER...] [OPTIONS]
#
# TRACKERS (positional, pick one or more)
#   jira         Jira credentials + deploy Jira/BB AI workflows
#   bb           Bitbucket credentials + deploy Jira/BB AI workflows
#   bitbucket    Same as bb
#   github       GitHub credentials + deploy GitHub AI workflows
#   gh           Same as github
#
# OPTIONS
#   -s, --server  URL    Queen URL          (env: FORMICARY_URL)
#   -t, --token   JWT    API token          (env: FORMICARY_TOKEN)
#       --namespace NS   k8s namespace      (default: default)
#   -c, --context CTX    kubectl context    (default: current-context)
#       --image   IMG    Ant image          (env: ANT_IMAGE)
#       --buffer-db P    SQLite path        (default: :memory:)
#       --repo-url URL   Formicary git URL  (env: FORMICARY_REPO_URL)
#       --skip-worker    Skip ant deploy, run creds/workflows only
#       --dry-run        Print what would happen without making changes
#   -h, --help
#
# EXAMPLES
#   # Deploy ant only
#   FORMICARY_URL=https://queen.example.com FORMICARY_TOKEN=<tok> setup-ant-worker.sh
#
#   # Deploy ant + set up Jira + BB workflows
#   setup-ant-worker.sh jira bb
#
#   # Deploy ant + all trackers
#   setup-ant-worker.sh jira bb github
#
#   # Credentials + workflows only (ant already running)
#   setup-ant-worker.sh --skip-worker jira github

set -euo pipefail

# ── Helpers ───────────────────────────────────────────────────────────────────
log()   { printf "  ▶ %s\n" "$*"; }
ok()    { printf "  ✓ %s\n" "$*"; }
warn()  { printf "  ⚠ %s\n" "$*" >&2; }
fail()  { printf "  ✗ ERROR: %s\n" "$*" >&2; exit 1; }
sep()   { printf -- "──────────────────────────────────────────────────────────\n"; }
hdr()   { printf "\n"; sep; printf "  %s\n" "$*"; sep; }

# Clean up temp files on any exit
_TMPFILES=()
_cleanup() { for f in "${_TMPFILES[@]+"${_TMPFILES[@]}"}"; do rm -f "$f"; done; }
trap _cleanup EXIT

# ── Defaults ──────────────────────────────────────────────────────────────────
FORMICARY_URL="${FORMICARY_URL:-}"
FORMICARY_TOKEN="${FORMICARY_TOKEN:-}"
FORMICARY_REPO_URL="${FORMICARY_REPO_URL:-https://github.com/bhatti/formicary}"
QUEEN_PORT="${QUEEN_PORT:-}"
QUEEN_S3_PORT="${QUEEN_S3_PORT:-}"
BUFFER_DB_PATH="${BUFFER_DB_PATH:-:memory:}"
ANT_IMAGE="${ANT_IMAGE:-}"
AI_DEV_TOOLS_IMAGE="${AI_DEV_TOOLS_IMAGE:-plexobject/ai-dev-tools:latest}"
ANT_IMAGE_PULL_POLICY="Always"
NAMESPACE="default"
KUBE_CONTEXT="${KUBE_CONTEXT:-}"
ANT_TLS_SKIP_VERIFY="${ANT_TLS_SKIP_VERIFY:-}"
SKIP_WORKER=false
DRY_RUN=false

# Trackers requested — will accumulate from positional args
_DO_JIRA=false
_DO_BB=false
_DO_GH=false

# ── Argument parsing ──────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)       sed -n '/^# USAGE/,/^set -/p' "$0" | grep '^#' | sed 's/^# \?//'; exit 0 ;;
    -s|--server)     FORMICARY_URL="$2";      shift 2 ;;
    -t|--token)      FORMICARY_TOKEN="$2";    shift 2 ;;
    --port)          QUEEN_PORT="$2";         shift 2 ;;
    --s3-port)       QUEEN_S3_PORT="$2";      shift 2 ;;
    --namespace)     NAMESPACE="$2";          shift 2 ;;
    -c|--context)    KUBE_CONTEXT="$2";       shift 2 ;;
    --image)         ANT_IMAGE="$2"; ANT_IMAGE_PULL_POLICY="Never"; shift 2 ;;
    --buffer-db)     BUFFER_DB_PATH="$2";     shift 2 ;;
    --repo-url)      FORMICARY_REPO_URL="$2"; shift 2 ;;
    --skip-worker)   SKIP_WORKER=true;        shift ;;
    --dry-run)       DRY_RUN=true;            shift ;;
    --queen)         FORMICARY_URL="https://$2"; shift 2 ;;  # legacy alias
    --user)          shift 2 ;;                              # legacy, ignored
    --tracker|-T)    # legacy: single tracker flag
      case "$2" in
        jira)              _DO_JIRA=true ;;
        bb|bitbucket)      _DO_BB=true ;;
        github|gh)         _DO_GH=true ;;
        all)               _DO_JIRA=true; _DO_BB=true; _DO_GH=true ;;
        *) fail "Unknown tracker '$2'. Valid: jira bb bitbucket github gh all" ;;
      esac
      shift 2 ;;
    jira)        _DO_JIRA=true; shift ;;
    bb|bitbucket) _DO_BB=true; shift ;;
    github|gh)   _DO_GH=true;  shift ;;
    all)         _DO_JIRA=true; _DO_BB=true; _DO_GH=true; shift ;;
    *) fail "Unknown option: $1  (try --help)" ;;
  esac
done

# ── Token fallback ────────────────────────────────────────────────────────────
if [[ -z "${FORMICARY_TOKEN}" && -f "${HOME}/.zshrc" ]]; then
  FORMICARY_TOKEN="$(grep 'FORMICARY_TOKEN=' "${HOME}/.zshrc" \
    | sed "s/.*FORMICARY_TOKEN=['\"]\\?\\([^'\"]*\\)['\"]\\?.*/\\1/" | tail -1)"
fi
if [[ -z "${FORMICARY_TOKEN}" && -f "${HOME}/.bashrc" ]]; then
  FORMICARY_TOKEN="$(grep 'FORMICARY_TOKEN=' "${HOME}/.bashrc" \
    | sed "s/.*FORMICARY_TOKEN=['\"]\\?\\([^'\"]*\\)['\"]\\?.*/\\1/" | tail -1)"
fi

# ── Validation ────────────────────────────────────────────────────────────────
_errors=0
[[ -z "${FORMICARY_URL}"   ]] && { warn "--server / FORMICARY_URL is required";   (( _errors++ )) || true; }
[[ -z "${FORMICARY_TOKEN}" ]] && { warn "--token / FORMICARY_TOKEN is required";   (( _errors++ )) || true; }
[[ "${_errors}" -gt 0 ]] && { echo ""; echo "Run '$(basename "$0") --help' for usage." >&2; exit 1; }

command -v kubectl &>/dev/null || fail "'kubectl' not found in PATH"

# ── Derived settings ──────────────────────────────────────────────────────────
QUEEN_HOST="$(printf '%s' "${FORMICARY_URL}" | sed 's|^[^/]*//||' | sed 's|/.*||')"
_SCHEME="$(printf '%s' "${FORMICARY_URL}" | sed 's|://.*||')"

if [[ "${_SCHEME}" == "https" ]]; then
  ANT_WS_SCHEME="${ANT_WS_SCHEME:-wss}"
  [[ -z "${QUEEN_PORT}"    ]] && QUEEN_PORT="443"
  [[ -z "${QUEEN_S3_PORT}" ]] && QUEEN_S3_PORT="4443"
else
  ANT_WS_SCHEME="${ANT_WS_SCHEME:-ws}"
  [[ -z "${QUEEN_PORT}"    ]] && QUEEN_PORT="7777"
  [[ -z "${QUEEN_S3_PORT}" ]] && QUEEN_S3_PORT="19000"
fi

[[ -z "${ANT_TLS_SKIP_VERIFY}" ]] && {
  [[ "${ANT_WS_SCHEME}" == "wss" ]] && ANT_TLS_SKIP_VERIFY="true" || ANT_TLS_SKIP_VERIFY="false"
}

# kubectl wrapper for consistent context targeting
KUBECTL_CONTEXT_FLAG=""
[[ -n "${KUBE_CONTEXT}" ]] && KUBECTL_CONTEXT_FLAG="--context=${KUBE_CONTEXT}"
kubectl() { command kubectl ${KUBECTL_CONTEXT_FLAG} "$@"; }

# ── Resolve WORK_DIR (local checkout or sparse clone) ─────────────────────────
_SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
_CANDIDATE_ROOT="$(cd "${_SCRIPT_DIR}/.." && pwd)"

_CLONED=false
if [[ -f "${_CANDIDATE_ROOT}/k8s/formicary-ant.yaml" \
   && -f "${_CANDIDATE_ROOT}/scripts/setup-user-creds.sh" ]]; then
  WORK_DIR="${_CANDIDATE_ROOT}"
else
  # Sparse-clone just the files we need into a fixed temp location.
  # Reuse the existing clone if it looks valid to avoid redundant network calls.
  WORK_DIR="/tmp/formicary-setup"
  if [[ ! -f "${WORK_DIR}/k8s/formicary-ant.yaml" ]]; then
    log "Cloning formicary scripts from ${FORMICARY_REPO_URL}..."
    command -v git &>/dev/null || fail "'git' not found — required to clone formicary scripts"
    rm -rf "${WORK_DIR}"
    git clone \
      --filter=blob:none \
      --no-checkout \
      --depth=1 \
      --quiet \
      "${FORMICARY_REPO_URL}" "${WORK_DIR}" 2>&1 || fail "git clone failed"
    (
      cd "${WORK_DIR}"
      git sparse-checkout init --cone --quiet
      git sparse-checkout set scripts docs/examples k8s
      git checkout --quiet
    ) || fail "sparse-checkout failed"
    _CLONED=true
    ok "scripts cloned to ${WORK_DIR}"
  else
    ok "using cached clone at ${WORK_DIR}"
  fi
fi

SCRIPTS_DIR="${WORK_DIR}/scripts"
EXAMPLES_DIR="${WORK_DIR}/docs/examples"
ANT_TEMPLATE="${WORK_DIR}/k8s/formicary-ant.yaml"

[[ -f "${ANT_TEMPLATE}" ]]                   || fail "k8s/formicary-ant.yaml not found in ${WORK_DIR}"
[[ -f "${SCRIPTS_DIR}/setup-user-creds.sh" ]] || fail "scripts/setup-user-creds.sh not found"

# ── Image detection ───────────────────────────────────────────────────────────
# Detect the current k8s context to decide on pull policy.
_CUR_CTX="${KUBE_CONTEXT:-$(command kubectl config current-context 2>/dev/null || echo "")}"

if [[ -z "${ANT_IMAGE}" ]]; then
  ANT_IMAGE="plexobject/formicary:latest"
fi

# Pull policy: Docker Desktop shares the daemon with k8s containerd — IfNotPresent
# avoids remote pull attempts for locally-built images.
# For all other clusters default to Always so images stay fresh.
if [[ "${ANT_IMAGE_PULL_POLICY}" == "Always" ]]; then
  if [[ "${_CUR_CTX}" == "docker-desktop" ]]; then
    ANT_IMAGE_PULL_POLICY="IfNotPresent"
  fi
fi

# ── Summary ───────────────────────────────────────────────────────────────────
_trackers=""
${_DO_JIRA} && _trackers="${_trackers} jira"
${_DO_BB}   && _trackers="${_trackers} bitbucket"
${_DO_GH}   && _trackers="${_trackers} github"
_trackers="${_trackers# }"

hdr "Formicary Ant Worker Setup"
printf "  Queen:      %s:%s  (S3: %s)\n"  "${QUEEN_HOST}" "${QUEEN_PORT}" "${QUEEN_S3_PORT}"
printf "  WS:         %s  TLS-skip: %s\n" "${ANT_WS_SCHEME}" "${ANT_TLS_SKIP_VERIFY}"
printf "  Image:      %s  (pull: %s)\n"   "${ANT_IMAGE}" "${ANT_IMAGE_PULL_POLICY}"
printf "  Namespace:  %s  Context: %s\n"  "${NAMESPACE}" \
  "${KUBE_CONTEXT:-$(command kubectl config current-context 2>/dev/null || echo default)}"
printf "  Trackers:   %s\n"               "${_trackers:-none}"
${SKIP_WORKER} && printf "  Worker:     skip (--skip-worker)\n"
${DRY_RUN}     && printf "  Mode:       DRY RUN\n"
sep

# ── Worker deploy ─────────────────────────────────────────────────────────────
if ! ${SKIP_WORKER}; then

  printf "\n"
  log "Step 1: Updating formicary-ant-credentials secret..."
  _secret_cmd=(
    kubectl create secret generic formicary-ant-credentials
    --namespace "${NAMESPACE}"
    "--from-literal=queen-host=${QUEEN_HOST}"
    "--from-literal=queue-token=${FORMICARY_TOKEN}"
    "--from-literal=buffer-db-path=${BUFFER_DB_PATH}"
    --save-config --dry-run=client -o yaml
  )
  if ${DRY_RUN}; then
    log "[dry-run] would apply formicary-ant-credentials secret"
  else
    "${_secret_cmd[@]}" | kubectl apply -f - --validate=false
    ok "secret updated"
  fi

  printf "\n"
  log "Step 2: Rendering k8s/formicary-ant.yaml..."
  _rendered="$(mktemp /tmp/formicary-ant-XXXXXX)" || fail "mktemp failed"
  _TMPFILES+=("${_rendered}")
  sed \
    -e "s|QUEEN_HOST|${QUEEN_HOST}|g" \
    -e "s|QUEEN_PORT|${QUEEN_PORT}|g" \
    -e "s|QUEEN_S3_PORT|${QUEEN_S3_PORT}|g" \
    -e "s|ANT_WS_SCHEME|${ANT_WS_SCHEME}|g" \
    -e "s|ANT_TLS_SKIP_VERIFY|${ANT_TLS_SKIP_VERIFY}|g" \
    -e "s|ANT_IMAGE_PULL_POLICY|${ANT_IMAGE_PULL_POLICY}|g" \
    -e "s|ANT_IMAGE|${ANT_IMAGE}|g" \
    "${ANT_TEMPLATE}" > "${_rendered}"

  if ${DRY_RUN}; then
    log "[dry-run] rendered manifest:"
    cat "${_rendered}"
    exit 0
  fi
  ok "manifest rendered"

  printf "\n"
  log "Step 3: Applying manifest..."
  kubectl apply -f "${_rendered}" --namespace "${NAMESPACE}" --validate=false
  ok "manifest applied"

  printf "\n"
  log "Step 4: Rolling restart..."
  kubectl rollout restart deployment/formicary-ant --namespace "${NAMESPACE}"

  log "Waiting for rollout (timeout: 90s)..."
  kubectl rollout status deployment/formicary-ant \
    --namespace "${NAMESPACE}" --timeout=90s \
    && ok "ant worker is running" \
    || warn "rollout timed out — pod may still be starting; run: kubectl get pods -n ${NAMESPACE}"

fi  # SKIP_WORKER

# ── Credentials setup ─────────────────────────────────────────────────────────
_any_tracker=false
${_DO_JIRA} && _any_tracker=true
${_DO_BB}   && _any_tracker=true
${_DO_GH}   && _any_tracker=true

if ${_any_tracker}; then
  printf "\n"
  log "Setting up credentials..."
  export FORMICARY_URL FORMICARY_TOKEN

  if ${_DO_JIRA}; then
    log "  jira credentials..."
    bash "${SCRIPTS_DIR}/setup-user-creds.sh" jira \
      --server "${FORMICARY_URL}" || warn "jira credential setup failed"
  fi

  if ${_DO_BB}; then
    log "  bitbucket credentials..."
    bash "${SCRIPTS_DIR}/setup-user-creds.sh" bitbucket \
      --server "${FORMICARY_URL}" || warn "bitbucket credential setup failed"
  fi

  if ${_DO_GH}; then
    log "  github credentials..."
    bash "${SCRIPTS_DIR}/setup-user-creds.sh" github \
      --server "${FORMICARY_URL}" || warn "github credential setup failed"
  fi
fi

# ── Workflow deploy ───────────────────────────────────────────────────────────
_deploy_jira_workflows=false
_deploy_gh_workflows=false
${_DO_JIRA} && _deploy_jira_workflows=true
${_DO_BB}   && _deploy_jira_workflows=true
${_DO_GH}   && _deploy_gh_workflows=true

if ${_deploy_jira_workflows}; then
  printf "\n"
  log "Deploying Jira/BB AI workflows..."
  bash "${EXAMPLES_DIR}/deploy-ai-jira-workflows.sh" \
    --server "${FORMICARY_URL}" \
    --set-configs \
    --create-k8s-secret \
    || warn "Jira workflow deploy failed — check credentials and re-run"
  ok "Jira/BB workflows deployed"
fi

if ${_deploy_gh_workflows}; then
  printf "\n"
  log "Deploying GitHub AI workflows..."
  bash "${EXAMPLES_DIR}/deploy-ai-workflows.sh" \
    --server "${FORMICARY_URL}" \
    --set-configs \
    --create-k8s-secret \
    || warn "GitHub workflow deploy failed — check credentials and re-run"
  ok "GitHub workflows deployed"
fi

# ── Doctor ────────────────────────────────────────────────────────────────────
doctor() {
  printf "\n"
  hdr "Doctor"

  # API reachability
  local _code
  _code="$(curl -sk -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer ${FORMICARY_TOKEN}" \
    "${FORMICARY_URL}/api/v1/ants" 2>/dev/null)" || _code="000"

  if [[ "${_code}" != "200" ]]; then
    warn "Cannot reach ${FORMICARY_URL}/api/v1/ants (HTTP ${_code})"
    warn "Check FORMICARY_URL and FORMICARY_TOKEN"
    return
  fi
  ok "Queen API reachable (${FORMICARY_URL})"

  # Ant workers
  local _ants _total
  _ants="$(curl -sk -H "Authorization: Bearer ${FORMICARY_TOKEN}" \
    "${FORMICARY_URL}/api/v1/ants" 2>/dev/null)" || _ants="{}"
  _total="$(printf '%s' "${_ants}" | grep -o '"total_records":[0-9]*' | cut -d: -f2)"
  if [[ "${_total:-0}" -gt 0 ]]; then
    ok "Ant workers: ${_total} registered"
    printf '%s' "${_ants}" | grep -o '"allocation_id":"[^"]*"' \
      | sed 's/"allocation_id":"//;s/"//' \
      | while IFS= read -r _id; do printf "    • %s\n" "${_id}"; done || true
  else
    warn "No ant workers registered yet (pod may still be starting)"
    warn "  Check: kubectl get pods -n ${NAMESPACE}"
    warn "  Logs:  kubectl logs -n ${NAMESPACE} deployment/formicary-ant --tail=30"
  fi

  # Job definitions
  local _jobs _jtotal
  _jobs="$(curl -sk -H "Authorization: Bearer ${FORMICARY_TOKEN}" \
    "${FORMICARY_URL}/api/v1/jobs/definitions?page_size=5" 2>/dev/null)" || _jobs="{}"
  _jtotal="$(printf '%s' "${_jobs}" | grep -o '"total_records":[0-9]*' | cut -d: -f2)"
  if [[ "${_jtotal:-0}" -gt 0 ]]; then
    ok "Job definitions: ${_jtotal} found (${FORMICARY_URL}/dashboard/jobs/definitions)"
  else
    warn "No job definitions found"
    warn "  Deploy workflows: $(basename "$0") jira  OR  $(basename "$0") github"
  fi
}

doctor

# ── Done ──────────────────────────────────────────────────────────────────────
printf "\n"
sep
printf "  Dashboard:  %s/dashboard/ants\n" "${FORMICARY_URL}"
if ! ${_any_tracker}; then
  printf "\n  Set up credentials and deploy workflows:\n"
  printf "    $(basename "$0") jira        # Jira + Bitbucket workflows\n"
  printf "    $(basename "$0") gh          # GitHub workflows\n"
  printf "    $(basename "$0") jira bb gh  # All three\n"
fi
sep
printf "\n"
