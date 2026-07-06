// SPDX-License-Identifier: AGPL-3.0-or-later

package manager

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/approval"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/notify"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/resource"
	"plexobject.com/formicary/queen/stats"
	"plexobject.com/formicary/queen/types"
)

// ─── test helpers ─────────────────────────────────────────────────────────────

// newApprovalJobManager creates a JobManager wired with a real approval.Service backed by SQLite.
func newApprovalJobManager(t *testing.T) (*JobManager, *repository.JobRequestRepositoryImpl, *approval.Service) {
	t.Helper()
	serverCfg := config.TestServerConfig()

	locator, err := repository.NewTestLocator()
	require.NoError(t, err)

	approvalRepo, err := approval.NewRepositoryImpl(locator.DB)
	require.NoError(t, err)
	approvalSvc := approval.NewService(locator.DB, approvalRepo)

	mgr, err := newJobManagerWithApproval(serverCfg, approvalSvc)
	require.NoError(t, err)

	jobRequestRepo, err := repository.NewTestJobRequestRepository()
	require.NoError(t, err)

	return mgr, jobRequestRepo, approvalSvc
}

// buildApprovalJobDef creates and saves a job definition with a MANUAL task that has
// the given approval policy. Returns the saved definition.
func buildApprovalJobDef(t *testing.T, qc *common.QueryContext, jm *JobManager,
	jobName string, policy *types.ApprovalPolicy) *types.JobDefinition {
	t.Helper()

	job := types.NewJobDefinition("io.formicary.test." + jobName)
	job.UserID = qc.GetUserID()
	job.OrganizationID = qc.GetOrganizationID()

	// Task 0: normal shell task
	prep := types.NewTaskDefinition("prepare", common.Shell)
	prep.Script = []string{"echo prepare"}
	prep.OnExitCode["completed"] = "approval-gate"
	job.AddTask(prep)

	// Task 1: MANUAL approval gate
	gate := types.NewTaskDefinition("approval-gate", common.Manual)
	gate.ApprovalPolicy = policy
	job.AddTask(gate)

	job.UpdateRawYaml()

	saved, err := jm.SaveJobDefinition(qc, job)
	require.NoError(t, err)
	return saved
}

// createRequestInApprovalState saves a job request, a job execution, and marks the approval
// task as MANUAL_APPROVAL_REQUIRED so CastApprovalVote can operate on it.
func createRequestInApprovalState(t *testing.T,
	qc *common.QueryContext,
	jm *JobManager,
	jobDef *types.JobDefinition,
	locator *repository.Locator,
) (*types.JobRequest, *types.TaskExecution) {
	t.Helper()

	req, err := types.NewJobRequestFromDefinition(jobDef)
	require.NoError(t, err)
	req.OrganizationID = qc.GetOrganizationID()
	req.UserID = qc.GetUserID()
	req.UserKey = ulid.Make().String()

	savedReq, err := jm.SaveJobRequest(qc, req)
	require.NoError(t, err)

	// Build and save a job execution with all tasks.
	jobExec := types.NewJobExecution(savedReq.ToInfo())
	for _, td := range jobDef.Tasks {
		jobExec.AddTask(td)
	}
	savedExec, err := locator.JobExecutionRepository.Save(jobExec)
	require.NoError(t, err)

	// Wire job_request → job_execution and set MANUAL_APPROVAL_REQUIRED state.
	err = locator.DB.Table("formicary_job_requests").
		Where("id = ?", savedReq.ID).
		Updates(map[string]interface{}{
			"job_execution_id": savedExec.ID,
			"job_state":        string(common.MANUAL_APPROVAL_REQUIRED),
		}).Error
	require.NoError(t, err)

	// Mark the approval-gate task as awaiting approval.
	var approvalTaskExec *types.TaskExecution
	for _, te := range savedExec.Tasks {
		if te.TaskType == "approval-gate" {
			approvalTaskExec = te
			break
		}
	}
	require.NotNil(t, approvalTaskExec, "approval-gate task must exist in job execution")

	err = locator.DB.Table("formicary_task_executions").
		Where("id = ?", approvalTaskExec.ID).
		Update("task_state", string(common.MANUAL_APPROVAL_REQUIRED)).Error
	require.NoError(t, err)

	// Reload so the state is reflected in memory.
	savedReq.JobState = common.MANUAL_APPROVAL_REQUIRED
	savedReq.JobExecutionID = savedExec.ID

	return savedReq, approvalTaskExec
}

