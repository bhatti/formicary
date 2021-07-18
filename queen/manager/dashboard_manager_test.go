package manager

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func Test_ShouldBuildRanges(t *testing.T) {
	now := time.Date(2021, 7, 11, 0, 0, 0, 0, time.UTC)
	expected := []time.Time{
		time.Date(2021, 7, 1, 0, 0, 0, 0, now.Location()),
		time.Date(2021, 7, 1, 0, 0, 0, 0, now.Location()),

		time.Date(2021, 12, 1, 0, 0, 0, 0, now.Location()),
		time.Date(2021, 2, 1, 0, 0, 0, 0, now.Location()),

		time.Date(2022, 5, 1, 0, 0, 0, 0, now.Location()),
		time.Date(2020, 9, 1, 0, 0, 0, 0, now.Location()),

		time.Date(2022, 10, 1, 0, 0, 0, 0, now.Location()),
		time.Date(2020, 4, 1, 0, 0, 0, 0, now.Location()),

		time.Date(2023, 3, 1, 0, 0, 0, 0, now.Location()),
		time.Date(2019, 11, 1, 0, 0, 0, 0, now.Location()),
	}
	j := 0
	for i := 0; i < 25; i += 5 {
		from := addMonthYear(now, i)
		to := addMonthYear(now, i*-1)
		require.Equalf(t, expected[j].Unix(), from.Unix(), "expected %s, actual %s", expected[j], from)
		require.Equalf(t, expected[j+1].Unix(), to.Unix(), "expected %s, actual %s", expected[j+1], to)
		j += 2
	}

	ranges := BuildRanges(now, 10, 5, 5)
	require.Equal(t, 21, len(ranges))

	ranges = BuildRanges(now, 1, 1, 1)
	for i, r := range ranges {
		t.Logf("i=%d, begin=%s, end=%s\n", i, r.StartDate, r.EndDate)
	}
	require.Equal(t, 4, len(ranges))
}
