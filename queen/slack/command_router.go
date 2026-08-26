// SPDX-License-Identifier: AGPL-3.0-or-later

package slack

import (
	"errors"
	"regexp"
	"strings"
	"unicode"

	"plexobject.com/formicary/queen/config"
)

// githubPattern matches the word "github", the abbreviation "gh" as a whole word,
// or a github.com URL anywhere in the text.
var githubPattern = regexp.MustCompile(`(?i)\bgithub\.com\b|\bgithub\b|\bgh\b`)

// jiraPattern matches the word "jira", "bitbucket", or known hosted domain names.
var jiraPattern = regexp.MustCompile(`(?i)\bjira\.com\b|\batlassian\.net\b|\bbitbucket\.org\b|\bjira\b|\bbitbucket\b`)

// DetectTracker returns "github", "jira", or "" based on URL patterns and keywords
// in the text. github signals take precedence over jira signals.
func DetectTracker(text string) string {
	if githubPattern.MatchString(text) {
		return "github"
	}
	if jiraPattern.MatchString(text) {
		return "jira"
	}
	return ""
}

// ErrUnknownCommand is returned when the text does not match any route or builtin.
var ErrUnknownCommand = errors.New("unknown command")

// builtinVerbs are handled directly by the service; they are never submitted as jobs.
var builtinVerbs = map[string]bool{
	"setup":     true,
	"configure": true,
	"help":      true,
}

// CommandRouter maps Slack command text to a Formicary job type.
// It is a pure, deterministic lookup — no AI, no skill invocation.
// Base routes come from the static server config; org configs can extend them
// at deploy time by storing a "SlackRoutes" JSON array via the API.
type CommandRouter struct {
	base   []config.SlackRouteConfig // static routes from server config (never mutated)
	routes []config.SlackRouteConfig // effective routes: base + org overrides
}

// NewCommandRouter constructs a CommandRouter from the static server config routes.
func NewCommandRouter(routes []config.SlackRouteConfig) *CommandRouter {
	r := &CommandRouter{base: routes}
	r.routes = routes
	return r
}

// WithOrgRoutes returns a new CommandRouter that prepends orgRoutes before the base
// static routes. Org routes take precedence — first matching trigger wins.
// The receiver is not mutated; callers should use the returned router for the request.
func (r *CommandRouter) WithOrgRoutes(orgRoutes []config.SlackRouteConfig) *CommandRouter {
	merged := make([]config.SlackRouteConfig, 0, len(orgRoutes)+len(r.base))
	merged = append(merged, orgRoutes...)
	merged = append(merged, r.base...)
	return &CommandRouter{base: r.base, routes: merged}
}

// RouteResult holds the full routing outcome for a matched command.
type RouteResult struct {
	JobType         string
	Trailing        string            // text after the trigger; bound to IdVar when set
	IdVar           string            // job param name that receives Trailing (e.g. PRUrl, IssueNumber, Prompt)
	Params          map[string]string // fixed params merged verbatim from route config
	Description     string            // human-readable label from route config (used as job description)
	TrackerVariants map[string]string // tracker name → job type override (e.g. "github" → "ai-gh-implement")
}

// ResolveJobType returns the job type for the given tracker, falling back to JobType
// when the tracker is empty or has no entry in TrackerVariants.
func (r *RouteResult) ResolveJobType(tracker string) string {
	if tracker != "" && len(r.TrackerVariants) > 0 {
		if jt, ok := r.TrackerVariants[tracker]; ok && jt != "" {
			return jt
		}
	}
	return r.JobType
}

// Route parses text and returns a RouteResult plus flags.
//
//   - result:    non-nil when a route matched (isBuiltin=false, err=nil)
//   - isBuiltin: true for setup/configure/help — handled by service, never queued
//   - error:     ErrUnknownCommand when nothing matches
//
// Matching is case-insensitive prefix; the first matching trigger wins.
func (r *CommandRouter) Route(text string) (result *RouteResult, isBuiltin bool, err error) {
	normalized := normalize(text)
	if normalized == "" {
		return nil, false, ErrUnknownCommand
	}

	// Builtins short-circuit before the route table.
	first := strings.Fields(normalized)
	if len(first) > 0 && builtinVerbs[first[0]] {
		return nil, true, nil
	}

	for _, route := range r.routes {
		for _, trigger := range route.Triggers {
			t := normalize(trigger)
			if t == "" {
				continue
			}
			var trailing string
			if normalized == t {
				trailing = ""
			} else if strings.HasPrefix(normalized, t+" ") {
				trailing = strings.TrimSpace(text[len(t):])
			} else {
				continue
			}
			return &RouteResult{
				JobType:         route.JobType,
				Trailing:        trailing,
				IdVar:           route.IdVar,
				Params:          route.Params,
				Description:     route.Description,
				TrackerVariants: route.TrackerVariants,
			}, false, nil
		}
	}
	return nil, false, ErrUnknownCommand
}

// Routes returns a snapshot of the effective merged route slice (base + org overrides).
func (r *CommandRouter) Routes() []config.SlackRouteConfig {
	out := make([]config.SlackRouteConfig, len(r.routes))
	copy(out, r.routes)
	return out
}

// normalize lowercases and strips leading/trailing punctuation from text.
func normalize(s string) string {
	s = strings.ToLower(strings.TrimFunc(s, func(r rune) bool {
		return unicode.IsPunct(r) || unicode.IsSpace(r)
	}))
	return s
}
