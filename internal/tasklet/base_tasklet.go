package tasklet

import (
	"context"
	"encoding/json"
	"fmt"
	cutils "plexobject.com/formicary/internal/utils"
	"time"

	evbus "github.com/asaskevich/EventBus"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/types"
)

// Executor interface
type Executor interface {
	PreExecute(
		_ context.Context,
		_ *types.TaskRequest) bool

	Execute(
		_ context.Context,
		_ *types.TaskRequest) (taskResp *types.TaskResponse, err error)

	TerminateContainer(
		_ context.Context,
		_ *types.TaskRequest) (taskResp *types.TaskResponse, err error)

	ListContainers(
		_ context.Context,
		_ *types.TaskRequest) (taskResp *types.TaskResponse, err error)
}

// BaseTasklet structure
type BaseTasklet struct {
	ID                 string
	Config             *types.CommonConfig
	QueueClient        queue.Client
	RequestRegistry    RequestRegistry
	RequestTopic       string
	RegistrationTopic  string
	registration       types.AntRegistration
	executor           Executor
	done               chan bool
	registrationTicker *time.Ticker
	EventBus           evbus.Bus
}

// NewBaseTasklet constructor
func NewBaseTasklet(
	idSuffix string,
	cfg *types.CommonConfig,
	queueClient queue.Client,
	requestRegistry RequestRegistry,
	requestTopic string,
	registrationTopic string,
	registration types.AntRegistration,
	executor Executor,
) *BaseTasklet {
	return &BaseTasklet{
		ID:                cfg.ID + idSuffix,
		Config:            cfg,
		QueueClient:       queueClient,
		RequestRegistry:   requestRegistry,
		RequestTopic:      requestTopic,
		RegistrationTopic: registrationTopic,
		registration:      registration,
		executor:          executor,
		EventBus:          evbus.New(),
		done:              make(chan bool, 1),
	}
}

// Start subscribes for incoming requests
func (t *BaseTasklet) Start(
	ctx context.Context) error {
	if err := t.subscribeToIncomingRequests(ctx); err != nil {
		return err
	}
	if err := t.subscribeToJobLifecycleEvent(ctx, t.Config.GetJobExecutionLifecycleTopic()); err != nil {
		_ = t.Stop(ctx)
		return err
	}
	if err := t.subscribeToTaskLifecycleEvent(ctx, t.Config.GetTaskExecutionLifecycleTopic()); err != nil {
		_ = t.Stop(ctx)
		return err
	}
	t.startTickerForRegistration(ctx)
	logrus.WithFields(
		logrus.Fields{
			"Component":    "BaseTasklet",
			"Tasklet":      t.ID,
			"Registration": t.registration,
		}).Info("added ticker for registration and subscribed to receive incoming requests/job/task events")
	return nil
}

