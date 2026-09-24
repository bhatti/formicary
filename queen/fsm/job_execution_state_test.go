package fsm

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	qtypes "plexobject.com/formicary/queen/types"
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

// Test_ShouldPauseJobFromExecutingState verifies that PauseJob() correctly transitions
// a job from EXECUTING → PAUSED, increments PausedCount and Retried, and persists the
// state. This covers the state-machine half of the exit-code-3 → PAUSE_JOB flow.
func Test_ShouldPauseJobFromExecutingState(t *testing.T) {
	// GIVEN a job state machine in EXECUTING state
	jsm, err := NewTestJobStateMachine()
	require.NoError(t, err)
	err = jsm.PrepareLaunch(jsm.JobExecution.ID)
	require.NoError(t, err)
	err = jsm.SetJobStatusToExecuting(context.Background())
	require.NoError(t, err)
	require.Equal(t, common.EXECUTING, jsm.JobExecution.JobState)

	retriedBefore := jsm.Request.GetRetried()
	pausedBefore := jsm.Request.GetPausedCount()

	// WHEN pausing the job
	err = jsm.PauseJob()

	// THEN no error, job state is PAUSED, counters incremented
	require.NoError(t, err)
	require.Equal(t, common.PAUSED, jsm.JobExecution.JobState)
	require.Equal(t, common.PAUSED, jsm.Request.GetJobState())
	require.Equal(t, retriedBefore+1, jsm.Request.GetRetried())
	require.Equal(t, pausedBefore+1, jsm.Request.GetPausedCount())
}

// Test_ShouldNotPauseJobWhenNotExecuting verifies PauseJob() rejects an invalid
// state transition so we never accidentally double-pause or pause a finished job.
func Test_ShouldNotPauseJobWhenNotExecuting(t *testing.T) {
	// GIVEN a job state machine NOT in EXECUTING state (still PENDING after setup)
	jsm, err := NewTestJobStateMachine()
	require.NoError(t, err)
	// JobExecution.JobState starts as PENDING, not EXECUTING
	require.NotEqual(t, common.EXECUTING, jsm.JobExecution.JobState)

	// WHEN pausing the job
	err = jsm.PauseJob()

	// THEN it should reject the transition
	require.Error(t, err)
	require.Contains(t, err.Error(), "not executing")
}

// Test_ShouldSetTaskStateTosPausedOnExitCodeMapping verifies that
// OverrideStatusAndErrorCode correctly maps a numeric exit code to PAUSED state
// when on_exit_code maps that code to PAUSE_JOB — this is the task-definition-level
// contract that the supervisor relies on.
func Test_ShouldSetTaskStateToPausedOnExitCodeMapping(t *testing.T) {
	// GIVEN a task definition with on_exit_code: {"3": "PAUSE_JOB"}
	td := qtypes.NewTaskDefinition("poll-pr", common.Kubernetes)
	td.OnExitCode[common.NewRequestState("3")] = string(common.PAUSE_JOB)

	// WHEN overriding status for exit code "3"
	status, errorCode := td.OverrideStatusAndErrorCode("3")

	// THEN status must be PAUSED and error code must be ErrorPauseJob
	require.Equal(t, common.PAUSED, status)
	require.Equal(t, common.ErrorPauseJob, errorCode)
}

// Test_ShouldGetNilNextTaskForPauseJobExitCode verifies that GetNextTask returns nil
// (no next task to dispatch) when exit code maps to PAUSE_JOB — the sentinel that
// tells executeNextTask to pause the job rather than continue execution.
func Test_ShouldGetNilNextTaskForPauseJobExitCode(t *testing.T) {
	// GIVEN a task definition with on_exit_code: {"3": "PAUSE_JOB"}
	// (used directly without a full state machine to keep this test lightweight)
	task := qtypes.NewTaskDefinition("poll-pr", common.Kubernetes)
	task.OnExitCode[common.NewRequestState("3")] = string(common.PAUSE_JOB)

	jd := qtypes.NewJobDefinition("io.formicary.test.pause-test")
	jd.AddTask(task)

	// WHEN computing the next task after exit code "3" with PAUSED task state
	nextTask, _, err := jd.GetNextTask(task, common.PAUSED, "3")

	// THEN no next task (PAUSE_JOB is a sentinel action, not a task name)
	require.NoError(t, err)
	require.Nil(t, nextTask)
}

