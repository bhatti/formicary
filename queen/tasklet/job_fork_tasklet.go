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
	"plexobject.com/formicary/queen/types"
)

const forkedJobIDSuffix = ".ForkedJobID"
const forkedJobTypeSuffix = ".ForkedJobType"
const forkedJobVersionSuffix = ".ForkedJobVersion"

// JobForkTasklet structure
type JobForkTasklet struct {
	*tasklet.BaseTasklet
	jobManager *manager.JobManager
}

// NewJobForkTasklet constructor
func NewJobForkTasklet(
	serverCfg *config.ServerConfig,
	requestRegistry tasklet.RequestRegistry,
	jobManager *manager.JobManager,
	queueClient queue.Client,
	requestTopic string,
) *JobForkTasklet {
	id := serverCfg.ID + "-job-fork-tasklet"
	registration := common.AntRegistration{
		AntID:        id,
		AntTopic:     requestTopic,
		MaxCapacity:  serverCfg.Jobs.MaxForkTaskletCapacity,
		Tags:         []string{},
		Methods:      []common.TaskMethod{common.ForkJob},
		Allocations:  make(map[uint64]*common.AntAllocation),
		AutoRefresh:  true,
		CreatedAt:    time.Now(),
		AntStartedAt: time.Now(),
	}
	t := &JobForkTasklet{
		jobManager: jobManager,
	}

	t.BaseTasklet = tasklet.NewBaseTasklet(
		id,
		&serverCfg.CommonConfig,
		queueClient,
		nil,
		requestRegistry,
		requestTopic,
		serverCfg.GetRegistrationTopic(),
		&registration,
		t,
	)
	return t
}

// TerminateContainer terminates container
func (t *JobForkTasklet) TerminateContainer(
	_ context.Context,
	_ *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	return nil, fmt.Errorf("cannot terminate container")
}

// ListContainers list containers
func (t *JobForkTasklet) ListContainers(
	_ context.Context,
	req *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	taskResp = common.NewTaskResponse(req)
	taskResp.AntID = t.ID
	taskResp.Host = "server"
	taskResp.Status = common.COMPLETED
	taskResp.AddContext("containers", make([]*events.ContainerLifecycleEvent, 0))
	return
}

// PreExecute check if request can be executed
func (t *JobForkTasklet) PreExecute(
	_ context.Context,
	_ *common.TaskRequest) bool {
	return true
}

// Execute task request
func (t *JobForkTasklet) Execute(
	_ context.Context,
	taskReq *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	queryContext := common.NewQueryContextFromIDs(taskReq.UserID, taskReq.OrganizationID)
	if taskReq.ExecutorOpts.ForkJobType == "" {
		return taskReq.ErrorResponse(fmt.Errorf("fork_job_type is not specified for job %s and request %d", taskReq.JobType, taskReq.JobRequestID)), nil
	}
	jobDef, err := t.jobManager.GetJobDefinitionByType(
		queryContext,
		taskReq.ExecutorOpts.ForkJobType,
		taskReq.ExecutorOpts.ForkJobVersion)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":      "JobForkTasklet",
			"Request":        taskReq,
			"ForkJobType":    taskReq.ExecutorOpts.ForkJobType,
			"ForkJobVersion": taskReq.ExecutorOpts.ForkJobVersion,
			"Error":          err,
		}).Warnf("failed to find plugin to fork")
		return taskReq.ErrorResponse(err), nil
	}
	req, err := types.NewJobRequestFromDefinition(jobDef)
	if err != nil {
		return taskReq.ErrorResponse(err), nil
	}
	req.JobVersion = taskReq.ExecutorOpts.ForkJobVersion
	for k, v := range taskReq.Variables {
		if !v.Secret {
			_, _ = req.AddParam(k, v.Value)
		}
	}
	_, _ = req.AddParam(common.ForkedJob, true)
	req.ParentID = taskReq.JobRequestID
	_, _ = req.AddParam(fmt.Sprintf("%s_%d", types.ParentJobTypePrefix, req.ParentID), taskReq.JobType)

	saved, err := t.jobManager.SaveJobRequest(
		common.NewQueryContextFromIDs(taskReq.UserID, taskReq.OrganizationID),
		req)
	if err != nil {
		return taskReq.ErrorResponse(err), nil
	}

	taskResp = common.NewTaskResponse(taskReq)
	taskResp.AntID = t.ID
	taskResp.Host = "server"
	taskResp.Status = common.COMPLETED
	taskResp.AddContext(taskReq.TaskType+forkedJobIDSuffix, saved.ID)
	taskResp.AddContext(taskReq.TaskType+forkedJobTypeSuffix, saved.JobType)
	taskResp.AddContext(taskReq.TaskType+forkedJobVersionSuffix, saved.JobVersion)
	taskResp.AddJobContext(taskReq.TaskType+forkedJobIDSuffix, saved.ID)
	return
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
