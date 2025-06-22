package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"

	"plexobject.com/formicary/internal/types"
)

// JobExecutionLaunchEvent is used to launch execution of a job
type JobExecutionLaunchEvent struct {
	BaseEvent
	UserID         string                           `json:"user_id"`
	JobRequestID   string                           `json:"job_request_id"`   // JobRequestID defines key for job request
	JobType        string                           `json:"job_type"`         // JobType defines type of job
	JobExecutionID string                           `json:"job_execution_id"` // JobExecutionID
	Reservations   map[string]*types.AntReservation `json:"reservations"`     // Reservations by task-types
}

// NewJobExecutionLaunchEvent constructor
func NewJobExecutionLaunchEvent(
	source string,
	userID string,
	requestID string,
	jobType string,
	jobExecutionID string,
	reservations map[string]*types.AntReservation) *JobExecutionLaunchEvent {
	return &JobExecutionLaunchEvent{
		BaseEvent: BaseEvent{
			ID:        ulid.Make().String(),
			Source:    source,
			CreatedAt: time.Now(),
		},
		UserID:         userID,
		JobRequestID:   requestID,
		JobType:        jobType,
		JobExecutionID: jobExecutionID,
		Reservations:   reservations,
	}
}

// UnmarshalJobExecutionLaunchEvent unmarshal
func UnmarshalJobExecutionLaunchEvent(b []byte) (*JobExecutionLaunchEvent, error) {
	var event JobExecutionLaunchEvent
	if err := json.Unmarshal(b, &event); err != nil {
		return nil, err
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return &event, nil
}

// String format
func (jie *JobExecutionLaunchEvent) String() string {
	return fmt.Sprintf("RequestID=%s JobType=%s JobExecution=%s",
		jie.JobRequestID, jie.JobType, jie.JobExecutionID)
}

// Validate validates event for launching job
func (jie *JobExecutionLaunchEvent) Validate() error {
	if jie.JobExecutionID == "" {
		return fmt.Errorf("jobExecutionID is not specified")
	}
	if jie.JobRequestID == "" {
		return fmt.Errorf("requestID is not specified")
	}
	if jie.JobType == "" {
		return fmt.Errorf("jobType is not specified")
	}
	return nil
}

// Marshal serializes event
func (jie *JobExecutionLaunchEvent) Marshal() ([]byte, error) {
	if err := jie.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(jie)
}

// JobExecutionLifecycleEvent is used to update lifecycle events of job execution
type JobExecutionLifecycleEvent struct {
	BaseEvent
	UserID string `json:"user_id"`
	// JobRequestID defines key for job request
	JobRequestID string `json:"job_request_id"`
	// JobType defines type of job
	JobType string `json:"job_type"`
	// JobExecutionID
	JobExecutionID string `json:"job_execution_id"`
	// JobState defines state of job that is maintained throughout the lifecycle of a job
	JobState types.RequestState `json:"job_state"`
	// JobPriority
	JobPriority int `json:"job_priority"`
	// Contexts defines context variables of job
	Contexts map[string]interface{} `json:"contexts"`
}

// NewJobExecutionLifecycleEvent constructor
func NewJobExecutionLifecycleEvent(
	source string,
	userID string,
	requestID string,
	jobType string,
	jobExecutionID string,
	jobState types.RequestState,
	jobPriority int,
	contexts map[string]interface{}) *JobExecutionLifecycleEvent {
	return &JobExecutionLifecycleEvent{
		BaseEvent: BaseEvent{
			ID:        ulid.Make().String(),
			Source:    source,
			EventType: "JobExecutionLifecycleEvent",
			CreatedAt: time.Now(),
		},
		UserID:         userID,
		JobRequestID:   requestID,
		JobType:        jobType,
		JobExecutionID: jobExecutionID,
		JobState:       jobState,
		JobPriority:    jobPriority,
		Contexts:       contexts,
	}
}

// String format
func (jelc *JobExecutionLifecycleEvent) String() string {
	return fmt.Sprintf("JobRequestID=%s JobType=%s JobExecution=%s JobState=%s",
		jelc.JobRequestID, jelc.JobType, jelc.JobExecutionID, jelc.JobState)
}

// UnmarshalJobExecutionLifecycleEvent unmarshal
func UnmarshalJobExecutionLifecycleEvent(b []byte) (*JobExecutionLifecycleEvent, error) {
	var event JobExecutionLifecycleEvent
	if err := json.Unmarshal(b, &event); err != nil {
		return nil, err
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return &event, nil
}

// Validate validates event for lifecycle job
func (jelc *JobExecutionLifecycleEvent) Validate() error {
	if jelc.JobExecutionID == "" {
		return fmt.Errorf("jobExecutionID is not specified")
	}
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
func (jelc *JobExecutionLifecycleEvent) Marshal() ([]byte, error) {
	if err := jelc.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(jelc)
}
