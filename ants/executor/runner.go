package executor

import (
	"bytes"
	"context"
	"io"
	common "plexobject.com/formicary/internal/types"
	"time"
)

// CommandRunner provides access to results of defines methods for executing a script command
type CommandRunner interface {
	Await(
		ctx context.Context) (
		stdout []byte,
		stderr []byte,
		err error)                               // awaits completion
	IsRunning(ctx context.Context) (bool, error) // checks if command is running
	IsHelper(ctx context.Context) bool
	Stop(ctx context.Context, timeout time.Duration) error // stops/kill command
	GetExitCode() int
	GetExitMessage() string
	Elapsed() string // time since script started
}

// BaseCommandRunner struct defines attributes for execution context of executor while we execute the command
type BaseCommandRunner struct {
	*BaseExecutor
	ID              string       // command-executor-id
	Command         string       // command to execute
	ExitMessage     string       // exit message
	ExitCode        int          // exit-code
	Stdout          bytes.Buffer // captures stdout
	Stderr          bytes.Buffer // captures stderr
	Stdin           io.Reader    // captures stdin
	Running         bool         // keeps state of running
	Err             error        // captures error
	StartedAt       time.Time    // start-time
	HelperContainer bool         // for helper container
}

// NewBaseCommandRunner constructor base runner
func NewBaseCommandRunner(executor *BaseExecutor, cmd string, helper bool) BaseCommandRunner {
	return BaseCommandRunner{
		BaseExecutor:    executor,
		Command:         cmd,
		HelperContainer: helper,
		StartedAt:       time.Now(),
	}
}

// Elapsed returns time since script command started
func (r *BaseCommandRunner) Elapsed() string {
	return time.Since(r.StartedAt).String()
}

// GetExitCode returns exit code
func (r *BaseCommandRunner) GetExitCode() int {
	return r.ExitCode
}

// GetExitMessage returns exit message
func (r *BaseCommandRunner) GetExitMessage() string {
	return r.ExitMessage
}

// IsHelper checks if container is helper
func (r *BaseCommandRunner) IsHelper(ctx context.Context) bool {
	return ctx.Value(common.HelperContainerKey) != nil
}
