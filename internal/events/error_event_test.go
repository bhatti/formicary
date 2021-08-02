package events

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldCreateErrorEvent(t *testing.T) {
	// Given job error event
	e := NewErrorEvent(
		"source",
		"userID",
		"msg",
	)

	// WHEN accessing properties
	// THEN it should return saved value
	require.Equal(t, e.Message, e.String())
	require.NoError(t, e.Validate())
}

func Test_ShouldMarshalErrorEvent(t *testing.T) {
	// Given job error event
	e := NewErrorEvent(
		"source",
		"userID",
		"msg",
	)

	// WHEN marshaling event
	// THEN it should return serialized bytes
	b := e.Marshal()
	copy, err := UnmarshalErrorEvent(b)
	require.NoError(t, err)
	require.Equal(t, e.String(), copy.String())
}
