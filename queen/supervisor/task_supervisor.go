package supervisor

import (
	"context"
	"fmt"
	"time"

	"plexobject.com/formicary/queen/config"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/queue"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/fsm"
)

// TaskSupervisor for executing task
type TaskSupervisor struct {
	serverCfg        *config.ServerConfig
	taskStateMachine *fsm.TaskExecutionStateMachine
	cancel           context.CancelFunc
}

// NewTaskSupervisor creates supervisor for task execution
func NewTaskSupervisor(
	serverCfg *config.ServerConfig,
	stateMachine *fsm.TaskExecutionStateMachine) *TaskSupervisor {
	return &TaskSupervisor{
		serverCfg:        serverCfg,
		taskStateMachine: stateMachine,
		cancel:           func() {},
	}
}

// Execute - creates periodic ticker for scheduling pending jobs
func (ts *TaskSupervisor) Execute(
	ctx context.Context) error {
	return ts.execute(ctx)
}

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
// executing job
func (ts *TaskSupervisor) execute(
	ctx context.Context) (err error) {
	started := time.Now()

	timeout := ts.taskStateMachine.TaskDefinition.Timeout
	if timeout == 0 && ts.serverCfg.MaxTaskTimeout > 0 {
		timeout = ts.serverCfg.MaxTaskTimeout
	}

	if timeout > 0 {
		// timeout will be handled by ant but here we are adding additional check with additional time
		ctx, ts.cancel = context.WithTimeout(ctx, timeout+time.Second*2)
	} else {
		ctx, ts.cancel = context.WithCancel(ctx)
	}

	defer ts.cancel()

	// If this is continuation from last execution and task was completed successfully then use it
	if ts.taskStateMachine.TaskExecution.TaskState.Completed() {
		logrus.WithFields(ts.taskStateMachine.LogFields("TaskSupervisor")).
			Infof("task %s already completed so won't run it again",
				ts.taskStateMachine.TaskDefinition.TaskType)

		return nil
	}

	// we will save task state in the end
	defer func() {
		if !ts.taskStateMachine.TaskExecution.TaskState.IsTerminal() {
			if err == nil {
				if ctx.Err() != nil {
					err = fmt.Errorf("%v (timeout=%s/%s)",
						ctx.Err(), timeout, time.Now().Sub(started).String())
				} else {
					err = fmt.Errorf("unknown error executing task (timeout=%s/%s)",
						timeout, time.Now().Sub(started).String())
				}
			}
			ts.taskStateMachine.SetFailed(err)
		}
		// save final state
		saveErr := ts.taskStateMachine.FinalizeTaskState(ctx)
		if ts.taskStateMachine.TaskExecution.Failed() {
			logrus.WithFields(ts.taskStateMachine.LogFields("TaskSupervisor", err, saveErr)).
				Warnf("failed to run task '%s', exit=%s!",
					ts.taskStateMachine.TaskDefinition.TaskType, ts.taskStateMachine.TaskExecution.ExitCode)
		} else {
			logrus.WithFields(ts.taskStateMachine.LogFields("TaskSupervisor", err, saveErr)).
				Infof("completed task successfully '%s', exit=%s!",
					ts.taskStateMachine.TaskDefinition.TaskType, ts.taskStateMachine.TaskExecution.ExitCode)
		}
	}()

	// PrepareExecution validates ant reservation and initialize previous task execution if needed
	if err = ts.taskStateMachine.PrepareExecution(ctx); err != nil {
		// task is updated with FAILED
		// changing job state from EXECUTING to FAILED
		return fmt.Errorf("failed to prepare task for execution due to %w", err)
	}

	logrus.WithFields(ts.taskStateMachine.LogFields("TaskSupervisor")).
		Infof("starting task %s ...",
			ts.taskStateMachine.TaskDefinition.TaskType,
		)

	// mark task as executing
	if err = ts.taskStateMachine.SetTaskToExecuting(ctx); err == nil {
		err = ts.tryExecuteTask(ctx)
	}

	return
}

