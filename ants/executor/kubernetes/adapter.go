package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"plexobject.com/formicary/internal/ant_config"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"plexobject.com/formicary/internal/utils"

	"plexobject.com/formicary/internal/utils/trace"

	"github.com/sirupsen/logrus"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/internal/async"
	domain "plexobject.com/formicary/internal/types"
)

// Adapter for kubernetes APIs
type Adapter interface {
	GetConfigInfo() map[string]any
	GetPod(
		ctx context.Context,
		name string) (*api.Pod, error)
	GetPodPhase(
		ctx context.Context,
		name string) (res PodPhaseResponse, err error)
	GetLogs(
		ctx context.Context,
		namespace string,
		name string,
		maxBytes int64) (io.ReadCloser, error)
	AwaitPodTerminating(
		ctx context.Context,
		trace trace.JobTrace,
		name string,
		timeout time.Duration,
		poll time.Duration) (PodPhaseResponse, error)
	AwaitPodRunning(
		ctx context.Context,
		trace trace.JobTrace,
		name string,
		timeout time.Duration,
		poll time.Duration) (PodPhaseResponse, error)
	List(ctx context.Context) ([]executor.Info, error)
	Stop(ctx context.Context, containerID string) error
	BuildRegistryCredentials(ctx context.Context) (*api.Secret, error)
	BuildPod(
		ctx context.Context,
		opts *domain.ExecutorOptions,
		initContainers []api.Container,
		credentials *api.Secret) (*api.Pod, []string, []string, float64, error)
	Execute(
		ctx context.Context,
		base *executor.BaseCommandRunner,
		podName string,
		containerName string,
		cmd string,
		useAttach bool,
		executeCommandWithoutShell bool) (*api.Pod, error)
	GetEvents(
		ctx context.Context,
		namespace string,
		name string,
		resourceVersion string,
		labels map[string]string,
	) (*api.EventList, error)
	GetRuntimeInfo(
		ctx context.Context,
		podName string) string
	Dispose(
		ctx context.Context,
		namespace string,
		services []api.Service,
		credentials *api.Secret,
		configMap *api.ConfigMap,
		timeout time.Duration) []error
}

// Utils defines structure for adapter
// https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
// https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/
// https://godoc.org/k8s.io/client-go/kubernetes#Clientset
type Utils struct {
	config     *ant_config.AntConfig
	restConfig *restclient.Config
	cli        *kubernetes.Clientset
}

// NewKubernetesUtils - creates new adapter for kubernetes
func NewKubernetesUtils(
	config *ant_config.AntConfig,
	cli *kubernetes.Clientset,
	restConfig *restclient.Config) (*Utils, error) {
	return &Utils{
		config:     config,
		restConfig: restConfig,
		cli:        cli,
	}, nil
}

// List -- list containers
// See https://godoc.org/k8s.io/client-go/kubernetes/typed/core/v1#PodsGetter
// https://godoc.org/k8s.io/client-go/kubernetes/typed/core/v1#PodInterface
// https://godoc.org/k8s.io/api/core/v1#Pod
func (u *Utils) List(
	ctx context.Context) ([]executor.Info, error) {
	pods, err := u.cli.CoreV1().Pods(u.config.Kubernetes.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers due to %w", err)
	}
	execs := make([]executor.Info, 0)

	for _, pod := range pods.Items {
		opts := domain.NewExecutorOptions("", "")
		opts.PodLabels = pod.Labels
		opts.PodAnnotations = pod.Annotations
		exec := executor.BaseExecutor{
			ExecutorOptions: opts,
			ID:              string(pod.UID),
			Name:            pod.Name,
			StartedAt:       pod.CreationTimestamp.Time,
			State:           executor.State(strings.ToUpper(string(pod.Status.Phase))),
			Labels:          pod.ObjectMeta.Labels,
			Annotations:     pod.ObjectMeta.Annotations,
		}
		execs = append(execs, &exec)
	}
	return execs, nil
}

// GetPod - finds pod
// https://godoc.org/k8s.io/client-go/kubernetes/typed/core/v1#PodsGetter
// See pod.Namespace, pod.Name
func (u *Utils) GetPod(
	ctx context.Context,
	name string) (*api.Pod, error) {
	return u.cli.CoreV1().Pods(u.config.Kubernetes.Namespace).Get(ctx, name, metav1.GetOptions{})
}

// PodPhaseResponse defines a structure to store pod state
type PodPhaseResponse struct {
	phase             api.PodPhase
	containerStatuses []api.ContainerStatus
	conditions        []api.PodCondition
	hostIP            string
	podIP             string
}

// GetPodPhase finds pod phase
func (u *Utils) GetPodPhase(
	ctx context.Context,
	name string) (res PodPhaseResponse, err error) {
	if name == "" {
		return PodPhaseResponse{}, fmt.Errorf("container name cannot be empty when checking pod phase")
	}
	pod, err := u.GetPod(ctx, name)
	if err != nil {
		return PodPhaseResponse{}, fmt.Errorf("failed to get pod-phase for '%s' due to %w", name, err)
	}
	return PodPhaseResponse{
		phase:             pod.Status.Phase,
		hostIP:            pod.Status.HostIP,
		podIP:             pod.Status.PodIP,
		containerStatuses: pod.Status.ContainerStatuses,
		conditions:        pod.Status.Conditions,
	}, nil
}

