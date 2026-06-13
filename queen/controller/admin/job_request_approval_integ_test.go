// SPDX-License-Identifier: AGPL-3.0-or-later

package admin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/approval"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"

	common "plexobject.com/formicary/internal/types"
)

// ─── fixtures ────────────────────────────────────────────────────────────────

type approvalControllerFixture struct {
	ctrl     *JobRequestAdminController
	jm       *manager.JobManager
	locator  *repository.Locator
	qc       *common.QueryContext
	jobDef   *types.JobDefinition
	savedReq *types.JobRequest
	taskExec *types.TaskExecution
}

func newApprovalControllerFixture(t *testing.T, policy *types.ApprovalPolicy) *approvalControllerFixture {
	t.Helper()

	locator, err := repository.NewTestLocator()
	require.NoError(t, err)

	approvalRepo, err := approval.NewRepositoryImpl(locator.DB)
	require.NoError(t, err)
	approvalSvc := approval.NewService(locator.DB, approvalRepo)

	// queen/controller/admin is 3 levels from repo root; templates live at root/public/views/notify
	jm, err := manager.NewTestJobManagerWithApproval(approvalSvc, "../../../public/views/notify")
	require.NoError(t, err)

	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	jobName := fmt.Sprintf("ctrl-approval-%d", time.Now().UnixNano())
	jobDef := buildCtrlApprovalJobDef(t, qc, jm, jobName, policy)
	savedReq, taskExec := createCtrlRequestInApprovalState(t, qc, jm, jobDef, locator)

	webServer := web.NewStubWebServer()
	ctrl := NewJobRequestAdminController(jm, webServer)

	return &approvalControllerFixture{
		ctrl:     ctrl,
		jm:       jm,
		locator:  locator,
		qc:       qc,
		jobDef:   jobDef,
		savedReq: savedReq,
		taskExec: taskExec,
	}
}

func buildCtrlApprovalJobDef(t *testing.T, qc *common.QueryContext, jm *manager.JobManager,
	jobName string, policy *types.ApprovalPolicy) *types.JobDefinition {
	t.Helper()

	job := types.NewJobDefinition("io.formicary.test." + jobName)
	job.UserID = qc.GetUserID()
	job.OrganizationID = qc.GetOrganizationID()

	prep := types.NewTaskDefinition("prepare", common.Shell)
	prep.Script = []string{"echo prepare"}
	prep.OnExitCode["completed"] = "approval-gate"
	job.AddTask(prep)

	gate := types.NewTaskDefinition("approval-gate", common.Manual)
	gate.ApprovalPolicy = policy
	job.AddTask(gate)

	job.UpdateRawYaml()
	saved, err := jm.SaveJobDefinition(qc, job)
	require.NoError(t, err)
	return saved
}

func createCtrlRequestInApprovalState(t *testing.T, qc *common.QueryContext,
	jm *manager.JobManager, jobDef *types.JobDefinition,
	locator *repository.Locator) (*types.JobRequest, *types.TaskExecution) {
	t.Helper()

	req, err := types.NewJobRequestFromDefinition(jobDef)
	require.NoError(t, err)
	req.OrganizationID = qc.GetOrganizationID()
	req.UserID = qc.GetUserID()
	req.UserKey = ulid.Make().String()

	savedReq, err := jm.SaveJobRequest(qc, req)
	require.NoError(t, err)

	jobExec := types.NewJobExecution(savedReq.ToInfo())
	for _, td := range jobDef.Tasks {
		jobExec.AddTask(td)
	}
	savedExec, err := locator.JobExecutionRepository.Save(jobExec)
	require.NoError(t, err)

	err = locator.DB.Table("formicary_job_requests").
		Where("id = ?", savedReq.ID).
		Updates(map[string]interface{}{
			"job_execution_id": savedExec.ID,
			"job_state":        string(common.MANUAL_APPROVAL_REQUIRED),
		}).Error
	require.NoError(t, err)

	var approvalTaskExec *types.TaskExecution
	for _, te := range savedExec.Tasks {
		if te.TaskType == "approval-gate" {
			approvalTaskExec = te
			break
		}
	}
	require.NotNil(t, approvalTaskExec, "approval-gate task must exist")

	err = locator.DB.Table("formicary_task_executions").
		Where("id = ?", approvalTaskExec.ID).
		Update("task_state", string(common.MANUAL_APPROVAL_REQUIRED)).Error
	require.NoError(t, err)

	savedReq.JobState = common.MANUAL_APPROVAL_REQUIRED
	savedReq.JobExecutionID = savedExec.ID
	return savedReq, approvalTaskExec
}

