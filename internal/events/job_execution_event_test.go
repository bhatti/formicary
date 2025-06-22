package events

import (
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/types"
	"testing"
)

func Test_ShouldCreateJobExecutionLaunchEvent(t *testing.T) {
	// Given job-execution-launch event
	e := NewJobExecutionLaunchEvent(
		"source",
		"userID",
		ulid.Make().String(),
		"jobType",
		"executionID",
		make(map[string]*types.AntReservation),
	)

	// WHEN accessing properties
	// THEN it should return saved value
	require.NotEqual(t, "", e.String())
	require.NoError(t, e.Validate())
}

func Test_ShouldMarshalJobExecutionLaunchEvent(t *testing.T) {
	// Given job-execution-launch event
	e := NewJobExecutionLaunchEvent(
		"source",
		"userID",
		ulid.Make().String(),
		"jobType",
		"executionID",
		make(map[string]*types.AntReservation),
	)

	// WHEN marshaling event
	// THEN it should return serialized bytes
	b, err := e.Marshal()
	require.NoError(t, err)
	launchEvent, err := UnmarshalJobExecutionLaunchEvent(b)
	require.NoError(t, err)
	require.Equal(t, e.String(), launchEvent.String())
}

func Test_ShouldCreateJobExecutionLifecycleEvent(t *testing.T) {
	// Given event
	e := NewJobExecutionLifecycleEvent(
		"source",
		"userID",
		ulid.Make().String(),
		"jobType",
		"executionID",
		types.EXECUTING,
		1,
		make(map[string]interface{}),
	)

	// WHEN accessing properties
	// THEN it should return saved value
	require.NotEqual(t, "", e.String())
	require.NoError(t, e.Validate())
}

func Test_ShouldMarshalJobExecutionLifecycleEvent(t *testing.T) {
	// Given job-execution event
	e := NewJobExecutionLifecycleEvent(
		"source",
		"userID",
		ulid.Make().String(),
		"jobType",
		"executionID",
		types.EXECUTING,
		1,
		make(map[string]interface{}),
	)

	// WHEN marshaling event
	// THEN it should return serialized bytes
	b, err := e.Marshal()
	require.NoError(t, err)
	lifecycleEvent, err := UnmarshalJobExecutionLifecycleEvent(b)
	require.NoError(t, err)
	require.Equal(t, e.String(), lifecycleEvent.String())
}
