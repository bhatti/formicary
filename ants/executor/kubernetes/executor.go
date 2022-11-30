package kubernetes

import (
	"context"
	"fmt"
	"os"
	"plexobject.com/formicary/internal/utils/trace"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	api "k8s.io/api/core/v1"

	"github.com/twinj/uuid"
	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/internal/types"
)

// Executor - executor for kubernetes
type Executor struct {
	executor.BaseExecutor
	lock                sync.RWMutex
	adapter             Adapter
	pod                 *api.Pod
	registryCredentials *api.Secret
	services            []api.Service // TODO add proxy services
	serviceNames        []string
}

// NewKubernetesExecutor - creating kubernetes executor
// &backoff.Backoff{Min: time.Second, Max: 30 * time.Second},
func NewKubernetesExecutor(
	ctx context.Context,
	cfg *config.AntConfig,
	trace trace.JobTrace,
	adapter Adapter,
	registryCredentials *api.Secret,
	opts *types.ExecutorOptions) (*Executor, error) {
	base, err := executor.NewBaseExecutor(cfg, trace, opts)
	if err != nil {
		return nil, err
	}
	base.ID = uuid.NewV4().String()
	base.Name = opts.Name
	hostName, _ := os.Hostname()
	_ = base.WriteTrace(ctx, fmt.Sprintf(
		"🔥 running with formicary %s on %s", cfg.ID, hostName))
	_ = base.WriteTraceInfo(ctx, fmt.Sprintf(
		"🐳 preparing kubernetes container '%s' with image '%s'",
		opts.Name, opts.MainContainer.Image))
	return &Executor{
		BaseExecutor:        base,
		adapter:             adapter,
		registryCredentials: registryCredentials,
		services:            make([]api.Service, 0),
	}, nil
}

// GetRuntimeInfo - runtime info for kubernetes executor
func (ke *Executor) GetRuntimeInfo(ctx context.Context) string {
	ke.lock.RLock()
	defer ke.lock.RUnlock()
	var podName string
	if ke.pod != nil {
		podName = ke.pod.Name
	} else {
		podName = "unknown"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s %s %s] Name=%s Image=%s HelperImage=%s Labels=%s ServiceNames=%v MainRuntime=%v",
		ke.AntConfig.Kubernetes.Namespace,
		ke.BaseExecutor.ID,
		ke.BaseExecutor.Name,
		ke.BaseExecutor.ExecutorOptions.Name,
		ke.BaseExecutor.ExecutorOptions.MainContainer.Image,
		ke.BaseExecutor.ExecutorOptions.HelperContainer.Image,
		ke.BaseExecutor.ExecutorOptions.PodLabels,
		ke.serviceNames,
		ke.adapter.GetRuntimeInfo(ctx, podName)))
	if ke.pod != nil {
		for _, s := range ke.serviceNames {
			if events, err := ke.adapter.GetEvents(
				ctx, ke.pod.Namespace, ke.pod.Name, ke.pod.ResourceVersion, nil); err == nil {
				sb.WriteString(fmt.Sprintf(" Service %s=%s",
					s, events))
			}
		}
	}
	return sb.String()
}

// AsyncHelperExecute for executing by shell executor on helper container
func (ke *Executor) AsyncHelperExecute(
	ctx context.Context,
	cmd string,
	variables map[string]types.VariableValue,
) (executor.CommandRunner, error) {
	return ke.doAsyncExecute(ctx, ke.BaseExecutor.GetHelperName(), cmd, true, variables)
}

// AsyncExecute - executing command by kubernetes executor
func (ke *Executor) AsyncExecute(
	ctx context.Context,
	cmd string,
	variables map[string]types.VariableValue,
) (executor.CommandRunner, error) {
	return ke.doAsyncExecute(ctx, ke.BaseExecutor.Name, cmd, false, variables)
}

