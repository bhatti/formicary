package docker

import (
	"context"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/utils/trace"
	"sync"
)

// ExecutorProvider defines base structure for docker executor provider
type ExecutorProvider struct {
	executor.BaseExecutorProvider
	cli       *client.Client
	adapter   Adapter
	executors map[string]*Executor
	lock      sync.RWMutex
}

// NewExecutorProvider creates executor-provider for local docker based execution
func NewExecutorProvider(
	config *config.AntConfig) (executor.Provider, error) {
	log.WithFields(log.Fields{
		"Component": "DockerExecutorProvider",
		"Host":      config.Docker.Host,
	}).Info("docker client connecting...")
	cli, err := client.NewClientWithOpts(
		client.WithHost(config.Docker.Host),
		client.WithVersion("1.37"))
	if err != nil {
		return nil, err
	}
	return &ExecutorProvider{
		BaseExecutorProvider: executor.BaseExecutorProvider{
			AntConfig: config,
		},
		executors: make(map[string]*Executor),
		cli:       cli,
		adapter:   NewDockerUtils(&config.Docker, cli),
	}, nil
}

// ListExecutors lists current executors
func (dep *ExecutorProvider) ListExecutors(
	context.Context) ([]executor.Info, error) {
	dep.lock.RLock()
	defer dep.lock.RUnlock()
	execs := make([]executor.Info, 0)
	for _, e := range dep.executors {
		execs = append(execs, e)
	}
	return execs, nil
}

// AllRunningExecutors returns running executors
func (dep *ExecutorProvider) AllRunningExecutors(
	ctx context.Context) ([]executor.Info, error) {
	dep.lock.RLock()
	defer dep.lock.RUnlock()
	return dep.adapter.List(ctx)
}

// StopExecutor stops executor
func (dep *ExecutorProvider) StopExecutor(
	ctx context.Context,
	id string,
	opts *types.ExecutorOptions) error {
	dep.lock.Lock()
	defer dep.lock.Unlock()
	exec := dep.executors[id]
	if exec == nil {
		log.WithFields(log.Fields{
			"Component": "DockerExecutorProvider",
			"Name":      id,
		}).Warn("✋ stopping unknown container")
		return dep.adapter.Stop(
			ctx,
			id,
			opts,
			dep.AntConfig.GetShutdownTimeout())
	}
	delete(dep.executors, id)
	return exec.Stop(ctx)
}

// NewExecutor creates new executor
func (dep *ExecutorProvider) NewExecutor(
	ctx context.Context,
	trace trace.JobTrace,
	opts *types.ExecutorOptions) (executor.Executor, error) {
	dep.lock.Lock()
	defer dep.lock.Unlock()
	exec, err := NewDockerExecutor(ctx, dep.AntConfig, trace, opts, dep.adapter)
	if err != nil {
		return nil, err
	}
	dep.executors[exec.ID] = exec
	return exec, nil
}
