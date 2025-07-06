package types

import (
	"errors"
	"fmt"
	"plexobject.com/formicary/internal/math"
	"strconv"
	"strings"
	"sync"
	"time"

	"plexobject.com/formicary/internal/types"
)

const previousTaskExecutionCostSecs = "PreviousTaskExecutionCostSecs"
const previousTaskExecutionID = "PreviousTaskExecutionID"

// TaskExecution records the execution of a task or a unit of work, carried out by ant-workers in accordance
// with the specifications of the task-definition. It captures the status and the outputs produced by the
// task execution, storing them in the database and the object-store.
type TaskExecution struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID string `json:"id" gorm:"primary_key"`
	// JobExecutionID defines foreign key for JobExecution
	JobExecutionID string `json:"job_execution_id"`
	// TaskType defines type of task
	TaskType string `json:"task_type"`
	// Method defines method of communication
	Method types.TaskMethod `yaml:"method" json:"method"`
	// TaskState defines state of task that is maintained throughout the lifecycle of a task
	TaskState types.RequestState `json:"task_state"`
	// AllowFailure means the task is optional and can fail without failing entire job
	AllowFailure bool `json:"allow_failure"`
	// ExitCode defines exit status from the job execution
	ExitCode string `json:"exit_code"`
	// ExitMessage defines exit message from the job execution
	ExitMessage string `json:"exit_message"`
	// ErrorCode captures error code at the end of job execution if it fails
	ErrorCode string `json:"error_code"`
	// ErrorMessage captures error message at the end of job execution if it fails
	ErrorMessage string `json:"error_message"`
	// FailedCommand captures command that failed
	FailedCommand string `json:"failed_command"`
	// Comments
	Comments string `json:"comments"`
	// AntID - id of ant with version
	AntID string `json:"ant_id"`
	// AntHost - host where ant ran the task
	AntHost string `json:"ant_host"`
	// ManualApprovedBy for manual task
	ManualApprovedBy string `json:"manual_approved_by" gorm:"manual_approved_by"`
	// ManualApprovedAt for manual task
	ManualApprovedAt *time.Time `json:"manual_approved_at" gorm:"manual_approved_at"`
	// Retried keeps track of retry attempts
	Retried int `json:"retried"`
	// Contexts defines context variables of task
	Contexts []*TaskExecutionContext `json:"contexts" gorm:"ForeignKey:TaskExecutionID" gorm:"auto_preload"`
	// Artifacts defines list of artifacts that are generated for the task
	Artifacts []*types.Artifact `json:"artifacts" gorm:"ForeignKey:TaskExecutionID"`
	// TaskOrder
	TaskOrder int `json:"task_order"`
	// CountServices
	CountServices int `json:"count_services"`
	// CostFactor
	CostFactor float64 `json:"cost_factor"`
	// StartedAt job creation time
	StartedAt time.Time `json:"started_at"`
	// EndedAt job update time
	EndedAt *time.Time `json:"ended_at"`
	// UpdatedAt job execution last update time
	UpdatedAt time.Time `json:"updated_at"`
	// Active is used to softly delete job definition
	Active bool     `yaml:"-" json:"-"`
	Stdout []string `json:"stdout" gorm:"-"`
	// Transient properties -- these are populated when AfterLoad or Validate is called
	lookupContexts map[string]*TaskExecutionContext
	lookupLock     sync.RWMutex
}

// TableName overrides default table name
func (TaskExecution) TableName() string {
	return "formicary_task_executions"
}

// NewTaskExecution creates new instance of task-execution
func NewTaskExecution(task *TaskDefinition) *TaskExecution {
	var taskExec TaskExecution
	taskExec.TaskType = task.TaskType
	taskExec.Method = task.Method
	taskExec.AllowFailure = task.AllowFailure
	taskExec.Contexts = make([]*TaskExecutionContext, 0)
	taskExec.TaskState = types.READY
	taskExec.StartedAt = time.Now()
	taskExec.UpdatedAt = time.Now()
	taskExec.lookupContexts = make(map[string]*TaskExecutionContext)
	return &taskExec
}

