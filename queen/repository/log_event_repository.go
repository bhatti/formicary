package repository

import "plexobject.com/formicary/internal/events"

// LogEventRepository defines data access methods for log-events
type LogEventRepository interface {
	// Query - queries logs by parameters
	Query(
		params map[string]interface{},
		page int,
		pageSize int,
		order []string) (jobs []*events.LogEvent, totalRecords int64, err error)
	// DeleteByRequestID delete all logs by request-id
	DeleteByRequestID(requestID uint64) (int64, error)
	// DeleteByJobExecutionID delete all logs by job-execution-id
	DeleteByJobExecutionID(jobExecutionID string) (int64, error)
	// DeleteByTaskExecutionID delete all logs by task-execution-id
	DeleteByTaskExecutionID(taskExecutionID string) (int64, error)
	// Save saves log events
	Save(job *events.LogEvent) (*events.LogEvent, error)
}
