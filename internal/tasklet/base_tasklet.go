package tasklet

import (
	"context"
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
	ID                          string
	Config                      *types.CommonConfig
	QueueClient                 queue.Client
	QueueFilter                 queue.Filter
	RequestRegistry             RequestRegistry
	RequestTopic                string
	RegistrationTopic           string
	registration                *types.AntRegistration
	executor                    Executor
	totalExecuted               int
	done                        chan bool
	registrationTicker          *time.Ticker
	EventBus                    evbus.Bus
	reqSubscriptionID           string
	jobLifecycleSubscriptionID  string
	taskLifecycleSubscriptionID string
}

// NewBaseTasklet constructor
func NewBaseTasklet(
	idSuffix string,
	cfg *types.CommonConfig,
	queueClient queue.Client,
	queueFilter queue.Filter,
	requestRegistry RequestRegistry,
	requestTopic string,
	registrationTopic string,
	registration *types.AntRegistration,
	executor Executor,
) *BaseTasklet {
	return &BaseTasklet{
		ID:                cfg.ID + idSuffix,
		Config:            cfg,
		QueueClient:       queueClient,
		QueueFilter:       queueFilter,
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
	ctx context.Context) (err error) {
	if t.reqSubscriptionID, err = t.subscribeToIncomingRequests(ctx); err != nil {
		return err
	}
	if t.jobLifecycleSubscriptionID, err = t.subscribeToJobLifecycleEvent(ctx, t.Config.GetJobExecutionLifecycleTopic()); err != nil {
		_ = t.Stop(ctx)
		return err
	}
	if t.taskLifecycleSubscriptionID, err = t.subscribeToTaskLifecycleEvent(ctx, t.Config.GetTaskExecutionLifecycleTopic()); err != nil {
		_ = t.Stop(ctx)
		return err
	}
	if t.registration.AutoRefresh {
		t.startTickerForRegistration(ctx) // renew registration
	}
	logrus.WithFields(
		logrus.Fields{
			"Component":    "BaseTasklet",
			"Tasklet":      t.ID,
			"RequestTopic": t.RequestTopic,
			"Registration": t.registration,
			"AutoRefresh":  t.registration.AutoRefresh,
		}).Info("added ticker for registration and subscribed to receive incoming requests/job/task events")
	return nil
}

// Stop stops subscribing to incoming requests
func (t *BaseTasklet) Stop(
	ctx context.Context) error {
	err1 := t.QueueClient.UnSubscribe(
		ctx,
		t.RequestTopic,
		t.reqSubscriptionID)
	err3 := t.QueueClient.UnSubscribe(
		ctx,
		t.Config.GetJobExecutionLifecycleTopic(),
		t.jobLifecycleSubscriptionID)
	err2 := t.QueueClient.UnSubscribe(
		ctx,
		t.Config.GetTaskExecutionLifecycleTopic(),
		t.taskLifecycleSubscriptionID)
	if t.registrationTicker != nil {
		t.registrationTicker.Stop()
	}
	return cutils.ErrorsAny(err1, err2, err3)
}

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (t *BaseTasklet) handleRequest(
	ctx context.Context,
	taskRequest *types.TaskRequest,
	replyTopic string) (err error) {
	started := time.Now()
	logrus.WithFields(
		logrus.Fields{
			"Component":       "BaseTasklet",
			"Tasklet":         t.ID,
			"RequestTopic":    t.RequestTopic,
			"ResponseTopic":   replyTopic,
			"RequestID":       taskRequest.JobRequestID,
			"Action":          taskRequest.Action,
			"JobType":         taskRequest.JobType,
			"TaskType":        taskRequest.TaskType,
			"TaskExecutionID": taskRequest.TaskExecutionID,
			"CoRelationID":    taskRequest.CoRelationID,
			"Name":            taskRequest.ContainerName(),
		}).Info("<<<<received request>>>>")
	ctx, cancel := context.WithCancel(ctx)
	taskRequest.Cancel = cancel
	defer cancel()

	if taskRequest.Action == types.CANCEL { // async -- no response
		if err := t.RequestRegistry.Cancel(taskRequest.Key()); err != nil {
			logrus.WithFields(
				logrus.Fields{
					"Component":     "BaseTasklet",
					"Tasklet":       t.ID,
					"RequestTopic":  t.RequestTopic,
					"ResponseTopic": replyTopic,
					"RequestID":     taskRequest.JobRequestID,
					"Action":        taskRequest.Action,
					"Request":       taskRequest,
					"CoRelationID":  taskRequest.CoRelationID,
					"Name":          taskRequest.ContainerName(),
					"Error":         err,
				}).Error("failed to cancel request")
		} else {
			logrus.WithFields(
				logrus.Fields{
					"Component":     "BaseTasklet",
					"Tasklet":       t.ID,
					"RequestTopic":  t.RequestTopic,
					"ResponseTopic": replyTopic,
					"RequestID":     taskRequest.JobRequestID,
					"Action":        taskRequest.Action,
					"Request":       taskRequest,
					"Name":          taskRequest.ContainerName(),
					"CoRelationID":  taskRequest.CoRelationID,
				}).Info("cancelled request")
		}
	} else if taskRequest.Action == types.EXECUTE {
		if proceed := t.executor.PreExecute(ctx, taskRequest); !proceed {
			return nil
		}
		if err = t.RequestRegistry.Add(taskRequest); err == nil {
			defer func() {
				if err := t.RequestRegistry.Remove(taskRequest); err != nil {
					if logrus.IsLevelEnabled(logrus.DebugLevel) {
						logrus.WithFields(
							logrus.Fields{
								"Component":     "BaseTasklet",
								"Tasklet":       t.ID,
								"RequestTopic":  t.RequestTopic,
								"ResponseTopic": replyTopic,
								"RequestID":     taskRequest.JobRequestID,
								"Action":        taskRequest.Action,
								"Request":       taskRequest,
								"CoRelationID":  taskRequest.CoRelationID,
								"Name":          taskRequest.ContainerName(),
								"Error":         err,
							}).Debug("failed to remove request")
					}
				}
			}()
			var taskResp *types.TaskResponse
			t.totalExecuted++
			if taskResp, err = t.executor.Execute(ctx, taskRequest); err != nil {
				logrus.WithFields(
					logrus.Fields{
						"Component":     "BaseTasklet",
						"Tasklet":       t.ID,
						"RequestTopic":  t.RequestTopic,
						"ResponseTopic": replyTopic,
						"RequestID":     taskRequest.JobRequestID,
						"Action":        taskRequest.Action,
						"Request":       taskRequest,
						"CoRelationID":  taskRequest.CoRelationID,
						"Name":          taskRequest.ContainerName(),
						"Error":         err,
					}).Warn("failed to execute request")
				taskResp = types.NewTaskResponse(taskRequest)
				taskResp.Status = types.FAILED
				taskResp.ErrorCode = types.ErrorAntExecutionFailed
				taskResp.ErrorMessage = err.Error()
				err = t.sendResponse(ctx, taskRequest, taskResp, replyTopic, started)
			} else {
				err = t.sendResponse(ctx, taskRequest, taskResp, replyTopic, started)
			}
		} else {
			logrus.WithFields(
				logrus.Fields{
					"Component":     "BaseTasklet",
					"Tasklet":       t.ID,
					"RequestTopic":  t.RequestTopic,
					"ResponseTopic": replyTopic,
					"RequestID":     taskRequest.JobRequestID,
					"Action":        taskRequest.Action,
					"Request":       taskRequest,
					"CoRelationID":  taskRequest.CoRelationID,
					"Name":          taskRequest.ContainerName(),
					"Error":         err,
				}).Warn("failed to add request to registry")
		}
	} else if taskRequest.Action == types.TERMINATE {
		var taskResp *types.TaskResponse
		if taskResp, err = t.executor.TerminateContainer(ctx, taskRequest); err != nil {
			logrus.WithFields(
				logrus.Fields{
					"Component":     "BaseTasklet",
					"Tasklet":       t.ID,
					"RequestTopic":  t.RequestTopic,
					"ResponseTopic": replyTopic,
					"RequestID":     taskRequest.JobRequestID,
					"Action":        taskRequest.Action,
					"Request":       taskRequest,
					"CoRelationID":  taskRequest.CoRelationID,
					"Name":          taskRequest.ContainerName(),
					"Error":         err,
				}).Warn("failed to terminate container")
			taskResp = types.NewTaskResponse(taskRequest)
			taskResp.Status = types.FAILED
			taskResp.ErrorCode = types.ErrorAntExecutionFailed
			taskResp.ErrorMessage = err.Error()
			err = t.sendResponse(ctx, taskRequest, taskResp, replyTopic, started)
		} else {
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(
					logrus.Fields{
						"Component":     "BaseTasklet",
						"Tasklet":       t.ID,
						"RequestTopic":  t.RequestTopic,
						"ResponseTopic": replyTopic,
						"RequestID":     taskRequest.JobRequestID,
						"Action":        taskRequest.Action,
						"Request":       taskRequest,
						"Response":      taskResp,
						"CoRelationID":  taskRequest.CoRelationID,
						"Name":          taskRequest.ContainerName(),
					}).Debugf("sending response for terminate container")
			}
			err = t.sendResponse(ctx, taskRequest, taskResp, replyTopic, started)
		}
	} else if taskRequest.Action == types.PING {
		taskResp := types.NewTaskResponse(taskRequest)
		taskResp.Status = types.COMPLETED
		err = t.sendResponse(ctx, taskRequest, taskResp, replyTopic, started)
	} else if taskRequest.Action == types.LIST {
		var taskResp *types.TaskResponse
		if taskResp, err = t.executor.ListContainers(ctx, taskRequest); err != nil {
			logrus.WithFields(
				logrus.Fields{
					"Component":     "BaseTasklet",
					"Tasklet":       t.ID,
					"RequestTopic":  t.RequestTopic,
					"ResponseTopic": replyTopic,
					"RequestID":     taskRequest.JobRequestID,
					"Action":        taskRequest.Action,
					"Request":       taskRequest,
					"CoRelationID":  taskRequest.CoRelationID,
					"Name":          taskRequest.ContainerName(),
					"Error":         err,
				}).Warn("failed to list containers")
			taskResp = types.NewTaskResponse(taskRequest)
			taskResp.Status = types.FAILED
			taskResp.ErrorCode = types.ErrorAntExecutionFailed
			taskResp.ErrorMessage = err.Error()
			err = t.sendResponse(ctx, taskRequest, taskResp, replyTopic, started)
		} else {
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(
					logrus.Fields{
						"Component":     "BaseTasklet",
						"Tasklet":       t.ID,
						"RequestTopic":  t.RequestTopic,
						"ResponseTopic": replyTopic,
						"RequestID":     taskRequest.JobRequestID,
						"Action":        taskRequest.Action,
						"Request":       taskRequest,
						"Response":      taskResp,
						"ExitCode":      taskResp.ExitCode,
						"ErrorCode":     taskResp.ErrorCode,
						"CoRelationID":  taskRequest.CoRelationID,
						"Name":          taskRequest.ContainerName(),
					}).Debugf("sending response for list container")
			}
			err = t.sendResponse(ctx, taskRequest, taskResp, replyTopic, started)
		}
	} else {
		logrus.WithFields(
			logrus.Fields{
				"Component":     "BaseTasklet",
				"Tasklet":       t.ID,
				"RequestTopic":  t.RequestTopic,
				"ResponseTopic": replyTopic,
				"RequestID":     taskRequest.JobRequestID,
				"Action":        taskRequest.Action,
				"CoRelationID":  taskRequest.CoRelationID,
				"Name":          taskRequest.ContainerName(),
				"Request":       taskRequest,
			}).Error("received unknown request")
		taskResp := types.NewTaskResponse(taskRequest)
		taskResp.Status = types.FAILED
		taskResp.ErrorCode = types.ErrorAntExecutionFailed
		taskResp.ErrorMessage = fmt.Sprintf("received unknown action %s", taskRequest.Action)
		err = t.sendResponse(ctx, taskRequest, taskResp, replyTopic, started)
	}
	return
}

