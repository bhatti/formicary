package repository

import (
	"fmt"
	"github.com/oklog/ulid/v2"
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
	total, err := repo.DeleteByRequestID("")
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

	var reqIds []string
	// AND log records in the database
	for i := 1; i <= 10; i++ {
		reqId := ulid.Make().String()
		for j := 0; j < 5; j++ {
			e := events.NewLogEvent(
				"source",
				"username",
				reqId,
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
		reqIds = append(reqIds, reqId)
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
	total, err = repo.DeleteByRequestID(reqIds[0])
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

func Test_ShouldFilterByLevel(t *testing.T) {
	repo, err := NewTestLogEventRepository()
	require.NoError(t, err)
	repo.clear()

	save := func(level string) {
		e := events.NewLogEvent("c", "u", ulid.Make().String(), "jt", "tt",
			ulid.Make().String(), ulid.Make().String(), "msg-"+level, "", "ant")
		e.Level = level
		_, _ = repo.Save(e)
	}
	save("info")
	save("warn")
	save("error")

	_, total, err := repo.Query(map[string]interface{}{"level": "warn"}, 0, 100, nil)
	require.NoError(t, err)
	require.Equal(t, int64(2), total, "warn filter should return warn + error")

	_, total, err = repo.Query(map[string]interface{}{"level": "error"}, 0, 100, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), total, "error filter should return only error")

	_, total, err = repo.Query(map[string]interface{}{"level": "info"}, 0, 100, nil)
	require.NoError(t, err)
	require.Equal(t, int64(3), total, "info filter should return all")
}

func Test_ShouldFilterBySource(t *testing.T) {
	repo, err := NewTestLogEventRepository()
	require.NoError(t, err)
	repo.clear()

	taskEvent := events.NewLogEvent("c", "u", ulid.Make().String(), "jt", "tt",
		ulid.Make().String(), ulid.Make().String(), "task-msg", "", "ant")
	_, _ = repo.Save(taskEvent)

	sysEvent := events.NewSystemLogEvent("HealthMonitor", "db lost", "error")
	_, _ = repo.Save(sysEvent)

	_, total, err := repo.Query(map[string]interface{}{"source": "task"}, 0, 100, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)

	_, total, err = repo.Query(map[string]interface{}{"source": "system"}, 0, 100, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
}

func Test_ShouldReturnLevelsAtAndAbove(t *testing.T) {
	require.Equal(t, []string{"info", "warn", "error"}, logLevelsAtAndAbove("info"))
	require.Equal(t, []string{"warn", "error"}, logLevelsAtAndAbove("warn"))
	require.Equal(t, []string{"error"}, logLevelsAtAndAbove("error"))
	require.Nil(t, logLevelsAtAndAbove("debug"))
}
