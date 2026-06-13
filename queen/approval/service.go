// SPDX-License-Identifier: AGPL-3.0-or-later
package approval

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

// Service handles multi-party approval logic: casting votes, evaluating quorum,
// resolving task/job state transitions, and SLA deadline enforcement.
type Service struct {
	db         *gorm.DB
	repository Repository
}

// NewService creates a new approval service.
func NewService(db *gorm.DB, repository Repository) *Service {
	return &Service{db: db, repository: repository}
}

// CastVote records a vote for the given task execution and resolves quorum if reached.
// Returns the current ApprovalStatus after the vote.
func (s *Service) CastVote(
	ctx context.Context,
	qc *common.QueryContext,
	request *types.JobRequest,
	taskType string,
	taskExecutionID string,
	policy *types.ApprovalPolicy,
	voterID string,
	voterName string,
	decision types.ApprovalDecision,
	comments string,
) (*types.ApprovalStatus, error) {
	if request == nil {
		return nil, common.NewValidationError(fmt.Errorf("job request is required"))
	}
	if taskType == "" {
		return nil, common.NewValidationError(fmt.Errorf("task_type is required"))
	}
	if taskExecutionID == "" {
		return nil, common.NewValidationError(fmt.Errorf("task_execution_id is required"))
	}
	if voterID == "" {
		return nil, common.NewValidationError(fmt.Errorf("voter_id is required"))
	}
	if decision != types.ApprovalDecisionApproved && decision != types.ApprovalDecisionRejected {
		return nil, common.NewValidationError(fmt.Errorf("decision must be APPROVED or REJECTED"))
	}

	// Verify job is in MANUAL_APPROVAL_REQUIRED state.
	if request.JobState != common.MANUAL_APPROVAL_REQUIRED {
		return nil, common.NewConflictError(fmt.Sprintf(
			"job request %s is not awaiting approval (state: %s)", request.ID, request.JobState))
	}

	// Check voter authorization — always, default-deny when policy is nil.
	if err := s.authorizeVoter(qc, policy, voterID); err != nil {
		return nil, err
	}

	// Idempotency: if this voter already voted, return the current status without re-processing.
	if alreadyVoted, err := s.repository.HasVoted(taskExecutionID, voterID); err != nil {
		return nil, fmt.Errorf("failed to check prior vote: %w", err)
	} else if alreadyVoted {
		return s.GetStatus(taskExecutionID, request.ID, policy)
	}

	// Rejection requires a comment explaining why.
	if decision == types.ApprovalDecisionRejected && strings.TrimSpace(comments) == "" {
		return nil, common.NewValidationError(fmt.Errorf("comments are required for rejection"))
	}

	// Record the vote (idempotent).
	vote := &types.ApprovalVote{
		TaskExecutionID: taskExecutionID,
		JobRequestID:    request.ID,
		VoterID:         voterID,
		VoterName:       voterName,
		Decision:        decision,
		Comments:        comments,
		VotedAt:         time.Now(),
	}
	if _, err := s.repository.SaveVote(vote); err != nil {
		return nil, fmt.Errorf("failed to record vote: %w", err)
	}

	// Get updated vote counts.
	approvals, rejections, err := s.repository.CountVotes(taskExecutionID)
	if err != nil {
		return nil, fmt.Errorf("failed to count votes: %w", err)
	}

	allVotes, err := s.repository.GetVotes(taskExecutionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get votes: %w", err)
	}

	minRequired := 1
	if policy != nil {
		minRequired = policy.MinApprovals
	}

	status := &types.ApprovalStatus{
		TaskExecutionID:      taskExecutionID,
		JobRequestID:         request.ID,
		ApprovalsReceived:    approvals,
		RejectionsReceived:   rejections,
		MinApprovalsRequired: minRequired,
		Votes:                allVotes,
		Policy:               policy,
	}

	// Evaluate rejection threshold.
	rejected := s.isRejected(approvals, rejections, policy)
	// Evaluate quorum.
	quorumReached := approvals >= minRequired

	if rejected {
		status.Rejected = true
		if err := s.finalizeRejection(ctx, qc, request, taskType, taskExecutionID, voterID, comments); err != nil {
			return status, err
		}
		logrus.WithFields(logrus.Fields{
			"Component":  "ApprovalService",
			"RequestID":  request.ID,
			"TaskType":   taskType,
			"VoterID":    voterID,
			"Approvals":  approvals,
			"Rejections": rejections,
		}).Info("Approval rejected")
	} else if quorumReached {
		status.QuorumReached = true
		if err := s.finalizeApproval(ctx, qc, request, taskType, taskExecutionID, voterID, comments); err != nil {
			return status, err
		}
		logrus.WithFields(logrus.Fields{
			"Component":   "ApprovalService",
			"RequestID":   request.ID,
			"TaskType":    taskType,
			"VoterID":     voterID,
			"Approvals":   approvals,
			"MinRequired": minRequired,
		}).Info("Approval quorum reached")
	} else {
		logrus.WithFields(logrus.Fields{
			"Component":   "ApprovalService",
			"RequestID":   request.ID,
			"TaskType":    taskType,
			"VoterID":     voterID,
			"Approvals":   approvals,
			"MinRequired": minRequired,
		}).Infof("Vote recorded, waiting for quorum (%d/%d)", approvals, minRequired)
	}

	return status, nil
}

