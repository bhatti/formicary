# AI Software Factory — Project Context

## Vision

Build an **AI software factory with a learning feedback loop**: Slack-triggered AI workflows
handle the full SDLC (implement, review, audit, standup), and each run feeds learnings back
into the shared skill library (`you-got-skills`). The goal is to continuously reduce human
review burden by improving AI skills from evidence in merged PRs.

---

## Three-Repo Architecture

Every feature spans three repos. Always edit in this order:

```
you-got-skills  →  ai-dev-tools  →  formicary
(skill protocol)   (Python scripts)   (job YAMLs + routing)
```

### 1. `formicary` — Job Orchestrator
**Path:** `/Users/sbhatti/workplace/formicary`
**Role:** Kubernetes-native workflow engine. Accepts YAML job definitions, dispatches to ant workers running in K8s pods.

Key paths:
- `docs/examples/ai-*.yaml` — Job definitions for every AI workflow
- `docs/examples/deploy-ai-workflows.sh` — Deploys all YAML definitions + org configs
- `docs/examples/setup-slack-admin.sh` — Pushes Slack route table (SET_ROUTES=true default)
- `k8s/formicary-ant.yaml` — Ant worker deployment (imagePullPolicy: Always)
- `internal/types/organization.go` / `user.go` — Concurrency defaults (now 10)

**Critical behavior — ENTRYPOINT is overridden:**
Formicary overrides the Docker container ENTRYPOINT with its own shell. `entrypoint.sh`
**never runs** in task pods. All setup (YGS clone, settings.json, credentials) must happen
inside Python scripts.

**Job variables:** `{{.VarName}}` template syntax injected at submit time.
**Credentials:** Come from `ai-dev-credentials` K8s secret (mounted via `env_from`).
**Tailscale:** Task pods use `host_network: true` → access to `http://ai/bedrock`.

**Known bug — `max_limits` ignored:** `MaxLimits` in kubernetes_config.go uses
`resource.Quantity` which mapstructure can't deserialize from YAML strings. The
`max_limits` config section is always nil after load. Default was bumped to 8G in
`Validate()` as workaround.

### 2. `ai-dev-tools` — Python Workflow Scripts + Docker Image
**Path:** `/Users/sbhatti/workplace/ai-dev-tools`
**Role:** Python scripts running inside Formicary task pods. Each script implements one
workflow step.

Key paths:
- `scripts/common/claude_runner.py` — Wraps `claude` CLI; handles YGS install,
  settings.json, skill inventory injection, SKILLS_INVOKED tracking
- `scripts/common/config.py` — Env var loading, CamelCase→UPPER_SNAKE aliases, model IDs
- `scripts/review/run.py` — PR review pipeline; loads skill, inlines shared refs, runs Claude
- `scripts/standup/synthesize.py` — Standup synthesis; loads ygs-standup skill directly
- `scripts/slack/router.py` — Three-tier Slack dispatch
- `scripts/analyze/pr_fetcher.py` — Fetches PRs from GitHub/Bitbucket with comments/reviews
- `scripts/analyze/run_pr_audit.py` — PR audit analyzer
- `scripts/analyze/create_skill_pr.py` — Creates skill-improvement PRs against target repo
- `tests/test_pod_functional.py` — Pod-based functional tests (inject local scripts over image)

Docker image: `plexobject/ai-dev-tools:latest` (multi-arch amd64+arm64, Docker Hub)
Version file: `VERSION`

### 3. `you-got-skills` — Claude Skill Library
**Path:** `/Users/sbhatti/workplace/you-got-skills`
**Repo:** `https://github.com/bhatti/you-got-skills.git`
**Role:** Markdown SKILL.md files providing Claude with task-specific protocols.
Cloned into `~/.claude/skills/` inside every task pod by `_ensure_ygs_skills()`.

⚠️ **NEVER edit `~/.claude/skills/` — it is a read-only symlink cache.**
Always edit `/Users/sbhatti/workplace/you-got-skills/` then push.

