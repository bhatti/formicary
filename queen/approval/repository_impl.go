// SPDX-License-Identifier: AGPL-3.0-or-later
package approval

import (
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

// RepositoryImpl implements Repository using GORM.
type RepositoryImpl struct {
	db *gorm.DB
}

// NewRepositoryImpl creates a new approval repository.
func NewRepositoryImpl(db *gorm.DB) (*RepositoryImpl, error) {
	return &RepositoryImpl{db: db}, nil
}

// SavePolicy creates or updates an ApprovalPolicy.
func (r *RepositoryImpl) SavePolicy(policy *types.ApprovalPolicy) (*types.ApprovalPolicy, error) {
	if err := policy.Validate(); err != nil {
		return nil, common.NewValidationError(err)
	}
	now := time.Now()
	policy.UpdatedAt = now
	if policy.ID == "" {
		policy.ID = ulid.Make().String()
		policy.CreatedAt = now
		if res := r.db.Create(policy); res.Error != nil {
			return nil, res.Error
		}
	} else {
		if res := r.db.Save(policy); res.Error != nil {
			return nil, res.Error
		}
	}
	return policy, nil
}

// GetPolicyByTaskDefinition loads the policy for a task definition.
func (r *RepositoryImpl) GetPolicyByTaskDefinition(taskDefinitionID string) (*types.ApprovalPolicy, error) {
	var policy types.ApprovalPolicy
	res := r.db.Where("task_definition_id = ?", taskDefinitionID).First(&policy)
	if res.Error != nil {
		if res.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, res.Error
	}
	return &policy, nil
}

// SaveVote records a single vote. Uses INSERT OR IGNORE semantics for idempotency.
func (r *RepositoryImpl) SaveVote(vote *types.ApprovalVote) (*types.ApprovalVote, error) {
	if err := vote.Validate(); err != nil {
		return nil, common.NewValidationError(err)
	}
	if vote.ID == "" {
		vote.ID = ulid.Make().String()
	}
	if vote.VotedAt.IsZero() {
		vote.VotedAt = time.Now()
	}
	// OnConflict DoNothing makes this idempotent: same voter on same task = no-op.
	res := r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task_execution_id"}, {Name: "voter_id"}},
		DoNothing: true,
	}).Create(vote)
	if res.Error != nil {
		return nil, res.Error
	}
	// Reload from DB to reflect the actual stored state (may be the original if duplicate).
	var stored types.ApprovalVote
	if err := r.db.Where("task_execution_id = ? AND voter_id = ?",
		vote.TaskExecutionID, vote.VoterID).First(&stored).Error; err != nil {
		return nil, err
	}
	return &stored, nil
}

// GetVotes returns all votes for a task execution ordered by vote time.
func (r *RepositoryImpl) GetVotes(taskExecutionID string) ([]*types.ApprovalVote, error) {
	var votes []*types.ApprovalVote
	if err := r.db.Where("task_execution_id = ?", taskExecutionID).
		Order("voted_at ASC").Find(&votes).Error; err != nil {
		return nil, err
	}
	return votes, nil
}

// CountVotes returns approval and rejection counts for a task execution.
func (r *RepositoryImpl) CountVotes(taskExecutionID string) (approvals int, rejections int, err error) {
	type result struct {
		Decision string
		Count    int
	}
	var rows []result
	if err = r.db.Model(&types.ApprovalVote{}).
		Select("decision, COUNT(*) as count").
		Where("task_execution_id = ?", taskExecutionID).
		Group("decision").Scan(&rows).Error; err != nil {
		return
	}
	for _, row := range rows {
		if row.Decision == string(types.ApprovalDecisionApproved) {
			approvals = row.Count
		} else if row.Decision == string(types.ApprovalDecisionRejected) {
			rejections = row.Count
		}
	}
	return
}

// HasVoted returns true if the voter already has a vote recorded.
func (r *RepositoryImpl) HasVoted(taskExecutionID, voterID string) (bool, error) {
	var count int64
	err := r.db.Model(&types.ApprovalVote{}).
		Where("task_execution_id = ? AND voter_id = ?", taskExecutionID, voterID).
		Count(&count).Error
	return count > 0, err
}

