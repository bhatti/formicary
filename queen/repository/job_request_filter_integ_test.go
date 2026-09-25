// SPDX-License-Identifier: AGPL-3.0-or-later

package repository

// Integration tests for job-request search/filter combinations.
//
// These tests cover the exact URL patterns that were broken in production,
// especially the case where empty URL params (job_type=&user_id=) caused
// addQueryParamsWhere to emit WHERE job_type = '' returning zero rows.

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	common "plexobject.com/formicary/internal/types"

	"plexobject.com/formicary/queen/types"
)

// saveTerminalRequest creates a job request, immediately sets its state to the
// given terminal state via a direct DB update (matching the prod pattern), and
// returns the saved request.
func saveTerminalRequest(t *testing.T, repo *JobRequestRepositoryImpl, qc *common.QueryContext,
	job *types.JobDefinition, state common.RequestState) *types.JobRequest {
	t.Helper()
	req, err := types.NewJobRequestFromDefinition(job)
	require.NoError(t, err)
	req.UserID = qc.User.ID
	req.OrganizationID = qc.User.OrganizationID
	saved, err := repo.Save(qc, req)
	require.NoError(t, err)
	// Direct DB update to bypass state-machine guards in repo.Save — same as
	// how the prod scheduler transitions jobs.
	err = repo.db.Table("formicary_job_requests").
		Where("id = ?", saved.ID).
		Update("job_state", string(state)).Error
	require.NoError(t, err)
	saved.JobState = state
	return saved
}

// Test_ShouldFilterByJobTypeAndDoneState is the exact scenario reported by the
// user: History tab (job_state=DONE) plus a job_type filter.  Before the fix,
// any empty string param (job_type=, user_id=) added WHERE job_type = '' which
// returned zero rows even when matching records existed.
func Test_ShouldFilterByJobTypeAndDoneState(t *testing.T) {
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	typeA := "io.formicary.filter-test.alpha"
	typeB := "io.formicary.filter-test.beta"
	jobA, err := SaveTestJobDefinitionWithType(qc, typeA)
	require.NoError(t, err)
	jobB, err := SaveTestJobDefinitionWithType(qc, typeB)
	require.NoError(t, err)

	// 3 COMPLETED requests of type A
	for i := 0; i < 3; i++ {
		saveTerminalRequest(t, repo, qc, jobA, common.COMPLETED)
	}
	// 2 FAILED requests of type A
	for i := 0; i < 2; i++ {
		saveTerminalRequest(t, repo, qc, jobA, common.FAILED)
	}
	// 4 COMPLETED requests of type B (should not appear in type-A results)
	for i := 0; i < 4; i++ {
		saveTerminalRequest(t, repo, qc, jobB, common.COMPLETED)
	}
	// 2 PENDING requests of type A (should not appear in DONE filter)
	for i := 0; i < 2; i++ {
		req, err := types.NewJobRequestFromDefinition(jobA)
		require.NoError(t, err)
		req.UserID = qc.User.ID
		req.OrganizationID = qc.User.OrganizationID
		_, err = repo.Save(qc, req)
		require.NoError(t, err)
	}

	adminQC := common.NewQueryContext(nil, "")

	// ── Scenario 1: job_state=DONE&job_type=typeA (core bug scenario) ──────────
	params := map[string]interface{}{"job_state": "DONE", "job_type": typeA}
	recs, total, err := repo.Query(adminQC, params, 0, 100, nil)
	require.NoError(t, err)
	require.Equal(t, int64(5), total, "job_state=DONE + job_type=typeA should return 5 terminal records")
	require.Equal(t, 5, len(recs))
	for _, r := range recs {
		require.Equal(t, typeA, r.JobType)
		require.True(t, r.IsTerminal(), "all returned records must be in terminal state, got %s", r.JobState)
	}

	// ── Scenario 2: job_state=DONE with empty string job_type and user_id ──────
	// This reproduces the exact URL: ?job_state=DONE&q=&job_type=&user_id=
	// Empty strings must be treated as "no filter" (not WHERE job_type = '').
	params = map[string]interface{}{"job_state": "DONE", "q": "", "job_type": "", "user_id": ""}
	recs, total, err = repo.Query(adminQC, params, 0, 100, nil)
	require.NoError(t, err)
	require.Equal(t, int64(9), total, "empty string params must not restrict results; all 9 terminal records expected")

	// ── Scenario 3: job_type only (no state filter) ───────────────────────────
	params = map[string]interface{}{"job_type": typeB}
	recs, total, err = repo.Query(adminQC, params, 0, 100, nil)
	require.NoError(t, err)
	require.Equal(t, int64(4), total)
	for _, r := range recs {
		require.Equal(t, typeB, r.JobType)
	}

	// ── Scenario 4: job_state=RUNNING ─────────────────────────────────────────
	// Save 2 EXECUTING requests of type A
	for i := 0; i < 2; i++ {
		saveTerminalRequest(t, repo, qc, jobA, common.EXECUTING)
	}
	params = map[string]interface{}{"job_state": "RUNNING"}
	recs, total, err = repo.Query(adminQC, params, 0, 100, nil)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	for _, r := range recs {
		require.True(t, r.Running(), "RUNNING filter should only return EXECUTING/STARTED records")
	}

	// ── Scenario 5: job_state=WAITING ─────────────────────────────────────────
	params = map[string]interface{}{"job_state": "WAITING"}
	recs, total, err = repo.Query(adminQC, params, 0, 100, nil)
	require.NoError(t, err)
	require.Equal(t, int64(2), total, "2 PENDING requests should match WAITING filter")
	for _, r := range recs {
		require.True(t, r.Pending() || r.Waiting(), "WAITING filter should only return waiting-state records, got %s", r.JobState)
	}

	// ── Scenario 6: combined RUNNING + job_type=typeA ─────────────────────────
	params = map[string]interface{}{"job_state": "RUNNING", "job_type": typeA}
	recs, total, err = repo.Query(adminQC, params, 0, 100, nil)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	for _, r := range recs {
		require.Equal(t, typeA, r.JobType)
		require.True(t, r.Running())
	}
}