// AwaitPodRunning - waits for pod to be in running state
func (u *Utils) AwaitPodRunning(
	ctx context.Context,
	trace trace.JobTrace,
	name string,
	timeout time.Duration,
	poll time.Duration) (PodPhaseResponse, error) {
	if name == "" {
		return PodPhaseResponse{}, fmt.Errorf("container name cannot be empty when awaiting for running")
	}
	started := time.Now()
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout*time.Second)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()
	abort := func(ctx context.Context, payload interface{}) (interface{}, error) {
		//u.Stop(namespace, name)
		return nil, nil
	}
	var tried int32
	handler := func(ctx context.Context, payload interface{}) (bool, interface{}, error) {
		payload, err := u.GetPodPhase(ctx, name)
		if err != nil {
			return true, false, err
		}
		if ctx.Err() != nil {
			return true, false, ctx.Err()
		}
		res := payload.(PodPhaseResponse)
		if res.phase == api.PodRunning {
			_, _ = trace.Writeln(fmt.Sprintf("[%s KUBERNETES %s] ✅ pod-running ready with Status=%s",
				time.Now().Format(time.RFC3339), name, res.phase), domain.ExecTags)
			return true, res, nil
		} else if res.phase == api.PodSucceeded {
			return true, nil, fmt.Errorf("⛔ failed to wait for running state, pod %s is already succeeded", name)
		} else if res.phase == api.PodFailed {
			return true, nil, fmt.Errorf("⛔ failed to wait for running state, pod %s is already failed", name)
		}

		for _, container := range res.containerStatuses {
			if container.Ready {
				continue
			}
			if container.State.Waiting == nil {
				continue
			}

			switch container.State.Waiting.Reason {
			case "ErrImagePull", "ImagePullBackOff":
				err = fmt.Errorf("⛔ image pull failed: %s", container.State.Waiting.Message)
				_, _ = trace.Writeln(fmt.Sprintf("[%s %s %s] ⌛ waiting for pod-running failed with Status=%s Error=%v",
					time.Now().Format(time.RFC3339), u.config.Kubernetes.Namespace, name, res.phase, err), domain.ExecTags)
				return false, res, err
			}
		}
		for _, condition := range res.conditions {
			if condition.Reason == "" {
				continue
			}
			//_, _ = trace.Writeln(fmt.Sprintf("[%s %s %s] ⌛ waiting for pod-running failed with Condition=%s %s: %q",
			//	time.Now().Format(time.RFC3339), u.config.Kubernetes.Namespace, name, res.phase,
			//	condition.Reason, condition.Message))
		}
		_, _ = trace.Writeln(fmt.Sprintf("[%s KUBERNETES %s] ⌛ waiting for running state but status is still %s",
			time.Now().Format(time.RFC3339), name, res.phase), domain.ExecTags)
		if tried%60 == 0 {
			if pod, err := u.GetPod(ctx, name); err == nil {
				for _, c := range pod.Status.Conditions {
					_, _ = trace.Writeln(fmt.Sprintf("[%s KUBERNETES %s] 🎛️ pod-state %s %v %v message=%s reason=%s condition=%v",
						time.Now().Format(time.RFC3339), name, pod.Status.Phase, pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses, pod.Status.Message, pod.Status.Reason, c), domain.ExecTags)
				}
			} else if events, err := u.GetEvents(ctx, u.config.Kubernetes.Namespace, name, "", make(map[string]string)); err == nil {
				for _, ev := range events.Items {
					_, _ = trace.Writeln(fmt.Sprintf("[%s KUBERNETES %s] 🎛️ type=%s reason=%s from=%s message=%s action=%s",
						time.Now().Format(time.RFC3339), name, ev.Type, ev.Reason, ev.Source, ev.Message, ev.Action), domain.ExecTags)
				}
			}
		}
		atomic.AddInt32(&tried, 1)
		logrus.WithFields(logrus.Fields{
			"Component": "KubernetesAdapter",
			"POD":       name,
			"Phase":     res.phase,
			"Timeout":   timeout,
			"Namespace": u.config.Kubernetes.Namespace,
		}).Info("waiting for running...")
		return false, res, nil
	}

	future := async.ExecutePolling(ctx, handler, abort, 0, poll)
	res, err := future.Await(ctx)
	if err != nil {
		_, _ = trace.Writeln(fmt.Sprintf("[%s KUBERNETES %s] ⌛ waiting for running but status timeout %v elapsed %s",
			time.Now().Format(time.RFC3339), name, timeout, time.Since(started)), domain.ExecTags)
		return PodPhaseResponse{}, err
	}
	return res.(PodPhaseResponse), nil
}

// AwaitPodTerminating - waits for pod to terminate
func (u *Utils) AwaitPodTerminating(
	ctx context.Context,
	trace trace.JobTrace,
	name string,
	timeout time.Duration,
	poll time.Duration) (PodPhaseResponse, error) {
	if name == "" {
		return PodPhaseResponse{}, fmt.Errorf("container name cannot be empty when awaiting for termination")
	}
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout*time.Second)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()
	handler := func(ctx context.Context, payload interface{}) (bool, interface{}, error) {
		payload, err := u.GetPodPhase(ctx, name)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				err = nil
			}
			return true, PodPhaseResponse{}, err
		}
		if ctx.Err() != nil {
			return true, PodPhaseResponse{}, ctx.Err()
		}
		res := payload.(PodPhaseResponse)
		if res.phase != api.PodRunning {
			return true, res, nil
		}
		_, _ = trace.Writeln(fmt.Sprintf("[%s %s %s] ⌛ waiting for terminating but status is still %s",
			time.Now().Format(time.RFC3339), u.config.Kubernetes.Namespace, name, res.phase), domain.ExecTags)
		logrus.WithFields(logrus.Fields{
			"Component": "KubernetesAdapter",
			"POD":       name,
			"Phase":     res.phase,
			"Timeout":   timeout,
			"Namespace": u.config.Kubernetes.Namespace,
		}).Info("waiting for terminating...")
		return false, res, nil
	}

	future := async.ExecutePolling(ctx, handler, async.NoAbort, 0, poll)
	res, err := future.Await(ctx)
	if err != nil {
		return PodPhaseResponse{}, err
	}
	return res.(PodPhaseResponse), nil
}

// GetEvents - finds kubernetes events for pod
// https://godoc.org/k8s.io/client-go/kubernetes/typed/core/v1#EventExpansion
func (u *Utils) GetEvents(
	ctx context.Context,
	namespace string,
	name string,
	resourceVersion string,
	labels map[string]string,
) (*api.EventList, error) {
	opts := metav1.ListOptions{
		Watch:         false,
		FieldSelector: "metadata.name=" + name,
	}
	if resourceVersion != "" {
		opts.ResourceVersion = resourceVersion
	}
	if len(labels) > 0 {
		for k := range labels {
			opts.LabelSelector = k
			break
		}
	}

	return u.cli.CoreV1().Events(namespace).List(ctx, opts)
}

