package supervisor

import (
	"context"
	evbus "github.com/asaskevich/EventBus"
	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/fsm"
	"testing"
	"time"
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
