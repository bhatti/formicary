package events

import (
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/types"
	"testing"
)

func Test_ShouldCreateTaskExecutionLifecycleEvent(t *testing.T) {
	// Given task-execution event
	e := NewTaskExecutionLifecycleEvent(
		"source",
		"userID",
		12,
		"jobType",
		"executionID",
		"taskType",
		types.EXECUTING,
		"exit",
		"ant",
		make(map[string]interface{}),
	)

	// WHEN accessing properties
	// THEN it should return saved value
	require.NotEqual(t, "", e.String())
	require.NoError(t, e.Validate())
}

func Test_ShouldMarshalTaskExecutionLifecycleEvent(t *testing.T) {
	// Given task-execution event
	e := NewTaskExecutionLifecycleEvent(
		"source",
		"userID",
		12,
		"jobType",
		"executionID",
		"taskType",
		types.EXECUTING,
		"exit",
		"ant",
		make(map[string]interface{}),
	)

	// WHEN marshaling event
	// THEN it should return serialized bytes
	b, err := e.Marshal()
	require.NoError(t, err)
	copy, err := UnmarshalTaskExecutionLifecycleEvent(b)
	require.NoError(t, err)
	require.Equal(t, e.String(), copy.String())
}
