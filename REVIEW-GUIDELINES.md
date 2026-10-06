# Formicary / ai-dev-tools / you-got-skills — PR Review Checklist

Review all changes as a principal engineer/architect who must approve for production. Every item is a MUST.

## 1. Three-Repo Alignment & Design

- MUST follow the edit order: `you-got-skills → ai-dev-tools → formicary` — skill protocol first, then scripts, then job YAMLs
- MUST align with existing design, conventions, and implementation across all three repos — review existing code before changing anything
- MUST review existing abstractions and functions before creating new ones — reuse first, no duplicates
- MUST use deep modules not shallow (ref: softengbook.org/articles/deep-modules)
- MUST ask: why is this change needed? Is there a simpler way? Are we changing someone else's code? Does the change make sense for that code or is it fixing a problem higher in the stack?
- MUST encapsulate internal logic with clean, minimal interfaces — principle of least surprise
- MUST not create circular imports
- MUST keep code/scripts/docs/skills DRY, concise, and modular across all three repos

## 2. Tracker & Routing Correctness

- MUST follow tracker resolution priority: `--tracker` flag > PR URL domain > `DEFAULT_TRACKER` from YAML
- MUST call `_resolve_effective_tracker()` (or `resolve_tracker()`) BEFORE any code that branches on tracker
- MUST NEVER use `{{.DefaultTracker}}` in tracker-specific YAMLs — hardcode `DEFAULT_TRACKER: "github"` or `"jira"` per variant
- MUST NEVER put Jira env vars in GH YAMLs or vice versa
- MUST set `pass_through_args: true` on routes where the target script owns its own flag parser (e.g., `ai-skill`)
- MUST verify Slack routing is consistent for both GitHub and Bitbucket/Jira paths
- MUST pass `repo_url` to `resolve_tracker()` when a `--repo` flag is present

## 3. YAML Job Definitions & Artifact Flow

- MUST list all output files in `artifacts.paths:` — formicary only collects what's listed
- MUST declare all data-producing tasks in `dependencies:` (artifacts are NOT transitive)
- MUST include `FORMICARY_PUBLIC_URL` and `JOB_ID` env vars in report tasks for Slack artifact links
- MUST ensure `filename` in `post_report()` matches the actual basename written to `reports/`
- MUST keep `{{default ...}}` fallbacks and `job_variables` defaults in sync — inconsistency is misleading
- MUST add the `touch /tmp/.adt_bootstrap_done` line AFTER the bootstrap line in every task's `script:` (os.execv kills inline code)
- MUST add `volumes:` to any service that needs to share filesystem with the main container — services do NOT inherit main container volumes
- MUST remember: `args:` and `env:` on services are silently ignored by formicary k8s executor — only `Command` is passed through

## 4. Python Script Quality (ai-dev-tools)

- MUST `sys.exit(1)` on meaningful failures — silent exit 0 hides failures from formicary
- MUST use `_load_skill_md()` + `_inline_shared_refs()` to embed skills in Claude prompts — NEVER invoke skills via the Skill tool in scripts
- MUST gate all debug logs; no string interpolation in log messages
- MUST not change log/error messages consumers may depend on
- MUST not touch code or comments unrelated to the change
- MUST remove all dead code, unused variables, backward-compatibility shims
- MUST use concise comments — no filler; design speaks for itself

## 5. Go Code Quality (formicary)

- MUST run `make build && make test` before any deploy
- MUST verify PUT API responses with GET — formicary PUT may return 200 but not persist
- MUST not break existing Slack route parsing or `extractFlags` behavior
- MUST keep `Service` struct changes backward-compatible — `ToContainer` only passes `Command`

## 6. Skills Quality (you-got-skills)

- MUST NEVER edit `~/.claude/skills/` — always edit `/Users/sbhatti/workplace/you-got-skills/` and push
- MUST keep `shared/*.md` refs resolvable — `_inline_shared_refs()` depends on exact paths
- MUST keep infra skills (e.g., `integ-tests`) in `ai-dev-tools/.claude/skills/`, not you-got-skills
- MUST ensure skill changes work with both GH and Jira/BB tracker variants

## 7. Security & Privacy

- MUST review all multi-tenant isolation changes for correctness
- MUST not include ANY private info — no internal project names, repos, or IPs in code, docs, or skills
- MUST verify no secrets or private references leak into any artifact, report, or Slack post
- MUST not expose credentials in log output, error messages, or artifact files

## 8. Testing — Full Pyramid, No Shortcuts

- MUST follow the testing pyramid in order: unit → pod functional → e2e — NEVER skip levels
- MUST verify all changes by running tests — full regression, data-driven assertions only, no hunches
- MUST not break existing tests — use stash to validate assumptions before changing
- MUST test real behavior via actual method calls and side effects — minimize mocks
- MUST keep tests simple, robust, not brittle/flaky — no timing/sleep-based assertions, no concurrency races
- MUST test interfaces not internals; same code paths as production — no test-only flags/params/DI
- MUST not use grep to validate test correctness
- MUST ensure pod test manifests mirror formicary YAML volume/env configuration exactly — false greens from divergent pod specs are the most dangerous bugs

## 9. Performance, Scalability & Resource Budgets

- MUST evaluate at scale — millions of requests, large data sizes; no memory bloat or CPU waste
- MUST review container memory limits: main container vs service sidecars, JVM startup peaks vs steady state
- MUST consider blast radius, backward compatibility, cost of change
- MUST simplify implementation to scale easily — could this be done simpler?
- MUST not waste node resources (e.g., 2G main container when 512Mi suffices for pure HTTP)
- MUST review concurrency settings — check `max_concurrency` defaults won't serialize jobs
- MUST think about partial failure: what happens when one task in a multi-task pipeline fails? Does cleanup happen? Are artifacts left in a consistent state?

## 10. Documentation & Consistency

- MUST update docs, READMEs, and examples in every affected repo
- MUST update PROJECT_CONTEXT.md when adding new workflows, fixing bugs, or discovering gotchas
- MUST keep all three repos' documentation consistent — a YAML change that adds a new env var must be reflected in the script that reads it and the skill that depends on it
- MUST document new scripts with usage examples

## 11. Review Discipline

- MUST provide feedback with specific code references, file paths, and line numbers
- MUST explain why each fix is needed
- MUST double-check all feedback — no hallucinations, no false positives
- MUST verify feedback against both the diff AND the existing implementation
- MUST use available review skills (`/ygs-code-review`) for structured checklist coverage
