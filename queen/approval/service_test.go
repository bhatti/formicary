// SPDX-License-Identifier: AGPL-3.0-or-later
package approval

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// ─── test scaffold ────────────────────────────────────────────────────────────

type svcFixture struct {
	svc    *Service
	repo   *RepositoryImpl
	db     *gorm.DB
	qc     *common.QueryContext
	jobReq *types.JobRequest
	jobExec *types.JobExecution
	taskExecID string
	taskType   string
}

// newSvcFixture builds a real Service backed by SQLite, with a persisted
// job-request + job-execution in MANUAL_APPROVAL_REQUIRED state.
func newSvcFixture(t *testing.T, policy *types.ApprovalPolicy) *svcFixture {
	t.Helper()

	locator, err := repository.NewTestLocator()
	require.NoError(t, err)

	repo, err := NewRepositoryImpl(locator.DB)
	require.NoError(t, err)

	svc := NewService(locator.DB, repo)

	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	// Create a persisted job-request + job-execution.
	jobReq, jobExec, err := repository.NewTestJobExecution(qc, fmt.Sprintf("approval-svc-%d", time.Now().UnixNano()))
	require.NoError(t, err)

	saved, err := locator.JobExecutionRepository.Save(jobExec)
	require.NoError(t, err)
	require.NotEmpty(t, saved.Tasks)

	taskType := saved.Tasks[0].TaskType
	taskExecID := saved.Tasks[0].ID

	// Wire job_request → job_execution.
	err = locator.DB.Table("formicary_job_requests").
		Where("id = ?", jobReq.ID).
		Updates(map[string]interface{}{
			"job_execution_id": saved.ID,
			"job_state":        string(common.MANUAL_APPROVAL_REQUIRED),
		}).Error
	require.NoError(t, err)

	// Set task to MANUAL_APPROVAL_REQUIRED.
	err = locator.DB.Table("formicary_task_executions").
		Where("id = ?", taskExecID).
		Update("task_state", string(common.MANUAL_APPROVAL_REQUIRED)).Error
	require.NoError(t, err)

	// Reload the job request with the updated state.
	jobReq.JobState = common.MANUAL_APPROVAL_REQUIRED
	jobReq.JobExecutionID = saved.ID

	return &svcFixture{
		svc:        svc,
		repo:       repo,
		db:         locator.DB,
		qc:         qc,
		jobReq:     jobReq,
		jobExec:    saved,
		taskExecID: taskExecID,
		taskType:   taskType,
	}
}

// openPolicy returns a policy with no user/role restrictions (any authenticated voter may vote).
func openPolicy(minApprovals int) *types.ApprovalPolicy {
	return &types.ApprovalPolicy{MinApprovals: minApprovals}
}

// userPolicy returns a policy that restricts votes to the listed users.
func userPolicy(users string, minApprovals int) *types.ApprovalPolicy {
	return &types.ApprovalPolicy{
		MinApprovals: minApprovals,
		AllowedUsers: users,
	}
}

// ─── tests ────────────────────────────────────────────────────────────────────

func Test_ShouldReachQuorumWithSingleApprover(t *testing.T) {
	// GIVEN: policy requires 1 approval
	f := newSvcFixture(t, openPolicy(1))
	ctx := context.Background()

	// WHEN: alice approves
	status, err := f.svc.CastVote(ctx, f.qc, f.jobReq, f.taskType, f.taskExecID,
		openPolicy(1), "alice", "Alice Smith", types.ApprovalDecisionApproved, "LGTM")

	// THEN: quorum immediately reached
	require.NoError(t, err)
	assert.True(t, status.QuorumReached)
	assert.False(t, status.Rejected)
	assert.Equal(t, 1, status.ApprovalsReceived)
}

func Test_ShouldNotReachQuorumUntilThresholdMet(t *testing.T) {
	// GIVEN: policy requires 2 approvals, alice,bob,carol are allowed
	policy := userPolicy("alice,bob,carol", 2)
	f := newSvcFixture(t, policy)
	ctx := context.Background()

	// WHEN: only alice votes
	status, err := f.svc.CastVote(ctx, f.qc, f.jobReq, f.taskType, f.taskExecID,
		policy, "alice", "Alice", types.ApprovalDecisionApproved, "")

	// THEN: quorum not yet reached
	require.NoError(t, err)
	assert.False(t, status.QuorumReached)
	assert.False(t, status.Rejected)
	assert.Equal(t, 1, status.ApprovalsReceived)

	// AND WHEN: bob approves too
	status2, err := f.svc.CastVote(ctx, f.qc, f.jobReq, f.taskType, f.taskExecID,
		policy, "bob", "Bob", types.ApprovalDecisionApproved, "")

	// THEN: quorum reached
	require.NoError(t, err)
	assert.True(t, status2.QuorumReached)
	assert.Equal(t, 2, status2.ApprovalsReceived)
}