// GetStatus returns the current approval status for a task execution.
func (s *Service) GetStatus(
	taskExecutionID string,
	jobRequestID string,
	policy *types.ApprovalPolicy,
) (*types.ApprovalStatus, error) {
	votes, err := s.repository.GetVotes(taskExecutionID)
	if err != nil {
		return nil, err
	}

	var approvals, rejections int
	for _, v := range votes {
		if v.IsApproved() {
			approvals++
		} else {
			rejections++
		}
	}

	minRequired := 1
	if policy != nil {
		minRequired = policy.MinApprovals
	}

	return &types.ApprovalStatus{
		TaskExecutionID:      taskExecutionID,
		JobRequestID:         jobRequestID,
		ApprovalsReceived:    approvals,
		RejectionsReceived:   rejections,
		MinApprovalsRequired: minRequired,
		QuorumReached:        approvals >= minRequired,
		Rejected:             s.isRejected(approvals, rejections, policy),
		Votes:                votes,
		Policy:               policy,
	}, nil
}

// ListPendingApprovals returns tasks currently awaiting approval votes.
func (s *Service) ListPendingApprovals(
	qc *common.QueryContext,
	page int,
	pageSize int,
) ([]*types.PendingApproval, int64, error) {
	return s.repository.FindPendingApprovals(qc, page, pageSize)
}

// FindBreachedDeadlines returns unresolved, non-escalated deadlines past the given time.
func (s *Service) FindBreachedDeadlines(now time.Time, limit int) ([]*types.ApprovalDeadline, error) {
	return s.repository.FindBreachedDeadlines(now, limit)
}

// HandleSLABreach executes the timeout action for an expired deadline.
func (s *Service) HandleSLABreach(
	ctx context.Context,
	qc *common.QueryContext,
	deadline *types.ApprovalDeadline,
	request *types.JobRequest,
	taskType string,
	notifyFn func(recipients []string, message string) error,
) error {
	switch deadline.TimeoutAction {
	case types.TimeoutActionAutoApprove:
		if err := s.finalizeApproval(ctx, qc, request, taskType, deadline.TaskExecutionID,
			types.SyntheticVoterID, "Auto-approved by SLA timeout"); err != nil {
			return err
		}
		return s.repository.MarkDeadlineResolved(deadline.ID)

	case types.TimeoutActionAutoReject:
		if err := s.finalizeRejection(ctx, qc, request, taskType, deadline.TaskExecutionID,
			types.SyntheticVoterID, "Auto-rejected by SLA timeout"); err != nil {
			return err
		}
		return s.repository.MarkDeadlineResolved(deadline.ID)

	default: // ESCALATE
		tmp := types.ApprovalPolicy{EscalationRecipients: deadline.EscalationRecipients}
		recipients := tmp.EscalationRecipientList()
		if len(recipients) > 0 && notifyFn != nil {
			msg := fmt.Sprintf("Approval SLA breached for job request %s (task: %s)",
				deadline.JobRequestID, taskType)
			if err := notifyFn(recipients, msg); err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":  "ApprovalService",
					"DeadlineID": deadline.ID,
					"Error":      err,
				}).Warn("Failed to send escalation notification")
			}
		}
		return s.repository.MarkDeadlineEscalated(deadline.ID)
	}
}

