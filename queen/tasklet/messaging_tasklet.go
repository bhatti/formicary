package tasklet

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/events"
	"time"

	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tasklet"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
)

// MessagingTasklet structure
type MessagingTasklet struct {
	*tasklet.BaseTasklet
	serverCfg   *config.ServerConfig
	jobManager  *manager.JobManager
	queueClient queue.Client
}

// NewMessagingTasklet constructor
func NewMessagingTasklet(
	serverCfg *config.ServerConfig,
	requestRegistry tasklet.RequestRegistry,
	jobManager *manager.JobManager,
	queueClient queue.Client,
	requestTopic string,
) *MessagingTasklet {
	id := serverCfg.Common.ID + "-job-messaging-tasklet"
	registration := common.AntRegistration{
		AntID:         id,
		AntTopic:      requestTopic,
		AutoRefresh:   true,
		MaxCapacity:   serverCfg.Jobs.MaxMessagingTaskletCapacity,
		Tags:          []string{},
		EncryptionKey: serverCfg.Jobs.MessagingEncryptionKey,
		Methods:       []common.TaskMethod{common.Messaging},
		Allocations:   make(map[uint64]*common.AntAllocation),
		CreatedAt:     time.Now(),
		AntStartedAt:  time.Now(),
	}
	t := &MessagingTasklet{
		serverCfg:   serverCfg,
		jobManager:  jobManager,
		queueClient: queueClient,
	}

	t.BaseTasklet = tasklet.NewBaseTasklet(
		id,
		&serverCfg.Common,
		queueClient,
		nil,
		requestRegistry,
		requestTopic,
		serverCfg.Common.GetRegistrationTopic(),
		&registration,
		t,
	)
	return t
}

// TerminateContainer terminates container
func (t *MessagingTasklet) TerminateContainer(
	_ context.Context,
	_ *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	return nil, fmt.Errorf("cannot terminate container")
}

// ListContainers list containers
func (t *MessagingTasklet) ListContainers(
	_ context.Context,
	req *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	taskResp = common.NewTaskResponse(req)
	taskResp.Status = common.COMPLETED
	taskResp.AddContext("containers", make([]*events.ContainerLifecycleEvent, 0))
	return
}

// PreExecute check if request can be executed
func (t *MessagingTasklet) PreExecute(
	_ context.Context,
	_ *common.TaskRequest) bool {
	return true
}

// Execute task request
func (t *MessagingTasklet) Execute(
	ctx context.Context,
	taskReq *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	if taskReq.ExecutorOpts.MessagingRequestQueue == "" {
		return taskReq.ErrorResponse(
			fmt.Errorf("messaging_request_queue is not specified for %s", taskReq.TaskType)), nil
	}
	if taskReq.ExecutorOpts.MessagingReplyQueue == "" {
		return taskReq.ErrorResponse(
			fmt.Errorf("messaging_reply_queue is not specified for %s", taskReq.TaskType)), nil
	}

	var b []byte
	if b, err = taskReq.Marshal(""); err != nil {
		return nil, fmt.Errorf("failed to marshal %s due to %w", taskReq, err)
	}
	var event *queue.MessageEvent
	logrus.WithFields(logrus.Fields{
		"Component":     "MessagingTasklet",
		"RequestTopic":  taskReq.ExecutorOpts.MessagingRequestQueue,
		"ResponseTopic": taskReq.ExecutorOpts.MessagingReplyQueue,
	}).
		Infof("sending request")
	if event, err = t.queueClient.SendReceive(
		ctx,
		taskReq.ExecutorOpts.MessagingRequestQueue,
		b,
		taskReq.ExecutorOpts.MessagingReplyQueue,
		make(map[string]string),
	); err != nil {
		return nil, err
	}
	if event == nil {
		logrus.WithFields(logrus.Fields{
			"Component":     "MessagingTasklet",
			"RequestTopic":  taskReq.ExecutorOpts.MessagingRequestQueue,
			"ResponseTopic": taskReq.ExecutorOpts.MessagingReplyQueue,
			"Request":       taskReq,
		}).
			Errorf("failed to receive reply")
		return nil, fmt.Errorf("failed to receive reply from " + taskReq.ExecutorOpts.MessagingReplyQueue)
	}
	taskResp, err = common.UnmarshalTaskResponse(t.serverCfg.Jobs.MessagingEncryptionKey, event.Payload)
	if err != nil {
		return taskReq.ErrorResponse(err), nil
	}
	taskResp.Status = common.COMPLETED
	if taskResp.AntID == "" {
		taskResp.AntID = t.ID
	}
	if taskResp.Host == "" {
		taskResp.Host = "server"
	}
	taskResp.AddContext("MessageQueue", taskReq.ExecutorOpts.MessagingRequestQueue)
	taskResp.AddContext("ResponseQueue", taskReq.ExecutorOpts.MessagingReplyQueue)
	return
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
