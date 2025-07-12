package repository

import (
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
	"time"
)

// JobExecutionRepository defines data access methods for job-definition
type JobExecutionRepository interface {
	// Get - finds JobExecution by id
	Get(id string) (*types.JobExecution, error)
	// Save - saves job-definition
	Save(job *types.JobExecution) (*types.JobExecution, error)
	// Query - queries job-definition by parameters
	Query(
		params map[string]interface{},
		page int,
		pageSize int,
		order []string) (jobs []*types.JobExecution, totalRecords int64, err error)
	// Count - counts records by query
	Count(params map[string]interface{}) (totalRecords int64, err error)
	// Delete job execution
	Delete(id string) error
	// DeleteTask task execution
	DeleteTask(taskID string) error
	// SaveTask saves task execution
	SaveTask(task *types.TaskExecution) (*types.TaskExecution, error)
	// UpdateJobContext updates context of job-execution
	UpdateJobContext(id string, contexts []*types.JobExecutionContext) error
	// FinalizeJobRequestAndExecutionState updates final state of job-execution and job-request
	FinalizeJobRequestAndExecutionState(
		id string,
		oldState common.RequestState,
		newState common.RequestState,
		errorMessage string,
		errorCode string,
		elapsed int64,
		scheduleDelay time.Duration,
		retried int,
	) error
	// UpdateJobRequestAndExecutionState updates intermediate state of job-execution and job-request
	UpdateJobRequestAndExecutionState(
		id string,
		oldState common.RequestState,
		newState common.RequestState,
		manualTaskType string) error
	// ResetStateToReady resets state to ready
	ResetStateToReady(id string) error
	// UpdateTaskState sets state of task-execution
	UpdateTaskState(
		id string,
		oldState common.RequestState,
		newState common.RequestState) error
	// GetResourceUsageByOrgUser - Finds usage between time by user and organization
	GetResourceUsageByOrgUser(
		ranges []types.DateRange,
		limit int) ([]types.ResourceUsage, error)
	// GetResourceUsage usage
	GetResourceUsage(
		qc *common.QueryContext,
		ranges []types.DateRange) ([]types.ResourceUsage, error)

	ResumeFromManualApproval(request types.ReviewTaskRequest) error
}
