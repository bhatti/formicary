package types

import (
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/types"
	"testing"
	"time"
)

func Test_ShouldCreateJobTime(t *testing.T) {
	jt := &JobTime{
		ID:             100,
		JobType:        "job",
		UserID:         "user",
		OrganizationID: "org",
		JobVersion:     "v1.0",
		JobState:       types.PENDING,
		ScheduledAt:    time.Now(),
		CreatedAt:      time.Now(),
	}
	require.Equal(t, "org-job:v1.0", jt.GetUserJobTypeKey())
	require.Equal(t, "job", jt.GetJobType())
	require.Equal(t, "v1.0", jt.GetJobVersion())
	require.Equal(t, "org", jt.GetOrganizationID())
	require.Equal(t, "user", jt.GetUserID())
	require.Equal(t, uint64(100), jt.GetID())
	require.Equal(t, -1, jt.GetJobPriority())
	require.Equal(t, types.PENDING, jt.GetJobState())
	require.False(t, jt.GetScheduledAt().IsZero())
	require.False(t, jt.GetCreatedAt().IsZero())
}

func Test_ShouldConvertJobTimeToInfo(t *testing.T) {
	jt := &JobTime{
		ID:          100,
		JobType:     "job",
		JobState:    types.PENDING,
		ScheduledAt: time.Now(),
		CreatedAt:   time.Now(),
	}
	require.NotNil(t, jt.ToInfo())
	require.True(t, jt.Pending())
	require.False(t, jt.Completed())
	require.False(t, jt.Failed())
	require.False(t, jt.Cancelled())
}

func Test_ShouldCalculateElapsedDurationForJobTime(t *testing.T) {
	jt := &JobTime{
		ID:          100,
		JobType:     "job",
		JobState:    types.PENDING,
		ScheduledAt: time.Now(),
		CreatedAt:   time.Now(),
	}
	require.Equal(t, "", jt.ElapsedDuration())
	require.Equal(t, int64(0), jt.Elapsed())
	jt.StartedAt = &jt.ScheduledAt
	require.Equal(t, "", jt.ElapsedDuration())
	require.Equal(t, int64(0), jt.Elapsed())
	end := jt.ScheduledAt.Add(time.Second)
	jt.EndedAt = &end
	require.Equal(t, "1s", jt.ElapsedDuration())
	require.Equal(t, int64(1000), jt.Elapsed())
}
