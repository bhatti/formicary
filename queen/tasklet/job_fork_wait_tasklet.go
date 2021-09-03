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
	id := serverCfg.ID + "-job-fork-await-tasklet"
	registration := common.AntRegistration{
		AntID:        id,
		AntTopic:     requestTopic,
		MaxCapacity:  serverCfg.Jobs.MaxForkAwaitTaskletCapacity,
		Tags:         []string{},
		Methods:      []common.TaskMethod{common.AwaitForkedJob},
		Allocations:  make(map[uint64]*common.AntAllocation),
		CreatedAt:    time.Now(),
		AntStartedAt: time.Now(),
	}
	t := &JobForkWaitTasklet{
		jobManager: jobManager,
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
		t.jobManager,
		taskReq)
	if err != nil {
		return buildTaskResponseWithError(taskReq, err)
	}
	if err = t.EventBus.Subscribe(common.JobExecutionLifecycleTopic, waiter.UpdateFromJobLifecycleEvent); err != nil {
		return buildTaskResponseWithError(taskReq, fmt.Errorf("failed to subscribe to event bus %v", err))
	}

	defer func() {
		_ = t.EventBus.Unsubscribe(common.JobExecutionLifecycleTopic, waiter.UpdateFromJobLifecycleEvent)
	}()

	sleep := 1 * time.Second
	for {
		done, err := waiter.Poll()
		if err != nil {
			return buildTaskResponseWithError(taskReq, fmt.Errorf("failed to poll job due to %v", err))
		}
		if done {
			break
		}
		_ = waiter.Await(ctx, sleep)
		if sleep*2 <= 16*time.Second {
			sleep *= 2
		}
	}

	taskResp, err = waiter.BuildTaskResponse(taskReq)
	if err == nil {
		logrus.WithFields(
			logrus.Fields{
				"Component":         "JobForkWaitTasklet",
				"RequestIDs":        waiter.requestIDs,
				"CompletedRequests": len(waiter.requests),
				"UserID":            taskReq.UserID,
				"Completed":         waiter.completed(),
				"Elapsed":           time.Since(started),
			}).Info("returning with response")
	} else {
		logrus.WithFields(
			logrus.Fields{
				"Component":         "JobForkWaitTasklet",
				"RequestIDs":        waiter.requestIDs,
				"CompletedRequests": len(waiter.requests),
				"UserID":            taskReq.UserID,
				"Completed":         waiter.completed(),
				"Elapsed":           time.Since(started),
				"Error":             err,
			}).Warnf("returning with error")
	}
	return
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
