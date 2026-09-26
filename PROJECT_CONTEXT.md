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

### `pass_through_args` — When to use it

Routes where the *target script* owns its own flag parser must set `pass_through_args: true`
in `slack-routes.json`. Without it, `service.go`'s `extractFlags()` strips every `--key value`
pair from the trailing text before binding it to `IdVar`, and the target script never sees them.

```json
{"triggers":["skill"],"job_type":"ai-skill","id_var":"RawArgs","pass_through_args":true, ...}
```

**Set `pass_through_args: true` when:**
- The route's `id_var` is `RawArgs` (or similar "pass everything as-is")
- The target script (e.g. `run_skill.py`) calls its own `parse_skill_flags()` / `argparse`
- Any `--head`, `--base`, `--branch` or other domain flags must reach the script verbatim

**Leave `pass_through_args` unset (defaults false) when:**
- The route wants formicary to extract `--repo`, `--branch`, `--model` into job params
  automatically (most routes: implement, review, pr-audit, etc.)

The flag is propagated: `SlackRouteConfig.PassThroughArgs` → `RouteResult.PassThroughArgs`
→ `service.go` skips `extractFlags` for that request.

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

### `resolve_tracker` URL Override — `scripts/mq/_shared.py`

`scripts/mq/_shared.py:resolve_tracker(config, repo_url="")` resolves the effective
tracker for MQ scripts (clone, PR fetch, etc.) using the same priority as above:

```python
# URL domain overrides DEFAULT_TRACKER from config
tracker = resolve_tracker(config, repo_url=repo or "")
# → "github" if repo_url contains "github.com"
# → "bitbucket" if repo_url contains "bitbucket.org"
# → from config["DEFAULT_TRACKER"] otherwise
```

**Always pass `repo_url` when a `--repo` flag is present.** Without it, a config with
`DEFAULT_TRACKER=jira` will call `clone_by_tracker` with "bitbucket" even for a GitHub URL,
cloning the wrong org's repo.

This mirrors `scripts/skill/flags.py:resolve_tracker(flags, config)` — the pattern is DRY
across both paths. The MQ version takes `(config, repo_url)` to match its calling convention.

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

## ai-contract-test Workflow (added 2026-09)

Contract recording + fuzzing pipeline via api-mock-service proxy.
Slack: `contract-test <repo-url> --service <docker-image>`

**Task pipeline:** `record` → `fuzz` → `report` → `done` (on failure: `notify-error`)

**Key files:**
- `formicary/docs/examples/ai-contract-test.yaml` — Job definition
- `ai-dev-tools/scripts/contract/record.py` — Wait for AMS + service, drive endpoints through proxy
- `ai-dev-tools/scripts/contract/fuzz.py` — Replay contracts, run security probes, write JUnit XML
- `ai-dev-tools/scripts/contract/_shared.py` — `wait_for_service`, `probe_through_proxy`, `run_security_probes`, `discover_endpoints`

**Bugs found and fixed during setup (2026-09-25/26):**

**Bug 1 — `ServiceImage` vs `Service` param mismatch (`proxy_used: false`)**
- `--service <image>` → `flagToPascal("service")` = `"Service"`, not `"ServiceImage"`
- All `{{.ServiceImage}}` references in YAML + Go test used wrong name → services never spawned → AMS never started → proxy_used always false
- Fix: renamed `ServiceImage` → `Service` everywhere in YAML and Go test

**Bug 2 — `challenge_` keyword false positive in cred_leak check**
- `"challenge_"` substring matched `challenge_id`, `challenge_name` in `/api/Challenges` response
- WrongSecrets normal challenge listing flagged as credential exposure
- Fix: replaced `"challenge_"` with `"challenge_answer"` and `"wrongsecrets_flag"` in `_shared.py`

