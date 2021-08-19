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
	// SUSPENDED request
	SUSPENDED RequestState = "SUSPENDED"

	// HISTORY request -- collection of FAILED/COMPLETED/CANCELLED
	// Grouping use for queries
	HISTORY RequestState = "HISTORY"
	// RUNNING request -- collection of STARTED/EXECUTING
	RUNNING RequestState = "RUNNING"
	// WAITING request -- collection of PENDING/READY
	WAITING RequestState = "WAITING"

	// FATAL request
	FATAL RequestState = "FATAL"

	// RESERVED request
	// Resource management
	RESERVED RequestState = "RESERVED"
	// DELETED state
	DELETED RequestState = "DELETED"
	// UNKNOWN state
	UNKNOWN RequestState = "UNKNOWN"
)

// NewRequestState constructor
func NewRequestState(state string) RequestState {
	return RequestState(strings.ToUpper(state))
}

// IsTerminal returns true if state is terminal
func (rs RequestState) IsTerminal() bool {
	return rs == COMPLETED || rs == FAILED || rs == CANCELLED
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

// Failed failed status
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
	return rs == STARTED || rs == EXECUTING
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
		rs != SUSPENDED
}
