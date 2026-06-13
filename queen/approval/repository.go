// SPDX-License-Identifier: AGPL-3.0-or-later
package approval

import (
	"time"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

// Repository defines persistence operations for multi-party approval data.
type Repository interface {
	// SavePolicy creates or updates an ApprovalPolicy.
	SavePolicy(policy *types.ApprovalPolicy) (*types.ApprovalPolicy, error)

	// GetPolicyByTaskDefinition loads the policy for a task definition (nil if none configured).
	GetPolicyByTaskDefinition(taskDefinitionID string) (*types.ApprovalPolicy, error)

	// SaveVote records a single vote. Idempotent: duplicate (task_execution_id, voter_id) is a no-op.
	SaveVote(vote *types.ApprovalVote) (*types.ApprovalVote, error)

	// GetVotes returns all votes for a task execution.
	GetVotes(taskExecutionID string) ([]*types.ApprovalVote, error)

	// CountVotes returns the number of approvals and rejections for a task execution.
	CountVotes(taskExecutionID string) (approvals int, rejections int, err error)

	// HasVoted returns true if the given voter has already voted on this task execution.
	HasVoted(taskExecutionID, voterID string) (bool, error)

	// SaveDeadline persists an SLA deadline row.
	SaveDeadline(deadline *types.ApprovalDeadline) (*types.ApprovalDeadline, error)

	// FindBreachedDeadlines returns unresolved, unescalated deadlines past the given time (batch limited).
	FindBreachedDeadlines(now time.Time, limit int) ([]*types.ApprovalDeadline, error)

	// MarkDeadlineResolved marks a deadline as resolved (no further action needed).
	MarkDeadlineResolved(deadlineID string) error

	// MarkDeadlineEscalated marks a deadline as escalated (notification sent, still open for manual resolution).
	MarkDeadlineEscalated(deadlineID string) error

	// ResolveDeadlineForTask marks all unresolved deadlines for a task execution as resolved.
	ResolveDeadlineForTask(taskExecutionID string) error

	// FindPendingApprovals returns task executions in MANUAL_APPROVAL_REQUIRED state.
	FindPendingApprovals(qc *common.QueryContext, page, pageSize int) ([]*types.PendingApproval, int64, error)
}
