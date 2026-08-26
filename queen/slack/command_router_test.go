// SPDX-License-Identifier: AGPL-3.0-or-later

package slack

import (
	"testing"

	"github.com/stretchr/testify/require"

	"plexobject.com/formicary/queen/config"
)

func testRoutes() []config.SlackRouteConfig {
	return []config.SlackRouteConfig{
		{Triggers: []string{"standup", "status"}, JobType: "ai-standup-jira", Description: "Daily standup"},
		{Triggers: []string{"adhoc"}, JobType: "ai-adhoc", IdVar: "Prompt", Description: "Ad-hoc task"},
		{
			Triggers:    []string{"implement"},
			JobType:     "ai-jira-implement",
			IdVar:       "IssueNumber",
			Description: "Implement issue",
			TrackerVariants: map[string]string{
				"github": "ai-gh-implement",
				"jira":   "ai-jira-implement",
			},
		},
		{
			Triggers:    []string{"review"},
			JobType:     "ai-jira-review",
			IdVar:       "PRUrl",
			Description: "Review PR",
			TrackerVariants: map[string]string{
				"github": "ai-gh-review",
				"jira":   "ai-jira-review",
			},
		},
		{
			Triggers:    []string{"risk"},
			JobType:     "ai-adhoc",
			Description: "Risk scan",
			Params:      map[string]string{"Skill": "ygs-risk-scan"},
		},
	}
}

func Test_Should_Route_Standup_Verb(t *testing.T) {
	// GIVEN a router with standard routes
	router := NewCommandRouter(testRoutes())

	// WHEN routing "standup"
	result, isBuiltin, err := router.Route("standup")

	// THEN it maps to ai-standup-jira
	require.NoError(t, err)
	require.False(t, isBuiltin)
	require.Equal(t, "ai-standup-jira", result.JobType)
	require.Equal(t, "", result.Trailing)
}

func Test_Should_Route_Status_Alias(t *testing.T) {
	// GIVEN a router with standard routes
	router := NewCommandRouter(testRoutes())

	// WHEN routing "status" (alias for standup)
	result, isBuiltin, err := router.Route("status")

	// THEN it maps to ai-standup-jira
	require.NoError(t, err)
	require.False(t, isBuiltin)
	require.Equal(t, "ai-standup-jira", result.JobType)
	require.Equal(t, "", result.Trailing)
}

func Test_Should_Route_Adhoc_With_Trailing_Prompt(t *testing.T) {
	// GIVEN a router with standard routes
	router := NewCommandRouter(testRoutes())

	// WHEN routing "adhoc fix the login bug"
	result, isBuiltin, err := router.Route("adhoc fix the login bug")

	// THEN jobType is ai-adhoc, trailing is the prompt, IdVar is Prompt
	require.NoError(t, err)
	require.False(t, isBuiltin)
	require.Equal(t, "ai-adhoc", result.JobType)
	require.Equal(t, "fix the login bug", result.Trailing)
	require.Equal(t, "Prompt", result.IdVar)
}

func Test_Should_Route_Implement_With_Issue_ID(t *testing.T) {
	// GIVEN a router with standard routes
	router := NewCommandRouter(testRoutes())

	// WHEN routing "implement AB-42"
	result, isBuiltin, err := router.Route("implement AB-42")

	// THEN jobType is ai-jira-implement, trailing is the issue ID, IdVar is IssueNumber
	require.NoError(t, err)
	require.False(t, isBuiltin)
	require.Equal(t, "ai-jira-implement", result.JobType)
	require.Equal(t, "AB-42", result.Trailing)
	require.Equal(t, "IssueNumber", result.IdVar)
}

func Test_Should_Route_Fixed_Params_Passed_Through(t *testing.T) {
	// GIVEN a router where a route has fixed Params
	router := NewCommandRouter(testRoutes())

	// WHEN routing "risk"
	result, isBuiltin, err := router.Route("risk")

	// THEN fixed Params are present on the result unchanged
	require.NoError(t, err)
	require.False(t, isBuiltin)
	require.Equal(t, "ai-adhoc", result.JobType)
	require.Equal(t, "ygs-risk-scan", result.Params["Skill"])
}

func Test_Should_Route_Unknown_Returns_Error(t *testing.T) {
	// GIVEN a router with standard routes
	router := NewCommandRouter(testRoutes())

	// WHEN routing an unknown command
	_, isBuiltin, err := router.Route("xyzzy")

	// THEN ErrUnknownCommand is returned
	require.ErrorIs(t, err, ErrUnknownCommand)
	require.False(t, isBuiltin)
}