// BuildPod - builds pod definition
func (u *Utils) BuildPod(
	ctx context.Context,
	opts *domain.ExecutorOptions,
	initContainers []api.Container,
	credentials *api.Secret) (*api.Pod, []string, []string, float64, error) {
	if opts == nil {
		return nil, nil, nil, 0, fmt.Errorf("executor options not specified")
	}
	if opts.Name == "" {
		return nil, nil, nil, 0, fmt.Errorf("container name in executor options not specified")
	}
	if opts.MainContainer.Image == "" {
		return nil, nil, nil, 0, fmt.Errorf("image not specified")
	}

	// pod container specifications
	serviceNames := make([]string, 0)
	podServices := make([]api.Container, 0)
	volumes := u.config.Kubernetes.Volumes.GetVolumes()
	hostAliases, err := domain.CreateHostAliases(opts.Services, u.config.Kubernetes.GetHostAliases())
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to create host aliases due to %w", err)
	}
	aliasNames := make([]string, len(hostAliases))
	for i, a := range hostAliases {
		aliasNames[i] = fmt.Sprintf("h:%v, ip=%s;", a.Hostnames, a.IP)
	}

	privileged := opts.Privileged
	var totalCost float64
	for i, service := range opts.Services {
		if service.Name == "" {
			service.Name = fmt.Sprintf("svc-%d", i)
		}
		if service.Instances > u.config.Kubernetes.MaxServicesPerPod {
			service.Instances = u.config.Kubernetes.MaxServicesPerPod
		}
		if service.Instances < 1 {
			service.Instances = 1
		}
		baseSvcName := service.Name
		for j := 0; j < service.Instances; j++ {
			svcName := baseSvcName
			envMap := make(map[string]string)
			if service.Instances > 1 {
				svcName = fmt.Sprintf("%s-%d", baseSvcName, j)
				envMap["SERVICE_INSTANCE"] = strconv.Itoa(j)
				envMap[strings.ToUpper(baseSvcName)+"_SERVICE_INSTANCE"] = strconv.Itoa(j)
			}

			// Enhanced resource handling - use structured resources if available
			var serviceRequests, serviceLimits api.ResourceList
			var cost float64

			if u.hasStructuredServiceResources(service) {
				//serviceRequests, serviceLimits, cost, err = u.createStructuredServiceResources(svcName, service)
			} else {
				serviceRequests, cost, err = u.config.Kubernetes.CreateResourceList(
					"service-request-"+svcName,
					service.CPURequest,
					service.MemoryRequest,
					service.EphemeralStorageRequest)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"Component":    "KubernetesAdapter",
						"POD":          opts.Name,
						"ResourceType": u.getResourceConfigType(),
						"Options":      opts.String(),
						"CWD":          opts.WorkingDirectory,
						"Namespace":    u.config.Kubernetes.Namespace,
						"Services":     len(opts.Services),
						"Service":      service.Name,
						"Instances":    service.Instances,
					}).WithError(err).Errorf("failed to create service request for %s", svcName)
					return nil, nil, nil, 0, fmt.Errorf("failed to create service request for %s due to %w", svcName, err)
				}

				serviceLimits, _, err = u.config.Kubernetes.CreateResourceList(
					"service-limit-"+svcName,
					service.CPULimit,
					service.MemoryLimit,
					service.EphemeralStorageLimit)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"Component":    "KubernetesAdapter",
						"POD":          opts.Name,
						"ResourceType": u.getResourceConfigType(),
						"Options":      opts.String(),
						"CWD":          opts.WorkingDirectory,
						"Namespace":    u.config.Kubernetes.Namespace,
						"Services":     len(opts.Services),
						"Service":      service.Name,
						"Instances":    service.Instances,
					}).WithError(err).Errorf("failed to create service limits for %s", svcName)
					return nil, nil, nil, 0, fmt.Errorf("failed to create service limits for %s due to %w", svcName, err)
				}
			}
			totalCost += cost
			volumes = service.GetKubernetesVolumes().AddVolumes(volumes)
			volumeMounts := service.GetKubernetesVolumes().AddVolumeMounts(u.config.Kubernetes.Volumes.GetVolumeMounts())
			podService := buildContainer(
				&u.config.Kubernetes,
				svcName,
				service.WorkingDirectory,
				service.Image,
				service.ToImageDefinition(),
				serviceRequests,
				serviceLimits,
				volumeMounts,
				buildVariables(&u.config.Kubernetes, opts, false, envMap),
				privileged)
			podServices = append(podServices, podService)
			serviceNames = append(serviceNames, podService.Name)
		}
	}

	// Main Container
	{
		volumes = opts.MainContainer.GetKubernetesVolumes().AddVolumes(volumes)
		volumeMounts := opts.MainContainer.GetKubernetesVolumes().AddVolumeMounts(
			u.config.Kubernetes.Volumes.GetVolumeMounts())

		var requests, limits api.ResourceList
		var cost float64

		// Use enhanced resource creation
		if u.hasStructuredMainContainerResources(opts) {
			//requests, limits, cost, err = u.createStructuredMainContainerResources(opts)
		} else {
			requests, cost, err = u.config.Kubernetes.CreateResourceList(
				"request",
				opts.MainContainer.CPURequest,
				opts.MainContainer.MemoryRequest,
				opts.MainContainer.EphemeralStorageRequest)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":    "KubernetesAdapter",
					"POD":          opts.Name,
					"ResourceType": u.getResourceConfigType(),
					"Options":      opts.String(),
					"CWD":          opts.WorkingDirectory,
					"Namespace":    u.config.Kubernetes.Namespace,
					"Services":     len(opts.Services),
				}).WithError(err).Errorf("failed to create main resource for %s", opts.Name)
				return nil, nil, nil, 0, fmt.Errorf("failed to create main resource for %s due to %w", opts.Name, err)
			}

			limits, _, err = u.config.Kubernetes.CreateResourceList(
				"limit",
				opts.MainContainer.CPULimit,
				opts.MainContainer.MemoryLimit,
				opts.MainContainer.EphemeralStorageLimit)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":    "KubernetesAdapter",
					"POD":          opts.Name,
					"ResourceType": u.getResourceConfigType(),
					"Options":      opts.String(),
					"CWD":          opts.WorkingDirectory,
					"Namespace":    u.config.Kubernetes.Namespace,
					"Services":     len(opts.Services),
				}).WithError(err).Errorf("failed to create main limit for %s", opts.Name)
				return nil, nil, nil, 0, fmt.Errorf("failed to create main limit for %s due to %w", opts.Name, err)
			}
		}
		totalCost += cost
		mainContainer := buildContainer(
			&u.config.Kubernetes,
			opts.Name,
			opts.WorkingDirectory,
			opts.MainContainer.Image,
			opts.MainContainer.ImageDefinition,
			requests,
			limits,
			volumeMounts,
			buildVariables(&u.config.Kubernetes, opts, false, nil),
			privileged)
		podServices = append(podServices, mainContainer)
		//serviceNames = append(serviceNames, opts.Name)
	}

	if opts.HelperContainer.Image != "" {
		helperName := fmt.Sprintf("%s-helper", opts.Name)

		var helperRequests, helperLimits api.ResourceList
		var cost float64
		// Use enhanced resource creation for helper container
		if u.hasStructuredHelperResources(opts) {
			helperRequests, helperLimits, cost, err = u.createStructuredHelperResources(opts)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":    "KubernetesAdapter",
					"POD":          opts.Name,
					"ResourceType": u.getResourceConfigType(),
					"Options":      opts.String(),
					"CWD":          opts.WorkingDirectory,
					"Namespace":    u.config.Kubernetes.Namespace,
					"Services":     len(opts.Services),
				}).WithError(err).Errorf("failed to create helper structured resource for %s", opts.Name)
				return nil, nil, nil, 0, fmt.Errorf("failed to create structured helper resources for %s due to %w", helperName, err)
			}
		} else {
			// Legacy resource creation
			helperRequests, _, err = u.config.Kubernetes.CreateResourceList(
				"helper-request",
				opts.HelperContainer.CPURequest,
				opts.HelperContainer.MemoryRequest,
				opts.HelperContainer.EphemeralStorageRequest)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":    "KubernetesAdapter",
					"POD":          opts.Name,
					"ResourceType": u.getResourceConfigType(),
					"Options":      opts.String(),
					"CWD":          opts.WorkingDirectory,
					"Namespace":    u.config.Kubernetes.Namespace,
					"Services":     len(opts.Services),
				}).WithError(err).Errorf("failed to create helper resource for %s", opts.Name)
				return nil, nil, nil, 0, fmt.Errorf("failed to create helper resource for %s due to %w", helperName, err)
			}

			helperLimits, cost, err = u.config.Kubernetes.CreateResourceList(
				"helper-limit",
				opts.HelperContainer.CPULimit,
				opts.HelperContainer.MemoryLimit,
				opts.HelperContainer.EphemeralStorageLimit)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":    "KubernetesAdapter",
					"POD":          opts.Name,
					"ResourceType": u.getResourceConfigType(),
					"Options":      opts.String(),
					"CWD":          opts.WorkingDirectory,
					"Namespace":    u.config.Kubernetes.Namespace,
					"Services":     len(opts.Services),
				}).WithError(err).Errorf("failed to create helper limit for %s", opts.Name)
				return nil, nil, nil, 0, fmt.Errorf("failed to create helper limit for %s, cost %f due to %w", helperName, cost, err)
			}
		}
		// totalCost += cost // not needed for helper

		volumes = opts.HelperContainer.GetKubernetesVolumes().AddVolumes(volumes)
		volumeMounts := opts.HelperContainer.GetKubernetesVolumes().AddVolumeMounts(
			u.config.Kubernetes.Volumes.GetVolumeMounts())

		helperContainer := buildContainer(
			&u.config.Kubernetes,
			helperName,
			"",
			opts.HelperContainer.Image,
			opts.HelperContainer.ImageDefinition,
			helperRequests,
			helperLimits,
			volumeMounts,
			buildVariables(&u.config.Kubernetes, opts, true, nil),
			privileged)
		podServices = append(podServices, helperContainer)
		//serviceNames = append(serviceNames, helperName)
	}

	// We set a default label to the pod. This label will be used later
	// by the services, to link each service to the pod
	labels := opts.PodLabels

	annotations := opts.PodAnnotations

	var imagePullSecrets []api.LocalObjectReference
	for _, imagePullSecret := range u.config.Kubernetes.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, api.LocalObjectReference{Name: imagePullSecret})
	}

	if credentials != nil {
		imagePullSecrets = append(imagePullSecrets, api.LocalObjectReference{Name: credentials.Name})
	}

	nodeSelector := opts.NodeSelector
	tolerations := opts.NodeTolerations.Get()
	affinity := opts.GetAffinity()

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":     "KubernetesAdapter",
			"POD":           opts.Name,
			"Services":      serviceNames,
			"ServicesCount": len(podServices),
			"TotalCost":     totalCost,
			"ResourceType":  u.getResourceConfigType(),
			"Options":       opts.String(),
			"Labels":        labels,
			"Privileged":    privileged,
			"CWD":           opts.WorkingDirectory,
			"Annotations":   annotations,
			"NodeSelector":  nodeSelector,
			"Tolerations":   len(tolerations),
			"Affinity":      affinity != nil,
			"Namespace":     u.config.Kubernetes.Namespace,
		}).Debugf("creating pod with enhanced configuration...")
	}

	podConfig, err := preparePodConfig(
		u.config,
		opts.Name,
		podServices,
		labels,
		annotations,
		imagePullSecrets,
		hostAliases,
		nodeSelector,
		tolerations,
		affinity,
		volumes,
		opts.HostNetwork,
		initContainers)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":    "KubernetesAdapter",
			"POD":          podConfig.Name,
			"Options":      opts.String(),
			"Labels":       labels,
			"CWD":          opts.WorkingDirectory,
			"Annotations":  annotations,
			"Error":        err,
			"Namespace":    u.config.Kubernetes.Namespace,
			"Memory":       utils.MemUsageMiBString(),
			"ResourceType": u.getResourceConfigType(),
			"TotalCost":    totalCost,
		}).Warnf("failed to create pod config for %s", opts.Name)
		return nil, nil, nil, 0, fmt.Errorf("failed to create pod config for %s due to %w", opts.Name, err)
	}

	pod, err := u.cli.CoreV1().Pods(u.config.Kubernetes.Namespace).Create(ctx, podConfig, metav1.CreateOptions{})
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":    "KubernetesAdapter",
			"POD":          podConfig.Name,
			"Options":      opts.String(),
			"Labels":       labels,
			"CWD":          opts.WorkingDirectory,
			"Annotations":  annotations,
			"Error":        err,
			"Namespace":    u.config.Kubernetes.Namespace,
			"Memory":       utils.MemUsageMiBString(),
			"ResourceType": u.getResourceConfigType(),
			"TotalCost":    totalCost,
		}).Warnf("failed to create pod with enhanced configuration: %s", opts.Name)
	} else {
		logrus.WithFields(logrus.Fields{
			"Component":     "KubernetesAdapter",
			"POD":           podConfig.Name,
			"Options":       opts.String(),
			"Labels":        labels,
			"Services":      serviceNames,
			"ServicesCount": len(podServices),
			"CWD":           opts.WorkingDirectory,
			"Annotations":   annotations,
			"Namespace":     u.config.Kubernetes.Namespace,
			"Memory":        utils.MemUsageMiBString(),
			"ResourceType":  u.getResourceConfigType(),
			"TotalCost":     totalCost,
		}).Infof("created pod with enhanced configuration: %s", opts.Name)
	}

	return pod, serviceNames, aliasNames, totalCost, err
}

