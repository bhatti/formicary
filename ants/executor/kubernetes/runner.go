package kubernetes

import (
	"context"
	"fmt"
	"plexobject.com/formicary/internal/types"
	cutils "plexobject.com/formicary/internal/utils"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	api "k8s.io/api/core/v1"
	"plexobject.com/formicary/internal/async"

	"github.com/twinj/uuid"
	"plexobject.com/formicary/ants/executor"
)

type podPhaseError struct {
	name  string
	phase api.PodPhase
}

func (p *podPhaseError) Error() string {
	return fmt.Sprintf("Pod=%q Status=%q", p.name, p.phase)
}

// CommandRunner command runner for shell
type CommandRunner struct {
	executor.BaseCommandRunner
	exec          *Executor
	adapter       Adapter
	podName       string
	containerName string
	future        async.Awaiter
	cancel        context.CancelFunc
}

// NewCommandRunner constructor
func NewCommandRunner(
	exec *Executor,
	adapter Adapter,
	podName string,
	containerName string,
	cmd string,
	helper bool) (*CommandRunner, error) {
	if exec == nil {
		return nil, fmt.Errorf("executor not specified")
	}
	if podName == "" {
		if containerName == "" {
			return nil, fmt.Errorf("pod-name not specified, container %s", containerName)
		}
		logrus.WithFields(logrus.Fields{
			"Component": "KubernetesCommandRunner",
			"Container": containerName,
			"Memory":    cutils.MemUsageMiBString(),
		}).Info("pod-name not specified, using container-name")
		podName = containerName
		debug.PrintStack()
	}
	if containerName == "" {
		return nil, fmt.Errorf("container-name not specified")
	}
	base := executor.NewBaseCommandRunner(&exec.BaseExecutor, cmd, helper)
	base.ID = uuid.NewV4().String()
	return &CommandRunner{
		BaseCommandRunner: base,
		exec:              exec,
		adapter:           adapter,
		podName:           podName,
		containerName:     containerName,
	}, nil
}

// Await awaits for completion
func (kcr *CommandRunner) Await(ctx context.Context) (
	[]byte, []byte, error) {
	if kcr.future == nil {
		return nil, nil, fmt.Errorf("command '%s' is not running, call run()", kcr.Command)
	}
	ctx, kcr.cancel = context.WithCancel(ctx)
	_, err := kcr.future.Await(ctx)
	kcr.Err = err

	if kcr.ExecutorOptions.Debug || !kcr.IsHelper(ctx) {
		if len(kcr.Stdout.Bytes()) > 0 {
			_, _ = kcr.Trace.Write(kcr.Stdout.Bytes(), types.StdoutTags)
		}
		if len(kcr.Stderr.Bytes()) > 0 {
			_, _ = kcr.Trace.Write(kcr.Stderr.Bytes(), types.StderrTags)
		}
	}
	if err == nil {
		if kcr.ExecutorOptions.Debug || !kcr.IsHelper(ctx) {
			_ = kcr.BaseExecutor.WriteTraceSuccess(ctx,
				fmt.Sprintf("✅ %s succeeded Duration=%v",
					kcr.Command, kcr.BaseExecutor.Elapsed()))
		}
		logrus.WithFields(logrus.Fields{
			"Component": "KubernetesCommandRunner",
			"ID":        kcr.ID,
			"Name":      kcr.Name,
			"Container": kcr.containerName,
			"Command":   kcr.Command,
			"StdoutLen": len(kcr.Stdout.Bytes()),
			"Host":      kcr.Host,
			"IP":        kcr.ContainerIP,
			"Elapsed":   kcr.BaseExecutor.Elapsed(),
			"Memory":    cutils.MemUsageMiBString(),
		}).Info("succeeded in executing command")
	} else {
		tks := strings.Split(err.Error(), " ")
		kcr.ExitCode, _ = strconv.Atoi(tks[len(tks)-1])
		kcr.ExitMessage = fmt.Sprintf("command terminated with Message=%d", kcr.ExitCode)
		logrus.WithFields(logrus.Fields{
			"Component": "KubernetesCommandRunner",
			"ID":        kcr.ID,
			"Name":      kcr.Name,
			"Container": kcr.containerName,
			"Command":   kcr.Command,
			"StderrLen": len(kcr.Stderr.Bytes()),
			"Host":      kcr.Host,
			"IP":        kcr.ContainerIP,
			"ExitCode":  kcr.ExitCode,
			"Message":   kcr.ExitMessage,
			"Error":     err,
			"Elapsed":   kcr.BaseExecutor.Elapsed(),
			"Memory":    cutils.MemUsageMiBString(),
		}).Warn("failed to execute command in kubernetes")
		if kcr.ExecutorOptions.Debug || !kcr.IsHelper(ctx) {
			_ = kcr.BaseExecutor.WriteTraceError(ctx,
				fmt.Sprintf("❌ %s failed to execute Host=%s Exitcode=%d Error=%s Duration=%v",
					kcr.Command, kcr.ContainerIP, kcr.ExitCode, err, kcr.BaseExecutor.Elapsed()))
			if !kcr.DumpedRuntimeInfo && !kcr.IsHelper(ctx) {
				kcr.DumpedRuntimeInfo = true
				_, _ = kcr.Trace.Write([]byte("*********************** <<KUBERNETES RUNTIME-INFO BEGIN>> **************************"), types.DumpTags)
				_, _ = kcr.Trace.Write([]byte(kcr.exec.GetRuntimeInfo(ctx)), types.DumpTags)
				_, _ = kcr.Trace.Write([]byte("*********************** <<KUBERNETES RUNTIME-INFO END>>  **************************"), types.DumpTags)
			}
		}
	}
	return kcr.Stdout.Bytes(), kcr.Stderr.Bytes(), err
}

