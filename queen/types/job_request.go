package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"plexobject.com/formicary/internal/types"
)

// ReservedRequestProperties reserved properties
const ReservedRequestProperties = "platform,jobType,jobGroup,jobPriority,orgID,userID"

// ParentJobTypePrefix prefix for forked job id
const ParentJobTypePrefix = "ParentJobType"

// UserJobTypeKey defines key for job-type by user/org
type UserJobTypeKey interface {
	// GetJobType defines the type of job
	GetJobType() string
	// GetJobVersion defines the version of job
	GetJobVersion() string
	// GetOrganizationID defines the organization-id of the job creator
	GetOrganizationID() string
	// GetUserID defines the user-id of the job creator
	GetUserID() string
	// GetUserJobTypeKey defines a unique key for the user and job
	GetUserJobTypeKey() string
}

// IJobRequestSummary defines interface for job request summary
type IJobRequestSummary interface {
	// GetID defines UUID for primary key
	GetID() uint64
	// GetJobType defines the type of job
	GetJobType() string
	// GetJobVersion defines the version of job
	GetJobVersion() string
	// GetJobState defines state of job that is maintained throughout the lifecycle of a job
	GetJobState() types.RequestState
	// GetOrganizationID defines the organization-id of the job creator
	GetOrganizationID() string
	// GetUserID defines the user-id of the job creator
	GetUserID() string
	//GetUserJobTypeKey key of job-type
	GetUserJobTypeKey() string
	// GetScheduledAt defines schedule time
	GetScheduledAt() time.Time
	// GetJobPriority priority
	GetJobPriority() int
	// GetCreatedAt job creation time
	GetCreatedAt() time.Time
}

// IJobRequest defines interface for basic job request properties
type IJobRequest interface {
	// GetID defines UUID for primary key
	GetID() uint64
	GetJobDefinitionID() string
	GetJobExecutionID() string
	GetLastJobExecutionID() string
	SetJobExecutionID(jobExecutionID string)
	// GetScheduleAttempts - number of times request was attempted to schedule
	GetScheduleAttempts() int
	GetJobPriority() int
	// GetJobType defines the type of job
	GetJobType() string
	GetJobVersion() string
	// GetJobState defines state of job that is maintained throughout the lifecycle of a job
	GetJobState() types.RequestState
	// SetJobState sets job state
	SetJobState(state types.RequestState)
	GetGroup() string
	GetOrganizationID() string
	GetUserID() string
	GetRetried() int
	// IncrRetried increment Retried
	IncrRetried() int
	// GetCronTriggered is true if request was triggered by cron
	GetCronTriggered() bool
	// GetScheduledAt defines schedule time
	GetScheduledAt() time.Time
	// GetCreatedAt job creation time
	GetCreatedAt() time.Time
	//GetUserJobTypeKey key of job-type
	GetUserJobTypeKey() string
	GetParams() []*JobRequestParam
	SetParams(params []*JobRequestParam)
	Editable(userID string, organizationID string) bool
}