// CreateDeadlineIfNeeded creates an SLA deadline row if the policy has an SLA configured.
func (s *Service) CreateDeadlineIfNeeded(
	taskExecutionID string,
	jobRequestID string,
	policy *types.ApprovalPolicy,
) error {
	if policy == nil || policy.SLADeadline <= 0 {
		return nil
	}
	deadline := &types.ApprovalDeadline{
		TaskExecutionID:      taskExecutionID,
		JobRequestID:         jobRequestID,
		Deadline:             time.Now().Add(policy.SLADeadline),
		TimeoutAction:        policy.EffectiveTimeoutAction(),
		EscalationRecipients: policy.EscalationRecipients,
	}
	_, err := s.repository.SaveDeadline(deadline)
	return err
}

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////

// authorizeVoter checks that the voter is in the allowed roles or user list.
// nil policy is default-deny: no policy configured means no one may vote externally.
func (s *Service) authorizeVoter(qc *common.QueryContext, policy *types.ApprovalPolicy, voterID string) error {
	// SyntheticVoterID is reserved for internal SLA-timeout auto-decisions.
	// No external caller — authenticated or not — may use this ID.
	if voterID == types.SyntheticVoterID {
		return common.NewPermissionError(fmt.Errorf(
			"voter ID %q is reserved for system use", types.SyntheticVoterID))
	}

	if policy == nil {
		return common.NewPermissionError(fmt.Errorf("no approval policy configured for this task"))
	}

	allowedUsers := policy.AllowedUserList()
	allowedRoles := policy.AllowedRoleList()

	// If neither list is configured, any authenticated user may vote.
	if len(allowedUsers) == 0 && len(allowedRoles) == 0 {
		return nil
	}

	// Check explicit user list.
	for _, uid := range allowedUsers {
		if uid == voterID || uid == qc.GetUserID() {
			return nil
		}
	}

	// Check role membership via query context user.
	if len(allowedRoles) > 0 && qc.User != nil {
		for _, required := range allowedRoles {
			if qc.User.HasRole(acl.RoleType(strings.TrimSpace(required))) {
				return nil
			}
		}
	}

	return common.NewPermissionError(fmt.Errorf(
		"voter %s is not authorized to vote on this task (allowed roles: %s, allowed users: %s)",
		voterID, policy.AllowedRoles, policy.AllowedUsers))
}

// isRejected returns true when the rejection threshold is met.
// When require_unanimous=true: any single rejection fails.
// When require_unanimous=false: only reject when quorum is mathematically impossible —
// i.e., even all remaining potential approvers cannot reach min_approvals.
func (s *Service) isRejected(approvals, rejections int, policy *types.ApprovalPolicy) bool {
	if rejections == 0 {
		return false
	}
	if policy == nil {
		// No policy and there's a rejection — fail.
		return true
	}
	if policy.RequireUnanimous {
		return true
	}
	// Non-unanimous: reject only when quorum is mathematically impossible.
	// Total allowed voters = len(AllowedUsers); if 0, we can't bound the pool, so use
	// the already-cast votes as the universe and check if approvals can still reach min.
	allowedUsers := policy.AllowedUserList()
	totalAllowed := len(allowedUsers)
	if totalAllowed == 0 {
		// Pool size unknown — treat any rejection as failing (conservative).
		return true
	}
	remainingVoters := totalAllowed - approvals - rejections
	if remainingVoters < 0 {
		remainingVoters = 0
	}
	return approvals+remainingVoters < policy.MinApprovals
}

