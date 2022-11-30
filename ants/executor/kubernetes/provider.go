package kubernetes

import (
	"context"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
	api "k8s.io/api/core/v1"
	"plexobject.com/formicary/internal/utils/trace"

	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/internal/types"
)

// ExecutorProvider defines base structure for kubernetes executor provider
type ExecutorProvider struct {
	executor.BaseExecutorProvider
	adapter             Adapter
	registryCredentials *api.Secret
	executors           map[string]*Executor
	lock                sync.RWMutex
}

// NewExecutorProvider creates executor-provider for local kubernetes based execution
func NewExecutorProvider(
	config *config.AntConfig) (executor.Provider, error) {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithFields(log.Fields{
			"Component": "KubernetesExecutorProvider",
			"Namespace": config.Kubernetes.Namespace,
			"Server":    config.Kubernetes.Server,
			"Username":  config.Kubernetes.Username,
		}).Debug("connecting to Kubernetes ...")
	}

	cli, restConfig, err := getKubeClient(config)
	if err != nil {
		return nil, err
	}
	adapter, err := NewKubernetesUtils(config, cli, restConfig)
	if err != nil {
		return nil, err
	}
	return &ExecutorProvider{
		BaseExecutorProvider: executor.BaseExecutorProvider{
			AntConfig: config,
		},
		adapter:   adapter,
		executors: make(map[string]*Executor),
	}, nil
}

// ListExecutors lists current executors
func (kep *ExecutorProvider) ListExecutors(
	context.Context) ([]executor.Info, error) {
	execs := make([]executor.Info, 0)
	kep.lock.RLock()
	defer kep.lock.RUnlock()
	for _, e := range kep.executors {
		execs = append(execs, e)
	}
	return execs, nil
}

// AllRunningExecutors returns running executors
func (kep *ExecutorProvider) AllRunningExecutors(
	ctx context.Context) ([]executor.Info, error) {
	kep.lock.RLock()
	defer kep.lock.RUnlock()
	return kep.adapter.List(ctx)
}

// StopExecutor stops executor
func (kep *ExecutorProvider) StopExecutor(
	ctx context.Context,
	id string,
	_ *types.ExecutorOptions) error {
	kep.lock.Lock()
	defer kep.lock.Unlock()
	exec := kep.executors[id]
	if exec == nil {
		log.WithFields(log.Fields{
			"Component": "KubernetesExecutorProvider",
			"Name":      id,
		}).Warn("✋ stopping unknown pod")
		return kep.adapter.Stop(ctx, id)
	}
	delete(kep.executors, id)
	return exec.Stop(ctx)
}

// NewExecutor creates new executor
func (kep *ExecutorProvider) NewExecutor(
	ctx context.Context,
	trace trace.JobTrace,
	opts *types.ExecutorOptions) (executor.Executor, error) {
	kep.lock.Lock()
	defer kep.lock.Unlock()
	var err error

	kep.registryCredentials, err = kep.adapter.BuildRegistryCredentials(ctx)
	if err != nil {
		_, _ = trace.Writeln(fmt.Sprintf("📌 failed to setup registry credentials due to %s", err), types.ExecTags)
		return nil, fmt.Errorf("setting up registryCredentials due to %w", err)
	}
	exec, err := NewKubernetesExecutor(ctx, kep.AntConfig, trace, kep.adapter, kep.registryCredentials, opts)
	if err != nil {
		return nil, err
	}
	kep.executors[exec.ID] = exec
	return exec, nil
}

// Dispose disposes connection
func (kep *ExecutorProvider) Dispose(
	ctx context.Context) []error {
	kep.lock.Lock()
	defer kep.lock.Unlock()
	return kep.adapter.Dispose(
		ctx,
		kep.AntConfig.Kubernetes.Namespace,
		make([]api.Service, 0),
		kep.registryCredentials,
		nil,
		kep.AntConfig.GetShutdownTimeout())
}
