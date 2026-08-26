#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# check-credentials.sh — local credential health check for Formicary workers.
#
# Verifies that all credentials needed by AI workflow jobs are set and valid
# BEFORE attempting to deploy. Runs API calls against live endpoints.
#
# Usage:
#   ./scripts/check-credentials.sh                # check all available creds
#   ./scripts/check-credentials.sh --gh-only      # GitHub + Formicary only
#   ./scripts/check-credentials.sh --jira-only    # Jira/Bitbucket + Formicary only
#
# Env vars checked (all read from environment, sourced from ~/.zshrc if missing):
#   FORMICARY_URL, FORMICARY_TOKEN
#   GH_TOKEN, GH_ORG, GH_REPO, SSH_PRIVATE_KEY
#   JIRA_API_TOKEN, JIRA_EMAIL, JIRA_BASE_URL
#   BITBUCKET_TOKEN, BITBUCKET_USERNAME, BITBUCKET_WORKSPACE
#   SLACK_BOT_TOKEN
#
# Exit code:
#   0 = all checked credentials passed
#   1 = one or more credentials failed (see output)
#
set -euo pipefail

# ── Colour helpers ────────────────────────────────────────────────────────────
_RED='\033[0;31m'; _GREEN='\033[0;32m'; _YELLOW='\033[1;33m'
_CYAN='\033[0;36m'; _BOLD='\033[1m'; _RESET='\033[0m'

ok()   { printf "  ${_GREEN}✓${_RESET}  %s\n" "$*"; }
fail() { printf "  ${_RED}✗${_RESET}  %s\n" "$*"; _FAILED=1; }
warn() { printf "  ${_YELLOW}!${_RESET}  %s\n" "$*"; }
hdr()  { printf "\n${_BOLD}${_CYAN}%s${_RESET}\n" "$*"; }

_FAILED=0
_GH_ONLY=false
_JIRA_ONLY=false

for _arg in "$@"; do
  case "$_arg" in
    --gh-only)   _GH_ONLY=true ;;
    --jira-only) _JIRA_ONLY=true ;;
  esac
done

# ── Autodetect from ~/.zshrc / ~/.bashrc ─────────────────────────────────────
# Reads `export VAR=value` lines from RC files. Values containing $VAR references
# are expanded using already-exported env vars (e.g. FORMICARY_URL="${QUEEN_IP}.nip.io").
_autodetect_var() {
  local _var="$1"
  if [[ -z "${!_var:-}" ]]; then
    for _rc in "$HOME/.zshrc" "$HOME/.bashrc"; do
      [[ -f "$_rc" ]] || continue
      local _line _val
      _line="$(grep -E "^export ${_var}=" "$_rc" | tail -1)" || true
      if [[ -n "$_line" ]]; then
        _val="$(echo "$_line" | sed "s/^export ${_var}=//;s/^['\"]//;s/['\"]$//")"
        if [[ -n "$_val" ]]; then
          # Expand $VAR / ${VAR} references using already-known env vars
          _val="$(eval echo "\"${_val}\"" 2>/dev/null || echo "${_val}")"
          if [[ -n "$_val" ]]; then
            export "${_var}=${_val}"
            return 0
          fi
        fi
      fi
    done
  fi
}

# QUEEN_IP must be resolved first so FORMICARY_URL="https://${QUEEN_IP}.nip.io" expands correctly
for _v in QUEEN_IP FORMICARY_URL FORMICARY_TOKEN \
          GH_TOKEN GH_ORG GH_REPO SSH_PRIVATE_KEY \
          JIRA_API_TOKEN JIRA_EMAIL JIRA_BASE_URL \
          BITBUCKET_TOKEN BITBUCKET_USERNAME BITBUCKET_WORKSPACE \
          SLACK_BOT_TOKEN; do
  _autodetect_var "$_v"
done

# ── HTTP helper ───────────────────────────────────────────────────────────────
# _http_check <label> <url> <auth_header> <expect_field>
# Passes if the response is 2xx and (optionally) contains expect_field.
_http_check() {
  local _label="$1" _url="$2" _auth="$3" _field="${4:-}"
  local _body _code
  local _curl_args=(-sk -w "\n%{http_code}")
  [[ -n "$_auth" ]] && _curl_args+=(-H "$_auth")
  _curl_args+=("$_url")
  _body="$(curl "${_curl_args[@]}" 2>/dev/null)" || {
    fail "$_label: curl failed (network error?)"
    return
  }
  _code="${_body##*$'\n'}"
  _body="${_body%$'\n'*}"
  if [[ "$_code" -lt 200 || "$_code" -ge 300 ]]; then
    fail "$_label: HTTP $_code"
    return
  fi
  if [[ -n "$_field" ]] && ! echo "$_body" | grep -q "\"$_field\""; then
    fail "$_label: response missing field '$_field'"
    return
  fi
  ok "$_label"
}

# ── Formicary ────────────────────────────────────────────────────────────────
hdr "Formicary"
if [[ -z "${FORMICARY_URL:-}" ]]; then
  fail "FORMICARY_URL not set (set QUEEN_IP in ~/.zshrc and FORMICARY_URL=https://\${QUEEN_IP}.nip.io)"
