#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# deploy-approval-workflows.sh
#
# Deploys all multi-party approval workflow examples to a running Formicary queen
# and provides ready-to-run curl commands for end-to-end testing.
#
# Works with auth ENABLED (--token) or DISABLED (default, no token needed).
#
# Usage:
#   ./deploy-approval-workflows.sh                               # no auth, localhost:7777
#   ./deploy-approval-workflows.sh --server http://host:7777     # remote server, no auth
#   ./deploy-approval-workflows.sh --token <TOKEN>               # auth enabled
#   ./deploy-approval-workflows.sh --server http://host:7777 \
#                                  --token <TOKEN>               # remote + auth
#   ./deploy-approval-workflows.sh --test                        # deploy + submit + vote
#   ./deploy-approval-workflows.sh --test --voter alice          # override voter name
#
# Environment variables (alternative to flags):
#   FORMICARY_URL    base URL  (default: http://localhost:7777)
#   FORMICARY_TOKEN  API token (default: empty — auth disabled)
#   VOTER_ID         voter identity for --test mode (default: alice)
#
# Workflows deployed:
#   manual.yaml                  — semi-automated workflow (2 MANUAL tasks)
#   multi-party-approval.yaml    — 2-of-3 quorum, no SLA
#   approval-with-sla.yaml       — 4h SLA, ESCALATE on breach
#   approval-auto-reject.yaml    — 1h SLA, AUTO_REJECT on breach
#   approval-unanimous.yaml      — unanimous consent required
#   cicd-approval-gate.yaml      — full CI/CD pipeline with approval gate
#   secure-go-cicd.yaml          — Go CI/CD with production deploy approval
#   dr-playbook.yaml             — disaster-recovery failover with ops approval

set -euo pipefail

# ── Defaults ──────────────────────────────────────────────────────────────────
FORMICARY_URL="${FORMICARY_URL:-http://localhost:7777}"
TOKEN="${FORMICARY_TOKEN:-}"
VOTER_ID="${VOTER_ID:-alice}"
RUN_TEST=false
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ── Argument parsing ───────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --server)  FORMICARY_URL="$2"; shift 2 ;;
    --token)   TOKEN="$2";         shift 2 ;;
    --voter)   VOTER_ID="$2";      shift 2 ;;
    --test)    RUN_TEST=true;      shift   ;;
    --help|-h)
      sed -n '2,/^[^#]/p' "$0" | grep '^#' | sed 's/^# \{0,1\}//'
      exit 0 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# ── Helpers ────────────────────────────────────────────────────────────────────
log()  { echo "▶  $*"; }
ok()   { echo "   ✓ $*"; }
warn() { echo "   ⚠ $*"; }
fail() { echo "   ✗ $*" >&2; exit 1; }
sep()  { echo; echo "──────────────────────────────────────────────────────"; echo; }

# Build the auth header value (empty string when auth is disabled).
auth_header() {
  if [[ -n "$TOKEN" ]]; then
    echo "Authorization: Bearer ${TOKEN}"
  else
    echo ""
  fi
}

# curl wrapper: appends Authorization header only when a token is set.
# Usage: api_curl <curl-args...>
api_curl() {
  local auth
  auth="$(auth_header)"
  if [[ -n "$auth" ]]; then
    curl -s -f "$@" -H "$auth"
  else
    curl -s -f "$@"
  fi
}

# Deploy a YAML workflow definition.
# Usage: deploy <file> <label>
deploy() {
  local file="$1"
  local label="$2"
  log "Deploying ${label} ..."
  local http_status
  http_status=$(
    if [[ -n "$TOKEN" ]]; then
      curl -s -o /tmp/_deploy_resp.json -w "%{http_code}" \
        -X POST "${FORMICARY_URL}/api/jobs/definitions" \
        -H "Content-Type: application/yaml" \
        -H "Authorization: Bearer ${TOKEN}" \
        --data-binary "@${file}"
    else
      curl -s -o /tmp/_deploy_resp.json -w "%{http_code}" \
        -X POST "${FORMICARY_URL}/api/jobs/definitions" \
        -H "Content-Type: application/yaml" \
        --data-binary "@${file}"
    fi
  )
  if [[ "${http_status}" -ge 200 && "${http_status}" -lt 300 ]]; then
    ok "HTTP ${http_status}"
  else
    echo "   FAILED (HTTP ${http_status})"
    cat /tmp/_deploy_resp.json 2>/dev/null || true
    echo
    exit 1
  fi
}

