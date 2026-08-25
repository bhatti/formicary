package events

import (
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldCreateLogEvent(t *testing.T) {
	// Given log event
	e := NewLogEvent(
		"source",
		"userID",
		ulid.Make().String(),
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
		ulid.Make().String(),
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

func Test_ShouldSetDefaultLevelAndSource(t *testing.T) {
	e := NewLogEvent("comp", "u", ulid.Make().String(), "jt", "tt", "exec-id", "task-id", "msg", "", "ant")
	require.Equal(t, "info", e.Level)
	require.Equal(t, "task", e.Source)
}

func Test_ShouldCreateSystemLogEvent(t *testing.T) {
	e := NewSystemLogEvent("Slack", "socket disconnected", "error")
	require.Equal(t, "error", e.Level)
	require.Equal(t, "system", e.Source)
	require.NoError(t, e.Validate())
}

func Test_ShouldMarshalSystemLogEvent(t *testing.T) {
	e := NewSystemLogEvent("HealthMonitor", "db connection lost", "warn")
	b, err := e.Marshal()
	require.NoError(t, err)
	got, err := UnmarshalLogEvent(b)
	require.NoError(t, err)
	require.Equal(t, "warn", got.Level)
	require.Equal(t, "system", got.Source)
	require.Equal(t, "db connection lost", got.Message)
}

func Test_ShouldRejectSystemLogEventWithEmptyMessage(t *testing.T) {
	e := NewSystemLogEvent("comp", "", "error")
	require.Error(t, e.Validate())
}