**Bug 3 — Bootstrap double clone → pod OOM (root cause took 4 days)**
The bootstrap step calls `ensure_debug_mode()` via `python -c "..."`. When `AI_DEV_TOOLS_DEBUG=1`,
`ensure_debug_mode()` calls `os.execv(sys.executable, [sys.executable] + sys.argv)` to re-exec.
From a `-c` invocation, `sys.argv = ["-c"]` so the re-exec becomes `python -c` (no code) → fails silently.
This means the marker in `bootstrap.py` (written before `os.execv`) IS written — but only in the
new bootstrap.py. The OLD docker image runs OLD `bootstrap.py` (no marker logic), clones the code,
then os.execv fails. `/app/scripts` is updated but marker is never written.
When the next script step (`python -m scripts.contract.record`) runs, it calls `ensure_debug_mode()`
again via `load_config()` — `DEBUG=1`, no marker → clones AGAIN → two concurrent clones + JVM
startup = OOM → pod evicted.

**Wrong fixes attempted:**
- ❌ Inline `.touch()` after `ensure_debug_mode()` in the `-c` command: `os.execv` replaces
  the process — the `.touch()` is dead code and never executes
- ❌ Increasing `ServiceMemoryLimit` to 3G without fixing the double clone: pod still OOMed

**Correct fix:**
Add a SEPARATE script line `touch /tmp/.adt_bootstrap_done` AFTER the bootstrap line.
Formicary spawns each script line as an independent process — the touch runs in a new process
unaffected by `os.execv` in the previous step. All 17 other YAMLs updated the same way.

```yaml
script:
  - python -c "from scripts.common.bootstrap import ensure_debug_mode; ensure_debug_mode()" 2>/dev/null || true
  - touch /tmp/.adt_bootstrap_done   # ← this is a separate process; os.execv in step above cannot kill it
  - python -m scripts.contract.record
```

**Bug 4 — record.py probed WrongSecrets before JVM was ready → OOM**
- record.py waited for AMS (fast Go binary) but NOT for WrongSecrets (Spring Boot JVM, 30-90s startup)
- Once AMS was ready, `probe_through_proxy` hit WrongSecrets during peak startup memory → OOM
- Fix: added `wait_for_service(service_url, "service-under-test", retries=45, delay=2.0)` in
  `record.py` after the AMS wait — ensures WrongSecrets is serving before any probes start

**Memory budget (final):**
- WrongSecrets service: `4G` limit (Spring Boot peaks at 2-2.5G at startup; 4G gives headroom)
- AMS service: `512Mi` (Go binary)
- Record main container: `512Mi` (pure HTTP — was 2G, wasted node memory WrongSecrets needed)
- Fuzz main container: `2G` (HTTP + YAML processing — was 4G)
- Total record pod: ~5G; fuzz pod: ~6.5G

**Bug 5 — AMS service cannot write recordings to main container's workspace (pod test passes, e2e fails)**

This is the most dangerous class of bug: pod functional tests pass but the real Kubernetes job is silently broken.

**Root cause:** In the pod functional test (`_POD_MANIFEST_WITH_SERVICES`), all containers (`main`, `wrongsecrets`, `api-mock-service`) explicitly mount the same `workspace` emptyDir at `/workspace`. AMS writes recordings to `/workspace/recordings` and the main container reads them — works fine.

In the formicary YAML, `volumes:` is only defined under `container:` (main container). Services get their volume mounts from `adapter.go` as:
```go
volumeMounts := service.GetKubernetesVolumes().AddVolumeMounts(u.config.Kubernetes.Volumes.GetVolumeMounts())
```
This starts from the ant config's global volumes + the service's OWN declared volumes. It does NOT include the main container's volumes. So the AMS container starts without `/workspace` mounted, writes recordings to its own local overlay filesystem, and the main container's `/workspace/recordings` stays empty.

The fuzz task then downloads an empty recordings directory, discovers 0 endpoints, runs 0 probes, and always reports `findings=0` regardless of what the service exposes.

**Fix:** Add `volumes:` to the AMS service definition in the YAML. `addVolumes` deduplicates by name (no-op if `workspace` already exists in pod spec from main container), but `AddVolumeMounts` still adds the mount to the AMS container — both share the same emptyDir.

```yaml
    - name: api-mock-service
      ...
      volumes:
        empty_dir:
          - name: workspace        # same name as main container's workspace
            mount_path: /workspace # AMS now shares /workspace with main container
```