// Test_ShouldLoadOrgConfigsWhenUserIsNil verifies that cron/system jobs (jsm.User == nil)
// load org configs from the DB using the org ID stored in the job request.
// There is no "default" org fallback — configs must be stored under the real org ID.
func Test_ShouldLoadOrgConfigsWhenUserIsNil(t *testing.T) {
	// GIVEN a job state machine with a specific org ID on the request
	testOrgID := "test-org-" + ulid.Make().String()
	jsm, err := NewTestJobStateMachineWithOrgID(testOrgID)
	require.NoError(t, err)

	qc := common.NewQueryContextFromIDs("", testOrgID)
	ghOrgCfg, err := common.NewOrgConfig(testOrgID, "GitHubOrg", "myorg", false)
	require.NoError(t, err)
	_, err = jsm.userManager.SaveConfig(qc, ghOrgCfg)
	require.NoError(t, err)
	ghRepoCfg, err := common.NewOrgConfig(testOrgID, "GitHubRepo", "myrepo", false)
	require.NoError(t, err)
	_, err = jsm.userManager.SaveConfig(qc, ghRepoCfg)
	require.NoError(t, err)

	// Simulate a cron/system job: no authenticated user, org ID carried on the request.
	jsm.User = nil

	// WHEN building the config portion of dynamic params
	configs := jsm.buildDynamicConfigs()

	// THEN org configs must be present
	require.Contains(t, configs, "GitHubOrg", "GitHubOrg org config must appear when User is nil")
	require.Contains(t, configs, "GitHubRepo", "GitHubRepo org config must appear when User is nil")
	require.Equal(t, "myorg", configs["GitHubOrg"].Value)
	require.Equal(t, "myrepo", configs["GitHubRepo"].Value)
}

// Test_ShouldSkipAntCheckForInternalOnlyJob verifies that jobs whose tasks use only
// queen-internal methods (FAN_OUT_JOB, FORK_JOB, etc.) pass CheckAntResourcesAndConcurrencyForJob
// without requiring any registered ant. This is the root cause of the parallel-test failure
// where FAN_OUT_JOB was incorrectly sent to HasAntsForJobTags.
func Test_ShouldSkipAntCheckForInternalOnlyJob(t *testing.T) {
	// GIVEN a job state machine with JobDefinition loaded
	jsm, err := NewTestJobStateMachine()
	require.NoError(t, err)
	err = jsm.PrepareLaunch(jsm.JobExecution.ID)
	require.NoError(t, err)

	// AND a job definition whose only method is FAN_OUT_JOB (internal)
	jsm.JobDefinition.Methods = string(common.FanOutJob)

	// WHEN checking ant resources
	err = jsm.CheckAntResourcesAndConcurrencyForJob()

	// THEN it should succeed — no ant is needed for internal methods
	require.NoError(t, err)
}

// Test_ShouldSkipAntCheckForMixedInternalAndExternal verifies that when a job mixes
// internal (FAN_OUT_JOB) and external (KUBERNETES) methods, only the external ones
// are passed to HasAntsForJobTags.
func Test_ShouldSkipAntCheckForMixedInternalAndExternal(t *testing.T) {
	// GIVEN a job state machine with a registered KUBERNETES ant (stub already has one)
	jsm, err := NewTestJobStateMachine()
	require.NoError(t, err)
	err = jsm.PrepareLaunch(jsm.JobExecution.ID)
	require.NoError(t, err)

	// AND a job that has both FAN_OUT_JOB (internal) and KUBERNETES (external) methods
	jsm.JobDefinition.Methods = string(common.FanOutJob) + ", " + string(common.Kubernetes)

	// WHEN checking ant resources
	err = jsm.CheckAntResourcesAndConcurrencyForJob()

	// THEN it should succeed — KUBERNETES ant is registered in the stub, FAN_OUT_JOB is skipped
	require.NoError(t, err)
}

