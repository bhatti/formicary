// SPDX-License-Identifier: AGPL-3.0-or-later
package types

import (
	"errors"
	"time"
)

// ApprovalDecision represents an APPROVED or REJECTED vote decision.
type ApprovalDecision string

const (
	// ApprovalDecisionApproved marks the task as approved.
	ApprovalDecisionApproved ApprovalDecision = "APPROVED"
	// ApprovalDecisionRejected marks the task as rejected.
	ApprovalDecisionRejected ApprovalDecision = "REJECTED"
)

// SyntheticVoterID is the voter ID used by the system for SLA-triggered auto-decisions.
const SyntheticVoterID = "system:sla-timeout"

// ApprovalVote records a single approver's decision on a task execution.
type ApprovalVote struct {
	// ID primary key
	ID string `json:"id" gorm:"primary_key"`
	// TaskExecutionID foreign key to the task execution being approved
	TaskExecutionID string `json:"task_execution_id" gorm:"uniqueIndex:uq_approval_vote_voter_task;not null"`
	// JobRequestID for cross-referencing and pending-approvals queries
	JobRequestID string `json:"job_request_id"`
	// VoterID identifies the approver — composite unique key with task_execution_id
	VoterID string `json:"voter_id" gorm:"uniqueIndex:uq_approval_vote_voter_task;not null"`
	// VoterName display name of the approver
	VoterName string `json:"voter_name"`
	// Decision is APPROVED or REJECTED
	Decision ApprovalDecision `json:"decision"`
	// Comments optional rationale
	Comments string `json:"comments"`
	// VotedAt when the vote was cast
	VotedAt time.Time `json:"voted_at"`
}

// TableName overrides the default GORM table name.
func (ApprovalVote) TableName() string {
	return "formicary_approval_votes"
}

// Validate checks required fields.
func (v *ApprovalVote) Validate() error {
	if v.TaskExecutionID == "" {
		return errors.New("task_execution_id is required")
	}
	if v.JobRequestID == "" {
		return errors.New("job_request_id is required")
	}
	if v.VoterID == "" {
		return errors.New("voter_id is required")
	}
	if v.Decision != ApprovalDecisionApproved && v.Decision != ApprovalDecisionRejected {
		return errors.New("decision must be APPROVED or REJECTED")
	}
	return nil
}

// IsApproved returns true when the vote is an approval.
func (v *ApprovalVote) IsApproved() bool {
	return v.Decision == ApprovalDecisionApproved
}

// ApprovalStatus is the aggregate approval state for a single task execution.
type ApprovalStatus struct {
	TaskExecutionID      string          `json:"task_execution_id"`
	JobRequestID         string          `json:"job_request_id"`
	ApprovalsReceived    int             `json:"approvals_received"`
	RejectionsReceived   int             `json:"rejections_received"`
	MinApprovalsRequired int             `json:"min_approvals_required"`
	QuorumReached        bool            `json:"quorum_reached"`
	Rejected             bool            `json:"rejected"`
	SLABreached          bool            `json:"sla_breached"`
	Deadline             *time.Time      `json:"deadline,omitempty"`
	Votes                []*ApprovalVote `json:"votes"`
	Policy               *ApprovalPolicy `json:"policy,omitempty"`
}

// PendingApproval represents a task awaiting approval votes.
type PendingApproval struct {
	JobRequestID    string          `json:"job_request_id"`
	TaskExecutionID string          `json:"task_execution_id"`
	JobType         string          `json:"job_type"`
	TaskType        string          `json:"task_type"`
	Status          *ApprovalStatus `json:"status"`
	RequestedAt     time.Time       `json:"requested_at"`
}