func Test_ShouldRejectImmediatelyWhenRequireUnanimous(t *testing.T) {
	// GIVEN: unanimous policy — any rejection fails
	policy := &types.ApprovalPolicy{
		MinApprovals:     2,
		AllowedUsers:     "alice,bob,carol",
		RequireUnanimous: true,
	}
	f := newSvcFixture(t, policy)
	ctx := context.Background()

	// WHEN: carol rejects
	status, err := f.svc.CastVote(ctx, f.qc, f.jobReq, f.taskType, f.taskExecID,
		policy, "carol", "Carol", types.ApprovalDecisionRejected, "Not ready")

	// THEN: task is rejected immediately
	require.NoError(t, err)
	assert.True(t, status.Rejected)
	assert.False(t, status.QuorumReached)
}

func Test_ShouldNotRejectWhenQuorumStillPossible(t *testing.T) {
	// GIVEN: 3 allowed voters, need 2 approvals, no unanimous requirement
	policy := userPolicy("alice,bob,carol", 2)
	f := newSvcFixture(t, policy)
	ctx := context.Background()

	// WHEN: carol rejects (alice and bob can still approve to reach quorum)
	status, err := f.svc.CastVote(ctx, f.qc, f.jobReq, f.taskType, f.taskExecID,
		policy, "carol", "Carol", types.ApprovalDecisionRejected, "Needs review")

	// THEN: NOT yet rejected — quorum is still mathematically possible
	require.NoError(t, err)
	assert.False(t, status.Rejected)
	assert.False(t, status.QuorumReached)
}

func Test_ShouldRejectWhenQuorumMathematicallyImpossible(t *testing.T) {
	// GIVEN: 3 allowed voters, need 2 approvals, 2 rejections cast
	policy := userPolicy("alice,bob,carol", 2)
	f := newSvcFixture(t, policy)
	ctx := context.Background()

	// alice rejects
	_, err := f.svc.CastVote(ctx, f.qc, f.jobReq, f.taskType, f.taskExecID,
		policy, "alice", "Alice", types.ApprovalDecisionRejected, "No")
	require.NoError(t, err)

	// bob rejects — only carol remains, she alone cannot reach min_approvals=2
	status, err := f.svc.CastVote(ctx, f.qc, f.jobReq, f.taskType, f.taskExecID,
		policy, "bob", "Bob", types.ApprovalDecisionRejected, "Nope")

	// THEN: quorum impossible → rejected
	require.NoError(t, err)
	assert.True(t, status.Rejected)
}

func Test_ShouldBeDuplicateVoteIdempotent(t *testing.T) {
	// GIVEN: alice already voted
	policy := userPolicy("alice,bob,carol", 2)
	f := newSvcFixture(t, policy)
	ctx := context.Background()

	_, err := f.svc.CastVote(ctx, f.qc, f.jobReq, f.taskType, f.taskExecID,
		policy, "alice", "Alice", types.ApprovalDecisionApproved, "first")
	require.NoError(t, err)

	// WHEN: alice tries to vote again (rejected decision this time)
	status, err := f.svc.CastVote(ctx, f.qc, f.jobReq, f.taskType, f.taskExecID,
		policy, "alice", "Alice", types.ApprovalDecisionRejected, "changed my mind")

	// THEN: no error; original approval is preserved, count stays 1
	require.NoError(t, err)
	assert.Equal(t, 1, status.ApprovalsReceived, "duplicate vote must not change counts")
	assert.Equal(t, 0, status.RejectionsReceived)
}

func Test_ShouldDenyVoterNotInAllowedUsers(t *testing.T) {
	// GIVEN: policy restricts to alice and bob
	policy := userPolicy("alice,bob", 1)
	f := newSvcFixture(t, policy)
	ctx := context.Background()

	// WHEN: mallory (not in list) tries to vote
	_, err := f.svc.CastVote(ctx, f.qc, f.jobReq, f.taskType, f.taskExecID,
		policy, "mallory", "Mallory", types.ApprovalDecisionApproved, "")

	// THEN: permission error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not authorized")
}

