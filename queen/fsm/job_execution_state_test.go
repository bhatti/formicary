package fsm

import (
	"context"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
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