// Stop stops subscribing to incoming requests
func (t *BaseTasklet) Stop(
	ctx context.Context) error {
	err1 := t.QueueClient.UnSubscribe(ctx, t.RequestTopic, t.ID)
	err3 := t.QueueClient.UnSubscribe(ctx, t.Config.GetJobExecutionLifecycleTopic(), t.ID)
	err2 := t.QueueClient.UnSubscribe(ctx, t.Config.GetTaskExecutionLifecycleTopic(), t.ID)
	if t.registrationTicker != nil {
		t.registrationTicker.Stop()
	}
	return cutils.ErrorsAny(err1, err2, err3)
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (t *BaseTasklet) handleRequest(
	ctx context.Context,
	req *types.TaskRequest) (err error) {
	started := time.Now()
	logrus.WithFields(
		logrus.Fields{
			"Component":       "BaseTasklet",
			"Tasklet":         t.ID,
			"RequestID":       req.JobRequestID,
			"JobType":         req.JobType,
			"TaskType":        req.TaskType,
			"TaskExecutionID": req.TaskExecutionID,
			"Params":          req.Variables,
		}).Info("received request")
	ctx, cancel := context.WithCancel(ctx)
	req.Cancel = cancel
	defer cancel()

	if req.Action == types.CANCEL { // async -- no response
		if err := t.RequestRegistry.Cancel(req.Key()); err != nil {
			logrus.WithFields(
				logrus.Fields{
					"Component": "BaseTasklet",
					"Tasklet":   t.ID,
					"Request":   req,
					"Error":     err,
				}).Error("failed to cancel request")
		} else {
			logrus.WithFields(
				logrus.Fields{
					"Component": "BaseTasklet",
					"Tasklet":   t.ID,
					"Request":   req,
				}).Info("cancelled request")
		}
	} else if req.Action == types.EXECUTE {
		if proceed := t.executor.PreExecute(ctx, req); !proceed {
			return nil
		}
		if err = t.RequestRegistry.Add(req); err == nil {
			defer func() {
				if err := t.RequestRegistry.Remove(req); err != nil {
					if logrus.IsLevelEnabled(logrus.DebugLevel) {
						logrus.WithFields(
							logrus.Fields{
								"Component": "BaseTasklet",
								"Tasklet":   t.ID,
								"Request":   req,
								"Error":     err,
							}).Debug("failed to remove request")
					}
				}
			}()
			var taskResp *types.TaskResponse
			if taskResp, err = t.executor.Execute(ctx, req); err != nil {
				logrus.WithFields(
					logrus.Fields{
						"Component": "BaseTasklet",
						"Tasklet":   t.ID,
						"Request":   req,
						"Error":     err,
					}).Warn("failed to execute request")
				taskResp = types.NewTaskResponse(req)
				taskResp.Status = types.FAILED
				taskResp.ErrorCode = types.ErrorAntExecutionFailed
				taskResp.ErrorMessage = err.Error()
				err = t.sendResponse(ctx, taskResp, req.ResponseTopic, started)
			} else {
				err = t.sendResponse(ctx, taskResp, req.ResponseTopic, started)
			}
		}
	} else if req.Action == types.TERMINATE {
		var taskResp *types.TaskResponse
		if taskResp, err = t.executor.TerminateContainer(ctx, req); err != nil {
			logrus.WithFields(
				logrus.Fields{
					"Component": "BaseTasklet",
					"Tasklet":   t.ID,
					"Request":   req,
					"Error":     err,
				}).Warn("failed to terminate container")
			taskResp = types.NewTaskResponse(req)
			taskResp.Status = types.FAILED
			taskResp.ErrorCode = types.ErrorAntExecutionFailed
			taskResp.ErrorMessage = err.Error()
			err = t.sendResponse(ctx, taskResp, req.ResponseTopic, started)
		} else {
			err = t.sendResponse(ctx, taskResp, req.ResponseTopic, started)
		}
	} else if req.Action == types.LIST {
		var taskResp *types.TaskResponse
		if taskResp, err = t.executor.ListContainers(ctx, req); err != nil {
			logrus.WithFields(
				logrus.Fields{
					"Component": "BaseTasklet",
					"Tasklet":   t.ID,
					"Request":   req,
					"Error":     err,
				}).Warn("failed to list containers")
			taskResp = types.NewTaskResponse(req)
			taskResp.Status = types.FAILED
			taskResp.ErrorCode = types.ErrorAntExecutionFailed
			taskResp.ErrorMessage = err.Error()
			err = t.sendResponse(ctx, taskResp, req.ResponseTopic, started)
		} else {
			err = t.sendResponse(ctx, taskResp, req.ResponseTopic, started)
		}
	} else {
		logrus.WithFields(
			logrus.Fields{
				"Component": "BaseTasklet",
				"Tasklet":   t.ID,
				"Action":    req.Action,
				"Request":   req,
			}).Error("received unknown request")
		taskResp := types.NewTaskResponse(req)
		taskResp.Status = types.FAILED
		taskResp.ErrorCode = types.ErrorAntExecutionFailed
		taskResp.ErrorMessage = fmt.Sprintf("received unknown action %s", req.Action)
		err = t.sendResponse(ctx, taskResp, req.ResponseTopic, started)
	}
	return
}

func (t *BaseTasklet) sendResponse(
	ctx context.Context,
	taskResp *types.TaskResponse,
	responseTopic string,
	started time.Time) error {
	b, err := json.Marshal(taskResp)
	fields := logrus.Fields{
		"Component":       "BaseTasklet",
		"Tasklet":         t.ID,
		"RequestID":       taskResp.JobRequestID,
		"JobType":         taskResp.JobType,
		"TaskType":        taskResp.TaskType,
		"TaskExecutionID": taskResp.TaskExecutionID,
		"Status":          taskResp.Status,
		"ErrorMessage":    taskResp.ErrorMessage,
		"Message":         taskResp.ExitCode,
		//"TaskContext":     taskResp.TaskContext,
		//"JobContext":      taskResp.JobContext,
		"Artifacts":     len(taskResp.Artifacts),
		"ResponseTopic": responseTopic,
		"Elapsed":       time.Since(started).String(),
	}

	if err == nil {
		// we don't need to keep producer because each response topic is different so we
		// will close producer right after sending it
		_, err = t.QueueClient.Send(ctx, responseTopic, map[string]string{}, b, false)
		if err == nil {
			if taskResp.Status == types.COMPLETED {
				logrus.WithFields(fields).Info("sent response with COMPLETED")
			} else {
				logrus.WithFields(fields).Warn("sent response with FAILED")
			}
		} else {
			fields["Error"] = err
			logrus.WithFields(fields).Error("failed to sendResponse response")
		}
	}
	return err
}