// SaveDeadline persists an SLA deadline row.
func (r *RepositoryImpl) SaveDeadline(deadline *types.ApprovalDeadline) (*types.ApprovalDeadline, error) {
	if deadline.ID == "" {
		deadline.ID = ulid.Make().String()
	}
	if deadline.CreatedAt.IsZero() {
		deadline.CreatedAt = time.Now()
	}
	if res := r.db.Create(deadline); res.Error != nil {
		return nil, res.Error
	}
	return deadline, nil
}

// FindBreachedDeadlines returns unresolved, non-escalated deadlines that have passed.
func (r *RepositoryImpl) FindBreachedDeadlines(now time.Time, limit int) ([]*types.ApprovalDeadline, error) {
	var deadlines []*types.ApprovalDeadline
	err := r.db.Where("resolved = ? AND escalated = ? AND deadline < ?", false, false, now).
		Order("deadline ASC").Limit(limit).Find(&deadlines).Error
	return deadlines, err
}

// MarkDeadlineResolved marks a deadline as fully handled.
func (r *RepositoryImpl) MarkDeadlineResolved(deadlineID string) error {
	return r.db.Model(&types.ApprovalDeadline{}).
		Where("id = ?", deadlineID).
		Updates(map[string]interface{}{"resolved": true}).Error
}

// MarkDeadlineEscalated marks that an escalation notification was sent.
func (r *RepositoryImpl) MarkDeadlineEscalated(deadlineID string) error {
	return r.db.Model(&types.ApprovalDeadline{}).
		Where("id = ?", deadlineID).
		Updates(map[string]interface{}{"escalated": true}).Error
}

// ResolveDeadlineForTask marks all open deadlines for a task execution as resolved.
func (r *RepositoryImpl) ResolveDeadlineForTask(taskExecutionID string) error {
	return r.db.Model(&types.ApprovalDeadline{}).
		Where("task_execution_id = ? AND resolved = ?", taskExecutionID, false).
		Updates(map[string]interface{}{"resolved": true}).Error
}

// FindPendingApprovals returns PendingApproval records for all task executions in
// MANUAL_APPROVAL_REQUIRED state, scoped to the query context's org/user.
func (r *RepositoryImpl) FindPendingApprovals(
	qc *common.QueryContext, page, pageSize int) ([]*types.PendingApproval, int64, error) {
	type row struct {
		TaskExecutionID string
		JobRequestID    string
		JobType         string
		TaskType        string
		StartedAt       time.Time
	}
	query := r.db.Table("formicary_task_executions te").
		Select("te.id as task_execution_id, jr.id as job_request_id, jr.job_type, te.task_type, te.started_at").
		Joins("JOIN formicary_job_executions je ON te.job_execution_id = je.id").
		Joins("JOIN formicary_job_requests jr ON je.job_request_id = jr.id").
		Where("te.task_state = ?", "MANUAL_APPROVAL_REQUIRED")

	if qc.GetOrganizationID() != "" {
		query = query.Where("jr.organization_id = ?", qc.GetOrganizationID())
	} else if qc.GetUserID() != "" {
		query = query.Where("jr.user_id = ?", qc.GetUserID())
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []row
	if err := query.Offset(page * pageSize).Limit(pageSize).Order("te.started_at ASC").Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	pending := make([]*types.PendingApproval, 0, len(rows))
	for _, r2 := range rows {
		votes, err := r.GetVotes(r2.TaskExecutionID)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get votes for %s: %w", r2.TaskExecutionID, err)
		}
		var approvals, rejections int
		for _, v := range votes {
			if v.IsApproved() {
				approvals++
			} else {
				rejections++
			}
		}
		pending = append(pending, &types.PendingApproval{
			JobRequestID:    r2.JobRequestID,
			TaskExecutionID: r2.TaskExecutionID,
			JobType:         r2.JobType,
			TaskType:        r2.TaskType,
			RequestedAt:     r2.StartedAt,
			Status: &types.ApprovalStatus{
				TaskExecutionID:    r2.TaskExecutionID,
				JobRequestID:       r2.JobRequestID,
				ApprovalsReceived:  approvals,
				RejectionsReceived: rejections,
				Votes:              votes,
			},
		})
	}
	return pending, total, nil
}