// BuildRegistryCredentials - stores docker credentials as a secret
func (u *Utils) BuildRegistryCredentials(ctx context.Context) (*api.Secret, error) {
	authConfigs := make(map[string]interface{})
	logrus.WithFields(logrus.Fields{
		"Component": "KubernetesAdapter",
		"Username":  u.config.Kubernetes.Username,
	}).Info("adding registry secret")
	authConfigs[u.config.Kubernetes.Registry.Server] = map[string]string{
		"Username": u.config.Kubernetes.Username, "Password": u.config.Kubernetes.Password}

	serialized, err := json.Marshal(authConfigs)
	if err != nil {
		return nil, err
	}

	secret := api.Secret{}
	secret.GenerateName = utils.MakeDNS1123Compatible("credential-secret")
	secret.Namespace = u.config.Kubernetes.Namespace
	secret.Type = api.SecretTypeDockercfg
	secret.Data = map[string][]byte{}
	secret.Data[api.DockerConfigKey] = serialized
	return u.cli.CoreV1().Secrets(u.config.Kubernetes.Namespace).Create(ctx, &secret, metav1.CreateOptions{})
}

// Execute - executes a command in Kubernetes container
func (u *Utils) Execute(
	ctx context.Context,
	base *executor.BaseCommandRunner,
	podName string,
	containerName string,
	cmd string,
	useAttach bool,
	executeCommandWithoutShell bool) (*api.Pod, error) {
	pod, err := u.cli.CoreV1().Pods(u.config.Kubernetes.Namespace).
		Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("couldn't get pod details due to %w", err)
	}

	if pod.Status.Phase != api.PodRunning {
		return nil, fmt.Errorf(
			"pod %q (on namespace %q) is not running and cannot execute commands; current phase is %q",
			podName, u.config.Kubernetes.Namespace, pod.Status.Phase,
		)
	}

	if containerName == "" {
		containerName = pod.Spec.Containers[0].Name
	}

	if base.Debug || logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":                  "KubernetesAdapter",
			"POD":                        pod.Name,
			"Namespace":                  pod.Namespace,
			"ID":                         pod.UID,
			"Command":                    cmd,
			"Container":                  containerName,
			"Status":                     pod.Status.Phase,
			"Memory":                     utils.MemUsageMiBString(),
			"ExecuteCommandWithoutShell": executeCommandWithoutShell,
			"UseAttach":                  useAttach,
		}).Info("executing...")
	}

	var req *restclient.Request
	var stdin *strings.Reader
	if useAttach {
		// Ending with a newline is important to actually run the script -- See api.PodExecOptions for different syntax
		stdin = strings.NewReader(fmt.Sprintf("/bin/sh -c %s\n", cmd)) // stdin = &base.Stdin
		req = u.cli.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(pod.Name).
			Namespace(pod.Namespace).
			SubResource("attach").
			VersionedParams(&api.PodAttachOptions{
				Container: containerName,
				Stdin:     true,
				Stdout:    false, // verify
				Stderr:    false, // verify
				TTY:       false,
			}, scheme.ParameterCodec)
	} else {
		var cmds []string
		if executeCommandWithoutShell {
			cmds = strings.Split(cmd, " ")
		} else {
			cmds = []string{"/bin/sh", "-c", cmd}
		}
		stdin = strings.NewReader("") // TODO from container command
		req = u.cli.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(pod.Name).
			Namespace(pod.Namespace).
			SubResource("exec").
			Param("container", containerName). //Context(ctx)
			VersionedParams(&api.PodExecOptions{
				Container: containerName,
				Command:   cmds,
				Stdin:     true,
				Stdout:    true, // verify?
				Stderr:    true, // verify?
			}, scheme.ParameterCodec)
	}

	exec, err := remotecommand.NewSPDYExecutor(u.restConfig, http.MethodPost, req.URL())
	if err != nil {
		return pod, fmt.Errorf("failed to create create spdy executor for %s due to %w",
			pod.Name, err)
	}

	if err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: &base.Stdout, // base.Trace
		Stderr: &base.Stderr,
		Tty:    false,
	}); err != nil {
		return nil, fmt.Errorf("failed to execute with stream %s in pod %s due to: %w",
			cmd, pod.Name, err)
	}
	return pod, nil
}

