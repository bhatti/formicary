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