// Stop - stop executing command by kubernetes executor
func (ke *Executor) Stop(
	ctx context.Context,
) error {
	ke.lock.Lock()
	defer ke.lock.Unlock()
	if ke.pod == nil {
		return fmt.Errorf("no pod is running")
	}
	started := time.Now()

	if ke.State == executor.Removing {
		_ = ke.WriteTrace(ctx,
			fmt.Sprintf("cannot remove container as it's already stopped"))
		return fmt.Errorf("container [%s %s] is already stopped", ke.pod.UID, ke.Name)
	}
	_ = ke.BaseExecutor.WriteTraceInfo(ctx, fmt.Sprintf("✋ stopping container"))

	cancelCtx, cancel := context.WithTimeout(context.Background(), ke.AntConfig.GetShutdownTimeout())
	defer cancel()
	ke.State = executor.Removing
	now := time.Now()
	ke.EndedAt = &now
	err := ke.adapter.Stop(cancelCtx, ke.pod.Name)
	errors := ke.adapter.Dispose(
		cancelCtx,
		ke.pod.Namespace,
		ke.services,
		nil,
		nil,
		ke.AntConfig.GetShutdownTimeout())
	if err != nil {
		errors = append(errors, err)
	}

	if err != nil {
		_ = ke.BaseExecutor.WriteTraceInfo(ctx,
			fmt.Sprintf("🛑 failed to stop container: Error=%v Elapsed=%v, StopWait=%v",
				err, time.Since(started).String(), ke.AntConfig.GetShutdownTimeout()))
	} else {
		_ = ke.BaseExecutor.WriteTraceInfo(ctx,
			fmt.Sprintf("🛑 stopped container: Errors=%v Elapsed=%v, StopWait=%v",
				len(errors), time.Since(started).String(), ke.AntConfig.GetShutdownTimeout()))
	}

	for _, err := range errors {
		_ = ke.BaseExecutor.WriteTrace(ctx, fmt.Sprintf("dispose failed %v", err.Error()))
	}
	if err == nil && ke.AntConfig.Kubernetes.AwaitShutdownPod {
		_ = ke.BaseExecutor.WriteTrace(ctx, fmt.Sprintf("awaiting for container to stop"))
		if _, err = ke.adapter.AwaitPodTerminating(
			cancelCtx,
			ke.Trace,
			ke.pod.Name,
			ke.AntConfig.GetShutdownTimeout(),
			ke.AntConfig.GetPollInterval(),
		); err == nil {
			_ = ke.BaseExecutor.WriteTrace(ctx,
				fmt.Sprintf("🛑 done waiting for container to stop Error=%v Elapsed %v",
					err, time.Since(started).String()))
		}
	}
	if err != nil {
		_ = ke.BaseExecutor.WriteTrace(ctx,
			fmt.Sprintf("⛔ failed waiting for container to stop, Error=%v Elapsed=%v",
				err, time.Since(started).String()))
		logrus.WithFields(logrus.Fields{
			"Component": "KubernetesExecutor",
			"Elapsed":   time.Since(started),
			"Error":     err}).Error("failed waiting for container to stop")
	}
	return err
}

// Elapsed time
func (ke *Executor) Elapsed() string {
	return ""
}

const maxBuildPodTries = 10