Key skills:
- `skills/ygs-review-pr/SKILL.md` — PR review (4 domains)
- `skills/ygs-pr-audit/SKILL.md` — Cross-PR audit (spec, design, skills, practices)
- `skills/ygs-standup/SKILL.md` — Daily standup brief
- `skills/ygs-learn/SKILL.md` — Post-merge learnings extraction
- `skills/ygs-codebase-audit/SKILL.md` — Codebase health audit
- `skills/shared/review-scaffold.md` — Severity, finding format, quality bar
- `skills/shared/ownership-principles.md` — Review ownership protocol
- `skills/shared/tracker.md` — Credential gates + tracker abstraction

---

## Data Flow

```
Slack message
  → formicary queen (Go) — routes via SlackRoutes table
    → formicary ant (K8s pod) — runs plexobject/ai-dev-tools:latest
      → Python script
        → _ensure_ygs_skills() clones you-got-skills → ~/.claude/skills/
        → _load_skill_md("ygs-review-pr") reads + inlines shared refs
        → run_claude() invokes `claude --print --dangerously-skip-permissions`
          → Claude executes skill with Bash/Read/Write tools
        → ::add-task-context markers parsed from stdout → Formicary context vars
        → Slack post_findings / notify script posts result to thread
```

**Skill loading in ai-dev-tools:**
- Always embed SKILL.md content directly in the Claude prompt (NOT via Skill tool)
- Use `_inline_shared_refs()` to resolve `read shared/X.md` backtick references before
  passing to Claude — without this, Claude skips the reads due to "start immediately" rules
- The inlined skill grows from ~9KB to ~25KB (review-scaffold, ownership, tracker included)

**EXTRA_SKILLS_REPOS:** Org config lets teams inject skills from private repos
(Bitbucket/GitHub) via sparse-checkout. Repo-local `.claude/skills/review-pr/SKILL.md`
overrides take priority over the generic `ygs-review-pr`.

---

## Mandatory Testing Pyramid

**NEVER skip levels. NEVER go to e2e without passing pod tests.**

```
1. Unit tests (fast, no network, no K8s)
   cd /Users/sbhatti/workplace/ai-dev-tools
   python3 -m pytest tests/test_<module>.py -v

2. Pod functional tests (real K8s pod, local scripts injected over image)
   source ~/.zshrc
   cd /Users/sbhatti/workplace/ai-dev-tools
   PYTHONPATH=. python3 tests/test_pod_functional.py --tests <test-name>
   # Add --list to see all test names
   # SKIP_COPY=1 to test scripts from the docker image (no local inject)

3. E2E job (full formicary job submission, slow)
   source ~/.zshrc
   bash /Users/sbhatti/workplace/formicary/docs/examples/deploy-ai-workflows.sh --set-configs
   # then submit job via API
```

**Pod test mechanics:** Creates pod → copies local scripts over image → runs steps via
kubectl exec → parses `::add-task-context KEY::VALUE` markers → asserts → deletes pod.
Each test is self-contained and cleans up after itself.

---

## Deploy Sequence

```bash
# 1. Rebuild ai-dev-tools image (when scripts change)
cd /Users/sbhatti/workplace/ai-dev-tools
make docker-build NO_CACHE=1       # forces full rebuild, ~15 min, multi-arch

# 2. Restart ant (picks up new image — Always pull policy)
kubectl rollout restart deployment/formicary-ant -n default
kubectl rollout status deployment/formicary-ant -n default --timeout=120s

# 3. Redeploy workflows (when YAML job definitions change)
source ~/.zshrc
bash /Users/sbhatti/workplace/formicary/docs/examples/deploy-ai-workflows.sh \
  --create-k8s-secret --set-configs

# 4. Validate with pod tests (no Docker rebuild needed — copies local scripts)
cd /Users/sbhatti/workplace/ai-dev-tools
PYTHONPATH=. python3 tests/test_pod_functional.py \
  --tests review-skill-loading,jira-query,gh-query
```

