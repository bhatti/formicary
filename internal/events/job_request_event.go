package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"plexobject.com/formicary/internal/types"
)

// JobRequestLifecycleEvent is used to update lifecycle events of job execution
type JobRequestLifecycleEvent struct {
	BaseEvent
	UserID string `json:"user_id"`
	// JobRequestID defines key for job request
	JobRequestID string `json:"job_request_id"`
	// JobType defines type of job
	JobType string `json:"job_type"`
	// JobState defines state of job that is maintained throughout the lifecycle of a job
	JobState types.RequestState `json:"job_state"`
	// Params defines parameters job
	Params map[string]interface{} `json:"params"`
}

// NewJobRequestLifecycleEvent constructor
func NewJobRequestLifecycleEvent(
	source string,
	userID string,
	requestID string,
	jobType string,
	jobState types.RequestState,
	params map[string]interface{}) *JobRequestLifecycleEvent {
	return &JobRequestLifecycleEvent{
		BaseEvent: BaseEvent{
			ID:        ulid.Make().String(),
			Source:    source,
			EventType: "JobRequestLifecycleEvent",
			CreatedAt: time.Now(),
		},
		UserID:       userID,
		JobRequestID: requestID,
		JobType:      jobType,
		JobState:     jobState,
		Params:       params,
	}
}

// String format
func (jelc *JobRequestLifecycleEvent) String() string {
	return fmt.Sprintf("JobRequestID=%s JobType=%s JobState=%s",
		jelc.JobRequestID, jelc.JobType, jelc.JobState)
}

// UnmarshalJobRequestLifecycleEvent unmarshal
func UnmarshalJobRequestLifecycleEvent(b []byte) (*JobRequestLifecycleEvent, error) {
	var event JobRequestLifecycleEvent
	if err := json.Unmarshal(b, &event); err != nil {
		return nil, err
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return &event, nil
}

// Validate validates event for lifecycle job
func (jelc *JobRequestLifecycleEvent) Validate() error {
	if jelc.JobRequestID == "" {
		return fmt.Errorf("requestID is not specified")
	}
	if jelc.JobType == "" {
		return fmt.Errorf("jobType is not specified")
	}
	if jelc.JobState == "" {
		return fmt.Errorf("job state is not specified")
	}
	return nil
}

// Marshal serializes event
func (jelc *JobRequestLifecycleEvent) Marshal() ([]byte, error) {
	if err := jelc.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(jelc)
}