// ///////////////////////////////////////// PRIVATE METHODS ///////////////////////////////////////////
// fmt.Sprintf("%s 2>&1 | tee -a %s", cmd, logFile)
func (ke *Executor) ensurePodsConfigured() (err error) {
	if ke.pod != nil {
		return nil
	}
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), ke.AntConfig.GetAwaitRunningPeriod())
	defer cancel()

	// TODO setup config-map
	initContainers := ke.AntConfig.Kubernetes.GetInitContainers()
	// retry build pod if we can
	var aliases []string
	for i := 0; i < maxBuildPodTries; i++ {
		ke.pod, ke.serviceNames, aliases, ke.ExecutorOptions.CostFactor, err = ke.adapter.BuildPod(
			ctx,
			ke.ExecutorOptions,
			initContainers,
			ke.registryCredentials)
		if err == nil {
			if i > 0 {
				logrus.WithFields(logrus.Fields{
					"Component":    "KubernetesExecutor",
					"Elapsed":      time.Since(started),
					"I":            i,
					"ExecutorOpts": ke.ExecutorOptions}).
					Infof("succeeded to create pod after failure '%s'", ke.ExecutorOptions.Name)
			}
			break
		}
		if i == maxBuildPodTries-1 || !strings.Contains(err.Error(), "try again") {
			return fmt.Errorf("setting up failed for pod due to %w (%s), tries %d", err, ke.ExecutorOptions.Name, i+1)
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	_, _ = ke.Trace.Writeln(fmt.Sprintf("[%s KUBERNETES %s] 🐳 creating pod Image=%s Containers=%d Services=%v Aliases=%v Privileged=%v/%v Cost=%v",
		time.Now().Format(time.RFC3339),
		ke.pod.Name,
		ke.ExecutorOptions.MainContainer.Image,
		len(ke.pod.Spec.Containers),
		ke.serviceNames,
		aliases,
		ke.ExecutorOptions.Privileged,
		ke.AntConfig.Kubernetes.AllowPrivilegeEscalation,
		ke.ExecutorOptions.CostFactor), types.ExecTags)

	var status PodPhaseResponse
	status, err = ke.adapter.AwaitPodRunning(
		ctx,
		ke.Trace,
		ke.pod.Name,
		ke.AntConfig.GetPollTimeout(),
		ke.AntConfig.GetPollInterval(),
	)
	if err != nil {
		_, _ = ke.Trace.Writeln(fmt.Sprintf("[%s KUBERNETES %s] ⛔ failed to create pod Image=%s Error=%v Elapsed=%s",
			time.Now().Format(time.RFC3339), ke.pod.Name, ke.ExecutorOptions.MainContainer.Image, err, time.Since(started)), types.ExecTags)

		return fmt.Errorf("waiting for pod running: %w, AwaitRunningPeriod=%v, Timeout=%v, Elapsed=%s",
			err, ke.AntConfig.GetAwaitRunningPeriod(), ke.AntConfig.GetPollTimeout(), time.Since(started))
	}

	if status.phase != api.PodRunning {
		_, _ = ke.Trace.Writeln(fmt.Sprintf("[%s KUBERNETES %s] ⛔ failed to enter running status pod Image=%s Status=%v Elapsed=%s",
			time.Now().Format(time.RFC3339), ke.pod.Name, ke.ExecutorOptions.MainContainer.Image, status, time.Since(started)), types.ExecTags)

		return fmt.Errorf("pod failed to enter running State=%v Elapsed=%s", status, time.Since(started))
	}

	return nil
}

// doAsyncExecute - executing command by kubernetes executor
func (ke *Executor) doAsyncExecute(
	ctx context.Context,
	containerName string,
	cmd string,
	helper bool,
	_ map[string]types.VariableValue) (executor.CommandRunner, error) {
	ke.lock.Lock()
	defer ke.lock.Unlock()

	if ke.State == executor.Removing {
		_ = ke.WriteTraceError(ctx, fmt.Sprintf("❌ failed to execute '%s' because container is already stopped", cmd))
		return nil, fmt.Errorf("failed to execute '%s' because container is already stopped", cmd)
	}

	if len(ke.ExecutorOptions.MainContainer.ImageDefinition.Entrypoint) == 0 {
		ke.ExecutorOptions.MainContainer.ImageDefinition.Entrypoint = ke.AntConfig.DefaultShell
	}
	if len(ke.ExecutorOptions.HelperContainer.ImageDefinition.Entrypoint) == 0 {
		ke.ExecutorOptions.HelperContainer.ImageDefinition.Entrypoint = ke.AntConfig.DefaultShell
	}

	err := ke.ensurePodsConfigured()
	if err != nil {
		ke.State = executor.ContainerFailed
		return nil, err
	}
	ke.Name = ke.pod.Name
	ke.State = executor.Running
	runner, err := NewCommandRunner(
		ke,
		ke.adapter,
		ke.pod.Name,
		containerName,
		cmd,
		helper)
	if err != nil {
		return nil, err
	}
	return runner, runner.run(ctx)
}