// String provides short summary of task
func (te *TaskExecution) String() string {
	return fmt.Sprintf("ID=%s TaskType=%s Contexts=%s TaskState=%s ExitCode=%s ErrorCode=%s",
		te.ID, te.TaskType, te.ContextString(), te.TaskState, te.ExitCode, te.ErrorCode)
}

// ElapsedDuration time duration of job execution
func (te *TaskExecution) ElapsedDuration() string {
	if te.EndedAt == nil || te.TaskState == types.EXECUTING {
		return time.Now().Sub(te.StartedAt).String()
	}
	if te.EndedAt.Sub(te.StartedAt).Milliseconds() < 0 {
		return ""
	}
	return te.EndedAt.Sub(te.StartedAt).String()
}

// ExecutionCostSecs cost of execution
func (te *TaskExecution) ExecutionCostSecs() int64 {
	ended := te.EndedAt
	if ended == nil || te.TaskState == types.EXECUTING {
		now := time.Now()
		ended = &now
	}
	cost := math.Max64(int64(ended.Sub(te.StartedAt).Seconds()*te.CostFactor),
		int64(ended.Sub(te.StartedAt).Seconds()))
	cost += te.GetPreviousExecutionCostSecs()
	if te.CostFactor == 0 {
		return cost + int64(te.CountServices)*cost
	}
	return math.Max64(cost, 0)
}

// CanRestart checks if task can be restarted
func (te *TaskExecution) CanRestart() bool {
	return te.TaskState.CanRestart()
}

// CanCancel checks if task can be cancelled
func (te *TaskExecution) CanCancel() bool {
	return te.TaskState.CanCancel()
}

// CanApprove checks if task can be approved
func (te *TaskExecution) CanApprove() bool {
	return te.TaskState.CanApprove()
}

// IsManuallyApproved checks if task was manually approved
func (te *TaskExecution) IsManuallyApproved() bool {
	return te.ManualApprovedBy != "" && te.ManualApprovedAt != nil
}

// Completed task
func (te *TaskExecution) Completed() bool {
	return te.TaskState.Completed()
}

// Paused task
func (te *TaskExecution) Paused() bool {
	return te.TaskState.Paused()
}

// Failed task
func (te *TaskExecution) Failed() bool {
	return te.TaskState.Failed()
}

// NotTerminal task that is not in final state
func (te *TaskExecution) NotTerminal() bool {
	return !te.TaskState.IsTerminal()
}

// ContextString textual context
func (te *TaskExecution) ContextString() string {
	var b strings.Builder
	for _, c := range te.Contexts {
		b.WriteString(c.Name + "=" + c.Value + ",")
	}
	b.WriteString(";")
	return b.String()
}

// ContextMap map representation
func (te *TaskExecution) ContextMap() map[string]interface{} {
	res := make(map[string]interface{})
	for _, c := range te.Contexts {
		if val, err := c.GetParsedValue(); err == nil {
			res[c.Name] = val
		}
	}
	return res
}

// GetContext gets task context
func (te *TaskExecution) GetContext(name string) *TaskExecutionContext {
	te.lookupLock.RLock()
	defer te.lookupLock.RUnlock()
	return te.lookupContexts[name]
}

// SetStatus updates status
func (te *TaskExecution) SetStatus(status types.RequestState) *TaskExecution {
	te.TaskState = status
	return te
}

// AddArtifact adds artifact
func (te *TaskExecution) AddArtifact(art *types.Artifact) {
	te.Artifacts = append(te.Artifacts, art)
}

// LogArtifact finds logs
func (te *TaskExecution) LogArtifact() *types.Artifact {
	for _, ar := range te.Artifacts {
		if ar.Kind == types.ArtifactKindLogs {
			return ar
		}
	}
	return nil
}

// AddPreviousExecutionCostSecs adds previous task execution
func (te *TaskExecution) AddPreviousExecutionCostSecs(previousTaskExecutionID string, secs int64) {
	if secs > 0 {
		_, _ = te.AddContext(previousTaskExecutionCostSecs, secs)
		_, _ = te.AddContext(previousTaskExecutionID, previousTaskExecutionID)
	}
}

// GetPreviousExecutionCostSecs finds previous task execution
func (te *TaskExecution) GetPreviousExecutionCostSecs() int64 {
	for _, c := range te.Contexts {
		if c.Name == previousTaskExecutionCostSecs {
			n, _ := strconv.ParseInt(c.Value, 10, 64)
			return n
		}
	}
	return 0
}

