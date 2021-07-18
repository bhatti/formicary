package math

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldCalculateRollingAverage(t *testing.T) {
	ra := NewRollingAverage(10)
	for i:=0; i<100; i++ {
		ra.Add(int64(i))
	}
	total := 0
	for i:=90; i<100; i++ {
		total += i
	}
	require.Equal(t, float64(total) / float64(10), ra.Average())
}

func Test_ShouldCalculateRollingMinMax(t *testing.T) {
	ra := NewRollingMinMax(10)
	for i:=0; i<100; i++ {
		ra.Add(int64(i))
	}
	require.Equal(t, int64(0), ra.Min)
	require.Equal(t, int64(99), ra.Max)
}

