// SPDX-License-Identifier: AGPL-3.0-or-later
package types

import "time"

// ApprovalDeadline tracks an active SLA deadline for a task awaiting approval.
// The scheduler ticker queries this table to fire auto-decisions or escalations.
type ApprovalDeadline struct {
	// ID primary key
	ID string `json:"id" gorm:"primary_key"`
	// TaskExecutionID foreign key
	TaskExecutionID string `json:"task_execution_id"`
	// JobRequestID for logging and correlation
	JobRequestID string `json:"job_request_id"`
	// Deadline is when the SLA expires
	Deadline time.Time `json:"deadline"`
	// Escalated is set true once an ESCALATE notification has been sent.
	// The deadline is NOT resolved in ESCALATE mode to allow manual resolution.
	Escalated bool `json:"escalated"`
	// Resolved is set true once the deadline has been fully handled.
	Resolved bool `json:"resolved"`
	// TimeoutAction mirrors the policy's timeout action at creation time.
	TimeoutAction TimeoutAction `json:"timeout_action"`
	// EscalationRecipients mirrors the policy's recipients at creation time.
	EscalationRecipients string `json:"escalation_recipients"`
	// CreatedAt creation time
	CreatedAt time.Time `json:"created_at"`
}

// TableName overrides the default GORM table name.
func (ApprovalDeadline) TableName() string {
	return "formicary_approval_deadlines"
}
