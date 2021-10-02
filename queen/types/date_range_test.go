package types

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func Test_ShouldBuildDateRange(t *testing.T) {
	start := time.Date(2025, 3, 15, 0, 0, 0, 0, time.Now().Location())
	dt := DateRange{
		StartDate: start,
		EndDate: start.Add(time.Hour*30),
	}
	require.Equal(t, "Mar 15 - 16", dt.StartAndEndString())
	require.Equal(t, "Mar 15", dt.StartString())
}
