// SPDX-License-Identifier: AGPL-3.0-or-later
// Hand-written extension methods for proto-generated JobExecution, TaskExecution,
// JobExecutionContext, and TaskExecutionContext types.
// This file is NEVER overwritten by buf generate.

package queen

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"plexobject.com/formicary/gen/go/formicary/v1/resource"
	"plexobject.com/formicary/internal/math"
)

const previousTaskExecutionCostSecsKey = "PreviousTaskExecutionCostSecs"
const previousTaskExecutionIDKey = "PreviousTaskExecutionID"

// lookupContextsKey is a sentinel used to store the safe-map on the JobExecution.
// Since proto-generated structs don't support unexported fields, we keep the map
// in a package-level sync.Map keyed by execution ID.
var jeContextCache sync.Map // map[string]map[string]*JobExecutionContext

func jeContexts(id string) map[string]*JobExecutionContext {
	if v, ok := jeContextCache.Load(id); ok {
		return v.(map[string]*JobExecutionContext)
	}
	m := make(map[string]*JobExecutionContext)
	jeContextCache.Store(id, m)
	return m
}

// TableName implements the GORM Tabler interface.
func (*JobExecution) TableName() string { return "formicary_job_executions" }

// TableName implements the GORM Tabler interface.
func (*JobExecutionContext) TableName() string { return "formicary_job_execution_context" }

// Summary provides a short human-readable summary of the job execution.
// (String() is reserved for the proto text-format representation.)
func (je *JobExecution) Summary() string {
	return fmt.Sprintf("ID=%s JobType=%s JobState=%s Context=%s;",
		je.Id, je.JobType, je.JobState, je.ContextString())
}

// JobTypeAndVersion returns type:version or just type if no version.
func (je *JobExecution) JobTypeAndVersion() string {
	if je.JobVersion == "" {
		return je.JobType
	}
	return je.JobType + ":" + je.JobVersion
}

// ElapsedDuration returns human-readable elapsed time for the execution.
func (je *JobExecution) ElapsedDuration() string {
	if je.EndedAt == nil || je.JobState == "EXECUTING" {
		return time.Since(je.StartedAt.AsTime()).String()
	}
	d := je.EndedAt.AsTime().Sub(je.StartedAt.AsTime())
	if d.Milliseconds() < 0 {
		return ""
	}
	return d.String()
}