// Test_ShouldFilterByUserID verifies that user_id filter only returns records
// belonging to the specified user.
func Test_ShouldFilterByUserID(t *testing.T) {
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	repo.Clear()
	qc1, err := NewTestQC()
	require.NoError(t, err)
	qc2, err := NewTestQC()
	require.NoError(t, err)

	job1, err := SaveTestJobDefinition(qc1, "filter-user-job-1", "")
	require.NoError(t, err)
	job2, err := SaveTestJobDefinition(qc2, "filter-user-job-2", "")
	require.NoError(t, err)

	// 3 requests from user1, 2 from user2
	for i := 0; i < 3; i++ {
		req, err := types.NewJobRequestFromDefinition(job1)
		require.NoError(t, err)
		req.UserID = qc1.User.ID
		req.OrganizationID = qc1.User.OrganizationID
		_, err = repo.Save(qc1, req)
		require.NoError(t, err)
	}
	for i := 0; i < 2; i++ {
		req, err := types.NewJobRequestFromDefinition(job2)
		require.NoError(t, err)
		req.UserID = qc2.User.ID
		req.OrganizationID = qc2.User.OrganizationID
		_, err = repo.Save(qc2, req)
		require.NoError(t, err)
	}

	adminQC := common.NewQueryContext(nil, "")

	// Filter by user1
	params := map[string]interface{}{"user_id": qc1.User.ID}
	recs, total, err := repo.Query(adminQC, params, 0, 100, nil)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	for _, r := range recs {
		require.Equal(t, qc1.User.ID, r.UserID)
	}

	// Filter by user2
	params = map[string]interface{}{"user_id": qc2.User.ID}
	recs, total, err = repo.Query(adminQC, params, 0, 100, nil)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	for _, r := range recs {
		require.Equal(t, qc2.User.ID, r.UserID)
	}

	// Empty user_id must not filter — same as not providing user_id
	params = map[string]interface{}{"user_id": ""}
	_, total, err = repo.Query(adminQC, params, 0, 100, nil)
	require.NoError(t, err)
	require.Equal(t, int64(5), total, "empty user_id must not restrict results")
}

// Test_ShouldFilterByTextSearch verifies that the q param performs LIKE search
// across job_type and related fields.
func Test_ShouldFilterByTextSearch(t *testing.T) {
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	job, err := SaveTestJobDefinition(qc, "searchable-unique-xyzzy", "")
	require.NoError(t, err)
	otherJob, err := SaveTestJobDefinition(qc, "other-job-qqqqq", "")
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		req, err := types.NewJobRequestFromDefinition(job)
		require.NoError(t, err)
		req.UserID = qc.User.ID
		req.OrganizationID = qc.User.OrganizationID
		_, err = repo.Save(qc, req)
		require.NoError(t, err)
	}
	req, err := types.NewJobRequestFromDefinition(otherJob)
	require.NoError(t, err)
	req.UserID = qc.User.ID
	req.OrganizationID = qc.User.OrganizationID
	_, err = repo.Save(qc, req)
	require.NoError(t, err)

	adminQC := common.NewQueryContext(nil, "")

	// Search by unique substring in job_type
	params := map[string]interface{}{"q": "xyzzy"}
	recs, total, err := repo.Query(adminQC, params, 0, 100, nil)
	require.NoError(t, err)
	require.Equal(t, int64(3), total, "text search should match records with 'xyzzy' in job_type")
	for _, r := range recs {
		require.Contains(t, r.JobType, "xyzzy")
	}

	// Empty q must not filter
	params = map[string]interface{}{"q": ""}
	_, total, err = repo.Query(adminQC, params, 0, 100, nil)
	require.NoError(t, err)
	require.Equal(t, int64(4), total, "empty q must not restrict results")
}

