package events

import (
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/types"
	"testing"
)

func Test_ShouldCreateJobRequestLifecycleEvent(t *testing.T) {
	// Given job request event
	e := NewJobRequestLifecycleEvent(
		"source",
		"userID",
		ulid.Make().String(),
		"jobType",
		types.EXECUTING,
		make(map[string]interface{}),
	)

	// WHEN accessing properties
	// THEN it should return saved value
	require.NotEqual(t, "", e.String())
	require.NoError(t, e.Validate())
}

func Test_ShouldMarshalJobRequestLifecycleEvent(t *testing.T) {
	// Given job request event
	e := NewJobRequestLifecycleEvent(
		"source",
		"userID",
		ulid.Make().String(),
		"jobType",
		types.EXECUTING,
		make(map[string]interface{}),
	)

	// WHEN marshaling event
	// THEN it should return serialized bytes
	b, err := e.Marshal()
	require.NoError(t, err)
	copy, err := UnmarshalJobRequestLifecycleEvent(b)
	require.NoError(t, err)
	require.Equal(t, e.String(), copy.String())
}
