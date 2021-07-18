package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/twinj/uuid"
)

// JobDefinitionStateChange defines enum for state of definition
type JobDefinitionStateChange string

// Types of request states
const (
	// UPDATED request
	UPDATED JobDefinitionStateChange = "UPDATED"
	// DELETED request
	DELETED JobDefinitionStateChange = "DELETED"
	// PAUSED request
	PAUSED JobDefinitionStateChange = "PAUSED"
	// UNPAUSED request
	UNPAUSED JobDefinitionStateChange = "UNPAUSED"
)

// JobDefinitionLifecycleEvent is used to update lifecycle events of job execution
type JobDefinitionLifecycleEvent struct {
	BaseEvent
	UserID string `json:"user_id"`
	// JobDefinitionID defines key for job definition
	JobDefinitionID string `json:"job_definition_id"`
	// JobType defines type of job
	JobType string `json:"job_type"`
	// StateChange defines type of event
	StateChange JobDefinitionStateChange `json:"state_change"`
}

// NewJobDefinitionLifecycleEvent constructor
func NewJobDefinitionLifecycleEvent(
	source string,
	userID string,
	definitionID string,
	jobType string,
	stateChange JobDefinitionStateChange) *JobDefinitionLifecycleEvent {
	return &JobDefinitionLifecycleEvent{
		BaseEvent: BaseEvent{
			ID:        uuid.NewV4().String(),
			Source:    source,
			EventType: "JobDefinitionLifecycleEvent",
			CreatedAt: time.Now(),
		},
		UserID:          userID,
		JobDefinitionID: definitionID,
		JobType:         jobType,
		StateChange:     stateChange,
	}
}

// String format
func (jelc *JobDefinitionLifecycleEvent) String() string {
	return fmt.Sprintf("JobDefinitionID=%s JobType=%s StateChange=%s",
		jelc.JobDefinitionID, jelc.JobType, jelc.StateChange)
}

// UnmarshalJobDefinitionLifecycleEvent unmarshal
func UnmarshalJobDefinitionLifecycleEvent(b []byte) (*JobDefinitionLifecycleEvent, error) {
	var event JobDefinitionLifecycleEvent
	if err := json.Unmarshal(b, &event); err != nil {
		return nil, err
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return &event, nil
}

// Validate validates event for lifecycle job
func (jelc *JobDefinitionLifecycleEvent) Validate() error {
	if jelc.JobDefinitionID == "" {
		return fmt.Errorf("definitionID is not specified")
	}
	if jelc.JobType == "" {
		return fmt.Errorf("jobType is not specified")
	}
	if jelc.StateChange == "" {
		return fmt.Errorf("stateChange is not specified")
	}
	return nil
}

// Marshal serializes event
func (jelc *JobDefinitionLifecycleEvent) Marshal() ([]byte, error) {
	if err := jelc.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(jelc)
}
