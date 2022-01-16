package types

import (
	"fmt"
	"regexp"
	"time"
)

// ErrorCodeAction defines enum for actions that can be taken based on the error-code.
type ErrorCodeAction string

const (
	// SuspendJob action for error code
	SuspendJob ErrorCodeAction = "SUSPEND_JOB"
	// RetryJob action for error code
	RetryJob ErrorCodeAction = "RETRY_JOB"
	// RetryTask action for error code
	RetryTask ErrorCodeAction = "RETRY_TASK"
	// HardFailure action for error code
	HardFailure ErrorCodeAction = "HARD_FAILURE"
)

const (
	// ErrorJobExecute error code
	ErrorJobExecute = "ERR_JOB_EXECUTE"
	// ErrorQuotaExceeded error code
	ErrorQuotaExceeded = "ERR_QUOTA_EXCEEDED"
	// ErrorJobSchedule error code
	ErrorJobSchedule = "ERR_JOB_SCHEDULE"
	// ErrorJobCancelled error code
	ErrorJobCancelled = "ERR_JOB_CANCELLED"
	// ErrorAntsUnavailable error code
	ErrorAntsUnavailable = "ERR_ANTS_UNAVAILABLE"
	// ErrorTaskExecute error code
	ErrorTaskExecute = "ERR_TASK_EXECUTE"
	// ErrorInvalidNextTask error code
	ErrorInvalidNextTask = "ERR_INVALID_NEXT_TASK"
	// ErrorContainerNotFound error code
	ErrorContainerNotFound = "ERR_CONTAINER_NOT_FOUND"
	// ErrorContainerStoppedFailed error code
	ErrorContainerStoppedFailed = "ERR_CONTAINER_STOPPED_FAILED"
	// ErrorMarshalingFailed error code
	ErrorMarshalingFailed = "ERR_MARSHALING_FAILED"
	// ErrorAntExecutionFailed error code
	ErrorAntExecutionFailed = "ERR_ANT_EXECUTION_FAILED"
	// ErrorValidation error code
	ErrorValidation = "ERR_VALIDATION"
	// ErrorFilteredJob error code
	ErrorFilteredJob = "ERR_FILTERED_JOB"
	// ErrorAntResources error code
	ErrorAntResources = "ERR_ANT_RESOURCES"
	// ErrorFatal error code
	ErrorFatal = "ERR_FATAL"
	// ErrorRestartJob error code
	ErrorRestartJob = "ERR_RESTART_JOB"
	// ErrorRestartTask error code
	ErrorRestartTask = "ERR_RESTART_TASK"
	// ErrorTaskTimedOut  error code
	ErrorTaskTimedOut = "ERR_TASK_TIMED_OUT"
)

// ErrorCode defines codes for tracking different types of errors.
type ErrorCode struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID string `json:"id" gorm:"primary_key"`
	// Regex matches error-code
	Regex string `json:"regex"`
	// ExitCode defines exit-code for error
	ExitCode int `json:"exit_code"`
	// ErrorCode defines error code
	ErrorCode string `json:"error_code"`
	// Description of error
	Description string `json:"description"`
	// DisplayMessage defines user message for error
	DisplayMessage string `json:"display_message"`
	// DisplayCode defines user code for error
	DisplayCode string `json:"display_code"`
	// JobType defines type for the job
	JobType string `json:"job_type"`
	// TaskTypeScope only applies error code for task_type
	TaskTypeScope string `json:"task_type_scope"`
	// PlatformScope only applies error code for platform
	PlatformScope string `json:"platform_scope"`
	// CommandScope only applies error code for command
	CommandScope string `json:"command_scope"`
	// UserID defines user who owns the error code
	UserID string `json:"user_id"`
	// OrganizationID defines org who owns the error code
	OrganizationID string `json:"organization_id"`
	// Action defines actions for errors
	Action ErrorCodeAction `json:"action"`
	// HardFailure determines if this error can be retried or is hard failure
	HardFailure bool `json:"hard_failure"`
	// Retry defines number of tries if task is failed with this error code
	Retry int `json:"retry"`

	// CreatedAt job creation time
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt job update time
	UpdatedAt time.Time `json:"updated_at"`
	// Following are transient properties
	CanEdit bool              `yaml:"-" json:"-" gorm:"-"`
	Errors  map[string]string `yaml:"-" json:"-" gorm:"-"`
}

// NewErrorCode creates new instance of error-code
func NewErrorCode(
	jobType string,
	regex string,
	cmd string,
	errorCode string) *ErrorCode {
	return &ErrorCode{
		JobType:      jobType,
		Regex:        regex,
		CommandScope: cmd,
		ErrorCode:    errorCode,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// TableName overrides default table name
func (ErrorCode) TableName() string {
	return "formicary_error_codes"
}

// Editable checks if user can edit
func (ec *ErrorCode) Editable(userID string, organizationID string) bool {
	if ec.OrganizationID != "" || organizationID != "" {
		return ec.OrganizationID == organizationID
	}
	return ec.UserID == userID
}

// ShortID short id
func (ec *ErrorCode) ShortID() string {
	if len(ec.ID) > 8 {
		return ec.ID[0:8] + "..."
	}
	return ec.ID
}

// Matches matches error message
func (ec *ErrorCode) Matches(message string) bool {
	if message == "" {
		return false
	}
	if match, err := regexp.MatchString(ec.Regex, message); err == nil && match {
		return true
	}
	return false
}

// Validate checks error code for required properties
func (ec *ErrorCode) Validate() (err error) {
	ec.Errors = make(map[string]string)
	if ec.ErrorCode == "" {
		err = fmt.Errorf("errorCode is not specified")
		ec.Errors["ErrorCode"] = err.Error()
	}
	if ec.Regex == "" && ec.ExitCode == 0 {
		err = fmt.Errorf("regex or exitCode must be specified")
		ec.Errors["Regex"] = err.Error()
	}
	return
}

// ValidateBeforeSave validation
func (ec *ErrorCode) ValidateBeforeSave() error {
	return ec.Validate()
}
