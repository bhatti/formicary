package supervisor

import (
	"context"
	"fmt"
	"strings"
	"time"

	evbus "github.com/asaskevich/EventBus"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/queen/types"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/async"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/fsm"
)

// JobSupervisor for launching jobs
type JobSupervisor struct {
	serverCfg       *config.ServerConfig
	jobStateMachine *fsm.JobExecutionStateMachine
	eventBus        evbus.Bus
	id              string
	cancel          context.CancelFunc
}

// NewJobSupervisor creates supervisor for job execution runs each task in the job
func NewJobSupervisor(
	serverCfg *config.ServerConfig,
	stateMachine *fsm.JobExecutionStateMachine,
	eventBus evbus.Bus,
) *JobSupervisor {
	return &JobSupervisor{
		serverCfg:       serverCfg,
		jobStateMachine: stateMachine,
		eventBus:        eventBus,
		id:              fmt.Sprintf("supervisor-%s", stateMachine.Request.GetID()),
		cancel:          func() {},
	}
}

// AsyncExecute - executes job execution in a separate goroutine
func (js *JobSupervisor) AsyncExecute(
	ctx context.Context) async.Awaiter {
	handler := func(ctx context.Context, _ interface{}) (interface{}, error) {
		return nil, js.tryExecuteJob(ctx)
	}
	js.jobStateMachine.MetricsRegistry.Incr(
		"job_started_total",
		map[string]string{
			"Org": js.jobStateMachine.Request.GetOrganizationID(),
			"Job": js.jobStateMachine.Request.GetJobType()})
	return async.Execute(ctx, handler, async.NoAbort, nil)
}

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
// executing job and all tasks within it
func (js *JobSupervisor) tryExecuteJob(
	ctx context.Context) (err error) {
	logrus.WithFields(js.jobStateMachine.LogFields(
		"JobSupervisor",
	)).Infof("starting job %s...", js.jobStateMachine.JobDefinition.JobType)

	// Marking job request/execution to EXECUTING
	if err = js.jobStateMachine.SetJobStatusToExecuting(ctx); err != nil {
		return js.jobStateMachine.LaunchFailed(
			ctx,
			fmt.Errorf("failed to set job-execution state to EXECUTING due to %w", err))
	}

	timeout := js.jobStateMachine.JobDefinition.Timeout
	if js.jobStateMachine.JobDefinition.Timeout == 0 && js.serverCfg.Common.MaxJobTimeout > 0 {
		timeout = js.serverCfg.Common.MaxJobTimeout
	}
	if timeout > 0 {
		ctx, js.cancel = context.WithTimeout(
			ctx,
			timeout+time.Second*2)
	} else {
		ctx, js.cancel = context.WithCancel(ctx)
	}

	defer js.cancel()

	if err = js.eventBus.SubscribeAsync(
		js.serverCfg.Common.GetJobExecutionLifecycleTopic(),
		js.UpdateFromJobLifecycleEvent,
		false,
	); err != nil {
		return fmt.Errorf("failed to subscribe to event bus due to %w", err)
	}

	ticker := js.startTickerToUpdateRequestTimestamp(ctx)

	defer func() {
		_ = js.eventBus.Unsubscribe(
			js.serverCfg.Common.GetJobExecutionLifecycleTopic(), js.UpdateFromJobLifecycleEvent)
		ticker.Stop()
	}()

	var task *types.TaskDefinition
	var errorCode string

	// ONLY check for resume from manual approval at job start (not expensive per-task)
	startTaskType := js.jobStateMachine.JobExecution.GetCurrentTask()
	if startTaskType != "" {
		_, taskExec := js.jobStateMachine.JobExecution.GetTask("", startTaskType)
		var taskState common.RequestState
		if taskExec != nil {
			taskState = taskExec.TaskState
		}
		logrus.WithFields(js.jobStateMachine.LogFields("JobSupervisor")).
			Infof("resuming job execution after approval, starting with task: %s, state: %s",
				startTaskType, taskState)
	}

	// Begin execution with first task in retry loop - by default it will run job once unless retry is set
	for canExecute := true; canExecute; canExecute = js.jobStateMachine.Request.IncrRetried() > 0 &&
		js.jobStateMachine.CanRetry() {

		// Use determined start task or find first task for new execution
		if startTaskType != "" {
			task = js.jobStateMachine.JobDefinition.GetTask(startTaskType)
		}
		if task == nil {
			// Find the first task to run or in case of restart, execute last task executing
			task, err = js.jobStateMachine.JobDefinition.GetFirstTask()
			if err != nil {
				break
			}
		}
		errorCode, err = js.executeNextTask(ctx, task.TaskType)
		if err == nil {
			// if task had on-failed to next task, we will try to find failed status of that task
			var failedTask *types.TaskExecution
			failedTask, errorCode, err = js.jobStateMachine.JobExecution.GetFailedTaskError()
			if failedTask != nil {
				logrus.WithFields(js.jobStateMachine.LogFields("JobSupervisor")).
					Infof("overriding error-code and error from task-execution job='%s' retried=%d error-code=%s error=%s failed-task=%s, active=%v",
						js.jobStateMachine.JobDefinition.JobType,
						js.jobStateMachine.Request.GetRetried(),
						errorCode,
						err,
						failedTask,
						failedTask.Active)
			}
		} else {
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(js.jobStateMachine.LogFields("JobSupervisor")).
					WithError(err).
					Debugf("failed to get next for job='%s' task=%s oncompleted=%s onfailed=%s errorcode=%s",
						js.jobStateMachine.JobDefinition.JobType, task.TaskType,
						task.OnCompleted, task.OnFailed, errorCode)
			}
		}

		// quit retrying upon success, fatal error or if job needs to be restarted and rescheduled later
		if err == nil || errorCode == common.ErrorFatal || errorCode == common.ErrorRestartJob ||
			errorCode == common.ErrorPauseJob || errorCode == common.ErrorManualApprovalRequired ||
			js.jobStateMachine.Request.GetRetried() == js.jobStateMachine.JobDefinition.Retry {
			break
		}

		// retry after a short delay
		sleepDuration := js.jobStateMachine.JobDefinition.GetDelayBetweenRetries()

		logrus.WithFields(js.jobStateMachine.LogFields("JobSupervisor")).
			Warnf("retrying job='%s' retried=%d error-code=%s error=%s wait=%s ...",
				js.jobStateMachine.JobDefinition.JobType,
				js.jobStateMachine.Request.GetRetried(),
				errorCode,
				err,
				sleepDuration)
		time.Sleep(sleepDuration)

		// Reset start task type for retries
		startTaskType = ""
	}

	// check if job ran successfully
	if err == nil {
		logrus.WithFields(js.jobStateMachine.LogFields(
			"JobSupervisor",
		)).Infof("completed job successfully %s state=%s", js.jobStateMachine.JobDefinition.JobType,
			js.jobStateMachine.Request.GetJobState())
		return js.jobStateMachine.ExecutionCompleted(ctx)
	}

	// check finalized task if job failed - TODO move this up
	if js.jobStateMachine.Request.GetJobState().IsTerminal() {
		for _, lastTask := range js.jobStateMachine.JobDefinition.GetLastAlwaysRunTasks() {
			if lastTask != nil {
				_, lastTaskExists := js.jobStateMachine.JobExecution.GetTask("", lastTask.TaskType)
				if lastTaskExists == nil {
					_, _ = js.submitTask(ctx, lastTask.TaskType)
				}
			}
		}
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(js.jobStateMachine.LogFields(
			"JobSupervisor",
		)).Debugf("job '%s' failed with error-code '%s' and error '%s'",
			js.jobStateMachine.JobDefinition.JobType, errorCode, err)
	}

	// check if job was restarted to put back in the queue
	if errorCode == common.ErrorRestartJob {
		if err == nil {
			err = fmt.Errorf("forcing job to restarted state")
		}
		return js.jobStateMachine.RestartJobBackToPendingPaused(err)
	} else if errorCode == common.ErrorPauseJob {
		return js.jobStateMachine.PauseJob()
	} else if errorCode == common.ErrorManualApprovalRequired {
		logrus.WithFields(js.jobStateMachine.LogFields(
			"JobSupervisor",
		)).Warnf("[js] request %s with job '%s' requires manual approval",
			js.jobStateMachine.Request.GetID(), js.jobStateMachine.JobDefinition.JobType)
		return nil
	} else {
		logrus.WithFields(js.jobStateMachine.LogFields(
			"JobSupervisor",
		)).Warnf("[js] request %s with job '%s' will be marked failed",
			js.jobStateMachine.Request.GetID(), js.jobStateMachine.JobDefinition.JobType)
	}

	// job failed
	if saveErr := js.jobStateMachine.ExecutionFailed(
		ctx,
		errorCode,
		err.Error()); saveErr != nil {
		logrus.WithFields(js.jobStateMachine.LogFields(
			"JobSupervisor",
		)).Warnf("job '%s' could not be saved due to error '%s', job error '%s'",
			js.jobStateMachine.JobDefinition.JobType, saveErr, err)

	}
	return err
}

