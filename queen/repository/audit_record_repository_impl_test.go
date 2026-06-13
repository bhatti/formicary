package repository

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

// Test QueryJobSubmissions aggregation using formicary_job_requests as the source.
func Test_ShouldQueryJobSubmissions(t *testing.T) {
	jobRepo, err := NewTestJobRequestRepository()
	require.NoError(t, err)

	// Clean up job requests so the test is isolated.
	jobRepo.Clear()

	// Helper to save a job request owned by a specific user/org with a given terminal state.
	jobSeq := 0
	saveJob := func(userID, orgID, jobType string, state common.RequestState) string {
		jobSeq++
		qc := common.NewQueryContextFromIDs(userID, orgID)
		req := types.NewRequest()
		req.JobType = jobType
		req.JobDefinitionID = "test-job-def-id"
		req.UserKey = fmt.Sprintf("test-key-%d", jobSeq)
		saved, saveErr := jobRepo.Save(qc, req)
		require.NoError(t, saveErr)
		if state != common.PENDING {
			// Use direct DB update to set terminal state without triggering the
			// user_key="" release that would cause a UNIQUE collision across test rows.
			res := jobRepo.db.Exec(
				"UPDATE formicary_job_requests SET job_state = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
				state, saved.ID)
			require.NoError(t, res.Error)
		}
		return saved.ID
	}

	// user1/org1/build: 2 submitted, 1 COMPLETED, 1 FAILED
	saveJob("user1", "org1", "build", common.COMPLETED)
	saveJob("user1", "org1", "build", common.FAILED)
	// user2/org1/deploy: 1 submitted, 1 COMPLETED
	saveJob("user2", "org1", "deploy", common.COMPLETED)
	// user2/org2/test: 1 submitted, still PENDING (no terminal state yet)
	saveJob("user2", "org2", "test", common.PENDING)

	// WHEN querying all job submissions
	params := make(map[string]interface{})
	summaries, total, err := jobRepo.QueryJobSubmissions(params, 0, 100, []string{})
	require.NoError(t, err)
	// 3 distinct (user, org, job_type) groups
	require.Equal(t, int64(3), total)

	// WHEN filtering by org1 only
	params["organization_id"] = "org1"
	summaries, total, err = jobRepo.QueryJobSubmissions(params, 0, 100, []string{})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)

	// THEN user1/org1/build has correct counts
	var found *JobSubmissionSummary
	for _, s := range summaries {
		if s.UserID == "user1" && s.OrganizationID == "org1" && s.JobType == "build" {
			found = s
			break
		}
	}
	require.NotNil(t, found)
	require.Equal(t, int64(2), found.SubmittedCount)
	require.Equal(t, int64(1), found.SucceededCount)
	require.Equal(t, int64(1), found.FailedCount)
}

// Test that QueryJobSubmissions rejects invalid order parameters.
func Test_ShouldRejectInvalidOrderInJobSubmissions(t *testing.T) {
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)

	params := make(map[string]interface{})
	_, _, err = repo.QueryJobSubmissions(params, 0, 100, []string{"submitted_count; DROP TABLE formicary_job_requests;--"})
	require.Error(t, err, "SQL injection attempt must be rejected")

	_, _, err = repo.QueryJobSubmissions(params, 0, 100, []string{"(SELECT password FROM users)"})
	require.Error(t, err, "arbitrary SQL must be rejected")

	_, _, err = repo.QueryJobSubmissions(params, 0, 100, []string{"submitted_count bogus"})
	require.Error(t, err, "invalid direction must be rejected")

	// Valid orders must succeed
	_, _, err = repo.QueryJobSubmissions(params, 0, 100, []string{"submitted_count desc"})
	require.NoError(t, err)

	_, _, err = repo.QueryJobSubmissions(params, 0, 100, []string{"user_id asc"})
	require.NoError(t, err)
}

// Test SaveFile and query
func Test_ShouldSaveAndQueryAuditRecords(t *testing.T) {
	// GIVEN an audit repository
	repo, err := NewTestAuditRecordRepository()
	require.NoError(t, err)
	clearAuditRecords(repo)
	kinds := []types.AuditKind{
		types.JobRequestCreated,
		types.JobDefinitionUpdated,
		types.JobResourceUpdated,
		types.OrganizationUpdated}

	// AND an existing audit records
	for i := 0; i < 10; i++ {
		for j := 0; j < len(kinds); j++ {
			ec := types.NewAuditRecord(kinds[j], fmt.Sprintf("message %v-%v", i, j))
			_, err = repo.Save(ec)
			require.NoError(t, err)
		}
	}
	params := make(map[string]interface{})

	// WHEN querying audit records
	_, total, err := repo.Query(params, 0, 1000, []string{"id"})

	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(40), total)

	// WHEN querying by kind
	params["kind"] = "JOB_REQUEST_CREATED"
	_, total, err = repo.Query(params, 0, 1000, make([]string, 0))
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(10), total)

	loadedKinds, err := repo.GetKinds()
	require.NoError(t, err)
	require.Equal(t, len(kinds), len(loadedKinds))
}
