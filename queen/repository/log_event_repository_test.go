package repository

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"plexobject.com/formicary/internal/events"
)

func Test_ShouldNotDeletingNonExistingTaskExecutionID(t *testing.T) {
	// GIVEN a log repository
	repo, err := NewTestLogEventRepository()
	require.NoError(t, err)
	// WHEN deleting non-existing task-execution-id
	total, err := repo.DeleteByJobExecutionID("xx")
	// THEN total should be zero
	require.NoError(t, err)
	require.Equal(t, int64(0), total)
}

func Test_ShouldNotDeletingNonExistingJobExecutionID(t *testing.T) {
	// GIVEN a log repository
	repo, err := NewTestLogEventRepository()
	require.NoError(t, err)
	// WHEN deleting non-existing job-execution-id
	total, err := repo.DeleteByJobExecutionID("xx")
	// THEN total should be zero
	require.NoError(t, err)
	require.Equal(t, int64(0), total)
}

func Test_ShouldNotDeletingNonExistingRequestID(t *testing.T) {
	// GIVEN a log repository
	repo, err := NewTestLogEventRepository()
	require.NoError(t, err)
	// WHEN deleting non-existing request-id
	total, err := repo.DeleteByRequestID(0)
	// THEN total should be zero
	require.NoError(t, err)
	require.Equal(t, int64(0), total)
}

// Test Save and query
func Test_ShouldSaveAndQueryLogEvents(t *testing.T) {
	// GIVEN a log repository
	repo, err := NewTestLogEventRepository()
	require.NoError(t, err)
	repo.clear()

	// AND log records in the database
	for i := 1; i <= 10; i++ {
		for j := 0; j < 5; j++ {
			e := events.NewLogEvent(
				"source",
				"username",
				uint64(i),
				"job-type",
				"taskType",
				fmt.Sprintf("job-exec-%d", i),
				fmt.Sprintf("task-exec-%d-%d", i, j),
				fmt.Sprintf("message-%d-%d", i, j),
				"tags",
				"ant")
			_, err = repo.Save(e)
			require.NoError(t, err)
		}
	}

	params := make(map[string]interface{})

	// WHEN querying log events
	_, total, err := repo.Query(params, 0, 1000, []string{"id"})

	// THEN it should return valid results
	require.NoError(t, err)
	require.Equal(t, int64(50), total)

	// WHEN querying by task-execution-id
	params["task_execution_id"] = "task-exec-1-0"
	_, total, err = repo.Query(params, 0, 1000, make([]string, 0))
	// THEN it should return valid results
	require.NoError(t, err)
	require.Equal(t, int64(1), total)

	// WHEN deleting by request id
	total, err = repo.DeleteByRequestID(1)
	// THEN it should return valid results
	require.NoError(t, err)
	require.Equal(t, int64(5), total)

	// WHEN deleting by job-execution-id
	total, err = repo.DeleteByJobExecutionID("job-exec-1")
	// THEN it should return valid results
	require.NoError(t, err)
	require.Equal(t, int64(0), total)

	// WHEN deleting by task-execution-id
	total, err = repo.DeleteByTaskExecutionID("task-exec-2-0")
	// THEN it should return valid results
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
}