// UpdateFromJobLifecycleEvent updates if current job is cancelled
func (js *JobSupervisor) UpdateFromJobLifecycleEvent(
	ctx context.Context,
	jobExecutionLifecycleEvent *events.JobExecutionLifecycleEvent) error {
	// Check if this is cancel request from outside
	if jobExecutionLifecycleEvent.JobRequestID == js.jobStateMachine.Request.GetID() &&
		jobExecutionLifecycleEvent.JobState.IsTerminal() {
		defer js.cancel()
		// Note ExecutionCancelled won't call lifecycle event because cancel API fires it
		errorCode := common.ErrorJobCancelled
		errorMessage := "job cancelled by user"
		if js.jobStateMachine.JobExecution.ErrorCode != "" {
			errorCode = js.jobStateMachine.JobExecution.ErrorCode
		}
		if js.jobStateMachine.JobExecution.ErrorMessage != "" {
			errorMessage = js.jobStateMachine.JobExecution.ErrorMessage
		}
		if err := js.jobStateMachine.ExecutionCancelled(
			ctx,
			errorCode,
			errorMessage); err != nil {
			if !strings.Contains(err.Error(), "terminal") {
				logrus.WithFields(logrus.Fields{
					"Component":                  "JobSupervisor",
					"ID":                         jobExecutionLifecycleEvent.ID,
					"Target":                     js.id,
					"RequestID":                  jobExecutionLifecycleEvent.JobRequestID,
					"RequestState":               js.jobStateMachine.Request.GetJobState(),
					"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
					"Error":                      err}).Warnf("failed to cancel job from lifecycle job event")
			}
			return err
		}

		logrus.WithFields(logrus.Fields{
			"Component":                  "JobSupervisor",
			"ID":                         jobExecutionLifecycleEvent.ID,
			"Target":                     js.id,
			"RequestID":                  jobExecutionLifecycleEvent.JobRequestID,
			"RequestState":               js.jobStateMachine.Request.GetJobState(),
			"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
		}).Infof("cancelled job as a result of job lifecycle event")
	} else if jobExecutionLifecycleEvent.JobRequestID == js.jobStateMachine.Request.GetID() &&
		jobExecutionLifecycleEvent.JobState.Paused() {
		defer js.cancel()
		errorCode := common.ErrorPauseJob
		errorMessage := "job paused by user"
		if js.jobStateMachine.JobExecution.ErrorCode != "" {
			errorCode = js.jobStateMachine.JobExecution.ErrorCode
		}
		if js.jobStateMachine.JobExecution.ErrorMessage != "" {
			errorMessage = js.jobStateMachine.JobExecution.ErrorMessage
		}
		if err := js.jobStateMachine.PauseJob(); err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":                  "JobSupervisor",
				"ID":                         jobExecutionLifecycleEvent.ID,
				"Target":                     js.id,
				"RequestID":                  jobExecutionLifecycleEvent.JobRequestID,
				"RequestState":               js.jobStateMachine.Request.GetJobState(),
				"ErrorCode":                  errorCode,
				"ErrorMessage":               errorMessage,
				"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
				"Error":                      err}).Warnf("failed to pause job from lifecycle job event")
			return err
		}

		logrus.WithFields(logrus.Fields{
			"Component":                  "JobSupervisor",
			"ID":                         jobExecutionLifecycleEvent.ID,
			"Target":                     js.id,
			"RequestID":                  jobExecutionLifecycleEvent.JobRequestID,
			"RequestState":               js.jobStateMachine.Request.GetJobState(),
			"ErrorCode":                  errorCode,
			"ErrorMessage":               errorMessage,
			"JobExecutionLifecycleEvent": jobExecutionLifecycleEvent,
		}).Infof("paused job as a result of job lifecycle event")
	}

	return nil
}

