package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/twinj/uuid"
	"plexobject.com/formicary/internal/types"
)

// TaskExecutionLifecycleEvent is used to update lifecycle events of task execution
type TaskExecutionLifecycleEvent struct {
	BaseEvent
	UserID string `json:"user_id"`
	// JobRequestID defines key for job request
	JobRequestID uint64 `json:"job_request_id"`
	// JobType defines type of job
	JobType string `json:"job_type"`
	// JobExecutionID
	JobExecutionID string `json:"job_execution_id"`
	// TaskExecutionID
	TaskExecutionID string `json:"task_execution_id"`
	// TaskType defines type of job
	TaskType string `json:"task_type"`
	// TaskState
	TaskState types.RequestState `json:"task_state"`
	// ExitCode
	ExitCode string `json:"exit_code"`
	// AntID
	AntID string `json:"ant_id"`
	// Contexts defines context variables of job
	Contexts map[string]interface{} `json:"contexts"`
}

// NewTaskExecutionLifecycleEvent constructor
func NewTaskExecutionLifecycleEvent(
	source string,
	userID string,
	requestID uint64,
	jobType string,
	jobExecutionID string,
	taskType string,
	taskState types.RequestState,
	exitCode string,
	antID string,
	contexts map[string]interface{}) *TaskExecutionLifecycleEvent {
	return &TaskExecutionLifecycleEvent{
		BaseEvent: BaseEvent{
			ID:        uuid.NewV4().String(),
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
	return fmt.Sprintf("RequestID=%d JobType=%s JobExecution=%s TaskType=%s TaskState=%s AntID=%s",
		telc.JobRequestID, telc.JobType, telc.JobExecutionID, telc.TaskType, telc.TaskState, telc.AntID)
}

// Validate validates event for lifecycle job
func (telc *TaskExecutionLifecycleEvent) Validate() error {
	if telc.JobExecutionID == "" {
		return fmt.Errorf("jobExecutionID is not specified")
	}
	if telc.JobRequestID == 0 {
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
