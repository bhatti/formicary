package executor

import (
	"context"
	"plexobject.com/formicary/internal/ant_config"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/utils/trace"
)

// Provider defines operations to manage executors
type Provider interface {
	// AllRunningExecutors lists all running executors
	AllRunningExecutors(ctx context.Context) ([]Info, error)
	// ListExecutors lists current executors
	ListExecutors(ctx context.Context) ([]Info, error)
	// StopExecutor stops executor
	StopExecutor(
		ctx context.Context,
		id string,
		opts *types.ExecutorOptions) error
	// NewExecutor creates new executor
	NewExecutor(
		ctx context.Context,
		trace trace.JobTrace,
		opts *types.ExecutorOptions,
	) (Executor, error)
}

// BaseExecutorProvider defines base structure for executor provider
type BaseExecutorProvider struct {
	*ant_config.AntConfig
}