# Submit a job and echo the new request ID.
# Usage: submit_job <job_type> [params_json]
submit_job() {
  local job_type="$1"
  local params="${2:-{}}"
  local body="{\"job_type\":\"${job_type}\",\"params\":${params}}"
  local resp
  if [[ -n "$TOKEN" ]]; then
    resp=$(curl -s -f -X POST "${FORMICARY_URL}/api/jobs/requests" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer ${TOKEN}" \
      -d "$body")
  else
    resp=$(curl -s -f -X POST "${FORMICARY_URL}/api/jobs/requests" \
      -H "Content-Type: application/json" \
      -d "$body")
  fi
  echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || \
    echo "$resp" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4
}

# Poll job state until it leaves PENDING/READY/EXECUTING or timeout.
# Usage: wait_for_approval <request_id> [timeout_secs]
wait_for_approval() {
  local req_id="$1"
  local timeout="${2:-30}"
  local elapsed=0
  log "Waiting for job ${req_id} to reach MANUAL_APPROVAL_REQUIRED ..."
  while [[ $elapsed -lt $timeout ]]; do
    local state
    if [[ -n "$TOKEN" ]]; then
      state=$(curl -s -f "${FORMICARY_URL}/api/jobs/requests/${req_id}" \
        -H "Authorization: Bearer ${TOKEN}" | \
        python3 -c "import sys,json; print(json.load(sys.stdin).get('job_state',''))" 2>/dev/null || echo "")
    else
      state=$(curl -s -f "${FORMICARY_URL}/api/jobs/requests/${req_id}" | \
        python3 -c "import sys,json; print(json.load(sys.stdin).get('job_state',''))" 2>/dev/null || echo "")
    fi
    if [[ "$state" == "MANUAL_APPROVAL_REQUIRED" ]]; then
      ok "Job is waiting for approval (state: ${state})"
      return 0
    elif [[ "$state" == "COMPLETED" || "$state" == "FAILED" || "$state" == "FATAL" ]]; then
      warn "Job reached terminal state: ${state} (skipping vote)"
      return 1
    fi
    sleep 2
    elapsed=$((elapsed + 2))
  done
  warn "Timed out waiting for MANUAL_APPROVAL_REQUIRED (current state may still be transitioning)"
  return 1
}

# ── Auth status ────────────────────────────────────────────────────────────────
sep
if [[ -n "$TOKEN" ]]; then
  log "Auth mode : ENABLED  (Bearer token provided)"
else
  log "Auth mode : DISABLED (no token — all requests sent without Authorization header)"
fi
log "Server    : ${FORMICARY_URL}"
log "Voter ID  : ${VOTER_ID} (used in --test mode)"

# ── Verify server ──────────────────────────────────────────────────────────────
log "Checking Formicary at ${FORMICARY_URL} ..."
_health_args=(-s -o /dev/null -w "%{http_code}" "${FORMICARY_URL}/api/jobs/definitions")
[[ -n "$TOKEN" ]] && _health_args+=(-H "Authorization: Bearer ${TOKEN}")
_http_status=$(curl "${_health_args[@]}" 2>/dev/null || echo "000")
case "$_http_status" in
  2*) ok "Server reachable (HTTP ${_http_status})" ;;
  000) fail "Cannot connect to ${FORMICARY_URL} — is the server running?" ;;
  401) fail "Server returned 401 — auth is enabled but FORMICARY_TOKEN is not set. Export your API token." ;;
  403) fail "Server returned 403 — token is invalid or expired. Get a fresh token from the UI: ${FORMICARY_URL}/dashboard/users and update FORMICARY_TOKEN in ~/.zshrc." ;;
  *)   fail "Server returned HTTP ${_http_status} — unexpected response from ${FORMICARY_URL}" ;;
esac

# ── Deploy workflows ───────────────────────────────────────────────────────────
sep
echo "=== Deploying approval workflow examples ==="
echo

