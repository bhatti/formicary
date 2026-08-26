#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Deploy all fan-out workflow examples.
#
# Fan-out has two modes:
#   Task fan-out  — dispatches one TaskRequest per array item directly to ant workers.
#                   Children share the parent JobExecutionID (no separate JobRequest records).
#   Job fan-out   — spawns one child JobRequest per item using the FORK_JOB machinery.
#                   Requires the child job type to be registered first.
#
# Usage:
#   ./deploy-fan-out.sh                               # no auth, localhost:7777
#   ./deploy-fan-out.sh --server http://host:7777     # remote server, no auth
#   ./deploy-fan-out.sh --token <TOKEN>               # auth enabled
#   ./deploy-fan-out.sh --server http://host:7777 --token <TOKEN>
#
# Environment variables (alternative to flags):
#   FORMICARY_URL    base URL  (default: http://localhost:7777)
#   FORMICARY_TOKEN  API token (default: empty — auth disabled)
#
# Examples deployed:
#   fan-out-task-regions.yaml  — task fan-out, SHELL, multi-region deploy
#   fan-out-deploy.yaml        — task fan-out, KUBERNETES, multi-region deploy
#   fan-out-job-etl.yaml       — job fan-out, spawns child ETL jobs per dataset
#                                (requires child job type io.formicary.test.child-etl-workflow)

set -euo pipefail

BASE_URL="${FORMICARY_URL:-http://localhost:7777}"
TOKEN="${FORMICARY_TOKEN:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ── Argument parsing ──────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --server|-s) BASE_URL="$2"; shift 2 ;;
    --token|-t)  TOKEN="$2";    shift 2 ;;
    *) echo "Unknown argument: $1"; exit 1 ;;
  esac
done

# ── Helpers ───────────────────────────────────────────────────────────────────
# Build curl args array with optional Authorization header
curl_args() {
  local args=()
  [[ -n "$TOKEN" ]] && args+=(-H "Authorization: Bearer ${TOKEN}")
  printf '%s\n' "${args[@]}"
}

deploy() {
    local file="$1"
    local label="$2"
    echo "Deploying $label ..."
    local args=(-s -o /tmp/deploy_resp.json -w "%{http_code}"
        -X POST "${BASE_URL}/api/jobs/definitions"
        -H "Content-Type: application/yaml"
        --data-binary "@${file}")
    [[ -n "$TOKEN" ]] && args+=(-H "Authorization: Bearer ${TOKEN}")
    HTTP_STATUS=$(curl "${args[@]}")
    if [ "${HTTP_STATUS}" -ge 200 ] && [ "${HTTP_STATUS}" -lt 300 ]; then
        echo "  OK (HTTP ${HTTP_STATUS})"
    else
        echo "  FAILED (HTTP ${HTTP_STATUS})"
        cat /tmp/deploy_resp.json
        echo ""
        exit 1
    fi
}

# ── Startup log ───────────────────────────────────────────────────────────────
if [[ -n "$TOKEN" ]]; then
  echo "Auth mode : ENABLED  (Bearer token provided)"
else
  echo "Auth mode : DISABLED (no token — requests sent without Authorization header)"
fi
echo "Server    : ${BASE_URL}"
echo ""

# ── Verify server ─────────────────────────────────────────────────────────────
echo "Checking Formicary at ${BASE_URL} ..."
_health_args=(-s -o /dev/null -w "%{http_code}" "${BASE_URL}/api/jobs/definitions")
[[ -n "$TOKEN" ]] && _health_args+=(-H "Authorization: Bearer ${TOKEN}")
_http_status=$(curl "${_health_args[@]}" 2>/dev/null || echo "000")
case "$_http_status" in
  2*) echo "  OK — server reachable (HTTP ${_http_status})" ;;
  000) echo "  ERROR: Cannot connect to ${BASE_URL} — is the server running?" >&2; exit 1 ;;
  401) echo "  ERROR: Server returned 401 — auth is enabled but FORMICARY_TOKEN is not set. Export your API token." >&2; exit 1 ;;
  403) echo "  ERROR: Server returned 403 — token is invalid or expired. Get a fresh token from the UI: ${BASE_URL}/dashboard/users and update FORMICARY_TOKEN in ~/.zshrc." >&2; exit 1 ;;
  *)   echo "  ERROR: Server returned HTTP ${_http_status} — unexpected response." >&2; exit 1 ;;