elif [[ -z "${FORMICARY_TOKEN:-}" ]]; then
  fail "FORMICARY_TOKEN not set — create one at ${FORMICARY_URL}/dashboard → Profile → API Tokens"
else
  # Reachability: any HTTP response (not 000/network error) means server is up
  _reach_code="$(curl -sk --max-time 10 -o /dev/null -w "%{http_code}" \
    "${FORMICARY_URL}/api/v1/ants" 2>/dev/null)" || _reach_code="000"
  if [[ "$_reach_code" == "000" ]]; then
    fail "Formicary server unreachable at ${FORMICARY_URL}"
  else
    ok "Formicary server reachable (HTTP ${_reach_code})"
    # Token validity: must return 200 with total_records field
    _http_check "Formicary token valid" \
      "${FORMICARY_URL}/api/v1/ants" \
      "Authorization: Bearer ${FORMICARY_TOKEN}" "total_records"
  fi
fi

# ── GitHub ───────────────────────────────────────────────────────────────────
if [[ "$_JIRA_ONLY" != "true" ]]; then
  hdr "GitHub"
  if [[ -z "${GH_TOKEN:-}" ]]; then
    fail "GH_TOKEN not set (needed by ai-gh-implement workflows)"
  else
    _http_check "GH_TOKEN valid (github.com/user)" \
      "https://api.github.com/user" \
      "Authorization: token ${GH_TOKEN}" "login"

    if [[ -n "${GH_ORG:-}" && -n "${GH_REPO:-}" ]]; then
      _http_check "Repo ${GH_ORG}/${GH_REPO} accessible" \
        "https://api.github.com/repos/${GH_ORG}/${GH_REPO}" \
        "Authorization: token ${GH_TOKEN}" "full_name"
    else
      warn "GH_ORG / GH_REPO not set — repo access check skipped"
    fi
  fi

  # SSH key validation
  if [[ -z "${SSH_PRIVATE_KEY:-}" ]]; then
    fail "SSH_PRIVATE_KEY not set — git clone/push will fail"
  else
    _tmp="$(mktemp)"
    printf '%s\n' "${SSH_PRIVATE_KEY}" > "$_tmp"
    if ssh-keygen -y -f "$_tmp" > /dev/null 2>&1; then
      ok "SSH_PRIVATE_KEY is a valid private key"
    else
      fail "SSH_PRIVATE_KEY is invalid — must be PEM content (not a file path). Use: export SSH_PRIVATE_KEY=\"\$(cat ~/.ssh/id_rsa)\""
    fi
    rm -f "$_tmp"
  fi
fi

# ── Jira ─────────────────────────────────────────────────────────────────────
if [[ "$_GH_ONLY" != "true" ]]; then
  hdr "Jira"
  if [[ -z "${JIRA_API_TOKEN:-}" || -z "${JIRA_EMAIL:-}" || -z "${JIRA_BASE_URL:-}" ]]; then
    warn "JIRA_API_TOKEN / JIRA_EMAIL / JIRA_BASE_URL not fully set — skipping Jira checks"
  else
    _b64="$(printf '%s:%s' "${JIRA_EMAIL}" "${JIRA_API_TOKEN}" | base64 | tr -d '\n')"
    _http_check "Jira token valid (${JIRA_BASE_URL})" \
      "${JIRA_BASE_URL}/rest/api/2/myself" \
      "Authorization: Basic ${_b64}" "accountId"
  fi

  # ── Bitbucket ───────────────────────────────────────────────────────────────
  hdr "Bitbucket"
  if [[ -z "${BITBUCKET_TOKEN:-}" || -z "${BITBUCKET_WORKSPACE:-}" ]]; then
    warn "BITBUCKET_TOKEN / BITBUCKET_WORKSPACE not set — skipping Bitbucket checks"
  else
    _http_check "Bitbucket workspace accessible" \
      "https://api.bitbucket.org/2.0/workspaces/${BITBUCKET_WORKSPACE}" \
      "Authorization: Bearer ${BITBUCKET_TOKEN}" "slug"
  fi
fi

# ── Slack ────────────────────────────────────────────────────────────────────
hdr "Slack (worker bot token)"
if [[ -z "${SLACK_BOT_TOKEN:-}" ]]; then
  warn "SLACK_BOT_TOKEN not set — job Slack notifications will be disabled"
else
  _http_check "SLACK_BOT_TOKEN valid" \
    "https://slack.com/api/auth.test" \
    "Authorization: Bearer ${SLACK_BOT_TOKEN}" "ok"
fi

# ── Summary ──────────────────────────────────────────────────────────────────
printf "\n"
if [[ $_FAILED -eq 0 ]]; then
  printf "${_GREEN}${_BOLD}All credential checks passed.${_RESET}\n\n"
else
  printf "${_RED}${_BOLD}One or more credential checks failed. Fix the above before deploying.${_RESET}\n\n"
  exit 1
fi
