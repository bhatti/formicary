package handler

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"plexobject.com/formicary/internal/ant_config"
	"plexobject.com/formicary/internal/events"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/ants/executor/utils"
	"plexobject.com/formicary/ants/registry"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tasklet"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
)

// RequestHandler structure
type RequestHandler struct {
	*tasklet.BaseTasklet
	antCfg            *ant_config.AntConfig
	queueClient       queue.Client
	webClient         web.HTTPClient
	containerRegistry *registry.AntContainersRegistry
	metricsRegistry   *metrics.Registry
	executor          RequestExecutor
	healthChecker     *MethodHealthChecker
}

// NewRequestHandler constructor
func NewRequestHandler(
	antCfg *ant_config.AntConfig,
	queueClient queue.Client,
	webClient web.HTTPClient,
	requestRegistry tasklet.RequestRegistry,
	containerRegistry *registry.AntContainersRegistry,
	metricsRegistry *metrics.Registry,
	executor RequestExecutor,
	requestTopic string) *RequestHandler {
	t := &RequestHandler{
		antCfg:            antCfg,
		queueClient:       queueClient,
		webClient:         webClient,
		containerRegistry: containerRegistry,
		metricsRegistry:   metricsRegistry,
		executor:          executor,
	}

	// Filter incoming task messages to this ant only.
	// The queen sets MessageTarget to the ant's AntID when dispatching; an empty
	// target means broadcast (e.g. cancel/terminate commands).  This prevents
	// ants from processing messages intended for a different ant on shared topics.
	// When an ant connects with an org-scoped JWT the queen appends "@<orgID>"
	// to the ant ID before storing the registration, so MessageTarget arrives as
	// "desktop-control-plane@<orgID>".  Accept both the bare ID and any
	// "<antID>@<suffix>" form so the filter works regardless of auth mode.
	antID := antCfg.Common.ID
	msgFilter := func(ctx context.Context, event *queue.MessageEvent) bool {
		target := event.Properties[queue.MessageTarget]
		return target == "" || target == antID || strings.HasPrefix(target, antID+"@")
	}

	registration := antCfg.NewAntRegistration()
	t.BaseTasklet = tasklet.NewBaseTasklet(
		antCfg.Common.ID+"-request-handler",
		&antCfg.Common,
		queueClient,
		msgFilter,
		requestRegistry,
		requestTopic,
		antCfg.Common.GetRegistrationTopic(),
		registration,
		t,
	)
	t.healthChecker = NewMethodHealthChecker(antCfg, registration)
	return t
}

// PreExecute checks if request can proceed
func (rh *RequestHandler) PreExecute(
	ctx context.Context,
	req *types.TaskRequest) bool {
	if status, err := rh.containerRegistry.CheckIfAlreadyRunning(
		req.ExecutorOpts.Method, req.ExecutorOpts.Name); err != nil {
		rh.metricsRegistry.Incr(
			"ant_duplicate_request_total", nil)
		logrus.WithFields(
			logrus.Fields{
				"Component":       "RequestHandler",
				"AntID":           rh.ID,
				"ContainerStatus": status,
				"UserID":          req.UserID,
				"Request":         req,
				"Error":           err,
			}).Warn("received duplicate request so ignoring it")
		if status == registry.ContainerExistsWithGoodAnt {
			return false // the other ant will respond
		}
		// for orphan container, let's try to kill it
		_ = utils.StopContainer(ctx, rh.antCfg, rh.webClient, req.ExecutorOpts, req.ExecutorOpts.Name)
	}
	return true
}

// Start overrides BaseTasklet.Start to also launch the per-method health checker.
func (rh *RequestHandler) Start(ctx context.Context) error {
	if err := rh.BaseTasklet.Start(ctx); err != nil {
		return err
	}
	rh.healthChecker.Start(ctx)
	return nil
}

// Stop overrides BaseTasklet.Stop to also stop the health checker and release its clients.
func (rh *RequestHandler) Stop(ctx context.Context) error {
	rh.healthChecker.Stop()
	return rh.BaseTasklet.Stop(ctx)
}