// GetRuntimeInfo returns runtime info
func (u *Utils) GetRuntimeInfo(
	ctx context.Context,
	podName string) string {
	result := make(map[string]interface{})
	pod, err := u.GetPod(ctx, podName)
	if err != nil {
		return fmt.Sprintf("pod=%s error=%s", podName, err.Error())
	}
	pod.ObjectMeta.ManagedFields = make([]metav1.ManagedFieldsEntry, 0) // mask useless raw data
	result["PodInfo"] = pod
	events, err := u.GetEvents(ctx, pod.Namespace, pod.Name, pod.ResourceVersion, pod.Labels)
	if err == nil {
		result["PodEvents"] = events
	} else {
		result["PodEventsError"] = err.Error()
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("pod=%s error=%v status=%s %v %v\n",
		podName, err, pod.Status.Phase, pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses))
	for _, c := range pod.Status.Conditions {
		sb.WriteString(fmt.Sprintf("pod message=%s reason=%s condition=%v\n", pod.Status.Message, pod.Status.Reason, c))
	}

	if u.config.Common.Debug {
		if reader, err := u.GetLogs(ctx, pod.Namespace, pod.Name, 1024*1024); err == nil {
			if data, err := io.ReadAll(reader); err == nil {
				sb.Write(data)
			}
		}
	}

	if data, err := json.Marshal(result); err == nil {
		sb.Write(data)
	}
	return sb.String()
}

// Stop stops container
func (u *Utils) Stop(
	ctx context.Context,
	containerID string) error {
	return u.cli.CoreV1().Pods(u.config.Kubernetes.Namespace).
		Delete(ctx, containerID, metav1.DeleteOptions{})
}

// Dispose disposes kubernetes client
func (u *Utils) Dispose(
	ctx context.Context,
	namespace string,
	services []api.Service,
	credentials *api.Secret,
	configMap *api.ConfigMap,
	timeout time.Duration,
) []error {
	errors := u.cleanupResources(ctx, namespace, credentials, configMap)
	errors = append(errors, u.cleanupServices(ctx, timeout, services)...)
	closeKubeClient(u.cli)
	return errors
}

// GetLogs return logs
func (u *Utils) GetLogs(
	ctx context.Context,
	namespace string,
	name string,
	maxBytes int64) (io.ReadCloser, error) {
	return u.cli.CoreV1().Pods(namespace).GetLogs(
		name,
		&api.PodLogOptions{Follow: true, LimitBytes: &maxBytes}).Stream(ctx)
}

// ///////////////////////////////////////// PRIVATE METHODS ///////////////////////////////////////////
func (u *Utils) createKubernetesService(
	ctx context.Context,
	service *api.Service) (*api.Service, error) {
	return u.cli.CoreV1().Services(u.config.Kubernetes.Namespace).
		Create(ctx, service, metav1.CreateOptions{})
}

func (u *Utils) cleanupResources(
	ctx context.Context,
	namespace string,
	credentials *api.Secret,
	configMap *api.ConfigMap) []error {
	errors := make([]error, 0)
	if credentials != nil {
		if err := u.cli.CoreV1().Secrets(namespace).
			Delete(ctx, credentials.Name, metav1.DeleteOptions{}); err != nil {
			errors = append(errors, fmt.Errorf("error cleaning up secrets due to %w", err))
		}
	}
	if configMap != nil {
		if err := u.cli.CoreV1().ConfigMaps(namespace).
			Delete(ctx, configMap.Name, metav1.DeleteOptions{}); err != nil {
			errors = append(errors, fmt.Errorf("error cleaning up configmap due to %w", err))
		}
	}
	return errors
}

func (u *Utils) cleanupServices(
	ctx context.Context,
	timeout time.Duration,
	services []api.Service) []error {
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout*time.Second)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()
	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		svc := payload.(api.Service)
		return nil, u.cli.CoreV1().Services(svc.ObjectMeta.Namespace).Delete(
			ctx,
			svc.ObjectMeta.Name,
			metav1.DeleteOptions{})
	}
	futures := make([]async.Awaiter, len(services))
	for i := range services {
		futures[i] = async.Execute(ctx, handler, async.NoAbort, services[i])
	}

	errors := make([]error, 0)
	results := async.AwaitAll(ctx, futures...)
	for _, res := range results {
		if res.Err != nil {
			errors = append(errors, res.Err)
		}
	}

	return errors
}