// Test_ShouldFilterJobDefinitionsByType verifies that job definitions can be
// filtered by job_type (partial or exact) and that empty user_id does not add
// a spurious WHERE user_id = '' clause.
func Test_ShouldFilterJobDefinitionsByType(t *testing.T) {
	defRepo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	prefix := fmt.Sprintf("io.formicary.deffilter.%s", qc.User.ID[:6])
	for i := 0; i < 3; i++ {
		job := NewTestJobDefinition(qc.User, fmt.Sprintf("deffilter-%d", i))
		job.JobType = fmt.Sprintf("%s.type%d", prefix, i)
		job.UserID = qc.User.ID
		job.OrganizationID = qc.User.OrganizationID
		_, err = defRepo.Save(qc, job)
		require.NoError(t, err)
	}

	adminQC := common.NewQueryContext(nil, "")

	// Filter by explicit job_type
	params := map[string]interface{}{"job_type": prefix + ".type1"}
	recs, total, err := defRepo.Query(adminQC, params, 0, 100, nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(1))
	found := false
	for _, r := range recs {
		if r.JobType == prefix+".type1" {
			found = true
		}
	}
	require.True(t, found, "job_type filter must find the matching definition")

	// Empty user_id with valid job_type must not suppress results
	params = map[string]interface{}{"job_type": prefix + ".type0", "user_id": ""}
	recs, total, err = defRepo.Query(adminQC, params, 0, 100, nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(1))
	found = false
	for _, r := range recs {
		if r.JobType == prefix+".type0" {
			found = true
		}
	}
	require.True(t, found, "empty user_id must not suppress results when job_type matches")
}

// Test_ShouldQueryWithLEAndGEOperators verifies that the <= and >= comparison
// operators work correctly. Before the fix, HasPrefix("<") matched "<=" first
// so "<=" was dead code and always emitted "< ?" instead of "<= ?".
func Test_ShouldQueryWithLEAndGEOperators(t *testing.T) {
	defRepo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)

	qc, err := NewTestQC()
	require.NoError(t, err)

	prefix := fmt.Sprintf("io.formicary.optest.%s", qc.User.ID[:8])
	// Create 5 definitions with distinct job_types that sort lexicographically.
	jobTypes := make([]string, 5)
	for i := 0; i < 5; i++ {
		jt := fmt.Sprintf("%s.%d", prefix, i)
		jobTypes[i] = jt
		job := NewTestJobDefinition(qc.User, jt)
		job.JobType = jt
		_, err = defRepo.Save(qc, job)
		require.NoError(t, err)
	}
	// jobTypes[0..4] are sorted: prefix.0 < prefix.1 < ... < prefix.4

	adminQC := common.NewQueryContext(nil, "")

	// WHEN querying with <= prefix.2 — should return prefix.0, prefix.1, prefix.2 (3 records)
	params := map[string]interface{}{"job_type:<=": jobTypes[2]}
	_, total, err := defRepo.Query(adminQC, params, 0, 100, nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(3), "<= operator must include the boundary value")

	// Verify it includes the boundary: exact count within our prefix
	count := int64(0)
	recs, _, _ := defRepo.Query(adminQC, params, 0, 100, nil)
	for _, r := range recs {
		if r.JobType == jobTypes[2] {
			count++
		}
	}
	require.Equal(t, int64(1), count, "<= must include records equal to the boundary")

	// WHEN querying with < prefix.2 — boundary record must NOT appear
	params = map[string]interface{}{"job_type:<": jobTypes[2]}
	recs, _, err = defRepo.Query(adminQC, params, 0, 100, nil)
	require.NoError(t, err)
	for _, r := range recs {
		require.NotEqual(t, jobTypes[2], r.JobType, "< must exclude the boundary value")
	}

	// WHEN querying with >= prefix.2 — should include prefix.2, prefix.3, prefix.4
	params = map[string]interface{}{"job_type:>=": jobTypes[2]}
	recs, _, err = defRepo.Query(adminQC, params, 0, 100, nil)
	require.NoError(t, err)
	found := false
	for _, r := range recs {
		if r.JobType == jobTypes[2] {
			found = true
		}
	}
	require.True(t, found, ">= must include records equal to the boundary")

	// WHEN querying with > prefix.2 — boundary record must NOT appear
	params = map[string]interface{}{"job_type:>": jobTypes[2]}
	recs, _, err = defRepo.Query(adminQC, params, 0, 100, nil)
	require.NoError(t, err)
	for _, r := range recs {
		require.NotEqual(t, jobTypes[2], r.JobType, "> must exclude the boundary value")
	}
}

// SaveTestJobDefinitionWithType creates a job definition with the given exact
// job_type string (not the "io.formicary.test." prefix applied by SaveTestJobDefinition).
func SaveTestJobDefinitionWithType(qc *common.QueryContext, jobType string) (*types.JobDefinition, error) {
	repo, err := NewTestJobDefinitionRepository()
	if err != nil {
		return nil, err
	}
	job := NewTestJobDefinition(qc.User, jobType)
	job.JobType = jobType // override the prefix added by NewTestJobDefinition
	job.UserID = qc.User.ID
	job.OrganizationID = qc.User.OrganizationID
	return repo.Save(qc, job)
}
