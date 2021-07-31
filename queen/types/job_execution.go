package types

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"plexobject.com/formicary/internal/math"

	"plexobject.com/formicary/internal/utils"

	"plexobject.com/formicary/internal/types"
)

// JobExecution represents instance of job-definition and captures state of job-execution when a job-request is processed.
type JobExecution struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID string `json:"id" gorm:"primary_key"`
	// JobRequestID defines foreign key for job request
	JobRequestID uint64 `json:"job_request_id"`
	// JobType defines type for the job
	JobType    string `json:"job_type"`
	JobVersion string `json:"job_version"`
	// JobState defines state of job that is maintained throughout the lifecycle of a job
	JobState types.RequestState `json:"job_state"`
	// OrganizationID defines org who submitted the job
	OrganizationID string `json:"organization_id"`
	// UserID defines user who submitted the job
	UserID string `json:"user_id"`
	// ExitCode defines exit status from the job execution
	ExitCode string `json:"exit_code"`
	// ExitMessage defines exit message from the job execution
	ExitMessage string `json:"exit_message"`
	// ErrorCode captures error code at the end of job execution if it fails
	ErrorCode string `json:"error_code"`
	// ErrorMessage captures error message at the end of job execution if it fails
	ErrorMessage string `json:"error_message"`
	// Contexts defines context variables of job
	Contexts []*JobExecutionContext `json:"contexts" gorm:"ForeignKey:JobExecutionID" gorm:"auto_preload"`
	// Tasks defines list of tasks that are executed for the job
	Tasks []*TaskExecution `json:"tasks" gorm:"ForeignKey:JobExecutionID" gorm:"auto_preload"`
	// StartedAt job execution start time
	StartedAt time.Time `json:"started_at"`
	// EndedAt job execution end time
	EndedAt *time.Time `json:"ended_at"`
	// UpdatedAt job execution last update time
	UpdatedAt time.Time `json:"updated_at"`
	// CPUSecs execution time
	CPUSecs int64 `json:"cpu_secs"`
	// Active is used to soft delete job definition
	Active bool `yaml:"-" json:"-"`
	// Following are transient properties -- these are populated when AfterLoad or Validate is called
	lookupTasks    *utils.SafeMap
	lookupContexts *utils.SafeMap
}

// TableName overrides default table name
func (JobExecution) TableName() string {
	return "formicary_job_executions"
}

// NewJobExecution creates new instance of job-execution
func NewJobExecution(req IJobRequest) *JobExecution {
	var jobExec JobExecution
	jobExec.JobRequestID = req.GetID()
	jobExec.JobType = req.GetJobType()
	jobExec.JobVersion = req.GetJobVersion()
	jobExec.UserID = req.GetUserID()
	jobExec.OrganizationID = req.GetOrganizationID()
	jobExec.JobState = types.READY
	jobExec.Tasks = make([]*TaskExecution, 0)
	jobExec.StartedAt = time.Now()
	jobExec.UpdatedAt = time.Now()
	jobExec.lookupTasks = utils.NewSafeMap()
	jobExec.lookupContexts = utils.NewSafeMap()
	return &jobExec
}

// String provides short summary of job
func (je *JobExecution) String() string {
	return fmt.Sprintf("JobType=%s JobState=%s Context=%s;",
		je.JobType, je.JobState, je.ContextString())
}

// JobTypeAndVersion with version
func (je *JobExecution) JobTypeAndVersion() string {
	if je.JobVersion == "" {
		return je.JobType
	}
	return je.JobType + ":" + je.JobVersion
}

// ElapsedDuration time duration of job execution
func (je *JobExecution) ElapsedDuration() string {
	if je.EndedAt == nil || je.JobState == types.EXECUTING {
		return time.Now().Sub(je.StartedAt).String()
	}
	return je.EndedAt.Sub(je.StartedAt).String()
}

// ElapsedMillis unix time elapsed of job execution
func (je *JobExecution) ElapsedMillis() int64 {
	if je.EndedAt == nil || je.JobState == types.EXECUTING {
		return time.Now().Sub(je.StartedAt).Milliseconds()
	}
	return je.EndedAt.Sub(je.StartedAt).Milliseconds()
}

// CostFactor - factor multiplier
func (je *JobExecution) CostFactor() float64 {
	var total float64
	for _, t := range je.Tasks {
		total += t.AppliedCost
	}
	return total / float64(len(je.Tasks))
}

// ExecutionCostSecs unix time elapsed of job execution
func (je *JobExecution) ExecutionCostSecs() int64 {
	ended := je.EndedAt
	if ended == nil || je.JobState == types.EXECUTING {
		now := time.Now()
		ended = &now
	}
	var total int64
	for _, t := range je.Tasks {
		total += t.ExecutionCostSecs()
	}
	return math.Max64(
		int64(ended.Sub(je.StartedAt).Seconds()),
		total)
}

