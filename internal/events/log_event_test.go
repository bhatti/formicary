package events

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldCreateLogEvent(t *testing.T) {
	// Given log event
	e := NewLogEvent(
		"source",
		"userID",
		10,
		"jobType",
		"taskType",
		"execution-id",
		"task-id",
		"message",
		"tags",
		"ant-id",
	)

	// WHEN accessing properties
	// THEN it should return saved value
	require.NotEqual(t, "", e.String())
	require.NoError(t, e.Validate())
}

func Test_ShouldMarshalLogEvent(t *testing.T) {
	// Given log event
	e := NewLogEvent(
		"source",
		"userID",
		10,
		"jobType",
		"taskType",
		"execution-id",
		"task-id",
		"message",
		"tags",
		"ant-id",
	)

	// WHEN marshaling event
	// THEN it should return serialized bytes
	b, err := e.Marshal()
	require.NoError(t, err)
	logEvent, err := UnmarshalLogEvent(b)
	require.NoError(t, err)
	require.Equal(t, e.String(), logEvent.String())
}