func Test_Should_Route_Empty_Returns_Error(t *testing.T) {
	// GIVEN a router with standard routes
	router := NewCommandRouter(testRoutes())

	// WHEN routing empty text
	_, _, err := router.Route("")

	// THEN ErrUnknownCommand is returned
	require.ErrorIs(t, err, ErrUnknownCommand)
}

func Test_Should_Identify_Setup_As_Builtin(t *testing.T) {
	// GIVEN a router with standard routes
	router := NewCommandRouter(testRoutes())

	// WHEN routing "setup"
	result, isBuiltin, err := router.Route("setup")

	// THEN isBuiltin is true and no error
	require.NoError(t, err)
	require.True(t, isBuiltin)
	require.Nil(t, result)
}

func Test_Should_Identify_Help_As_Builtin(t *testing.T) {
	// GIVEN a router with standard routes
	router := NewCommandRouter(testRoutes())

	// WHEN routing "help"
	_, isBuiltin, err := router.Route("help")

	// THEN isBuiltin is true
	require.NoError(t, err)
	require.True(t, isBuiltin)
}

func Test_Should_Route_Case_Insensitive(t *testing.T) {
	// GIVEN a router with standard routes
	router := NewCommandRouter(testRoutes())

	// WHEN routing with different casing
	r1, _, _ := router.Route("STANDUP")
	r2, _, _ := router.Route("Standup")
	r3, _, _ := router.Route("standup")

	// THEN all map to the same job type
	require.Equal(t, "ai-standup-jira", r1.JobType)
	require.Equal(t, "ai-standup-jira", r2.JobType)
	require.Equal(t, "ai-standup-jira", r3.JobType)
}

// DetectTracker tests

func Test_DetectTracker_Github_URL(t *testing.T) {
	require.Equal(t, "github", DetectTracker("implement https://github.com/bhatti/todo-sample/issues/2"))
}

func Test_DetectTracker_Github_Keyword(t *testing.T) {
	require.Equal(t, "github", DetectTracker("implement github issue #5"))
}

func Test_DetectTracker_GH_Abbreviation(t *testing.T) {
	require.Equal(t, "github", DetectTracker("fix gh #42"))
}

func Test_DetectTracker_Jira_URL(t *testing.T) {
	require.Equal(t, "jira", DetectTracker("implement https://myorg.atlassian.net/browse/PROJ-123"))
}

func Test_DetectTracker_Bitbucket_URL(t *testing.T) {
	require.Equal(t, "jira", DetectTracker("review https://bitbucket.org/myteam/myrepo/pull-requests/7"))
}

func Test_DetectTracker_Jira_Keyword(t *testing.T) {
	require.Equal(t, "jira", DetectTracker("implement jira issue ABC-42"))
}

func Test_DetectTracker_Empty(t *testing.T) {
	require.Equal(t, "", DetectTracker("implement ABC-42"))
}

func Test_DetectTracker_Github_Takes_Precedence(t *testing.T) {
	// when both appear, github wins
	require.Equal(t, "github", DetectTracker("implement github.com/repo/issues/1 jira"))
}

func Test_DetectTracker_GH_Not_Matched_As_Substring(t *testing.T) {
	// "gh" inside another word must not match
	require.Equal(t, "", DetectTracker("implement nightly build"))
	require.Equal(t, "", DetectTracker("implement highlight issue"))
}

// RouteResult.ResolveJobType tests

func Test_ResolveJobType_Uses_Variant_When_Tracker_Matches(t *testing.T) {
	result := &RouteResult{
		JobType:         "ai-jira-implement",
		TrackerVariants: map[string]string{"github": "ai-gh-implement", "jira": "ai-jira-implement"},
	}
	require.Equal(t, "ai-gh-implement", result.ResolveJobType("github"))
	require.Equal(t, "ai-jira-implement", result.ResolveJobType("jira"))
}

func Test_ResolveJobType_Falls_Back_To_Default_When_No_Match(t *testing.T) {
	result := &RouteResult{
		JobType:         "ai-jira-implement",
		TrackerVariants: map[string]string{"github": "ai-gh-implement"},
	}
	// tracker "linear" has no variant — use JobType
	require.Equal(t, "ai-jira-implement", result.ResolveJobType("linear"))
}