func preparePodConfig(
	config *ant_config.AntConfig,
	name string,
	containers []api.Container,
	labels map[string]string,
	annotations map[string]string,
	imagePullSecrets []api.LocalObjectReference,
	hostAliases []api.HostAlias,
	nodeSelector map[string]string,
	tolerations []api.Toleration,
	affinity *api.Affinity,
	volumes []api.Volume,
	hostNetwork bool,
	initContainers []api.Container) (*api.Pod, error) {
	dnsPolicy, err := config.Kubernetes.DNSPolicy.Get()
	if err != nil {
		return nil, err
	}
	terminationGracePeriodSeconds := int64(config.TerminationGracePeriod.Seconds())
	pod := api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:         name,
			GenerateName: utils.MakeDNS1123Compatible(name),
			Namespace:    config.Kubernetes.Namespace,
			Labels:       labels,
			Annotations:  annotations,
		},
		Spec: api.PodSpec{
			Volumes:                       volumes,
			ServiceAccountName:            config.Kubernetes.ServiceAccount,
			RestartPolicy:                 api.RestartPolicyNever,
			NodeSelector:                  nodeSelector,
			Tolerations:                   tolerations,
			Affinity:                      affinity,
			InitContainers:                initContainers,
			Containers:                    containers,
			HostAliases:                   hostAliases,
			ImagePullSecrets:              imagePullSecrets,
			SecurityContext:               config.Kubernetes.GetPodSecurityContext(),
			DNSPolicy:                     dnsPolicy,
			DNSConfig:                     config.Kubernetes.GetDNSConfig(),
			HostNetwork:                   hostNetwork,
			TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
		},
	}

	return &pod, nil
}

// buildContainer builds container with enhanced resource support while maintaining backward compatibility
func buildContainer(
	config *ant_config.KubernetesConfig,
	name string,
	cwd string,
	image string,
	imageDefinition domain.Image,
	requests api.ResourceList,
	limits api.ResourceList,
	volumeMounts []api.VolumeMount,
	env []api.EnvVar,
	privileged bool,
	containerCommand ...string) api.Container {

	var allowPrivilegeEscalation *bool
	containerPorts, proxyPorts := imageDefinition.GetPorts()

	// TODO add proxy support
	if len(proxyPorts) > 0 {
		serviceName := imageDefinition.Alias
		if serviceName == "" {
			serviceName = name
			serviceName = fmt.Sprintf("proxy-%s", name)
		}
	}

	allowPrivilegeEscalation = &config.AllowPrivilegeEscalation
	if !config.AllowPrivilegeEscalation {
		privileged = false
	}
	command, args := getCommandAndArgs(imageDefinition, containerCommand...)

	container := api.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: api.PullPolicy(config.PullPolicy.GetKubernetesPullPolicy()),
		Command:         command,
		Args:            args,
		Env:             env,
		Ports:           containerPorts,
		Resources: api.ResourceRequirements{
			Limits:   limits,
			Requests: requests,
		},
		VolumeMounts: volumeMounts,
		SecurityContext: &api.SecurityContext{
			Privileged:               &privileged,
			AllowPrivilegeEscalation: allowPrivilegeEscalation,
			Capabilities: getCapabilities(
				getDefaultCapDrop(),
				config.CapAdd,
				config.CapDrop,
			),
		},
		Stdin: true,
	}

	if cwd != "" {
		container.WorkingDir = cwd
	}

	// Enhanced logging for resource configuration
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":                "KubernetesAdapter",
			"Container":                name,
			"Privileged":               privileged,
			"AllowPrivilegeEscalation": *allowPrivilegeEscalation,
			"Requests":                 requests,
			"Limits":                   limits,
		}).Debug("built container with enhanced resource configuration")
	}

	return container
}

// hasStructuredServiceResources checks if service has structured resource configuration
func (u *Utils) hasStructuredServiceResources(_ domain.Service) bool {
	// Check if global config has structured resources configured
	// Services will use global defaults unless they have their own config
	//return u.config.Kubernetes.DefaultResources.CPURequest != "" ||
	//	u.config.Kubernetes.DefaultResources.MemoryRequest != ""
	return false
}

// createStructuredServiceResources creates resources using structured configuration
//func (u *Utils) createStructuredServiceResources(svcName string, service domain.Service) (requests, limits api.ResourceList, cost float64, err error) {
//	// Start with global defaults
//	resourceConfig := u.config.Kubernetes.DefaultResources
//
//	// Create a service-specific config based on the service's resource requests/limits
//	serviceResourceConfig := ant_config.ResourceConfig{
//		CPURequest:              getServiceStringValue(service.CPURequest, resourceConfig.CPURequest),
//		MemoryRequest:           getServiceStringValue(service.MemoryRequest, resourceConfig.MemoryRequest),
//		CPULimit:                getServiceStringValue(service.CPULimit, resourceConfig.CPULimit),
//		MemoryLimit:             getServiceStringValue(service.MemoryLimit, resourceConfig.MemoryLimit),
//		EphemeralStorageRequest: getServiceStringValue(service.EphemeralStorageRequest, resourceConfig.EphemeralStorageRequest),
//		EphemeralStorageLimit:   getServiceStringValue(service.EphemeralStorageLimit, resourceConfig.EphemeralStorageLimit),
//	}
//
//	// Build resource requirements
//	resourceReqs := serviceResourceConfig.BuildResourceRequirements()
//
//	// Validate against limits
//	if err = u.config.Kubernetes.ValidateResourceLimits("service-"+svcName, resourceReqs); err != nil {
//		return nil, nil, 0, err
//	}
//
//	cost = u.config.Kubernetes.CalculateResourceCost(resourceReqs)
//
//	return resourceReqs.Requests, resourceReqs.Limits, cost, nil
//}

// hasStructuredMainContainerResources checks if main container has structured resources
func (u *Utils) hasStructuredMainContainerResources(_ *domain.ExecutorOptions) bool {
	//return u.config.Kubernetes.DefaultResources.CPURequest != "" ||
	//	u.config.Kubernetes.DefaultResources.MemoryRequest != ""
	return false
}

