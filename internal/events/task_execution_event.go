package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"plexobject.com/formicary/internal/types"
)

// TaskExecutionLifecycleEvent is used to update lifecycle events of task execution
type TaskExecutionLifecycleEvent struct {
	BaseEvent
	UserID          string                 `json:"user_id"`
	JobRequestID    string                 `json:"job_request_id"`    // JobRequestID defines key for job request
	JobType         string                 `json:"job_type"`          // JobType defines type of job
	JobExecutionID  string                 `json:"job_execution_id"`  // JobExecutionID
	TaskExecutionID string                 `json:"task_execution_id"` // TaskExecutionID
	TaskType        string                 `json:"task_type"`         // TaskType defines type of job
	TaskState       types.RequestState     `json:"task_state"`        // TaskState
	ExitCode        string                 `json:"exit_code"`         // ExitCode
	AntID           string                 `json:"ant_id"`            // AntID
	Contexts        map[string]interface{} `json:"contexts"`          // Contexts defines context variables of job
}

// NewTaskExecutionLifecycleEvent constructor
func NewTaskExecutionLifecycleEvent(
	source string,
	userID string,
	requestID string,
	jobType string,
	jobExecutionID string,
	taskType string,
	taskState types.RequestState,
	exitCode string,
	antID string,
	contexts map[string]interface{}) *TaskExecutionLifecycleEvent {
	return &TaskExecutionLifecycleEvent{
		BaseEvent: BaseEvent{
			ID:        ulid.Make().String(),
			Source:    source,
			EventType: "TaskExecutionLifecycleEvent",
			CreatedAt: time.Now(),
		},
		UserID:         userID,
		JobRequestID:   requestID,
		JobType:        jobType,
		JobExecutionID: jobExecutionID,
		TaskType:       taskType,
		TaskState:      taskState,
		ExitCode:       exitCode,
		AntID:          antID,
		Contexts:       contexts,
	}
}

// String format
func (telc *TaskExecutionLifecycleEvent) String() string {
	return fmt.Sprintf("RequestID=%s JobType=%s JobExecution=%s TaskType=%s TaskState=%s AntID=%s",
		telc.JobRequestID, telc.JobType, telc.JobExecutionID, telc.TaskType, telc.TaskState, telc.AntID)
}

// Validate validates event for lifecycle job
func (telc *TaskExecutionLifecycleEvent) Validate() error {
	if telc.JobExecutionID == "" {
		return fmt.Errorf("jobExecutionID is not specified")
	}
	if telc.JobRequestID == "" {
		return fmt.Errorf("requestID is not specified")
	}
	if telc.TaskType == "" {
		return fmt.Errorf("taskType is not specified")
	}
	if telc.TaskState == "" {
		return fmt.Errorf("task state is not specified")
	}
	return nil
}

// UnmarshalTaskExecutionLifecycleEvent unmarshal
func UnmarshalTaskExecutionLifecycleEvent(b []byte) (*TaskExecutionLifecycleEvent, error) {
	var event TaskExecutionLifecycleEvent
	if err := json.Unmarshal(b, &event); err != nil {
		return nil, err
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return &event, nil
}

// Marshal serializes event
func (telc *TaskExecutionLifecycleEvent) Marshal() ([]byte, error) {
	if err := telc.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(telc)
}

// Key of task request
func (telc *TaskExecutionLifecycleEvent) Key() string {
	return types.TaskKey(telc.JobRequestID, telc.TaskType)
}
