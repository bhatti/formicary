package docker

import (
	"context"
	"fmt"
	"os"
	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/utils/trace"
	"sync"
	"time"
)

const helperSuffix = "-helper"

// Executor for docker
type Executor struct {
	executor.BaseExecutor
	adapter  Adapter
	lock     sync.RWMutex
	helperID string
}

// NewDockerExecutor creates new docker executor
func NewDockerExecutor(
	ctx context.Context,
	cfg *config.AntConfig,
	trace trace.JobTrace,
	opts *types.ExecutorOptions,
	adapter Adapter) (exec *Executor, err error) {
	base, err := executor.NewBaseExecutor(cfg, trace, opts)
	if err != nil {
		return nil, err
	}
	exec = &Executor{
		BaseExecutor: base,
		adapter:      adapter,
	}
	if opts.MainContainer.Image == "" {
		return nil, fmt.Errorf("image not specified")
	}

	// create helper container
	if opts.HelperContainer.Image != "" {
		exec.helperID, err = adapter.Build(
			ctx,
			opts,
			opts.Name+helperSuffix,
			opts.HelperContainer.Image,
			cfg.DefaultShell,
			true)
		if err != nil {
			return nil, err
		}
	}

	// creating main container
	exec.ID, err = adapter.Build(
		ctx,
		opts,
		opts.Name,
		opts.MainContainer.Image,
		cfg.DefaultShell,
		false)
	if err != nil {
		if exec.helperID != "" {
			_ = adapter.Stop(
				ctx,
				exec.helperID,
				opts,
				cfg.GetShutdownTimeout()) // TODO check timeout
		}
		return nil, err
	}

	exec.Name = opts.Name

	hostName, _ := os.Hostname()
	_ = base.WriteTrace(ctx, fmt.Sprintf(
		"[%s DOCKER %s] 🔥 running with formicary %s on %s",
		time.Now().Format(time.RFC3339), opts.Name, cfg.Common.ID, hostName))
	_ = exec.WriteTraceInfo(ctx, fmt.Sprintf(
		"[%s DOCKER %s] 🐳 preparing docker container with image %s",
		time.Now().Format(time.RFC3339), opts.Name, opts.MainContainer.Image))
	return
}

// GetRuntimeInfo - runtime info by docker executor
func (de *Executor) GetRuntimeInfo(ctx context.Context) string {
	de.lock.RLock()
	defer de.lock.RUnlock()
	return fmt.Sprintf("[%s] container ID=%s Image=%s Helper=%s HelperImage=%s Labels=%s\n%v",
		de.BaseExecutor.ExecutorOptions.Name,
		de.ID,
		de.BaseExecutor.ExecutorOptions.MainContainer.Image,
		de.helperID,
		de.BaseExecutor.ExecutorOptions.HelperContainer.Image,
		de.BaseExecutor.ExecutorOptions.PodLabels,
		de.adapter.GetRuntimeInfo(ctx, de.Name),
	)
}

// AsyncHelperExecute for executing by shell executor on helper container
func (de *Executor) AsyncHelperExecute(
	ctx context.Context,
	cmd string,
	variables map[string]types.VariableValue,
) (executor.CommandRunner, error) {
	return de.doAsyncExecute(ctx, de.helperID, cmd, true, variables)
}

// AsyncExecute - executing command by docker executor
func (de *Executor) AsyncExecute(
	ctx context.Context,
	cmd string,
	variables map[string]types.VariableValue,
) (executor.CommandRunner, error) {
	return de.doAsyncExecute(ctx, de.ID, cmd, false, variables)
}

// Stop - stop executing command by docker executor
func (de *Executor) Stop(ctx context.Context) error {
	de.lock.Lock()
	defer de.lock.Unlock()
	if de.State == executor.Removing {
		_ = de.WriteTraceError(ctx, fmt.Sprintf("⛔ cannot remove container as it's already stopped"))
		return fmt.Errorf("container [%s %s] is already stopped", de.ID, de.Name)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = de.BaseExecutor.WriteTraceInfo(ctx, fmt.Sprintf("✋ stopping container"))

	// stopping main and helper container
	err := de.adapter.Stop(
		ctx,
		de.ID,
		de.ExecutorOptions,
		de.AntConfig.GetShutdownTimeout())

	if de.helperID != "" {
		_ = de.adapter.Stop(
			ctx,
			de.helperID,
			de.ExecutorOptions,
			de.AntConfig.GetShutdownTimeout())
	}
	now := time.Now()
	de.EndedAt = &now
	de.State = executor.Removing
	if err != nil {
		_ = de.WriteTraceError(ctx, fmt.Sprintf("⛔ failed to stop container: Error=%v Elapsed=%v, StopWait=%v",
			err, de.Elapsed(), de.AntConfig.GetShutdownTimeout()))
	} else {
		_ = de.WriteTraceInfo(ctx, fmt.Sprintf("🛑 stopped container: Elapsed=%v, StopWait=%v",
			de.Elapsed(), de.AntConfig.GetShutdownTimeout()))
	}
	return err
}

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
// doAsyncExecute - executing command by docker executor
func (de *Executor) doAsyncExecute(
	ctx context.Context,
	containerName string,
	cmd string,
	helper bool,
	_ map[string]types.VariableValue,
) (executor.CommandRunner, error) {
	de.lock.Lock()
	defer de.lock.Unlock()
	if de.State == executor.Removing {
		_ = de.WriteTraceError(ctx, fmt.Sprintf("❌ failed to execute '%s' because container is already stopped", cmd))
		return nil, fmt.Errorf("failed to execute '%s' because container is already stopped", cmd)
	}
	de.State = executor.Running
	runner, err := NewCommandRunner(de, de.adapter, containerName, cmd, helper)
	if err != nil {
		return nil, err
	}
	return runner, runner.run(ctx, helper)
}
