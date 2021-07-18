package types

import (
	"plexobject.com/formicary/internal/types"
	"time"
)

// JobCounts defines counts on job types by state and error code.
type JobCounts struct {
	// JobType defines type for the job
	JobType string `json:"job_type"`
	// OrganizationID defines org who submitted the job
	OrganizationID string `json:"organization_id"`
	// UserID defines user who submitted the job
	UserID string `json:"user_id"`
	// JobState defines state of the job
	JobState types.RequestState `json:"job_state"`
	// ErrorCode defines error code if job failed
	ErrorCode string `json:"error_code"`
	// Counts defines total number of records matching stats
	Counts int64 `json:"counts"`
	// StartTime stores first occurrence of the stats
	StartTime time.Time `json:"start_time"`
	// EndTime stores last occurrence of the stats
	EndTime time.Time `json:"end_time"`
	// StartTime stores first occurrence of the stats for sqlite
	StartTimeString string `json:"start_time_stirng"`
	// EndTime stores last occurrence of the stats for sqlite
	EndTimeString string `json:"end_time_string"`
	// Date for unix epoch
	Day string `json:"-"`
}

// GetStartTime time because sqlite returns time as string but other dbs use time.Time
func (c *JobCounts) GetStartTime() time.Time {
	if c.StartTimeString != "" {
		if d, err := time.Parse("2006-01-02 15:04:05.999999-07:00", c.StartTimeString); err == nil {
			return d
		}
	}
	return c.StartTime
}

// GetEndTime time because sqlite returns time as string but other dbs use time.Time
func (c *JobCounts) GetEndTime() time.Time {
	if c.EndTimeString != "" {
		if d, err := time.Parse("2006-01-02 15:04:05.999999-07:00", c.EndTimeString); err == nil {
			return d
		}
	}
	return c.EndTime
}

// GetStartTimeString formatted date
func (c *JobCounts) GetStartTimeString() string {
	return c.GetStartTime().Format("Jan _2, 15:04:05 MST")
}

// GetEndTimeString formatted date
func (c *JobCounts) GetEndTimeString() string {
	return c.GetEndTime().Format("Jan _2, 15:04:05 MST")
}

// Completed job
func (c *JobCounts) Completed() bool {
	return c.JobState.Completed()
}

// Failed job
func (c *JobCounts) Failed() bool {
	return c.JobState.Failed()
}

// NotTerminal - job that is not in final completed/failed state
func (c *JobCounts) NotTerminal() bool {
	return !c.JobState.IsTerminal()
}

// GetUserJobTypeKey defines key
func (c *JobCounts) GetUserJobTypeKey() string {
	return getUserJobTypeKey(c.OrganizationID, c.UserID, c.JobType)
}

// GetJobType defines the type of job
func (c *JobCounts) GetJobType() string {
	return c.JobType
}

// GetOrganizationID returns org
func (c *JobCounts) GetOrganizationID() string {
	return c.OrganizationID
}

// GetUserID returns user-id
func (c *JobCounts) GetUserID() string {
	return c.UserID
}