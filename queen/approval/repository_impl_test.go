// SPDX-License-Identifier: AGPL-3.0-or-later
package approval

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// newTestRepo creates an approval repository backed by the shared test SQLite DB.
func newTestRepo(t *testing.T) *RepositoryImpl {
	t.Helper()
	locator, err := repository.NewTestLocator()
	require.NoError(t, err)
	repo, err := NewRepositoryImpl(locator.DB)
	require.NoError(t, err)
	return repo
}

// newTestPolicy returns a minimal valid ApprovalPolicy (not persisted).
func newTestPolicy(taskDefID string) *types.ApprovalPolicy {
	return &types.ApprovalPolicy{
		TaskDefinitionID: taskDefID,
		MinApprovals:     1,
	}
}

func Test_ShouldSaveAndGetApprovalPolicy(t *testing.T) {
	// GIVEN
	repo := newTestRepo(t)
	policy := newTestPolicy("task-def-" + time.Now().Format("20060102150405.999999999"))

	// WHEN
	saved, err := repo.SavePolicy(policy)

	// THEN
	require.NoError(t, err)
	assert.NotEmpty(t, saved.ID)

	// AND: retrievable by task definition ID
	loaded, err := repo.GetPolicyByTaskDefinition(saved.TaskDefinitionID)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, saved.ID, loaded.ID)
	assert.Equal(t, 1, loaded.MinApprovals)
}

