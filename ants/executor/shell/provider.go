package shell

import (
	"context"
	"fmt"
	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/utils/trace"
	"sync"
)

// ExecutorProvider defines base structure for shell executor provider
type ExecutorProvider struct {
	executor.BaseExecutorProvider
	executors map[string]*Executor
	lock      sync.RWMutex
}

// NewExecutorProvider creates executor-provider for local shell based execution
func NewExecutorProvider(config *config.AntConfig) (executor.Provider, error) {
	return &ExecutorProvider{
		BaseExecutorProvider: executor.BaseExecutorProvider{
			AntConfig: config,
		},
		executors: make(map[string]*Executor),
	}, nil
}

// ListExecutors lists current executors
func (sep *ExecutorProvider) ListExecutors(context.Context) ([]executor.Info, error) {
	sep.lock.RLock()
	defer sep.lock.RUnlock()
	execs := make([]executor.Info, 0)
	for _, e := range sep.executors {
		execs = append(execs, e)
	}
	return execs, nil
}

// AllRunningExecutors returns running executors
func (sep *ExecutorProvider) AllRunningExecutors(ctx context.Context) ([]executor.Info, error) {
	return sep.ListExecutors(ctx)
}

// StopExecutor stops executor
func (sep *ExecutorProvider) StopExecutor(
	ctx context.Context,
	id string,
	_ *types.ExecutorOptions) error {
	sep.lock.Lock()
	defer sep.lock.Unlock()
	exec := sep.executors[id]
	if exec == nil {
		return fmt.Errorf("failed to find executor with id %v", id)
	}
	delete(sep.executors, id)
	return exec.Stop(ctx)
}

// NewExecutor creates new executor
func (sep *ExecutorProvider) NewExecutor(
	ctx context.Context,
	trace trace.JobTrace,
	opts *types.ExecutorOptions) (executor.Executor, error) {
	sep.lock.Lock()
	defer sep.lock.Unlock()
	exec, err := NewShellExecutor(ctx, sep.AntConfig, trace, opts)
	if err != nil {
		return nil, err
	}
	sep.executors[exec.ID] = exec
	return exec, nil
}