// AddContext adds task context
func (te *TaskExecution) AddContext(
	name string,
	value interface{}) (*TaskExecutionContext, error) {
	ctx, err := NewTaskExecutionContext(name, value)
	if err != nil {
		return nil, err
	}
	ctx.TaskExecutionID = te.ID
	te.lookupLock.Lock()
	defer te.lookupLock.Unlock()
	if te.lookupContexts[name] == nil {
		te.Contexts = append(te.Contexts, ctx)
	} else {
		for _, next := range te.Contexts {
			if next.Name == name {
				next.Value = ctx.Value
			}
		}
	}
	te.lookupContexts[name] = ctx
	return ctx, nil
}

// DeleteContext removes context by name
func (te *TaskExecution) DeleteContext(name string) *TaskExecutionContext {
	te.lookupLock.Lock()
	defer te.lookupLock.Unlock()
	old := te.lookupContexts[name]
	if old == nil {
		return nil
	}
	delete(te.lookupContexts, name)
	for i, c := range te.Contexts {
		if c.Name == name {
			te.Contexts = append(te.Contexts[:i], te.Contexts[i+1:]...)
			return c
		}
	}
	return nil
}

// Equals compares other task-definition for equality
func (te *TaskExecution) Equals(other *TaskExecution) error {
	if other == nil {
		return errors.New("found nil other task")
	}
	if err := te.ValidateBeforeSave(); err != nil {
		return err
	}
	if err := other.ValidateBeforeSave(); err != nil {
		return err
	}

	if te.TaskType != other.TaskType {
		return fmt.Errorf("expected taskType %v but was %v", te.TaskType, other.TaskType)
	}
	if len(te.Contexts) != len(other.Contexts) {
		return fmt.Errorf("expected number of context variables %v but was %v", len(te.Contexts), len(other.Contexts))
	}
	te.lookupLock.RLock()
	defer te.lookupLock.RUnlock()
	for _, c := range other.Contexts {
		if te.lookupContexts[c.Name] == nil || te.lookupContexts[c.Name].Value != c.Value {
			return fmt.Errorf("expected context variables for %v as %v but was %v", c.Name, te.lookupContexts[c.Name], c.Value)
		}
	}
	return nil
}

// AfterLoad initializes task
func (te *TaskExecution) AfterLoad() error {
	te.lookupLock.Lock()
	defer te.lookupLock.Unlock()
	te.lookupContexts = make(map[string]*TaskExecutionContext)
	for _, c := range te.Contexts {
		_, err := c.GetParsedValue()
		if err != nil {
			return err
		}
		te.lookupContexts[c.Name] = c
	}
	for _, a := range te.Artifacts {
		_ = a.AfterLoad()
	}
	return nil
}

// Validate validates task
func (te *TaskExecution) Validate() error {
	if te.TaskType == "" {
		return errors.New("taskType is not specified")
	}
	return nil
}

// ValidateBeforeSave updates state before save
func (te *TaskExecution) ValidateBeforeSave() error {
	//if te.JobExecutionID == "" {
	//	return errors.New("jobExecutionID is not specified in task execution")
	//}
	return te.Validate()
}

// TaskExecutionContext defines context variables for the task execution that are captured by the task executor.
type TaskExecutionContext struct {
	//gorm.Model
	// Inheriting name, value, type
	types.NameTypeValue
	// ID defines UUID for primary key
	ID string `json:"id" gorm:"primary_key"`
	// TaskExecutionID defines foreign key for task-execution
	TaskExecutionID string `json:"task_execution_id"`
	// CreatedAt task context creation time
	CreatedAt time.Time `json:"created_at"`
}

// TableName overrides default table name
func (TaskExecutionContext) TableName() string {
	return "formicary_task_execution_context"
}

// NewTaskExecutionContext creates new task context variables
func NewTaskExecutionContext(
	name string,
	value interface{}) (*TaskExecutionContext, error) {
	nv, err := types.NewNameTypeValue(name, value, false)
	if err != nil {
		return nil, err
	}
	return &TaskExecutionContext{
		NameTypeValue: nv,
		CreatedAt:     time.Now(),
	}, nil
}