func Test_ShouldReturnNilForMissingPolicy(t *testing.T) {
	// GIVEN
	repo := newTestRepo(t)

	// WHEN
	loaded, err := repo.GetPolicyByTaskDefinition("does-not-exist-" + time.Now().Format("20060102150405.999999999"))

	// THEN: no error, nil result
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

func Test_ShouldSaveAndRetrieveVote(t *testing.T) {
	// GIVEN
	repo := newTestRepo(t)
	taskExecID := fmt.Sprintf("te-%d", time.Now().UnixNano())
	reqID := fmt.Sprintf("req-%d", time.Now().UnixNano())

	vote := &types.ApprovalVote{
		TaskExecutionID: taskExecID,
		JobRequestID:    reqID,
		VoterID:         "alice",
		Decision:        types.ApprovalDecisionApproved,
		Comments:        "LGTM",
		VotedAt:         time.Now(),
	}

	// WHEN
	saved, err := repo.SaveVote(vote)

	// THEN
	require.NoError(t, err)
	assert.NotEmpty(t, saved.ID)
	assert.Equal(t, types.ApprovalDecisionApproved, saved.Decision)

	// AND: retrievable
	votes, err := repo.GetVotes(taskExecID)
	require.NoError(t, err)
	require.Len(t, votes, 1)
	assert.Equal(t, "alice", votes[0].VoterID)
}

func Test_ShouldBeIdempotentOnDuplicateVote(t *testing.T) {
	// GIVEN: a vote already exists
	repo := newTestRepo(t)
	taskExecID := fmt.Sprintf("te-idem-%d", time.Now().UnixNano())
	reqID := fmt.Sprintf("req-idem-%d", time.Now().UnixNano())

	vote := &types.ApprovalVote{
		TaskExecutionID: taskExecID,
		JobRequestID:    reqID,
		VoterID:         "bob",
		Decision:        types.ApprovalDecisionApproved,
		VotedAt:         time.Now(),
	}
	_, err := repo.SaveVote(vote)
	require.NoError(t, err)

	// WHEN: same voter tries to vote again (different decision)
	duplicate := &types.ApprovalVote{
		TaskExecutionID: taskExecID,
		JobRequestID:    reqID,
		VoterID:         "bob",
		Decision:        types.ApprovalDecisionRejected,
		VotedAt:         time.Now(),
	}
	saved, err := repo.SaveVote(duplicate)

	// THEN: no error, returns the ORIGINAL vote (ON CONFLICT DO NOTHING)
	require.NoError(t, err)
	assert.Equal(t, types.ApprovalDecisionApproved, saved.Decision, "duplicate vote must not overwrite original")

	// AND: still only one vote in DB
	votes, err := repo.GetVotes(taskExecID)
	require.NoError(t, err)
	assert.Len(t, votes, 1)
}

func Test_ShouldCountVotesCorrectly(t *testing.T) {
	// GIVEN: 2 approvals, 1 rejection
	repo := newTestRepo(t)
	taskExecID := fmt.Sprintf("te-count-%d", time.Now().UnixNano())
	reqID := fmt.Sprintf("req-count-%d", time.Now().UnixNano())

	for _, voter := range []struct {
		id       string
		decision types.ApprovalDecision
	}{
		{"alice", types.ApprovalDecisionApproved},
		{"bob", types.ApprovalDecisionApproved},
		{"carol", types.ApprovalDecisionRejected},
	} {
		_, err := repo.SaveVote(&types.ApprovalVote{
			TaskExecutionID: taskExecID,
			JobRequestID:    reqID,
			VoterID:         voter.id,
			Decision:        voter.decision,
			VotedAt:         time.Now(),
		})
		require.NoError(t, err)
	}

	// WHEN
	approvals, rejections, err := repo.CountVotes(taskExecID)

	// THEN
	require.NoError(t, err)
	assert.Equal(t, 2, approvals)
	assert.Equal(t, 1, rejections)
}

func Test_ShouldHasVotedReturnTrueAfterVoting(t *testing.T) {
	// GIVEN
	repo := newTestRepo(t)
	taskExecID := fmt.Sprintf("te-hv-%d", time.Now().UnixNano())
	reqID := fmt.Sprintf("req-hv-%d", time.Now().UnixNano())

	_, err := repo.SaveVote(&types.ApprovalVote{
		TaskExecutionID: taskExecID,
		JobRequestID:    reqID,
		VoterID:         "dave",
		Decision:        types.ApprovalDecisionApproved,
		VotedAt:         time.Now(),
	})
	require.NoError(t, err)

	// WHEN
	voted, err := repo.HasVoted(taskExecID, "dave")
	require.NoError(t, err)
	assert.True(t, voted)

	notVoted, err := repo.HasVoted(taskExecID, "eve")
	require.NoError(t, err)
	assert.False(t, notVoted)
}

func Test_ShouldSaveAndFindBreachedDeadlines(t *testing.T) {
	// GIVEN
	repo := newTestRepo(t)
	taskExecID := fmt.Sprintf("te-dl-%d", time.Now().UnixNano())
	reqID := fmt.Sprintf("req-dl-%d", time.Now().UnixNano())

	// A deadline already in the past
	deadline := &types.ApprovalDeadline{
		TaskExecutionID:      taskExecID,
		JobRequestID:         reqID,
		Deadline:             time.Now().Add(-10 * time.Minute),
		TimeoutAction:        types.TimeoutActionAutoReject,
		EscalationRecipients: "ops@example.com",
	}
	saved, err := repo.SaveDeadline(deadline)
	require.NoError(t, err)
	assert.NotEmpty(t, saved.ID)

	// WHEN: query breached deadlines
	breached, err := repo.FindBreachedDeadlines(time.Now(), 10)
	require.NoError(t, err)

	// THEN: our deadline appears
	found := false
	for _, d := range breached {
		if d.ID == saved.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "past deadline must appear in breached list")
}

func Test_ShouldNotReturnFutureDeadlineAsBreached(t *testing.T) {
	// GIVEN: deadline in the future
	repo := newTestRepo(t)
	taskExecID := fmt.Sprintf("te-future-%d", time.Now().UnixNano())
	reqID := fmt.Sprintf("req-future-%d", time.Now().UnixNano())

	deadline := &types.ApprovalDeadline{
		TaskExecutionID: taskExecID,
		JobRequestID:    reqID,
		Deadline:        time.Now().Add(24 * time.Hour),
		TimeoutAction:   types.TimeoutActionEscalate,
	}
	saved, err := repo.SaveDeadline(deadline)
	require.NoError(t, err)

	// WHEN
	breached, err := repo.FindBreachedDeadlines(time.Now(), 100)
	require.NoError(t, err)

	// THEN: not in the list
	for _, d := range breached {
		assert.NotEqual(t, saved.ID, d.ID, "future deadline must not appear in breached list")
	}
}

func Test_ShouldNotReturnEscalatedDeadlineAsBreached(t *testing.T) {
	// GIVEN: past deadline that was already escalated
	repo := newTestRepo(t)
	taskExecID := fmt.Sprintf("te-esc-%d", time.Now().UnixNano())
	reqID := fmt.Sprintf("req-esc-%d", time.Now().UnixNano())

	deadline := &types.ApprovalDeadline{
		TaskExecutionID: taskExecID,
		JobRequestID:    reqID,
		Deadline:        time.Now().Add(-5 * time.Minute),
		TimeoutAction:   types.TimeoutActionEscalate,
	}
	saved, err := repo.SaveDeadline(deadline)
	require.NoError(t, err)

	err = repo.MarkDeadlineEscalated(saved.ID)
	require.NoError(t, err)

	// WHEN
	breached, err := repo.FindBreachedDeadlines(time.Now(), 100)
	require.NoError(t, err)

	// THEN: escalated row NOT returned
	for _, d := range breached {
		assert.NotEqual(t, saved.ID, d.ID, "escalated deadline must not reappear")
	}
}

func Test_ShouldMarkDeadlineResolved(t *testing.T) {
	// GIVEN
	repo := newTestRepo(t)
	taskExecID := fmt.Sprintf("te-res-%d", time.Now().UnixNano())
	reqID := fmt.Sprintf("req-res-%d", time.Now().UnixNano())

	deadline := &types.ApprovalDeadline{
		TaskExecutionID: taskExecID,
		JobRequestID:    reqID,
		Deadline:        time.Now().Add(-1 * time.Minute),
		TimeoutAction:   types.TimeoutActionAutoApprove,
	}
	saved, err := repo.SaveDeadline(deadline)
	require.NoError(t, err)

	// WHEN
	err = repo.MarkDeadlineResolved(saved.ID)
	require.NoError(t, err)

	// THEN: no longer in breached list
	breached, err := repo.FindBreachedDeadlines(time.Now(), 100)
	require.NoError(t, err)
	for _, d := range breached {
		assert.NotEqual(t, saved.ID, d.ID, "resolved deadline must not appear in breached list")
	}
}

func Test_ShouldResolveDeadlineForTask(t *testing.T) {
	// GIVEN: two deadlines for the same task execution
	repo := newTestRepo(t)
	taskExecID := fmt.Sprintf("te-rft-%d", time.Now().UnixNano())
	reqID := fmt.Sprintf("req-rft-%d", time.Now().UnixNano())

	for i := 0; i < 2; i++ {
		_, err := repo.SaveDeadline(&types.ApprovalDeadline{
			TaskExecutionID: taskExecID,
			JobRequestID:    reqID,
			Deadline:        time.Now().Add(-1 * time.Minute),
			TimeoutAction:   types.TimeoutActionAutoReject,
		})
		require.NoError(t, err)
	}

	// WHEN
	err := repo.ResolveDeadlineForTask(taskExecID)
	require.NoError(t, err)

	// THEN: none appear in breached list
	breached, err := repo.FindBreachedDeadlines(time.Now(), 100)
	require.NoError(t, err)
	for _, d := range breached {
		assert.NotEqual(t, taskExecID, d.TaskExecutionID)
	}
}

func Test_ShouldFindPendingApprovalsWithCorrectJoin(t *testing.T) {
	// GIVEN: a job execution + task in MANUAL_APPROVAL_REQUIRED state
	locator, err := repository.NewTestLocator()
	require.NoError(t, err)
	repo, err := NewRepositoryImpl(locator.DB)
	require.NoError(t, err)

	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	jobReq, jobExec, err := repository.NewTestJobExecution(qc, "approval-pending-test")
	require.NoError(t, err)

	// Save the job execution (gives IDs to tasks)
	saved, err := locator.JobExecutionRepository.Save(jobExec)
	require.NoError(t, err)
	require.NotEmpty(t, saved.Tasks, "job execution must have tasks")

	// Wire job request → job execution (required by FindPendingApprovals JOIN)
	err = locator.DB.Table("formicary_job_requests").
		Where("id = ?", jobReq.ID).
		Update("job_execution_id", saved.ID).Error
	require.NoError(t, err)

	// Mark first task as MANUAL_APPROVAL_REQUIRED
	taskExecID := saved.Tasks[0].ID
	err = locator.DB.Table("formicary_task_executions").
		Where("id = ?", taskExecID).
		Update("task_state", "MANUAL_APPROVAL_REQUIRED").Error
	require.NoError(t, err)

	// WHEN
	pending, total, err := repo.FindPendingApprovals(qc, 0, 20)

	// THEN
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))

	found := false
	for _, p := range pending {
		if p.TaskExecutionID == taskExecID {
			found = true
			assert.Equal(t, jobReq.ID, p.JobRequestID, "job_request_id must come from correct JOIN")
			break
		}
	}
	assert.True(t, found, "task in MANUAL_APPROVAL_REQUIRED must appear in pending list")
}
