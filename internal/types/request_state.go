package types

import (
	"strings"
)

// RequestState defines enum for the state of request or execution such as pending/completed/failed.
type RequestState string

// Types of request states
const (
	// PENDING request
	PENDING RequestState = "PENDING"
	// READY request
	READY RequestState = "READY"
	// COMPLETED request
	COMPLETED RequestState = "COMPLETED"
	// FAILED request
	FAILED RequestState = "FAILED"
	// EXECUTING request
	EXECUTING RequestState = "EXECUTING"
	// STARTED request
	STARTED RequestState = "STARTED"
	// CANCELLED request
	CANCELLED RequestState = "CANCELLED"
	// PAUSED request
	PAUSED RequestState = "PAUSED"
	// MANUAL_APPROVAL_REQUIRED request
	MANUAL_APPROVAL_REQUIRED RequestState = "MANUAL_APPROVAL_REQUIRED"

	// HISTORY request -- collection of FAILED/COMPLETED/CANCELLED
	// Grouping use for queries
	HISTORY RequestState = "HISTORY"
	// RUNNING request -- collection of STARTED/EXECUTING
	RUNNING RequestState = "RUNNING"
	// WAITING request -- collection of PENDING/PAUSED/MANUAL_APPROVAL_REQUIRED/READY
	WAITING RequestState = "WAITING"

	// FATAL request
	FATAL RequestState = "FATAL"

	// RESTART_JOB request
	RESTART_JOB RequestState = "RESTART_JOB"

	// PAUSE_JOB request
	PAUSE_JOB RequestState = "PAUSE_JOB"

	// WAIT_FOR_APPROVAL request
	WAIT_FOR_APPROVAL RequestState = "WAIT_FOR_APPROVAL"

	// RESTART_TASK request
	RESTART_TASK RequestState = "RESTART_TASK"

	// APPROVED request
	APPROVED RequestState = "APPROVED"

	// REJECTED request
	REJECTED RequestState = "REJECTED"

	// RESERVED request
	// Resource management
	RESERVED RequestState = "RESERVED"
	// DELETED state
	DELETED RequestState = "DELETED"
	// UNKNOWN state
	UNKNOWN RequestState = "UNKNOWN"
)

// TerminalStates defines immutable states
var TerminalStates = []string{string(COMPLETED), string(FAILED), string(CANCELLED)}

// RunningStates defines executing or starting state
var RunningStates = []string{string(EXECUTING), string(STARTED)}

// WaitingStates defines pending or ready state
var WaitingStates = []string{string(PENDING), string(PAUSED), string(READY), string(MANUAL_APPROVAL_REQUIRED)}

// NotRestartableStates defines pending or completed state
var NotRestartableStates = []string{string(PENDING), string(READY), string(COMPLETED)}

// OrphanStates defines started, ready and executing state
var OrphanStates = []string{string(STARTED), string(READY), string(EXECUTING)}

// NewRequestState constructor
func NewRequestState(state string) RequestState {
	return RequestState(strings.ToUpper(state))
}

// IsTerminal returns true if state is terminal
func (rs RequestState) IsTerminal() bool {
	return rs == COMPLETED || rs == FAILED || rs == CANCELLED
}

// CanFinalize returns true if state is terminal or be finalized
func (rs RequestState) CanFinalize() bool {
	return rs.IsTerminal() || rs == PAUSED || rs == MANUAL_APPROVAL_REQUIRED
}

// Processing returns true if state is still processing
func (rs RequestState) Processing() bool {
	return rs == WAITING || rs == PENDING || rs == PAUSED || rs == READY || rs == EXECUTING ||
		rs == STARTED || rs == MANUAL_APPROVAL_REQUIRED
}

// CanRestart checks if request can be restarted
func (rs RequestState) CanRestart() bool {
	return rs == FAILED || rs == CANCELLED || rs == PAUSED // not MANUAL_APPROVAL_REQUIRED
}

// CanCancel checks if request can be cancelled
func (rs RequestState) CanCancel() bool {
	return !rs.IsTerminal() || rs == PAUSED || rs == MANUAL_APPROVAL_REQUIRED
}

