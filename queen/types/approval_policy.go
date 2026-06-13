// SPDX-License-Identifier: AGPL-3.0-or-later
package types

import (
	"errors"
	"strings"
	"time"
)

// TimeoutAction defines what happens when an approval SLA deadline is breached.
type TimeoutAction string

const (
	// TimeoutActionAutoApprove automatically approves the task on SLA breach.
	TimeoutActionAutoApprove TimeoutAction = "AUTO_APPROVE"
	// TimeoutActionAutoReject automatically rejects the task on SLA breach.
	TimeoutActionAutoReject TimeoutAction = "AUTO_REJECT"
	// TimeoutActionEscalate sends an escalation notification but does not resolve.
	TimeoutActionEscalate TimeoutAction = "ESCALATE"
)

// ApprovalPolicy defines quorum rules and SLA config for a MANUAL task.
type ApprovalPolicy struct {
	// ID primary key
	ID string `json:"id" gorm:"primary_key"`
	// TaskDefinitionID foreign key to the owning task definition
	TaskDefinitionID string `json:"task_definition_id"`
	// MinApprovals is the quorum threshold (N of M votes needed to approve).
	MinApprovals int `yaml:"min_approvals" json:"min_approvals"`
	// AllowedRoles comma-separated roles permitted to vote (empty = no role restriction).
	AllowedRoles string `yaml:"allowed_roles,omitempty" json:"allowed_roles"`
	// AllowedUsers comma-separated user IDs permitted to vote (empty = no user restriction).
	AllowedUsers string `yaml:"allowed_users,omitempty" json:"allowed_users"`
	// RequireUnanimous when true, any single rejection immediately fails the task.
	RequireUnanimous bool `yaml:"require_unanimous,omitempty" json:"require_unanimous"`
	// SLADeadline max wait time before timeout_action fires (e.g. "4h", "30m"). 0 = no SLA.
	SLADeadline time.Duration `yaml:"sla_deadline,omitempty" json:"sla_deadline"`
	// TimeoutAction: AUTO_APPROVE, AUTO_REJECT, or ESCALATE (default).
	TimeoutAction TimeoutAction `yaml:"timeout_action,omitempty" json:"timeout_action"`
	// EscalationRecipients comma-separated emails/Slack channels for SLA breach notifications.
	EscalationRecipients string `yaml:"escalation_recipients,omitempty" json:"escalation_recipients"`
	// EscalationMessage custom message for escalation notifications.
	EscalationMessage string `yaml:"escalation_message,omitempty" json:"escalation_message"`
	// CreatedAt creation time
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt update time
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName overrides the default GORM table name.
func (ApprovalPolicy) TableName() string {
	return "formicary_approval_policies"
}

// Validate checks required fields and logical consistency.
func (p *ApprovalPolicy) Validate() error {
	if p.MinApprovals < 1 {
		return errors.New("min_approvals must be at least 1")
	}
	if p.SLADeadline < 0 {
		return errors.New("sla_deadline cannot be negative")
	}
	if p.TimeoutAction != "" &&
		p.TimeoutAction != TimeoutActionAutoApprove &&
		p.TimeoutAction != TimeoutActionAutoReject &&
		p.TimeoutAction != TimeoutActionEscalate {
		return errors.New("timeout_action must be AUTO_APPROVE, AUTO_REJECT, or ESCALATE")
	}
	return nil
}

// AllowedRoleList returns the split list of allowed roles (trimmed, non-empty).
func (p *ApprovalPolicy) AllowedRoleList() []string {
	return splitTrimmed(p.AllowedRoles)
}

// AllowedUserList returns the split list of allowed user IDs (trimmed, non-empty).
func (p *ApprovalPolicy) AllowedUserList() []string {
	return splitTrimmed(p.AllowedUsers)
}

// EscalationRecipientList returns the split list of escalation recipients.
func (p *ApprovalPolicy) EscalationRecipientList() []string {
	return splitTrimmed(p.EscalationRecipients)
}

// EffectiveTimeoutAction returns ESCALATE when timeout_action is not set.
func (p *ApprovalPolicy) EffectiveTimeoutAction() TimeoutAction {
	if p.TimeoutAction == "" {
		return TimeoutActionEscalate
	}
	return p.TimeoutAction
}

func splitTrimmed(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
