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
		12,
		"task",
	)

	// WHEN accessing properties
	// THEN it should return saved value
	require.Contains(t, alloc.String(), "ant")
	require.Equal(t, 1, alloc.Load())
	require.NotEqual(t, "", alloc.AllocatedAtString())
}

func Test_ShouldMarshalAntRegistration(t *testing.T) {
	// Given ant registration
	reg := AntRegistration{
		AntID:        "ant",
		AntTopic:     "topic",
		MaxCapacity:  10,
		Tags:         []string{"a", "b"},
		Methods:      []TaskMethod{Kubernetes},
		CurrentLoad:  0,
		Allocations:  make(map[uint64]*AntAllocation),
		ReceivedAt:   time.Now(),
		CreatedAt:    time.Now(),
		AntStartedAt: time.Now(),
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
