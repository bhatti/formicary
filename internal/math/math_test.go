package math

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldCalculateMax(t *testing.T) {
	require.Equal(t, 15, Max(5, 15))
}

func Test_ShouldCalculateMin(t *testing.T) {
	require.Equal(t, 5, Min(5, 15))
}