// Stop stop runner
func (kcr *CommandRunner) Stop(
	context.Context,
	time.Duration) error {
	if kcr.cancel != nil {
		kcr.cancel()
		return nil
	}
	return fmt.Errorf("cannot cancel command")
}

// IsRunning checks if runner is active
func (kcr *CommandRunner) IsRunning(context.Context) (bool, error) {
	if kcr.future == nil {
		return false, fmt.Errorf("command '%s' is not running, call run()", kcr.Command)
	}
	return kcr.future.IsRunning(), nil
}

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (kcr *CommandRunner) run(
	ctx context.Context) error {
	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		if kcr.ExecutorOptions.Debug || !kcr.IsHelper(ctx) {
			_ = kcr.BaseExecutor.WriteTrace(ctx, fmt.Sprintf("🔄 $ %s",
				kcr.Command))
		}
		pod, err := kcr.adapter.Execute(
			ctx,
			&kcr.BaseCommandRunner,
			kcr.podName,
			kcr.containerName,
			kcr.Command,
			false,
			kcr.ExecutorOptions.ExecuteCommandWithoutShell)
		if err == nil {
			kcr.Host = pod.Status.HostIP
			kcr.ContainerIP = pod.Status.PodIP
			_ = kcr.ExecutorOptions.Environment.AddFromEnvCommand(kcr.Command)
		} else {
			//_ = kcr.BaseExecutor.WriteTraceError(
			//	fmt.Sprintf("❌ %s failed to run! Pod=%s Error=%s",
			//		kcr.Command, kcr.containerName, err.Error()))
		}
		return pod, err
	}
	//abort := func(ctx context.Context, payload interface{}) (interface{}, error) {
	//	return nil, kcr.adapter.Stop(ctx, kcr.podName)
	//}
	errorHandler := func(ctx context.Context, payload interface{}) error {
		status, err := kcr.adapter.GetPodPhase(ctx, kcr.podName)
		if err != nil {
			return fmt.Errorf("failed to check pod phase before executing command '%s' for pod '%s' ['%s']  due to '%s'",
				kcr.Command, kcr.podName, kcr.containerName, err)
		}

		if status.phase != api.PodRunning {
			return &podPhaseError{
				name:  kcr.podName,
				phase: status.phase,
			}
		}
		return nil
	}

	kcr.future = async.ExecuteWatchdog(
		ctx,
		handler,
		errorHandler,
		async.NoAbort,
		0,
		kcr.AntConfig.GetPollInterval())
	return nil
}