func Test_ShouldDenyWhenNilPolicy(t *testing.T) {
	// GIVEN: nil policy (no approval config)
	f := newSvcFixture(t, nil)
	ctx := context.Background()

	// WHEN: anyone tries to vote
	_, err := f.svc.CastVote(ctx, f.qc, f.jobReq, f.taskType, f.taskExecID,
		nil, "anyone", "Anyone", types.ApprovalDecisionApproved, "")

	// THEN: permission error (default-deny)
	require.Error(t, err)
}

func Test_ShouldBlockSyntheticVoterIDFromExternalCaller(t *testing.T) {
	// GIVEN: qc has a real user ID (external authenticated caller)
	f := newSvcFixture(t, nil)
	ctx := context.Background()

	// WHEN: external caller tries to use the synthetic system voter ID
	_, err := f.svc.CastVote(ctx, f.qc, f.jobReq, f.taskType, f.taskExecID,
		openPolicy(1), types.SyntheticVoterID, "System", types.ApprovalDecisionApproved, "")

	// THEN: blocked — system:sla-timeout is reserved
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved for system use")
}

func Test_ShouldBlockSyntheticVoterIDFromSystemContext(t *testing.T) {
	// GIVEN: qc has NO user ID (internal system call — but CastVote is the external API)
	// SLA auto-decisions go through HandleSLABreach → finalizeApproval/finalizeRejection directly,
	// bypassing CastVote entirely. CastVote must always block SyntheticVoterID to prevent
	// impersonation of system actions by any external caller.
	f := newSvcFixture(t, nil)
	ctx := context.Background()

	systemQC := common.NewQueryContextFromIDs("", "")

	// WHEN: system context tries to use synthetic voter ID via CastVote
	_, err := f.svc.CastVote(ctx, systemQC, f.jobReq, f.taskType, f.taskExecID,
		openPolicy(1), types.SyntheticVoterID, "SLA System", types.ApprovalDecisionApproved, "Auto-approved")

	// THEN: always blocked — SLA actions must use HandleSLABreach, not CastVote
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved for system use")
}

func Test_ShouldHandleSLABreachAutoApprove(t *testing.T) {
	// GIVEN
	f := newSvcFixture(t, nil)
	ctx := context.Background()

	// Create a deadline (already past — simulating SLA breach)
	deadline := &types.ApprovalDeadline{
		TaskExecutionID: f.taskExecID,
		JobRequestID:    f.jobReq.ID,
		Deadline:        time.Now().Add(-5 * time.Minute),
		TimeoutAction:   types.TimeoutActionAutoApprove,
	}
	saved, err := f.repo.SaveDeadline(deadline)
	require.NoError(t, err)

	// Verify it appears in breached list before handling
	breached, err := f.repo.FindBreachedDeadlines(time.Now(), 100)
	require.NoError(t, err)
	foundBefore := false
	for _, d := range breached {
		if d.ID == saved.ID {
			foundBefore = true
			break
		}
	}
	require.True(t, foundBefore, "deadline must appear in breached list before handling")

	systemQC := common.NewQueryContextFromIDs("", "")
	f.jobReq.JobState = common.MANUAL_APPROVAL_REQUIRED

	// WHEN
	err = f.svc.HandleSLABreach(ctx, systemQC, saved, f.jobReq, f.taskType, nil)

	// THEN: no error, deadline resolved (no longer appears in breached list)
	require.NoError(t, err)
	breached2, err := f.repo.FindBreachedDeadlines(time.Now(), 100)
	require.NoError(t, err)
	for _, d := range breached2 {
		assert.NotEqual(t, saved.ID, d.ID, "auto-approved deadline must be resolved")
	}
}

func Test_ShouldHandleSLABreachAutoReject(t *testing.T) {
	// GIVEN
	f := newSvcFixture(t, nil)
	ctx := context.Background()

	deadline := &types.ApprovalDeadline{
		TaskExecutionID: f.taskExecID,
		JobRequestID:    f.jobReq.ID,
		Deadline:        time.Now().Add(-1 * time.Minute),
		TimeoutAction:   types.TimeoutActionAutoReject,
	}
	saved, err := f.repo.SaveDeadline(deadline)
	require.NoError(t, err)

	systemQC := common.NewQueryContextFromIDs("", "")
	f.jobReq.JobState = common.MANUAL_APPROVAL_REQUIRED

	// WHEN
	err = f.svc.HandleSLABreach(ctx, systemQC, saved, f.jobReq, f.taskType, nil)

	// THEN: no error, deadline resolved (no longer appears in breached list)
	require.NoError(t, err)
	breached, err := f.repo.FindBreachedDeadlines(time.Now(), 100)
	require.NoError(t, err)
	for _, d := range breached {
		assert.NotEqual(t, saved.ID, d.ID, "auto-rejected deadline must be resolved")
	}
}