// executeNextTask next task iteratively until we reach last task
func (js *JobSupervisor) executeNextTask(
	ctx context.Context,
	taskType string) (errorCode string, err error) {
	// Abort if job already cancelled
	if js.jobStateMachine.JobExecution.JobState.IsTerminal() {
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(js.jobStateMachine.LogFields("JobSupervisor")).
				Debugf("job execution for %s is in terminal state %s",
					js.jobStateMachine.JobDefinition.JobType, js.jobStateMachine.JobExecution.JobState)
		}
		return
	}

	// Create task state machine and run task
	var taskStateMachine *fsm.TaskExecutionStateMachine
	taskStateMachine, err = js.submitTask(ctx, taskType)

	// initialize error code if available
	if taskStateMachine != nil {
		errorCode = taskStateMachine.TaskExecution.ErrorCode
	}

	if err != nil {
		logrus.WithFields(js.jobStateMachine.LogFields("JobSupervisor")).
			WithError(err).
			Warnf("[js] failed to submit task for job='%s' task=%s errorcode=%s",
				js.jobStateMachine.JobDefinition.JobType, taskType, errorCode)
		return
	}

	// Handle manual approval case - stop execution and save state
	if taskStateMachine.TaskExecution.TaskState == common.MANUAL_APPROVAL_REQUIRED {
		logrus.WithFields(js.jobStateMachine.LogFields("JobSupervisor")).
			Infof("[js] Job paused for manual approval of task: %s", taskStateMachine.TaskDefinition.TaskType)

		// Set job to manual approval required state using existing repository method
		if err = js.jobStateMachine.JobManager.SetJobRequestAndExecutingStatusToApprovalRequired(
			js.jobStateMachine.JobExecution.ID, taskStateMachine.TaskDefinition.TaskType); err != nil {
			return errorCode, fmt.Errorf("failed to set job to manual approval state: %w", err)
		}

		// Stop execution here - scheduler will resume when approved
		return common.ErrorManualApprovalRequired, fmt.Errorf("job paused for manual approval of task: %s", taskStateMachine.TaskDefinition.TaskType)
	}

	// Continue with next task if task failed but is optional or succeeded
	var nextTaskDef *types.TaskDefinition
	nextTaskDef, _, err = js.jobStateMachine.JobDefinition.GetNextTask(
		taskStateMachine.TaskDefinition,
		taskStateMachine.TaskExecution.TaskState,
		taskStateMachine.TaskExecution.ExitCode)

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":    "JobSupervisor",
			"ID":           js.serverCfg.Common.ID,
			"Task":         taskStateMachine.TaskDefinition.TaskType,
			"ExitCode":     taskStateMachine.TaskExecution.ExitCode,
			"Status":       taskStateMachine.TaskExecution.TaskState,
			"AllowFailure": taskStateMachine.TaskDefinition.AllowFailure,
			"Retry":        js.jobStateMachine.Request.GetRetried(),
			"MaxRetry":     js.jobStateMachine.JobDefinition.Retry,
			"Error":        err,
			"Next":         nextTaskDef,
		}).Debugf("fetching next task")
	}

	if err != nil {
		return
	}

	// check if keep going
	if nextTaskDef == nil {
		if taskStateMachine.TaskExecution.TaskState == common.FAILED &&
			!taskStateMachine.TaskDefinition.AllowFailure {
			return taskStateMachine.TaskExecution.ErrorCode,
				fmt.Errorf("%s", taskStateMachine.TaskExecution.ErrorMessage)
		} else if taskStateMachine.TaskExecution.TaskState == common.PAUSED {
			return taskStateMachine.TaskExecution.ErrorCode,
				fmt.Errorf("%s", taskStateMachine.TaskExecution.ErrorMessage)
		} else if len(taskStateMachine.TaskDefinition.OnExitCode) > 0 {
			return common.ErrorInvalidNextTask,
				fmt.Errorf("cannot find next task after %s, unexpected task status=%s, exit-code: %s, error-code: %s, multiple exits=%v",
					taskType, taskStateMachine.TaskExecution.TaskState, taskStateMachine.TaskExecution.ExitCode,
					taskStateMachine.TaskExecution.ErrorCode, taskStateMachine.TaskDefinition.OnExitCode)
		} else if taskStateMachine.TaskExecution.TaskState == common.COMPLETED ||
			taskStateMachine.TaskDefinition.AllowFailure {
			return "", nil
		} else {
			return taskStateMachine.TaskExecution.ErrorCode,
				fmt.Errorf("last task failed after %s, unknown task status=%s, exit=%s",
					taskType, taskStateMachine.TaskExecution.TaskState, taskStateMachine.TaskExecution.ExitCode)
		}
	} else {
		return js.executeNextTask(ctx, nextTaskDef.TaskType)
	}
}

func (js *JobSupervisor) submitTask(
	ctx context.Context,
	taskType string) (taskStateMachine *fsm.TaskExecutionStateMachine, err error) {
	// Creating state machine for request and execution
	taskStateMachine, err = fsm.NewTaskExecutionStateMachine(js.jobStateMachine, taskType)
	if err != nil {
		return nil, err
	}
	// Execute the task
	err = NewTaskSupervisor(js.serverCfg, taskStateMachine).Execute(ctx)

	// return error if it's not optional
	if err != nil && !taskStateMachine.TaskDefinition.AllowFailure {
		// changing state from EXECUTING to FAILED
		return taskStateMachine,
			fmt.Errorf("[js] failed to execute task for '%s' due to %w", taskType, err)
	}
	return taskStateMachine, nil
}