// ─── integration tests ────────────────────────────────────────────────────────

func Test_ShouldApprovalInteg_SingleVoteReachesQuorum(t *testing.T) {
	// GIVEN: job with approval gate requiring 1 vote, no voter restriction
	jm, _, _ := newApprovalJobManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	locator, err := repository.NewTestLocator()
	require.NoError(t, err)

	policy := &types.ApprovalPolicy{MinApprovals: 1}
	jobDef := buildApprovalJobDef(t, qc, jm, fmt.Sprintf("single-vote-%d", time.Now().UnixNano()), policy)
	savedReq, _ := createRequestInApprovalState(t, qc, jm, jobDef, locator)

	// WHEN: alice votes APPROVED
	voteReq := &types.ApprovalVoteRequest{
		RequestID: savedReq.ID,
		TaskType:  "approval-gate",
		VoterID:   qc.GetUserID(),
		Decision:  types.ApprovalDecisionApproved,
		Comments:  "Looks good",
	}
	status, err := jm.CastApprovalVote(context.Background(), qc, voteReq)

	// THEN: quorum reached, no error
	require.NoError(t, err)
	assert.True(t, status.QuorumReached)
	assert.False(t, status.Rejected)
	assert.Equal(t, 1, status.ApprovalsReceived)
}

func Test_ShouldApprovalInteg_RequiresTwoVotes(t *testing.T) {
	// GIVEN: policy requires 2 approvals, 3 allowed users
	jm, _, _ := newApprovalJobManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	locator, err := repository.NewTestLocator()
	require.NoError(t, err)

	policy := &types.ApprovalPolicy{
		MinApprovals: 2,
		AllowedUsers: fmt.Sprintf("%s,bob,carol", qc.GetUserID()),
	}
	jobDef := buildApprovalJobDef(t, qc, jm, fmt.Sprintf("two-vote-%d", time.Now().UnixNano()), policy)
	savedReq, _ := createRequestInApprovalState(t, qc, jm, jobDef, locator)
	ctx := context.Background()

	// WHEN: first vote — not enough yet
	vote1 := &types.ApprovalVoteRequest{
		RequestID: savedReq.ID,
		TaskType:  "approval-gate",
		VoterID:   qc.GetUserID(),
		Decision:  types.ApprovalDecisionApproved,
		Comments:  "",
	}
	status1, err := jm.CastApprovalVote(ctx, qc, vote1)
	require.NoError(t, err)
	assert.False(t, status1.QuorumReached, "one vote is not enough")

	// WHEN: second vote from bob
	bobQC := common.NewQueryContextFromIDs("bob", qc.GetOrganizationID())
	vote2 := &types.ApprovalVoteRequest{
		RequestID: savedReq.ID,
		TaskType:  "approval-gate",
		VoterID:   "bob",
		Decision:  types.ApprovalDecisionApproved,
		Comments:  "",
	}
	status2, err := jm.CastApprovalVote(ctx, bobQC, vote2)
	require.NoError(t, err)
	assert.True(t, status2.QuorumReached, "two votes should reach quorum")
	assert.Equal(t, 2, status2.ApprovalsReceived)
}

