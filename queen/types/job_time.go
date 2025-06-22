package types

import (
	"time"

	"plexobject.com/formicary/internal/types"
)

// JobTime defines job times
type JobTime struct {
	// ID defines UUID for primary key
	ID string `json:"id"`
	// OrganizationID defines org who submitted the job
	OrganizationID string `json:"organization_id"`
	// UserID defines user who submitted the job
	UserID string `json:"user_id"`
	// JobExecutionID
	JobExecutionID string `json:"job_execution_id"`
	// JobType defines type for the job
	JobType    string `json:"job_type"`
	JobVersion string `json:"job_version"`
	// JobState defines state of job that is maintained throughout the lifecycle of a job
	JobState types.RequestState `json:"job_state"`
	// StartedAt job execution start time
	StartedAt *time.Time `json:"started_at"`
	// EndedAt job execution end time
	EndedAt *time.Time `json:"ended_at"`
	// JobPriority defines priority of the job
	JobPriority int `json:"job_priority"`
	// ScheduledAt defines schedule time
	ScheduledAt time.Time `json:"scheduled_at"`
	// CreatedAt job creation time
	CreatedAt time.Time `json:"created_at"`
}

// ToInfo creates new request info for processing a job
func (je *JobTime) ToInfo() *JobRequestInfo {
	return &JobRequestInfo{
		ID:             je.ID,
		OrganizationID: je.OrganizationID,
		UserID:         je.UserID,
		JobType:        je.JobType,
		JobVersion:     je.JobVersion,
		JobPriority:    je.JobPriority,
		JobState:       je.JobState,
		ScheduledAt:    je.ScheduledAt,
		CreatedAt:      je.CreatedAt,
	}
}

// ElapsedDuration time duration of job execution
func (je *JobTime) ElapsedDuration() string {
	if je.EndedAt == nil || je.StartedAt == nil {
		return ""
	}
	return je.EndedAt.Sub(*je.StartedAt).String()
}

// Elapsed unix time elapsed of job execution
func (je *JobTime) Elapsed() int64 {
	if je.EndedAt == nil || je.StartedAt == nil {
		return 0
	}
	return je.EndedAt.Sub(*je.StartedAt).Milliseconds()
}

// Pending job
func (je *JobTime) Pending() bool {
	return je.JobState.Pending()
}

// Completed status
func (je *JobTime) Completed() bool {
	return je.JobState.Completed()
}

// Failed status
func (je *JobTime) Failed() bool {
	return je.JobState.Failed()
}

// Cancelled status
func (je *JobTime) Cancelled() bool {
	return je.JobState.Cancelled()
}

// GetUserJobTypeKey defines key
func (je *JobTime) GetUserJobTypeKey() string {
	return getUserJobTypeKey(je.OrganizationID, je.UserID, je.JobType, je.JobVersion)
}

// GetJobType defines the type of job
func (je *JobTime) GetJobType() string {
	return je.JobType
}

// GetJobVersion defines the version of job
func (je *JobTime) GetJobVersion() string {
	return je.JobVersion
}

// GetOrganizationID returns org
func (je *JobTime) GetOrganizationID() string {
	return je.OrganizationID
}

// GetUserID returns user-id
func (je *JobTime) GetUserID() string {
	return je.UserID
}

// IMPLEMENTING JobRequestInfoSummary

// GetID request id
func (je *JobTime) GetID() string {
	return je.ID
}

// GetJobPriority -- N/A
func (je *JobTime) GetJobPriority() int {
	return -1
}

// GetJobState - job state
func (je *JobTime) GetJobState() types.RequestState {
	return je.JobState
}

// GetScheduledAt - scheduled
func (je *JobTime) GetScheduledAt() time.Time {
	return je.ScheduledAt
}

// GetCreatedAt - created
func (je *JobTime) GetCreatedAt() time.Time {
	return je.CreatedAt
}