func Test_ResolveJobType_Falls_Back_When_Tracker_Empty(t *testing.T) {
	result := &RouteResult{
		JobType:         "ai-jira-implement",
		TrackerVariants: map[string]string{"github": "ai-gh-implement"},
	}
	require.Equal(t, "ai-jira-implement", result.ResolveJobType(""))
}

func Test_ResolveJobType_Falls_Back_When_No_Variants(t *testing.T) {
	result := &RouteResult{JobType: "ai-standup-jira"}
	require.Equal(t, "ai-standup-jira", result.ResolveJobType("github"))
}

func Test_ResolveJobType_Falls_Back_When_Variant_Value_Is_Empty(t *testing.T) {
	// A misconfigured route with a blank variant value must fall back to JobType,
	// not return "" and cause a downstream validation error.
	result := &RouteResult{
		JobType:         "ai-jira-implement",
		TrackerVariants: map[string]string{"github": ""},
	}
	require.Equal(t, "ai-jira-implement", result.ResolveJobType("github"))
}

// End-to-end: route + tracker resolution

func Test_Should_Route_Github_URL_To_GH_Implement(t *testing.T) {
	// GIVEN a router with tracker_variants on the implement route
	router := NewCommandRouter(testRoutes())

	// WHEN routing a GitHub URL
	result, _, err := router.Route("implement https://github.com/bhatti/todo-sample/issues/2")
	require.NoError(t, err)

	// THEN the base job type is jira (the default)
	require.Equal(t, "ai-jira-implement", result.JobType)

	// AND resolving with the detected tracker gives the github variant
	tracker := DetectTracker("implement https://github.com/bhatti/todo-sample/issues/2")
	require.Equal(t, "github", tracker)
	require.Equal(t, "ai-gh-implement", result.ResolveJobType(tracker))
}

func Test_Should_Route_Jira_Key_To_Jira_Implement(t *testing.T) {
	router := NewCommandRouter(testRoutes())
	result, _, err := router.Route("implement ABC-42")
	require.NoError(t, err)

	tracker := DetectTracker("implement ABC-42")
	require.Equal(t, "", tracker)
	// no tracker detected, no org default → falls back to JobType
	require.Equal(t, "ai-jira-implement", result.ResolveJobType(tracker))
}

func Test_Should_Route_Github_PR_To_GH_Review(t *testing.T) {
	router := NewCommandRouter(testRoutes())
	result, _, err := router.Route("review https://github.com/org/repo/pull/99")
	require.NoError(t, err)

	tracker := DetectTracker("review https://github.com/org/repo/pull/99")
	require.Equal(t, "github", tracker)
	require.Equal(t, "ai-gh-review", result.ResolveJobType(tracker))
}

func Test_Should_Route_Org_Default_Tracker_Applied(t *testing.T) {
	// GIVEN no tracker signal in text, org default is "github"
	router := NewCommandRouter(testRoutes())
	result, _, err := router.Route("implement PROJ-5")
	require.NoError(t, err)

	// WHEN text has no tracker but org says "github"
	tracker := DetectTracker("implement PROJ-5")
	require.Equal(t, "", tracker) // no signal in text
	// caller (dispatch) would fall back to defaultTracker → "github"
	require.Equal(t, "ai-gh-implement", result.ResolveJobType("github"))
}

// "@bot setup <code>" — "setup" is a builtin verb so the router classifies it
// as builtin regardless of trailing text. dispatch() further intercepts
// "setup <code>" to do the code exchange before route lookup.
func Test_Should_Detect_Setup_With_Code_As_Builtin(t *testing.T) {
	router := NewCommandRouter(testRoutes())
	result, isBuiltin, err := router.Route("setup abc123")

	// "setup" is in builtinVerbs — must be classified as builtin, not a job route.
	require.NoError(t, err)
	require.True(t, isBuiltin)
	require.Nil(t, result)
}

// ResolveJobType must be consistent across all callers — verifies that the resolved
// job type is the same value that should be used for UNIQUE constraint lookups.
func Test_ResolveJobType_Consistent_For_Constraint_Lookup(t *testing.T) {
	result := &RouteResult{
		JobType:         "ai-jira-implement",
		TrackerVariants: map[string]string{"github": "ai-gh-implement", "jira": "ai-jira-implement"},
	}
	// Simulate what dispatch() does: resolve once, use everywhere.
	resolved := result.ResolveJobType("github")
	require.Equal(t, "ai-gh-implement", resolved)
	// The resolved type must NOT equal the raw default — callers that used
	// result.JobType directly (before this fix) would look up the wrong job.
	require.NotEqual(t, result.JobType, resolved)
}