func Test_ShouldApprovalInteg_RejectionWithUnanimousPolicy(t *testing.T) {
	// GIVEN: unanimous policy — any rejection fails
	jm, _, _ := newApprovalJobManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	locator, err := repository.NewTestLocator()
	require.NoError(t, err)

	policy := &types.ApprovalPolicy{
		MinApprovals:     2,
		AllowedUsers:     fmt.Sprintf("%s,bob,carol", qc.GetUserID()),
		RequireUnanimous: true,
	}
	jobDef := buildApprovalJobDef(t, qc, jm, fmt.Sprintf("unanimous-%d", time.Now().UnixNano()), policy)
	savedReq, _ := createRequestInApprovalState(t, qc, jm, jobDef, locator)

	// WHEN: carol rejects immediately
	carolQC := common.NewQueryContextFromIDs("carol", qc.GetOrganizationID())
	voteReq := &types.ApprovalVoteRequest{
		RequestID: savedReq.ID,
		TaskType:  "approval-gate",
		VoterID:   "carol",
		Decision:  types.ApprovalDecisionRejected,
		Comments:  "Not ready for prod",
	}
	status, err := jm.CastApprovalVote(context.Background(), carolQC, voteReq)

	// THEN: rejected immediately
	require.NoError(t, err)
	assert.True(t, status.Rejected)
	assert.False(t, status.QuorumReached)
}

func Test_ShouldApprovalInteg_DuplicateVoteIsIdempotent(t *testing.T) {
	// GIVEN: open policy
	jm, _, _ := newApprovalJobManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	locator, err := repository.NewTestLocator()
	require.NoError(t, err)

	policy := &types.ApprovalPolicy{
		MinApprovals: 2,
		AllowedUsers: fmt.Sprintf("%s,bob,carol", qc.GetUserID()),
	}
	jobDef := buildApprovalJobDef(t, qc, jm, fmt.Sprintf("dup-vote-%d", time.Now().UnixNano()), policy)
	savedReq, _ := createRequestInApprovalState(t, qc, jm, jobDef, locator)
	ctx := context.Background()

	voteReq := &types.ApprovalVoteRequest{
		RequestID: savedReq.ID,
		TaskType:  "approval-gate",
		VoterID:   qc.GetUserID(),
		Decision:  types.ApprovalDecisionApproved,
	}

	// First vote
	_, err = jm.CastApprovalVote(ctx, qc, voteReq)
	require.NoError(t, err)

	// WHEN: same vote again (different decision this time)
	voteReq.Decision = types.ApprovalDecisionRejected
	status, err := jm.CastApprovalVote(ctx, qc, voteReq)

	// THEN: no error, original approval preserved
	require.NoError(t, err)
	assert.Equal(t, 1, status.ApprovalsReceived, "duplicate vote must not overwrite original")
	assert.Equal(t, 0, status.RejectionsReceived)
}

func Test_ShouldApprovalInteg_UnauthorizedVoterDenied(t *testing.T) {
	// GIVEN: policy allows only alice and bob
	jm, _, _ := newApprovalJobManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	locator, err := repository.NewTestLocator()
	require.NoError(t, err)

	policy := &types.ApprovalPolicy{
		MinApprovals: 1,
		AllowedUsers: "alice,bob",
	}
	jobDef := buildApprovalJobDef(t, qc, jm, fmt.Sprintf("unauth-%d", time.Now().UnixNano()), policy)
	savedReq, _ := createRequestInApprovalState(t, qc, jm, jobDef, locator)

	// WHEN: mallory (not in list) tries to vote
	malloryQC := common.NewQueryContextFromIDs("mallory", qc.GetOrganizationID())
	voteReq := &types.ApprovalVoteRequest{
		RequestID: savedReq.ID,
		TaskType:  "approval-gate",
		VoterID:   "mallory",
		Decision:  types.ApprovalDecisionApproved,
	}
	_, err = jm.CastApprovalVote(context.Background(), malloryQC, voteReq)

	// THEN: permission error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not authorized")
}

