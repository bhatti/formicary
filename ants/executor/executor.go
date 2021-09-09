package executor

import (
	"context"
	"fmt"
	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/internal/utils"
	"plexobject.com/formicary/internal/utils/trace"
	"strings"
	"time"

	"plexobject.com/formicary/internal/types"
)

// State defines state of the executor
// https://godoc.org/github.com/docker/docker/api/types#ExecutorState
type State string

const (
	// Creating state
	Creating State = "CREATING"
	// Running state
	Running State = "RUNNING"
	// Removing state
	Removing State = "REMOVING"
	// Unknown state
	Unknown State = "UNKNOWN"
	// Pending state
	Pending State = "PENDING"
	// Succeeded state
	Succeeded State = "SUCCEEDED"
	// Failed state
	Failed State = "FAILED"
)

// ReadyOrRunning state
func (s State) ReadyOrRunning() bool {
	return s == Creating || s == Running || s == Pending
}

// Done state
func (s State) Done() bool {
	return s == Removing || s == Unknown || s == Succeeded || s == Failed
}

// Info interface defines methods for getting basic details
// swagger:ignore
type Info interface {
	GetID() string   // executor id
	GetName() string // executor name
	GetState() State // get executor state
	GetStartedAt() time.Time
	GetEndedAt() *time.Time
	GetLabels() map[string]string // returns labels
	ElapsedSecs() time.Duration
	GetRuntimeInfo(
		ctx context.Context) string // returns runtime information
}

// TraceWriter writes to job tracer
// swagger:ignore
type TraceWriter interface {
	WriteTrace(
		msg string) error
	WriteTraceInfo(
		msg string) (err error)
	WriteTraceSuccess(
		msg string) (err error)
	WriteTraceError(
		msg string) (err error)
}

// Executor interface defines methods for starting an executor and executing commands
// swagger:ignore
type Executor interface {
	GetID() string            // executor id
	GetName() string          // executor name
	GetHelperName() string    // executor helper name
	GetTrace() trace.JobTrace // fetch job trace
	GetState() State          // get executor state
	GetRuntimeInfo(
		ctx context.Context) string // returns runtime information
	ElapsedSecs() time.Duration
	AsyncExecute(
		ctx context.Context,
		cmd string,
		variables map[string]types.VariableValue,
	) (CommandRunner, error) // executes command asynchronously
	AsyncHelperExecute(
		ctx context.Context,
		cmd string,
		variables map[string]types.VariableValue,
	) (CommandRunner, error) // executes command asynchronously on helper container
	Stop() error // stops executor
	GetStartedAt() time.Time
	GetEndedAt() *time.Time
	Elapsed() string // time since executor started
	WriteTrace(
		msg string) error
	WriteTraceInfo(
		msg string) (err error)
	WriteTraceSuccess(
		msg string) (err error)
	WriteTraceError(
		msg string) (err error)
	GetHost() string
	GetContainerIP() string
	GetLabels() map[string]string // returns labels
}

// BaseExecutor struct defines attributes for the executor
// swagger:ignore
type BaseExecutor struct {
	*config.AntConfig
	*types.ExecutorOptions
	ID                string
	Name              string
	Host              string // host where command ran
	ContainerIP       string // container ip-address where command ran
	State             State
	StartedAt         time.Time
	EndedAt           *time.Time
	Trace             trace.JobTrace
	Labels            map[string]string
	Annotations       map[string]string
	DumpedRuntimeInfo bool
}

// NewBaseExecutor constructor for base executor
func NewBaseExecutor(
	cfg *config.AntConfig,
	trace trace.JobTrace,
	opts *types.ExecutorOptions) (BaseExecutor, error) {
	exec := BaseExecutor{
		AntConfig:       cfg,
		ExecutorOptions: opts,
		State:           Creating,
		StartedAt:       time.Now(),
		Labels:          opts.PodLabels,
		Annotations:     opts.PodAnnotations,
		Trace:           trace,
	}
	return exec, nil
}

// WriteTraceYellow writes message
func (e *BaseExecutor) WriteTraceYellow(msg string) (err error) {
	return e.WriteTrace(utils.AnsiBoldYellow + msg + utils.AnsiReset)
}

// WriteTraceInfo writes message
func (e *BaseExecutor) WriteTraceInfo(msg string) (err error) {
	return e.WriteTrace(utils.AnsiBoldCyan + msg + utils.AnsiReset)
}

// WriteTraceSuccess writes message
func (e *BaseExecutor) WriteTraceSuccess(msg string) (err error) {
	return e.WriteTrace(utils.AnsiBoldGreen + msg + utils.AnsiReset)
}

// WriteTraceError writes message
func (e *BaseExecutor) WriteTraceError(msg string) (err error) {
	return e.WriteTrace(utils.AnsiBoldRed + msg + utils.AnsiReset)
}

// WriteTrace writes message
func (e *BaseExecutor) WriteTrace(msg string) (err error) {
	helper := ""
	if strings.Contains(e.Name, "helper") {
		helper = "-helper"
	}
	_, err = e.Trace.Writeln(fmt.Sprintf("[%s %s %s%s] %s",
		time.Now().Format(time.RFC3339),
		e.ExecutorOptions.Method,
		//e.ID,
		e.ExecutorOptions.Name,
		helper,
		//e.ContainerIP,
		msg,
	))
	return
}

// String
func (e *BaseExecutor) String() string {
	return fmt.Sprintf("ID=%s, Name=%s, JobState=%v", e.GetID(), e.GetName(), e.GetState())
}

// GetRuntimeInfo returns runtime information
func (e *BaseExecutor) GetRuntimeInfo(context.Context) string {
	return fmt.Sprintf("ID=%s, Name=%s, JobState=%v", e.GetID(), e.GetName(), e.GetState())
}

// GetLabels returns labels
func (e *BaseExecutor) GetLabels() map[string]string {
	if e.Labels == nil {
		e.Labels = make(map[string]string, 0)
	}
	return e.Labels
}

// Elapsed returns time since executor started
func (e *BaseExecutor) Elapsed() string {
	if e.EndedAt == nil {
		return time.Now().Sub(e.StartedAt).String()
	}
	return e.EndedAt.Sub(e.StartedAt).String()
}

// ElapsedSecs returns time since executor started in secs
func (e *BaseExecutor) ElapsedSecs() time.Duration {
	if e.EndedAt == nil {
		return time.Duration(time.Now().Sub(e.StartedAt).Seconds()) * time.Second
	}
	return time.Duration(e.EndedAt.Sub(e.StartedAt).Seconds()) * time.Second
}

// GetStartedAt time
func (e *BaseExecutor) GetStartedAt() time.Time {
	return e.StartedAt
}

// GetEndedAt time
func (e *BaseExecutor) GetEndedAt() *time.Time {
	return e.EndedAt
}

// GetName returns name of the executor
func (e *BaseExecutor) GetName() string {
	return e.Name
}

// GetHelperName returns helper name of the executor
func (e *BaseExecutor) GetHelperName() string {
	return e.Name + "-helper"
}

// GetID returns id of the executor
func (e *BaseExecutor) GetID() string {
	return e.ID
}

// GetState returns Executor State
func (e *BaseExecutor) GetState() State {
	return e.State
}

// GetTrace -- trace buffer
func (e *BaseExecutor) GetTrace() trace.JobTrace {
	return e.Trace
}

// GetHost -- host for executor
func (e *BaseExecutor) GetHost() string {
	return e.Host
}

// GetContainerIP --
func (e *BaseExecutor) GetContainerIP() string {
	return e.ContainerIP
}