// This method checks if task was completed in last job run then reuse it
// Otherwise it submits the request to remote ant
func (ts *TaskSupervisor) tryExecuteTask(
	ctx context.Context) (err error) {

	if ts.taskStateMachine.TaskDefinition.IsExcept() {
		_, _ = ts.taskStateMachine.TaskExecution.AddContext(
			"Except", ts.taskStateMachine.TaskDefinition.Except)
		ts.taskStateMachine.TaskExecution.TaskState = common.COMPLETED
		ts.taskStateMachine.TaskExecution.ExitCode = "SKIPPED"
		ts.taskStateMachine.TaskExecution.ExitMessage = "Skipped task due to except flag"
		return nil
	}

	// Build task request
	var taskReq *common.TaskRequest
	if taskReq, err = ts.taskStateMachine.BuildTaskRequest(); err != nil {
		return fmt.Errorf("failed to build request for '%s' due to %v",
			ts.taskStateMachine.TaskDefinition.TaskType, err)
	}

	// Reuse previous task state if completed successfully
	if ts.taskStateMachine.CanReusePreviousResult() {
		if taskResp, err := ts.taskStateMachine.BuildTaskResponseFromPreviousResult(); err == nil {
			_, _ = ts.taskStateMachine.TaskExecution.AddContext(
				"ReusedPreviousResultFromTaskExecution",
				ts.taskStateMachine.LastTaskExecution.ID)
			return ts.taskStateMachine.UpdateTaskFromResponse(taskReq, taskResp)
		}
	}

	// Try running the task with retry loop - by default it will run once if no retry is set
	for executing := true; ts.taskStateMachine.CanRetry() || executing; ts.taskStateMachine.TaskExecution.Retried++ {
		taskReq.TaskRetry = ts.taskStateMachine.TaskExecution.Retried
		// send request and wait synchronously for response
		taskResp, err := ts.invoke(ctx, taskReq)
		if err == nil {
			err = ctx.Err()
		}

		if err == nil {
			err = ts.taskStateMachine.UpdateTaskFromResponse(taskReq, taskResp)
			executing = taskResp.Status == common.EXECUTING
			// error will be nil if status is COMPLETED
			if executing {
				// will keep calling task
			} else if err == nil ||
				ts.taskStateMachine.TaskExecution.Retried >= ts.taskStateMachine.TaskDefinition.Retry ||
				taskResp.Status == common.FATAL {
				break
			}
			sleepDuration := ts.taskStateMachine.TaskDefinition.GetDelayBetweenRetries()
			logrus.WithFields(
				ts.taskStateMachine.LogFields(
					"TaskSupervisor",
				)).Warnf("retrying task=%s status=%s exit=%s retried=%d delay=%s executing=%v",
				ts.taskStateMachine.TaskDefinition.TaskType,
				taskResp.Status,
				taskResp.ExitCode,
				ts.taskStateMachine.TaskExecution.Retried,
				sleepDuration,
				executing)
			time.Sleep(sleepDuration)
		} else {
			break
		}
	}
	return err
}

// invoking task request
func (ts *TaskSupervisor) invoke(
	ctx context.Context,
	taskReq *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	var b []byte
	if b, err = taskReq.Marshal(ts.taskStateMachine.Reservation.EncryptionKey); err != nil {
		return nil, fmt.Errorf("failed to marshal %s due to %w", taskReq, err)
	}
	var event *queue.MessageEvent

	ts.taskStateMachine.TaskExecution.AntID = ts.taskStateMachine.Reservation.AntID
	ts.taskStateMachine.TaskExecution.AddContext("AntTopic", ts.taskStateMachine.Reservation.AntTopic)

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "TaskSupervisor",
			"Task":      ts.taskStateMachine.TaskDefinition,
			"AntID":     ts.taskStateMachine.Reservation.AntID,
			"Request":   taskReq,
			"ReqTopic":  ts.taskStateMachine.Reservation.AntTopic,
			"ResTopic":  ts.serverCfg.GetResponseTopicTaskReply(),
			"RequestID": ts.taskStateMachine.Request.GetID(),
			"Retried":   ts.taskStateMachine.Request.GetRetried(),
		}).Infof("sending request to remote ant worker")
	}

	if event, err = ts.taskStateMachine.QueueClient.SendReceive(
		ctx,
		ts.taskStateMachine.Reservation.AntTopic,
		b,
		ts.serverCfg.GetResponseTopicTaskReply(),
		queue.NewMessageHeaders(
			queue.DisableBatchingKey, "true",
			queue.MessageTarget, ts.taskStateMachine.Reservation.AntID,
			"RequestID", fmt.Sprintf("%d", taskReq.JobRequestID),
			"TaskType", taskReq.TaskType,
			"UserID", taskReq.UserID,
		),
	); err != nil {
		return nil, err
	}
	if event == nil {
		return nil, fmt.Errorf("received nil response from request %v", taskReq)
	}
	taskResp, err = common.UnmarshalTaskResponse(ts.taskStateMachine.Reservation.EncryptionKey, event.Payload)
	if err != nil {
		return taskReq.ErrorResponse(err), nil
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":         "TaskSupervisor",
			"Task":              ts.taskStateMachine.TaskDefinition,
			"AntID":             ts.taskStateMachine.Reservation.AntTopic,
			"OriginalState":     taskResp.Status,
			"OriginalErrorCode": taskResp.ErrorCode,
			"ExitCode":          taskResp.ExitCode,
			"TaskResp":          taskResp,
			"Event":             event.Properties,
			"ReqTopic":          ts.taskStateMachine.Reservation.AntTopic,
			"ResTopic":          ts.serverCfg.GetResponseTopicTaskReply(),
			"RequestID":         ts.taskStateMachine.Request.GetID(),
			"Retried":           ts.taskStateMachine.Request.GetRetried(),
		}).Debugf("received reply")
	}
	newState, newErrorCode := ts.taskStateMachine.TaskDefinition.OverrideStatusAndErrorCode(taskResp.ExitCode)
	if newState != "" {
		logrus.WithFields(logrus.Fields{
			"Component":         "TaskSupervisor",
			"Task":              ts.taskStateMachine.TaskDefinition,
			"OriginalState":     taskResp.Status,
			"OriginalErrorCode": taskResp.ErrorCode,
			"ExitCode":          taskResp.ExitCode,
			"NewState":          newState,
			"NewErrorCode":      newErrorCode,
			"TaskResp":          taskResp,
			"RequestID":         ts.taskStateMachine.Request.GetID(),
			"Retried":           ts.taskStateMachine.Request.GetRetried(),
		}).Warnf("overriding state and error code")
		taskResp.Status = newState
		if taskResp.ErrorCode != "" {
			taskResp.AddContext("OriginalErrorCode", taskResp.ErrorCode)
		}
		if newErrorCode != "" {
			taskResp.ErrorCode = newErrorCode
		}
	}
	event.Ack() // auto-ack
	return
}