func Test_ShouldApprovalInteg_GetApprovalStatus(t *testing.T) {
	// GIVEN: one vote cast
	jm, _, _ := newApprovalJobManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	locator, err := repository.NewTestLocator()
	require.NoError(t, err)

	policy := &types.ApprovalPolicy{
		MinApprovals: 2,
		AllowedUsers: fmt.Sprintf("%s,bob,carol", qc.GetUserID()),
	}
	jobDef := buildApprovalJobDef(t, qc, jm, fmt.Sprintf("status-%d", time.Now().UnixNano()), policy)
	savedReq, _ := createRequestInApprovalState(t, qc, jm, jobDef, locator)
	ctx := context.Background()

	voteReq := &types.ApprovalVoteRequest{
		RequestID: savedReq.ID,
		TaskType:  "approval-gate",
		VoterID:   qc.GetUserID(),
		Decision:  types.ApprovalDecisionApproved,
	}
	_, err = jm.CastApprovalVote(ctx, qc, voteReq)
	require.NoError(t, err)

	// WHEN: get status
	status, err := jm.GetApprovalStatus(ctx, qc, savedReq.ID, "approval-gate")

	// THEN: tally reflects the one vote
	require.NoError(t, err)
	assert.Equal(t, 1, status.ApprovalsReceived)
	assert.Equal(t, 0, status.RejectionsReceived)
	assert.False(t, status.QuorumReached, "need 2, only have 1")
	assert.Len(t, status.Votes, 1)
}

func Test_ShouldApprovalInteg_ListPendingApprovals(t *testing.T) {
	// GIVEN: a job request with an approval-gate task in MANUAL_APPROVAL_REQUIRED
	jm, _, _ := newApprovalJobManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	locator, err := repository.NewTestLocator()
	require.NoError(t, err)

	policy := &types.ApprovalPolicy{MinApprovals: 1}
	jobDef := buildApprovalJobDef(t, qc, jm, fmt.Sprintf("list-pending-%d", time.Now().UnixNano()), policy)
	savedReq, _ := createRequestInApprovalState(t, qc, jm, jobDef, locator)

	// WHEN
	pending, total, err := jm.ListPendingApprovals(context.Background(), qc, 0, 20)

	// THEN
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))

	found := false
	for _, p := range pending {
		if p.JobRequestID == savedReq.ID {
			found = true
			assert.Equal(t, "approval-gate", p.TaskType)
			break
		}
	}
	assert.True(t, found, "pending approvals must include the waiting task")
}

func Test_ShouldApprovalInteg_SLADeadlineCreatedAndFound(t *testing.T) {
	// GIVEN: policy with 2h SLA
	jm, _, approvalSvc := newApprovalJobManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	locator, err := repository.NewTestLocator()
	require.NoError(t, err)

	policy := &types.ApprovalPolicy{
		MinApprovals:  1,
		SLADeadline:   2 * time.Hour,
		TimeoutAction: types.TimeoutActionEscalate,
	}
	jobDef := buildApprovalJobDef(t, qc, jm, fmt.Sprintf("sla-deadline-%d", time.Now().UnixNano()), policy)
	_, approvalTask := createRequestInApprovalState(t, qc, jm, jobDef, locator)

	req := &types.JobRequest{ID: ulid.Make().String()}

	// WHEN: create deadline via service
	err = approvalSvc.CreateDeadlineIfNeeded(approvalTask.ID, req.ID, policy)
	require.NoError(t, err)

	// THEN: deadline exists (not yet breached since 2h in the future)
	farFuture := time.Now().Add(3 * time.Hour)
	approvalRepo, repoErr := approval.NewRepositoryImpl(locator.DB)
	require.NoError(t, repoErr)

	all, err := approvalRepo.FindBreachedDeadlines(farFuture, 100)
	require.NoError(t, err)

	found := false
	for _, d := range all {
		if d.TaskExecutionID == approvalTask.ID {
			found = true
			assert.Equal(t, types.TimeoutActionEscalate, d.TimeoutAction)
			break
		}
	}
	assert.True(t, found, "SLA deadline must be persisted")
}

