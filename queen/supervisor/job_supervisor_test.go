package supervisor

import (
	"context"
	"testing"
	"time"

	evbus "github.com/asaskevich/EventBus"
	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/fsm"
)

func Test_ShouldNotExecuteJobWithoutWorkers(t *testing.T) {
	// GIVEN job supervisor
	supervisor := newTestJobSupervisor(t)
	// WHEN preparing without worker allocations
	supervisor.jobStateMachine.Reservations = make(map[string]*common.AntReservation)
	err := supervisor.jobStateMachine.PrepareLaunch(supervisor.jobStateMachine.JobExecution.ID)
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected ant allocations")
}

func Test_ShouldExecuteJob(t *testing.T) {
	// GIVEN job supervisor
	supervisor := newTestJobSupervisor(t)
	err := supervisor.jobStateMachine.PrepareLaunch(supervisor.jobStateMachine.JobExecution.ID)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// WHEN calling execute with proper ant workers
	awaiter := supervisor.AsyncExecute(ctx)
	_, err = awaiter.Await(ctx)

	// THEN it should not fail
	require.NoError(t, err)
}

// Test_ShouldPauseJobWhenTaskExitsWithPauseJobCode is the regression test for the bug
// where a task exiting with code 3 (mapped via on_exit_code to PAUSE_JOB) caused the
// job to be marked FAILED instead of PAUSED.
//
// Root cause: UpdateTaskFromResponse intentionally leaves TaskExecution.ErrorCode empty
// for PAUSED tasks, but executeNextTask was returning that empty ErrorCode, which meant
// the errorCode == ErrorPauseJob check in Execute() never fired → ExecutionFailed() ran.
//
// Fix: executeNextTask now returns common.ErrorPauseJob directly when TaskState==PAUSED.
func Test_ShouldPauseJobWhenTaskExitsWithPauseJobCode(t *testing.T) {
	// GIVEN a job whose poll-task has on_exit_code: {3: PAUSE_JOB} in the stored YAML,
	// and a mock ant that returns exit code "3".
	// This tests the regression: previously the job ended FAILED because
	// TaskExecution.ErrorCode was empty for PAUSED tasks and executeNextTask returned
	// that empty code instead of common.ErrorPauseJob.
	jsm, err := fsm.NewTestJobStateMachineForPause()
	require.NoError(t, err)

	err = jsm.PrepareLaunch(jsm.JobExecution.ID)
	require.NoError(t, err)

	supervisor := NewJobSupervisor(config.TestServerConfig(), jsm, evbus.New())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// WHEN executing the job
	awaiter := supervisor.AsyncExecute(ctx)
	_, err = awaiter.Await(ctx)

	// THEN the job must end in PAUSED state (not FAILED) and Execute returns nil
	require.NoError(t, err, "Execute must not return an error when job is paused")
	require.Equal(t, common.PAUSED, jsm.Request.GetJobState(),
		"job request state must be PAUSED, not FAILED")
	require.Equal(t, common.PAUSED, jsm.JobExecution.JobState,
		"job execution state must be PAUSED, not FAILED")
	require.Equal(t, 1, jsm.Request.GetPausedCount(), "PausedCount must be incremented")
}


func newTestJobSupervisor(t *testing.T) *JobSupervisor {
	// Initializing dependent objects
	cfg := config.TestServerConfig()
	jsm, err := fsm.NewTestJobStateMachine()
	require.NoError(t, err)

	return NewJobSupervisor(
		cfg,
		jsm,
		evbus.New())
}
