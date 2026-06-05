// SPDX-License-Identifier: AGPL-3.0-or-later

package repository

import (
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
	"plexobject.com/formicary/queen/types"
)

var _ TriggerStateRepository = &TriggerStateRepositoryImpl{}

// TriggerStateRepositoryImpl implements TriggerStateRepository using GORM.
type TriggerStateRepositoryImpl struct {
	db *gorm.DB
}

// NewTriggerStateRepositoryImpl creates a new TriggerStateRepositoryImpl.
func NewTriggerStateRepositoryImpl(db *gorm.DB) (*TriggerStateRepositoryImpl, error) {
	return &TriggerStateRepositoryImpl{db: db}, nil
}

// FindByJobDefinitionID returns all TriggerState rows for a job definition.
func (r *TriggerStateRepositoryImpl) FindByJobDefinitionID(jobDefinitionID string) ([]*types.TriggerState, error) {
	if jobDefinitionID == "" {
		return nil, fmt.Errorf("job_definition_id is required")
	}
	var states []*types.TriggerState
	res := r.db.Where("job_definition_id = ?", jobDefinitionID).Find(&states)
	if res.Error != nil {
		return nil, res.Error
	}
	return states, nil
}

// FindByJobAndTrigger returns the TriggerState for one trigger, or nil if not found.
func (r *TriggerStateRepositoryImpl) FindByJobAndTrigger(jobDefinitionID, triggerName string) (*types.TriggerState, error) {
	if jobDefinitionID == "" || triggerName == "" {
		return nil, fmt.Errorf("job_definition_id and trigger_name are required")
	}
	var state types.TriggerState
	res := r.db.Where("job_definition_id = ? AND trigger_name = ?", jobDefinitionID, triggerName).First(&state)
	if res.Error != nil {
		if res.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, res.Error
	}
	return &state, nil
}

// Upsert saves a TriggerState, inserting or updating as needed.
func (r *TriggerStateRepositoryImpl) Upsert(state *types.TriggerState) (*types.TriggerState, error) {
	if state == nil {
		return nil, fmt.Errorf("trigger state is required")
	}
	now := time.Now()
	if state.ID == "" {
		state.ID = ulid.Make().String()
		state.CreatedAt = now
	}
	state.UpdatedAt = now
	res := r.db.Save(state)
	if res.Error != nil {
		return nil, res.Error
	}
	return state, nil
}

// IncrementWindowCount increments the rate-limit window counter for the given trigger.
// Uses a DB-level conditional UPDATE to prevent TOCTOU races under concurrent webhook hits.
// If the window has expired or no row exists, it resets the window first.
// The returned count is approximate under high concurrency — the UPDATE is atomic,
// but the returned value is read in a separate query.
func (r *TriggerStateRepositoryImpl) IncrementWindowCount(
	jobDefinitionID, triggerName string,
	windowDuration time.Duration,
) (int32, error) {
	if jobDefinitionID == "" || triggerName == "" {
		return 0, fmt.Errorf("job_definition_id and trigger_name are required")
	}
	now := time.Now()
	windowExpiry := now.Add(-windowDuration)

	// Attempt to increment within an active window.
	res := r.db.Model(&types.TriggerState{}).
		Where("job_definition_id = ? AND trigger_name = ? AND window_start >= ?",
			jobDefinitionID, triggerName, windowExpiry).
		UpdateColumn("window_count", gorm.Expr("window_count + 1"))
	if res.Error != nil {
		return 0, res.Error
	}

	if res.RowsAffected == 0 {
		// Either no row exists yet, or the window expired — reset and set count=1.
		// Use Save (upsert by PK) after FindOrInit pattern.
		state, err := r.FindByJobAndTrigger(jobDefinitionID, triggerName)
		if err != nil {
			return 0, err
		}
		if state == nil {
			state = &types.TriggerState{
				JobDefinitionID: jobDefinitionID,
				TriggerName:     triggerName,
			}
		}
		state.WindowStart = now
		state.WindowCount = 1
		if _, err = r.Upsert(state); err != nil {
			return 0, err
		}
		return 1, nil
	}

	// Re-read the current count after the increment.
	var updated types.TriggerState
	res2 := r.db.Where("job_definition_id = ? AND trigger_name = ?", jobDefinitionID, triggerName).
		First(&updated)
	if res2.Error != nil {
		return 0, res2.Error
	}
	return updated.WindowCount, nil
}

// RecordFired updates LastSeenTime to now. Creates the row if it doesn't exist yet.
func (r *TriggerStateRepositoryImpl) RecordFired(jobDefinitionID, triggerName string) error {
	if jobDefinitionID == "" || triggerName == "" {
		return fmt.Errorf("job_definition_id and trigger_name are required")
	}
	now := time.Now()
	res := r.db.Model(&types.TriggerState{}).
		Where("job_definition_id = ? AND trigger_name = ?", jobDefinitionID, triggerName).
		Updates(map[string]interface{}{
			"last_seen_time": now,
			"updated_at":     now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		// No row yet — create one.
		state := &types.TriggerState{
			ID:              ulid.Make().String(),
			JobDefinitionID: jobDefinitionID,
			TriggerName:     triggerName,
			LastSeenTime:    now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		return r.db.Create(state).Error
	}
	return nil
}

// Reset clears poll marker and rate-limit counters for a trigger.
// No error is returned if the trigger has no state row yet (idempotent).
func (r *TriggerStateRepositoryImpl) Reset(jobDefinitionID, triggerName string) error {
	if jobDefinitionID == "" || triggerName == "" {
		return fmt.Errorf("job_definition_id and trigger_name are required")
	}
	var zero time.Time
	res := r.db.Model(&types.TriggerState{}).
		Where("job_definition_id = ? AND trigger_name = ?", jobDefinitionID, triggerName).
		Updates(map[string]interface{}{
			"last_seen_key":  "",
			"last_seen_time": zero,
			"window_start":   zero,
			"window_count":   0,
			"updated_at":     time.Now(),
		})
	return res.Error
}

// DeleteByJobDefinitionID removes all trigger states for a job definition.
func (r *TriggerStateRepositoryImpl) DeleteByJobDefinitionID(jobDefinitionID string) error {
	if jobDefinitionID == "" {
		return fmt.Errorf("job_definition_id is required")
	}
	return r.db.Where("job_definition_id = ?", jobDefinitionID).Delete(&types.TriggerState{}).Error
}
