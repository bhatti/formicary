package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/ants/config"
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
	antCfg            *config.AntConfig
	queueClient       queue.Client
	webClient         web.HTTPClient
	containerRegistry *registry.AntContainersRegistry
	metricsRegistry   *metrics.Registry
	executor          RequestExecutor
}

// NewRequestHandler constructor
func NewRequestHandler(
	antCfg *config.AntConfig,
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

	t.BaseTasklet = tasklet.NewBaseTasklet(
		antCfg.ID+"-request-handler",
		&antCfg.CommonConfig,
		queueClient,
		nil,
		requestRegistry,
		requestTopic,
		antCfg.GetRegistrationTopic(),
		antCfg.NewAntRegistration(),
		t,
	)
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
	container := rh.containerRegistry.GetContainerEvent(taskReq.ExecutorOpts.Method, taskReq.ExecutorOpts.Name)
	if container == nil {
		taskResp = types.NewTaskResponse(taskReq)
		taskResp.Status = types.FAILED
		taskResp.ErrorCode = types.ErrorContainerNotFound
		taskResp.ErrorMessage = fmt.Sprintf("failed to find container for %s method %s",
			taskReq.ExecutorOpts.Name, taskReq.ExecutorOpts.Method)
		return
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
	if sendErr := sendContainerEvent(
		ctx,
		rh.antCfg,
		rh.queueClient,
		taskReq.UserID,
		taskReq.ExecutorOpts.Method,
		types.CANCELLED,
		container); sendErr != nil {
		logrus.WithFields(
			logrus.Fields{
				"Component": "RequestHandler",
				"AntID":     rh.antCfg.ID,
				"Container": container,
				"Error":     sendErr,
			}).Warnf("failed to send stop lifecycle event container by request-handler")
	}
	return
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
