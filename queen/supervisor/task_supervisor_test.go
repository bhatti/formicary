package supervisor

import (
	"context"
	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/fsm"
	"testing"
	"time"
)

func Test_ShouldNotExecuteTaskWithoutWorkers(t *testing.T) {
	// GIVEN job supervisor
	supervisor := newTestTaskSupervisor(t)

	supervisor.taskStateMachine.Reservations = make(map[string]*common.AntReservation)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// WHEN calling execute without proper ant workers
	err := supervisor.Execute(ctx)

	// THEN it should fail
	require.Error(t, err)
}

func Test_ShouldExecuteTask(t *testing.T) {
	// GIVEN job supervisor
	supervisor := newTestTaskSupervisor(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// WHEN calling execute with proper ant workers
	err := supervisor.taskStateMachine.PrepareLaunch(supervisor.taskStateMachine.Request.GetJobExecutionID())
	require.NoError(t, err)
	err = supervisor.Execute(ctx)

	// THEN it should not fail
	require.NoError(t, err)
}

func newTestTaskSupervisor(t *testing.T) *TaskSupervisor {
	// Initializing dependent objects
	cfg := config.TestServerConfig()
	taskStateMachine, err := fsm.NewTestTaskStateMachine()
	require.NoError(t, err)
	return NewTaskSupervisor(
		cfg,
		taskStateMachine,
	)
}
