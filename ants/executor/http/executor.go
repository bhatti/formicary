package http

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"plexobject.com/formicary/internal/ant_config"
	"plexobject.com/formicary/internal/utils/trace"
	"plexobject.com/formicary/internal/web"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/internal/types"
)

// Executor for HTTP
type Executor struct {
	executor.BaseExecutor
	client  web.HTTPClient
	runners map[string]*CommandRunner
	lock    sync.RWMutex
}

// NewHTTPExecutor for creating http executor
func NewHTTPExecutor(
	ctx context.Context,
	cfg *ant_config.AntConfig,
	trace trace.JobTrace,
	client web.HTTPClient,
	opts *types.ExecutorOptions) (*Executor, error) {
	base, err := executor.NewBaseExecutor(cfg, trace, opts)
	if err != nil {
		return nil, err
	}
	base.ID = ulid.Make().String()
	base.Name = opts.Name

	hostName, _ := os.Hostname()
	_ = base.WriteTrace(ctx, fmt.Sprintf(
		"✅ running with formicary %s on %s", cfg.Common.ID, hostName))
	_ = base.WriteTraceInfo(ctx, fmt.Sprintf("✅ preparing http executor"))

	return &Executor{
		BaseExecutor: base,
		client:       client,
		runners:      make(map[string]*CommandRunner),
	}, nil
}

func (h *Executor) GetConfigInfo() map[string]any {
	return make(map[string]any)
}

// GetRuntimeInfo for getting runtime info for http executor
func (h *Executor) GetRuntimeInfo(
	context.Context) string {
	var buf bytes.Buffer
	h.lock.RLock()
	defer h.lock.RUnlock()
	buf.WriteString(fmt.Sprintf("✅ HTTP ID=%s Name=%s Runners=%d", h.ID, h.Name, len(h.runners)))
	for _, r := range h.runners {
		buf.WriteString(fmt.Sprintf("✅ $ %s HTTPCode=%d\n", r.Command, r.ExitCode))
	}
	return buf.String()
}

// AsyncHelperExecute for executing by http executor on helper container
func (h *Executor) AsyncHelperExecute(
	ctx context.Context,
	cmd string,
	variables map[string]types.VariableValue,
) (executor.CommandRunner, error) {
	return h.doAsyncExecute(ctx, cmd, true, variables)
}

// AsyncExecute for executing by http executor
func (h *Executor) AsyncExecute(
	ctx context.Context,
	cmd string,
	variables map[string]types.VariableValue,
) (executor.CommandRunner, error) {
	return h.doAsyncExecute(ctx, cmd, false, variables)
}

// Stop stopping execution by http executor
func (h *Executor) Stop(ctx context.Context) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	if h.State == executor.Removing {
		_ = h.WriteTraceError(ctx, fmt.Sprintf("⛔ cannot remove executor as it's already stopped"))
		return fmt.Errorf("executor [%s] is already stopped", h.Name)
	}
	started := time.Now()
	h.State = executor.Removing
	now := time.Now()
	h.EndedAt = &now
	var err error
	_ = h.WriteTrace(ctx, fmt.Sprintf("✋ stopping runners=%d",
		len(h.runners)))

	ctx, cancel := context.WithTimeout(context.Background(), h.AntConfig.GetShutdownTimeout())
	defer cancel()

	for _, r := range h.runners {
		rErr := r.Stop(ctx, h.AntConfig.GetShutdownTimeout())
		if rErr != nil && !strings.Contains(rErr.Error(), "process already finished") {
			err = rErr
		}
	}
	_ = h.BaseExecutor.WriteTraceInfo(ctx,
		fmt.Sprintf("🛑 stopped http-container: Error=%v Elapsed=%v, StopWait=%v",
			err, time.Since(started).String(), h.AntConfig.GetShutdownTimeout()))
	h.runners = make(map[string]*CommandRunner)
	return err
}

func (h *Executor) doAsyncExecute(
	ctx context.Context,
	cmd string,
	helper bool,
	variables map[string]types.VariableValue) (executor.CommandRunner, error) {
	h.lock.Lock()
	defer h.lock.Unlock()
	if h.State == executor.Removing {
		_ = h.WriteTraceError(ctx, fmt.Sprintf("❌ failed to execute '%s' because container is already stopped", cmd))
		return nil, fmt.Errorf("failed to execute '%s' because container is already stopped", cmd)
	}
	h.State = executor.Running
	r, err := NewCommandRunner(ctx, h.client, &h.BaseExecutor, cmd, helper, variables)
	if err != nil {
		return nil, err
	}
	h.runners[r.ID] = r
	return r, nil
}
