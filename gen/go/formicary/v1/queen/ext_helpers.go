// SPDX-License-Identifier: AGPL-3.0-or-later
// Shared helpers used by all _ext.go files in this package.
// This file is NEVER overwritten by buf generate.

package queen

import (
	"fmt"
	"math/rand"
	"time"

	ulidpkg "github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ulid generates a new ULID string for use as a primary key.
func ulid() string {
	entropy := ulidpkg.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0) //nolint:gosec
	return ulidpkg.MustNew(ulidpkg.Timestamp(time.Now()), entropy).String()
}

// nowTimestamp returns the current time as a protobuf Timestamp.
func nowTimestamp() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// ──────────────────────────────────────────────────────────────────────────────
// State machine predicates (shared by JobExecution, TaskExecution, JobRequest)
// ──────────────────────────────────────────────────────────────────────────────

// isTerminalState matches internal/types RequestState.IsTerminal().
func isTerminalState(s string) bool {
	return s == "COMPLETED" || s == "FAILED" || s == "CANCELLED"
}

// canRestart matches internal/types RequestState.CanRestart().
func canRestart(s string) bool {
	return s == "FAILED" || s == "CANCELLED" || s == "PAUSED"
}

// canCancel matches internal/types RequestState.CanCancel(): !terminal || PAUSED || MANUAL_APPROVAL_REQUIRED.
func canCancel(s string) bool {
	return !isTerminalState(s) || s == "PAUSED" || s == "MANUAL_APPROVAL_REQUIRED"
}

// canApprove matches internal/types RequestState.CanApprove().
func canApprove(s string) bool {
	return s == "MANUAL_APPROVAL_REQUIRED"
}

// ──────────────────────────────────────────────────────────────────────────────
// Key helpers (shared by JobDefinition and JobRequest)
// ──────────────────────────────────────────────────────────────────────────────

// getUserJobTypeKey builds the canonical org/user + job-type key.
func getUserJobTypeKey(orgID, userID, jobType, jobVersion string) string {
	prefix := orgID
	if prefix == "" {
		prefix = userID
	}
	if jobVersion != "" {
		return fmt.Sprintf("%s:%s:%s", prefix, jobType, jobVersion)
	}
	return fmt.Sprintf("%s:%s", prefix, jobType)
}

// ──────────────────────────────────────────────────────────────────────────────
// Type inference helper (shared by JobRequest param management)
// ──────────────────────────────────────────────────────────────────────────────

// kindOf infers the string kind from a Go value type.
func kindOf(v interface{}) string {
	switch v.(type) {
	case int, int32, int64, float32, float64:
		return "INT"
	case bool:
		return "BOOL"
	default:
		return "STRING"
	}
}
