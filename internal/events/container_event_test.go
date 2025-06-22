package events

import (
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/types"
	"testing"
	"time"
)

func Test_ShouldCreateContainerEvent(t *testing.T) {
	// Given job container event
	e := NewContainerLifecycleEvent(
		"source",
		"userID",
		"antID",
		types.Kubernetes,
		"containerName",
		"containerID",
		types.EXECUTING,
		map[string]string{"l": "k"},
		time.Now(),
		&time.Time{},
	)

	// WHEN accessing properties
	// THEN it should return saved value
	require.Equal(t, e.ContainerID, e.GetID())
	require.Equal(t, e.ContainerName, e.GetName())
	require.Equal(t, e.StartedAt, e.GetStartedAt())
	require.Equal(t, e.EndedAt, e.GetEndedAt())
	require.Equal(t, e.Labels, e.GetLabels())
	require.Equal(t, "", e.GetRuntimeInfo(nil))
	require.NotEqual(t, "", e.String())
	require.NotEqual(t, "", e.Key())
	require.NotEqual(t, "", e.StartedAtString())
}

func Test_ShouldGetRequestIDForContainerEvent(t *testing.T) {
	// Given job container event
	e := NewContainerLifecycleEvent(
		"source",
		"userID",
		"antID",
		types.Kubernetes,
		"containerName",
		"containerID",
		types.EXECUTING,
		map[string]string{"l": "k"},
		time.Now(),
		&time.Time{},
	)

	// WHEN accessing properties
	// THEN it should return saved value
	require.Equal(t, "", e.RequestID())
	e.Labels["RequestID"] = "2"
	require.Equal(t, "2", e.RequestID())
}

func Test_ShouldGetElapsedForContainerEvent(t *testing.T) {
	start := time.Now()
	end := start.Add(time.Second)
	// Given job container event
	e := NewContainerLifecycleEvent(
		"source",
		"userID",
		"antID",
		types.Kubernetes,
		"containerName",
		"containerID",
		types.EXECUTING,
		map[string]string{"l": "k"},
		start,
		&end,
	)

	// WHEN accessing properties
	// THEN it should return saved value
	require.Equal(t, int64(0), e.Elapsed())
	require.Equal(t, time.Duration(0), e.ElapsedSecs())
	require.NotEqual(t, "", e.ElapsedDuration())

	e.ContainerState = types.COMPLETED
	require.Equal(t, int64(1000), e.Elapsed())
	require.Equal(t, 1*time.Second, e.ElapsedSecs())
	require.Equal(t, "1s", e.ElapsedDuration())
}

func Test_ShouldMarshalContainerEvent(t *testing.T) {
	start := time.Now()
	end := start.Add(time.Second)
	// Given job container event
	e := NewContainerLifecycleEvent(
		"source",
		"userID",
		"antID",
		types.Kubernetes,
		"containerName",
		"containerID",
		types.EXECUTING,
		map[string]string{"l": "k"},
		start,
		&end,
	)
	// WHEN marshaling
	// THEN it should return serialized content
	b, err := e.Marshal()
	require.NoError(t, err)

	// AND WHEN unmarshaling
	// THEN it should return back the event
	copyEvent, err := UnmarshalContainerLifecycleEvent(b)
	require.NoError(t, err)
	require.Equal(t, e.String(), copyEvent.String())
}
