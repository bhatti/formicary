package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/events"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

// seedCompletedRequestWithExecution creates a job request + execution + task execution + log event
// with updated_at set to the given time. Returns the request ID and job execution ID.
func seedRequestWithExecution(t *testing.T, qc *common.QueryContext, jobType string, state common.RequestState, updatedAt time.Time) (requestID string, execID string) {
	t.Helper()
	locator, err := NewTestLocator()
	require.NoError(t, err)

	jobDef := NewTestJobDefinition(qc.User, jobType)
	jobDef.JobType = "io.formicary.test." + jobType
	savedDef, err := locator.JobDefinitionRepository.Save(qc, jobDef)
	require.NoError(t, err)

	req, err := types.NewJobRequestFromDefinition(savedDef)
	require.NoError(t, err)
	req.UserID = qc.User.ID
	req.OrganizationID = qc.User.OrganizationID
	savedReq, err := locator.JobRequestRepository.Save(qc, req)
	require.NoError(t, err)

	// Set state and back-date updated_at directly to bypass validation that resets to PENDING.
	locator.DB.Exec("UPDATE formicary_job_requests SET job_state = ?, updated_at = ? WHERE id = ?", state, updatedAt, savedReq.ID)

	// Create a job execution.
	jobExec := types.NewJobExecution(savedReq.ToInfo())
	savedExec, err := locator.JobExecutionRepository.Save(jobExec)
	require.NoError(t, err)

	// Seed a log event referencing this execution.
	logEvent := events.NewLogEvent("test-source", savedReq.UserID, savedReq.ID, savedReq.JobType, "task1", savedExec.ID, savedExec.ID, "test log message", "", "test-ant")
	_, err = locator.LogEventRepository.Save(logEvent)
	require.NoError(t, err)

	return savedReq.ID, savedExec.ID
}

// countRows returns row count for a table matching the given condition.
func countRows(t *testing.T, table, col, id string) int64 {
	t.Helper()
	locator, err := NewTestLocator()
	require.NoError(t, err)
	var count int64
	locator.DB.Raw("SELECT COUNT(*) FROM "+table+" WHERE "+col+" = ?", id).Scan(&count)
	return count
}

// Test_ShouldPurgeOldRequests verifies cascade delete covers all child tables.
func Test_ShouldPurgeOldRequests(t *testing.T) {
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	oldDate := time.Now().AddDate(0, 0, -10)
	reqID, execID := seedRequestWithExecution(t, qc, "purge-cascade", common.COMPLETED, oldDate)

	// Verify rows exist before purge.
	require.Equal(t, int64(1), countRows(t, "formicary_job_requests", "id", reqID))
	require.Equal(t, int64(1), countRows(t, "formicary_job_executions", "id", execID))
	require.Greater(t, countRows(t, "formicary_log_events", "job_execution_id", execID), int64(0))

	// WHEN purging requests older than 5 days.
	deleted, err := repo.PurgeOldRequests("io.formicary.test.purge-cascade", common.COMPLETED, time.Now().AddDate(0, 0, -5), 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)

	// THEN all child rows must be gone.
	require.Equal(t, int64(0), countRows(t, "formicary_job_requests", "id", reqID))
	require.Equal(t, int64(0), countRows(t, "formicary_job_executions", "id", execID))
	require.Equal(t, int64(0), countRows(t, "formicary_log_events", "job_execution_id", execID))
}

// Test_ShouldNotPurgeRequestsNewerThanThreshold ensures requests within retention window are kept.
func Test_ShouldNotPurgeRequestsNewerThanThreshold(t *testing.T) {
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	// Seed a COMPLETED request updated just 1 day ago.
	reqID, _ := seedRequestWithExecution(t, qc, "purge-recent", common.COMPLETED, time.Now().AddDate(0, 0, -1))

	// WHEN purging with a threshold of 5 days ago (request is newer).
	deleted, err := repo.PurgeOldRequests("io.formicary.test.purge-recent", common.COMPLETED, time.Now().AddDate(0, 0, -5), 100)
	require.NoError(t, err)
	require.Equal(t, int64(0), deleted)

	// THEN the request must still exist.
	require.Equal(t, int64(1), countRows(t, "formicary_job_requests", "id", reqID))
}

// Test_ShouldRefusePurgeExecutingState ensures EXECUTING (non-terminal) is rejected.
func Test_ShouldRefusePurgeExecutingState(t *testing.T) {
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)

	_, err = repo.PurgeOldRequests("any-job", common.EXECUTING, time.Now(), 100)
	require.Error(t, err)
}

// Test_ShouldRefusePurgePendingState ensures non-terminal states are rejected.
func Test_ShouldRefusePurgePendingState(t *testing.T) {
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)

	for _, state := range []common.RequestState{common.PENDING, common.READY, common.STARTED, common.PAUSED, common.MANUAL_APPROVAL_REQUIRED} {
		_, err = repo.PurgeOldRequests("any-job", state, time.Now(), 100)
		require.Errorf(t, err, "expected error for state %s", state)
	}
}

// Test_ShouldPurgeCancelledRequests verifies CANCELLED is a valid state to purge.
func Test_ShouldPurgeCancelledRequests(t *testing.T) {
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	oldDate := time.Now().AddDate(0, 0, -10)
	reqID, _ := seedRequestWithExecution(t, qc, "purge-cancelled", common.CANCELLED, oldDate)

	deleted, err := repo.PurgeOldRequests("io.formicary.test.purge-cancelled", common.CANCELLED, time.Now().AddDate(0, 0, -5), 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)
	require.Equal(t, int64(0), countRows(t, "formicary_job_requests", "id", reqID))
}

// Test_ShouldRespectBatchLimit ensures the limit parameter is respected.
func Test_ShouldRespectBatchLimit(t *testing.T) {
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	locator, err := NewTestLocator()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	// Save the job definition once; reuse it for all 5 requests.
	jobDef := NewTestJobDefinition(qc.User, "purge-batch-limit")
	jobDef.JobType = "io.formicary.test.purge-batch-limit"
	savedDef, err := locator.JobDefinitionRepository.Save(qc, jobDef)
	require.NoError(t, err)

	oldDate := time.Now().AddDate(0, 0, -10)
	for i := 0; i < 5; i++ {
		req, err := types.NewJobRequestFromDefinition(savedDef)
		require.NoError(t, err)
		req.UserID = qc.User.ID
		req.OrganizationID = qc.User.OrganizationID
		saved, err := locator.JobRequestRepository.Save(qc, req)
		require.NoError(t, err)
		locator.DB.Exec("UPDATE formicary_job_requests SET job_state = ?, updated_at = ? WHERE id = ?", common.FAILED, oldDate, saved.ID)
	}

	// WHEN limit is 3.
	deleted, err := repo.PurgeOldRequests("io.formicary.test.purge-batch-limit", common.FAILED, time.Now().AddDate(0, 0, -5), 3)
	require.NoError(t, err)
	require.Equal(t, int64(3), deleted)

	// AND a second call removes the rest.
	deleted, err = repo.PurgeOldRequests("io.formicary.test.purge-batch-limit", common.FAILED, time.Now().AddDate(0, 0, -5), 3)
	require.NoError(t, err)
	require.Equal(t, int64(2), deleted)
}

