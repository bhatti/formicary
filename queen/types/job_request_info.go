package types

import (
	"errors"
	"time"

	"plexobject.com/formicary/internal/types"
)

// JobRequestInfo defines basic id/state of the job request.
type JobRequestInfo struct {
	// ID defines UUID for primary key
	ID uint64 `json:"id"`
	// JobDefinitionID points to the job-definition version
	JobDefinitionID string `json:"job_definition_id"`
	// JobExecutionID
	JobExecutionID string `json:"job_execution_id"`
	// LastJobExecutionID defines foreign key for JobExecution
	LastJobExecutionID string `json:"last_job_execution_id"`
	// JobType defines type for the job
	JobType    string `json:"job_type"`
	JobVersion string `json:"job_version"`
	// JobPriority defines priority of the job
	JobPriority int `json:"job_priority"`
	// JobState defines state of job that is maintained throughout the lifecycle of a job
	JobState types.RequestState `json:"job_state"`
	// ScheduleAttempts defines attempts of schedule
	ScheduleAttempts int `json:"schedule_attempts"`
	// OrganizationID defines org who submitted the job
	OrganizationID string `json:"organization_id"`
	// UserID defines user who submitted the job
	UserID string `json:"user_id"`
	// CronTriggered is true if request was triggered by cron
	CronTriggered bool `json:"cron_triggered"`
	// Params are passed with job request
	Params []*JobRequestParam `yaml:"-" json:"-" gorm:"-"`
	// ScheduledAt defines schedule time
	ScheduledAt time.Time `json:"scheduled_at"`
	// CreatedAt job creation time
	CreatedAt time.Time `json:"created_at"`
}

// Validate validates job-request-info
func (jri *JobRequestInfo) Validate() error {
	if jri.JobType == "" {
		return errors.New("jobType is not specified")
	}
	if jri.JobState == "" {
		return errors.New("jobState is not specified")
	}
	return nil
}

// GetUserJobTypeKey defines key
func (jri *JobRequestInfo) GetUserJobTypeKey() string {
	return getUserJobTypeKey(jri.OrganizationID, jri.UserID, jri.JobType, jri.JobVersion)
}

// getUserJobTypeKey defines key
func getUserJobTypeKey(organizationID string, userID string, jobType string, jobVersion string) string {
	if organizationID == "" {
		return userID + "-" + jobType + ":" + jobVersion
	}
	return organizationID + "-" + jobType + ":" + jobVersion
}

// GetID defines UUID for primary key
func (jri *JobRequestInfo) GetID() uint64 {
	return jri.ID
}

// GetLastJobExecutionID returns last-execution-id
func (jri *JobRequestInfo) GetLastJobExecutionID() string {
	return jri.LastJobExecutionID
}

// GetJobExecutionID returns execution-id
func (jri *JobRequestInfo) GetJobExecutionID() string {
	return jri.JobExecutionID
}

// GetJobDefinitionID returns job-definition-id
func (jri *JobRequestInfo) GetJobDefinitionID() string {
	return jri.JobDefinitionID
}

// SetJobExecutionID sets execution-id
func (jri *JobRequestInfo) SetJobExecutionID(jobExecutionID string) {
	jri.JobExecutionID = jobExecutionID
}

// GetScheduleAttempts - number of times request was attempted to schedule
func (jri *JobRequestInfo) GetScheduleAttempts() int {
	return jri.ScheduleAttempts
}

// GetJobType defines the type of job
func (jri *JobRequestInfo) GetJobType() string {
	return jri.JobType
}

// GetJobVersion defines the version of job
func (jri *JobRequestInfo) GetJobVersion() string {
	return jri.JobVersion
}

// GetJobPriority returns priority
func (jri *JobRequestInfo) GetJobPriority() int {
	return jri.JobPriority
}

// GetJobState defines state of job that is maintained throughout the lifecycle of a job
func (jri *JobRequestInfo) GetJobState() types.RequestState {
	return jri.JobState
}

// SetJobState sets job state
func (jri *JobRequestInfo) SetJobState(state types.RequestState) {
	jri.JobState = state
}

// GetGroup returns group
func (jri *JobRequestInfo) GetGroup() string {
	return ""
}

// GetOrganizationID returns org
func (jri *JobRequestInfo) GetOrganizationID() string {
	return jri.OrganizationID
}

// GetUserID returns user-id
func (jri *JobRequestInfo) GetUserID() string {
	return jri.UserID
}

// GetRetried - retry attempts
func (jri *JobRequestInfo) GetRetried() int {
	return 0
}

// IncrRetried - increment retry attempts
func (jri *JobRequestInfo) IncrRetried() int {
	return 0
}

// GetCronTriggered is true if request was triggered by cron
func (jri *JobRequestInfo) GetCronTriggered() bool {
	return jri.CronTriggered
}

// GetCreatedAt job creation time
func (jri *JobRequestInfo) GetCreatedAt() time.Time {
	return jri.CreatedAt
}

// GetScheduledAt defines schedule time
func (jri *JobRequestInfo) GetScheduledAt() time.Time {
	return jri.ScheduledAt
}

// GetParams returns params
func (jri *JobRequestInfo) GetParams() []*JobRequestParam {
	return jri.Params
}

// SetParams set params
func (jri *JobRequestInfo) SetParams(params []*JobRequestParam) {
	jri.Params = params
}

// NewJobRequestInfo creates new request info for processing a job
func NewJobRequestInfo(req IJobRequest) *JobRequestInfo {
	return &JobRequestInfo{
		ID:                 req.GetID(),
		JobDefinitionID:    req.GetJobDefinitionID(),
		JobExecutionID:     req.GetJobExecutionID(),
		LastJobExecutionID: req.GetLastJobExecutionID(),
		JobType:            req.GetJobType(),
		JobVersion:         req.GetJobVersion(),
		JobPriority:        req.GetJobPriority(),
		JobState:           req.GetJobState(),
		ScheduleAttempts:   req.GetScheduleAttempts(),
		UserID:             req.GetUserID(),
		OrganizationID:     req.GetOrganizationID(),
		CronTriggered:      req.GetCronTriggered(),
		Params:             req.GetParams(),
		ScheduledAt:        req.GetScheduledAt(),
		CreatedAt:          req.GetCreatedAt(),
	}
}
