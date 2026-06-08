// SPDX-License-Identifier: AGPL-3.0-or-later

package tasklet

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tasklet"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/types"
	"plexobject.com/formicary/queen/utils"
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
	id := serverCfg.Common.ID + "-job-fork-tasklet"
	registration := common.AntRegistration{
		AntID:        id,
		AntTopic:     requestTopic,
		MaxCapacity:  serverCfg.Jobs.MaxForkTaskletCapacity,
		Tags:         []string{},
		Methods:      []common.TaskMethod{common.ForkJob},
		Allocations:  make(map[string]*common.AntAllocation),
		AutoRefresh:  true,
		CreatedAt:    time.Now(),
		AntStartedAt: time.Now(),
	}
	t := &JobForkTasklet{
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
	ctx context.Context,
	taskReq *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	queryContext := common.NewQueryContextFromIDs(taskReq.UserID, taskReq.OrganizationID)
	if taskReq.ExecutorOpts.ForkJobType == "" {
		return taskReq.ErrorResponse(fmt.Errorf("fork_job_type is not specified for job %s and request %s",
			taskReq.JobType, taskReq.JobRequestID)), nil
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

	// Build child job params from sub_workflow.input_params.
	// Each entry: Name = child param name, Value = Go template expression resolved
	// against the parent's non-secret variables. Secrets are excluded from template data.
	// If input_params is absent or empty, no parent variables are forwarded to the child.
	sw := taskReq.ExecutorOpts.SubWorkflow
	if sw != nil && len(sw.InputParams) > 0 {
		templateData := common.VariableValuesToMap(common.MaskVariableValues(taskReq.Variables))
		inputMap, mapErr := sw.InputMap()
		if mapErr != nil {
			return taskReq.ErrorResponse(mapErr), nil
		}
		for childParam, templateExpr := range inputMap {
			resolved, resolveErr := utils.ParseTemplate(templateExpr, templateData)
			if resolveErr != nil {
				return taskReq.ErrorResponse(
					fmt.Errorf("sub_workflow.input_params: failed to resolve param '%s' (expr: %q): %w",
						childParam, templateExpr, resolveErr)), nil
			}
			if _, addErr := req.AddParam(childParam, resolved); addErr != nil {
				return taskReq.ErrorResponse(
					fmt.Errorf("sub_workflow.input_params: failed to set param '%s': %w",
						childParam, addErr)), nil
			}
		}
		logrus.WithFields(logrus.Fields{
			"Component":      "JobForkTasklet",
			"ParentJobType":  taskReq.JobType,
			"ForkJobType":    taskReq.ExecutorOpts.ForkJobType,
			"InputParamCount": len(sw.InputParams),
			"UserID":         taskReq.UserID,
		}).Info("sub_workflow: resolved input_params for child job")
	}

	_, _ = req.AddParam(common.ForkedJob, true)
	req.ParentID = taskReq.JobRequestID
	_, _ = req.AddParam(fmt.Sprintf("%s_%s", types.ParentJobTypePrefix, req.ParentID), taskReq.JobType)

	// Always cascade cancel: a forked child is always cancelled when its parent is cancelled.
	req.CascadeCancel = true

	saved, err := t.jobManager.SaveJobRequest(
		common.NewQueryContextFromIDs(taskReq.UserID, taskReq.OrganizationID),
		req)
	if err != nil {
		return taskReq.ErrorResponse(err), nil
	}

	logrus.WithFields(logrus.Fields{
		"Component":       "JobForkTasklet",
		"ParentJobType":   taskReq.JobType,
		"ParentRequestID": taskReq.JobRequestID,
		"ChildJobType":    saved.JobType,
		"ChildRequestID":  saved.ID,
		"CascadeCancel":   req.CascadeCancel,
		"WaitForCompletion":   sw != nil && sw.WaitForCompletion,
		"UserID":          taskReq.UserID,
	}).Info("forked child job request created")

	// When wait_for_completion is set, block inline until the child finishes.
	if sw != nil && sw.WaitForCompletion {
		return t.waitForChild(ctx, taskReq, saved.ID)
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

// waitForChild subscribes to lifecycle events and polls until the child job reaches a terminal state.
// It reuses JobWaiter exactly as JobForkWaitTasklet does, then applies the output_map if configured.
func (t *JobForkTasklet) waitForChild(
	ctx context.Context,
	taskReq *common.TaskRequest,
	childRequestID string,
) (taskResp *common.TaskResponse, err error) {
	// Inject the child job ID into variables so that JobWaiter can find it via the
	// standard "{taskType}.ForkedJobID" key pattern.
	taskReq.Variables[taskReq.TaskType+forkedJobIDSuffix] = common.NewVariableValue(childRequestID, false)
	taskReq.ExecutorOpts.AwaitForkedTasks = []string{taskReq.TaskType}

	waiter, err := NewJobWaiter(ctx, t.ID, t.jobManager, taskReq)
	if err != nil {
		return taskReq.ErrorResponse(err), nil
	}

	started := time.Now()
	waitCtx := ctx
	var cancelWait context.CancelFunc
	if taskReq.Timeout > 0 {
		waitCtx, cancelWait = context.WithTimeout(ctx, taskReq.Timeout)
		defer cancelWait()
	}
	topic := t.Config.GetJobExecutionLifecycleTopic()
	if waitErr := waiter.RunAndWait(waitCtx,
		func(h func(*events.JobExecutionLifecycleEvent) error) error {
			return t.EventBus.Subscribe(topic, h)
		},
		func(h func(*events.JobExecutionLifecycleEvent) error) {
			_ = t.EventBus.Unsubscribe(topic, h)
		},
	); waitErr != nil {
		return taskReq.ErrorResponse(
			fmt.Errorf("sub_workflow wait_for_completion: %w", waitErr)), nil
	}

	taskResp, err = waiter.BuildTaskResponse(taskReq)
	if err == nil {
		taskResp.AddContext("RequestIDs", waiter.RequestIDs())
		taskResp.AddContext("TotalRequests", len(waiter.RequestIDs()))
		logrus.WithFields(logrus.Fields{
			"Component":      "JobForkTasklet",
			"ChildRequestID": childRequestID,
			"Status":         taskResp.Status,
			"Elapsed":        time.Since(started),
			"UserID":         taskReq.UserID,
		}).Info("sub_workflow: child job completed")
	} else {
		logrus.WithFields(logrus.Fields{
			"Component":      "JobForkTasklet",
			"ChildRequestID": childRequestID,
			"Elapsed":        time.Since(started),
			"Error":          err,
		}).Warn("sub_workflow: child job wait failed")
	}
	return
}