// createStructuredMainContainerResources creates main container resources
//func (u *Utils) createStructuredMainContainerResources(opts *domain.ExecutorOptions) (requests, limits api.ResourceList, cost float64, err error) {
//	// Start with global defaults
//	resourceConfig := u.config.Kubernetes.DefaultResources
//
//	// Override with main container specific values if available
//	mainResourceConfig := ant_config.ResourceConfig{
//		CPURequest:              getContainerStringValue(opts.MainContainer.CPURequest, resourceConfig.CPURequest),
//		MemoryRequest:           getContainerStringValue(opts.MainContainer.MemoryRequest, resourceConfig.MemoryRequest),
//		CPULimit:                getContainerStringValue(opts.MainContainer.CPULimit, resourceConfig.CPULimit),
//		MemoryLimit:             getContainerStringValue(opts.MainContainer.MemoryLimit, resourceConfig.MemoryLimit),
//		EphemeralStorageRequest: getContainerStringValue(opts.MainContainer.EphemeralStorageRequest, resourceConfig.EphemeralStorageRequest),
//		EphemeralStorageLimit:   getContainerStringValue(opts.MainContainer.EphemeralStorageLimit, resourceConfig.EphemeralStorageLimit),
//	}
//
//	resourceReqs := mainResourceConfig.BuildResourceRequirements()
//
//	if err = u.config.Kubernetes.ValidateResourceLimits("main-"+opts.Name, resourceReqs); err != nil {
//		return nil, nil, 0, err
//	}
//
//	cost = u.config.Kubernetes.CalculateResourceCost(resourceReqs)
//
//	return resourceReqs.Requests, resourceReqs.Limits, cost, nil
//}

// hasStructuredHelperResources checks if helper container has structured resources
func (u *Utils) hasStructuredHelperResources(_ *domain.ExecutorOptions) bool {
	//return u.config.Kubernetes.DefaultResources.CPURequest != "" ||
	//	u.config.Kubernetes.DefaultResources.MemoryRequest != ""
	return false
}

// createStructuredHelperResources creates helper container resources
func (u *Utils) createStructuredHelperResources(opts *domain.ExecutorOptions) (requests, limits api.ResourceList, cost float64, err error) {
	// Helper containers typically use smaller resources
	baseResourceConfig := ant_config.ResourceConfig{
		CPURequest:    "100m",
		MemoryRequest: "128Mi",
		CPULimit:      "200m",
		MemoryLimit:   "256Mi",
	}

	// If helper container has specific resource config, use it; otherwise use scaled defaults
	helperResourceConfig := ant_config.ResourceConfig{
		CPURequest:              getContainerStringValue(opts.HelperContainer.CPURequest, baseResourceConfig.CPURequest),
		MemoryRequest:           getContainerStringValue(opts.HelperContainer.MemoryRequest, baseResourceConfig.MemoryRequest),
		CPULimit:                getContainerStringValue(opts.HelperContainer.CPULimit, baseResourceConfig.CPULimit),
		MemoryLimit:             getContainerStringValue(opts.HelperContainer.MemoryLimit, baseResourceConfig.MemoryLimit),
		EphemeralStorageRequest: getContainerStringValue(opts.HelperContainer.EphemeralStorageRequest, baseResourceConfig.EphemeralStorageRequest),
		EphemeralStorageLimit:   getContainerStringValue(opts.HelperContainer.EphemeralStorageLimit, baseResourceConfig.EphemeralStorageLimit),
	}

	resourceReqs := helperResourceConfig.BuildResourceRequirements()

	// Helper containers don't need strict validation, but we can still validate
	// Use a more lenient approach for helpers
	cost = u.config.Kubernetes.CalculateResourceCost(resourceReqs)

	return resourceReqs.Requests, resourceReqs.Limits, cost, nil
}

// getServiceStringValue returns service value if not empty, otherwise returns default
func getServiceStringValue(serviceValue, defaultValue string) string {
	if serviceValue != "" {
		return serviceValue
	}
	return defaultValue
}

// getContainerStringValue returns container value if not empty, otherwise returns default
func getContainerStringValue(containerValue, defaultValue string) string {
	if containerValue != "" {
		return containerValue
	}
	return defaultValue
}

// createPodWithContainers creates the final pod with all containers
func (u *Utils) createPodWithContainers(
	opts *domain.ExecutorOptions,
	containers []api.Container,
	volumes []api.Volume,
	hostAliases []api.HostAlias,
	aliasNames []string,
	totalCost float64) (*api.Pod, []string, []string, float64, error) {

	// Extract service names for logging
	serviceNames := make([]string, 0, len(containers))
	for _, container := range containers {
		serviceNames = append(serviceNames, container.Name)
	}

	// Create pod configuration using existing preparePodConfig function
	podConfig, err := preparePodConfig(
		u.config,
		opts.Name,
		containers,
		opts.PodLabels,
		opts.PodAnnotations,
		buildImagePullSecrets(u.config),
		hostAliases,
		opts.NodeSelector,
		opts.NodeTolerations.Get(),
		opts.GetAffinity(),
		volumes,
		opts.HostNetwork,
		u.config.Kubernetes.GetInitContainers())

	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to create pod config for %s due to %w", opts.Name, err)
	}

	// Log enhanced pod creation details
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":     "KubernetesAdapter",
			"POD":           podConfig.Name,
			"Services":      serviceNames,
			"ServicesCount": len(containers),
			"TotalCost":     totalCost,
			"ResourceType":  u.getResourceConfigType(),
			"Options":       opts.String(),
			"Namespace":     u.config.Kubernetes.Namespace,
		}).Debug("creating pod with enhanced resource configuration...")
	}

	// Create the pod
	pod, err := u.cli.CoreV1().Pods(u.config.Kubernetes.Namespace).Create(
		context.Background(), podConfig, metav1.CreateOptions{})

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "KubernetesAdapter",
			"POD":       podConfig.Name,
			"Error":     err,
			"Namespace": u.config.Kubernetes.Namespace,
		}).Warn("failed to create pod with enhanced configuration")
	} else {
		logrus.WithFields(logrus.Fields{
			"Component":     "KubernetesAdapter",
			"POD":           podConfig.Name,
			"Services":      serviceNames,
			"ServicesCount": len(containers),
			"TotalCost":     totalCost,
			"ResourceType":  u.getResourceConfigType(),
			"Namespace":     u.config.Kubernetes.Namespace,
		}).Info("created pod with enhanced resource configuration!")
	}

	return pod, serviceNames, aliasNames, totalCost, err
}

func (u *Utils) getResourceConfigType() string {
	//if u.config.Kubernetes.DefaultResources.CPURequest != "" {
	//	return "structured"
	//}
	if u.config.Kubernetes.DefaultLimits != nil {
		return "legacy"
	}
	return "default"
}

// buildImagePullSecrets creates image pull secrets
func buildImagePullSecrets(config *ant_config.AntConfig) []api.LocalObjectReference {
	var imagePullSecrets []api.LocalObjectReference
	for _, imagePullSecret := range config.Kubernetes.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, api.LocalObjectReference{Name: imagePullSecret})
	}
	return imagePullSecrets
}