esac
echo ""

# ---------------------------------------------------------------------------
# 1. Task fan-out examples (no child job registration required)
# ---------------------------------------------------------------------------
echo "=== Task fan-out examples ==="
deploy "${SCRIPT_DIR}/fan-out-task-regions.yaml" \
    "task fan-out: multi-region SHELL deploy (fan-out-task-regions)"

deploy "${SCRIPT_DIR}/fan-out-deploy.yaml" \
    "task fan-out: multi-region Kubernetes deploy (fan-out-deploy)"

# ---------------------------------------------------------------------------
# 2. Job fan-out example (child job type must be registered first)
# ---------------------------------------------------------------------------
echo ""
echo "=== Job fan-out example ==="
echo "Note: fan-out-job-etl.yaml spawns child jobs of type"
echo "  io.formicary.test.child-etl-workflow"
echo "Registering child workflow first ..."
deploy "${SCRIPT_DIR}/sub-workflow-etl-child.yaml" \
    "child ETL workflow (io.formicary.examples.child-etl v1.0)"

deploy "${SCRIPT_DIR}/fan-out-job-etl.yaml" \
    "job fan-out: ETL pipeline per dataset (fan-out-job-etl)"

deploy "${SCRIPT_DIR}/sub-workflow-etl-child.yaml" "child ETL workflow (io.formicary.examples.child-etl v1.0)"
deploy "${SCRIPT_DIR}/sub-workflow-etl.yaml"       "parent ETL workflow (io.formicary.examples.parent-etl)"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
AUTH_HDR=""
[[ -n "$TOKEN" ]] && AUTH_HDR="       -H 'Authorization: Bearer \${FORMICARY_TOKEN}' \\"

echo ""
echo "All fan-out workflows deployed."
echo ""
echo "Submit test runs:"
echo ""
echo "  # Task fan-out — SHELL multi-region deploy"
echo "  curl -X POST '${BASE_URL}/api/jobs/requests' \\"
[[ -n "$TOKEN" ]] && echo "       -H 'Authorization: Bearer ${TOKEN}' \\"
echo "       -H 'Content-Type: application/json' \\"
echo "       -d '{\"job_type\":\"fan-out-task-regions\"}'"
echo ""
echo "  # Task fan-out — Kubernetes multi-region deploy"
echo "  curl -X POST '${BASE_URL}/api/jobs/requests' \\"
[[ -n "$TOKEN" ]] && echo "       -H 'Authorization: Bearer ${TOKEN}' \\"
echo "       -H 'Content-Type: application/json' \\"
echo "       -d '{\"job_type\":\"fan-out-deploy\",\"params\":{\"regions\":\"[\\\"us-east-1\\\",\\\"eu-west-1\\\"]\"}}'"
echo ""
echo "  # Job fan-out — ETL pipeline spawning child jobs"
echo "  curl -X POST '${BASE_URL}/api/jobs/requests' \\"
[[ -n "$TOKEN" ]] && echo "       -H 'Authorization: Bearer ${TOKEN}' \\"
echo "       -H 'Content-Type: application/json' \\"
echo "       -d '{\"job_type\":\"fan-out-job-etl\"}'"
echo ""
echo "Result context keys produced by fan-out:"
echo "  {item_var}_{index}_status        — COMPLETED or FAILED"
echo "  {item_var}_{index}_exit_code     — process exit code"
echo "  {item_var}_{index}_error_message — error detail if failed"
echo "  {item_var}_{index}_{custom_key}  — any variable set by the child"
echo "  FanOutItemCount                  — total items dispatched"
echo "  FanOutSource                     — name of the source array variable"
echo "  FanOutMode                       — \"task\" or \"job\""
