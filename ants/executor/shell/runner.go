package shell

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	common "plexobject.com/formicary/internal/types"
	cutils "plexobject.com/formicary/internal/utils"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/oklog/ulid/v2"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/internal/async"
)

// CommandRunner command runner for shell
type CommandRunner struct {
	executor.BaseCommandRunner
	cancel context.CancelFunc
	cmd    *exec.Cmd
	pid    int
}

// NewCommandRunner constructor
func NewCommandRunner(
	e *executor.BaseExecutor,
	cmd string,
	helper bool) (*CommandRunner, error) {
	base := executor.NewBaseCommandRunner(e, cmd, helper)

	runner := CommandRunner{
		BaseCommandRunner: base,
		cmd:               exec.Command("/bin/sh", []string{"-c", cmd}...),
	}

	if e.ExecutorOptions.WorkingDirectory != "" {
		runner.cmd.Dir = e.ExecutorOptions.WorkingDirectory
	}
	if e.ExecutorOptions.Environment != nil {
		runner.cmd.Env = e.ExecutorOptions.Environment.AsArray()
	}
	// not supported on windows
	runner.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	runner.Host, _ = os.Hostname()
	runner.ID = ulid.Make().String()
	runner.cmd.Stdin = runner.Stdin
	runner.cmd.Stdout = &runner.Stdout
	runner.cmd.Stderr = &runner.Stderr
	return &runner, nil
}

func (scr *CommandRunner) run(ctx context.Context) error {
	if scr.ExecutorOptions.Debug || !scr.IsHelper(ctx) {
		_ = scr.BaseExecutor.WriteTrace(ctx,
			fmt.Sprintf("🔄 $ %s", scr.Command))
	}
	err := scr.cmd.Start()
	if err != nil {
		return err
	}
	scr.pid = scr.cmd.Process.Pid
	return nil
}

// Await - awaits for completion
func (scr *CommandRunner) Await(ctx context.Context) ([]byte, []byte, error) {
	ctx, scr.cancel = context.WithCancel(ctx)
	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		return nil, scr.cmd.Wait()
	}
	abort := func(ctx context.Context, payload interface{}) (interface{}, error) {
		return nil, scr.Stop(ctx, 0)
	}
	_, err := async.Execute(ctx, handler, abort, nil).Await(ctx)
	scr.Err = err
	if scr.ExecutorOptions.Debug || !scr.IsHelper(ctx) {
		if len(scr.Stdout.Bytes()) > 0 {
			_, _ = scr.Trace.Write(scr.Stdout.Bytes(), common.StdoutTags)
		}
		if len(scr.Stderr.Bytes()) > 0 {
			_, _ = scr.Trace.Write(scr.Stderr.Bytes(), common.StderrTags)
		}
	}
	//scr.Trace.Finish()
	if err == nil {
		logrus.WithFields(logrus.Fields{
			"Component": "ShellCommandRunner",
			"ID":        scr.ID,
			"Name":      scr.Name,
			"StdoutLen": len(scr.Stdout.Bytes()),
			"Command":   scr.Command,
			"Host":      scr.Host,
			"IP":        scr.ContainerIP,
			"Elapsed":   scr.BaseExecutor.Elapsed(),
			"Memory":    cutils.MemUsageMiBString(),
		}).Info("succeeded in executing command")
		if scr.ExecutorOptions.Debug || !scr.IsHelper(ctx) {
			_ = scr.BaseExecutor.WriteTraceSuccess(ctx, fmt.Sprintf(
				"✅ %s on Host=%s Duration=%v",
				scr.Command, scr.Host, scr.BaseExecutor.Elapsed()))
		}
	} else {
		tks := strings.Split(err.Error(), " ")
		scr.ExitCode, _ = strconv.Atoi(tks[len(tks)-1])
		scr.ExitMessage = fmt.Sprintf("command terminated with Message=%d", scr.ExitCode)
		logrus.WithFields(logrus.Fields{
			"Component": "ShellCommandRunner",
			"ID":        scr.ID,
			"Name":      scr.Name,
			"StderrLen": len(scr.Stderr.Bytes()),
			"Command":   scr.Command,
			"Host":      scr.Host,
			"IP":        scr.ContainerIP,
			"Message":   scr.ExitCode,
			"Error":     err,
			"Elapsed":   scr.BaseExecutor.Elapsed(),
			"Memory":    cutils.MemUsageMiBString(),
		}).Warn("failed to execute command")
		if scr.ExecutorOptions.Debug || !scr.IsHelper(ctx) {
			_ = scr.BaseExecutor.WriteTraceError(ctx, fmt.Sprintf(
				"❌ %s failed to execute on Host=%s Message=%d Error=%s Duration=%v",
				scr.Command, scr.Host, scr.ExitCode, err, scr.BaseExecutor.Elapsed()))
		}
	}
	return scr.Stdout.Bytes(), scr.Stderr.Bytes(), err
}

// Stop - stops runner
func (scr *CommandRunner) Stop(context.Context, time.Duration) error {
	if scr.cancel != nil {
		scr.cancel()
	}
	return scr.cmd.Process.Kill()
}

// IsRunning checks if runner is active
func (scr *CommandRunner) IsRunning(context.Context) (bool, error) {
	_, err := os.FindProcess(scr.pid)
	return err != nil, nil
}