// Test_BuildJobAPIToken_EmptyWhenNoSecret verifies that buildJobAPIToken returns ""
// when JWTSecret is not configured (the default in test configs — auth disabled).
// This is the safe-fallback path: no token injected means tasks won't have JobAPIToken.
func Test_BuildJobAPIToken_EmptyWhenNoSecret(t *testing.T) {
	jsm, err := NewTestJobStateMachine()
	require.NoError(t, err)
	// TestServerConfig has Auth.JWTSecret == "" (auth disabled in tests).
	require.Empty(t, jsm.serverCfg.Common.Auth.JWTSecret,
		"precondition: test config must not have a JWTSecret")

	token := jsm.buildJobAPIToken()
	require.Empty(t, token, "expected empty token when JWTSecret is not set")
}

// parseJobAPIToken parses a JWT string signed with the given secret and returns its claims.
func parseJobAPIToken(t *testing.T, tokenStr, secret string) *web.JwtClaims {
	t.Helper()
	parsed, err := jwt.ParseWithClaims(tokenStr, &web.JwtClaims{}, func(_ *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	require.NoError(t, err, "JWT must be parseable with the known secret")
	claims, ok := parsed.Claims.(*web.JwtClaims)
	require.True(t, ok, "claims must be *web.JwtClaims")
	return claims
}

// Test_BuildJobAPIToken_InjectedAsSecretVar verifies the full happy path:
// when JWTSecret is set, buildJobAPIToken returns a non-empty JWT, injects it
// as a secret variable, and the TTL clamp is enforced at both the 4h floor and
// the 24h ceiling.
func Test_BuildJobAPIToken_InjectedAsSecretVar(t *testing.T) {
	const jwtSecret = "test-jwt-secret-32-bytes-padding!!"

	setup := func(t *testing.T) *JobExecutionStateMachine {
		t.Helper()
		jsm, err := NewTestJobStateMachine()
		require.NoError(t, err)
		require.NoError(t, jsm.PrepareLaunch(jsm.JobExecution.ID))
		jsm.serverCfg.Common.Auth.JWTSecret = jwtSecret
		return jsm
	}

	t.Run("4h floor: timeout*retries < 4h is clamped up", func(t *testing.T) {
		jsm := setup(t)
		jsm.JobDefinition.Timeout = 30 * time.Minute
		jsm.JobDefinition.Retry = 1 // 30min * 2 = 1h < 4h → clamp to 4h

		token := jsm.buildJobAPIToken()
		require.NotEmpty(t, token)

		claims := parseJobAPIToken(t, token, jwtSecret)
		require.WithinDuration(t, time.Now().Add(4*time.Hour), claims.ExpiresAt.Time, 2*time.Minute,
			"1h computed TTL must be clamped up to 4h minimum")
	})

	t.Run("24h ceiling: timeout*retries > 24h is clamped down", func(t *testing.T) {
		jsm := setup(t)
		jsm.JobDefinition.Timeout = 24 * time.Hour
		jsm.JobDefinition.Retry = 10 // 24h * 11 = 264h → clamp to 24h

		token := jsm.buildJobAPIToken()
		require.NotEmpty(t, token)

		claims := parseJobAPIToken(t, token, jwtSecret)
		require.WithinDuration(t, time.Now().Add(24*time.Hour), claims.ExpiresAt.Time, 2*time.Minute,
			"264h computed TTL must be clamped down to 24h maximum")
	})

	t.Run("nominal: timeout*retries within [4h,24h] is unchanged", func(t *testing.T) {
		jsm := setup(t)
		jsm.JobDefinition.Timeout = 2 * time.Hour
		jsm.JobDefinition.Retry = 2 // 2h * 3 = 6h — in range

		token := jsm.buildJobAPIToken()
		require.NotEmpty(t, token)

		claims := parseJobAPIToken(t, token, jwtSecret)
		require.WithinDuration(t, time.Now().Add(6*time.Hour), claims.ExpiresAt.Time, 2*time.Minute,
			"6h computed TTL must be kept as-is")
	})

	t.Run("injected as secret in buildDynamicParams", func(t *testing.T) {
		jsm := setup(t)
		jsm.JobDefinition.Timeout = 30 * time.Minute
		jsm.JobDefinition.Retry = 1

		params := jsm.buildDynamicParams(nil)
		v, ok := params["JobAPIToken"]
		require.True(t, ok, "JobAPIToken must be present in dynamic params")
		require.True(t, v.Secret, "JobAPIToken must be marked as secret so it is never logged")
		require.NotEmpty(t, v.Value)
	})
}

