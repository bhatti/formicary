package events

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"time"

	"github.com/oklog/ulid/v2"
)

// LogEvent is used to publish console logs
type LogEvent struct {
	BaseEvent
	UserID          string `json:"user_id"`
	JobRequestID    string `json:"job_request_id"`           // JobRequestID defines key for job request
	JobType         string `json:"job_type"`                 // JobType defines type of job
	TaskType        string `json:"task_type"`                // TaskType defines type of job
	JobExecutionID  string `json:"job_execution_id"`         // JobExecutionID defines foreign key for JobExecution
	TaskExecutionID string `json:"task_execution_id"`        // TaskExecutionID defines foreign key for TaskExecution
	AntID           string `json:"ant_id"`                   // AntID
	Tags            string `json:"tags"`                     // Tags
	Message         string `json:"message" gorm:"-"`         // Message
	EncodedMessage  string `json:"-" gorm:"encoded_message"` // EncodedMessage
}

// NewLogEvent constructor
func NewLogEvent(
	source string,
	userID string,
	requestID string,
	jobType string,
	taskType string,
	jobExecutionID string,
	taskExecutionID string,
	msg string,
	tags string,
	antID string) *LogEvent {
	return &LogEvent{
		BaseEvent: BaseEvent{
			ID:        ulid.Make().String(),
			Source:    source,
			EventType: "LogEvent",
			CreatedAt: time.Now(),
		},
		UserID:          userID,
		JobRequestID:    requestID,
		JobType:         jobType,
		TaskType:        taskType,
		JobExecutionID:  jobExecutionID,
		TaskExecutionID: taskExecutionID,
		Message:         msg,
		Tags:            tags,
		AntID:           antID,
	}
}

// TableName overrides default table name
func (LogEvent) TableName() string {
	return "formicary_log_events"
}

// String format
func (l *LogEvent) String() string {
	return fmt.Sprintf("RequestID=%s JobType=%s TaskType=%s AntID=%s Message=%s",
		l.JobRequestID, l.JobType, l.TaskType, l.AntID, l.Message)
}

// Validate validates event for message event
func (l *LogEvent) Validate() error {
	if l.JobRequestID == "" {
		return fmt.Errorf("requestID is not specified")
	}
	if l.TaskType == "" {
		return fmt.Errorf("taskType is not specified")
	}
	if l.JobExecutionID == "" {
		return fmt.Errorf("jobExecutionID is not specified")
	}
	if l.TaskExecutionID == "" {
		return fmt.Errorf("taskExecutionID is not specified")
	}
	if l.Message == "" {
		return fmt.Errorf("message is not specified")
	}
	l.EncodedMessage = base64.StdEncoding.EncodeToString([]byte(l.Message))
	return nil
}

// AfterLoad initializes message
func (l *LogEvent) AfterLoad() {
	decodedString, err := base64.StdEncoding.DecodeString(l.EncodedMessage)
	if err == nil {
		l.Message = string(decodedString)
	} else {
		logrus.WithFields(logrus.Fields{
			"Component": "LogEvent",
			"Encoded":   l.EncodedMessage,
			"Error":     err,
		}).Warnf("failed to decode log message")
	}
}

// UnmarshalLogEvent unmarshal
func UnmarshalLogEvent(b []byte) (*LogEvent, error) {
	var event LogEvent
	if err := json.Unmarshal(b, &event); err != nil {
		return nil, err
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return &event, nil
}

// Marshal serializes event
func (l *LogEvent) Marshal() ([]byte, error) {
	if err := l.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(l)
}
