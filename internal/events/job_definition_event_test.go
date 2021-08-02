package events

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldCreateJobDefinitionLifecycleEvent(t *testing.T) {
	// Given job lifecycle event
	e := NewJobDefinitionLifecycleEvent(
		"source",
		"userID",
		"defID",
		"jobType",
		UPDATED,
	)

	// WHEN accessing properties
	// THEN it should return saved value
	require.NotEqual(t, "", e.String())
	require.NoError(t, e.Validate())
}

func Test_ShouldMarshalJobDefinitionLifecycleEvent(t *testing.T) {
	// Given job lifecycle event
	e := NewJobDefinitionLifecycleEvent(
		"source",
		"userID",
		"defID",
		"jobType",
		UPDATED,
	)

	// WHEN marshaling event
	// THEN it should return serialized bytes
	b, err := e.Marshal()
	require.NoError(t, err)
	copy, err := UnmarshalJobDefinitionLifecycleEvent(b)
	require.NoError(t, err)
	require.Equal(t, e.String(), copy.String())
}