// JobRequest defines user request to process a job, which is saved in the database as PENDING and is then scheduled for job execution.
type JobRequest struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID uint64 `json:"id" gorm:"primary_key"`
	// ParentID defines id for parent job
	ParentID uint64 `json:"parent_id"`
	// UserKey defines user-defined UUID and can be used to detect duplicate jobs
	UserKey string `json:"user_key"`
	// JobDefinitionID points to the job-definition version
	JobDefinitionID string `json:"job_definition_id"`
	// JobExecutionID defines foreign key for JobExecution
	JobExecutionID string `json:"job_execution_id"`
	// LastJobExecutionID defines foreign key for JobExecution
	LastJobExecutionID string `json:"last_job_execution_id"`
	// OrganizationID defines org who submitted the job
	OrganizationID string `json:"organization_id"`
	// UserID defines user who submitted the job
	UserID string `json:"user_id"`
	// Permissions provides who can access this request 0 - all, 1 - Org must match, 2 - UserID must match from authentication
	Permissions int `json:"permissions"`
	// Description of the request
	Description string `json:"description"`
	// Platform overrides platform property for targeting job to a specific follower
	Platform string `json:"platform"`
	// JobType defines type for the job
	JobType    string `json:"job_type"`
	JobVersion string `json:"job_version"`
	// JobState defines state of job that is maintained throughout the lifecycle of a job
	JobState types.RequestState `json:"job_state"`
	// JobGroup defines a property for grouping related job
	JobGroup string `json:"job_group"`
	// JobPriority defines priority of the job
	JobPriority int `json:"job_priority"`
	// Timeout defines max time a job should take, otherwise the job is aborted
	Timeout time.Duration `yaml:"timeout,omitempty" json:"timeout"`
	// ScheduleAttempts defines attempts of schedule
	ScheduleAttempts int `json:"schedule_attempts" gorm:"schedule_attempts"`
	// Retried keeps track of retry attempts
	Retried int `json:"retried"`
	// CronTriggered is true if request was triggered by cron
	CronTriggered bool `json:"cron_triggered"`
	// QuickSearch provides quick search to search a request by params
	QuickSearch string `json:"quick_search"`
	// ErrorCode captures error code at the end of job execution if it fails
	ErrorCode string `json:"error_code"`
	// ErrorMessage captures error message at the end of job execution if it fails
	ErrorMessage string `json:"error_message"`
	// Params are passed with job request
	Params []*JobRequestParam `yaml:"-" json:"-" gorm:"ForeignKey:JobRequestID" gorm:"auto_preload" gorm:"constraint:OnUpdate:CASCADE"`
	// ScheduledAt defines schedule time when job will be submitted so that you can submit a job
	// that will be executed later
	ScheduledAt time.Time `json:"scheduled_at"`
	// CreatedAt job creation time
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt job update time
	UpdatedAt time.Time `json:"updated_at" gorm:"updated_at"`
	// Execution refers to job-Execution
	Execution       *JobExecution          `yaml:"-" json:"execution" gorm:"-"`
	NameValueParams map[string]interface{} `yaml:"params,omitempty" json:"params" gorm:"-"`
	ParamsJSON      string                 `yaml:"-" json:"-" gorm:"-"`
	Errors          map[string]string      `yaml:"-" json:"-" gorm:"-"`
	lookupParams    map[string]*JobRequestParam
}

// TableName overrides default table name
func (JobRequest) TableName() string {
	return "formicary_job_requests"
}

