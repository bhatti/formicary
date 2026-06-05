// SPDX-License-Identifier: AGPL-3.0-or-later

package repository

import (
	"time"

	"plexobject.com/formicary/queen/types"
)

// TriggerStateRepository provides persistence for per-trigger runtime state.
type TriggerStateRepository interface {
	// FindByJobDefinitionID returns all TriggerState rows for a job definition.
	FindByJobDefinitionID(jobDefinitionID string) ([]*types.TriggerState, error)
	// FindByJobAndTrigger returns the TriggerState for one trigger, or nil if it doesn't exist yet.
	FindByJobAndTrigger(jobDefinitionID, triggerName string) (*types.TriggerState, error)
	// Upsert saves a TriggerState, inserting or updating as needed.
	Upsert(state *types.TriggerState) (*types.TriggerState, error)
	// IncrementWindowCount increments the rate-limit window counter for the given trigger.
	// It resets the window when the current window has expired (older than windowDuration).
	// Returns the new count after the increment (approximate under high concurrency — the
	// UPDATE is atomic, but the returned value is read in a separate query).
	IncrementWindowCount(jobDefinitionID, triggerName string, windowDuration time.Duration) (newCount int32, err error)
	// RecordFired updates LastSeenTime to now after a trigger successfully fires a job request.
	// Creates the row if it doesn't exist yet.
	RecordFired(jobDefinitionID, triggerName string) error
	// Reset clears LastSeenKey, LastSeenTime, WindowStart, and WindowCount for a trigger.
	// For S3 poll triggers this also resets the key cursor so old objects are re-processed.
	// For webhook/queue triggers this resets the rate-limit window counter.
	Reset(jobDefinitionID, triggerName string) error
	// DeleteByJobDefinitionID removes all trigger states for a job definition.
	DeleteByJobDefinitionID(jobDefinitionID string) error
}