// CanRestart checks if job can be restarted
func (je *JobExecution) CanRestart() bool {
	return je.JobState.CanRestart()
}

// CanCancel checks if job can be cancelled
func (je *JobExecution) CanCancel() bool {
	return je.JobState.CanCancel()
}

// Completed job
func (je *JobExecution) Completed() bool {
	return je.JobState.Completed()
}

// GetFailedTaskError returns error for any non-optional task failed
func (je *JobExecution) GetFailedTaskError() (string, error) {
	for _, t := range je.Tasks {
		if !t.AllowFailure && t.TaskState.Failed() {
			return t.ErrorCode, fmt.Errorf(t.ErrorMessage)
		}
	}
	return "", nil
}

// Failed job
func (je *JobExecution) Failed() bool {
	return je.JobState.Failed()
}

// NotTerminal job that is either pending, ready or executing but is not in final state such as completed/failed
func (je *JobExecution) NotTerminal() bool {
	return !je.JobState.IsTerminal()
}

// Methods of tasks
func (je *JobExecution) Methods() string {
	taskMethods := make(map[types.TaskMethod]bool)
	for _, t := range je.Tasks {
		if t.Method != "" {
			taskMethods[t.Method] = true
		} else {
			taskMethods[types.Kubernetes] = true
		}
	}
	var buf strings.Builder
	for k := range taskMethods {
		if buf.Len() > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(fmt.Sprintf("%v", k))
	}
	return buf.String()
}

// TasksString textual representation
func (je *JobExecution) TasksString() string {
	var b strings.Builder
	for _, t := range je.Tasks {
		b.WriteString(t.String())
	}
	b.WriteString(";")
	return b.String()
}

// ContextString textual representation
func (je *JobExecution) ContextString() string {
	var b strings.Builder
	for _, c := range je.Contexts {
		b.WriteString(c.Name + "=" + c.Value + " ")
	}
	b.WriteString(";")
	return b.String()
}

// AddTask adds task
func (je *JobExecution) AddTask(task *TaskDefinition) *TaskExecution {
	taskExec := NewTaskExecution(task)
	if old := je.GetTask(task.TaskType); old == nil {
		je.Tasks = append(je.Tasks, taskExec)
		taskExec.TaskOrder = len(je.Tasks)
	} else {
		taskExec.TaskOrder = old.TaskOrder
	}
	taskExec.JobExecutionID = je.ID
	je.lookupTasks.SetObject(task.TaskType, taskExec)
	return taskExec
}

// GetTask finds task
func (je *JobExecution) GetTask(taskType string) *TaskExecution {
	old := je.lookupTasks.GetObject(taskType)
	if old == nil {
		return nil
	}
	return old.(*TaskExecution)
}

// GetLastTask finds last task that ran
func (je *JobExecution) GetLastTask() *TaskExecution {
	if len(je.Tasks) > 0 {
		return je.Tasks[len(je.Tasks)-1]
	}
	return nil
}

// AddTasks adds tasks
func (je *JobExecution) AddTasks(tasks ...*TaskDefinition) *JobExecution {
	for _, t := range tasks {
		je.AddTask(t)
	}
	return je
}

// AddContext adds job context
func (je *JobExecution) AddContext(
	name string,
	value interface{}) (*JobExecutionContext, error) {
	ctx, err := NewJobExecutionContext(name, value, false)
	if err != nil {
		return nil, err
	}
	ctx.JobExecutionID = je.ID
	if je.lookupContexts.GetObject(name) == nil {
		je.Contexts = append(je.Contexts, ctx)
	} else {
		for _, next := range je.Contexts {
			if next.Name == name {
				next.Value = ctx.Value
			}
		}
	}
	je.lookupContexts.SetObject(name, ctx)
	return ctx, nil
}

// DeleteContext removes context by name
func (je *JobExecution) DeleteContext(name string) *JobExecutionContext {
	if je.lookupContexts.GetObject(name) == nil {
		return nil
	}
	je.lookupContexts.DeleteObject(name)
	for i, c := range je.Contexts {
		if c.Name == name {
			je.Contexts = append(je.Contexts[:i], je.Contexts[i+1:]...)
			return c
		}
	}
	return nil
}

// GetContext gets job context
func (je *JobExecution) GetContext(name string) *JobExecutionContext {
	v := je.lookupContexts.GetObject(name)
	if v == nil {
		return nil
	}
	return v.(*JobExecutionContext)
}

