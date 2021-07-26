package repository

import (
	"time"

	common "plexobject.com/formicary/internal/types"

	"plexobject.com/formicary/queen/types"
)

// JobRequestRepository defines request to process a job
type JobRequestRepository interface {
	// Get JobRequest by id
	Get(
		qc *common.QueryContext,
		id uint64) (*types.JobRequest, error)
	// GetByUserKey JobRequest by user-key
	GetByUserKey(
		qc *common.QueryContext,
		userKey string) (*types.JobRequest, error)
	// UpdateJobState sets state of job-request
	UpdateJobState(
		id uint64,
		oldState common.RequestState,
		newState common.RequestState,
		errorMessage string,
		errorCode string) error

	// Save saves job-request
	Save(req *types.JobRequest) (*types.JobRequest, error)
	// Query - Queries job-request by parameters
	Query(
		qc *common.QueryContext,
		params map[string]interface{},
		page int,
		pageSize int,
		order []string) (jobRequests []*types.JobRequest, totalRecords int64, err error)
	// Count - counts records by query
	Count(
		qc *common.QueryContext,
		params map[string]interface{}) (totalRecords int64, err error)
	// UpdatePriority update priority
	UpdatePriority(
		qc *common.QueryContext,
		id uint64,
		priority int32) error
	// SetReadyToExecute marks job as ready to execute
	SetReadyToExecute(
		id uint64,
		jobExecutionID string,
		lastJobExecutionID string) error
	// IncrementScheduleAttempts and optionally bump schedule time and decrement priority for jobs that are not ready
	IncrementScheduleAttempts(
		id uint64,
		scheduleSecs time.Duration,
		decrPriority int,
		errorMessage string) error
	// JobCountsByDays calculates stats for all job-types/statuses/error-codes within days
	JobCountsByDays(
		qc *common.QueryContext,
		days int,
	) ([]*types.JobCounts, error)
	// JobCounts counts of jobs
	JobCounts(
		qc *common.QueryContext,
		start time.Time,
		end time.Time) ([]*types.JobCounts, error)
	// FindActiveCronScheduledJobsByJobType queries scheduled jobs that are either running or waiting to be run
	FindActiveCronScheduledJobsByJobType(
		jobTypes []types.JobTypeCronTrigger,
	) ([]*types.JobRequestInfo, error)
	// NextSchedulableJobsByType returns next ready to schedule job types and state
	NextSchedulableJobsByType(
		jobTypes []string,
		state common.RequestState,
		limit int) ([]*types.JobRequestInfo, error)
	// GetJobTimes finds job times
	GetJobTimes(
		limit int) ([]*types.JobTime, error)
	// RequeueOrphanRequests queries jobs with EXECUTING/STARTED status and puts them back to PENDING
	RequeueOrphanRequests(
		staleInterval time.Duration) (total int64, err error)
	// QueryOrphanRequests queries jobs with EXECUTING/STARTED status but haven't updated since interval
	QueryOrphanRequests(
		limit int,
		offset int,
		staleInterval time.Duration) (jobRequests []*types.JobRequest, err error)
	// UpdateRunningTimestamp updates running timestamp of STARTED and EXECUTING jobs
	UpdateRunningTimestamp(id uint64) error
	CountByOrgAndState(
		org string,
		state common.RequestState) (totalRecords int64, err error)
	// Cancel - cancel a job
	Cancel(
		qc *common.QueryContext,
		id uint64) error
	// Restart restarts a job
	Restart(
		qc *common.QueryContext,
		id uint64) error
	// Delete removes a job
	Delete(
		qc *common.QueryContext,
		id uint64) error
	// RecentDeadIDs returns recently completed job-ids
	RecentDeadIDs(
		limit int) ([]uint64, error)
}
