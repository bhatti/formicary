package tasklet

import (
	"context"
	"fmt"
	"plexobject.com/formicary/internal/events"
	"time"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tasklet"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
)

// JobForkWaitTasklet structure
type JobForkWaitTasklet struct {
	*tasklet.BaseTasklet
	jobManager *manager.JobManager
}

// NewJobForkWaitTasklet constructor
func NewJobForkWaitTasklet(
	serverCfg *config.ServerConfig,
	requestRegistry tasklet.RequestRegistry,
	jobManager *manager.JobManager,
	queueClient queue.Client,
	requestTopic string) *JobForkWaitTasklet {
	id := serverCfg.Common.ID + "-job-fork-await-tasklet"
	registration := common.AntRegistration{
		AntID:        id,
		AntTopic:     requestTopic,
		MaxCapacity:  serverCfg.Jobs.MaxForkAwaitTaskletCapacity,
		Tags:         []string{},
		Methods:      []common.TaskMethod{common.AwaitForkedJob},
		Allocations:  make(map[string]*common.AntAllocation),
		AutoRefresh:  true,
		CreatedAt:    time.Now(),
		AntStartedAt: time.Now(),
	}
	t := &JobForkWaitTasklet{
		jobManager: jobManager,
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
func (t *JobForkWaitTasklet) TerminateContainer(
	_ context.Context,
	_ *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	return nil, fmt.Errorf("cannot terminate container")
}

// ListContainers list containers
func (t *JobForkWaitTasklet) ListContainers(
	_ context.Context,
	req *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	taskResp = common.NewTaskResponse(req)
	taskResp.Status = common.COMPLETED
	taskResp.AntID = t.ID
	taskResp.Host = "server"
	taskResp.AddContext("containers", make([]*events.ContainerLifecycleEvent, 0))
	return
}

// PreExecute checks if request can be executed
func (t *JobForkWaitTasklet) PreExecute(
	_ context.Context,
	_ *common.TaskRequest) bool {
	return true
}

// Execute request
func (t *JobForkWaitTasklet) Execute(
	ctx context.Context,
	taskReq *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	started := time.Now()
	waiter, err := NewJobWaiter(
		ctx,
		t.ID,
		t.jobManager,
		taskReq)
	if err != nil {
		return taskReq.ErrorResponse(err), nil
	}
	waitCtx := ctx
	var cancelWait context.CancelFunc
	if taskReq.Timeout > 0 {
		waitCtx, cancelWait = context.WithTimeout(ctx, taskReq.Timeout)
		defer cancelWait()
	}
	topic := t.Config.GetJobExecutionLifecycleTopic()
	if err = waiter.RunAndWait(waitCtx,
		func(h func(*events.JobExecutionLifecycleEvent) error) error {
			return t.EventBus.Subscribe(topic, h)
		},
		func(h func(*events.JobExecutionLifecycleEvent) error) {
			_ = t.EventBus.Unsubscribe(topic, h)
		},
	); err != nil {
		return taskReq.ErrorResponse(fmt.Errorf("failed to wait for forked jobs: %w", err)), nil
	}

	taskResp, err = waiter.BuildTaskResponse(taskReq)
	if err == nil {
		taskResp.AddContext("RequestIDs", waiter.RequestIDs())
		taskResp.AddContext("TotalRequests", len(waiter.RequestIDs()))
		logrus.WithFields(
			logrus.Fields{
				"Component":         "JobForkWaitTasklet",
				"RequestIDs":        waiter.RequestIDs(),
				"CompletedRequests": len(waiter.RequestIDs()),
				"UserID":            taskReq.UserID,
				"Elapsed":           time.Since(started),
			}).Info("returning with response")
	} else {
		logrus.WithFields(
			logrus.Fields{
				"Component":         "JobForkWaitTasklet",
				"RequestIDs":        waiter.RequestIDs(),
				"CompletedRequests": len(waiter.RequestIDs()),
				"UserID":            taskReq.UserID,
				"Elapsed":           time.Since(started),
				"Error":             err,
			}).Warnf("returning with error")
	}
	return
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