// CanApprove checks if request can be approved
func (rs RequestState) CanApprove() bool {
	return rs == MANUAL_APPROVAL_REQUIRED
}

// Completed returns true if state is completed.
func (rs RequestState) Completed() bool {
	return rs == COMPLETED
}

// Paused returns true if state is paused.
func (rs RequestState) Paused() bool {
	return rs == PAUSED
}

// ManualApprovalRequired returns true for manual approval.
func (rs RequestState) ManualApprovalRequired() bool {
	return rs == MANUAL_APPROVAL_REQUIRED
}

// Failed returns failed status
func (rs RequestState) Failed() bool {
	return rs == FAILED || rs == CANCELLED
}

// Ready returns true if state is ready
func (rs RequestState) Ready() bool {
	return rs == READY
}

// Pending returns true if state is pending
func (rs RequestState) Pending() bool {
	return rs == PENDING
}

// Cancelled returns true if state is cancelled
func (rs RequestState) Cancelled() bool {
	return rs == CANCELLED
}

// Executing returns true if state is executing
func (rs RequestState) Executing() bool {
	return rs == EXECUTING
}

// Started returns true if state is started
func (rs RequestState) Started() bool {
	return rs == STARTED
}

// Waiting returns true if state is waiting to run
func (rs RequestState) Waiting() bool {
	return rs == WAITING || rs == PENDING || rs == PAUSED || rs == READY || rs == MANUAL_APPROVAL_REQUIRED
}

// Running returns true if state is running
func (rs RequestState) Running() bool {
	return rs == STARTED || rs == EXECUTING || rs == RUNNING
}

// Done returns true if state is done
func (rs RequestState) Done() bool {
	return rs == FAILED || rs == COMPLETED || rs == CANCELLED
}

// Unknown status
func (rs RequestState) Unknown() bool {
	return rs != FAILED &&
		rs != CANCELLED &&
		rs != PENDING &&
		rs != PAUSED &&
		rs != MANUAL_APPROVAL_REQUIRED &&
		rs != READY &&
		rs != COMPLETED &&
		rs != EXECUTING &&
		rs != STARTED &&
		rs != HISTORY &&
		rs != RESTART_JOB &&
		rs != PAUSE_JOB &&
		rs != RESTART_TASK
}

// CanTransitionTo enforces state transition validation
func (rs RequestState) CanTransitionTo(newState RequestState) bool {
	if rs == FAILED || rs == CANCELLED {
		return newState == PENDING
	} else if rs == PENDING || rs == PAUSED {
		return newState == READY || newState == FAILED
	} else if rs == MANUAL_APPROVAL_REQUIRED {
		return newState == READY
	} else if rs == READY {
		return newState == STARTED || newState == FAILED || newState == PENDING || newState == PAUSED // not MANUAL_APPROVAL_REQUIRED
	} else if rs == STARTED {
		return newState == EXECUTING
	} else if rs == EXECUTING {
		return newState == FAILED || newState == COMPLETED || newState == PAUSED || newState == MANUAL_APPROVAL_REQUIRED
	}
	return false
}

const (
	successColor   = "darkseagreen4"
	failColor      = "firebrick4"
	unknownColor   = "goldenrod3"
	executingColor = "skyblue2"
	defaultColor   = "gray"
)

// DotColor for drawing diagrams image
func (rs RequestState) DotColor() string {
	if rs.Completed() {
		return successColor
	} else if rs.Failed() {
		return failColor
	} else if rs.Executing() {
		return executingColor
	} else if rs.Unknown() {
		return unknownColor
	} else {
		return defaultColor
	}
}

// SlackColor slack color
func (rs RequestState) SlackColor() string {
	if rs.Completed() {
		return "#28a745"
	} else if rs.Failed() {
		return "#dc3545"
	} else if rs.Executing() {
		return "#17a2b8"
	} else if rs.Unknown() {
		return "#6c757d"
	} else {
		return "#fd7e14"
	}
}

// Emoji getter
func (rs RequestState) Emoji() string {
	if rs.Completed() {
		return "✅"
	} else if rs.Failed() {
		return "❌"
	} else if rs.Executing() {
		return "➰"
	} else if rs.Unknown() {
		return "⚠️"
	} else {
		return ""
	}
}