func Test_ShouldApprovalInteg_SLABreachAutoApproves(t *testing.T) {
	// GIVEN: policy with SLA, deadline already in the past
	jm, _, approvalSvc := newApprovalJobManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	locator, err := repository.NewTestLocator()
	require.NoError(t, err)

	policy := &types.ApprovalPolicy{MinApprovals: 1}
	jobDef := buildApprovalJobDef(t, qc, jm, fmt.Sprintf("sla-breach-%d", time.Now().UnixNano()), policy)
	savedReq, approvalTask := createRequestInApprovalState(t, qc, jm, jobDef, locator)

	approvalRepo, err := approval.NewRepositoryImpl(locator.DB)
	require.NoError(t, err)

	// Insert a deadline already expired
	deadline := &types.ApprovalDeadline{
		TaskExecutionID: approvalTask.ID,
		JobRequestID:    savedReq.ID,
		Deadline:        time.Now().Add(-5 * time.Minute),
		TimeoutAction:   types.TimeoutActionAutoApprove,
	}
	savedDL, err := approvalRepo.SaveDeadline(deadline)
	require.NoError(t, err)

	// Verify it's in the breached list
	breached, err := approvalRepo.FindBreachedDeadlines(time.Now(), 100)
	require.NoError(t, err)
	foundBreached := false
	for _, d := range breached {
		if d.ID == savedDL.ID {
			foundBreached = true
			break
		}
	}
	require.True(t, foundBreached, "expired deadline must appear in breached list")

	// WHEN: handle the SLA breach
	systemQC := common.NewQueryContextFromIDs("", "")
	savedReq.JobState = common.MANUAL_APPROVAL_REQUIRED
	err = approvalSvc.HandleSLABreach(context.Background(), systemQC, savedDL, savedReq, "approval-gate", nil)
	require.NoError(t, err)

	// THEN: deadline is resolved (no longer in breached list)
	breached2, err := approvalRepo.FindBreachedDeadlines(time.Now(), 100)
	require.NoError(t, err)
	for _, d := range breached2 {
		assert.NotEqual(t, savedDL.ID, d.ID, "resolved deadline must not reappear")
	}
}

// ─── open-policy tests (the "miac" bug scenario) ──────────────────────────────

// Test_ShouldApprovalInteg_OpenPolicy_AnyUserCanVote verifies that when no
// allowed_users / allowed_roles are configured ANY authenticated user may cast
// a vote — this was the "miac is not authorized" production bug.
func Test_ShouldApprovalInteg_OpenPolicy_AnyUserCanVote(t *testing.T) {
	jm, _, _ := newApprovalJobManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	locator, err := repository.NewTestLocator()
	require.NoError(t, err)

	// Open policy — no user/role restrictions
	policy := &types.ApprovalPolicy{MinApprovals: 1}
	jobDef := buildApprovalJobDef(t, qc, jm,
		fmt.Sprintf("open-policy-%d", time.Now().UnixNano()), policy)
	savedReq, _ := createRequestInApprovalState(t, qc, jm, jobDef, locator)

	// A completely unrelated user "miac" — not the job owner, no special role
	miacQC := common.NewQueryContextFromIDs("miac", qc.GetOrganizationID())

	status, err := jm.CastApprovalVote(context.Background(), miacQC, &types.ApprovalVoteRequest{
		RequestID: savedReq.ID,
		TaskType:  "approval-gate",
		VoterID:   "miac",
		Decision:  types.ApprovalDecisionApproved,
		Comments:  "looks fine",
	})

	require.NoError(t, err, "open policy must allow any authenticated user to vote")
	assert.True(t, status.QuorumReached)
	assert.Equal(t, 1, status.ApprovalsReceived)
}

// Test_ShouldApprovalInteg_OpenPolicy_MultiVoters verifies 2-of-N quorum with
// open policy — multiple real users (none pre-listed) can each contribute a vote.
func Test_ShouldApprovalInteg_OpenPolicy_MultiVoters(t *testing.T) {
	jm, _, _ := newApprovalJobManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	locator, err := repository.NewTestLocator()
	require.NoError(t, err)

	// Open 2-vote policy — mimics multi-party-approval.yaml after the fix
	policy := &types.ApprovalPolicy{MinApprovals: 2}
	jobDef := buildApprovalJobDef(t, qc, jm,
		fmt.Sprintf("open-multi-%d", time.Now().UnixNano()), policy)
	savedReq, _ := createRequestInApprovalState(t, qc, jm, jobDef, locator)
	ctx := context.Background()

	voters := []string{"miac", "priya"}
	for i, voter := range voters {
		voterQC := common.NewQueryContextFromIDs(voter, qc.GetOrganizationID())
		status, voteErr := jm.CastApprovalVote(ctx, voterQC, &types.ApprovalVoteRequest{
			RequestID: savedReq.ID,
			TaskType:  "approval-gate",
			VoterID:   voter,
			Decision:  types.ApprovalDecisionApproved,
		})
		require.NoError(t, voteErr, "vote %d by %s must succeed on open policy", i+1, voter)

		if i < len(voters)-1 {
			assert.False(t, status.QuorumReached, "quorum must not be reached before all required votes")
		} else {
			assert.True(t, status.QuorumReached, "quorum must be reached after %d votes", len(voters))
		}
	}
}