**Lesson:** The pod test manifest MUST mirror the formicary YAML's volume configuration exactly. If the pod test shares volumes that the YAML doesn't, the pod test gives a false green.

**Key gotchas:**
- `args:` under services in YAML is silently ignored — no `Args` field in formicary `Service` struct. Only `Command []string` is passed to `ToContainer`. AMS works without args because it uses default ports.
- `env:` on services also not supported — no `Env` field. To pass JVM flags, increase memory limit instead.
- Template `{{default "2G" .ServiceMemoryLimit}}` and `job_variables.ServiceMemoryLimit` must be kept in sync or the `default` fallback creates confusion (variable wins, but inconsistency is misleading).
- `Service` struct in `internal/types/service.go` only passes `Command` to the Kubernetes container — `Args`, `Env`, `WorkingDirectory` are parsed but NOT passed through in `ToContainer`.
- **Service containers do NOT automatically inherit the main container's volumes.** Always add `volumes:` to any service that needs to share a filesystem with the main container.

---

## Current Open Work (as of 2026-09-24)

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

### Completed — ai-skill / ai-parallel-test Fan-Out Fixes (2026-09-24)

Two bugs fixed in the fan-out workflow:

**Bug 1 — Wrong tracker for `--repo` GitHub URL (`scripts/mq/_shared.py`)**
- Root cause: `DEFAULT_TRACKER=jira` caused `resolve_tracker()` to return "bitbucket",
  so `clone_by_tracker` cloned a Bitbucket org repo instead of the GitHub URL given.
- Fix: `resolve_tracker(config, repo_url="")` — URL domain overrides config value.
  Pass `repo_url=repo` in `clone_pr.py:main()`.
- Tests: `tests/test_mq_shared.py::TestResolveTracker` (6 URL override cases)
          `tests/test_mq_clone_pr.py::TestTrackerOverrideFromRepoUrl`

**Bug 2 — `extractFlags` stripping skill args before `RAW_ARGS` (`queen/slack/service.go`)**
- Root cause: `extractFlags()` (added in `1bbf2aa`) consumed ALL `--key value` pairs from
  any route's trailing text, including `--head`, `--base`, `--branch` needed by `run_skill.py`.
  These flags were extracted into job params (unknown keys → discarded) before `RawArgs` was set.
- Fix: `pass_through_args: true` on the `ai-skill` route; `service.go` skips `extractFlags`
  when `result.PassThroughArgs` is true, binding full trailing text to `IdVar` verbatim.
- Tests: `queen/slack/command_router_test.go::Test_PassThroughArgs_Propagated_From_Route_Config`
          `queen/slack/command_router_test.go::Test_PassThroughArgs_False_By_Default`

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

Generic skill invocation job. Slack: `skill <name> [--repo] [--branch] [--tracker github|jira] [--model <id>] [--service <image>] [-- instructions]`

**Task pipeline:** `run` → `post` → `done` (on failure: `notify-error`)

**Key files:**
- `formicary/docs/examples/ai-skill.yaml` — Job definition with service sidecar support
- `ai-dev-tools/scripts/skill/run_skill.py` — Parses RAW_ARGS, clones repo, runs Claude, writes reports
- `ai-dev-tools/scripts/skill/flags.py` — Flag parser (`--service`, `--service-port`, `--service-cmd`, `--service-args`)
- `ai-dev-tools/scripts/skill/post.py` — Post task: reads `skill_result.json` + `reports/report.md`, posts HTML report to Slack
- `ai-dev-tools/.claude/skills/integ-tests/SKILL.md` — Integration test skill (service health check + test runner discovery)
- `formicary/docs/examples/slack-routes.json` — Route entry has `"pass_through_args": true`

**`pass_through_args: true` is MANDATORY on this route.** `run_skill.py` owns its own flag
parser via `scripts/skill/flags.py`. All args (including `--head`, `--base`, `--branch`,
`--repo`, custom flags) must reach `RAW_ARGS` verbatim. If this is removed, skill-specific
flags will be consumed by formicary and lost before `run_skill.py` sees them.

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
