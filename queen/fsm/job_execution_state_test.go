package fsm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
)

func Test_ShouldValidateJobState(t *testing.T) {
	// GIVEN job state machine
	jsm, err := NewTestJobStateMachine()
	require.NoError(t, err)
	jsm.Reservations = make(map[string]*common.AntReservation)
	// WHEN validating
	err = jsm.Validate()
	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldPrepareJobState(t *testing.T) {
	// GIVEN job state machine
	jsm, err := NewTestJobStateMachine()
	require.NoError(t, err)
	// WHEN pareparing launch
	err = jsm.PrepareLaunch(jsm.Request.GetJobExecutionID())
	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldNotCreateJobExecution(t *testing.T) {
	// GIVEN job state machine
	jsm, err := NewTestJobStateMachine()
	require.NoError(t, err)
	// WHEN creating job execution with existing record
	err1, err2 := jsm.CreateJobExecution(context.Background())
	require.NoError(t, err2)
	// THEN it should fail
	require.Error(t, err1)
	require.Equal(t, err1.Error(), "job-execution already exists")
}

// Test_ShouldLoadOrgConfigsWhenUserIsNil reproduces the bug where cron jobs triggered
// without an authenticated user (auth disabled) never loaded org configs into the
// template variables, causing {{.GitHubOrg}} and {{.GitHubRepo}} to render as empty
// strings — resulting in `gh issue list -R "/"` and "unexpected format, got '/'" error.
//
// The fix in buildDynamicConfigs falls back to loading org configs directly from the
// repository using the org ID "default" when jsm.User == nil.
func Test_ShouldLoadOrgConfigsWhenUserIsNil(t *testing.T) {
	// GIVEN a job state machine created with a real user (so we can save org configs)
	jsm, err := NewTestJobStateMachine()
	require.NoError(t, err)

	// Save org configs under the "default" org (as the API does when auth is disabled)
	qc := common.NewQueryContextFromIDs("", "default")
	ghOrgCfg, err := common.NewOrganizationConfig("default", "GitHubOrg", "myorg", false)
	require.NoError(t, err)
	_, err = jsm.userManager.SaveOrgConfig(qc, ghOrgCfg)
	require.NoError(t, err)
	ghRepoCfg, err := common.NewOrganizationConfig("default", "GitHubRepo", "myrepo", false)
	require.NoError(t, err)
	_, err = jsm.userManager.SaveOrgConfig(qc, ghRepoCfg)
	require.NoError(t, err)

	// Simulate auth-disabled: clear the user so buildDynamicConfigs takes the fallback path.
	// buildDynamicConfigs is the inner function — test it directly to avoid needing
	// a fully-wired JobDefinition and Request (those are tested by other tests).
	jsm.User = nil

	// WHEN building the config portion of dynamic params
	configs := jsm.buildDynamicConfigs()

	// THEN org configs must be present
	require.Contains(t, configs, "GitHubOrg", "GitHubOrg org config must appear when User is nil")
	require.Contains(t, configs, "GitHubRepo", "GitHubRepo org config must appear when User is nil")
	require.Equal(t, "myorg", configs["GitHubOrg"].Value)
	require.Equal(t, "myrepo", configs["GitHubRepo"].Value)
}
