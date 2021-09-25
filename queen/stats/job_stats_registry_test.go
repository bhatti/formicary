package stats

import (
	"fmt"
	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
	"testing"
)

func Test_ShouldCountJobsAndGetStats(t *testing.T) {
	// GIVEN stats-registry
	jobStatsRegistry := NewJobStatsRegistry()

	// WHEN adding misc job stats
	for i := 0; i < 10; i++ {
		jobType := fmt.Sprintf("testing-job-%d", i)
		userID := fmt.Sprintf("user-%d", i)
		orgID := fmt.Sprintf("org-%d", i)
		for j := 0; j < 5; j++ {
			req := &types.JobRequestInfo{
				ID:             uint64(j),
				JobType:        jobType,
				UserID:         userID,
				OrganizationID: orgID,
			}
			jobStatsRegistry.SetAntsAvailable(req, true, "")
			jobStatsRegistry.Pending(req)
		}
		for j := 0; j < 5; j++ {
			req := &types.JobRequestInfo{
				ID:             uint64(j),
				JobType:        jobType,
				UserID:         userID,
				OrganizationID: orgID,
			}
			require.Equal(t, int32(j), jobStatsRegistry.GetExecutionCount(req))
			jobStatsRegistry.SetAntsAvailable(req, true, "")
			jobStatsRegistry.Started(req)
		}
		for j := 0; j < 5; j++ {
			req := &types.JobRequestInfo{
				ID:             uint64(j),
				JobType:        jobType,
				UserID:         userID,
				OrganizationID: orgID,
			}
			require.Equal(t, int32(5-j), jobStatsRegistry.GetExecutionCount(req))
			if j == 0 {
				jobStatsRegistry.Cancelled(req)
			} else if j%2 != 0 {
				jobStatsRegistry.Failed(req, int64(5+i))
			} else {
				jobStatsRegistry.Succeeded(req, int64(10+i))
			}
		}
	}

	// AND calling job stats

	stats := jobStatsRegistry.GetStats(common.NewQueryContext(nil, "").WithAdmin(), 0, 500)
	require.Equal(t, 10, len(stats))
}
