package utils

import (
	"context"
	"fmt"
	"plexobject.com/formicary/internal/ant_config"
	"plexobject.com/formicary/internal/health"
	"plexobject.com/formicary/internal/utils/trace"

	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/ants/executor/docker"
	"plexobject.com/formicary/ants/executor/http"
	"plexobject.com/formicary/ants/executor/kubernetes"
	"plexobject.com/formicary/ants/executor/shell"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
)

// StopContainer stops a running container
func StopContainer(
	ctx context.Context,
	antCfg *ant_config.AntConfig,
	httpClient web.HTTPClient,
	opts *types.ExecutorOptions,
	id string,
) (err error) {
	var provider executor.Provider
	if provider, err = BuildProvider(ctx, antCfg, httpClient, opts); err != nil {
		return err
	}
	return provider.StopExecutor(ctx, id, opts)
}

// AllRunningContainers list containers by all providers
func AllRunningContainers(
	ctx context.Context,
	antCfg *ant_config.AntConfig) (res map[types.TaskMethod][]executor.Info) {
	res = make(map[types.TaskMethod][]executor.Info)
	if provider, err := shell.NewExecutorProvider(antCfg); err == nil {
		if containers, err := provider.AllRunningExecutors(ctx); err == nil {
			res[types.Shell] = containers
		}
	}

	if err := health.IsNetworkHostPortAlive(antCfg.Docker.Host, "docker"); err == nil {
		if provider, err := docker.NewExecutorProvider(antCfg); err == nil {
			if containers, err := provider.AllRunningExecutors(ctx); err == nil {
				res[types.Docker] = containers
			}
		}
	}

	if provider, err := kubernetes.NewExecutorProvider(antCfg); err == nil {
		if containers, err := provider.AllRunningExecutors(ctx); err == nil {
			res[types.Kubernetes] = containers
		}
	}

	return
}

// BuildProvider build provider based on method
func BuildProvider(
	_ context.Context,
	antCfg *ant_config.AntConfig,
	httpClient web.HTTPClient,
	opts *types.ExecutorOptions) (provider executor.Provider, err error) {

	// creating provider based on method
	if opts.Method == types.Shell {
		return shell.NewExecutorProvider(antCfg)
	} else if opts.Method == types.Docker {
		return docker.NewExecutorProvider(antCfg)
	} else if opts.Method == types.Kubernetes {
		return kubernetes.NewExecutorProvider(antCfg)
	} else if opts.Method.IsHTTP() {
		return http.NewExecutorProvider(antCfg, httpClient)
	} else {
		return nil, fmt.Errorf("unsupported method %s", opts.Method)
	}
}

// BuildExecutor create executor
func BuildExecutor(
	ctx context.Context,
	antCfg *ant_config.AntConfig,
	trace trace.JobTrace,
	httpClient web.HTTPClient,
	opts *types.ExecutorOptions) (exe executor.Executor, err error) {

	var provider executor.Provider
	if provider, err = BuildProvider(ctx, antCfg, httpClient, opts); err != nil {
		return nil, err
	}
	return provider.NewExecutor(ctx, trace, opts)
}