// GetConfigInfo returns Kubernetes executor configuration information
func (u *Utils) GetConfigInfo() map[string]any {
	config := make(map[string]interface{})

	// Basic cluster info
	config["cluster_name"] = u.config.Kubernetes.ClusterName
	config["namespace"] = u.config.Kubernetes.Namespace
	config["service_account"] = u.config.Kubernetes.ServiceAccount
	config["host"] = fmt.Sprintf("%s", u.config.Kubernetes.Host)

	// Registry configuration
	if u.config.Kubernetes.Registry.Server != "" {
		config["registry_server"] = u.config.Kubernetes.Registry.Server
		config["registry_username"] = u.config.Kubernetes.Username
		config["registry_has_password"] = u.config.Kubernetes.Password != ""
		config["pull_policy"] = fmt.Sprintf("%s", u.config.Kubernetes.PullPolicy)
	}

	// Resource configuration
	config["resource_config_type"] = u.getResourceConfigType()
	if u.config.Kubernetes.DefaultLimits != nil {
		config["default_cpu_limit"] = u.config.Kubernetes.DefaultLimits.Cpu()
		config["default_memory_limit"] = u.config.Kubernetes.DefaultLimits.Memory()
		config["default_ephemeral_storage_limit"] = u.config.Kubernetes.DefaultLimits.StorageEphemeral()
	}

	// Security settings
	config["allow_privilege_escalation"] = u.config.Kubernetes.AllowPrivilegeEscalation
	config["image_pull_secrets"] = u.config.Kubernetes.ImagePullSecrets

	// DNS and networking
	if dnsPolicy, err := u.config.Kubernetes.DNSPolicy.Get(); err == nil {
		config["dns_policy"] = string(dnsPolicy)
	}

	// Pod security context
	if secCtx := u.config.Kubernetes.GetPodSecurityContext(); secCtx != nil {
		config["pod_run_as_user"] = secCtx.RunAsUser
		config["pod_run_as_group"] = secCtx.RunAsGroup
		config["pod_fs_group"] = secCtx.FSGroup
	}

	return config
}

// GetConnectionString returns a formatted connection string for Kubernetes
func (u *Utils) GetConnectionString() string {
	var parts []string

	if u.config.Kubernetes.ClusterName != "" {
		parts = append(parts, fmt.Sprintf("Cluster=%s", u.config.Kubernetes.ClusterName))
	}

	if u.config.Kubernetes.Host != "" {
		parts = append(parts, fmt.Sprintf("Host=%s", u.config.Kubernetes.Host))
	}

	if u.config.Kubernetes.Namespace != "" {
		parts = append(parts, fmt.Sprintf("Namespace=%s", u.config.Kubernetes.Namespace))
	}

	if u.config.Kubernetes.ServiceAccount != "" {
		parts = append(parts, fmt.Sprintf("ServiceAccount=%s", u.config.Kubernetes.ServiceAccount))
	}

	return fmt.Sprintf("Kubernetes[%s]", strings.Join(parts, ", "))
}

// GetClusterInfo returns detailed Kubernetes cluster information
func (u *Utils) GetClusterInfo() (map[string]interface{}, error) {
	ctx := context.Background()
	result := make(map[string]interface{})

	// Get server version
	version, err := u.cli.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %w", err)
	}
	result["server_version"] = version.String()
	result["git_version"] = version.GitVersion
	result["platform"] = version.Platform

	// Get cluster nodes info
	nodes, err := u.cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err == nil {
		nodeInfo := make([]map[string]interface{}, 0, len(nodes.Items))
		for _, node := range nodes.Items {
			nodeData := map[string]interface{}{
				"name":               node.Name,
				"ready":              isNodeReady(node),
				"kubernetes_version": node.Status.NodeInfo.KubeletVersion,
				"os_image":           node.Status.NodeInfo.OSImage,
				"architecture":       node.Status.NodeInfo.Architecture,
				"container_runtime":  node.Status.NodeInfo.ContainerRuntimeVersion,
				"internal_ip":        getNodeInternalIP(node),
			}
			nodeInfo = append(nodeInfo, nodeData)
		}
		result["nodes"] = nodeInfo
		result["node_count"] = len(nodes.Items)
	} else {
		result["nodes_error"] = err.Error()
	}

	// Get namespace info
	namespaces, err := u.cli.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err == nil {
		result["total_namespaces"] = len(namespaces.Items)
		result["current_namespace"] = u.config.Kubernetes.Namespace
	}

	return result, nil
}

// GetStatus returns current Kubernetes executor status
func (u *Utils) GetStatus() map[string]interface{} {
	status := make(map[string]interface{})

	// Basic connectivity and config
	status["connected"] = u.isClusterAccessible()
	status["config"] = u.GetConnectionString()

	// Get cluster info if possible
	if clusterInfo, err := u.GetClusterInfo(); err == nil {
		status["cluster_info"] = clusterInfo
	} else {
		status["cluster_error"] = err.Error()
	}

	// Get current namespace status
	if nsInfo, err := u.getNamespaceInfo(); err == nil {
		status["namespace_info"] = nsInfo
	} else {
		status["namespace_error"] = err.Error()
	}

	// Get current pods in namespace
	if pods, err := u.List(context.Background()); err == nil {
		status["active_pods"] = len(pods)
		status["pods"] = pods
	} else {
		status["pods_error"] = err.Error()
	}

	return status
}

// GetResourceQuotas returns resource quota information for the namespace
func (u *Utils) GetResourceQuotas() (map[string]interface{}, error) {
	ctx := context.Background()
	result := make(map[string]interface{})

	// Get resource quotas
	quotas, err := u.cli.CoreV1().ResourceQuotas(u.config.Kubernetes.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get resource quotas: %w", err)
	}

	quotaInfo := make([]map[string]interface{}, 0, len(quotas.Items))
	for _, quota := range quotas.Items {
		quotaData := map[string]interface{}{
			"name": quota.Name,
			"hard": quota.Status.Hard,
			"used": quota.Status.Used,
		}
		quotaInfo = append(quotaInfo, quotaData)
	}
	result["resource_quotas"] = quotaInfo

	// Get limit ranges
	limitRanges, err := u.cli.CoreV1().LimitRanges(u.config.Kubernetes.Namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		limitInfo := make([]map[string]interface{}, 0, len(limitRanges.Items))
		for _, lr := range limitRanges.Items {
			limitData := map[string]interface{}{
				"name":   lr.Name,
				"limits": lr.Spec.Limits,
			}
			limitInfo = append(limitInfo, limitData)
		}
		result["limit_ranges"] = limitInfo
	}

	return result, nil
}

// Helper methods

func (u *Utils) isClusterAccessible() bool {
	_, err := u.cli.Discovery().ServerVersion()
	return err == nil
}

func (u *Utils) getNamespaceInfo() (map[string]interface{}, error) {
	ctx := context.Background()
	ns, err := u.cli.CoreV1().Namespaces().Get(ctx, u.config.Kubernetes.Namespace, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"name":        ns.Name,
		"status":      string(ns.Status.Phase),
		"created":     ns.CreationTimestamp.Time,
		"labels":      ns.Labels,
		"annotations": ns.Annotations,
	}, nil
}

func isNodeReady(node api.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == api.NodeReady {
			return condition.Status == api.ConditionTrue
		}
	}
	return false
}

func getNodeInternalIP(node api.Node) string {
	for _, address := range node.Status.Addresses {
		if address.Type == api.NodeInternalIP {
			return address.Address
		}
	}
	return ""
}