**Shortcut for script-only changes:** Pod tests inject local scripts into the existing
image — no docker rebuild needed. Only rebuild when:
- New dependencies added to requirements.txt
- New files added that aren't injected by pod tests

---

## Slack Routing

Three-tier dispatch in `scripts/slack/router.py`:
1. `parse_verb` — exact verb lookup (no LLM)
2. `scan_triggers` — full-text scan against all trigger arrays in SlackRoutes (no LLM)
3. `classify_intent` — LLM fallback only when both fail

Route table managed via `setup-slack-admin.sh` (SET_ROUTES=true default).

**Tracker disambiguation:** Use explicit keyword prefixes in route triggers:
- `"gh review"`, `"github review"` → `ai-gh-review`
- `"jira review"`, `"bb review"` → `ai-jira-review`
- Bare `"implement"` falls to `ai-jira-implement` (org's DefaultTracker)

**NEVER use `{{.DefaultTracker}}` in tracker-specific YAMLs.** Org config overrides it,
causing GH jobs to use Jira and vice versa. Hardcode the tracker in each YAML variant:
`DEFAULT_TRACKER: "github"` in `ai-gh-*.yaml`, `DEFAULT_TRACKER: "jira"` in `ai-jira-*.yaml`.

---

## AI Workflows

| Workflow | Job Types | Description |
|----------|-----------|-------------|
| Implement | `ai-gh-implement`, `ai-jira-implement` | Full TDD implementation from GitHub/Jira issue |
| Review | `ai-gh-review`, `ai-jira-review` | PR review using ygs-review-pr skill |
| PR Audit | `ai-gh-pr-audit`, `ai-jira-pr-audit` | Cross-PR audit: spec/design/skills/practices gaps |
| Codebase Audit | `ai-gh-codebase-audit`, `ai-jira-codebase-audit` | Codebase health and technical debt |
| Standup | `ai-standup` | Daily standup brief from Jira/GitHub + Slack |
| Query | `ai-jira-query`, `ai-gh-query` | Ad-hoc queries against Jira/GitHub |
| Adhoc | `ai-adhoc` | Run any skill with `--skill` and `--prompt` args |
| Skill | `ai-skill` | Generic skill invocation with flag parsing, repo cloning, optional services. Slack: `skill <name> [--repo] [--branch] [--tracker] [--model] [--service] [-- instructions]` |

**PR Audit feedback loop:**
```
audit N merged PRs → findings report → create PR with skill improvements
→ poll PR for human feedback → apply feedback → re-audit → repeat
```

---

## Lessons Learned / Mistakes to Avoid

### Skills
- ❌ **NEVER edit `~/.claude/skills/`** — it's a symlink cache, edits are lost on next sync
- ✅ Always edit `/Users/sbhatti/workplace/you-got-skills/` and push
- ❌ **NEVER invoke skills via the Skill tool in scripts** — use `_load_skill_md()` to read
  SKILL.md directly and embed in the Claude prompt
- ✅ Use `_inline_shared_refs()` to resolve `read shared/X.md` refs before passing to Claude

### Job Variables & Trackers
- ❌ `{{.DefaultTracker}}` in tracker-specific YAMLs gets overridden by org config
- ✅ Hardcode `DEFAULT_TRACKER: "github"` or `"jira"` in each YAML variant
- ❌ Don't put Jira env vars in GH YAMLs or GH env vars in Jira YAMLs

### Tracker Resolution — Prompt Always Wins
**Any Slack prompt/message can override `DEFAULT_TRACKER` from the YAML config.**
Effective tracker is resolved in this strict priority order (highest → lowest):

```
1. Explicit --tracker flag in message  →  --tracker jira  /  --tracker github
2. Derived from PR URL in message       →  github.com/... → github
                                           atlassian.net/... → jira
3. DEFAULT_TRACKER from job config/env  →  set in YAML environment block
```

This is implemented by `_resolve_effective_tracker(config, slack_flags)` in
`scripts/analyze/run_pr_audit.py`. The resolved tracker is written back to
`config["DEFAULT_TRACKER"]` before any filter or branch logic runs, so ALL downstream
code (board filter, branch selection, PR fetch dispatch) sees the same value.

**Rules when adding new scripts/flags that depend on tracker:**
- ✅ Call `_resolve_effective_tracker()` (or the equivalent helper) BEFORE using tracker
- ✅ Parse `--tracker` in `_parse_slack_flags` so prompts can always override
- ❌ NEVER check `config.get("DEFAULT_TRACKER")` before resolving effective tracker from prompt
- ❌ NEVER gate a filter/flag on a tracker value derived only from the YAML default

### Pod / Docker / Entrypoint
- ❌ Formicary overrides Docker ENTRYPOINT — `entrypoint.sh` never runs in pods
- ✅ All setup (YGS clone, settings.json, credential setup) must be in Python scripts
- ❌ `AiDevToolsDebug=1` clones from GitHub main — if local changes aren't pushed, pod runs old code
- ✅ Always push before e2e testing when using AiDevToolsDebug mode

### Poll-PR Task
- ❌ `poll-pr` expects per-issue directories: `/workspace/{issue_id}/pr.json`
- ✅ Any new workflow using poll-pr must create `/workspace/{issue_id}/` and copy files there
- The implement workflow uses per-issue directories; pr-audit does NOT (bug to fix)

### Testing
- ❌ Never jump to e2e without passing unit + pod tests first
- ❌ Pod tests don't catch multi-task pipeline issues (e.g., poll-pr directory structure)
- ✅ Pod tests catch 90% of script failures in 30-60s without needing a full job submit
- ❌ Don't use sleep or timing-based test assertions — use condition variables or polling

### Artifact Paths & Cross-Task Data Flow
- ❌ Formicary only collects files listed in `artifacts.paths:` in the YAML
- ✅ Always add new output files to the relevant task's artifact list

**Each task runs in its own pod with its own empty volumes.** Data only flows between
tasks via artifacts:
1. **Upload:** The producing task lists files in `artifacts.paths:` → formicary zips and
   uploads them to S3/seaweedfs at task completion.
2. **Download:** A consuming task lists the producer in `dependencies:` → formicary
   downloads and extracts the producer's artifact zip into the consumer's workspace
   **before** the consumer's script runs.
3. **Transitive:** If task C depends on B which depends on A, C gets B's artifacts but
   NOT A's. To get A's artifacts in C, list both: `dependencies: [A, B]`.

**Common mistake:** A `report` task depends only on the last pipeline task but needs
`scope.json` from an earlier task. Fix: add all data-producing tasks to `dependencies`.

**Report tasks need these env vars** for Slack artifact links:
- `FORMICARY_PUBLIC_URL: "{{.FormicaryPublicURL}}"` — base URL for artifact download links
- `JOB_ID: "{{.JobID}}"` — used to build artifact download URLs
- Both must also be declared in `job_variables:` with empty defaults

**Artifact link filename MUST match the actual file written, not a display name:**
- The `filename` param in `post_report()` is used by BOTH Slack file upload AND the
  fallback artifact URL (`build_artifact_links` in `upload_html_report`).
- If the script writes `reports/report.html` but passes `filename="mq_report.html"`,
  the fallback link becomes `file=reports/mq_report.html` → 404.
- Rule: `filename` in `post_report()` must equal the basename of the file written to
  `reports/` dir. E.g., write `reports/report.html` → `filename="report.html"`.
- Existing patterns: standup→`report.html`, audit→`audit_report.html`,
  pr-audit→`pr_audit_report.html`, mq-report→`report.html`.

**Tracker-agnostic tasks** (any script using `_shared.resolve_tracker`) need:
- `DEFAULT_TRACKER: "{{.DefaultTracker}}"`
- `BITBUCKET_WORKSPACE: "{{.BitbucketWorkspace}}"`
- `BITBUCKET_REPO: "{{.BitbucketRepo}}"`
- `GH_ORG: "{{.GitHubOrg}}"` / `GH_REPO: "{{.GitHubRepo}}"`

**PRNumber with Jira tracker:** When `DefaultTracker=jira`, Slack routes set PRNumber
to a Jira ticket ID (e.g. `PROJ-40913`), not a numeric PR number. Scripts must call
`resolve_pr_number()` from `scripts.mq._shared` to search Bitbucket for the actual
numeric PR ID before calling any Bitbucket PR API.

### Concurrency
- Formicary `max_concurrency` defaults were 1 (effectively serialized all jobs)
- Fixed: `organization.go` → 10, `user.go` → 10
- If jobs queue unexpectedly, check the SQLite directly:
  ```
  /var/lib/rancher/k3s/storage/pvc-*/formicary.db
  UPDATE formicary_orgs SET max_concurrency = 10;
  ```

### Exit Codes
- ❌ Scripts that fail silently with exit 0 hide failures from Formicary
- ✅ Always `sys.exit(1)` on meaningful failures; verify exit codes during e2e

### API Verification
- ❌ Formicary's PUT API may return HTTP 200 but not persist (validation may reset)
- ✅ Always verify with GET after PUT; fall back to SQLite for critical config

---

## Infrastructure

- **Formicary queen + ant:** Running on EC2 (K3s cluster)
- **Bedrock endpoint:** `http://ai/bedrock` (Tailscale address on EC2)
- **K8s credentials secret:** `ai-dev-credentials` (all tokens: Jira, Bitbucket, Slack, GitHub)
- **SQLite location:** `/var/lib/rancher/k3s/storage/pvc-*/formicary.db`
- **Settings.json:** Written by `_ensure_ygs_skills()` if missing (Bedrock config + permissions)
- **Docker Hub:** `plexobject/ai-dev-tools:latest`

---

## Current Open Work (as of 2026-09-08)

### Pending — PR Audit
1. Fix `poll-pr` directory structure for pr-audit — add pre-step creating `/workspace/pr-audit/`
   and copying `pr.json`, `poll_state.json`, etc.
2. Verify `create_skill_pr.py` on GitHub main has `clone_by_tracker` + `sys.exit(1)` on clone failure
3. Rebuild docker image + restart ant after fixes
4. Re-run pod tests (4 tests: pr-audit-gh-fetch, pr-audit-gh-full, gh-query, jira-query)
5. Deploy workflows and run e2e for both GH and Jira
6. Verify report quality: PR IDs, skill assessment, recommended skill updates

### Pending — PR Audit Quality Improvements (see we-added-support-for-tranquil-sky.md)
1. **Data quality:** Fix `review_decision` (derive from `reviews` array, not null `reviewDecision`)
2. **Bot split:** Separate CI bots from code-review bots in `classify_comments()` (3 buckets)
3. **Raw artifacts:** Save `pr_data_raw.json` and `pr_reviews_raw.json` for debugging
4. **Cross-team learning:** `create_ygs_pr.py` — creates PR against you-got-skills when
   evidence cites 3+ PRs for same gap pattern
5. **Targeted deep review:** Phase 2.5 in ygs-pr-audit SKILL.md — invoke ygs-security-review
   on security-sensitive PRs, ygs-review-deep on large under-reviewed PRs

### Pending — Post-Merge PR Health Check (see we-are-working-on-serene-cosmos.md)
- Extend `jira/learn.py` and `gh/learn.py` to fetch single PR context and run Phase 0
  health check before ygs-learn; post combined report back to PR and issue

### Pending — Slack URL Routing
- `implement https://github.com/...` still falls to `ai-jira-implement` (no keyword)
- Need URL-content-based routing in router.py without hardcoded Go logic

### Pending — Infrastructure
- S3/seaweedfs port 4443 DNAT not forwarding to pods
- `_inline_shared_refs` should also be applied in `synthesize.py` for ygs-standup

---

## Org Config Reference

```
AiDevToolsDebug = 1/0   (1 = clone ai-dev-tools from GitHub main instead of using image)
DefaultTracker = jira   (overrides {{.DefaultTracker}} in all job YAMLs)
GitHubOrg = bhatti
GitHubRepo = todo-sample
BitbucketWorkspace = my-org
BitbucketRepo = my-repo
```

---

## Formicary Auth Configuration

Auth is **disabled by default** in formicary queen. No flags or env vars needed.

**Source:** `queen/config/server_config.go:164`
```go
viper.SetDefault("common.auth.enabled", "false")
```

When `auth.enabled=false`, `nonSessionMiddleware` fires on every request and sets
`web.AuthDisabled=true`. All `/api/*` endpoints accept requests without a JWT token.

**To enable auth** (production), add to the queen config YAML:
```yaml
common:
  auth:
    enabled: true
    jwt_secret: "<secret>"
```

Or via env: `COMMON_AUTH_ENABLED=true`.

**Health / reachability check (no auth required regardless of setting):**
`GET /api/health` — returns `{"status":"ok"}`.
`GET /api/metrics` — Prometheus metrics endpoint.

---

## Formicary Combined Mode (Queen + Embedded Ant)

Formicary supports three deployment modes:

| Mode | How | Use case |
|------|-----|----------|
| **Queen only** | `formicary queen --config queen.yaml` | Standalone leader, no workers |
| **Ant only** | `formicary ant --config ant.yaml` | Worker only, connects to queen |
| **Combined (queen + ant)** | Add `embedded_ant:` section to queen config YAML | All-in-one; queen runs built-in ant worker |

**Enabling combined mode** — add `embedded_ant:` to the queen YAML config:

```yaml
# queen-combined.yaml
common:
  id: queen1
  http_port: 7777
  auth:
    enabled: false       # auth off by default; omit for same effect

embedded_ant:            # presence of this block activates the embedded ant
  tags:
    - embedded
    - default
  methods:
    - KUBERNETES
    - SHELL
    - HTTP_POST_JSON
```

**Source:** `queen/config/server_config.go:219-221`
```go
if config.EmbeddedAnt != nil {
    config.EmbeddedAnt.Common = config.Common
    config.EmbeddedAnt.Common.ID = config.Common.ID + "_embedded_ant"
```

**`HasEmbeddedAnt()`** (`server_config.go:392`) returns `config.EmbeddedAnt != nil`.

There is **no** `--standalone` CLI flag. Combined mode is config-only.

**Using formicary as a service sidecar (for integ-tests):**
Submit the job with `ServiceImage=plexobject/formicary:latest` and a config file
mounted or generated at startup. Since auth is off by default, no credentials needed:
```bash
ServiceCommand: "formicary queen --config /tmp/queen.yaml"
# Or generate a minimal config inline:
ServiceCommand: "sh -c 'printf \"common:\\n  http_port: 7777\\nembedded_ant:\\n  tags: [embedded]\\n\" > /tmp/q.yaml && formicary queen --config /tmp/q.yaml'"
```

---

## ai-skill Workflow (added 2026-09)

Generic skill invocation job. Slack: `skill <name> [--repo] [--branch] [--service <image>] [-- instructions]`

**Task pipeline:** `run` → `post` → `done` (on failure: `notify-error`)

**Key files:**
- `formicary/docs/examples/ai-skill.yaml` — Job definition with service sidecar support
- `ai-dev-tools/scripts/skill/run_skill.py` — Parses RAW_ARGS, clones repo, runs Claude, writes reports
- `ai-dev-tools/scripts/skill/flags.py` — Flag parser (`--service`, `--service-port`, `--service-cmd`, `--service-args`)
- `ai-dev-tools/scripts/skill/post.py` — Post task: reads `skill_result.json` + `reports/report.md`, posts HTML report to Slack
- `ai-dev-tools/.claude/skills/integ-tests/SKILL.md` — Integration test skill (service health check + test runner discovery)

**integ-tests skill lives in `ai-dev-tools/.claude/skills/`, NOT you-got-skills.**
It is infrastructure, not a general-purpose skill. Copied into pods via `_DIRS_TO_COPY`.

**Service sidecar:** Set `ServiceImage` job variable at submission time. The sidecar is
started before the `run` task. Flags `--service-cmd`/`--service-args` from Slack are
**parsed into the prompt context only** — the actual command must be set via `ServiceCommand`/
`ServiceArgs` job variables. To pass env vars to the sidecar, embed them in `ServiceCommand`:
`env KEY=VAL formicary queen --config /tmp/q.yaml`.

**k8s executor limitation:** Per-service env injection (`ServiceEnvVars`) is not supported
by the formicary k8s executor (no `Env` field on the `Service` struct). Workaround above.

**Quoted multi-word flags:** `--service-cmd "formicary queen"` works — the regex in
`flags.py` matches `"[^"]*"|\S+` and strips surrounding quotes.

**Container image override (`RunImage`):** The `run` task defaults to `plexobject/ai-dev-tools:latest`.
Override at job submission by passing `RunImage` as a job param:
- API: `"RunImage":"node:22-bookworm"` in `params`
- Slack: add a new route in `deploy-ai-workflows.sh` with `params: {RunImage: "<image>"}` — no code changes needed.
The image must be set at submission time (before the container starts); it cannot be a RAW_ARGS flag.

---

## Useful Commands

```bash
# Pod functional tests
source ~/.zshrc
cd /Users/sbhatti/workplace/ai-dev-tools
PYTHONPATH=. python3 tests/test_pod_functional.py --list
PYTHONPATH=. python3 tests/test_pod_functional.py --tests review-skill-loading
PYTHONPATH=. python3 tests/test_pod_functional.py --tests pr-audit-gh-fetch,gh-query,jira-query

# E2E functional tests
PYTHONPATH=/Users/sbhatti/workplace/ai-dev-tools \
  python3 /Users/sbhatti/workplace/ai-dev-tools/tests/test_functional_workflows.py \
  --tests standup --timeout 900

# Docker rebuild
cd /Users/sbhatti/workplace/ai-dev-tools
make docker-build NO_CACHE=1

# Ant restart
kubectl rollout restart deployment/formicary-ant -n default

# Deploy workflows
source ~/.zshrc
bash /Users/sbhatti/workplace/formicary/docs/examples/deploy-ai-workflows.sh \
  --create-k8s-secret --set-configs

# Submit e2e job via API (example: pr audit)
curl -sk -X POST "${FORMICARY_URL}/api/jobs/requests" \
  -H "Authorization: Bearer $FORMICARY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"job_type":"ai-gh-pr-audit","params":{"NPrs":"20","PrAuditFocus":"all"}}'

# Trigger a cron job (DO NOT resubmit — use /trigger on the existing job request ID)
# Cron jobs are singletons: resubmitting fails with UNIQUE constraint because the
# cron entry already exists. Use the trigger endpoint to fire it immediately:
#
#   POST /api/v1/jobs/requests/:id/trigger
#
# To find the cron job ID:
JOB_TYPE="ai-standup-jira"
CRON_ID=$(curl -sk -H "Authorization: Bearer $FORMICARY_TOKEN" \
  "${FORMICARY_URL}/api/v1/jobs/requests?job_type=${JOB_TYPE}&job_state=PENDING&page_size=1" \
  | python3 -c "import sys,json; recs=json.load(sys.stdin).get('records',[]); print(recs[0]['id'] if recs else '')")
# Then trigger it:
curl -sk -X POST -H "Authorization: Bearer $FORMICARY_TOKEN" \
  "${FORMICARY_URL}/api/v1/jobs/requests/${CRON_ID}/trigger"
#
# test_functional_workflows.py::trigger_cron_job() already implements this correctly.

# Verify image contents (check if scripts made it in)
docker pull plexobject/ai-dev-tools:latest
docker run --rm --entrypoint grep plexobject/ai-dev-tools:latest \
  -n 'clone_by_tracker' /app/scripts/analyze/create_skill_pr.py
```