deploy "${SCRIPT_DIR}/manual.yaml"               "semi-automated (manual.yaml — 2 MANUAL tasks)"
deploy "${SCRIPT_DIR}/multi-party-approval.yaml"  "multi-party quorum 2-of-3 (multi-party-approval.yaml)"
deploy "${SCRIPT_DIR}/approval-with-sla.yaml"     "approval with 4h SLA escalation (approval-with-sla.yaml)"
deploy "${SCRIPT_DIR}/approval-auto-reject.yaml"  "approval with 1h auto-reject (approval-auto-reject.yaml)"
deploy "${SCRIPT_DIR}/approval-unanimous.yaml"    "unanimous consent required (approval-unanimous.yaml)"
deploy "${SCRIPT_DIR}/cicd-approval-gate.yaml"    "CI/CD pipeline with approval gate (cicd-approval-gate.yaml)"
deploy "${SCRIPT_DIR}/secure-go-cicd.yaml"        "secure Go CI/CD with production approval (secure-go-cicd.yaml)"
deploy "${SCRIPT_DIR}/dr-playbook.yaml"           "disaster-recovery failover approval (dr-playbook.yaml)"

sep
echo "All workflows deployed successfully."

# ── Optional end-to-end test ───────────────────────────────────────────────────
if [[ "$RUN_TEST" == "true" ]]; then
  sep
  echo "=== End-to-end test: multi-party-approval ==="
  echo

  log "Submitting multi-party-approval job ..."
  REQ_ID=$(submit_job "multi-party-approval-demo")
  if [[ -z "$REQ_ID" ]]; then
    fail "Failed to submit job — is the server running at ${FORMICARY_URL}?"
  fi
  ok "Job submitted: ${REQ_ID}"

  # Wait for the job to reach MANUAL_APPROVAL_REQUIRED
  if wait_for_approval "$REQ_ID" 30; then
    # Find the task type that is waiting
    TASK_TYPE="team-approval"

    log "Checking approval status ..."
    if [[ -n "$TOKEN" ]]; then
      curl -s "${FORMICARY_URL}/api/jobs/requests/${REQ_ID}/tasks/${TASK_TYPE}/approval" \
        -H "Authorization: Bearer ${TOKEN}" | python3 -m json.tool 2>/dev/null || true
    else
      curl -s "${FORMICARY_URL}/api/jobs/requests/${REQ_ID}/tasks/${TASK_TYPE}/approval" | \
        python3 -m json.tool 2>/dev/null || true
    fi

    log "Casting vote 1 (APPROVED) as ${VOTER_ID} ..."
    if [[ -n "$TOKEN" ]]; then
      curl -s -X POST \
        "${FORMICARY_URL}/api/jobs/requests/${REQ_ID}/tasks/${TASK_TYPE}/vote" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${TOKEN}" \
        -d "{\"decision\":\"APPROVED\",\"comments\":\"Looks good to me — ${VOTER_ID}\"}" | \
        python3 -m json.tool 2>/dev/null || true
    else
      curl -s -X POST \
        "${FORMICARY_URL}/api/jobs/requests/${REQ_ID}/tasks/${TASK_TYPE}/vote" \
        -H "Content-Type: application/json" \
        -d "{\"decision\":\"APPROVED\",\"comments\":\"Looks good to me — ${VOTER_ID}\"}" | \
        python3 -m json.tool 2>/dev/null || true
    fi
    ok "Vote 1 cast. (Min approvals: 2; need one more to reach quorum.)"

    log "Checking pending approvals queue ..."
    if [[ -n "$TOKEN" ]]; then
      curl -s "${FORMICARY_URL}/api/approvals/pending" \
        -H "Authorization: Bearer ${TOKEN}" | python3 -m json.tool 2>/dev/null || true
    else
      curl -s "${FORMICARY_URL}/api/approvals/pending" | python3 -m json.tool 2>/dev/null || true
    fi
  fi

  sep
  echo "Test complete."
fi

# ── Usage reference ────────────────────────────────────────────────────────────
sep
cat <<'USAGE'
═══════════════════════════════════════════════════════════════════════════════
  APPROVAL API QUICK REFERENCE
═══════════════════════════════════════════════════════════════════════════════

  Replace:
    BASE_URL  → http://localhost:7777  (or your server)
    TOKEN     → your API token         (omit -H Authorization when auth disabled)
    REQ_ID    → job request ID returned by submit
    TASK_TYPE → task_type value of the MANUAL task (e.g. team-approval)

── Submit a job ────────────────────────────────────────────────────────────────

  # Auth disabled
  curl -X POST "${BASE_URL}/api/jobs/requests" \
       -H 'Content-Type: application/json' \
       -d '{"job_type":"multi-party-approval-demo"}'

  # Auth enabled
  curl -X POST "${BASE_URL}/api/jobs/requests" \
       -H 'Content-Type: application/json' \
       -H "Authorization: Bearer ${TOKEN}" \
       -d '{"job_type":"multi-party-approval-demo"}'

── Cast an approval vote ────────────────────────────────────────────────────────

  # APPROVED — auth disabled (voter_id taken from session; empty session = system)
  curl -X POST "${BASE_URL}/api/jobs/requests/${REQ_ID}/tasks/${TASK_TYPE}/vote" \
       -H 'Content-Type: application/json' \
       -d '{"decision":"APPROVED","comments":"Reviewed and approved"}'

  # APPROVED — auth enabled (voter_id always taken from JWT, body field ignored)
  curl -X POST "${BASE_URL}/api/jobs/requests/${REQ_ID}/tasks/${TASK_TYPE}/vote" \
       -H 'Content-Type: application/json' \
       -H "Authorization: Bearer ${TOKEN}" \
       -d '{"decision":"APPROVED","comments":"Reviewed and approved"}'

  # REJECTED
  curl -X POST "${BASE_URL}/api/jobs/requests/${REQ_ID}/tasks/${TASK_TYPE}/vote" \
       -H 'Content-Type: application/json' \
       -d '{"decision":"REJECTED","comments":"Needs more testing"}'

── Get approval status (vote tally) ────────────────────────────────────────────

  curl "${BASE_URL}/api/jobs/requests/${REQ_ID}/tasks/${TASK_TYPE}/approval"

  # Auth enabled
  curl "${BASE_URL}/api/jobs/requests/${REQ_ID}/tasks/${TASK_TYPE}/approval" \
       -H "Authorization: Bearer ${TOKEN}"

── List all pending approvals ──────────────────────────────────────────────────

  curl "${BASE_URL}/api/approvals/pending"
  curl "${BASE_URL}/api/approvals/pending?page=0&page_size=20"

  # Auth enabled
  curl "${BASE_URL}/api/approvals/pending" \
       -H "Authorization: Bearer ${TOKEN}"

── Poll job state ────────────────────────────────────────────────────────────

  curl "${BASE_URL}/api/jobs/requests/${REQ_ID}"

── Workflow-specific task types ────────────────────────────────────────────────

  Workflow                     job_type                        MANUAL task_type
  ─────────────────────────────────────────────────────────────────────────────
  manual.yaml                  semi-automated                  make-doe, check-doe
  multi-party-approval.yaml    multi-party-approval-demo       team-approval
  approval-with-sla.yaml       approval-with-sla-demo          security-approval
  approval-auto-reject.yaml    approval-auto-reject-demo       hotfix-approval
  approval-unanimous.yaml      approval-unanimous-demo         board-approval
  cicd-approval-gate.yaml      cicd-approval-gate              production-approval
  secure-go-cicd.yaml          secure-go-cicd                  verify-production-deploy
  dr-playbook.yaml             dr-playbook                     verify-failover

── Test ALL workflows end-to-end ────────────────────────────────────────────────

  # No auth
  ./deploy-approval-workflows.sh --test

  # Auth enabled
  ./deploy-approval-workflows.sh --token <TOKEN> --test

  # Remote server with auth, custom voter
  ./deploy-approval-workflows.sh \
    --server http://queen.example.com:7777 \
    --token <TOKEN> \
    --voter bob \
    --test

── Note on voter identity ───────────────────────────────────────────────────────

  When auth is DISABLED:
    voter_id is taken from the session context (typically empty).
    The approval service will use an empty/system user ID.
    Policies with allowed_users/allowed_roles will DENY the vote unless
    the policy lists an empty string or no restrictions.
    For testing with auth disabled, use job definitions that have an
    open policy (no allowed_users, no allowed_roles).

  When auth is ENABLED:
    voter_id is ALWAYS taken from the JWT — the body field is ignored.
    The server enforces allowed_users/allowed_roles from the policy.
    Use --token with a token whose sub/user_id matches a listed approver.

USAGE