func (t *BaseTasklet) sendResponse(
	ctx context.Context,
	taskRequest *types.TaskRequest,
	taskResp *types.TaskResponse,
	responseTopic string,
	started time.Time) error {
	b, err := taskResp.Marshal(t.registration.EncryptionKey)
	fields := logrus.Fields{
		"Component":       "BaseTasklet",
		"Tasklet":         t.ID,
		"TotalExecuted":   t.totalExecuted,
		"Action":          taskRequest.Action,
		"RequestID":       taskResp.JobRequestID,
		"JobType":         taskResp.JobType,
		"TaskType":        taskResp.TaskType,
		"TaskExecutionID": taskResp.TaskExecutionID,
		"CoRelationID":    taskResp.CoRelationID,
		"Status":          taskResp.Status,
		"ExitCode":        taskResp.ExitCode,
		"ErrorCode":       taskResp.ErrorCode,
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
		_, err = t.QueueClient.Send(
			ctx,
			responseTopic,
			b,
			queue.NewMessageHeaders(
				queue.ReusableTopicKey, "false",
				queue.CorrelationIDKey, taskResp.CoRelationID,
				queue.MessageTarget, t.ID,
				"RequestID", taskResp.JobRequestID,
				"TaskType", taskResp.TaskType,
			))
		if err == nil {
			logrus.WithFields(fields).Infof("sent response with %s to %s", taskResp.Status, responseTopic)
		} else {
			fields["Error"] = err
			logrus.WithFields(fields).Warnf("failed to send response to %s", responseTopic)
		}
	}
	return err
}