func Test_ShouldHandleSLABreachEscalate(t *testing.T) {
	// GIVEN
	f := newSvcFixture(t, nil)
	ctx := context.Background()

	deadline := &types.ApprovalDeadline{
		TaskExecutionID:      f.taskExecID,
		JobRequestID:         f.jobReq.ID,
		Deadline:             time.Now().Add(-2 * time.Minute),
		TimeoutAction:        types.TimeoutActionEscalate,
		EscalationRecipients: "ops@example.com",
	}
	saved, err := f.repo.SaveDeadline(deadline)
	require.NoError(t, err)

	systemQC := common.NewQueryContextFromIDs("", "")

	// Track notification calls
	var notified []string
	notifyFn := func(recipients []string, _ string) error {
		notified = append(notified, recipients...)
		return nil
	}

	// WHEN
	err = f.svc.HandleSLABreach(ctx, systemQC, saved, f.jobReq, f.taskType, notifyFn)

	// THEN: escalation sent, deadline marked escalated (not resolved)
	require.NoError(t, err)
	assert.Contains(t, notified, "ops@example.com")

	// Verify deadline is escalated, not resolved, and not in breached list
	breached, err := f.repo.FindBreachedDeadlines(time.Now(), 100)
	require.NoError(t, err)
	for _, d := range breached {
		assert.NotEqual(t, saved.ID, d.ID, "escalated deadline must not reappear")
	}
}

func Test_ShouldNotCreateDeadlineWhenSLAIsZero(t *testing.T) {
	// GIVEN: policy with SLADeadline == 0
	f := newSvcFixture(t, nil)
	policy := &types.ApprovalPolicy{MinApprovals: 1, SLADeadline: 0}

	// WHEN
	err := f.svc.CreateDeadlineIfNeeded(f.taskExecID, f.jobReq.ID, policy)

	// THEN: no deadline row created
	require.NoError(t, err)
	breached, err := f.repo.FindBreachedDeadlines(time.Now().Add(time.Hour), 100)
	require.NoError(t, err)
	for _, d := range breached {
		assert.NotEqual(t, f.taskExecID, d.TaskExecutionID)
	}
}

func Test_ShouldCreateDeadlineWhenSLAIsConfigured(t *testing.T) {
	// GIVEN: policy with 2h SLA
	f := newSvcFixture(t, nil)
	policy := &types.ApprovalPolicy{
		MinApprovals:  1,
		SLADeadline:   2 * time.Hour,
		TimeoutAction: types.TimeoutActionEscalate,
	}

	// WHEN
	err := f.svc.CreateDeadlineIfNeeded(f.taskExecID, f.jobReq.ID, policy)

	// THEN: deadline row created (deadline is ~2h from now, not breached yet)
	require.NoError(t, err)

	// Verify deadline exists in DB by querying far in the future
	farFuture := time.Now().Add(3 * time.Hour)
	all, err := f.repo.FindBreachedDeadlines(farFuture, 100)
	require.NoError(t, err)
	found := false
	for _, d := range all {
		if d.TaskExecutionID == f.taskExecID {
			found = true
			assert.Equal(t, types.TimeoutActionEscalate, d.TimeoutAction)
			break
		}
	}
	assert.True(t, found, "deadline must be persisted for task with SLA policy")
}

func Test_ShouldGetStatusReturnsCurrentTally(t *testing.T) {
	// GIVEN: 2 approvals cast via raw repo
	f := newSvcFixture(t, nil)
	policy := userPolicy("alice,bob,carol", 2)

	for _, voter := range []string{"alice", "bob"} {
		_, err := f.repo.SaveVote(&types.ApprovalVote{
			TaskExecutionID: f.taskExecID,
			JobRequestID:    f.jobReq.ID,
			VoterID:         voter,
			Decision:        types.ApprovalDecisionApproved,
			VotedAt:         time.Now(),
		})
		require.NoError(t, err)
	}

	// WHEN
	status, err := f.svc.GetStatus(f.taskExecID, f.jobReq.ID, policy)

	// THEN
	require.NoError(t, err)
	assert.Equal(t, 2, status.ApprovalsReceived)
	assert.Equal(t, 0, status.RejectionsReceived)
	assert.True(t, status.QuorumReached)
	assert.Len(t, status.Votes, 2)
}