// finalizeApproval transitions the task to COMPLETED and the job to PENDING.
func (s *Service) finalizeApproval(
	ctx context.Context,
	qc *common.QueryContext,
	request *types.JobRequest,
	taskType string,
	taskExecutionID string,
	approvedBy string,
	comments string,
) error {
	return s.db.Transaction(func(db *gorm.DB) error {
		now := time.Now()
		msg := fmt.Sprintf("Approved by %s", approvedBy)
		if comments != "" {
			msg = fmt.Sprintf("%s: %s", msg, comments)
		}

		// 1. Transition task execution to COMPLETED.
		if res := db.Model(&types.TaskExecution{}).
			Where("job_execution_id = ? AND task_type = ? AND task_state = ?",
				request.JobExecutionID, taskType, common.MANUAL_APPROVAL_REQUIRED).
			Updates(map[string]interface{}{
				"task_state":   common.COMPLETED,
				"exit_code":    string(types.ApprovalDecisionApproved),
				"exit_message": msg,
				"comments":     comments,
				"ended_at":     now,
				"updated_at":   now,
			}); res.Error != nil {
			return res.Error
		}

		// 2. Transition job request to PENDING so the scheduler resumes it.
		if res := qc.AddOrgElseUserWhere(db, false).Model(&types.JobRequest{}).
			Where("id = ? AND job_state = ?", request.ID, common.MANUAL_APPROVAL_REQUIRED).
			Updates(map[string]interface{}{
				"job_state":     common.PENDING,
				"error_code":    "",
				"error_message": "",
				"updated_at":    now,
			}); res.Error != nil {
			return res.Error
		}

		// 3. Resolve any open SLA deadlines for this task.
		if err := db.Model(&types.ApprovalDeadline{}).
			Where("task_execution_id = ? AND resolved = ?", taskExecutionID, false).
			Updates(map[string]interface{}{"resolved": true}).Error; err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":       "ApprovalService",
				"TaskExecutionID": taskExecutionID,
				"Error":           err,
			}).Warn("failed to resolve deadline on approval")
		}
		return nil
	})
}

// finalizeRejection transitions the task to FAILED and the job to FAILED.
func (s *Service) finalizeRejection(
	ctx context.Context,
	qc *common.QueryContext,
	request *types.JobRequest,
	taskType string,
	taskExecutionID string,
	rejectedBy string,
	comments string,
) error {
	return s.db.Transaction(func(db *gorm.DB) error {
		now := time.Now()
		msg := fmt.Sprintf("Rejected by %s", rejectedBy)
		if comments != "" {
			msg = fmt.Sprintf("%s: %s", msg, comments)
		}

		// 1. Transition task execution to FAILED.
		if res := db.Model(&types.TaskExecution{}).
			Where("job_execution_id = ? AND task_type = ? AND task_state = ?",
				request.JobExecutionID, taskType, common.MANUAL_APPROVAL_REQUIRED).
			Updates(map[string]interface{}{
				"task_state":    common.FAILED,
				"exit_code":     string(types.ApprovalDecisionRejected),
				"exit_message":  msg,
				"error_code":    common.ErrorManualRejection,
				"error_message": msg,
				"comments":      comments,
				"ended_at":      now,
				"updated_at":    now,
			}); res.Error != nil {
			return res.Error
		}

		// 2. Transition job execution to FAILED.
		if res := db.Model(&types.JobExecution{}).
			Where("id = ? AND job_state = ?", request.JobExecutionID, common.MANUAL_APPROVAL_REQUIRED).
			Updates(map[string]interface{}{
				"job_state":     common.FAILED,
				"error_code":    common.ErrorManualRejection,
				"error_message": msg,
				"ended_at":      now,
				"updated_at":    now,
			}); res.Error != nil {
			return res.Error
		}

		// 3. Transition job request to FAILED.
		if res := qc.AddOrgElseUserWhere(db, false).Model(&types.JobRequest{}).
			Where("id = ? AND job_state = ?", request.ID, common.MANUAL_APPROVAL_REQUIRED).
			Updates(map[string]interface{}{
				"job_state":     common.FAILED,
				"error_code":    common.ErrorManualRejection,
				"error_message": msg,
				"updated_at":    now,
			}); res.Error != nil {
			return res.Error
		}

		// 4. Resolve any open SLA deadlines for this task.
		if err := db.Model(&types.ApprovalDeadline{}).
			Where("task_execution_id = ? AND resolved = ?", taskExecutionID, false).
			Updates(map[string]interface{}{"resolved": true}).Error; err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":       "ApprovalService",
				"TaskExecutionID": taskExecutionID,
				"Error":           err,
			}).Warn("failed to resolve deadline on rejection")
		}
		return nil
	})
}
