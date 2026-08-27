package types

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func Test_ShouldCreateAntAllocation(t *testing.T) {
	// Given ant-allocation
	alloc := NewAntAllocation(
		"ant",
		"topic",
		"12",
		"task",
	)

	// WHEN accessing properties
	// THEN it should return saved value
	require.Contains(t, alloc.String(), "ant")
	require.Equal(t, 1, alloc.Load())
	require.NotEqual(t, "", alloc.AllocatedAtString())
}

func Test_ShouldMarshalVersionFields(t *testing.T) {
	// GIVEN a registration with version info
	reg := AntRegistration{
		AntID:         "ant-v2",
		AntTopic:      "topic",
		MaxCapacity:   5,
		Methods:       []TaskMethod{Kubernetes},
		ReceivedAt:    time.Now(),
		CreatedAt:     time.Now(),
		AntStartedAt:  time.Now(),
		Version:       "0.1.83",
		Commit:        "72c1bc3",
		BuildDate:     "2026-08-26T19:00:31",
	}

	// WHEN marshaling and unmarshaling
	b, err := reg.Marshal()
	require.NoError(t, err)
	restored, err := UnmarshalAntRegistration(b)
	require.NoError(t, err)

	// THEN version fields round-trip correctly
	require.Equal(t, "0.1.83", restored.Version)
	require.Equal(t, "72c1bc3", restored.Commit)
	require.Equal(t, "2026-08-26T19:00:31", restored.BuildDate)
	require.Contains(t, reg.String(), "0.1.83")
}

func Test_ShouldMarshalAntRegistration(t *testing.T) {
	// Given ant registration
	reg := AntRegistration{
		AntID:         "ant",
		AntTopic:      "topic",
		MaxCapacity:   10,
		Tags:          []string{"a", "b"},
		Methods:       []TaskMethod{Kubernetes},
		CurrentLoad:   0,
		TotalExecuted: 0,
		Allocations:   make(map[string]*AntAllocation),
		ReceivedAt:    time.Now(),
		CreatedAt:     time.Now(),
		AntStartedAt:  time.Now(),
	}
	require.True(t, reg.Supports(Kubernetes, []string{"a"}, time.Hour))
	require.True(t, reg.Supports(Kubernetes, []string{"b"}, time.Hour))
	require.False(t, reg.Supports(Docker, []string{"b"}, time.Hour))

	// WHEN marshaling registration
	// THEN it should return serialized bytes
	b, err := reg.Marshal()
	require.NoError(t, err)
	unmarshalAntRegistration, err := UnmarshalAntRegistration(b)
	require.NoError(t, err)
	require.NoError(t, unmarshalAntRegistration.Validate())
	require.Equal(t, reg.String(), unmarshalAntRegistration.String())
	require.NotEqual(t, "", unmarshalAntRegistration.UpdatedAtString())
}