// Test_ShouldApprovalInteg_YAMLRoundTrip_PolicySurvivesReload verifies that the
// ApprovalPolicy (gorm:"-") is correctly hydrated after a job definition is
// saved to and reloaded from the database — this is the root cause of the
// "no approval policy configured" error class.
func Test_ShouldApprovalInteg_YAMLRoundTrip_PolicySurvivesReload(t *testing.T) {
	jm, _, _ := newApprovalJobManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	policy := &types.ApprovalPolicy{
		MinApprovals: 2,
		AllowedUsers: "alice,bob,carol",
	}
	savedDef := buildApprovalJobDef(t, qc, jm,
		fmt.Sprintf("yaml-rt-%d", time.Now().UnixNano()), policy)

	// Reload from the database through the manager (exercises postProcessJob path)
	reloaded, err := jm.GetJobDefinitionByType(qc, savedDef.JobType, "")
	require.NoError(t, err)

	// Find the approval-gate task and verify the policy was reloaded
	var gateTask *types.TaskDefinition
	for _, td := range reloaded.Tasks {
		if td.TaskType == "approval-gate" {
			gateTask = td
			break
		}
	}
	require.NotNil(t, gateTask, "approval-gate task must exist after reload")
	require.NotNil(t, gateTask.ApprovalPolicy, "ApprovalPolicy must be non-nil after reload from YAML")
	assert.Equal(t, 2, gateTask.ApprovalPolicy.MinApprovals)
	assert.Equal(t, "alice,bob,carol", gateTask.ApprovalPolicy.AllowedUsers)
}

// Test_ShouldApprovalInteg_AdminRoleOverridesUserList verifies that a user with
// the Admin system role can vote even when they are not listed in allowed_users.
func Test_ShouldApprovalInteg_AdminRoleOverridesUserList(t *testing.T) {
	jm, _, _ := newApprovalJobManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	locator, err := repository.NewTestLocator()
	require.NoError(t, err)

	// Policy restricts to alice,bob only
	policy := &types.ApprovalPolicy{
		MinApprovals: 1,
		AllowedRoles: "Admin",
		AllowedUsers: "alice,bob",
	}
	jobDef := buildApprovalJobDef(t, qc, jm,
		fmt.Sprintf("admin-role-%d", time.Now().UnixNano()), policy)
	savedReq, _ := createRequestInApprovalState(t, qc, jm, jobDef, locator)

	// "miac" is not in alice,bob — but has Admin role
	adminUser := common.NewUser(qc.GetOrganizationID(), "miac", "miac", "miac@example.com", acl.NewRolesWithAdmin())
	adminUser.ID = "miac"
	adminQC := common.NewQueryContext(adminUser, "")

	status, err := jm.CastApprovalVote(context.Background(), adminQC, &types.ApprovalVoteRequest{
		RequestID: savedReq.ID,
		TaskType:  "approval-gate",
		VoterID:   "miac",
		Decision:  types.ApprovalDecisionApproved,
	})

	require.NoError(t, err, "Admin role must allow voting even when not in allowed_users")
	assert.True(t, status.QuorumReached)
}

