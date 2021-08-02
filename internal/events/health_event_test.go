package events

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldCreateHealthEvent(t *testing.T) {
	// Given health event
	e := NewHealthErrorEvent(
		"source",
		"msg",
	)

	// WHEN accessing properties
	// THEN it should return saved value
	require.Equal(t, e.Error, e.String())
	require.NoError(t, e.Validate())
}

func Test_ShouldMarshalHealthEvent(t *testing.T) {
	// Given health event
	e := NewHealthErrorEvent(
		"source",
		"msg",
	)

	// WHEN marshaling event
	// THEN it should return serialized bytes
	b, err := e.Marshal()
	require.NoError(t, err)
	copy, err := UnmarshalHealthErrorEvent(b)
	require.NoError(t, err)
	require.Equal(t, e.String(), copy.String())
}
