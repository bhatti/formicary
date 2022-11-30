package shell

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"plexobject.com/formicary/internal/utils/trace"
	"strings"
	"sync"
	"time"

	"github.com/twinj/uuid"
	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/internal/types"
)

// Executor for shell
type Executor struct {
	executor.BaseExecutor
	runners map[string]*CommandRunner
	lock    sync.RWMutex
}

// NewShellExecutor for creating shell executor
func NewShellExecutor(
	ctx context.Context,
	cfg *config.AntConfig,
	trace trace.JobTrace,
	opts *types.ExecutorOptions) (*Executor, error) {
	base, err := executor.NewBaseExecutor(cfg, trace, opts)
	if err != nil {
		return nil, err
	}
	base.ID = uuid.NewV4().String()
	base.Name = opts.Name

	hostName, _ := os.Hostname()
	_ = base.WriteTrace(ctx, fmt.Sprintf(
		"[%s SHELL %s] 🔥 running with formicary %s on %s",
		time.Now().Format(time.RFC3339), opts.Name, cfg.ID, hostName))
	_ = base.WriteTraceInfo(ctx, fmt.Sprintf("[%s SHELL %s] 🌱 preparing shell executor",
		time.Now().Format(time.RFC3339), opts.Name))

	return &Executor{
		BaseExecutor: base,
		runners:      make(map[string]*CommandRunner),
	}, nil
}

// GetRuntimeInfo for getting runtime info for shell executor
func (se *Executor) GetRuntimeInfo(
	context.Context) string {
	var buf bytes.Buffer
	se.lock.RLock()
	defer se.lock.RUnlock()
	buf.WriteString(fmt.Sprintf("Shell ID=%s Name=%s Runners=%d",
		se.ID, se.Name, len(se.runners)))
	for _, r := range se.runners {
		buf.WriteString(fmt.Sprintf("🔄 $ %s PID=%d\n",
			r.cmd, r.pid))
	}
	return buf.String()
}

// AsyncHelperExecute for executing by shell executor on helper container
func (se *Executor) AsyncHelperExecute(
	ctx context.Context,
	cmd string,
	_ map[string]types.VariableValue,
) (executor.CommandRunner, error) {
	return se.doAsyncExecute(ctx, cmd, true)
}

// AsyncExecute for executing by shell executor
func (se *Executor) AsyncExecute(
	ctx context.Context,
	cmd string,
	_ map[string]types.VariableValue,
) (executor.CommandRunner, error) {
	return se.doAsyncExecute(ctx, cmd, false)
}

func (se *Executor) doAsyncExecute(ctx context.Context, cmd string, helper bool) (executor.CommandRunner, error) {
	se.lock.Lock()
	defer se.lock.Unlock()
	if se.State == executor.Removing {
		_ = se.WriteTraceError(ctx, fmt.Sprintf("❌ failed to execute '%s' because container is already stopped", cmd))
		return nil, fmt.Errorf("failed to execute '%s' because container is already stopped", cmd)
	}
	se.State = executor.Running
	r, err := NewCommandRunner(&se.BaseExecutor, cmd, helper)
	if err != nil {
		return nil, err
	}
	if err = r.run(ctx); err != nil {
		return nil, err
	}
	se.runners[r.ID] = r
	return r, nil
}

// Stop stopping execution by shell executor
func (se *Executor) Stop(ctx context.Context,
) error {
	se.lock.Lock()
	defer se.lock.Unlock()
	if se.State == executor.Removing {
		_ = se.WriteTraceError(ctx, fmt.Sprintf("⛔ cannot remove executor as it's already stopped"))
		return fmt.Errorf("executor [%s] is already stopped", se.Name)
	}
	started := time.Now()
	se.State = executor.Removing
	now := time.Now()
	se.EndedAt = &now
	var err error
	_ = se.WriteTrace(ctx, fmt.Sprintf("✋ stopping runners=%d",
		len(se.runners)))

	ctx, cancel := context.WithTimeout(context.Background(), se.AntConfig.GetShutdownTimeout())
	defer cancel()

	for _, r := range se.runners {
		rErr := r.Stop(ctx, se.AntConfig.GetShutdownTimeout())
		if rErr != nil && !strings.Contains(rErr.Error(), "process already finished") {
			err = rErr
		}
	}
	_ = se.BaseExecutor.WriteTraceInfo(ctx,
		fmt.Sprintf("🛑 stopped container: Error=%v Elapsed=%v, StopWait=%v",
			err, time.Since(started).String(), se.AntConfig.GetShutdownTimeout()))
	se.runners = make(map[string]*CommandRunner)
	return err
}