// Execute request
func (rh *RequestHandler) Execute(
	ctx context.Context,
	req *types.TaskRequest) (taskResp *types.TaskResponse, err error) {
	return rh.executor.Execute(ctx, req), nil
}

// TerminateContainer terminates container
func (rh *RequestHandler) TerminateContainer(
	ctx context.Context,
	taskReq *types.TaskRequest) (taskResp *types.TaskResponse, err error) {
	// Look up the registry entry to use for lifecycle event, but do NOT bail out if missing.
	// The container may have already been removed from the registry (e.g. after a job cancel)
	// while the pod is still running in Kubernetes — we must still attempt the actual deletion.
	container := rh.containerRegistry.GetContainerEvent(taskReq.ExecutorOpts.Method, taskReq.ExecutorOpts.Name)
	if container == nil {
		logrus.WithFields(logrus.Fields{
			"Component": "RequestHandler",
			"AntID":     rh.antCfg.Common.ID,
			"Name":      taskReq.ExecutorOpts.Name,
			"Method":    taskReq.ExecutorOpts.Method,
		}).Warn("container not in registry, attempting direct pod deletion anyway")
	}

	if err = utils.StopContainer(
		ctx,
		rh.antCfg,
		rh.webClient,
		taskReq.ExecutorOpts,
		taskReq.ExecutorOpts.Name); err != nil {
		taskResp = types.NewTaskResponse(taskReq)
		taskResp.Status = types.FAILED
		taskResp.ErrorCode = types.ErrorContainerStoppedFailed
		taskResp.ErrorMessage = err.Error()
		return
	}

	taskResp = types.NewTaskResponse(taskReq)
	taskResp.Status = types.COMPLETED

	// Always publish a CANCELLED lifecycle event so the queen removes this entry from its
	// registry. If we have the full registry entry use it; otherwise synthesize a minimal
	// event from the executor options so the queen can still clean up by key.
	eventInfo := containerInfo(container, taskReq, rh.antCfg.Common.ID)
	if sendErr := sendContainerEvent(
		ctx,
		rh.antCfg,
		rh.queueClient,
		taskReq.UserID,
		taskReq.ExecutorOpts.Method,
		types.CANCELLED,
		eventInfo); sendErr != nil {
		logrus.WithFields(
			logrus.Fields{
				"Component": "RequestHandler",
				"AntID":     rh.antCfg.Common.ID,
				"Name":      taskReq.ExecutorOpts.Name,
				"Error":     sendErr,
			}).Warnf("failed to send stop lifecycle event container by request-handler")
	}
	return
}

// containerInfo returns the registry event if available, or a minimal synthetic event
// built from executor options so the queen can always remove the entry by key.
func containerInfo(
	container *events.ContainerLifecycleEvent,
	taskReq *types.TaskRequest,
	antID string) *events.ContainerLifecycleEvent {
	if container != nil {
		return container
	}
	now := time.Now()
	return events.NewContainerLifecycleEvent(
		"RequestHandler",
		taskReq.UserID,
		antID,
		taskReq.ExecutorOpts.Method,
		taskReq.ExecutorOpts.Name,
		taskReq.ExecutorOpts.Name,
		types.CANCELLED,
		make(map[string]string),
		now,
		&now,
	)
}

// ListContainers list containers
func (rh *RequestHandler) ListContainers(
	_ context.Context,
	req *types.TaskRequest) (taskResp *types.TaskResponse, err error) {
	taskResp = types.NewTaskResponse(req)
	containers, err := json.Marshal(rh.containerRegistry.GetContainerEvents())
	if err == nil {
		taskResp.Status = types.COMPLETED
		taskResp.AddContext("containers", string(containers))
	} else {
		taskResp.Status = types.FAILED
		taskResp.ErrorCode = types.ErrorMarshalingFailed
		taskResp.ErrorMessage = err.Error()
	}
	return
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