// Test_ShouldApprovalInteg_RejectionWithOpenPolicy verifies that a rejection by
// any user (open policy) with require_unanimous=true immediately fails the gate.
func Test_ShouldApprovalInteg_RejectionWithOpenPolicy(t *testing.T) {
	jm, _, _ := newApprovalJobManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	locator, err := repository.NewTestLocator()
	require.NoError(t, err)

	policy := &types.ApprovalPolicy{
		MinApprovals:     2,
		RequireUnanimous: true,
	}
	jobDef := buildApprovalJobDef(t, qc, jm,
		fmt.Sprintf("open-reject-%d", time.Now().UnixNano()), policy)
	savedReq, _ := createRequestInApprovalState(t, qc, jm, jobDef, locator)

	miacQC := common.NewQueryContextFromIDs("miac", qc.GetOrganizationID())
	status, err := jm.CastApprovalVote(context.Background(), miacQC, &types.ApprovalVoteRequest{
		RequestID: savedReq.ID,
		TaskType:  "approval-gate",
		VoterID:   "miac",
		Decision:  types.ApprovalDecisionRejected,
		Comments:  "blocking this deploy",
	})

	require.NoError(t, err)
	assert.True(t, status.Rejected)
	assert.False(t, status.QuorumReached)
}

// Test_ShouldApprovalInteg_VoterIDEmptyRejected verifies that an empty voter_id
// is rejected regardless of policy — prevents anonymous votes.
func Test_ShouldApprovalInteg_VoterIDEmptyRejected(t *testing.T) {
	jm, _, _ := newApprovalJobManager(t)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	locator, err := repository.NewTestLocator()
	require.NoError(t, err)

	policy := &types.ApprovalPolicy{MinApprovals: 1}
	jobDef := buildApprovalJobDef(t, qc, jm,
		fmt.Sprintf("empty-voter-%d", time.Now().UnixNano()), policy)
	savedReq, _ := createRequestInApprovalState(t, qc, jm, jobDef, locator)

	_, err = jm.CastApprovalVote(context.Background(), qc, &types.ApprovalVoteRequest{
		RequestID: savedReq.ID,
		TaskType:  "approval-gate",
		VoterID:   "", // empty — must be rejected
		Decision:  types.ApprovalDecisionApproved,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "voter_id is required")
}

// ─── helper: TestJobManager variant with approval service ─────────────────────

func newJobManagerWithApproval(serverCfg *config.ServerConfig, approvalSvc *approval.Service) (*JobManager, error) {
	if err := serverCfg.Validate(); err != nil {
		return nil, err
	}

	auditRepo, err := repository.NewTestAuditRecordRepository()
	if err != nil {
		return nil, err
	}
	jobDefRepo, err := repository.NewTestJobDefinitionRepository()
	if err != nil {
		return nil, err
	}
	jobReqRepo, err := repository.NewTestJobRequestRepository()
	if err != nil {
		return nil, err
	}
	jobExecRepo, err := repository.NewTestJobExecutionRepository()
	if err != nil {
		return nil, err
	}
	emailVerifRepo, err := repository.NewTestEmailVerificationRepository()
	if err != nil {
		return nil, err
	}
	logRepo, err := repository.NewTestLogEventRepository()
	if err != nil {
		return nil, err
	}

	artifactManager, err := TestArtifactManager(serverCfg)
	if err != nil {
		return nil, err
	}

	notifier, err := notify.New(serverCfg, logRepo, emailVerifRepo)
	if err != nil {
		return nil, err
	}

	userManager, err := TestUserManager(serverCfg)
	if err != nil {
		return nil, err
	}

	queueClient, err := queue.NewClientManager().GetClient(context.Background(), &serverCfg.Common)
	if err != nil {
		return nil, err
	}

	resourceManager := resource.New(serverCfg, queueClient)

	return NewJobManager(
		context.Background(),
		serverCfg,
		auditRepo,
		jobDefRepo,
		jobReqRepo,
		jobExecRepo,
		userManager,
		resourceManager,
		artifactManager,
		stats.NewJobStatsRegistry(),
		metrics.New(),
		queueClient,
		notifier,
		approvalSvc,
		nil, // no scheduler in tests
	)
}
