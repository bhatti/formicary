package stats

import (
	"fmt"
	"sync/atomic"
	"time"

	"plexobject.com/formicary/internal/math"
	"plexobject.com/formicary/queen/types"
)

// JobStats captures usage statistics for each job-type.
type JobStats struct {
	// JobKey defines type of job
	JobKey types.UserJobTypeKey `json:"job_key"`
	// FirstJobAt time of job start
	FirstJobAt *time.Time `json:"first_job_at"`
	// LastJobAt update time of last job
	LastJobAt *time.Time `json:"last_job_at"`

	// SucceededJobsPercentages
	SucceededJobsPercentages int64 `json:"succeeded_jobs_percentage"`
	// SucceededJobs count
	SucceededJobs int64 `json:"succeeded_jobs"`
	// SucceededJobsAverage average
	SucceededJobsAverage float64 `json:"succeeded_jobs_average_latency"`
	// SucceededJobsMin min
	SucceededJobsMin int64 `json:"succeeded_jobs_min_latency"`
	// SucceededJobsMax max
	SucceededJobsMax int64 `json:"succeeded_jobs_max_latency"`

	// PausedJobs count
	PausedJobs int64 `json:"paused_jobs"`
	// FailedJobs count
	FailedJobs int64 `json:"failed_jobs"`
	// FailedJobsAverage average
	FailedJobsAverage float64 `json:"failed_jobs_average_latency"`
	// SailedJobsMin min
	FailedJobsMin int64 `json:"failed_jobs_min_latency"`
	// FailedJobsMax max
	FailedJobsMax int64 `json:"failed_jobs_max_latency"`

	// ExecutingJobs count
	ExecutingJobs int32 `json:"executing_jobs"`

	// AntsAvailable flag
	AntsAvailable bool `json:"ants_available"`

	// AntsCapacity
	AntsCapacity int `json:"ants_capacity"`

	// AntUnavailableError error
	AntUnavailableError string `json:"ant_unavailable_error"`

	// JobDisabled disabled flag
	JobDisabled bool `json:"job_disabled"`

	// succeededJobsMinMax average latency
	succeededJobsMinMax *math.RollingMinMax

	// failedJobsMinMax average latency
	failedJobsMinMax *math.RollingMinMax

	// pausedJobsMinMax average latency
	pausedJobsMinMax *math.RollingMinMax
}

// NewJobStats creates new instance of job-execution
func NewJobStats(jobKey types.UserJobTypeKey) *JobStats {
	return &JobStats{
		JobKey:              jobKey,
		succeededJobsMinMax: math.NewRollingMinMax(20),
		failedJobsMinMax:    math.NewRollingMinMax(20),
		pausedJobsMinMax:    math.NewRollingMinMax(20),
	}
}

// Started when job is started
func (j *JobStats) Started() {
	j.AntsAvailable = true
	j.AntUnavailableError = ""
	atomic.AddInt32(&j.ExecutingJobs, 1)
	now := time.Now()
	j.LastJobAt = &now
	if j.FirstJobAt == nil {
		j.FirstJobAt = &now
	}
}

// Cancelled when job is cancelled
func (j *JobStats) Cancelled() {
	atomic.AddInt32(&j.ExecutingJobs, -1)
}

// Succeeded when job is succeeded
func (j *JobStats) Succeeded(latency int64) {
	atomic.AddInt64(&j.SucceededJobs, 1)
	atomic.AddInt32(&j.ExecutingJobs, -1)
	j.succeededJobsMinMax.Add(latency)
}

// Failed when job is failed
func (j *JobStats) Failed(latency int64) {
	atomic.AddInt64(&j.FailedJobs, 1)
	atomic.AddInt32(&j.ExecutingJobs, -1)
	j.failedJobsMinMax.Add(latency)
}

// Paused when job is paused
func (j *JobStats) Paused(latency int64) {
	atomic.AddInt64(&j.PausedJobs, 1)
	atomic.AddInt32(&j.ExecutingJobs, -1)
	j.pausedJobsMinMax.Add(latency)
}

// RevertedPending when job is reverted back to pending
func (j *JobStats) RevertedPending() {
	atomic.AddInt32(&j.ExecutingJobs, -1)
}

// Calculate average/min/max
func (j *JobStats) Calculate() {
	j.SucceededJobsAverage = j.succeededJobsMinMax.Average()
	j.SucceededJobsMin = j.succeededJobsMinMax.Min
	j.SucceededJobsMax = j.succeededJobsMinMax.Max

	j.FailedJobsAverage = j.failedJobsMinMax.Average()
	j.FailedJobsMin = j.failedJobsMinMax.Min
	j.FailedJobsMax = j.failedJobsMinMax.Max
	if j.SucceededJobs+j.FailedJobs > 0 {
		j.SucceededJobsPercentages = j.SucceededJobs * 100 / (j.SucceededJobs + j.FailedJobs)
	} else {
		j.SucceededJobsPercentages = 0
	}
}

func (j *JobStats) String() string {
	return fmt.Sprintf("job=%s, succeeded=%s, failed=%s",
		j.JobKey.GetUserJobTypeKey(),
		j.SucceededJobsAverageString(),
		j.FailedJobsAverageString())
}

// FirstJobAtString formatted date
func (j *JobStats) FirstJobAtString() string {
	if j.FirstJobAt == nil {
		return ""
	}
	return j.FirstJobAt.Format("Jan _2, 15:04:05 MST")
}

// LastJobAtString formatted date
func (j *JobStats) LastJobAtString() string {
	if j.LastJobAt == nil {
		return ""
	}
	return j.LastJobAt.Format("Jan _2, 15:04:05 MST")
}

// SucceededJobsAverageString elapsed in string format
func (j *JobStats) SucceededJobsAverageString() string {
	if j.SucceededJobsAverage == 0 {
		return ""
	}
	return (time.Duration(j.SucceededJobsAverage) * time.Millisecond).String()
}

// FailedJobsAverageString elapsed in string format
func (j *JobStats) FailedJobsAverageString() string {
	if j.FailedJobsAverage == 0 {
		return ""
	}
	return (time.Duration(j.FailedJobsAverage) * time.Millisecond).String()
}
