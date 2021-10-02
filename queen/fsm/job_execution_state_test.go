package fsm

import (
	"context"
	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"testing"
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