// stubVoteCtx builds a StubContext ready for voteOnApproval.
// StubContext.FormValue(name) reads from ctx.Params[name], so form values go there.
func stubVoteCtx(requestID, taskType string, formValues map[string]string) web.APIContext {
	req := &http.Request{Body: io.NopCloser(strings.NewReader("")), URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	ctx.Params["id"] = requestID
	ctx.Params["taskType"] = taskType
	for k, v := range formValues {
		ctx.Params[k] = v
	}
	return ctx
}

// requireVoteSuccess asserts that the vote handler completed without a real error.
// The stub server returns "302 u /path" for successful redirects — treat as OK.
func requireVoteSuccess(t *testing.T, err error) {
	t.Helper()
	if err != nil && !strings.HasPrefix(err.Error(), "302 ") {
		require.NoError(t, err)
	}
}

// userWithSameOrg returns a User that belongs to the same org as qc but with a different identity.
// This lets the user find the job request (org matches) while carrying a different voter_id.
func userWithSameOrg(qc *common.QueryContext, userID string) *common.User {
	u := &common.User{}
	u.ID = userID
	u.Username = userID
	u.OrganizationID = qc.GetOrganizationID()
	org := common.NewOrganization("", "", "")
	org.ID = qc.GetOrganizationID()
	u.Organization = org
	return u
}

// ─── dashboard vote endpoint tests ───────────────────────────────────────────

func Test_ShouldDashboardVote_AuthEnabled_UserID(t *testing.T) {
	// GIVEN: open policy (no voter restrictions), authenticated session
	policy := &types.ApprovalPolicy{MinApprovals: 1}
	f := newApprovalControllerFixture(t, policy)

	ctx := stubVoteCtx(f.savedReq.ID, "approval-gate", map[string]string{
		"decision": "APPROVED",
		"comments": "looks good",
	})
	// Pass the full session user so BuildQueryContext resolves the correct org
	ctx.Set(web.DBUser, f.qc.User)

	// WHEN: vote submitted via dashboard
	err := f.ctrl.voteOnApproval(ctx)

	// THEN: redirect (302) — success
	requireVoteSuccess(t, err)
}

func Test_ShouldDashboardVote_AuthDisabled_FormVoterID(t *testing.T) {
	// GIVEN: open policy, auth-DISABLED — voter_id taken from form field
	policy := &types.ApprovalPolicy{MinApprovals: 1}
	f := newApprovalControllerFixture(t, policy)

	ctx := stubVoteCtx(f.savedReq.ID, "approval-gate", map[string]string{
		"decision": "APPROVED",
		"comments": "approved via no-auth form",
		"voter_id": "alice",
	})
	ctx.Set(web.AuthDisabled, true)

	// WHEN
	err := f.ctrl.voteOnApproval(ctx)

	// THEN: succeeds — quorum reached (min_approvals=1)
	requireVoteSuccess(t, err)
}

func Test_ShouldDashboardVote_NoVoterID_AuthEnabled_Fails(t *testing.T) {
	// GIVEN: auth-enabled mode, no session user — form voter_id must be ignored (security)
	policy := &types.ApprovalPolicy{MinApprovals: 1}
	f := newApprovalControllerFixture(t, policy)

	ctx := stubVoteCtx(f.savedReq.ID, "approval-gate", map[string]string{
		"decision": "APPROVED",
		"voter_id": "eve", // attacker tries to spoof; must be ignored when auth is enabled
	})
	// AuthDisabled NOT set — form voter_id must not be accepted

	// WHEN
	err := f.ctrl.voteOnApproval(ctx)

	// THEN: voter_id resolves to "" → validation error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "voter_id is required")
}

func Test_ShouldDashboardVote_MissingDecision_Fails(t *testing.T) {
	// GIVEN: valid session, no decision in form
	policy := &types.ApprovalPolicy{MinApprovals: 1}
	f := newApprovalControllerFixture(t, policy)

	ctx := stubVoteCtx(f.savedReq.ID, "approval-gate", map[string]string{})
	ctx.Set(web.DBUser, f.qc.User)

	// WHEN
	err := f.ctrl.voteOnApproval(ctx)

	// THEN
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decision is required")
}

func Test_ShouldDashboardVote_RejectionRequiresComments(t *testing.T) {
	// GIVEN: REJECTED without comments
	policy := &types.ApprovalPolicy{MinApprovals: 1}
	f := newApprovalControllerFixture(t, policy)

	ctx := stubVoteCtx(f.savedReq.ID, "approval-gate", map[string]string{
		"decision": "REJECTED",
		// comments intentionally omitted
	})
	ctx.Set(web.DBUser, f.qc.User)

	// WHEN
	err := f.ctrl.voteOnApproval(ctx)

	// THEN
	require.Error(t, err)
	assert.Contains(t, err.Error(), "comments are required")
}

func Test_ShouldDashboardVote_UnauthorizedVoter_Fails(t *testing.T) {
	// GIVEN: policy allows alice,bob only — carol (same org) is denied
	policy := &types.ApprovalPolicy{
		MinApprovals: 1,
		AllowedUsers: "alice,bob",
	}
	f := newApprovalControllerFixture(t, policy)

	// carol is in the same org so the job can be found, but is not in allowed_users
	ctx := stubVoteCtx(f.savedReq.ID, "approval-gate", map[string]string{
		"decision": "APPROVED",
	})
	ctx.Set(web.DBUser, userWithSameOrg(f.qc, "carol"))

	// WHEN
	err := f.ctrl.voteOnApproval(ctx)

	// THEN: permission denied
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not authorized")
}

func Test_ShouldDashboardVote_Rejection_Succeeds(t *testing.T) {
	// GIVEN: unanimous policy — any single rejection immediately fails the job
	policy := &types.ApprovalPolicy{
		MinApprovals:     2,
		AllowedUsers:     "alice,bob,carol",
		RequireUnanimous: true,
	}
	f := newApprovalControllerFixture(t, policy)

	ctx := stubVoteCtx(f.savedReq.ID, "approval-gate", map[string]string{
		"decision": "REJECTED",
		"comments": "not production ready",
		"voter_id": "alice",
	})
	ctx.Set(web.AuthDisabled, true)

	// WHEN: alice rejects
	err := f.ctrl.voteOnApproval(ctx)

	// THEN: no real error (redirect = success, rejection is a valid terminal outcome)
	requireVoteSuccess(t, err)

	// AND: tally shows rejected
	status, statusErr := f.jm.GetApprovalStatus(context.Background(), f.qc, f.savedReq.ID, "approval-gate")
	require.NoError(t, statusErr)
	assert.True(t, status.Rejected)
}

func Test_ShouldDashboardVote_MultiStep_QuorumReached(t *testing.T) {
	// GIVEN: 2-of-3 quorum policy
	policy := &types.ApprovalPolicy{
		MinApprovals: 2,
		AllowedUsers: "alice,bob,carol",
	}
	f := newApprovalControllerFixture(t, policy)

	// WHEN: first vote (alice) — quorum not yet reached
	ctx1 := stubVoteCtx(f.savedReq.ID, "approval-gate", map[string]string{
		"decision": "APPROVED",
		"voter_id": "alice",
	})
	ctx1.Set(web.AuthDisabled, true)
	requireVoteSuccess(t, f.ctrl.voteOnApproval(ctx1))

	status1, err := f.jm.GetApprovalStatus(context.Background(), f.qc, f.savedReq.ID, "approval-gate")
	require.NoError(t, err)
	assert.False(t, status1.QuorumReached, "one vote must not reach quorum of 2")
	assert.Equal(t, 1, status1.ApprovalsReceived)

	// WHEN: second vote (bob) — quorum reached
	ctx2 := stubVoteCtx(f.savedReq.ID, "approval-gate", map[string]string{
		"decision": "APPROVED",
		"voter_id": "bob",
	})
	ctx2.Set(web.AuthDisabled, true)
	requireVoteSuccess(t, f.ctrl.voteOnApproval(ctx2))

	status2, err := f.jm.GetApprovalStatus(context.Background(), f.qc, f.savedReq.ID, "approval-gate")
	require.NoError(t, err)
	assert.True(t, status2.QuorumReached)
	assert.Equal(t, 2, status2.ApprovalsReceived)
}

func Test_ShouldDashboardVote_DuplicateVoteIsIdempotent(t *testing.T) {
	// GIVEN: 2-vote policy
	policy := &types.ApprovalPolicy{
		MinApprovals: 2,
		AllowedUsers: "alice,bob",
	}
	f := newApprovalControllerFixture(t, policy)

	// WHEN: alice votes twice
	for i := 0; i < 2; i++ {
		ctx := stubVoteCtx(f.savedReq.ID, "approval-gate", map[string]string{
			"decision": "APPROVED",
			"voter_id": "alice",
		})
		ctx.Set(web.AuthDisabled, true)
		requireVoteSuccess(t, f.ctrl.voteOnApproval(ctx))
	}

	// THEN: only one approval recorded
	status, err := f.jm.GetApprovalStatus(context.Background(), f.qc, f.savedReq.ID, "approval-gate")
	require.NoError(t, err)
	assert.Equal(t, 1, status.ApprovalsReceived, "duplicate must not double-count")
	assert.False(t, status.QuorumReached, "need 2, only have 1")
}

func Test_ShouldDashboardGetApprovalStatus(t *testing.T) {
	// GIVEN: one vote cast directly via manager
	policy := &types.ApprovalPolicy{
		MinApprovals: 2,
		AllowedUsers: "alice,bob,carol",
	}
	f := newApprovalControllerFixture(t, policy)

	_, err := f.jm.CastApprovalVote(context.Background(), f.qc, &types.ApprovalVoteRequest{
		RequestID: f.savedReq.ID,
		TaskType:  "approval-gate",
		VoterID:   "alice",
		Decision:  types.ApprovalDecisionApproved,
	})
	require.NoError(t, err)

	// WHEN: query status via manager
	status, err := f.jm.GetApprovalStatus(context.Background(), f.qc, f.savedReq.ID, "approval-gate")

	// THEN: tally is correct
	require.NoError(t, err)
	assert.Equal(t, 1, status.ApprovalsReceived)
	assert.False(t, status.QuorumReached)
	assert.Len(t, status.Votes, 1)
}

func Test_ShouldDashboardListPendingApprovals(t *testing.T) {
	// GIVEN: a job in MANUAL_APPROVAL_REQUIRED state
	policy := &types.ApprovalPolicy{MinApprovals: 1}
	f := newApprovalControllerFixture(t, policy)

	// WHEN: list pending approvals
	pending, total, err := f.jm.ListPendingApprovals(context.Background(), f.qc, 0, 20)

	// THEN: the waiting job appears in the list
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))

	found := false
	for _, p := range pending {
		if p.JobRequestID == f.savedReq.ID {
			found = true
			assert.Equal(t, "approval-gate", p.TaskType)
			break
		}
	}
	assert.True(t, found, "pending list must include the waiting job")
}
