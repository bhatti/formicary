package docker

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/docker/docker/api/types"
	"plexobject.com/formicary/internal/async"

	"plexobject.com/formicary/ants/executor"
)

// CommandRunner command runner for shell
type CommandRunner struct {
	executor.BaseCommandRunner
	exec          *Executor
	adapter       Adapter
	hijack        types.HijackedResponse
	cancel        context.CancelFunc
	containerName string
}

// NewCommandRunner constructor
func NewCommandRunner(
	exec *Executor,
	adapter Adapter,
	containerName string,
	cmd string,
	helper bool) (*CommandRunner, error) {
	base := executor.NewBaseCommandRunner(&exec.BaseExecutor, cmd, helper)
	runner := &CommandRunner{
		BaseCommandRunner: base,
		exec:              exec,
		adapter:           adapter,
		containerName:     containerName,
	}
	return runner, nil
}

// Await waits for completion
func (dcr *CommandRunner) Await(ctx context.Context) ([]byte, []byte, error) {
	ctx, dcr.cancel = context.WithCancel(ctx)
	defer dcr.hijack.Close()
	abort := func(ctx context.Context, payload interface{}) (interface{}, error) {
		return nil, nil
	}
	completed := func(ctx context.Context, payload interface{}) (bool, interface{}, error) {
		running, err := dcr.IsRunning(ctx)
		if err != nil {
			return true, false, err
		}
		return !running, running, nil
	}
	future := async.ExecutePolling(ctx, completed, abort, 0, 1*time.Second)
	_, err := future.Await(ctx)
	dcr.Err = err
	//dcr.Trace.Finish()
	if dcr.ExecutorOptions.Debug || !dcr.IsHelper(ctx) {
		_, _ = dcr.Trace.Write(dcr.Stdout.Bytes())
		if len(dcr.Stderr.Bytes()) > 0 {
			_, _ = dcr.Trace.Write(dcr.Stderr.Bytes())
		}
	}

	if err == nil && dcr.ExitCode == 0 {
		logrus.WithFields(logrus.Fields{
			"Component": "DockerCommandRunner",
			"Command":   dcr.Command,
			"Container": dcr.containerName,
			"ID":        dcr.ID,
			"Name":      dcr.Name,
			"Host":      dcr.Host,
			"IP":        dcr.ContainerIP,
			"Elapsed":   dcr.BaseExecutor.Elapsed(),
		}).Info("succeeded in executing command")
		_ = dcr.BaseExecutor.WriteTraceSuccess(
			fmt.Sprintf("✅ %s Duration=%v",
				dcr.Command, dcr.BaseExecutor.Elapsed()))
	} else if err != nil || dcr.ExitCode != 0 {
		if err != nil {
			tks := strings.Split(err.Error(), " ")
			dcr.ExitCode, _ = strconv.Atoi(tks[len(tks)-1])
			dcr.ExitMessage = fmt.Sprintf("command terminated with Message=%d", dcr.ExitCode)
		}
		logrus.WithFields(logrus.Fields{
			"Component": "DockerCommandRunner",
			"Command":   dcr.Command,
			"Container": dcr.containerName,
			"ID":        dcr.ID,
			"Name":      dcr.Name,
			"Host":      dcr.Host,
			"IP":        dcr.ContainerIP,
			"Message":   dcr.ExitCode,
			"Error":     err,
			"Elapsed":   dcr.BaseExecutor.Elapsed(),
		}).Warn("failed to execute command")
		_ = dcr.BaseExecutor.WriteTraceError(
			fmt.Sprintf("❌ %s failed to execute Message=%s ExitCode=%d Host=%s Error=%v Duration=%v",
				dcr.Command, dcr.ExitMessage, dcr.ExitCode, dcr.ContainerIP, err, dcr.BaseExecutor.Elapsed()))
		if !dcr.DumpedRuntimeInfo {
			dcr.DumpedRuntimeInfo = true
			_, _ = dcr.Trace.Write([]byte("*********************** <<DOCKER RUNTIME-INFO BEGIN>> **************************"))
			_, _ = dcr.Trace.Write([]byte(dcr.exec.GetRuntimeInfo(ctx)))
			_, _ = dcr.Trace.Write([]byte("*********************** <<DOCKER RUNTIME-INFO END>>  **************************"))
		}
		if err == nil {
			err = fmt.Errorf("failed to execute command '%s' exit-code=%d", dcr.Command, dcr.ExitCode)
		}
	}
	return dcr.Stdout.Bytes(), dcr.Stderr.Bytes(), err
}

// Stop container
func (dcr *CommandRunner) Stop(
	context.Context,
	time.Duration) error {
	if dcr.cancel != nil {
		dcr.cancel()
		return nil
	}
	return fmt.Errorf("cannot cancel command")
}

// IsRunning returns true if container is running
func (dcr *CommandRunner) IsRunning(ctx context.Context) (bool, error) {
	running, exitCode, err := dcr.adapter.IsExecuteRunning(ctx, dcr.ID)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "DockerCommandRunner",
			"Name":      dcr.containerName,
			"ID":        dcr.ID,
			"IP":        dcr.ContainerIP,
			"Message":   exitCode,
			"Error":     err,
			"Running":   running,
		}).Debug("inspecting container...")
	}
	if err != nil {
		return false, err
	}
	dcr.ExitCode = exitCode
	return running, nil
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (dcr *CommandRunner) run(ctx context.Context) error {
	info, err := dcr.adapter.Execute(
		ctx,
		dcr.ExecutorOptions,
		dcr.containerName,
		dcr.Command,
		dcr.ExecutorOptions.ExecuteCommandWithoutShell)
	if err != nil {
		_ = dcr.BaseExecutor.WriteTraceError(fmt.Sprintf(
			"⛔ $ %s Error=%v",
			dcr.Command, err))
		return err
	}
	dcr.Host = info.HostName
	dcr.ContainerIP = info.IPAddress
	dcr.BaseCommandRunner.ID = info.ID
	_ = dcr.WriteTrace(fmt.Sprintf("🔄 $ %s", dcr.Command))
	dcr.hijack = info.Hijack
	go func() {
		_, _ = io.Copy(&dcr.Stdout, info.Hijack.Reader)
	}()
	return nil
}