// NewRequest creates new request for processing a job
func NewRequest() *JobRequest {
	return &JobRequest{
		JobState:        types.PENDING,
		ScheduledAt:     time.Now(),
		lookupParams:    make(map[string]*JobRequestParam),
		Params:          make([]*JobRequestParam, 0),
		NameValueParams: make(map[string]interface{}),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

// NewJobRequestFromDefinition creates new request from job Definition
func NewJobRequestFromDefinition(job *JobDefinition) (*JobRequest, error) {
	if job.Paused {
		return nil, fmt.Errorf("job %s is paused", job.JobType)
	}
	request := &JobRequest{
		JobType:         job.JobType,
		JobVersion:      job.SemVersion,
		JobDefinitionID: job.ID,
		JobState:        types.PENDING,
		Platform:        job.Platform,
		UserID:          job.UserID,
		OrganizationID:  job.OrganizationID,
		ScheduledAt:     time.Now(),
		lookupParams:    make(map[string]*JobRequestParam),
		Params:          make([]*JobRequestParam, 0),
		NameValueParams: make(map[string]interface{}),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	request.UpdateUserKeyFromScheduleIfCronJob(job)
	return request, nil
}

// Editable checks if user can edit
func (jr *JobRequest) Editable(userID string, organizationID string) bool {
	if jr.OrganizationID != "" || organizationID != "" {
		return jr.OrganizationID == organizationID
	}
	return jr.UserID == userID
}

// UpdateUserKeyFromScheduleIfCronJob updates schedule-time and user-key
func (jr *JobRequest) UpdateUserKeyFromScheduleIfCronJob(job *JobDefinition) {
	if scheduledAt, userKey := job.GetCronScheduleTimeAndUserKey(); scheduledAt != nil {
		jr.ScheduledAt = *scheduledAt
		jr.UserKey = userKey
		jr.CronTriggered = true
	}
}

// ElapsedDuration time duration of job execution
func (jr *JobRequest) ElapsedDuration() string {
	if jr.NotTerminal() {
		if jr.ScheduledAt.Unix() > time.Now().Unix() {
			return ""
		}
		return time.Now().Sub(jr.ScheduledAt).String()
	}
	return jr.UpdatedAt.Sub(jr.ScheduledAt).String()
}

// ShortUserID short user id
func (jr *JobRequest) ShortUserID() string {
	if len(jr.UserID) > 8 {
		return jr.UserID[0:8] + "..."
	}
	return jr.UserID
}

// IsForkedJob checks if job is forked
func (jr *JobRequest) IsForkedJob() bool {
	nv := jr.GetParam(types.ForkedJob)
	return nv != nil && nv.Value == "true"
}

// ShortJobType short job-type
func (jr *JobRequest) ShortJobType() string {
	if len(jr.JobType) > 10 {
		return jr.JobType[:10] + "..."
	}
	return jr.JobType
}

// CanRestart if job can be restarted
func (jr *JobRequest) CanRestart() bool {
	return jr.JobState.CanRestart()
}

// CanCancel if job can be cancelled
func (jr *JobRequest) CanCancel() bool {
	return jr.JobState.CanCancel()
}

// CanTriggerCron if job is triggered that can be scheduled
func (jr *JobRequest) CanTriggerCron() bool {
	return jr.JobState.Pending() && jr.CronTriggered
}

// Pending job
func (jr *JobRequest) Pending() bool {
	return jr.JobState.Pending()
}

// Completed job
func (jr *JobRequest) Completed() bool {
	return jr.JobState.Completed()
}

// Failed job
func (jr *JobRequest) Failed() bool {
	return jr.JobState.Failed()
}

// NotTerminal - job that is not in final completed/failed state
func (jr *JobRequest) NotTerminal() bool {
	return !jr.IsTerminal()
}

// IsTerminal - job that is in final completed/failed state
func (jr *JobRequest) IsTerminal() bool {
	return jr.JobState.IsTerminal()
}

// ToInfo converts into request info
func (jr *JobRequest) ToInfo() *JobRequestInfo {
	return NewJobRequestInfo(jr)
}

// GetID defines UUID for primary key
func (jr *JobRequest) GetID() uint64 {
	return jr.ID
}

// GetJobDefinitionID returns job-definition-id
func (jr *JobRequest) GetJobDefinitionID() string {
	return jr.JobDefinitionID
}

// GetJobExecutionID job-execution-id
func (jr *JobRequest) GetJobExecutionID() string {
	return jr.JobExecutionID
}

// GetLastJobExecutionID last-job-execution-id
func (jr *JobRequest) GetLastJobExecutionID() string {
	return jr.LastJobExecutionID
}

// GetJobPriority priority
func (jr *JobRequest) GetJobPriority() int {
	return jr.JobPriority
}

// SetJobExecutionID sets execution id
func (jr *JobRequest) SetJobExecutionID(jobExecutionID string) {
	jr.JobExecutionID = jobExecutionID
}

// GetCreatedAt job creation time
func (jr *JobRequest) GetCreatedAt() time.Time {
	return jr.CreatedAt
}

// GetScheduledAt defines schedule time
func (jr *JobRequest) GetScheduledAt() time.Time {
	return jr.ScheduledAt
}

// GetScheduleAttempts - number of times request was attempted to schedule
func (jr *JobRequest) GetScheduleAttempts() int {
	return jr.ScheduleAttempts
}

// GetJobType defines the type of job
func (jr *JobRequest) GetJobType() string {
	return jr.JobType
}

// GetJobVersion defines the version of job
func (jr *JobRequest) GetJobVersion() string {
	return jr.JobVersion
}

// GetRetried - retry attempts
func (jr *JobRequest) GetRetried() int {
	return jr.Retried
}

// IncrRetried - increment retry attempts
func (jr *JobRequest) IncrRetried() int {
	jr.Retried++
	return jr.Retried
}

// GetCronTriggered is true if request was triggered by cron
func (jr *JobRequest) GetCronTriggered() bool {
	return jr.CronTriggered
}

// GetJobState defines state of job that is maintained throughout the lifecycle of a job
func (jr *JobRequest) GetJobState() types.RequestState {
	return jr.JobState
}

// SetJobState sets job state
func (jr *JobRequest) SetJobState(state types.RequestState) {
	jr.JobState = state
}

// GetGroup returns group
func (jr *JobRequest) GetGroup() string {
	return jr.JobGroup
}

// GetOrganizationID returns org
func (jr *JobRequest) GetOrganizationID() string {
	return jr.OrganizationID
}

// GetUserID returns user-id
func (jr *JobRequest) GetUserID() string {
	return jr.UserID
}

// JobTypeAndVersion with version
func (jr *JobRequest) JobTypeAndVersion() string {
	if jr.JobVersion == "" {
		return jr.JobType
	}
	return jr.JobType + ":" + jr.JobVersion
}

// GetUserJobTypeKey key of job-type
func (jr *JobRequest) GetUserJobTypeKey() string {
	return getUserJobTypeKey(jr.OrganizationID, jr.UserID, jr.JobType, jr.JobVersion)
}

// Running returns true if job is running
func (jr *JobRequest) Running() bool {
	return jr.JobState == types.STARTED || jr.JobState == types.EXECUTING
}

// Waiting returns true if job is waiting to running
func (jr *JobRequest) Waiting() bool {
	return jr.JobState == types.PENDING || jr.JobState == types.READY
}

// Done returns true if job is done running
func (jr *JobRequest) Done() bool {
	return jr.JobState == types.COMPLETED || jr.JobState == types.FAILED || jr.JobState == types.CANCELLED
}

// UpdatedAtString formatted date
func (jr *JobRequest) UpdatedAtString() string {
	//return jr.UpdatedAt.Format("Jan _2, 15:04:05 MST")
	return jr.UpdatedAt.Format("Jan _2, 15:04:05")
}

// ScheduledAtString formatted date
func (jr *JobRequest) ScheduledAtString() string {
	return jr.ScheduledAt.Format("Jan _2, 15:04:05")
}

// ClearParams clear params
func (jr *JobRequest) ClearParams() {
	jr.lookupParams = make(map[string]*JobRequestParam)
	jr.NameValueParams = make(map[string]interface{})
	jr.Params = make([]*JobRequestParam, 0)
}

// GetParams returns params
func (jr *JobRequest) GetParams() []*JobRequestParam {
	return jr.Params
}

// SetParams set params
func (jr *JobRequest) SetParams(params []*JobRequestParam) {
	jr.Params = params
}

// AddParam adds parameter for job
func (jr *JobRequest) AddParam(
	name string,
	value interface{}) (*JobRequestParam, error) {
	param, err := NewJobRequestParam(name, value, false)
	if err != nil {
		return nil, err
	}
	param.JobRequestID = jr.ID
	if jr.lookupParams[name] == nil {
		jr.Params = append(jr.Params, param)
	} else {
		for _, next := range jr.Params {
			if next.Name == name {
				next.Value = param.Value
			}
		}
	}
	jr.lookupParams[name] = param
	jr.NameValueParams[name] = value
	return param, nil
}

// String defines description of request from properties
func (jr *JobRequest) String() string {
	return fmt.Sprintf("ID=%d JobDefinitionID=%s JobType=%s JobState=%s Param=%s",
		jr.ID, jr.JobDefinitionID, jr.JobType, jr.JobState, jr.ParamString())
}

// GetParamsJSON - params in JSON
func (jr *JobRequest) GetParamsJSON() string {
	b, err := json.Marshal(jr.NameValueParams)
	if err != nil {
		jr.ParamsJSON = "{}"
	} else {
		jr.ParamsJSON = string(b)
	}
	if jr.ParamsJSON == "null" {
		jr.ParamsJSON = "{}"
	}
	return jr.ParamsJSON
}

// SetParamsJSON - params in JSON
func (jr *JobRequest) SetParamsJSON(j string) error {
	jr.ParamsJSON = j
	return json.Unmarshal([]byte(j), &jr.NameValueParams)
}

// ParamString - text view of params
func (jr *JobRequest) ParamString() string {
	var b strings.Builder
	for i, c := range jr.Params {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(c.Name + "=" + c.Value)
	}
	return b.String()
}

// GetParam gets request parameter
func (jr *JobRequest) GetParam(name string) *JobRequestParam {
	return jr.lookupParams[name]
}

// Equals compares other job-request for equality
func (jr *JobRequest) Equals(other *JobRequest) error {
	if other == nil {
		return errors.New("other request is nil")
	}
	if err := jr.ValidateBeforeSave(); err != nil {
		return err
	}
	if err := other.ValidateBeforeSave(); err != nil {
		return err
	}

	if jr.ID != other.ID {
		return fmt.Errorf("expected id %v but was %v", jr.ID, other.ID)
	}
	if jr.JobType != other.JobType {
		return fmt.Errorf("expected jobType %v but was %v", jr.JobType, other.JobType)
	}
	if jr.JobState != other.JobState {
		return fmt.Errorf("expected jobState %v but was %v", jr.JobState, other.JobState)
	}
	if jr.JobPriority != other.JobPriority {
		return fmt.Errorf("expected jobPriority %v but was %v", jr.JobPriority, other.JobPriority)
	}
	if jr.JobGroup != other.JobGroup {
		return fmt.Errorf("expected jobGroup %v but was %v", jr.JobGroup, other.JobGroup)
	}
	if jr.UserKey != other.UserKey {
		return fmt.Errorf("expected userKey %v but was %v", jr.UserKey, other.UserKey)
	}
	if jr.JobDefinitionID != other.JobDefinitionID {
		return fmt.Errorf("expected jobDefinitionID %v but was %v", jr.JobDefinitionID, other.JobDefinitionID)
	}
	if jr.JobExecutionID != other.JobExecutionID {
		return fmt.Errorf("expected jobExecutionID %v but was %v", jr.JobExecutionID, other.JobExecutionID)
	}
	if jr.OrganizationID != other.OrganizationID {
		return fmt.Errorf("expected org %v but was %v", jr.OrganizationID, other.OrganizationID)
	}
	if jr.UserID != other.UserID {
		return fmt.Errorf("expected user-id %v but was %v", jr.UserID, other.UserID)
	}
	if jr.Description != other.Description {
		return fmt.Errorf("expected description %v but was %v", jr.Description, other.Description)
	}
	if jr.Platform != other.Platform {
		return fmt.Errorf("expected platform %v but was %v", jr.Platform, other.Platform)
	}
	if jr.Timeout != other.Timeout {
		return fmt.Errorf("expected timeout %v but was %v", jr.Timeout, other.Timeout)
	}
	if jr.Retried != other.Retried {
		return fmt.Errorf("expected retried %v but was %v", jr.Retried, other.Retried)
	}
	if jr.ErrorCode != other.ErrorCode {
		return fmt.Errorf("expected error code %v but was %v", jr.ErrorCode, other.ErrorCode)
	}
	if jr.ErrorMessage != other.ErrorMessage {
		return fmt.Errorf("expected error message %v but was %v", jr.ErrorMessage, other.ErrorMessage)
	}
	if jr.Params == nil {
		return fmt.Errorf("params not defined")
	}
	if jr.Params == nil || other.Params == nil {
		return fmt.Errorf("other params not defined")
	}
	if len(jr.Params) != len(other.Params) {
		return fmt.Errorf("expected %v params but was %v", len(jr.Params), len(other.Params))
	}
	for _, p := range jr.Params {
		if other.GetParam(p.Name) == nil || other.GetParam(p.Name).Value != p.Value {
			return fmt.Errorf("expected %v param but was %v", p.Value, other.GetParam(p.Name))
		}
	}
	return nil
}

// AfterLoad initializes job-request
func (jr *JobRequest) AfterLoad() error {
	jr.lookupParams = make(map[string]*JobRequestParam)
	jr.NameValueParams = make(map[string]interface{})
	for _, p := range jr.Params {
		_, err := p.GetParsedValue()
		if err != nil {
			return err
		}
		jr.lookupParams[p.Name] = p
		jr.NameValueParams[p.Name], _ = p.GetParsedValue()
	}

	return nil
}

// Validate validates job-request
func (jr *JobRequest) Validate() (err error) {
	jr.Errors = make(map[string]string)
	if err = jr.AfterLoad(); err != nil {
		jr.Errors["Error"] = err.Error()
		return err
	}
	if jr.JobType == "" {
		err = errors.New("jobType is not specified")
		jr.Errors["JobType"] = err.Error()
	}
	if jr.JobDefinitionID == "" {
		err = errors.New("jobDefinitionID is not specified")
		jr.Errors["JobDefinitionID"] = err.Error()
	}
	if jr.JobState == "" {
		err = errors.New("jobState is not specified")
		jr.Errors["JobState"] = err.Error()
	}
	if jr.ScheduledAt.IsZero() {
		err = errors.New("scheduledAt is not specified")
		jr.Errors["ScheduledAt"] = err.Error()
	}
	return
}

// UpdateScheduledAtFromCronTrigger sets scheduled time based on cron expression
func (jr *JobRequest) UpdateScheduledAtFromCronTrigger(jd *JobDefinition) bool {
	date, userKey := jd.GetCronScheduleTimeAndUserKey()
	if date != nil {
		jr.ScheduledAt = *date
		jr.UserKey = userKey
		return true
	}
	return false
}

// ValidateBeforeSave validates job-request
func (jr *JobRequest) ValidateBeforeSave() error {
	for k, v := range jr.NameValueParams {
		if _, err := jr.AddParam(k, v); err != nil {
			return err
		}
	}
	_ = jr.updateQuickSearch()
	return jr.Validate()
}

// updateQuickSearch
func (jr *JobRequest) updateQuickSearch() error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s %s %s %s %s;",
		jr.Description, jr.Platform, jr.JobType, jr.ErrorCode, jr.ErrorMessage))
	for k, v := range jr.NameValueParams {
		if sv, err := jr.AddParam(k, v); err == nil {
			sb.WriteString(k + "=" + sv.Value)
		} else {
			return err
		}
	}
	jr.QuickSearch = sb.String()
	if len(jr.QuickSearch) > 1000 {
		jr.QuickSearch = jr.QuickSearch[0:1000]
	}
	return jr.Validate()
}

// JobRequestParam defines configuration for job definition
type JobRequestParam struct {
	//gorm.Model
	// Inheriting name, value, type
	types.NameTypeValue
	// ID defines UUID for primary key
	ID string `json:"id" gorm:"primary_key"`
	// JobRequestID defines foreign key for job request
	JobRequestID uint64 `json:"job_request_id"`
	// CreatedAt job creation time
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt job update time
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName overrides default table name
func (JobRequestParam) TableName() string {
	return "formicary_job_request_params"
}

// NewJobRequestParam creates new request param
func NewJobRequestParam(name string, value interface{}, secret bool) (*JobRequestParam, error) {
	nv, err := types.NewNameTypeValue(name, value, secret)
	if err != nil {
		return nil, err
	}
	return &JobRequestParam{
		NameTypeValue: nv,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}, nil
}
