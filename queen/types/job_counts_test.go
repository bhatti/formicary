package types

import (
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/types"
	"testing"
	"time"
)

func Test_ShouldValidateStartEndTimeForJobCounts(t *testing.T) {
	jc := &JobCounts{
		JobType:   "myjob",
		JobState:  types.EXECUTING,
		Counts:    1,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(time.Minute),
		Day:       "1",
	}
	require.NotNil(t, jc.GetStartTime())
	require.NotNil(t, jc.GetEndTime())
	jc.StartTimeString = time.Now().String()
	jc.EndTimeString = time.Now().Add(time.Minute).String()
	require.NotNil(t, jc.GetStartTime())
	require.NotNil(t, jc.GetEndTime())
	jc.StartTimeString = "2026-01-02 15:04:05.999999-07:00"
	jc.EndTimeString = "2026-01-02 15:05:05.999999-07:00"
	require.NotNil(t, jc.GetStartTime())
	require.NotNil(t, jc.GetEndTime())
	require.NotNil(t, jc.GetStartTimeString())
	require.NotNil(t, jc.GetEndTimeString())
}
func Test_ShouldValidateStateForJobCounts(t *testing.T) {
	jc := &JobCounts{
		JobType:   "myjob",
		JobState:  types.EXECUTING,
		Counts:    1,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(time.Minute),
		Day:       "1",
	}
	require.Equal(t, jc.JobState.Completed(), jc.Completed())
	require.Equal(t, jc.JobState.Failed(), jc.Failed())
	require.Equal(t, !jc.JobState.IsTerminal(), jc.NotTerminal())
}

// Validate GetUserJobTypeKey
func Test_ShouldBuildUserJobTypeKeyForJobCounts(t *testing.T) {
	// Given job counts
	jc := &JobCounts{
		JobType:   "myjob",
		JobState:  types.EXECUTING,
		Counts:    1,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(time.Minute),
		Day:       "1",
	}
	jc.UserID = "456"
	// WHEN building user-key

	// THEN it should return valid user-key
	require.Equal(t, "456-myjob:", jc.GetUserJobTypeKey())
	require.Equal(t, jc.JobType, jc.GetJobType())
	require.Equal(t, jc.OrganizationID, jc.GetOrganizationID())
	require.Equal(t, jc.UserID, jc.GetUserID())
}

