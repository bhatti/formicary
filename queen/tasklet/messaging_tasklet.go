package tasklet

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/twinj/uuid"
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
	id := serverCfg.ID + "-job-messaging-tasklet"
	registration := common.AntRegistration{
		AntID:        id,
		AntTopic:     requestTopic,
		MaxCapacity:  serverCfg.Jobs.MaxMessagingTaskletCapacity,
		Tags:         []string{},
		Methods:      []common.TaskMethod{common.Messaging},
		Allocations:  make(map[uint64]*common.AntAllocation),
		CreatedAt:    time.Now(),
		AntStartedAt: time.Now(),
	}
	t := &MessagingTasklet{
		serverCfg:   serverCfg,
		jobManager:  jobManager,
		queueClient: queueClient,
	}

	t.BaseTasklet = tasklet.NewBaseTasklet(
		id,
		&serverCfg.CommonConfig,
		queueClient,
		requestRegistry,
		requestTopic,
		serverCfg.GetRegistrationTopic(),
		registration,
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
	if taskReq.ExecutorOpts.MessagingQueue == "" {
		return buildTaskResponseWithError(taskReq,
			fmt.Errorf("messaging_queue is not specified for %s", taskReq.TaskType))
	}

	var b []byte
	if b, err = taskReq.Marshal(""); err != nil {
		return nil, fmt.Errorf("failed to marshal %s due to %v", taskReq, err)
	}
	var event *queue.MessageEvent
	responseTopic := t.serverCfg.GetResponseTopic(uuid.NewV4().String())
	logrus.WithFields(logrus.Fields{
		"Component":     "MessagingTasklet",
		"RequestTopic":  taskReq.ExecutorOpts.MessagingQueue,
		"ResponseTopic": responseTopic,
	}).
		Infof("sending request")
	if event, err = t.queueClient.SendReceive(
		ctx,
		taskReq.ExecutorOpts.MessagingQueue,
		make(map[string]string),
		b,
		responseTopic); err != nil {
		return nil, err
	}
	taskResp = common.NewTaskResponse(taskReq)
	err = json.Unmarshal(event.Payload, taskResp)
	if err != nil {
		return buildTaskResponseWithError(taskReq, err)
	}

	taskResp.AddContext("MessageQueue", taskReq.ExecutorOpts.MessagingQueue)
	taskResp.AddContext("ResponseQueue", responseTopic)
	return
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
