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
		id string) (*types.JobRequest, error)
	// GetParams by id
	GetParams(
		id string) ([]*types.JobRequestParam, error)
	// GetByUserKey JobRequest by user-key
	GetByUserKey(
		qc *common.QueryContext,
		userKey string) (*types.JobRequest, error)
	// UpdateJobState sets state of job-request
	UpdateJobState(
		id string,
		oldState common.RequestState,
		newState common.RequestState,
		errorMessage string,
		errorCode string,
		scheduleDelay time.Duration,
		retried int) error

	// Save saves job-request
	Save(
		qc *common.QueryContext,
		req *types.JobRequest) (*types.JobRequest, error)
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
		id string,
		priority int32) error
	// SetReadyToExecute marks job as ready to execute
	SetReadyToExecute(
		id string,
		jobExecutionID string,
		lastJobExecutionID string) error
	// IncrementScheduleAttempts and optionally bump schedule time and decrement priority for jobs that are not ready
	IncrementScheduleAttempts(
		id string,
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
	// NextSchedulableJobsByTypes returns next ready to schedule job types and state
	NextSchedulableJobsByTypes(
		jobTypes []string,
		state []common.RequestState,
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
	UpdateRunningTimestamp(id string) error
	CountByOrgAndState(
		org string,
		state common.RequestState) (totalRecords int64, err error)
	// Cancel - cancel a job
	Cancel(
		qc *common.QueryContext,
		id string) error
	// Trigger triggers a scheduled job
	Trigger(
		qc *common.QueryContext,
		id string) error
	// Restart restarts a job
	Restart(
		qc *common.QueryContext,
		id string) error
	// Delete removes a job
	Delete(
		qc *common.QueryContext,
		id string) error
	// DeletePendingCronByJobType - delete pending cron job
	DeletePendingCronByJobType(
		qc *common.QueryContext,
		jobType string) error
	// RecentIDs returns job ids
	RecentIDs(
		limit int) (map[string]common.RequestState, error)
	// RecentLiveIDs returns recently alive - executing/pending/starting job-ids
	RecentLiveIDs(
		limit int) ([]string, error)
	// RecentDeadIDs returns recently completed job-ids
	RecentDeadIDs(
		limit int,
		fromOffset time.Duration,
		toOffset time.Duration,
	) ([]string, error)
}
