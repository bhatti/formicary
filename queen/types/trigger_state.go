// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import "time"

// TriggerState persists per-trigger runtime state: S3 poll marker and rate-limit window.
// This is the GORM-backed internal type; the proto type (queen.TriggerState) is used in the gRPC layer.
//
// The composite unique index (job_definition_id, trigger_name) is enforced by the SQL migration;
// the GORM tag here mirrors it for AutoMigrate in test environments.
type TriggerState struct {
	// ID is a 26-char ULID string.
	ID              string    `json:"id" gorm:"primaryKey;size:128"`
	JobDefinitionID string    `json:"job_definition_id" gorm:"not null;size:128;uniqueIndex:uq_trigger_states_job_name"`
	TriggerName     string    `json:"trigger_name" gorm:"not null;size:255;uniqueIndex:uq_trigger_states_job_name"`
	// LastSeenKey is the S3 object key of the last successfully processed object (poll cursor).
	LastSeenKey     string    `json:"last_seen_key" gorm:"not null;default:''"`
	LastSeenTime    time.Time `json:"last_seen_time"`
	// WindowStart is the beginning of the current rate-limit window.
	WindowStart     time.Time `json:"window_start"`
	WindowCount     int32     `json:"window_count" gorm:"not null;default:0"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TableName overrides the GORM table name.
func (TriggerState) TableName() string {
	return "formicary_trigger_states"
}