// Equals compares other job-execution for equality
func (je *JobExecution) Equals(other *JobExecution) error {
	if other == nil {
		return errors.New("found nil other job")
	}
	if err := je.ValidateBeforeSave(); err != nil {
		return err
	}
	if err := other.ValidateBeforeSave(); err != nil {
		return err
	}

	if je.JobType != other.JobType {
		return fmt.Errorf("expected jobType %v but was %v", je.JobType, other.JobType)
	}
	if len(je.Contexts) != len(other.Contexts) {
		return fmt.Errorf("expected number of contexts %v but was %v", len(je.Contexts), len(other.Contexts))
	}
	for _, c := range other.Contexts {
		otherC := je.lookupContexts.GetObject(c.Name)
		if otherC == nil {
			return fmt.Errorf("could ot find contexts for %v", c.Name)
		} else if otherC.(*JobExecutionContext).Value != c.Value {
			return fmt.Errorf("expected contexts for %v as %v but was %v", c.Name, otherC, c.Value)
		}
	}
	if len(je.Tasks) != len(other.Tasks) {
		return fmt.Errorf("expected number of tasks %v but was %v", len(je.Tasks), len(other.Tasks))
	}
	for _, t := range other.Tasks {
		otherT := je.lookupTasks.GetObject(t.TaskType)
		if otherT == nil {
			return fmt.Errorf("could not find task of type %s", t.TaskType)
		}
		if err := t.Equals(otherT.(*TaskExecution)); err != nil {
			return err
		}
	}
	return nil
}

// AfterLoad initializes context properties
func (je *JobExecution) AfterLoad() error {
	je.lookupContexts = utils.NewSafeMap()
	je.lookupTasks = utils.NewSafeMap()
	for _, c := range je.Contexts {
		_, err := c.GetParsedValue()
		if err != nil {
			return err
		}
		je.lookupContexts.SetObject(c.Name, c)
	}
	for _, t := range je.Tasks {
		if err := t.AfterLoad(); err != nil {
			return err
		}
		je.lookupTasks.SetObject(t.TaskType, t)
	}
	if !je.StartedAt.IsZero() && je.EndedAt != nil {
		je.CPUSecs = je.ExecutionCostSecs()
	}
	return nil
}

// Validate validates job-execution
func (je *JobExecution) Validate() error {
	if je.JobType == "" {
		return errors.New("jobType is not specified")
	}
	if je.JobState == "" {
		return errors.New("jobState is not specified")
	}

	return nil
}

// ValidateBeforeSave validates job-execution
func (je *JobExecution) ValidateBeforeSave() error {
	if err := je.Validate(); err != nil {
		return err
	}
	for _, t := range je.Tasks {
		if err := t.ValidateBeforeSave(); err != nil {
			return err
		}
		je.lookupTasks.SetObject(t.TaskType, t)
	}
	if !je.StartedAt.IsZero() && je.EndedAt != nil {
		je.CPUSecs = je.ExecutionCostSecs()
	}

	return nil
}

// IMPLEMENTING JobRequestInfoSummary

// GetID request id
func (je JobExecution) GetID() uint64 {
	return je.JobRequestID
}

// GetJobPriority -- N/A
func (je JobExecution) GetJobPriority() int {
	return -1
}

// GetJobType - job type
func (je JobExecution) GetJobType() string {
	return je.JobType
}

// GetJobVersion - job version
func (je JobExecution) GetJobVersion() string {
	return je.JobVersion
}

// GetJobState - job state
func (je JobExecution) GetJobState() types.RequestState {
	return je.JobState
}

// GetOrganizationID -
func (je JobExecution) GetOrganizationID() string {
	return je.OrganizationID
}

// GetUserID -
func (je JobExecution) GetUserID() string {
	return je.UserID
}

// GetScheduledAt - N/A
func (je JobExecution) GetScheduledAt() time.Time {
	return je.StartedAt
}

// GetCreatedAt - N/A
func (je JobExecution) GetCreatedAt() time.Time {
	return je.StartedAt
}

// GetUserJobTypeKey - job-key with org/user
func (je JobExecution) GetUserJobTypeKey() string {
	return getUserJobTypeKey(je.OrganizationID, je.UserID, je.JobType, je.JobVersion)
}

// JobExecutionContext defines context for the job execution.
type JobExecutionContext struct {
	// Inheriting name, value, type
	types.NameTypeValue
	// ID defines UUID for primary key
	ID string `json:"id" gorm:"primary_key"`
	//gorm.Model
	// JobExecutionID defines foreign key for JobExecution
	JobExecutionID string `json:"job_execution_id"`
	// CreatedAt job context creation time
	CreatedAt time.Time `json:"created_at"`
}

// TableName overrides default table name
func (JobExecutionContext) TableName() string {
	return "formicary_job_execution_context"
}

// NewJobExecutionContext creates new job context
func NewJobExecutionContext(name string, value interface{}, secret bool) (*JobExecutionContext, error) {
	nv, err := types.NewNameTypeValue(name, value, secret)
	if err != nil {
		return nil, err
	}
	return &JobExecutionContext{
		NameTypeValue: nv,
		CreatedAt:     time.Now(),
	}, nil
}
