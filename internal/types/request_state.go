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

	// HISTORY request -- collection of FAILED/COMPLETED/CANCELLED
	// Grouping use for queries
	HISTORY RequestState = "HISTORY"
	// RUNNING request -- collection of STARTED/EXECUTING
	RUNNING RequestState = "RUNNING"
	// WAITING request -- collection of PENDING/READY
	WAITING RequestState = "WAITING"

	// FATAL request
	FATAL RequestState = "FATAL"

	// RESTART_JOB request
	RESTART_JOB RequestState = "RESTART_JOB"

	// RESTART_TASK request
	RESTART_TASK RequestState = "RESTART_TASK"

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
var WaitingStates = []string{string(PENDING), string(READY)}

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

// Processing returns true if state is still processing
func (rs RequestState) Processing() bool {
	return rs == WAITING || rs == PENDING || rs == READY || rs == EXECUTING || rs == STARTED
}

// CanRestart checks if request can be restarted
func (rs RequestState) CanRestart() bool {
	return rs == FAILED || rs == CANCELLED
}

// CanCancel checks if request can be cancelled
func (rs RequestState) CanCancel() bool {
	return !rs.IsTerminal()
}

// Completed returns true if state is completed
func (rs RequestState) Completed() bool {
	return rs == COMPLETED
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
	return rs == WAITING || rs == PENDING || rs == READY
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
		rs != READY &&
		rs != COMPLETED &&
		rs != EXECUTING &&
		rs != STARTED &&
		rs != HISTORY &&
		rs != RESTART_JOB &&
		rs != RESTART_TASK
}

const (
	successColor   = "darkseagreen4"
	failColor      = "firebrick4"
	unknownColor   = "goldenrod3"
	executingColor = "skyblue2"
	defaultColor   = "gray"
)

// DotColor for drawing dot image
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