// ElapsedMillis returns elapsed milliseconds.
func (je *JobExecution) ElapsedMillis() int64 {
	if je.EndedAt == nil || je.JobState == "EXECUTING" {
		return time.Since(je.StartedAt.AsTime()).Milliseconds()
	}
	elapsed := je.EndedAt.AsTime().Sub(je.StartedAt.AsTime()).Milliseconds()
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

// CostFactor returns the average cost factor across all tasks.
func (je *JobExecution) CostFactor() float64 {
	if len(je.Tasks) == 0 {
		return 0
	}
	var total float64
	for _, t := range je.Tasks {
		total += t.CostFactor
	}
	return total / float64(len(je.Tasks))
}

// ExecutionCostSecs returns the billable CPU seconds for the execution.
func (je *JobExecution) ExecutionCostSecs() int64 {
	var endTime time.Time
	if je.EndedAt == nil || je.JobState == "EXECUTING" {
		endTime = time.Now()
	} else {
		endTime = je.EndedAt.AsTime()
	}
	secs := int64(endTime.Sub(je.StartedAt.AsTime()).Seconds())
	if je.CpuSecs > 0 {
		return math.Max64(secs, je.CpuSecs)
	}
	var total int64
	for _, t := range je.Tasks {
		total += t.ExecutionCostSecs()
	}
	return math.Max64(secs, total)
}

// CanRestart returns true if the job can be restarted in its current state.
func (je *JobExecution) CanRestart() bool {
	return canRestart(je.JobState)
}

// CanCancel returns true if the job can be cancelled in its current state.
func (je *JobExecution) CanCancel() bool {
	return canCancel(je.JobState)
}

// CanApprove returns true if the job is awaiting manual approval.
func (je *JobExecution) CanApprove() bool {
	return canApprove(je.JobState)
}

// Completed returns true if the job completed successfully.
func (je *JobExecution) Completed() bool {
	return je.JobState == "COMPLETED"
}

// Failed returns true if the job reached a failure state.
func (je *JobExecution) Failed() bool {
	return je.JobState == "FAILED" || je.JobState == "FATAL"
}

// NotTerminal returns true if the job has not yet reached a terminal state.
func (je *JobExecution) NotTerminal() bool {
	return !isTerminalState(je.JobState)
}

// GetFailedTaskError returns the first non-optional failed task and its error details.
func (je *JobExecution) GetFailedTaskError() (*TaskExecution, string, error) {
	for _, t := range je.Tasks {
		if !t.AllowFailure && t.TaskState == "FAILED" {
			return t, t.ErrorCode, fmt.Errorf("%s", t.ErrorMessage)
		}
	}
	return nil, "", nil
}

// Stdout returns all stdout lines across all tasks.
func (je *JobExecution) Stdout() []string {
	res := make([]string, 0)
	for _, t := range je.Tasks {
		res = append(res, t.Stdout...)
	}
	return res
}

// Methods returns a comma-separated string of task execution methods.
func (je *JobExecution) Methods() string {
	seen := make(map[string]bool)
	for _, t := range je.Tasks {
		m := t.Method
		if m == "" {
			m = "KUBERNETES"
		}
		seen[m] = true
	}
	parts := make([]string, 0, len(seen))
	for k := range seen {
		parts = append(parts, k)
	}
	return strings.Join(parts, ", ")
}

// TasksString returns a textual summary of all task executions.
func (je *JobExecution) TasksString() string {
	var b strings.Builder
	for _, t := range je.Tasks {
		b.WriteString(t.Summary())
	}
	b.WriteString(";")
	return b.String()
}

// ContextString returns a textual summary of all execution context variables.
func (je *JobExecution) ContextString() string {
	var b strings.Builder
	for _, c := range je.Contexts {
		b.WriteString(c.Name + "=" + c.Value + " ")
	}
	b.WriteString(";")
	return b.String()
}

// ContextMap returns a map of execution context variables with parsed values.
func (je *JobExecution) ContextMap() map[string]interface{} {
	res := make(map[string]interface{})
	for _, c := range je.Contexts {
		res[c.Name] = c.Value
	}
	return res
}

// AddTask creates a TaskExecution from a task definition and attaches it.
func (je *JobExecution) AddTask(taskType string, method string, allowFailure bool) *TaskExecution {
	if _, old := je.GetTask("", taskType); old != nil {
		return old
	}
	maxOrder := 0
	for _, t := range je.Tasks {
		if t.TaskOrder > int32(maxOrder) {
			maxOrder = int(t.TaskOrder)
		}
	}
	taskExec := &TaskExecution{
		JobExecutionId: je.Id,
		TaskType:       taskType,
		Method:         method,
		AllowFailure:   allowFailure,
		TaskState:      "READY",
		TaskOrder:      int32(maxOrder + 1),
		Contexts:       make([]*TaskExecutionContext, 0),
	}
	je.Tasks = append(je.Tasks, taskExec)
	return taskExec
}

// DeleteTask removes a task execution by ID.
func (je *JobExecution) DeleteTask(id string) bool {
	for i, t := range je.Tasks {
		if t.Id == id {
			je.Tasks = append(je.Tasks[:i], je.Tasks[i+1:]...)
			return true
		}
	}
	return false
}

// GetTask finds a task by ID or task type.
func (je *JobExecution) GetTask(id string, taskType string) (int, *TaskExecution) {
	for i, t := range je.Tasks {
		if (id != "" && t.Id == id) || (taskType != "" && t.TaskType == taskType) {
			return i, t
		}
	}
	return -1, nil
}

// GetLastTask returns the last task in execution order.
func (je *JobExecution) GetLastTask() *TaskExecution {
	if len(je.Tasks) > 0 {
		return je.Tasks[len(je.Tasks)-1]
	}
	return nil
}

// GetLastExecutedTask returns the most recently executed (non-processing) task.
func (je *JobExecution) GetLastExecutedTask() (last *TaskExecution) {
	for _, t := range je.Tasks {
		if t.TaskState != "EXECUTING" && t.TaskState != "READY" &&
			(last == nil || last.TaskOrder < t.TaskOrder) {
			last = t
		}
	}
	return
}

// AddContext adds or updates a named execution context variable.
func (je *JobExecution) AddContext(name string, value string, kind string) *JobExecutionContext {
	ctx := &JobExecutionContext{
		Id:             ulid(),
		JobExecutionId: je.Id,
		Name:           name,
		Value:          value,
		Kind:           kind,
	}
	cache := jeContexts(je.Id)
	if _, exists := cache[name]; !exists {
		je.Contexts = append(je.Contexts, ctx)
	} else {
		for _, c := range je.Contexts {
			if c.Name == name {
				c.Value = value
			}
		}
	}
	cache[name] = ctx
	return ctx
}

// DeleteContext removes a context variable by name.
func (je *JobExecution) DeleteContext(name string) *JobExecutionContext {
	cache := jeContexts(je.Id)
	old, exists := cache[name]
	if !exists {
		return nil
	}
	delete(cache, name)
	for i, c := range je.Contexts {
		if c.Name == name {
			je.Contexts = append(je.Contexts[:i], je.Contexts[i+1:]...)
			return old
		}
	}
	return nil
}

// GetContext retrieves a context variable by name.
func (je *JobExecution) GetContext(name string) *JobExecutionContext {
	return jeContexts(je.Id)[name]
}

// AfterLoad initializes the context lookup cache from the persisted Contexts slice.
func (je *JobExecution) AfterLoad() error {
	cache := make(map[string]*JobExecutionContext, len(je.Contexts))
	for _, c := range je.Contexts {
		cache[c.Name] = c
	}
	jeContextCache.Store(je.Id, cache)
	for _, t := range je.Tasks {
		t.AfterLoad()
	}
	return nil
}

// Validate checks required fields on the job execution.
func (je *JobExecution) Validate() error {
	if je.JobType == "" {
		return errors.New("jobType is not specified")
	}
	if je.JobState == "" {
		return errors.New("jobState is not specified")
	}
	return nil
}

// ValidateBeforeSave validates the job execution and all child task executions.
func (je *JobExecution) ValidateBeforeSave() error {
	if err := je.Validate(); err != nil {
		return err
	}
	for _, t := range je.Tasks {
		if err := t.ValidateBeforeSave(); err != nil {
			return err
		}
	}
	return nil
}

// Equals performs a deep equality check against another JobExecution.
func (je *JobExecution) Equals(other *JobExecution) error {
	if other == nil {
		return errors.New("found nil other job")
	}
	if je.JobType != other.JobType {
		return fmt.Errorf("expected jobType %v but was %v", je.JobType, other.JobType)
	}
	if len(je.Contexts) != len(other.Contexts) {
		return fmt.Errorf("expected %d contexts but was %d", len(je.Contexts), len(other.Contexts))
	}
	cache := jeContexts(je.Id)
	for _, c := range other.Contexts {
		if mine, ok := cache[c.Name]; !ok || mine.Value != c.Value {
			return fmt.Errorf("context mismatch for %s", c.Name)
		}
	}
	if len(je.Tasks) != len(other.Tasks) {
		return fmt.Errorf("expected %d tasks but was %d", len(je.Tasks), len(other.Tasks))
	}
	for _, t := range other.Tasks {
		_, mine := je.GetTask(t.Id, t.TaskType)
		if mine == nil {
			return fmt.Errorf("could not find task %s", t.TaskType)
		}
		if err := mine.Equals(t); err != nil {
			return err
		}
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// TaskExecution extension methods
// ──────────────────────────────────────────────────────────────────────────────

var teContextLocks sync.Map // map[string]*sync.RWMutex

func teLock(id string) *sync.RWMutex {
	v, _ := teContextLocks.LoadOrStore(id, &sync.RWMutex{})
	return v.(*sync.RWMutex)
}

// TableName implements the GORM Tabler interface.
func (*TaskExecution) TableName() string { return "formicary_task_executions" }

// TableName implements the GORM Tabler interface.
func (*TaskExecutionContext) TableName() string { return "formicary_task_execution_context" }

// Summary provides a short human-readable summary of the task execution.
func (te *TaskExecution) Summary() string {
	return fmt.Sprintf("ID=%s TaskType=%s Contexts=%s TaskState=%s ExitCode=%s ErrorCode=%s",
		te.Id, te.TaskType, te.ContextString(), te.TaskState, te.ExitCode, te.ErrorCode)
}

// ElapsedDuration returns human-readable elapsed time.
func (te *TaskExecution) ElapsedDuration() string {
	if te.EndedAt == nil || te.TaskState == "EXECUTING" {
		return time.Since(te.StartedAt.AsTime()).String()
	}
	d := te.EndedAt.AsTime().Sub(te.StartedAt.AsTime())
	if d.Milliseconds() < 0 {
		return ""
	}
	return d.String()
}

// ExecutionCostSecs returns the billable CPU seconds for the task.
func (te *TaskExecution) ExecutionCostSecs() int64 {
	var endTime time.Time
	if te.EndedAt == nil || te.TaskState == "EXECUTING" {
		endTime = time.Now()
	} else {
		endTime = te.EndedAt.AsTime()
	}
	secs := int64(endTime.Sub(te.StartedAt.AsTime()).Seconds())
	cost := math.Max64(int64(float64(secs)*te.CostFactor), secs)
	cost += te.GetPreviousExecutionCostSecs()
	if te.CostFactor == 0 {
		return cost + int64(te.CountServices)*cost
	}
	return math.Max64(cost, 0)
}

// CanRestart returns true if the task can be restarted.
func (te *TaskExecution) CanRestart() bool { return canRestart(te.TaskState) }

// CanCancel returns true if the task can be cancelled.
func (te *TaskExecution) CanCancel() bool { return canCancel(te.TaskState) }

// CanApprove returns true if the task requires manual approval.
func (te *TaskExecution) CanApprove() bool { return canApprove(te.TaskState) }

// IsManuallyApproved returns true if the task has been manually reviewed.
func (te *TaskExecution) IsManuallyApproved() bool {
	return te.ManualReviewedBy != "" && te.ManualReviewedAt != nil
}

// Completed returns true if the task completed successfully.
func (te *TaskExecution) Completed() bool { return te.TaskState == "COMPLETED" }

// Paused returns true if the task is paused.
func (te *TaskExecution) Paused() bool { return te.TaskState == "PAUSED" }

// Failed returns true if the task failed.
func (te *TaskExecution) Failed() bool { return te.TaskState == "FAILED" || te.TaskState == "FATAL" }

// NotTerminal returns true if the task has not reached a terminal state.
func (te *TaskExecution) NotTerminal() bool { return !isTerminalState(te.TaskState) }

// SetStatus updates the task state.
func (te *TaskExecution) SetStatus(status string) *TaskExecution {
	te.TaskState = status
	return te
}

// ContextString returns a textual summary of task context variables.
func (te *TaskExecution) ContextString() string {
	var b strings.Builder
	for _, c := range te.Contexts {
		b.WriteString(c.Name + "=" + c.Value + ",")
	}
	b.WriteString(";")
	return b.String()
}

// ContextMap returns context variables as a map.
func (te *TaskExecution) ContextMap() map[string]interface{} {
	res := make(map[string]interface{})
	for _, c := range te.Contexts {
		res[c.Name] = c.Value
	}
	return res
}

// GetContext retrieves a context variable by name.
func (te *TaskExecution) GetContext(name string) *TaskExecutionContext {
	mu := teLock(te.Id)
	mu.RLock()
	defer mu.RUnlock()
	for _, c := range te.Contexts {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// AddContext adds or updates a task context variable.
func (te *TaskExecution) AddContext(name string, value interface{}) (*TaskExecutionContext, error) {
	strVal := fmt.Sprintf("%v", value)
	ctx := &TaskExecutionContext{
		Id:              ulid(),
		TaskExecutionId: te.Id,
		Name:            name,
		Value:           strVal,
	}
	mu := teLock(te.Id)
	mu.Lock()
	defer mu.Unlock()
	for _, c := range te.Contexts {
		if c.Name == name {
			c.Value = strVal
			return ctx, nil
		}
	}
	te.Contexts = append(te.Contexts, ctx)
	return ctx, nil
}

// DeleteContext removes a context variable by name.
func (te *TaskExecution) DeleteContext(name string) *TaskExecutionContext {
	mu := teLock(te.Id)
	mu.Lock()
	defer mu.Unlock()
	for i, c := range te.Contexts {
		if c.Name == name {
			te.Contexts = append(te.Contexts[:i], te.Contexts[i+1:]...)
			return c
		}
	}
	return nil
}

// AddArtifact attaches an artifact to the task execution.
func (te *TaskExecution) AddArtifact(art *resource.Artifact) {
	te.Artifacts = append(te.Artifacts, art)
}

// LogArtifact returns the first artifact of kind LOGS, if any.
func (te *TaskExecution) LogArtifact() *resource.Artifact {
	for _, ar := range te.Artifacts {
		if ar.Kind == "LOGS" {
			return ar
		}
	}
	return nil
}

// AddPreviousExecutionCostSecs stores the cost from a previous task execution attempt.
func (te *TaskExecution) AddPreviousExecutionCostSecs(prevID string, secs int64) {
	if secs > 0 {
		_, _ = te.AddContext(previousTaskExecutionCostSecsKey, secs)
		_, _ = te.AddContext(previousTaskExecutionIDKey, prevID)
	}
}

// GetPreviousExecutionCostSecs retrieves the previously stored execution cost.
func (te *TaskExecution) GetPreviousExecutionCostSecs() int64 {
	for _, c := range te.Contexts {
		if c.Name == previousTaskExecutionCostSecsKey {
			n, _ := strconv.ParseInt(c.Value, 10, 64)
			return n
		}
	}
	return 0
}

// AfterLoad initialises the context lookup structures from persisted data.
func (te *TaskExecution) AfterLoad() {
	// Nothing to pre-compute — GetContext iterates Contexts directly.
}

// Validate checks required fields on the task execution.
func (te *TaskExecution) Validate() error {
	if te.TaskType == "" {
		return errors.New("taskType is not specified")
	}
	return nil
}

// ValidateBeforeSave validates the task execution before persistence.
func (te *TaskExecution) ValidateBeforeSave() error {
	return te.Validate()
}

// Equals performs a deep equality check against another TaskExecution.
func (te *TaskExecution) Equals(other *TaskExecution) error {
	if other == nil {
		return errors.New("found nil other task")
	}
	if te.TaskType != other.TaskType {
		return fmt.Errorf("expected taskType %v but was %v", te.TaskType, other.TaskType)
	}
	if len(te.Contexts) != len(other.Contexts) {
		return fmt.Errorf("expected %d contexts but was %d", len(te.Contexts), len(other.Contexts))
	}
	for _, c := range other.Contexts {
		mine := te.GetContext(c.Name)
		if mine == nil || mine.Value != c.Value {
			return fmt.Errorf("context mismatch for %s", c.Name)
		}
	}
	return nil
}

// State-machine helpers are defined in ext_helpers.go (shared with job_request_ext.go).
