package stats

import (
	"plexobject.com/formicary/queen/types"
	"time"
)

// JobWaitEstimate stores estimated wait times for given job-request.
type JobWaitEstimate struct {
	JobStats   *JobStats
	JobRequest *types.JobRequestInfo
	// QueueNumber number in queue
	QueueNumber int `json:"queue_number"`
	// EstimatedWait wait time
	EstimatedWait time.Duration `json:"estimated_wait"`
	// ScheduledAt - schedule time
	ScheduledAt time.Time `json:"scheduled_at"`
	// ErrorMessage
	ErrorMessage string `json:"error_message"`
	// PendingJobIDs
	PendingJobIDs []uint64 `json:"pending_job_ids"`
}

// PendingJobs returns number of pending jobs
func (j JobWaitEstimate) PendingJobs() int {
	return len(j.PendingJobIDs)
}
