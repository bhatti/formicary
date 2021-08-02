package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldMarshalAntReservation(t *testing.T) {
	// Given ant reservation
	reservation := NewAntReservation(
		"ant",
		"topic",
		12,
		"task",
		"",
		1,
	)
	require.NotEqual(t, "", reservation.AllocatedAtString())
	require.NotEqual(t, "", reservation.String())
}
