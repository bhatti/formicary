package fsm

import (
	"context"
	"fmt"
	"time"

	"plexobject.com/formicary/internal/utils"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/events"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

// TaskExecutionStateMachine for managing state of task and its execution
type TaskExecutionStateMachine struct {
	*JobExecutionStateMachine
	taskType          string
	TaskDefinition    *types.TaskDefinition
	ExecutorOptions   *common.ExecutorOptions
	TaskExecution     *types.TaskExecution
	LastTaskExecution *types.TaskExecution
	Reservation       *common.AntReservation
}

// NewTaskExecutionStateMachine creates new state machine for request execution
func NewTaskExecutionStateMachine(
	jobStateMachine *JobExecutionStateMachine,
	taskType string) (tsm *TaskExecutionStateMachine, err error) {

	tsm = &TaskExecutionStateMachine{
		JobExecutionStateMachine: jobStateMachine,
		taskType:                 taskType,
	}

	tsm.TaskExecution = tsm.JobExecution.GetTask(tsm.taskType)

	// Load task definition using job params because task is not built yet
	if tsm.TaskDefinition, tsm.ExecutorOptions, err = tsm.JobDefinition.GetDynamicTask(
		tsm.taskType,
		tsm.JobExecutionStateMachine.buildDynamicParams(nil)); err != nil {
		return nil, err
	}

	// If this is continuation from previous job execution and task is already completed, then simply return it
	if tsm.TaskExecution != nil {
		if tsm.TaskExecution.TaskState.Completed() {
			return
		}
		// otherwise let's remove last incomplete or failed task
		if err = tsm.JobManager.DeleteExecutionTask(tsm.TaskExecution.ID); err != nil {
			return nil, fmt.Errorf("failed to delete old task due to %v", err)
		}
	}

	// create new task execution
	tsm.TaskExecution = tsm.JobExecution.AddTask(tsm.TaskDefinition)
	if _, err = tsm.JobManager.SaveExecutionTask(tsm.TaskExecution); err != nil {
		tsm.TaskExecution.TaskState = common.FAILED
		tsm.TaskExecution.ErrorMessage = err.Error()
		return nil, err
	}
	return
}

// PrepareExecution - initializes task definition, execution and ant allocation
func (tsm *TaskExecutionStateMachine) PrepareExecution(
	ctx context.Context) (err error) {
	if err = ctx.Err(); err != nil {
		return err
	}

	if tsm.Reservation = tsm.Reservations[tsm.taskType]; tsm.Reservation == nil {
		logrus.WithFields(tsm.LogFields(
			"TaskExecutionStateMachine")).
			Warnf("reservations not found for %s in %v",
				tsm.taskType, tsm.Reservations)
		return fmt.Errorf("no ant found to execute the task '%s' "+
			" because matching ants not found in reservations (%d)",
			tsm.taskType, len(tsm.Reservations))
	}

	// validate ant allocation
	// TODO this can be retried --- in case ant disappear temporarily
	if tsm.Reservation, err = tsm.validateAntAllocation(
		tsm.TaskDefinition,
		tsm.Reservation); err != nil {
		tsm.TaskExecution.TaskState = common.FAILED
		tsm.TaskExecution.ErrorMessage = err.Error()
		tsm.TaskExecution.ErrorCode = common.ErrorAntsUnavailable
		return fmt.Errorf("failed to validate ant reservation for task '%s' "+
			" due to %v", tsm.taskType, err)
	}

	// load last executed task if exists
	if tsm.LastJobExecution != nil {
		tsm.LastTaskExecution = tsm.LastJobExecution.GetTask(
			tsm.TaskDefinition.TaskType)
	}

	// save ant-id and topic
	tsm.TaskExecution.AntID = tsm.Reservation.AntID
	_, _ = tsm.TaskExecution.AddContext("AntTopic", tsm.Reservation.AntTopic)
	_, _ = tsm.TaskExecution.AddContext("AntID", tsm.Reservation.AntID)

	return nil
}

// TaskKey creates unique key for the task
func (tsm *TaskExecutionStateMachine) TaskKey() string {
	return common.TaskKey(tsm.Request.GetID(), tsm.taskType)
}

// SetTaskToExecuting - Set task to EXECUTING from READY
func (tsm *TaskExecutionStateMachine) SetTaskToExecuting(
	ctx context.Context) error {
	tsm.TaskExecution.TaskState = common.EXECUTING
	saveError := tsm.JobManager.UpdateTaskExecutionState(
		tsm.TaskExecution.ID,
		common.READY,
		common.EXECUTING)

	// treating error sending task lifecycle event as non-fatal error
	if eventError := tsm.sendTaskExecutionLifecycleEvent(ctx); eventError != nil {
		logrus.WithFields(tsm.LogFields(
			"TaskExecutionStateMachine",
			eventError,
			saveError)).
			Warn("failed to send task lifecycle event after setting status to EXECUTING")
	}
	return saveError
}

// FinalizeTaskState - saves task (completed or failed) and updates job execution context
func (tsm *TaskExecutionStateMachine) FinalizeTaskState(
	ctx context.Context) (err error) {
	now := time.Now()
	if tsm.TaskExecution.TaskState.Completed() {
		tsm.TaskExecution.ErrorCode = ""
		tsm.TaskExecution.ErrorMessage = ""
	}

	tsm.TaskExecution.EndedAt = &now
	// optionally release resource if completed
	//if tsm.TaskExecution.TaskState.Completed() {
	//	delete(tsm.Reservations, tsm.TaskDefinition.TaskType)
	//}

	// SaveFile job context from task result
	_ = tsm.JobManager.UpdateJobExecutionContext(
		tsm.JobExecution.ID,
		tsm.JobExecution.Contexts)

	// we will return save error at the end
	_, err = tsm.JobManager.SaveExecutionTask(tsm.TaskExecution)

	// treating error sending lifecycle event as non-fatal error
	if eventError := tsm.sendTaskExecutionLifecycleEvent(ctx); eventError != nil {
		logrus.WithFields(tsm.LogFields("TaskExecutionStateMachine",
			eventError)).
			Warn("failed to send task lifecycle event after finishing task")
	}
	//debug.PrintStack()
	return
}

// CanReusePreviousResult returns true if previously executed task can be reused
func (tsm *TaskExecutionStateMachine) CanReusePreviousResult() bool {
	return !tsm.DoesRequireFullRestart() && tsm.LastTaskExecution != nil &&
		tsm.LastTaskExecution.TaskState.Completed()
}

// BuildTaskRequest - create a new task request
func (tsm *TaskExecutionStateMachine) BuildTaskRequest() (*common.TaskRequest, error) {
	// Add dependent artifacts if exist
	tsm.ExecutorOptions.DependentArtifactIDs = tsm.TaskDefinition.ArtifactIDs
	// find all dependent artifacts
	for _, dep := range tsm.TaskDefinition.Dependencies {
		matched := false
		for _, task := range tsm.JobExecution.Tasks {
			if dep == task.TaskType {
				matched = true
				for _, art := range task.Artifacts {
					if art.Kind == common.ArtifactKindTask {
						tsm.ExecutorOptions.DependentArtifactIDs =
							append(tsm.ExecutorOptions.DependentArtifactIDs, art.ID)
					}
				}
				break
			}
		}

		if !matched {
			return nil, fmt.Errorf("failed to find artifacts from dependent task '%s' for task '%s'",
				dep, tsm.TaskDefinition.TaskType)
		}
	}

	taskReq := &common.TaskRequest{
		UserID:          tsm.Request.GetUserID(),
		OrganizationID:  tsm.Request.GetOrganizationID(),
		JobDefinitionID: tsm.JobDefinition.ID,
		JobRequestID:    tsm.Request.GetID(),
		JobType:         tsm.Request.GetJobType(),
		JobTypeVersion:  tsm.TaskDefinition.JobVersion,
		JobExecutionID:  tsm.JobExecution.ID,
		TaskExecutionID: tsm.TaskExecution.ID,
		TaskType:        tsm.TaskDefinition.TaskType,
		Platform:        tsm.JobDefinition.Platform,
		ResponseTopic:   tsm.serverCfg.GetResponseTopic(tsm.TaskKey()),
		Action:          common.EXECUTE,
		Retry:           tsm.TaskExecution.Retried,
		AllowFailure:    tsm.TaskDefinition.AllowFailure,
		Tags:            tsm.TaskDefinition.Tags,
		BeforeScript:    tsm.TaskDefinition.BeforeScript,
		Script:          tsm.TaskDefinition.Script,
		AfterScript:     tsm.TaskDefinition.AfterScript,
		Timeout:         tsm.TaskDefinition.Timeout,
		Variables:       tsm.buildDynamicParams(),
		SecretConfigs:   tsm.buildSecretConfigs(),
		ExecutorOpts:    tsm.ExecutorOptions,
		StartedAt:       time.Now(),
	}

	if tsm.TaskDefinition.JobVersion != "" && tsm.ExecutorOptions.ForkJobVersion == "" {
		tsm.ExecutorOptions.ForkJobVersion = tsm.TaskDefinition.JobVersion
	}
	// TODO check default
	//if tsm.TaskDefinition.HostNetwork == "" {
	//	tsm.ExecutorOptions.HostNetwork = true
	//}
	// override name of main container in executor options
	taskReq.ExecutorOpts.Name = utils.MakeDNS1123Compatible(
		fmt.Sprintf("formicary-%d-%s-%d",
			tsm.Request.GetID(),
			tsm.TaskDefinition.TaskType,
			tsm.TaskExecution.Retried))
	taskReq.ExecutorOpts.PodLabels[common.RequestID] = fmt.Sprintf("%d", tsm.Request.GetID())
	taskReq.ExecutorOpts.PodLabels[common.UserID] = tsm.Request.GetUserID()
	taskReq.ExecutorOpts.PodLabels[common.OrgID] = tsm.Request.GetOrganizationID()
	taskReq.ExecutorOpts.PodLabels["FormicaryServer"] = tsm.serverCfg.ID

	if taskReq.Variables["debug"] == "true" || taskReq.Variables["debug"] == true {
		taskReq.ExecutorOpts.Debug = true
	}

	if taskReq.ExecutorOpts.ArtifactsDirectory == "" {
		taskReq.ExecutorOpts.ArtifactsDirectory =
			fmt.Sprintf("/tmp/formicary-artifacts/%s", taskReq.KeyPath())
	}

	if taskReq.ExecutorOpts.CacheDirectory == "" {
		taskReq.ExecutorOpts.CacheDirectory =
			fmt.Sprintf("/tmp/formicary-cache/%s", taskReq.KeyPath())
	}
	// Note: we will download cache on ant-worker side because it may require accessing key-files

	return taskReq, taskReq.Validate()
}

// BuildTaskResponseFromPreviousResult - create a new task response from old result
func (tsm *TaskExecutionStateMachine) BuildTaskResponseFromPreviousResult() (*common.TaskResponse, error) {
	taskReq, err := tsm.BuildTaskRequest()
	if err != nil {
		return nil, err
	}
	taskResp := common.NewTaskResponse(taskReq)
	taskResp.AntID = tsm.LastTaskExecution.AntID
	taskResp.Host = tsm.LastTaskExecution.AntHost
	taskResp.ExitCode = tsm.LastTaskExecution.ExitCode
	taskResp.ExitMessage = tsm.LastTaskExecution.ExitMessage

	taskResp.Status = tsm.LastTaskExecution.TaskState
	taskResp.ErrorCode = ""
	taskResp.ErrorMessage = ""

	// adding contexts
	for _, c := range tsm.LastTaskExecution.Contexts {
		if val, err := c.GetParsedValue(); err == nil {
			taskResp.AddContext(c.Name, val)
		}
	}

	for _, artifact := range tsm.LastTaskExecution.Artifacts {
		taskResp.AddArtifact(artifact)
	}

	return taskResp, nil
}

// SetFailed marks task execution as failed
func (tsm *TaskExecutionStateMachine) SetFailed(err error) {
	tsm.TaskExecution.TaskState = common.FAILED
	if tsm.TaskExecution.ErrorCode == "" {
		tsm.TaskExecution.ErrorCode = common.ErrorTaskExecute
	}
	tsm.TaskExecution.ErrorMessage = err.Error()
}

// CanRetry checks if task can be retried in case of failure
func (tsm *TaskExecutionStateMachine) CanRetry() bool {
	return tsm.TaskExecution.Retried < tsm.TaskDefinition.Retry+1 ||
		(tsm.errorCode != nil && tsm.errorCode.Action == common.RetryTask &&
			tsm.TaskExecution.Retried < tsm.errorCode.Retry+1)
}

// UpdateTaskFromResponse updates task execution from response
func (tsm *TaskExecutionStateMachine) UpdateTaskFromResponse(
	taskReq *common.TaskRequest,
	taskResp *common.TaskResponse) (err error) {
	for k, v := range taskResp.TaskContext {
		_, _ = tsm.TaskExecution.AddContext(k, v)
	}
	for k, v := range taskResp.JobContext {
		_, _ = tsm.JobExecution.AddContext(k, v)
	}

	tsm.TaskExecution.AntID = taskResp.AntID
	tsm.TaskExecution.AntHost = taskResp.Host
	tsm.TaskExecution.ExitCode = taskResp.ExitCode
	tsm.TaskExecution.ExitMessage = taskResp.ExitMessage
	tsm.TaskExecution.AppliedCost = taskResp.AppliedCost
	tsm.TaskExecution.CountServices = len(taskReq.ExecutorOpts.Services)

	// save status and error code/messages
	tsm.TaskExecution.TaskState = taskResp.Status
	if taskResp.Status == common.COMPLETED {
		tsm.TaskExecution.ErrorCode = ""
		tsm.TaskExecution.ErrorMessage = ""
	} else {
		if tsm.errorCode, err = tsm.ErrorCodeRepository.Match(
			taskResp.ErrorMessage,
			tsm.JobDefinition.Platform,
			tsm.JobDefinition.JobType,
			tsm.taskType); err == nil {
			taskResp.ErrorCode = tsm.errorCode.ErrorCode
		}
		if taskResp.ErrorCode == "" {
			taskResp.ErrorCode = common.ErrorTaskExecute
		}
		tsm.TaskExecution.ErrorCode = taskResp.ErrorCode
		tsm.TaskExecution.ErrorMessage = taskResp.ErrorMessage
		err = fmt.Errorf("ant failed to execute task '%s' due to %s",
			tsm.taskType, taskResp.ErrorMessage)
	}

	for i, artifact := range taskResp.Artifacts {
		oldArtifact, err := tsm.ArtifactManager.GetArtifact(
			context.Background(),
			tsm.QueryContext(),
			artifact.ID)
		if err == nil {
			for k, v := range artifact.Metadata {
				oldArtifact.AddMetadata(k, v)
			}
			artifact = oldArtifact
		}
		if tsm.ExecutorOptions.Method == common.ForkJob {
			artifact.TaskType = fmt.Sprintf("%d::%s", artifact.JobRequestID, artifact.TaskType)
		} else {
			artifact.TaskType = tsm.TaskExecution.TaskType
		}

		artifact.UserID = tsm.Request.GetUserID()
		artifact.OrganizationID = tsm.Request.GetOrganizationID()
		artifact.JobRequestID = tsm.Request.GetID()
		artifact.JobExecutionID = tsm.JobExecution.ID
		artifact.TaskExecutionID = tsm.TaskExecution.ID
		artifact.Group = tsm.Request.GetGroup()
		artifact.AddMetadata("status", string(taskResp.Status))

		if _, saveErr := tsm.ArtifactManager.UpdateArtifact(
			context.Background(),
			tsm.QueryContext(),
			artifact); saveErr != nil {
			logrus.WithFields(
				logrus.Fields{
					"Component": "TaskExecutionStateMachine",
					"Error":     saveErr,
					"Response":  taskResp,
					"Artifact":  artifact,
				}).Error("failed to save artifact")
			taskResp.AdditionalError(fmt.Sprintf(
				"failed to save artifact %v due to '%v'",
				artifact, saveErr), true)
		} else {
			artifactContextKey := fmt.Sprintf("%s_ArtifactURL_%d", tsm.taskType, i+1)
			_, _ = tsm.JobExecution.AddContext(artifactContextKey, artifact.URL)
			tsm.TaskExecution.AddArtifact(artifact)
		}
	}

	if len(taskResp.Warnings) > 0 {
		_, _ = tsm.TaskExecution.AddContext("Warnings", taskResp.Warnings)
	}
	_, _ = tsm.TaskExecution.AddContext("Timings", taskResp.Timings.String())

	return
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////

func (tsm *TaskExecutionStateMachine) validateAntAllocation(
	taskDefinition *types.TaskDefinition,
	allocation *common.AntReservation) (_ *common.AntReservation, err error) {
	// verify ant is still connected
	ant := tsm.ResourceManager.Registration(allocation.AntID)
	if ant == nil {
		// find another ant
		return tsm.ResourceManager.Reserve(
			tsm.Request.GetID(),
			tsm.TaskExecution.TaskType,
			taskDefinition.Method,
			taskDefinition.Tags)
	}
	if !ant.Supports(taskDefinition.Method, taskDefinition.Tags) {
		_ = tsm.ResourceManager.Release(allocation)
		return tsm.ResourceManager.Reserve(
			tsm.Request.GetID(),
			tsm.TaskExecution.TaskType,
			taskDefinition.Method,
			taskDefinition.Tags)
	}
	return allocation, nil
}

func (tsm *TaskExecutionStateMachine) buildDynamicParams() map[string]interface{} {
	res := tsm.JobExecutionStateMachine.buildDynamicParams(tsm.TaskDefinition.NameValueVariables.(map[string]interface{}))
	res["TaskType"] = tsm.taskType
	return res
}

// Fire event to notify task state
func (tsm *TaskExecutionStateMachine) sendTaskExecutionLifecycleEvent(
	ctx context.Context) (err error) {
	event := events.NewTaskExecutionLifecycleEvent(
		tsm.serverCfg.ID,
		tsm.Request.GetUserID(),
		tsm.Request.GetID(),
		tsm.Request.GetJobType(),
		tsm.JobExecution.ID,
		tsm.TaskExecution.TaskType,
		tsm.TaskExecution.TaskState,
		tsm.TaskExecution.ExitCode,
		tsm.TaskExecution.AntID,
		tsm.TaskExecution.ContextMap(),
	)
	var payload []byte
	if payload, err = event.Marshal(); err != nil {
		return fmt.Errorf("failed to marshal task-execution event due to %v", err)
	}
	if _, err = tsm.QueueClient.Publish(ctx,
		tsm.serverCfg.GetTaskExecutionLifecycleTopic(),
		make(map[string]string),
		payload,
		true); err != nil {
		return fmt.Errorf("failed to send task-execution event due to %v", err)
	}
	return nil
}

// LogFields for logging
func (tsm *TaskExecutionStateMachine) LogFields(
	component string,
	err ...error) logrus.Fields {
	fields := logrus.Fields(map[string]interface{}{
		"Component":          component,
		"RequestID":          tsm.Request.GetID(),
		"JobType":            tsm.Request.GetJobType(),
		"RequestState":       tsm.Request.GetJobState(),
		"LastJobExecutionID": tsm.Request.GetLastJobExecutionID(),
		"Priority":           tsm.Request.GetJobPriority(),
		"ExecutionState":     tsm.JobExecution.JobState,
		"JobState":           tsm.Request.GetJobState(),
		"JobRetried":         tsm.Request.GetRetried(),
		"JobExecutionID":     tsm.JobExecution.ID,
		"TaskExecutionID":    tsm.TaskExecution.ID,
		"TaskType":           tsm.TaskExecution.TaskType,
		"TaskStatus":         tsm.TaskExecution.TaskState,
		"TaskRetried":        tsm.TaskExecution.Retried,
		"Message":            tsm.TaskExecution.ExitCode,
		"AntID":              tsm.TaskExecution.AntID,
		"ErrorCode":          tsm.TaskExecution.ErrorCode,
		"ErrorMessage":       tsm.TaskExecution.ErrorMessage,
	})

	if tsm.LastTaskExecution != nil {
		fields["LastTaskExecutionID"] = tsm.LastTaskExecution.ID
	}

	for i, e := range err {
		if e != nil {
			fields[fmt.Sprintf("Error%d", i+1)] = e
		}
	}

	return fields
}
